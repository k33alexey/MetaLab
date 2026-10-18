//go:build windows

package servicestate

import (
	"fmt"
	"os"
	"path/filepath"
)

// Directory is the service's own directory: machine-wide, since the account
// that writes (an administrator in Manager) is not the one that reads (the
// service).
func Directory() (string, error) {
	programData := os.Getenv("ProgramData")
	if programData == "" {
		return "", fmt.Errorf("ProgramData is not configured")
	}
	return filepath.Join(programData, "MetaLab"), nil
}

// HandOver has nothing to do: the system, the administrators and the writing
// account already have access to what ownerfile creates.
func HandOver(string) error { return nil }
