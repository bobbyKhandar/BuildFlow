package workspace

import (
	"buildflow/internal/structured"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// DetectedPackage is one candidate work package found in the Step 4 output.
// Nothing is written until the user confirms the preview.
type DetectedPackage struct {
	ID             string    `json:"id"`
	Title          string    `json:"title"`
	Heading        string    `json:"heading"`
	HeadingLevel   int       `json:"headingLevel"`
	StartLine      int       `json:"startLine"`
	EndLine        int       `json:"endLine"`
	LineCount      int       `json:"lineCount"`
	Content        string    `json:"content"`
	TargetPath     string    `json:"targetPath"`
	FileExists     bool      `json:"fileExists"`
	ExistsHash     string    `json:"existsHash,omitempty"`
	ExistsModified time.Time `json:"existsModified,omitempty"`
	ExistingChars  int       `json:"existingChars,omitempty"`
	// Warnings explains why a package might be wrong, e.g. duplicate ids.
	Warnings   []string           `json:"warnings,omitempty"`
	Structured *structured.Result `json:"structured,omitempty"`
	Projection *AssemblySource    `json:"projection,omitempty"`
}

// DetectionResult is the full preview returned to the user.
type DetectionResult struct {
	Source      string             `json:"source"`
	TotalLines  int                `json:"totalLines"`
	Packages    []DetectedPackage  `json:"packages"`
	Unassigned  string             `json:"unassigned"`
	Warnings    []string           `json:"warnings"`
	TextPreview string             `json:"textPreview,omitempty"`
	Overview    string             `json:"overview,omitempty"`
	Footer      string             `json:"footer,omitempty"`
	Structured  *structured.Result `json:"structured,omitempty"`
}

// packageHeadingPatterns recognise the common ways a Step 4 output names work
// packages. They are intentionally generous because the user reviews and
// corrects the split before anything is written.
var packageHeadingPatterns = []*regexp.Regexp{
	// "## P01 - Title", "### Work Package P02: Title", "## WP P03 Title"
	regexp.MustCompile(`(?i)^(#{1,6})\s*(?:work\s*package|package|wp)?\s*[:#\-–—]?\s*(P\d{1,3})\b[\s.:,\-–—]*(.*)$`),
	// "**Work Package P01** - Title" (bold, not a heading)
	regexp.MustCompile(`(?i)^(?:[-*+]\s+|\d+[.)]\s+)?\*{0,2}\s*(?:work\s*package|package|wp)\s*\**\s*[:#\-–—]?\s*(P\d{1,3})\b[\s.:,\-–—]*\**\s*(.*)$`),
	// "P01 - Title" / "P01: Title" on its own line
	regexp.MustCompile(`(?i)^(?:[-*+]\s+|\d+[.)]\s+)?\**\s*(P\d{1,3})\b\**\s*[.:,\-–—]\s+(\S.*)$`),
	// "## P01" with nothing else on the line
	regexp.MustCompile(`(?i)^(#{1,6})\s*(?:work\s*package|package|wp)?\s*[:#\-–—]?\s*(P\d{1,3})\s*$`),
}

type headingHit struct {
	line  int // zero based
	level int
	id    string
	title string
	raw   string
}

// findPackageHeadings scans text and returns the ordered heading hits.
func findPackageHeadings(text string) []headingHit {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	var hits []headingHit
	for i, line := range lines {
		trimmed := strings.TrimRight(line, " \t")
		if strings.TrimSpace(trimmed) == "" {
			continue
		}
		for pattern, re := range packageHeadingPatterns {
			m := re.FindStringSubmatch(trimmed)
			if m == nil {
				continue
			}
			level := 0
			var id, title string
			switch pattern {
			case 0, 3:
				level = len(m[1])
				id = m[2]
				if len(m) > 3 {
					title = m[3]
				}
			case 1, 2:
				id, title = m[1], m[2]
			}
			id = strings.ToUpper(id)
			title = strings.Trim(title, "*-_ \t")
			if title == "" {
				// Maybe the title is on the following line.
				if i+1 < len(lines) {
					next := strings.TrimSpace(strings.Trim(lines[i+1], "#*_ \t"))
					if next != "" && len(next) < 140 && !strings.HasPrefix(next, "-") {
						title = next
					}
				}
			}
			// Ignore duplicates of the exact same heading line.
			if n := len(hits); n > 0 && hits[n-1].line == i {
				continue
			}
			hits = append(hits, headingHit{line: i, level: level, id: id, title: title, raw: trimmed})
			break
		}
	}
	// Drop hits that appear inside a fenced code block, which would be example
	// output rather than real package headings.
	return dropFencedHits(lines, hits)
}

func dropFencedHits(lines []string, hits []headingHit) []headingHit {
	if len(hits) == 0 {
		return nil
	}
	fenced := make([]bool, len(lines))
	var fenceChar byte
	width := 0
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if len(t) >= 3 && (t[0] == '`' || t[0] == '~') {
			n := 0
			for n < len(t) && t[n] == t[0] {
				n++
			}
			if n >= 3 {
				if fenceChar == 0 {
					fenceChar, width = t[0], n
					fenced[i] = true
					continue
				}
				if t[0] == fenceChar && n >= width && strings.TrimSpace(t[n:]) == "" {
					fenced[i] = true
					fenceChar = 0
					continue
				}
			}
		}
		fenced[i] = fenceChar != 0
	}

	var out []headingHit
	for _, h := range hits {
		if h.line < len(fenced) && fenced[h.line] {
			continue
		}
		out = append(out, h)
	}
	return out
}

