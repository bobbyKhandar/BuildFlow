// Package workspace owns every filesystem operation BuildFlow performs. The
// package is deliberately strict: callers never pass raw filesystem paths to
// it. They pass a document kind, a stage id and an optional package id, and the
// package resolves those to paths that are guaranteed to stay inside the
// chosen workspace folder.
package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// Document kinds understood by the API. The frontend never sends a path.
const (
	KindInput           = "input"            // projects/<p>/inputs/step-XX.md
	KindStep4Input      = "step4-input"      // projects/<p>/inputs/step-04/PNN.md
	KindOutput          = "output"           // projects/<p>/outputs/step-XX.md
	KindSystemContext   = "system-context"   // the Reusable System Context section of outputs/step-02-5.md
	KindPackage         = "package"          // projects/<p>/packages/PNN.md
	KindPackageInput    = "package-input"    // projects/<p>/inputs/step-05/PNN.md
	KindPackageOutput   = "package-output"   // projects/<p>/outputs/step-05/PNN.md
	KindSnapshot        = "snapshot"         // projects/<p>/prompts/snapshots/step-XX.md
	KindPackageSnapshot = "package-snapshot" // projects/<p>/prompts/snapshots/step-05-PNN.md
	KindTemplateGlobal  = "template-global"  // global-prompts/step-XX.md
	KindTemplateLocal   = "template-local"   // projects/<p>/prompts/local-overrides/step-XX.md
)

// SystemContextStage is the step that owns the reusable system context. The
// context is a section of that step's output, not a file of its own, so a
// caller cannot ask for it from any other stage.
const SystemContextStage = "step-02-5"

var allKinds = map[string]bool{
	KindInput: true, KindStep4Input: true, KindOutput: true, KindSystemContext: true,
	KindPackage: true, KindPackageInput: true, KindPackageOutput: true,
	KindSnapshot: true, KindPackageSnapshot: true,
	KindTemplateGlobal: true, KindTemplateLocal: true,
}

var (
	// PackageIDPattern is the only accepted package identifier form: P01, P02 ...
	PackageIDPattern = regexp.MustCompile(`^P[0-9]{1,3}$`)
	// packageDisplayPattern matches a zero padded 2-digit package number.
	packageDisplayPattern = regexp.MustCompile(`^P([0-9]{1,3})$`)
)

// windowsReservedNames are device names that cannot be used as file or folder
// names on Windows, even with an extension.
var windowsReservedNames = map[string]bool{
	"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"COM1": true, "COM2": true, "COM3": true, "COM4": true, "COM5": true,
	"COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true, "LPT5": true,
	"LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
}

// Stage describes one step of the manual AI workflow.
type Stage struct {
	ID       string `json:"id"`
	Num      string `json:"num"`
	Title    string `json:"title"`
	Question string `json:"question"`
}

// Stages is the canonical workflow order. Step 5 is handled per package.
var Stages = []Stage{
	{ID: "step-01", Num: "1", Title: "Research brief", Question: "Detailed use case to research brief"},
	{ID: "step-02", Num: "2", Title: "Decision and high-level design", Question: "Research brief and discussion to final decision and high-level design"},
	{ID: "step-02-5", Num: "2.5", Title: "Stack, cost and system context", Question: "High-level design to tech stack, packages, cost and reusable system context"},
	{ID: "step-03", Num: "3", Title: "Low-level design", Question: "High-level design and technology decision to low-level design"},
	{ID: "step-04", Num: "4", Title: "Work packages", Question: "Low-level design to multiple work packages"},
	{ID: "step-05", Num: "5", Title: "Package coding prompt", Question: "One selected package plus reusable system context to one coding prompt"},
}

// StageIDs returns every stage id, step-05 included.
func StageIDs() []string {
	out := make([]string, 0, len(Stages))
	for _, s := range Stages {
		out = append(out, s.ID)
	}
	return out
}

// IsStage reports whether id is a known workflow stage.
func IsStage(id string) bool {
	for _, s := range Stages {
		if s.ID == id {
			return true
		}
	}
	return false
}

// StageByID returns the stage definition for id.
func StageByID(id string) (Stage, bool) {
	for _, s := range Stages {
		if s.ID == id {
			return s, true
		}
	}
	return Stage{}, false
}

