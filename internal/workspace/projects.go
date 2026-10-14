package workspace

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ProjectMeta is the small JSON document stored at
// projects/<id>/project.json. Everything else lives in Markdown.
type ProjectMeta struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	Format      int       `json:"formatVersion"`
}

const projectFormatVersion = 1

// PackageInfo summarises one work package for the Step 5 list.
type PackageInfo struct {
	ID             string    `json:"id"`
	Title          string    `json:"title"`
	HasPackageDoc  bool      `json:"hasPackageDoc"`
	HasInput       bool      `json:"hasInput"`
	HasOutput      bool      `json:"hasOutput"`
	HasSnapshot    bool      `json:"hasSnapshot"`
	OutputModified time.Time `json:"outputModified,omitempty"`
	PackageDocHash string    `json:"packageDocHash,omitempty"`
	OutputHash     string    `json:"outputHash,omitempty"`
	InputHash      string    `json:"inputHash,omitempty"`
	SnapshotHash   string    `json:"snapshotHash,omitempty"`
	Status         string    `json:"status"`
}

// Project is the API view of one project folder.
type Project struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Path        string        `json:"path"`
	CreatedAt   time.Time     `json:"createdAt"`
	UpdatedAt   time.Time     `json:"updatedAt"`
	Packages    []PackageInfo `json:"packages"`
	Steps       []StepStatus  `json:"steps"`
}

// StepStatus reports whether a stage has content saved.
type StepStatus struct {
	Stage          string    `json:"stage"`
	Num            string    `json:"num"`
	Title          string    `json:"title"`
	HasInput       bool      `json:"hasInput"`
	HasOutput      bool      `json:"hasOutput"`
	HasSystem      bool      `json:"hasSystemContext"`
	HasSnapshot    bool      `json:"hasSnapshot"`
	HasLocal       bool      `json:"hasLocalOverride"`
	OutputModified time.Time `json:"outputModified,omitempty"`
}

func projectsDir(root string) string { return filepath.Join(root, "projects") }