// DetectPackages splits Step 4 output into candidate work packages. It never
// writes anything.
func (s *Service) DetectPackages(project, step04Content string) (DetectionResult, error) {
	if _, err := s.ProjectPath(project); err != nil {
		return DetectionResult{}, err
	}
	res := DetectionResult{Source: "outputs/step-04.md", Packages: []DetectedPackage{}, Warnings: []string{}}
	normalized := strings.ReplaceAll(step04Content, "\r\n", "\n")
	res.TotalLines = len(strings.Split(normalized, "\n"))
	parsed := s.InspectDocument(project, Doc{Kind: KindOutput, Stage: "step-04", Content: normalized})
	res.Structured = parsed
	narrative := normalized
	footerStart := -1
	if parsed != nil {
		footerStart = parsed.FooterStart
	}
	if footerStart < 0 {
		footerStart = outsideHeadingOffset(normalized, "## BuildFlow Structured View")
	}
	if footerStart >= 0 {
		narrative = normalized[:footerStart]
		res.Footer = normalized[footerStart:]
	}
	if overview := deliveryOverviewOffset(narrative); overview >= 0 {
		res.Overview = narrative[overview:]
		narrative = narrative[:overview]
	}
	lines := strings.Split(strings.TrimRight(narrative, "\n"), "\n")
	hits := findPackageHeadings(strings.Join(lines, "\n"))
	if parsed == nil || parsed.Status != structured.Valid {
		res.Warnings = append(res.Warnings, "Structured Stage 4 data is unavailable or invalid; review the Markdown split before importing.")
	}

	if len(hits) == 0 {
		res.Unassigned = narrative
		res.Warnings = append(res.Warnings,
			"No package headings were found. Expected headings such as \"## P01 - Title\". "+
				"You can still create packages manually with the \"Add next package\" action.")
		return res, nil
	}

	// Anything above the first heading is reported separately so the user can
	// decide where it belongs instead of silently losing it.
	firstLine := hits[0].line
	res.Unassigned = strings.TrimSpace(strings.Join(lines[:firstLine], "\n"))

	var design *structured.Envelope
	if doc, err := s.ReadDoc(KindOutput, "step-03", "", project); err == nil && doc.Exists {
		result := structured.Parse(doc.Content, structured.Target{Stage: "step-03", Scope: "stage"})
		if result.Status == structured.Valid {
			design = result.Artifact
		}
	}
	seen := map[string]int{}
	for n, h := range hits {
		end := len(lines)
		if n+1 < len(hits) {
			end = hits[n+1].line
		}
		body := strings.Join(lines[h.line:end], "\n")
		body = strings.TrimRight(body, "\n") + "\n"
		norm, err := NormalizePackageID(h.id)
		if err != nil {
			continue
		}
		dp := DetectedPackage{
			ID:           norm,
			Title:        h.title,
			Heading:      h.raw,
			HeadingLevel: h.level,
			StartLine:    h.line + 1,
			EndLine:      end,
			LineCount:    end - h.line,
			Content:      body,
		}
		seen[norm]++
		if seen[norm] > 1 {
			// Preserve duplicate content for review, but import only the first.
			res.Warnings = append(res.Warnings, norm+" appears more than once; only the first occurrence is imported")
			res.Unassigned += "\n\n" + body
			continue
		}
		if parsed != nil && parsed.Status == structured.Valid {
			projected, provenance, err := projectPackageFooter(parsed.Artifact, norm, h.title, res.Source, hashBytes([]byte(step04Content)), design)
			if err != nil {
				res.Unassigned += "\n\n" + body
				res.Warnings = append(res.Warnings, err.Error())
				continue
			}
			dp.Content += "\n" + projected
			dp.Projection = provenance
		}
		dp.Structured = s.InspectDocument(project, Doc{Kind: KindPackage, Package: norm, Content: dp.Content})
		res.Packages = append(res.Packages, dp)
	}

	if parsed != nil && parsed.Status == structured.Valid {
		for _, pkg := range parsed.Blocks.(*structured.WorkPackageBlocks).Packages {
			matched := false
			for _, detected := range res.Packages {
				if detected.ID == pkg.ID {
					matched = true
					break
				}
			}
			if !matched {
				res.Warnings = append(res.Warnings, pkg.ID+" has no matching Markdown package heading and title; it will not be imported")
			}
		}
	}
	// Report combined content that was not assigned to a package.
	if res.Unassigned != "" {
		res.Warnings = append(res.Warnings,
			"Some combined answer content is unassigned and will not be imported; review it before importing.")
	}

	// Annotate existing files so the UI can protect hand-edited packages.
	for i := range res.Packages {
		p := &res.Packages[i]
		rel, err := Resolve(KindPackage, "", p.ID, project)
		if err != nil {
			continue
		}
		p.TargetPath = filepath.ToSlash(rel)
		doc, err := s.ReadDoc(KindPackage, "", p.ID, project)
		if err == nil && doc.Exists {
			p.FileExists = true
			p.ExistsHash = doc.Hash
			p.ExistsModified = doc.ModTime
			p.ExistingChars = len([]rune(doc.Content))
		}
		if p.ExistingChars > 0 {
			p.Warnings = append(p.Warnings,
				fmt.Sprintf("%s.md already exists with %d characters; overwriting replaces your edits", p.ID, p.ExistingChars))
		}
		if p.Title == "" {
			p.Warnings = append(p.Warnings, "No title was detected for this package; edit the title before importing.")
		}
	}

	sortPackages(res.Packages)
	return res, nil
}