// StageStageIDs returns the stage ids that have a project-level input and
// output document, i.e. every stage except the per-package step 5.
func DocumentStageIDs() []string {
	out := make([]string, 0, len(Stages))
	for _, s := range Stages {
		if s.ID != "step-05" {
			out = append(out, s.ID)
		}
	}
	return out
}

// stageOrder lists the project-level stages in workflow order. Step 5 is
// per-package, so it is not part of this chain.
var stageOrder = []string{"step-01", "step-02", "step-02-5", "step-03", "step-04"}

// NextStageID returns the stage that follows stage in the workflow, or "" when
// stage is last (or is the per-package step 5). It is used to feed one step's
// output into the next step's input.
func NextStageID(stage string) string {
	for i, s := range stageOrder {
		if s == stage && i+1 < len(stageOrder) {
			return stageOrder[i+1]
		}
	}
	return ""
}

// PreviousStageID returns the stage that comes before stage in the workflow, or
// "" when stage is first. Step 5 is per-package and has no single predecessor in
// the chain, so it returns "" for step 5 as well.
func PreviousStageID(stage string) string {
	for i, s := range stageOrder {
		if s == stage && i > 0 {
			return stageOrder[i-1]
		}
	}
	return ""
}

var illegalNameChars = regexp.MustCompile(`[<>:"|?*\\/]`)

// ValidateSegment checks a single path component: no separators, no traversal,
// no control characters, no reserved device name, no trailing dot or space.
func ValidateSegment(label, seg string) error {
	seg = strings.TrimSpace(seg)
	if seg == "" {
		return fmt.Errorf("%s must not be empty", label)
	}
	if len(seg) > 96 {
		return fmt.Errorf("%s must be 96 characters or fewer (got %d)", label, len(seg))
	}
	if seg == "." || seg == ".." {
		return fmt.Errorf("%q is not a valid %s", seg, label)
	}
	if illegalNameChars.MatchString(seg) {
		return fmt.Errorf("%s %q must not contain any of < > : \" | ? * or / \\", label, seg)
	}
	for _, r := range seg {
		if unicode.IsControl(r) {
			return fmt.Errorf("%s %q must not contain control characters", label, seg)
		}
	}
	if strings.HasSuffix(seg, ".") || strings.HasSuffix(seg, " ") {
		return fmt.Errorf("%s %q must not end with a dot or a space", label, seg)
	}
	base := seg
	if dot := strings.Index(base, "."); dot > 0 {
		base = base[:dot]
	}
	if windowsReservedNames[strings.ToUpper(base)] {
		return fmt.Errorf("%q is a reserved Windows device name", seg)
	}
	return nil
}

// ValidatePackageID checks a package identifier such as P01.
func ValidatePackageID(id string) error {
	if !PackageIDPattern.MatchString(id) {
		return fmt.Errorf("package id %q is invalid: expected P01 style identifiers such as P01 or P07", id)
	}
	return nil
}

// NormalizePackageID pads a numeric package reference to the canonical form.
func NormalizePackageID(raw string) (string, error) {
	raw = strings.TrimSpace(strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(raw)), "P"))
	if raw == "" {
		return "", fmt.Errorf("package number is empty")
	}
	if len(raw) > 3 {
		return "", fmt.Errorf("package number %q is too large (max 999)", raw)
	}
	for _, r := range raw {
		if r < '0' || r > '9' {
			return "", fmt.Errorf("package number %q is not a number", raw)
		}
	}
	n := 0
	for _, r := range raw {
		n = n*10 + int(r-'0')
	}
	if n < 1 {
		return "", fmt.Errorf("package numbers start at 1")
	}
	return fmt.Sprintf("P%02d", n), nil
}

// SlugifyProjectName turns a free-form display name into a safe folder name.
func SlugifyProjectName(name string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.TrimSpace(name) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(unicode.ToLower(r))
			prevDash = false
		case r == ' ' || r == '-' || r == '_' || r == '.' || r == '(' || r == ')' || r == '&' || r == '+':
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		default:
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	slug := strings.Trim(b.String(), "-.")
	if len(slug) > 64 {
		slug = strings.Trim(slug[:64], "-.")
	}
	return slug
}

