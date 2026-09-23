//go:build darwin
// +build darwin

package main

import (
	"os"
	"path"
)

// steamConfigDir returns Steam's config directory on macOS.
func steamConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return path.Join(home, "Library", "Application Support", "Steam", "config")
}
