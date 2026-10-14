//go:build windows

package main

import (
	"fmt"

	"buildflow/internal/logging"
	"buildflow/internal/winutil"
)

func checkPlatformVersion(logger *logging.Logger) error {
	major, minor, build, err := winutil.WindowsVersion()
	if err != nil {
		return nil
	}
	if major < 10 {
		return fmt.Errorf("this Windows build is version %d.%d; BuildFlow is tested on Windows 11 and Windows 10", major, minor)
	}
	logger.Debugf("Windows %d.%d build %d", major, minor, build)
	return nil
}
