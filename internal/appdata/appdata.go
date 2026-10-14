// Package appdata resolves the per-user Windows application-data folder used by
// BuildFlow for configuration, logs and the local API token.
//
// The folder is normally %LOCALAPPDATA%\BuildFlow. Setting the BUILDFLOW_HOME
// environment variable overrides the location, which is useful for testing and
// for running a fully isolated copy side by side with the real installation.
package appdata

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	// AppDirName is the folder name created inside the user's local app data.
	AppDirName = "BuildFlow"
	// EnvOverride relocates all per-user state. Mainly used by tests.
	EnvOverride = "BUILDFLOW_HOME"
	// TokenFileName holds the local REST API token.
	TokenFileName = "api-token.txt"
	// ConfigFileName holds the JSON settings file.
	ConfigFileName = "config.json"
	// LogDirName holds rotated log files.
	LogDirName = "logs"
)

// DefaultPort is the stable loopback port BuildFlow listens on.
const DefaultPort = 8765

// Settings is the small JSON document persisted per user. It holds metadata
// only; no user content is stored here.
type Settings struct {
	Version            int       `json:"version"`
	Port               int       `json:"port"`
	WorkspacePath      string    `json:"workspacePath"`
	OpenBrowserOnStart bool      `json:"openBrowserOnStart"`
	StartWithWindows   bool      `json:"startWithWindows"`
	BackgroundLaunch   bool      `json:"backgroundLaunch"`
	LastProject        string    `json:"lastProject"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

func defaultSettings() Settings {
	return Settings{
		Version:            1,
		Port:               DefaultPort,
		OpenBrowserOnStart: true,
		BackgroundLaunch:   false,
	}
}

// Store owns the application-data folder and the settings file.
type Store struct {
	mu       sync.RWMutex
	dir      string
	settings Settings
}

// Dir returns the application-data folder, creating it when necessary.
func Dir() (string, error) {
	if override := strings.TrimSpace(os.Getenv(EnvOverride)); override != "" {
		abs, err := filepath.Abs(override)
		if err != nil {
			return "", fmt.Errorf("resolve %s: %w", EnvOverride, err)
		}
		return abs, nil
	}
	base := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
	if base == "" {
		home, err := os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("locate per-user application data folder: %w", err)
		}
		// UserConfigDir is %APPDATA%; BuildFlow keeps volatile state local.
		base = filepath.Dir(home)
	}
	return filepath.Join(base, AppDirName), nil
}

// Open creates (if needed) and loads the application-data store.
func Open() (*Store, error) {
	dir, err := Dir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(dir, LogDirName), 0o755); err != nil {
		return nil, fmt.Errorf("create application data folder %q: %w", dir, err)
	}
	s := &Store{dir: dir, settings: defaultSettings()}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) load() error {
	path := filepath.Join(s.dir, ConfigFileName)
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s.Save()
	}
	if err != nil {
		return fmt.Errorf("read settings %q: %w", path, err)
	}
	var loaded Settings
	if err := json.Unmarshal(raw, &loaded); err != nil {
		// Never lose the ability to start: keep a copy and start from defaults.
		broken := path + ".broken-" + time.Now().Format("20060102-150405")
		_ = os.WriteFile(broken, raw, 0o644)
		s.settings = defaultSettings()
		return s.Save()
	}
	def := defaultSettings()
	if loaded.Version == 0 {
		loaded.Version = def.Version
	}
	if loaded.Port <= 0 || loaded.Port > 65535 {
		loaded.Port = def.Port
	}
	s.settings = loaded
	return nil
}

// Save writes settings atomically.
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.settings.Version = defaultSettings().Version
	s.settings.UpdatedAt = time.Now().UTC()
	raw, err := json.MarshalIndent(s.settings, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	path := filepath.Join(s.dir, ConfigFileName)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("replace settings: %w", err)
	}
	return nil
}

// Update applies a mutation to the settings and persists the result.
func (s *Store) Update(fn func(*Settings)) error {
	s.mu.Lock()
	fn(&s.settings)
	s.mu.Unlock()
	return s.Save()
}

// Settings returns a copy of the current settings.
func (s *Store) Settings() Settings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settings
}

// SetWorkspacePath stores the validated workspace folder.
func (s *Store) SetWorkspacePath(p string) error {
	return s.Update(func(s *Settings) { s.WorkspacePath = p })
}

// SetLastProject remembers the active project between runs.
func (s *Store) SetLastProject(id string) error {
	return s.Update(func(s *Settings) { s.LastProject = id })
}

// Path returns a path inside the application-data folder.
func (s *Store) Path(parts ...string) string {
	return filepath.Join(append([]string{s.dir}, parts...)...)
}

// LogDir returns the folder holding rotated log files.
func (s *Store) LogDir() string { return s.Path(LogDirName) }

// ConfigPath returns the settings file path.
func (s *Store) ConfigPath() string { return s.Path(ConfigFileName) }

// TokenPath returns the file holding the local REST API token.
func (s *Store) TokenPath() string { return s.Path(TokenFileName) }
