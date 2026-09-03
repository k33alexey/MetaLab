//go:build !windows

package pgcopy

import (
	"fmt"
	"os"
)

func protectCredentialFile(path string) error {
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("protect temporary PostgreSQL password file: %w", err)
	}
	return nil
}