func sortPackages(pkgs []DetectedPackage) {
	sort.SliceStable(pkgs, func(i, j int) bool {
		return packageNumber(pkgs[i].ID) < packageNumber(pkgs[j].ID)
	})
}

// ImportRequest confirms a package split. Only ids listed in Accept are
// written, and only ids listed in Overwrite may replace an existing file.
type ImportRequest struct {
	Packages  []DetectedPackage `json:"packages"`
	Accept    []string          `json:"accept"`
	Overwrite []string          `json:"overwrite"`
}

// ImportResult reports what was written.
type ImportResult struct {
	Created     []string `json:"created"`
	Skipped     []string `json:"skipped"`
	Overwritten []string `json:"overwritten"`
	Refused     []string `json:"refused"`
}

// ImportPackages writes confirmed package files. Existing files are never
// replaced unless the user explicitly lists them in Overwrite.
func (s *Service) ImportPackages(project string, req ImportRequest) (ImportResult, error) {
	if _, err := s.ProjectPath(project); err != nil {
		return ImportResult{}, err
	}
	// Every list starts empty rather than nil. A nil slice marshals to JSON
	// null, and a client that reads result.created.length would then fail on a
	// perfectly ordinary "nothing was created" answer. An array is the honest
	// shape for "a list of package ids".
	result := ImportResult{
		Created:     []string{},
		Skipped:     []string{},
		Overwritten: []string{},
		Refused:     []string{},
	}
	seenIDs := map[string]bool{}
	for _, p := range req.Packages {
		if err := ValidatePackageID(p.ID); err != nil {
			return result, err
		}
		if seenIDs[p.ID] {
			return result, fmt.Errorf("duplicate package id %q", p.ID)
		}
		seenIDs[p.ID] = true
	}
	accept := toSet(req.Accept)
	overwrite := toSet(req.Overwrite)

	for _, p := range req.Packages {
		if err := ValidatePackageID(p.ID); err != nil {
			return result, fmt.Errorf("invalid package id %q", p.ID)
		}
		if !accept[p.ID] {
			result.Skipped = append(result.Skipped, p.ID)
			continue
		}
		content := buildPackageFile(p)
		existing, err := s.ReadDoc(KindPackage, "", p.ID, project)
		if err != nil {
			return result, err
		}
		if existing.Exists && !overwrite[p.ID] {
			result.Refused = append(result.Refused, p.ID)
			result.Skipped = append(result.Skipped, p.ID)
			continue
		}
		// The preview hash, rather than a fresh read, is the write precondition.
		if existing.Hash != p.ExistsHash || existing.Exists != p.FileExists {
			result.Refused = append(result.Refused, p.ID)
			result.Skipped = append(result.Skipped, p.ID)
			continue
		}
		if _, err := s.WriteDoc(WriteRequest{
			Kind: KindPackage, Package: p.ID, Project: project,
			Content: content, BaseHash: p.ExistsHash,
		}); err != nil {
			var ce *ConflictError
			if ok := asConflict(err, &ce); ok {
				result.Refused = append(result.Refused, p.ID)
				result.Skipped = append(result.Skipped, p.ID)
				continue
			}
			return result, fmt.Errorf("cannot write %s.md: %w", p.ID, err)
		}
		if existing.Exists {
			result.Overwritten = append(result.Overwritten, p.ID)
		} else {
			result.Created = append(result.Created, p.ID)
		}
	}
	// Keep ordering stable for the UI.
	sort.Strings(result.Created)
	sort.Strings(result.Overwritten)
	sort.Strings(result.Skipped)
	sort.Strings(result.Refused)
	return result, nil
}

