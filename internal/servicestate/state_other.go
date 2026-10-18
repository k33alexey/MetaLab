//go:build !windows && !linux

package servicestate

import (
	"fmt"
	"os"
	"path/filepath"
)

// Directory is the service's own directory: on macOS the service is a user
// service of the administrator, so its files are the user's own.
func Directory() (string, error) {
	directory, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve service directory: %w", err)
	}
	return filepath.Join(directory, "MetaLab"), nil
}

// HandOver has nothing to do: the service runs as the user who writes.
func HandOver(string) error { return nil }
