package workspace

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Template is a prompt template: the editable Markdown that becomes the
// instruction you hand to an AI chatbot.
type Template struct {
	Stage       string    `json:"stage"`
	Scope       string    `json:"scope"` // "global" or "local"
	Path        string    `json:"path"`
	Exists      bool      `json:"exists"`
	Content     string    `json:"content"`
	Body        string    `json:"body"`
	Hash        string    `json:"hash"`
	ModTime     time.Time `json:"modTime"`
	Title       string    `json:"title"`
	Modified    bool      `json:"modifiedFromGlobal"`
	IsStarter   bool      `json:"isStarter"`
	ActiveScope string    `json:"activeScope"`
	HasOverride bool      `json:"hasOverride"`
}

// AssemblySource records one document that fed the assembled prompt.
type AssemblySource struct {
	Token  string `json:"token"`
	Label  string `json:"label"`
	Path   string `json:"path"`
	Exists bool   `json:"exists"`
	Chars  int    `json:"chars"`
	Hash   string `json:"hash,omitempty"`
	Active bool   `json:"active"`
}

// AssemblyWarning explains a missing or suspicious input.
type AssemblyWarning struct {
	Token    string `json:"token"`
	Label    string `json:"label"`
	Path     string `json:"path"`
	Reason   string `json:"reason"`
	Severity string `json:"severity"` // "missing", "empty" or "note"
	Required bool   `json:"required"` // true when the stage cannot work without it
	Hint     string `json:"hint"`
}

// AssembledPrompt is the final prompt for a stage or a package.
type AssembledPrompt struct {
	Stage           string            `json:"stage"`
	StageNum        string            `json:"stageNum"`
	Package         string            `json:"package,omitempty"`
	Project         string            `json:"project"`
	ProjectName     string            `json:"projectName"`
	Content         string            `json:"content"`
	TemplateScope   string            `json:"templateScope"`
	TemplatePath    string            `json:"templatePath"`
	TemplateHash    string            `json:"templateHash"`
	Sources         []AssemblySource  `json:"sources"`
	Warnings        []AssemblyWarning `json:"warnings"`
	SnapshotPath    string            `json:"snapshotPath"`
	HasSnapshot     bool              `json:"hasSnapshot"`
	SnapshotSaved   time.Time         `json:"snapshotSaved,omitempty"`
	SnapshotDiffers bool              `json:"snapshotDiffers"`
	AssembledAt     time.Time         `json:"assembledAt"`
	CharCount       int               `json:"charCount"`
}

// frontMatterRe matches a leading YAML front matter block.
var frontMatterRe = regexp.MustCompile(`(?s)\A---\r?\n(.*?)\r?\n---\r?\n?`)

// SplitFrontMatter separates a YAML front matter header from the body.
func SplitFrontMatter(content string) (meta string, body string) {
	m := frontMatterRe.FindStringSubmatch(content)
	if m == nil {
		return "", content
	}
	return m[1], content[len(m[0]):]
}

func frontMatterValue(meta, key string) string {
	for _, line := range strings.Split(meta, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, key+":") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, key+":"))
		}
	}
	return ""
}

// tokenRe matches {{token}} placeholders in a template body.
var tokenRe = regexp.MustCompile(`\{\{\s*([a-z0-9_]+)\s*\}\}`)

// KnownTokens lists every placeholder BuildFlow substitutes, for documentation
// and for the "unknown token" warning.
var KnownTokens = []string{
	"project_name", "project_description", "project_id", "stage", "stage_num",
	"stage_title", "generated_at", "today", "user_notes", "missing_sources",
	"input", "system_context", "package_id", "package_title", "package_content",
	"package_input", "output_step_01", "output_step_02", "output_step_02_5",
	"output_step_03", "output_step_04", "previous_output",
}

var knownTokenSet = func() map[string]bool {
	m := make(map[string]bool, len(KnownTokens))
	for _, t := range KnownTokens {
		m[t] = true
	}
	return m
}()