// EnsurePackageFrontMatter returns content shaped as a work package document:
// a YAML front matter block with id, title and status, then the body. Content
// that already carries front matter is returned unchanged, so updating a
// package never duplicates the header. This is what lets Step 4 write a slot
// straight into packages/PNN.md.
func EnsurePackageFrontMatter(pkg, content string) string {
	if strings.HasPrefix(strings.TrimSpace(content), "---") {
		return content
	}
	title := "Work package " + pkg
	for _, line := range strings.Split(content, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "#") {
			title = strings.TrimSpace(strings.TrimLeft(t, "# "))
			break
		}
	}
	return "---\n" +
		"id: " + pkg + "\n" +
		"title: " + yamlQuote(title) + "\n" +
		"status: draft\n" +
		"---\n\n" +
		content
}

func buildPackageFile(p DetectedPackage) string {
	title := p.Title
	if title == "" {
		title = "Untitled work package"
	}
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("id: " + p.ID + "\n")
	b.WriteString("title: " + yamlQuote(title) + "\n")
	b.WriteString("status: draft\n")
	b.WriteString("source: outputs/step-04.md\n")
	b.WriteString("imported: " + time.Now().UTC().Format(time.RFC3339) + "\n")
	b.WriteString("---\n\n")
	if !strings.HasPrefix(p.Content, "#") {
		b.WriteString("# Work package " + p.ID + " - " + title + "\n\n")
	}
	b.WriteString(p.Content)
	if !strings.HasSuffix(p.Content, "\n") {
		b.WriteString("\n")
	}
	return b.String()
}

func yamlQuote(s string) string {
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	if strings.ContainsAny(s, `:#"'`) || strings.HasPrefix(s, " ") || strings.HasSuffix(s, " ") {
		return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
	}
	return s
}

func toSet(items []string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, i := range items {
		m[strings.ToUpper(strings.TrimSpace(i))] = true
	}
	return m
}

func asConflict(err error, out **ConflictError) bool {
	if ce, ok := err.(*ConflictError); ok {
		*out = ce
		return true
	}
	return false
}

