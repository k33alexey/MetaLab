// Package servicestate knows where ML Service keeps its own files on this
// machine — the secrets kept in files and the backups without a folder in the
// settings — and how a folder of its own is laid: closed to everyone but its
// owner and, on Linux, given to the service account (ML-SERVICE.md,
// «Управление службой»; PLATFORM-MODEL.md, «Секреты в Windows», «Секреты в
// Linux»). The service, Manager and the command line find them in one place.
package servicestate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/k33alexey/MetaLab/internal/ownerfile"
)

const (
	secretsFolder = "secrets"
	backupsFolder = "backups"
)

// Secrets is the folder of the secrets kept in files (Windows and Linux; the
// macOS keychain keeps them itself).
func Secrets() (string, error) { return folder(secretsFolder) }

// Backups is the folder of the backups when the settings name none.
func Backups() (string, error) { return folder(backupsFolder) }

func folder(name string) (string, error) {
	state, err := Directory()
	if err != nil {
		return "", err
	}
	return filepath.Join(state, name), nil
}

// PrepareOwnDirectory creates a folder only its owner opens, or closes one
// left with wider rights, and gives it to the service account (HandOver). A
// link in place of the folder is refused: what is written into it would go
// where the link points.
func PrepareOwnDirectory(directory string) error {
	if err := os.MkdirAll(filepath.Dir(directory), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", directory, err)
	}
	if err := ownerfile.Mkdir(directory); err != nil && !errors.Is(err, os.ErrExist) {
		return fmt.Errorf("create %s: %w", directory, err)
	}
	information, err := os.Lstat(directory)
	if err != nil {
		return fmt.Errorf("read %s: %w", directory, err)
	}
	if information.Mode().Type() != os.ModeDir {
		return fmt.Errorf("%s is not a directory", directory)
	}
	if err := ownerfile.Restrict(directory); err != nil {
		return err
	}
	return HandOver(directory)
}
