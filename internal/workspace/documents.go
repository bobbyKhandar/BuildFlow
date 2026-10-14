package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Doc is the unit BuildFlow reads and writes. Content is always UTF-8 Markdown.
type Doc struct {
	Kind     string    `json:"kind"`
	Stage    string    `json:"stage"`
	Package  string    `json:"package,omitempty"`
	Path     string    `json:"path"` // workspace-relative, for display
	AbsPath  string    `json:"-"`    // never serialised
	Exists   bool      `json:"exists"`
	Content  string    `json:"content"`
	Hash     string    `json:"hash"` // sha256 of the bytes on disk, "" when missing
	ModTime  time.Time `json:"modTime"`
	Size     int64     `json:"size"`
	Readonly bool      `json:"readonly,omitempty"`
}

// ErrNoWorkspace is returned when no workspace folder has been chosen yet.
var ErrNoWorkspace = errors.New("no workspace folder has been selected yet")

// ErrNotFound is returned when a project does not exist.
var ErrNotFound = errors.New("not found")

// ConflictError reports that the file on disk changed after it was loaded, so
// the requested write was refused. The caller's text is left untouched.
type ConflictError struct {
	Path      string    `json:"path"`
	Reason    string    `json:"reason"`
	Expected  string    `json:"expectedHash"`
	Actual    string    `json:"actualHash"`
	Modified  time.Time `json:"modified"`
	Content   string    `json:"content"`
	Existed   bool      `json:"existed"`
	CreatedBy string    `json:"createdExternally,omitempty"`
}

func (e *ConflictError) Error() string {
	if e.Reason == "created" {
		return fmt.Sprintf("%s was created outside BuildFlow after this document was loaded", e.Path)
	}
	return fmt.Sprintf("%s changed on disk after it was loaded in BuildFlow", e.Path)
}

// Service performs every workspace file operation. It is safe for concurrent
// use; writes are serialised so two saves cannot interleave.
type Service struct {
	mu    sync.RWMutex
	root  string
	write sync.Mutex
}

// NewService returns a service with no workspace selected yet.
func NewService() *Service { return &Service{} }

// Root returns the current workspace folder ("" when unset).
func (s *Service) Root() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.root
}

// SetRoot validates and stores the workspace folder.
func (s *Service) SetRoot(path string) error {
	clean, err := ValidateWorkspacePath(path)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.root = clean
	s.mu.Unlock()
	return nil
}

// ProjectPath returns the absolute folder for a project id.
func (s *Service) ProjectPath(project string) (string, error) {
	if err := ValidateSegment("project id", project); err != nil {
		return "", err
	}
	root := s.Root()
	if root == "" {
		return "", ErrNoWorkspace
	}
	return filepath.Join(root, "projects", project), nil
}

func (s *Service) absolute(rel string) (string, error) {
	root := s.Root()
	if root == "" {
		return "", ErrNoWorkspace
	}
	abs := filepath.Join(root, rel)
	if err := EnsureInsideRoot(root, abs); err != nil {
		return "", err
	}
	return abs, nil
}

func hashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// ReadDoc loads a document. Missing files are reported as Exists=false with
// empty content rather than as an error, because every stage starts empty.
func (s *Service) ReadDoc(kind, stage, pkg, project string) (Doc, error) {
	rel, err := Resolve(kind, stage, pkg, project)
	if err != nil {
		return Doc{}, err
	}
	abs, err := s.absolute(rel)
	if err != nil {
		return Doc{}, err
	}
	if err := EnsureNoReparsePoints(s.Root(), abs); err != nil {
		return Doc{}, err
	}
	doc := Doc{Kind: kind, Stage: stage, Package: pkg, Path: filepath.ToSlash(rel), AbsPath: abs}
	info, statErr := os.Stat(abs)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			return doc, nil
		}
		return doc, fmt.Errorf("cannot inspect %s: %w", doc.Path, statErr)
	}
	if info.IsDir() {
		return doc, fmt.Errorf("%s is a folder, not a Markdown file", doc.Path)
	}
	raw, readErr := os.ReadFile(abs)
	if readErr != nil {
		return doc, fmt.Errorf("cannot read %s: %w", doc.Path, readErr)
	}
	doc.Exists = true
	doc.Content = string(raw)
	doc.Hash = hashBytes(raw)
	doc.ModTime = info.ModTime().UTC()
	doc.Size = info.Size()
	return doc, nil
}

