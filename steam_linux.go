//go:build linux
// +build linux

package main

import (
	"os"
	"path"
)

// steamConfigDir returns Steam's config directory for a native Linux
// package install. Flatpak/Snap Steam use a different prefix and aren't
// handled here.
func steamConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return path.Join(home, ".local", "share", "Steam", "config")
}