// ListPackages returns the package documents found in packages/, enriched with
// the state of their Step 5 input, snapshot and output.
func (s *Service) ListPackages(project string) ([]PackageInfo, error) {
	names, err := s.ListMarkdown("projects/" + project + "/packages")
	if err != nil {
		return nil, err
	}
	out := []PackageInfo{}
	for _, n := range names {
		norm, err := NormalizePackageID(n)
		if err != nil {
			continue
		}
		info := PackageInfo{ID: norm, Status: "empty"}
		validPrompt := false
		if doc, err := s.ReadDoc(KindPackage, "", norm, project); err == nil && doc.Exists {
			info.HasPackageDoc = true
			info.PackageDocHash = doc.Hash
			info.Title = packageTitleFromContent(doc.Content, norm)
		}
		if s.Exists(KindPackageInput, "step-05", norm, project) {
			info.HasInput = true
		}
		if s.Exists(KindPackageSnapshot, "step-05", norm, project) {
			info.HasSnapshot = true
		}
		if doc, err := s.ReadDoc(KindPackageOutput, "step-05", norm, project); err == nil && doc.Exists && doc.Size > 0 {
			info.HasOutput = true
			info.OutputHash = doc.Hash
			info.OutputModified = doc.ModTime
			parsed := s.InspectDocument(project, doc)
			validPrompt = parsed != nil && parsed.Status == structured.Valid && parsed.Artifact.State == "ready"
		}
		switch {
		case validPrompt:
			info.Status = "prompt-generated"
		case info.HasOutput:
			info.Status = "output-saved"
		case info.HasSnapshot:
			info.Status = "prompt-ready"
		case info.HasPackageDoc:
			info.Status = "ready"
		default:
			info.Status = "empty"
		}
		out = append(out, info)
	}
	sortPackagesInfo(out)
	return out, nil
}

func sortPackagesInfo(p []PackageInfo) {
	sort.SliceStable(p, func(i, j int) bool { return packageNumber(p[i].ID) < packageNumber(p[j].ID) })
}

var frontMatterTitle = regexp.MustCompile(`(?m)^title:\s*(.+)$`)

func packageTitleFromContent(content, fallback string) string {
	head := content
	if len(head) > 2000 {
		head = head[:2000]
	}
	if strings.HasPrefix(head, "---") {
		if end := strings.Index(head[3:], "\n---"); end >= 0 {
			if m := frontMatterTitle.FindStringSubmatch(head[3 : end+3]); m != nil {
				return strings.Trim(strings.TrimSpace(m[1]), `"'`)
			}
		}
	}
	for _, line := range strings.Split(head, "\n") {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		if strings.HasPrefix(t, "#") {
			return strings.TrimSpace(strings.TrimLeft(t, "# "))
		}
		break
	}
	return fallback
}

// NextPackageID returns the next free package number: P01 for an empty project,
// and N+1 otherwise.
func (s *Service) NextPackageID(project string) (string, error) {
	names, err := s.ListMarkdown("projects/" + project + "/packages")
	if err != nil {
		return "", err
	}
	max := 0
	for _, n := range names {
		if num := packageNumber(n); num > max {
			max = num
		}
	}
	return fmt.Sprintf("P%02d", max+1), nil
}

