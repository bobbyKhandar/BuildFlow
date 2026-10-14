//go:build windows

package workspace

import "testing"

func TestValidateWindowsDriveAndUNCPaths(t *testing.T) {
	for _, input := range []string{`C:\BuildFlow`, `\\server\share\BuildFlow`} {
		if _, err := ValidateWorkspacePath(input); err != nil {
			t.Errorf("ValidateWorkspacePath(%q): %v", input, err)
		}
	}
	for _, input := range []string{`relative\workspace`, `\rooted-without-drive`} {
		if _, err := ValidateWorkspacePath(input); err == nil {
			t.Errorf("ValidateWorkspacePath(%q) should fail", input)
		}
	}
}