// WriteRequest describes a save. BaseHash must match the hash the client last
// received, otherwise the write is refused with a ConflictError.
type WriteRequest struct {
	Kind     string
	Stage    string
	Package  string
	Project  string
	Content  string
	BaseHash string
	Force    bool // explicit user confirmed overwrite of a changed file
}

// WriteDoc saves a document safely: it writes a sibling temporary file, then
// atomically replaces the target. A refused write never touches the target.
func (s *Service) WriteDoc(req WriteRequest) (Doc, error) {
	s.write.Lock()
	defer s.write.Unlock()

	rel, err := Resolve(req.Kind, req.Stage, req.Package, req.Project)
	if err != nil {
		return Doc{}, err
	}
	abs, err := s.absolute(rel)
	if err != nil {
		return Doc{}, err
	}
	root := s.Root()
	if err := EnsureNoReparsePoints(root, abs); err != nil {
		return Doc{}, err
	}
	doc := Doc{Kind: req.Kind, Stage: req.Stage, Package: req.Package, Path: filepath.ToSlash(rel), AbsPath: abs}

	raw := []byte(req.Content)
	if len(raw) > 8<<20 {
		return doc, fmt.Errorf("refusing to write more than 8 MiB to %s", doc.Path)
	}

	// Conflict detection against the live file on disk.
	current, readErr := s.readDocLocked(abs, doc)
	if readErr != nil {
		return doc, readErr
	}
	if !req.Force {
		switch {
		case current.Exists && req.BaseHash == "":
			// The client believed the file did not exist yet.
			return doc, &ConflictError{
				Path: doc.Path, Reason: "created", Expected: req.BaseHash, Actual: current.Hash,
				Modified: current.ModTime, Content: current.Content, Existed: false,
			}
		case current.Exists && current.Hash != req.BaseHash:
			return doc, &ConflictError{
				Path: doc.Path, Reason: "modified", Expected: req.BaseHash, Actual: current.Hash,
				Modified: current.ModTime, Content: current.Content, Existed: true,
			}
		case !current.Exists && req.BaseHash != "":
			// The file was deleted outside BuildFlow while the editor had it.
			return doc, &ConflictError{
				Path: doc.Path, Reason: "deleted", Expected: req.BaseHash, Actual: "",
				Content: current.Content, Existed: false,
			}
		}
	}

	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return doc, fmt.Errorf("cannot create folder for %s: %w", doc.Path, err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(abs), ".buildflow-*.tmp")
	if err != nil {
		return doc, fmt.Errorf("cannot create a temporary file next to %s: %w", doc.Path, err)
	}
	tmpName := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}
	if _, err := tmp.Write(raw); err != nil {
		cleanup()
		return doc, fmt.Errorf("cannot write %s: %w", doc.Path, err)
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return doc, fmt.Errorf("cannot flush %s: %w", doc.Path, err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return doc, fmt.Errorf("cannot close %s: %w", doc.Path, err)
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		_ = os.Remove(tmpName)
		return doc, fmt.Errorf("cannot set permissions on %s: %w", doc.Path, err)
	}
	if err := os.Rename(tmpName, abs); err != nil {
		_ = os.Remove(tmpName)
		return doc, fmt.Errorf("cannot replace %s: %w", doc.Path, err)
	}
	return s.readDocLocked(abs, doc)
}

