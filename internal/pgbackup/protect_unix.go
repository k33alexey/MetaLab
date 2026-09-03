//go:build !windows

package pgbackup

import (
	"fmt"
	"os"
)

func protectFile(path string) error {
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("protect backup file: %w", err)
	}
	return nil
}
