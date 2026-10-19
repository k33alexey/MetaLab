//go:build !windows

package atomicfile

import (
	"fmt"
	"os"
	"path/filepath"
)

// Replace puts source in place of destination in one step, and flushes the
// directory: until then a crash may bring back the previous file, or none.
func Replace(source, destination string) error {
	if err := os.Rename(source, destination); err != nil {
		return err
	}
	directory, err := os.Open(filepath.Dir(destination))
	if err != nil {
		return fmt.Errorf("open directory: %w", err)
	}
	syncErr := directory.Sync()
	closeErr := directory.Close()
	if syncErr != nil {
		return fmt.Errorf("flush directory: %w", syncErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close directory: %w", closeErr)
	}
	return nil
}