// GetTemplate loads a template. For the local scope it falls back to the
// global template and reports which one is active.
func (s *Service) GetTemplate(stage, scope, project string) (Template, error) {
	if !IsStage(stage) {
		return Template{}, fmt.Errorf("unknown stage %q", stage)
	}
	if scope != "global" && scope != "local" {
		scope = "local"
	}

	read := func(kind, scopeName string) Template {
		t := Template{Stage: stage, Scope: scopeName}
		rel, err := Resolve(kind, stage, "", project)
		if err != nil {
			return t
		}
		abs, err := s.absolute(rel)
		if err != nil {
			return t
		}
		t.Path = filepath.ToSlash(rel)
		if info, err := os.Stat(abs); err != nil || info.IsDir() {
			return t
		}
		raw, err := os.ReadFile(abs)
		if err != nil {
			return t
		}
		t.Exists = true
		t.Content = string(raw)
		t.Hash = hashBytes(raw)
		meta, body := SplitFrontMatter(t.Content)
		t.Body = strings.TrimLeft(body, "\n")
		t.Title = frontMatterValue(meta, "title")
		if info, err := os.Stat(abs); err == nil {
			t.ModTime = info.ModTime().UTC()
		}
		starter := StarterTemplate(stage)
		if starter != "" {
			t.IsStarter = normalizeForCompare(t.Body) == normalizeForCompare(stripFrontMatter(starter))
		}
		return t
	}

	global := read(KindTemplateGlobal, "global")
	local := read(KindTemplateLocal, "local")
	local.HasOverride = local.Exists

	active := local
	if !local.Exists || local.Body == "" {
		active = global
	}
	active.ActiveScope = "global"
	if local.Exists && local.Body != "" {
		active.ActiveScope = "local"
	}
	if !active.Exists {
		// Fall back to the embedded starter so a fresh workspace still works.
		starter := StarterTemplate(stage)
		if starter != "" {
			meta, body := SplitFrontMatter(starter)
			active = Template{
				Stage: stage, Scope: active.Scope, Path: "(built-in starter)",
				Exists: false, Content: starter, Body: strings.TrimLeft(body, "\n"),
				Title: frontMatterValue(meta, "title"), ActiveScope: "builtin", IsStarter: true,
			}
		}
	}
	if scope == "global" {
		active = global
		if !active.Exists {
			if starter := StarterTemplate(stage); starter != "" {
				meta, body := SplitFrontMatter(starter)
				active = Template{Stage: stage, Scope: "global", Path: "(built-in starter)",
					Content: starter, Body: strings.TrimLeft(body, "\n"),
					Title: frontMatterValue(meta, "title"), ActiveScope: "builtin", IsStarter: true}
			}
		}
	}
	if active.Scope == "local" {
		active.Modified = local.Exists && global.Exists &&
			normalizeForCompare(local.Body) != normalizeForCompare(global.Body)
	}
	return active, nil
}

func normalizeForCompare(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.TrimSpace(s)
}

func stripFrontMatter(content string) string {
	_, body := SplitFrontMatter(content)
	return body
}

// SaveTemplate writes a global or local template with conflict detection.
func (s *Service) SaveTemplate(stage, scope, project, content, baseHash string, force bool) (Template, error) {
	if !IsStage(stage) {
		return Template{}, fmt.Errorf("unknown stage %q", stage)
	}
	kind := KindTemplateGlobal
	if scope == "local" {
		kind = KindTemplateLocal
	} else if scope != "global" {
		return Template{}, fmt.Errorf("template scope must be \"global\" or \"local\"")
	}
	if kind == KindTemplateLocal {
		if err := ValidateSegment("project id", project); err != nil {
			return Template{}, err
		}
	}
	if len([]rune(content)) > 400_000 {
		return Template{}, fmt.Errorf("template is too large (over 400000 characters)")
	}
	if _, err := s.WriteDoc(WriteRequest{
		Kind: kind, Stage: stage, Project: project, Content: content,
		BaseHash: baseHash, Force: force,
	}); err != nil {
		return Template{}, err
	}
	return s.GetTemplate(stage, scope, project)
}

// RevertTemplate removes a local override, falling back to the global
// template. It refuses when the file changed on disk since it was loaded.
func (s *Service) RevertTemplate(stage, project, expectedHash string) (Template, error) {
	rel, err := Resolve(KindTemplateLocal, stage, "", project)
	if err != nil {
		return Template{}, err
	}
	abs, err := s.absolute(rel)
	if err != nil {
		return Template{}, err
	}
	if _, err := os.Stat(abs); err == nil && expectedHash != "" {
		current, err := os.ReadFile(abs)
		if err == nil && hashBytes(current) != expectedHash {
			return Template{}, &ConflictError{
				Path: filepath.ToSlash(rel), Reason: "modified",
				Expected: expectedHash, Actual: hashBytes(current), Content: string(current), Existed: true,
			}
		}
	}
	if err := s.DeleteDoc(KindTemplateLocal, stage, "", project); err != nil {
		return Template{}, err
	}
	return s.GetTemplate(stage, "local", project)
}

