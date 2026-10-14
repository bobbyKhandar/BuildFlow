//go:build !windows && !linux

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
	home, e := os.UserConfigDir()
	if e != nil {
		return "", "", false, e
	}
	dir := filepath.Join(filepath.Dir(home), AppDirName)
	return dir, dir, false, nil
}

func prepareAppDir(path string) error { return os.MkdirAll(path, 0o755) }

func protectToken(path string) error { return nil }