// ListProjects returns every direct child folder of projects/, sorted by name.
func (s *Service) ListProjects() ([]Project, error) {
	root := s.Root()
	if root == "" {
		return nil, ErrNoWorkspace
	}
	dir := projectsDir(root)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Project{}, nil
		}
		return nil, fmt.Errorf("cannot list projects: %w", err)
	}
	var out []Project
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if err := ValidateSegment("project id", e.Name()); err != nil {
			continue // skip anything that would not survive a round trip
		}
		p, err := s.describe(e.Name())
		if err != nil {
			continue
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

func (s *Service) describe(id string) (Project, error) {
	dir, err := s.ProjectPath(id)
	if err != nil {
		return Project{}, err
	}
	meta, err := s.readMeta(dir, id)
	if err != nil {
		return Project{}, err
	}
	p := Project{
		ID:          id,
		Name:        meta.Name,
		Description: meta.Description,
		Path:        dir,
		CreatedAt:   meta.CreatedAt,
		UpdatedAt:   meta.UpdatedAt,
		Packages:    []PackageInfo{},
		Steps:       []StepStatus{},
	}
	for _, st := range Stages {
		ss := StepStatus{Stage: st.ID, Num: st.Num, Title: st.Title}
		ss.HasInput = s.Exists(KindInput, st.ID, "", id)
		ss.HasOutput = s.Exists(KindOutput, st.ID, "", id)
		ss.HasSnapshot = s.Exists(KindSnapshot, st.ID, "", id)
		ss.HasLocal = s.Exists(KindTemplateLocal, st.ID, "", id)
		if st.ID == "step-02-5" {
			ss.HasSystem = s.HasSystemContext(id)
		}
		if ss.HasOutput {
			ss.OutputModified = s.StatTimes(KindOutput, st.ID, "", id)
		}
		p.Steps = append(p.Steps, ss)
	}
	pkgs, err := s.ListPackages(id)
	if err != nil {
		return p, nil // tolerate an unreadable packages folder
	}
	p.Packages = pkgs
	return p, nil
}

func (s *Service) readMeta(dir, id string) (ProjectMeta, error) {
	path := filepath.Join(dir, "project.json")
	raw, err := os.ReadFile(path)
	meta := ProjectMeta{ID: id, Name: id, Format: projectFormatVersion}
	if err != nil {
		if !os.IsNotExist(err) {
			return meta, fmt.Errorf("cannot read project metadata: %w", err)
		}
		// Fall back to the folder name and file timestamps.
		if info, statErr := os.Stat(dir); statErr == nil {
			meta.CreatedAt = info.ModTime().UTC()
			meta.UpdatedAt = info.ModTime().UTC()
		}
		return meta, nil
	}
	var loaded ProjectMeta
	if err := json.Unmarshal(raw, &loaded); err != nil {
		return meta, fmt.Errorf("project.json for %q is not valid JSON; BuildFlow ignores it: %w", id, err)
	}
	if loaded.Name == "" {
		loaded.Name = id
	}
	loaded.ID = id
	if loaded.Format == 0 {
		loaded.Format = projectFormatVersion
	}
	return loaded, nil
}

func (s *Service) writeMeta(dir string, meta ProjectMeta) error {
	raw, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return WriteFileAtomic(filepath.Join(dir, "project.json"), raw)
}

// ProjectFolders lists the folders a project must contain, relative to it.
func ProjectFolders() []string {
	return []string{
		"inputs",
		"inputs" + string(filepath.Separator) + "step-05",
		"outputs",
		"outputs" + string(filepath.Separator) + "step-05",
		"packages",
		filepath.Join("prompts", "local-overrides"),
		filepath.Join("prompts", "snapshots"),
	}
}

// CreateProject creates a new project folder with the full folder skeleton,
// an empty project.json and starter input notes for every stage.
func (s *Service) CreateProject(name, description string) (Project, error) {
	root := s.Root()
	if root == "" {
		return Project{}, ErrNoWorkspace
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return Project{}, errors.New("project name must not be empty")
	}
	if len(name) > 80 {
		return Project{}, errors.New("project name must be 80 characters or fewer")
	}
	slug := SlugifyProjectName(name)
	if slug == "" {
		slug = "project"
	}
	if err := ValidateSegment("project folder name", slug); err != nil {
		return Project{}, err
	}
	dir := filepath.Join(projectsDir(root), slug)
	if err := EnsureInsideRoot(root, dir); err != nil {
		return Project{}, err
	}
	if _, err := os.Stat(dir); err == nil {
		return Project{}, fmt.Errorf("a project folder named %q already exists", slug)
	}
	now := time.Now().UTC()
	meta := ProjectMeta{
		ID: slug, Name: name, Description: strings.TrimSpace(description),
		CreatedAt: now, UpdatedAt: now, Format: projectFormatVersion,
	}
	if err := s.writeProjectSkeleton(dir, meta); err != nil {
		_ = os.RemoveAll(dir)
		return Project{}, err
	}
	p, err := s.describe(slug)
	if err != nil {
		return p, err
	}
	return p, nil
}

// writeProjectSkeleton creates the folder tree, metadata and starter notes.
// It refuses to clobber an existing project.
func (s *Service) writeProjectSkeleton(dir string, meta ProjectMeta) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("cannot create project folder: %w", err)
	}
	for _, rel := range ProjectFolders() {
		if err := os.MkdirAll(filepath.Join(dir, rel), 0o755); err != nil {
			return fmt.Errorf("cannot create %q: %w", rel, err)
		}
	}
	if err := s.writeMeta(dir, meta); err != nil {
		return fmt.Errorf("cannot write project.json: %w", err)
	}
	// Input files deliberately start absent rather than holding placeholder
	// text. That keeps "does this step have an input?" an honest question
	// ("did the user actually write one?") and means a first save never looks
	// like an unexpected conflict. The editor shows guidance for empty panes.
	readme := "# " + meta.Name + "\n\nBuildFlow project folder. Every file here is plain UTF-8 Markdown.\n\n" +
		"- `inputs/step-01.md` to `step-04.md` - what you paste in for each step\n" +
		"- `outputs/step-01.md` to `step-04.md` - what the AI gave back\n" +
		"- `outputs/step-02-5.md` - stack, cost and the `## Reusable System Context`\n" +
		"  section that every Step 5 coding prompt is built on\n" +
		"- `inputs/step-05/P01.md` - your notes for the P01 coding session\n" +
		"- `outputs/step-05/P01.md` - what the AI delivered for P01\n" +
		"- `packages/P01.md` - work packages split from the Step 4 output\n" +
		"- `prompts/local-overrides/` - prompt templates for this project only\n" +
		"- `prompts/snapshots/` - the exact prompt used for each step or package\n\n" +
		"Edit any of these in Obsidian or in BuildFlow. BuildFlow notices edits made\n" +
		"outside the app and asks before it overwrites anything.\n"
	abs := filepath.Join(dir, "README.md")
	if _, err := os.Stat(abs); err != nil {
		if err := os.WriteFile(abs, []byte(readme), 0o644); err != nil {
			return fmt.Errorf("cannot write README.md: %w", err)
		}
	}
	return nil
}

