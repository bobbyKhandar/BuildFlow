//go:build linux

package main

import (
	"os"
	"path/filepath"
	"testing"

	"buildflow/internal/appdata"
)

func TestTokenFileIsPrivate(t *testing.T) {
	root := filepath.Join(t.TempDir(), "private")
	t.Setenv(appdata.EnvOverride, root)
	store, err := appdata.Open()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ensureToken(store, appdata.DefaultPort); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(store.TokenPath())
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("token mode = %04o", got)
	}
	if err := os.Chmod(store.TokenPath(), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ensureToken(store, appdata.DefaultPort); err != nil {
		t.Fatal(err)
	}
	info, err = os.Stat(store.TokenPath())
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("existing token mode = %04o", got)
	}
}