// placeholderSource describes one token for the current stage.
type placeholderSource struct {
	token string
	label string
	kind  string
	stage string
	pkg   string
	// prereq marks a document that must be saved before the prompt makes sense.
	prereq bool
	hint   string
}

// sourcesForStage returns the documents that feed a stage's prompt.
func sourcesForStage(stage, pkg string) []placeholderSource {
	switch stage {
	case "step-01":
		return []placeholderSource{
			{token: "input", label: "Step 1 input (detailed use case)", kind: KindInput, stage: stage, hint: "Describe the use case in the Input pane, then save."},
		}
	case "step-02":
		return []placeholderSource{
			{token: "input", label: "Step 2 input (research brief and discussion)", kind: KindInput, stage: stage, hint: "Paste the research brief and your discussion notes."},
			{token: "output_step_01", label: "Step 1 output (research brief)", kind: KindOutput, stage: "step-01", prereq: true, hint: "Save Step 1's output first."},
		}
	case "step-02-5":
		return []placeholderSource{
			{token: "input", label: "Step 2.5 input (technology discussion)", kind: KindInput, stage: stage, hint: "Optional. Add constraints or must-have technologies."},
			{token: "output_step_02", label: "Step 2 output (decision and high-level design)", kind: KindOutput, stage: "step-02", prereq: true, hint: "Save Step 2's output first."},
		}
	case "step-03":
		return []placeholderSource{
			{token: "input", label: "Step 3 input (low-level design discussion)", kind: KindInput, stage: stage, hint: "Optional. Add decisions already taken."},
			{token: "output_step_02", label: "Step 2 output (decision and high-level design)", kind: KindOutput, stage: "step-02", prereq: true, hint: "Save Step 2's output first."},
			{token: "output_step_02_5", label: "Step 2.5 output (stack, cost, system context)", kind: KindOutput, stage: "step-02-5", prereq: true, hint: "Save Step 2.5's output first."},
		}
	case "step-04":
		return []placeholderSource{
			{token: "input", label: "Step 4 input (packaging discussion)", kind: KindInput, stage: stage, hint: "Optional. Add how large each package should be."},
			{token: "output_step_03", label: "Step 3 output (low-level design)", kind: KindOutput, stage: "step-03", prereq: true, hint: "Save Step 3's output first."},
		}
	case "step-05":
		return []placeholderSource{
			{token: "package_content", label: "Work package " + pkg, kind: KindPackage, pkg: pkg, prereq: true, hint: "Create the package first, or import packages from Step 4."},
			{token: "system_context", label: "Reusable system context from Step 2.5", kind: KindSystemContext, stage: "step-02-5", prereq: true, hint: "Save Step 2.5's output, including the Reusable System Context section."},
			{token: "package_input", label: "Step 5 input notes for " + pkg, kind: KindPackageInput, stage: "step-05", pkg: pkg, hint: "Optional. Add per-package notes for this coding session."},
		}
	}
	return nil
}

// systemContext extracts the Reusable System Context section from the Step 2.5
// output when it is clearly delimited, otherwise the whole document is used.
var systemContextHeading = regexp.MustCompile(`(?im)^#{1,6}\s*(?:reusable\s+system\s+context|system\s+context)\b`)

// ExtractSystemContext returns the reusable system context section of the
// Step 2.5 output, or the empty string when there is no such section.
func ExtractSystemContext(step25 string) string {
	loc := systemContextHeading.FindStringIndex(step25)
	if loc == nil {
		return ""
	}
	rest := step25[loc[1]:]
	// Stop at the next heading of the same or higher level.
	startLevel := headingLevel(step25[loc[0]:])
	end := len(rest)
	for i, line := range strings.Split(rest, "\n") {
		lvl := headingLevel(line)
		if lvl > 0 && startLevel > 0 && lvl <= startLevel && strings.TrimSpace(line) != "" {
			abs := offsetOfLine(rest, i)
			if abs > 0 {
				end = abs
			}
			break
		}
	}
	return strings.TrimSpace(step25[loc[0] : loc[1]+end])
}

