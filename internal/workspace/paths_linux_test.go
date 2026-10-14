//go:build linux

package workspace

import (
	"path/filepath"
	"testing"
)

func TestValidatePOSIXWorkspacePath(t *testing.T) {
	for _, input := range []string{"/home/bobby/BuildFlow", "/tmp/Build Flow — café", "/tmp/space and 雪/flow"} {
		got, err := ValidateWorkspacePath(input)
		if err != nil {
			t.Errorf("ValidateWorkspacePath(%q): %v", input, err)
			continue
		}
		if !filepath.IsAbs(got) {
			t.Errorf("path %q is not absolute", got)
		}
	}
	if _, err := ValidateWorkspacePath("relative/workspace"); err == nil {
		t.Error("relative path should be rejected")
	}
}