// SetProjectDescription updates the description in project.json without
// touching the folder name.
func (s *Service) SetProjectDescription(id, description string) error {
	dir, err := s.ProjectPath(id)
	if err != nil {
		return err
	}
	meta, err := s.readMeta(dir, id)
	if err != nil {
		return err
	}
	if meta.Description == strings.TrimSpace(description) {
		return nil
	}
	meta.Description = strings.TrimSpace(description)
	meta.UpdatedAt = time.Now().UTC()
	return s.writeMeta(dir, meta)
}

// GetProject returns a single project description.
func (s *Service) GetProject(id string) (Project, error) {
	if _, err := s.ProjectPath(id); err != nil {
		return Project{}, err
	}
	dir, _ := s.ProjectPath(id)
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return Project{}, fmt.Errorf("project %q does not exist", id)
	}
	return s.describe(id)
}

// RenameProject changes the display name and, when needed, the folder name.
func (s *Service) RenameProject(id, newName string) (Project, error) {
	dir, err := s.ProjectPath(id)
	if err != nil {
		return Project{}, err
	}
	meta, err := s.readMeta(dir, id)
	if err != nil {
		return Project{}, err
	}
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return Project{}, errors.New("new project name must not be empty")
	}
	newSlug := SlugifyProjectName(newName)
	if err := ValidateSegment("project folder name", newSlug); err != nil {
		return Project{}, err
	}
	newDir := filepath.Join(projectsDir(s.Root()), newSlug)
	if newSlug != id {
		if _, err := os.Stat(newDir); err == nil {
			return Project{}, fmt.Errorf("a project folder named %q already exists", newSlug)
		}
		// Reject links at the destination before moving anything.
		if err := EnsureNoReparsePoints(projectsDir(s.Root()), newDir); err != nil {
			return Project{}, err
		}
		if err := os.Rename(dir, newDir); err != nil {
			return Project{}, fmt.Errorf("cannot rename the project folder: %w", err)
		}
		meta.ID = newSlug
	}
	meta.Name = newName
	meta.UpdatedAt = time.Now().UTC()
	if err := s.writeMeta(newDir, meta); err != nil {
		return Project{}, err
	}
	return s.describe(newSlug)
}

// DeleteProject removes an entire project folder. The caller must confirm by
// passing the project id, which guards against a mistyped deletion.
func (s *Service) DeleteProject(id, confirm string) error {
	dir, err := s.ProjectPath(id)
	if err != nil {
		return err
	}
	if confirm != id {
		return errors.New("delete requires confirm to match the project id exactly")
	}
	root := s.Root()
	if err := EnsureInsideRoot(root, dir); err != nil {
		return err
	}
	if err := EnsureNoReparsePoints(root, dir); err != nil {
		return err
	}
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("cannot delete project %q: %w", id, err)
	}
	return nil
}
