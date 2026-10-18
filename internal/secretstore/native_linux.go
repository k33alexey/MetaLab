//go:build linux

package secretstore

import (
	"fmt"
	"os"
	"path/filepath"
)

// A server has no unlocked Secret Service keyring, and a key kept on the same
// machine protects nothing root cannot read, so a secret is stored as is and
// the rights on the file and the directory keep it from other users.
const secretExtension = ".secret"

// replaceSecret renames the written file over the secret and flushes the
// directory: until then a crash may bring back the previous secret, or none.
func replaceSecret(temporaryPath, path string) error {
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace secret: %w", err)
	}
	directory, err := os.Open(filepath.Dir(path))
	if err != nil {
		return fmt.Errorf("open secret directory: %w", err)
	}
	syncErr := directory.Sync()
	closeErr := directory.Close()
	if syncErr != nil {
		return fmt.Errorf("flush secret directory: %w", syncErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close secret directory: %w", closeErr)
	}
	return nil
}

func sealSecret(plain []byte) ([]byte, error) { return plain, nil }

func openSecret(stored []byte) ([]byte, error) { return stored, nil }
