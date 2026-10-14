//go:build linux

package appdata

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func resolveDirectories() (configDir, stateDir string, isolated bool, err error) {
	if override := strings.TrimSpace(os.Getenv(EnvOverride)); override != "" {
		abs, e := filepath.Abs(override)
		if e != nil {
			return "", "", false, fmt.Errorf("resolve %s: %w", EnvOverride, e)
		}
		return abs, abs, true, nil
	}
	home, e := os.UserHomeDir()
	if e != nil || !filepath.IsAbs(home) {
		if e != nil {
			return "", "", false, fmt.Errorf("locate home directory: %w", e)
		}
		return "", "", false, fmt.Errorf("home directory must be absolute")
	}
	configBase := absoluteXDG("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	stateBase := absoluteXDG("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	return filepath.Join(configBase, "buildflow"), filepath.Join(stateBase, "buildflow"), false, nil
}

func absoluteXDG(name, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value != "" && filepath.IsAbs(value) {
		return filepath.Clean(value)
	}
	return fallback
}

// Only app-owned directories are chmodded. Shared XDG parents are never
// changed, even if their permissions differ from the XDG recommendation.
func prepareAppDir(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	return os.Chmod(path, 0o700)
}

func protectToken(path string) error { return os.Chmod(path, 0o600) }