func headingLevel(line string) int {
	t := strings.TrimSpace(line)
	if !strings.HasPrefix(t, "#") {
		return 0
	}
	n := 0
	for n < len(t) && t[n] == '#' {
		n++
	}
	if n > 6 || (n < len(t) && t[n] != ' ') {
		return 0
	}
	return n
}

func offsetOfLine(text string, lineIndex int) int {
	if lineIndex <= 0 {
		return 0
	}
	count := 0
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			count++
			if count == lineIndex {
				return i + 1
			}
		}
	}
	return len(text)
}

// AssemblePrompt builds the prompt for a stage or a package.
func (s *Service) AssemblePrompt(project, stage, pkg string) (AssembledPrompt, error) {
	if !IsStage(stage) {
		return AssembledPrompt{}, fmt.Errorf("unknown stage %q", stage)
	}
	if stage == "step-05" {
		if err := ValidatePackageID(pkg); err != nil {
			return AssembledPrompt{}, err
		}
	}
	proj, err := s.GetProject(project)
	if err != nil {
		return AssembledPrompt{}, err
	}
	st, _ := StageByID(stage)

	tmpl, err := s.GetTemplate(stage, "local", project)
	if err != nil {
		return AssembledPrompt{}, err
	}

	values := map[string]string{
		"project_name":        proj.Name,
		"project_id":          proj.ID,
		"project_description": proj.Description,
		"stage":               "Step " + st.Num,
		"stage_num":           st.Num,
		"stage_title":         st.Title,
		"today":               time.Now().Format("2006-01-02"),
		"generated_at":        time.Now().Format("2006-01-02 15:04:05"),
		"package_id":          pkg,
	}
	if pkg != "" {
		pi := findPackage(proj.Packages, pkg)
		title := pi.Title
		if title == "" {
			title = "Work package " + pkg
		}
		values["package_title"] = title
	} else {
		values["package_title"] = ""
		values["package_id"] = ""
	}

	var sources []AssemblySource
	var warnings []AssemblyWarning
	missingLines := []string{}

	load := func(src placeholderSource) string {
		doc, err := s.ReadDoc(src.kind, src.stage, src.pkg, project)
		rel, _ := Resolve(src.kind, src.stage, src.pkg, project)
		path := filepath.ToSlash(rel)
		if err != nil {
			warnings = append(warnings, AssemblyWarning{
				Token: src.token, Label: src.label, Path: path,
				Reason: "unreadable: " + err.Error(), Severity: "missing", Required: src.prereq, Hint: src.hint,
			})
			if src.prereq {
				missingLines = append(missingLines,
					fmt.Sprintf("- **%s** - %s (not saved yet). %s", src.label, path, src.hint))
			}
			sources = append(sources, AssemblySource{Token: src.token, Label: src.label, Path: path, Active: true})
			return ""
		}
		body := doc.Content
		// The input tokens carry the user's notes. Any auto-copied block from
		// the previous step's output is context, not notes, so it is stripped
		// here: the previous output already reaches the prompt through the
		// {{output_step_XX}} tokens, and the "no notes yet" warning should
		// still fire when the user has written nothing of their own.
		if src.token == "input" || src.token == "package_input" {
			body = UserNotes(doc.Content)
		}
		if src.token == "system_context" {
			section := ExtractSystemContext(doc.Content)
			if section != "" {
				body = section
			} else {
				// No clearly delimited section: the whole document is used, but
				// say so, because a Step 5 prompt built on an unmarked document
				// tends to drift from the rest of the project.
				warnings = append(warnings, AssemblyWarning{
					Token: src.token, Label: src.label, Path: path,
					Reason:   "no \"## Reusable System Context\" heading was found, so the whole Step 2.5 output was used",
					Severity: "note", Required: true,
					Hint: "Add a \"## Reusable System Context\" heading to the Step 2.5 output to control exactly what Step 5 receives.",
				})
			}
		}
		if !doc.Exists {
			sev, reason := "missing", "not saved yet"
			if !src.prereq {
				reason = "empty (optional)"
			}
			warnings = append(warnings, AssemblyWarning{
				Token: src.token, Label: src.label, Path: path,
				Reason: reason, Severity: sev, Required: src.prereq, Hint: src.hint,
			})
			if src.prereq {
				missingLines = append(missingLines,
					fmt.Sprintf("- **%s** - %s (not saved yet). %s", src.label, path, src.hint))
			}
			sources = append(sources, AssemblySource{Token: src.token, Label: src.label, Path: path, Exists: false, Active: true})
			return ""
		}
		if strings.TrimSpace(body) == "" {
			warnings = append(warnings, AssemblyWarning{
				Token: src.token, Label: src.label, Path: path,
				Reason: "file is empty", Severity: "empty", Required: src.prereq, Hint: src.hint,
			})
			if src.prereq {
				missingLines = append(missingLines,
					fmt.Sprintf("- **%s** - %s (saved but empty). %s", src.label, path, src.hint))
			}
			sources = append(sources, AssemblySource{Token: src.token, Label: src.label, Path: path, Exists: true, Active: true})
			return ""
		}
		sources = append(sources, AssemblySource{
			Token: src.token, Label: src.label, Path: path, Exists: true,
			Chars: len([]rune(body)), Hash: doc.Hash, Active: true,
		})
		return body
	}

	for _, src := range sourcesForStage(stage, pkg) {
		values[src.token] = load(src)
	}

	// {{previous_output}} is the previous step's output, placed at the bottom of
	// the assembled prompt so the whole thing can be copied to an LLM chatbot in
	// one go. For Step 5 the "previous" output is the package itself, which is
	// what the package's Step 5 input was seeded from.
	if stage == "step-05" {
		// For a package the "previous output" is the package itself. Strip the
		// front matter: it is metadata, not something an LLM needs to read.
		values["previous_output"] = StripFrontMatter(values["package_content"])
	} else if prev := PreviousStageID(stage); prev != "" {
		if doc, err := s.ReadDoc(KindOutput, prev, "", project); err == nil && doc.Exists {
			values["previous_output"] = doc.Content
		} else {
			values["previous_output"] = ""
		}
	} else {
		values["previous_output"] = ""
	}

	// Extra saved outputs are available to every stage so a template can pull
	// in a document the default wiring does not need.
	for _, id := range []string{"step-01", "step-02", "step-02-5", "step-03", "step-04"} {
		token := "output_" + strings.ReplaceAll(id, "-", "_")
		if _, taken := values[token]; taken {
			continue
		}
		doc, err := s.ReadDoc(KindOutput, id, "", project)
		rel, _ := Resolve(KindOutput, id, "", project)
		if err == nil && doc.Exists && strings.TrimSpace(doc.Content) != "" {
			values[token] = doc.Content
			sources = append(sources, AssemblySource{
				Token: token, Label: "Saved output " + id, Path: filepath.ToSlash(rel),
				Exists: true, Chars: len([]rune(doc.Content)), Hash: doc.Hash, Active: false,
			})
		} else {
			values[token] = ""
		}
	}

	if len(missingLines) == 0 {
		values["missing_sources"] = "_All prerequisite documents were found._"
	} else {
		values["missing_sources"] = strings.Join(missingLines, "\n")
	}

	body := tmpl.Body
	if body == "" {
		body = "Write a prompt for " + st.Title + "."
	}

	assembled := substitute(body, values, func(token string) {
		warnings = append(warnings, AssemblyWarning{
			Token: token, Label: token, Reason: "unknown placeholder {{" + token + "}}",
			Severity: "note", Hint: "This placeholder is not one of BuildFlow's known tokens, so it was left as written.",
		})
	})
	assembled = strings.TrimLeft(assembled, "\n")
	if !strings.HasSuffix(assembled, "\n") {
		assembled += "\n"
	}

	kind := KindSnapshot
	if stage == "step-05" {
		kind = KindPackageSnapshot
	}
	rel, _ := Resolve(kind, stage, pkg, project)
	out := AssembledPrompt{
		Stage:         stage,
		StageNum:      st.Num,
		Package:       pkg,
		Project:       project,
		ProjectName:   proj.Name,
		Content:       assembled,
		TemplateScope: tmpl.ActiveScope,
		TemplatePath:  tmpl.Path,
		TemplateHash:  tmpl.Hash,
		Sources:       sources,
		Warnings:      warnings,
		SnapshotPath:  filepath.ToSlash(rel),
		AssembledAt:   time.Now().UTC(),
		CharCount:     len([]rune(assembled)),
	}
	if snap, err := s.ReadDoc(kind, stage, pkg, project); err == nil && snap.Exists {
		out.HasSnapshot = true
		out.SnapshotSaved = snap.ModTime
		stored := stripSnapshotHeader(snap.Content)
		out.SnapshotDiffers = normalizeForCompare(stored) != normalizeForCompare(assembled)
	}
	return out, nil
}

