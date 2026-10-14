//go:build !windows

package main

import "buildflow/internal/logging"

func checkPlatformVersion(logger *logging.Logger) error { return nil }