func (s *Service) readDocLocked(abs string, doc Doc) (Doc, error) {
	info, statErr := os.Stat(abs)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			doc.Exists = false
			doc.Content = ""
			doc.Hash = ""
			doc.Size = 0
			return doc, nil
		}
		return doc, fmt.Errorf("cannot inspect %s: %w", doc.Path, statErr)
	}
	raw, readErr := os.ReadFile(abs)
	if readErr != nil {
		return doc, fmt.Errorf("cannot read %s: %w", doc.Path, readErr)
	}
	doc.Exists = true
	doc.Content = string(raw)
	doc.Hash = hashBytes(raw)
	doc.ModTime = info.ModTime().UTC()
	doc.Size = info.Size()
	return doc, nil
}

// DeleteDoc removes a document. Only the optional documents (local override,
// snapshot) support deletion; stage inputs and outputs do not, so a project
// never loses its history by accident.
func (s *Service) DeleteDoc(kind, stage, pkg, project string) error {
	switch kind {
	case KindTemplateLocal, KindSnapshot, KindPackageSnapshot:
	default:
		return fmt.Errorf("documents of kind %q cannot be deleted from BuildFlow; they are part of the project's history", kind)
	}
	rel, err := Resolve(kind, stage, pkg, project)
	if err != nil {
		return err
	}
	abs, err := s.absolute(rel)
	if err != nil {
		return err
	}
	if err := os.Remove(abs); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("cannot delete %s: %w", filepath.ToSlash(rel), err)
	}
	return nil
}

// ListMarkdown returns the base names (without extension) of *.md files in a
// workspace-relative folder, sorted naturally. The folder is split into
// segments and each one is validated, so a crafted value cannot escape the
// workspace.
func (s *Service) ListMarkdown(relFolder string) ([]string, error) {
	rel := filepath.FromSlash(relFolder)
	for _, seg := range strings.Split(rel, string(filepath.Separator)) {
		if seg == "" || seg == "." {
			continue
		}
		if err := ValidateSegment("folder", seg); err != nil {
			return nil, err
		}
	}
	abs, err := s.absolute(rel)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("cannot list %s: %w", filepath.ToSlash(relFolder), err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".md") {
			continue
		}
		out = append(out, strings.TrimSuffix(e.Name(), filepath.Ext(e.Name())))
	}
	SortPackageIDs(out)
	return out, nil
}

// Exists reports whether a resolved document is present on disk.
func (s *Service) Exists(kind, stage, pkg, project string) bool {
	rel, err := Resolve(kind, stage, pkg, project)
	if err != nil {
		return false
	}
	abs, err := s.absolute(rel)
	if err != nil {
		return false
	}
	info, err := os.Stat(abs)
	return err == nil && !info.IsDir()
}

// StatTimes returns the modification time of a document, or the zero time.
func (s *Service) StatTimes(kind, stage, pkg, project string) time.Time {
	rel, err := Resolve(kind, stage, pkg, project)
	if err != nil {
		return time.Time{}
	}
	abs, err := s.absolute(rel)
	if err != nil {
		return time.Time{}
	}
	info, err := os.Stat(abs)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime().UTC()
}

// WriteFileAbsolute writes a file outside the managed document set. It is used
// only for small metadata (project.json) and the API token, and it applies the
// same temporary-file-plus-rename strategy.
func WriteFileAtomic(abs string, raw []byte) error {
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(abs), ".buildflow-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		_ = os.Remove(name)
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(name)
		return err
	}
	if err := os.Rename(name, abs); err != nil {
		_ = os.Remove(name)
		return err
	}
	return nil
}

// ReadFileLimited reads a file, refusing anything larger than max bytes.
func ReadFileLimited(abs string, max int64) ([]byte, error) {
	f, err := os.Open(abs)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() > max {
		return nil, fmt.Errorf("file is too large (%d bytes)", info.Size())
	}
	return readAllLimited(f, max)
}

func readAllLimited(f *os.File, max int64) ([]byte, error) {
	var buf []byte
	tmp := make([]byte, 32*1024)
	var total int64
	for {
		n, err := f.Read(tmp)
		if n > 0 {
			total += int64(n)
			if total > max {
				return nil, fmt.Errorf("file is too large")
			}
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			if errors.Is(err, fs.ErrClosed) {
				return buf, nil
			}
			return buf, err
		}
	}
}