func findPackage(list []PackageInfo, id string) PackageInfo {
	for _, p := range list {
		if p.ID == id {
			return p
		}
	}
	return PackageInfo{}
}

// substitute replaces {{token}} placeholders. Unknown tokens are reported to
// the onUnknown callback and left untouched.
func substitute(body string, values map[string]string, onUnknown func(string)) string {
	return tokenRe.ReplaceAllStringFunc(body, func(match string) string {
		name := tokenRe.FindStringSubmatch(match)[1]
		if v, ok := values[name]; ok {
			return v
		}
		if onUnknown != nil {
			onUnknown(name)
		}
		return match
	})
}

// SaveSnapshot stores the exact assembled prompt used for a stage or package.
// The text is stored verbatim (behind a small provenance header) so the record
// always matches what was copied.
func (s *Service) SaveSnapshot(project, stage, pkg, content string) (AssembledPrompt, error) {
	if !IsStage(stage) {
		return AssembledPrompt{}, fmt.Errorf("unknown stage %q", stage)
	}
	kind := KindSnapshot
	if stage == "step-05" {
		kind = KindPackageSnapshot
		if err := ValidatePackageID(pkg); err != nil {
			return AssembledPrompt{}, err
		}
	}
	if strings.TrimSpace(content) == "" {
		return AssembledPrompt{}, fmt.Errorf("refusing to save an empty prompt snapshot")
	}
	st, _ := StageByID(stage)
	label := "Step " + st.Num
	if pkg != "" {
		label = "Step 5 / " + pkg
	}
	header := "---\n" +
		"stage: " + stage + "\n" +
		"label: " + yamlQuote(label) + "\n" +
		"package: " + yamlQuote(pkg) + "\n" +
		"project: " + project + "\n" +
		"saved: " + time.Now().UTC().Format(time.RFC3339) + "\n" +
		"note: \"Exact prompt text used. Edit above this line only with care.\"\n" +
		"---\n\n"
	rel, err := Resolve(kind, stage, pkg, project)
	if err != nil {
		return AssembledPrompt{}, err
	}
	existing, err := s.ReadDoc(kind, stage, pkg, project)
	if err != nil {
		return AssembledPrompt{}, err
	}
	if _, err := s.WriteDoc(WriteRequest{
		Kind: kind, Stage: stage, Package: pkg, Project: project,
		Content: header + content, BaseHash: existing.Hash, Force: true,
	}); err != nil {
		return AssembledPrompt{}, err
	}
	_ = rel
	return s.AssemblePrompt(project, stage, pkg)
}

func stripSnapshotHeader(content string) string {
	_, body := SplitFrontMatter(content)
	return body
}

// HasSystemContext reports whether a project's Step 2.5 output actually
// contains a delimited Reusable System Context section, which is what Step 5
// assembles from. A Step 2.5 output with no such heading exists but is not
// ready to hand to a coding session.
func (s *Service) HasSystemContext(project string) bool {
	doc, err := s.ReadDoc(KindSystemContext, SystemContextStage, "", project)
	if err != nil || !doc.Exists {
		return false
	}
	return ExtractSystemContext(doc.Content) != ""
}

// LoadSnapshot returns the stored prompt snapshot for a stage or package.
func (s *Service) LoadSnapshot(project, stage, pkg string) (Doc, error) {
	kind := KindSnapshot
	if stage == "step-05" {
		kind = KindPackageSnapshot
	}
	doc, err := s.ReadDoc(kind, stage, pkg, project)
	if err != nil {
		return doc, err
	}
	doc.Content = strings.TrimLeft(stripSnapshotHeader(doc.Content), "\n")
	return doc, nil
}

// GenerateToken creates a random 256 bit token for the local REST API.
func GenerateToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate API token: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
