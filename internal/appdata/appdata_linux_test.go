//go:build linux

package appdata

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestXDGDefaultsAndAbsoluteOverrides(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv(EnvOverride, "")
	config, state, isolated, err := resolveDirectories()
	if err != nil {
		t.Fatal(err)
	}
	if isolated {
		t.Fatal("unexpected isolated mode")
	}
	if config != filepath.Join(home, ".config", "buildflow") {
		t.Fatalf("config path = %q", config)
	}
	if state != filepath.Join(home, ".local", "state", "buildflow") {
		t.Fatalf("state path = %q", state)
	}

	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config custom"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state custom"))
	config, state, _, err = resolveDirectories()
	if err != nil {
		t.Fatal(err)
	}
	if config != filepath.Join(home, "config custom", "buildflow") || state != filepath.Join(home, "state custom", "buildflow") {
		t.Fatalf("override paths: %q %q", config, state)
	}
}

func TestRelativeXDGValuesUseDefaultsAndPrivateDirs(t *testing.T) {
	home := t.TempDir()
	configParent := filepath.Join(home, "shared-config")
	stateParent := filepath.Join(home, "shared-state")
	if err := os.MkdirAll(configParent, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(stateParent, 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.Chmod(configParent, 0o755)
	_ = os.Chmod(stateParent, 0o755)
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "relative/config")
	t.Setenv("XDG_STATE_HOME", "relative/state")
	t.Setenv(EnvOverride, "")
	t.Setenv("HOME", home)
	store, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{store.ConfigPath(), store.LogDir(), store.TokenPath()} {
		if !filepath.IsAbs(path) {
			t.Fatalf("non-absolute path %q", path)
		}
	}
	for _, path := range []string{filepath.Join(home, ".config", "buildflow"), filepath.Join(home, ".local", "state", "buildflow"), filepath.Join(home, ".local", "state", "buildflow", "logs")} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != 0o700 {
			t.Errorf("mode %s = %04o", path, got)
		}
	}
	for _, path := range []string{configParent, stateParent} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != 0o755 {
			t.Errorf("shared parent mode %s changed to %04o", path, got)
		}
	}
}

func TestBuildFlowHomeKeepsUnifiedIsolatedRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "my isolated flow")
	t.Setenv(EnvOverride, root)
	store, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	if !store.Isolated() {
		t.Fatal("BUILDFLOW_HOME should mark the run isolated")
	}
	if store.ConfigPath() != filepath.Join(root, ConfigFileName) || store.TokenPath() != filepath.Join(root, TokenFileName) || store.LogDir() != filepath.Join(root, LogDirName) {
		t.Fatalf("BUILDFLOW_HOME did not retain unified layout")
	}
}

func TestLoginStartupCanonicalFalseWinsLegacyTrue(t *testing.T) {
	var settings Settings
	if err := json.Unmarshal([]byte(`{"startAtLogin":false,"startWithWindows":true}`), &settings); err != nil {
		t.Fatal(err)
	}
	if settings.StartAtLogin == nil {
		t.Fatal("canonical field presence was lost")
	}
	if settings.LoginStartupEnabled() {
		t.Fatal("false canonical value must take precedence")
	}
	settings = Settings{}
	if err := json.Unmarshal([]byte(`{"startWithWindows":true}`), &settings); err != nil {
		t.Fatal(err)
	}
	if !settings.LoginStartupEnabled() {
		t.Fatal("legacy value should be used when canonical value is absent")
	}
}
