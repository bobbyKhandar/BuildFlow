// Package appdata resolves the per-user application-data folders used by
// BuildFlow for configuration, logs and the local API token.
//
// The folder is normally %LOCALAPPDATA%\BuildFlow. Setting the BUILDFLOW_HOME
// environment variable overrides the location, which is useful for testing and
// for running a fully isolated copy side by side with the real installation.
package appdata

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	StartAtLogin       *bool     `json:"startAtLogin,omitempty"`
	BackgroundLaunch   bool      `json:"backgroundLaunch"`
	LastProject        string    `json:"lastProject"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

// LoginStartupEnabled resolves the canonical setting and legacy alias. A
// present startAtLogin value wins even when it is false.
func (settings Settings) LoginStartupEnabled() bool {
	if settings.StartAtLogin != nil {
		return *settings.StartAtLogin
	}
	return settings.StartWithWindows
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
	mu        sync.RWMutex
	dir       string
	configDir string
	stateDir  string
	isolated  bool
	settings  Settings
}

// Dir returns the application-data folder, creating it when necessary.
func Dir() (string, error) {
	configDir, _, _, err := resolveDirectories()
	return configDir, err
}

// StateDir resolves the state root without opening the store.
func StateDir() (string, error) {
	_, stateDir, _, err := resolveDirectories()
	return stateDir, err
}

// Open creates (if needed) and loads the application-data store.
func Open() (*Store, error) {
	configDir, stateDir, isolated, err := resolveDirectories()
	if err != nil {
		return nil, err
	}
	if err := prepareAppDir(configDir); err != nil {
		return nil, fmt.Errorf("create configuration folder %q: %w", configDir, err)
	}
	if err := prepareAppDir(stateDir); err != nil {
		return nil, fmt.Errorf("create state folder %q: %w", stateDir, err)
	}
	if err := prepareAppDir(filepath.Join(stateDir, LogDirName)); err != nil {
		return nil, fmt.Errorf("create log folder %q: %w", filepath.Join(stateDir, LogDirName), err)
	}
	s := &Store{dir: configDir, configDir: configDir, stateDir: stateDir, isolated: isolated, settings: defaultSettings()}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) load() error {
	path := s.ConfigPath()
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
	path := s.ConfigPath()
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

// RootPath is the canonical application data root used for instance identity.
// On Linux the settings and state directories are separate; an explicit
// BUILDFLOW_HOME keeps the historic unified-root layout.
func (s *Store) RootPath() string { return s.dir }

// StatePath returns a path inside the state root (logs and secrets).
func (s *Store) StatePath(parts ...string) string {
	return filepath.Join(append([]string{s.stateDir}, parts...)...)
}

// IdentityKey is stable across launches and specific to this configuration
// and state pair. Symlinks are resolved when possible before hashing.
func (s *Store) IdentityKey() string {
	paths := []string{s.configDir, s.stateDir}
	for i, p := range paths {
		if abs, err := filepath.Abs(p); err == nil {
			p = abs
		}
		if real, err := filepath.EvalSymlinks(p); err == nil {
			p = real
		}
		paths[i] = filepath.Clean(p)
	}
	sum := sha256.Sum256([]byte(paths[0] + "\x00" + paths[1]))
	return hex.EncodeToString(sum[:])
}

// Isolated reports whether BUILDFLOW_HOME is active.
func (s *Store) Isolated() bool { return s.isolated }

// LoginStartupEnabled resolves the canonical setting and legacy alias.
func (s *Store) LoginStartupEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settings.LoginStartupEnabled()
}

// LogDir returns the folder holding rotated log files.
func (s *Store) LogDir() string { return filepath.Join(s.stateDir, LogDirName) }

// ConfigPath returns the settings file path.
func (s *Store) ConfigPath() string { return filepath.Join(s.configDir, ConfigFileName) }

// TokenPath returns the file holding the local REST API token.
func (s *Store) TokenPath() string { return filepath.Join(s.stateDir, TokenFileName) }

// ProtectToken enforces the platform's token-file permissions when a token
// already exists. On Linux this is mode 0600.
func ProtectToken(path string) error { return protectToken(path) }
