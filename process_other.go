//go:build !windows
// +build !windows

package main

import (
	"os"
	"strconv"
	"strings"
)

// isProcRunning checks /proc for a process whose comm matches one of names.
// Callers pass Windows-style names like "vrserver.exe"; SteamVR's Linux
// binary is just "vrserver", so the ".exe" suffix is stripped before matching.
func isProcRunning(names ...string) (bool, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return false, err
	}

	want := make(map[string]bool, len(names))
	for _, name := range names {
		want[strings.ToLower(strings.TrimSuffix(name, ".exe"))] = true
	}

	for _, entry := range entries {
		if _, err := strconv.Atoi(entry.Name()); err != nil {
			continue
		}

		comm, err := os.ReadFile("/proc/" + entry.Name() + "/comm")
		if err != nil {
			continue
		}

		if want[strings.ToLower(strings.TrimSpace(string(comm)))] {
			return true, nil
		}
	}

	return false, nil
}
