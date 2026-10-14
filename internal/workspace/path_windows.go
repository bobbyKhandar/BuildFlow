//go:build windows

package workspace

import (
	"fmt"
	"path/filepath"
	"strings"
)

func validatePlatformWorkspacePath(clean string) error {
	vol := filepath.VolumeName(clean)
	if vol == "" {
		return fmt.Errorf("workspace path %q has no drive or share", clean)
	}
	unc := strings.HasPrefix(vol, `\\`)
	if !unc && len(vol) == 2 && vol[1] != ':' {
		return fmt.Errorf("workspace path %q has no drive or share", clean)
	}
	if unc {
		if strings.Trim(vol, `\`) == "" {
			return fmt.Errorf("UNC workspace path %q is missing a host name", clean)
		}
	} else {
		drive := strings.ToUpper(vol[:1])
		if drive < "A" || drive > "Z" {
			return fmt.Errorf("workspace path %q is not on a valid drive", clean)
		}
	}
	return nil
}