// ValidateWorkspacePath checks a user supplied absolute workspace folder.
func ValidateWorkspacePath(input string) (string, error) {
	raw := strings.TrimSpace(input)
	if raw == "" {
		return "", fmt.Errorf("workspace path must not be empty")
	}
	if strings.ContainsRune(raw, 0) {
		return "", fmt.Errorf("workspace path contains an invalid character")
	}
	// Expand a leading ~ so pasted Unix-style paths are not silently mangled.
	if raw == "~" || strings.HasPrefix(raw, "~/") || strings.HasPrefix(raw, `~\`) {
		if home, err := os.UserHomeDir(); err == nil {
			raw = filepath.Join(home, strings.TrimLeft(raw[1:], `/\`))
		}
	}
	// Reject traversal in the *uncleaned* input so "../../etc" can never reach
	// filepath.Clean unnoticed.
	if strings.Contains(raw, "..") {
		return "", fmt.Errorf("workspace path must not contain \"..\"")
	}
	if !filepath.IsAbs(raw) {
		return "", fmt.Errorf("workspace path must be absolute, for example D:\\Obsidian\\BuildFlow-workspace")
	}
	clean := filepath.Clean(raw)
	vol := filepath.VolumeName(clean)
	if vol == "" {
		return "", fmt.Errorf("workspace path %q has no drive or share", clean)
	}
	unc := strings.HasPrefix(vol, `\\`)
	if !unc && len(vol) == 2 && vol[1] != ':' {
		return "", fmt.Errorf("workspace path %q has no drive or share", clean)
	}
	for _, r := range clean {
		if unicode.IsControl(r) {
			return "", fmt.Errorf("workspace path contains an invalid character")
		}
	}
	if unc {
		host := strings.Trim(vol, `\`)
		if host == "" {
			return "", fmt.Errorf("UNC workspace path %q is missing a host name", clean)
		}
	} else {
		drive := strings.ToUpper(vol[:1])
		if drive < "A" || drive > "Z" {
			return "", fmt.Errorf("workspace path %q is not on a valid drive", clean)
		}
	}
	if len(clean) > 4000 {
		return "", fmt.Errorf("workspace path is too long")
	}
	return clean, nil
}

// CheckWorkspaceDir verifies that a validated workspace folder is usable and
// reports whether it already exists.
func CheckWorkspaceDir(path string) (exists bool, writable bool, err error) {
	info, statErr := os.Stat(path)
	if os.IsNotExist(statErr) {
		return false, false, nil
	}
	if statErr != nil {
		return false, false, fmt.Errorf("cannot inspect %q: %w", path, statErr)
	}
	if !info.IsDir() {
		return false, false, fmt.Errorf("%q exists but is a file, not a folder", path)
	}
	probe := filepath.Join(path, ".buildflow-write-test")
	if err := os.WriteFile(probe, []byte("ok"), 0o644); err != nil {
		return true, false, fmt.Errorf("%q is not writable: %w", path, err)
	}
	_ = os.Remove(probe)
	return true, true, nil
}

// Resolve maps a document kind plus identifiers onto a workspace-relative path.
// It never trusts caller supplied path strings: every component is validated
// and the result is a fixed set of well known locations.
func Resolve(kind, stage, pkg, project string) (string, error) {
	if !allKinds[kind] {
		return "", fmt.Errorf("unknown document kind %q", kind)
	}
	var parts []string

	switch kind {
	case KindTemplateGlobal:
		// Global templates live next to projects, so no project id is needed.
		if !IsStage(stage) {
			return "", fmt.Errorf("unknown stage %q", stage)
		}
		parts = []string{"global-prompts", stage + ".md"}

	case KindTemplateLocal, KindInput, KindStep4Input, KindOutput, KindSnapshot:
		if err := ValidateSegment("project id", project); err != nil {
			return "", err
		}
		if !IsStage(stage) {
			return "", fmt.Errorf("unknown stage %q", stage)
		}
		switch kind {
		case KindTemplateLocal:
			parts = []string{"projects", project, "prompts", "local-overrides", stage + ".md"}
		case KindInput:
			parts = []string{"projects", project, "inputs", stage + ".md"}
		case KindStep4Input:
			// Step 4 keeps one input note file per work package, so a slot can
			// hold its own notes without disturbing the others.
			if err := ValidatePackageID(pkg); err != nil {
				return "", err
			}
			parts = []string{"projects", project, "inputs", "step-04", pkg + ".md"}
		case KindOutput:
			parts = []string{"projects", project, "outputs", stage + ".md"}
		case KindSnapshot:
			parts = []string{"projects", project, "prompts", "snapshots", stage + ".md"}
		}

	case KindSystemContext:
		if err := ValidateSegment("project id", project); err != nil {
			return "", err
		}
		// The reusable system context lives inside the Step 2.5 output, under a
		// "## Reusable System Context" heading. Callers may only ask for it at
		// its own stage, so a stray stage value cannot redirect the read.
		if IsStage(stage) && stage != SystemContextStage {
			return "", fmt.Errorf("the reusable system context belongs to %s, not %s", SystemContextStage, stage)
		}
		parts = []string{"projects", project, "outputs", SystemContextStage + ".md"}

	case KindPackage, KindPackageInput, KindPackageOutput, KindPackageSnapshot:
		if err := ValidateSegment("project id", project); err != nil {
			return "", err
		}
		if err := ValidatePackageID(pkg); err != nil {
			return "", err
		}
		switch kind {
		case KindPackage:
			parts = []string{"projects", project, "packages", pkg + ".md"}
		case KindPackageInput:
			parts = []string{"projects", project, "inputs", "step-05", pkg + ".md"}
		case KindPackageOutput:
			parts = []string{"projects", project, "outputs", "step-05", pkg + ".md"}
		case KindPackageSnapshot:
			parts = []string{"projects", project, "prompts", "snapshots", "step-05-" + pkg + ".md"}
		}

	default:
		return "", fmt.Errorf("unknown document kind %q", kind)
	}

	return filepath.Join(parts...), nil
}

// EnsureInsideRoot is the last line of defence against path traversal. It
// resolves symlinks where possible and confirms that target is root or lives
// below it.
func EnsureInsideRoot(root, target string) error {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	absRoot = filepath.Clean(absRoot)
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	absTarget = filepath.Clean(absTarget)
	rel, err := filepath.Rel(absRoot, absTarget)
	if err != nil {
		return fmt.Errorf("path %q is outside the workspace", target)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path %q is outside the workspace", target)
	}
	if filepath.IsAbs(rel) {
		return fmt.Errorf("path %q is outside the workspace", target)
	}
	return nil
}

// EnsureNoReparsePoints walks root down to target and rejects any junction or
// symbolic link, so a link planted inside the workspace cannot be used to read
// or overwrite files elsewhere on the machine.
func EnsureNoReparsePoints(root, target string) error {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(absRoot, absTarget)
	if err != nil {
		return fmt.Errorf("path %q is outside the workspace", target)
	}
	current := absRoot
	segments := []string{}
	if rel != "." {
		segments = strings.Split(rel, string(filepath.Separator))
	}
	// Probe the root itself first.
	info, err := os.Lstat(absRoot)
	if err != nil {
		return fmt.Errorf("cannot inspect workspace root %q: %w", absRoot, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("workspace root %q is a symbolic link; choose the real folder", absRoot)
	}
	for _, seg := range segments {
		if seg == "" || seg == "." {
			continue
		}
		current = filepath.Join(current, seg)
		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				return nil // does not exist yet: nothing can redirect it
			}
			return fmt.Errorf("cannot inspect %q: %w", current, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%q is a symbolic link or junction; BuildFlow refuses to follow links inside the workspace", current)
		}
	}
	return nil
}

// SortPackageIDs orders package ids numerically: P01, P02, P10.
func SortPackageIDs(ids []string) {
	sort.Slice(ids, func(i, j int) bool { return packageNumber(ids[i]) < packageNumber(ids[j]) })
}

func packageNumber(id string) int {
	m := packageDisplayPattern.FindStringSubmatch(strings.ToUpper(strings.TrimSpace(id)))
	if m == nil {
		return 0
	}
	n := 0
	for _, r := range m[1] {
		n = n*10 + int(r-'0')
	}
	return n
}