// CreatePackage adds a new package document, continuing the sequence. This is
// the "Add next package" action, so a project with N packages can reach N+1.
func (s *Service) CreatePackage(project, requestedID, title string) (PackageInfo, error) {
	id := strings.TrimSpace(requestedID)
	if id == "" {
		next, err := s.NextPackageID(project)
		if err != nil {
			return PackageInfo{}, err
		}
		id = next
	}
	norm, err := NormalizePackageID(id)
	if err != nil {
		return PackageInfo{}, err
	}
	if s.Exists(KindPackage, "", norm, project) {
		return PackageInfo{}, fmt.Errorf("%s.md already exists; open it instead of creating a new one", norm)
	}
	title = strings.TrimSpace(title)
	if title == "" {
		title = "Work package " + norm
	}
	content := "---\n" +
		"id: " + norm + "\n" +
		"title: " + yamlQuote(title) + "\n" +
		"status: draft\n" +
		"created: " + time.Now().UTC().Format(time.RFC3339) + "\n" +
		"---\n\n" +
		"# Work package " + norm + " - " + title + "\n\n" +
		"**Goal**\n\n_Describe the single outcome this package delivers._\n\n" +
		"**Scope in**\n\n- \n\n" +
		"**Scope out**\n\n- \n\n" +
		"**Prerequisites**\n\n- \n\n" +
		"**Deliverables**\n\n- \n\n" +
		"**Tests**\n\n- \n\n" +
		"**Done when**\n\n- \n"
	if _, err := s.WriteDoc(WriteRequest{
		Kind: KindPackage, Package: norm, Project: project, Content: content,
	}); err != nil {
		return PackageInfo{}, err
	}
	// Also make sure the Step 5 folders for this package exist.
	for _, kind := range []string{KindPackageInput, KindPackageOutput} {
		rel, err := Resolve(kind, "step-05", norm, project)
		if err != nil {
			continue
		}
		abs, err := s.absolute(rel)
		if err != nil {
			continue
		}
		_ = os.MkdirAll(filepath.Dir(abs), 0o755)
	}
	list, err := s.ListPackages(project)
	if err != nil {
		return PackageInfo{ID: norm, Title: title}, nil
	}
	for _, p := range list {
		if p.ID == norm {
			return p, nil
		}
	}
	return PackageInfo{ID: norm, Title: title}, nil
}

// Step4Slot is one work-package slot in Step 4: the notes the user wrote for
// that package and the package content itself. Filling a slot's output is what
// creates the matching Step 5 work package.
type Step4Slot struct {
	ID         string             `json:"id"`
	Title      string             `json:"title"`
	Input      string             `json:"input"`
	Output     string             `json:"output"`
	HasPackage bool               `json:"hasPackage"`
	HasInput   bool               `json:"hasInput"`
	InputHash  string             `json:"inputHash"`
	OutputHash string             `json:"outputHash"`
	Structured *structured.Result `json:"structured,omitempty"`
}

// ListStep4Slots returns the work-package slots for Step 4, merging the
// per-package input notes with the package documents. The list always ends
// with at least one empty slot so the interface can offer a place to add the
// next package: a project with N filled slots shows N+1.
func (s *Service) ListStep4Slots(project string) ([]Step4Slot, error) {
	if _, err := s.ProjectPath(project); err != nil {
		return nil, err
	}
	pkgNames, err := s.ListMarkdown("projects/" + project + "/packages")
	if err != nil {
		return nil, err
	}
	inputNames, err := s.ListMarkdown("projects/" + project + "/inputs/step-04")
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, n := range pkgNames {
		seen[strings.ToUpper(n)] = true
	}
	for _, n := range inputNames {
		seen[strings.ToUpper(n)] = true
	}
	var out []Step4Slot
	for raw := range seen {
		norm, err := NormalizePackageID(raw)
		if err != nil {
			continue
		}
		slot := Step4Slot{ID: norm}
		if doc, err := s.ReadDoc(KindPackage, "", norm, project); err == nil && doc.Exists {
			slot.Output = doc.Content
			slot.OutputHash = doc.Hash
			slot.Structured = s.InspectDocument(project, doc)
			slot.HasPackage = true
			slot.Title = packageTitleFromContent(doc.Content, norm)
		}
		if doc, err := s.ReadDoc(KindStep4Input, "step-04", norm, project); err == nil {
			slot.Input = doc.Content
			slot.InputHash = doc.Hash
			slot.HasInput = strings.TrimSpace(doc.Content) != ""
		}
		out = append(out, slot)
	}
	sortStep4Slots(out)
	// Always offer one more slot than have been filled in, so there is always
	// somewhere to type the next package. A slot counts as filled once it holds
	// anything — notes or a package — so saving a package also opens the next
	// slot.
	filled := 0
	for _, sl := range out {
		if sl.HasInput || sl.HasPackage {
			filled++
		}
	}
	if len(out) < filled+1 {
		max := 0
		for _, slot := range out {
			if n := packageNumber(slot.ID); n > max {
				max = n
			}
		}
		out = append(out, Step4Slot{ID: fmt.Sprintf("P%02d", max+1)})
	}

	return out, nil
}

func sortStep4Slots(slots []Step4Slot) {
	sort.SliceStable(slots, func(i, j int) bool {
		return packageNumber(slots[i].ID) < packageNumber(slots[j].ID)
	})
}
