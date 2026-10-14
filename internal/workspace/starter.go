package workspace

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed starter/*.md
var starterFS embed.FS

// StarterTemplate returns the built-in starter prompt for a stage, or "" when
// the stage is unknown. The templates ship inside the executable so a fresh
// workspace is immediately usable with no downloads.
func StarterTemplate(stage string) string {
	if !IsStage(stage) {
		return ""
	}
	raw, err := starterFS.ReadFile("starter/" + stage + ".md")
	if err != nil {
		return ""
	}
	return string(raw)
}

// StarterTemplateIDs lists the stage ids that have a built-in starter.
func StarterTemplateIDs() []string {
	entries, err := starterFS.ReadDir("starter")
	if err != nil {
		return StageIDs()
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		out = append(out, strings.TrimSuffix(e.Name(), ".md"))
	}
	SortPackageIDs(out)
	return out
}

// Scaffolding describes what EnsureWorkspace created or found.
type Scaffolding struct {
	Root               string   `json:"root"`
	Created            []string `json:"created"`
	Existing           []string `json:"existing"`
	GlobalTemplates    []string `json:"globalTemplates"`
	Readme             bool     `json:"readmeCreated"`
	WorkspaceGitignore bool     `json:"workspaceGitignoreCreated"`
}

// EnsureWorkspace creates the standard BuildFlow folder structure inside a
// validated workspace root and installs the starter prompt templates.
// Existing files are never overwritten.
func (s *Service) EnsureWorkspace(root string) (Scaffolding, error) {
	clean, err := ValidateWorkspacePath(root)
	if err != nil {
		return Scaffolding{}, err
	}
	if err := s.SetRoot(clean); err != nil {
		return Scaffolding{}, err
	}
	res := Scaffolding{Root: clean}

	folders := []string{
		"global-prompts",
		"projects",
	}
	for _, f := range folders {
		abs := filepath.Join(clean, f)
		if info, err := os.Stat(abs); err == nil && info.IsDir() {
			res.Existing = append(res.Existing, f)
			continue
		}
		if err := os.MkdirAll(abs, 0o755); err != nil {
			return res, fmt.Errorf("cannot create %q in the workspace: %w", f, err)
		}
		res.Created = append(res.Created, f)
	}

	// Starter global prompt templates, only where nothing exists yet.
	for _, stage := range StageIDs() {
		content := StarterTemplate(stage)
		if content == "" {
			continue
		}
		abs := filepath.Join(clean, "global-prompts", stage+".md")
		rel := "global-prompts/" + stage + ".md"
		if info, err := os.Stat(abs); err == nil {
			if info.IsDir() {
				return res, fmt.Errorf("%s is a folder, expected a Markdown file", rel)
			}
			res.Existing = append(res.Existing, rel)
			res.GlobalTemplates = append(res.GlobalTemplates, rel)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return res, err
		}
		if err := WriteFileAtomic(abs, []byte(content)); err != nil {
			return res, fmt.Errorf("cannot install starter template %s: %w", rel, err)
		}
		res.Created = append(res.Created, rel)
		res.GlobalTemplates = append(res.GlobalTemplates, rel)
	}

	// A workspace README so the folder makes sense in Obsidian on its own.
	readme := filepath.Join(clean, "README.md")
	if _, err := os.Stat(readme); os.IsNotExist(err) {
		body := "# BuildFlow workspace\n\n" +
			"This folder is the BuildFlow workspace. Open it as an Obsidian vault, or browse it in any editor.\n\n" +
			"```text\n" +
			"global-prompts/   prompt templates shared by every project\n" +
			"projects/         one folder per project\n" +
			"  <project>/\n" +
			"    project.json   small metadata only\n" +
			"    inputs/        what you paste in for each step\n" +
			"    outputs/       what the AI returned, per step and per package\n" +
			"    packages/      work packages split from Step 4\n" +
			"    prompts/\n" +
			"      local-overrides/  prompt templates for this project only\n" +
			"      snapshots/       the exact prompt used for each step or package\n" +
			"```\n\n" +
			"Everything except `project.json` is plain UTF-8 Markdown, so it reads and edits cleanly in Obsidian.\n"
		if err := WriteFileAtomic(readme, []byte(body)); err != nil {
			return res, fmt.Errorf("cannot write the workspace README: %w", err)
		}
		res.Created = append(res.Created, "README.md")
		res.Readme = true
	} else {
		res.Existing = append(res.Existing, "README.md")
	}

	// Keep Obsidian's own metadata out of version control if the user commits.
	gi := filepath.Join(clean, ".gitignore")
	if _, err := os.Stat(gi); os.IsNotExist(err) {
		body := ".obsidian/workspace.json\n.obsidian/workspace-mobile.json\n.buildflow-*.tmp\n"
		if err := WriteFileAtomic(gi, []byte(body)); err != nil {
			return res, err
		}
		res.Created = append(res.Created, ".gitignore")
		res.WorkspaceGitignore = true
	}
	return res, nil
}
