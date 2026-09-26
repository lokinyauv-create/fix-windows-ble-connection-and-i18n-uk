//go:build windows
// +build windows

package main

import "os"

// steamConfigDir returns Steam's config directory on Windows.
func steamConfigDir() string {
	return os.ExpandEnv("${ProgramFiles(x86)}\\Steam\\config")
}
