//go:build linux

package secretstore

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"

	"github.com/k33alexey/MetaLab/internal/ownerfile"
)

// ServiceAccount is the account ML Service runs under on Linux; the secrets
// belong to it (PLATFORM-MODEL.md, «Секреты в Linux»).
const ServiceAccount = "metalab"

// linuxSecretDirectory is the service state directory's secrets folder.
const linuxSecretDirectory = "/var/lib/metalab/secrets"

// A server has no unlocked Secret Service keyring, and a key kept on the same
// machine protects nothing root cannot read, so a secret is stored as is and
// the rights on the file and the directory keep it from other users.
const secretExtension = ".secret"

func secretDirectory() (string, error) { return linuxSecretDirectory, nil }

func prepareSecretDirectory(directory string) error {
	if err := os.MkdirAll(filepath.Dir(directory), 0o755); err != nil {
		return fmt.Errorf("create secret directory: %w", err)
	}
	if err := os.Mkdir(directory, 0o700); err != nil && !os.IsExist(err) {
		return fmt.Errorf("create secret directory: %w", err)
	}
	information, err := os.Lstat(directory)
	if err != nil {
		return fmt.Errorf("read secret directory: %w", err)
	}
	if information.Mode().Type() != os.ModeDir {
		return fmt.Errorf("secret directory %s is not a directory", directory)
	}
	if err := ownerfile.Restrict(directory); err != nil {
		return fmt.Errorf("restrict secret directory: %w", err)
	}
	return handOverSecret(directory)
}

// handOverSecret gives a path root writes to the service account: root reads
// everything anyway, and the service must read what an administrator stored.
// Any other writer is the service account itself or is refused by the rights.
func handOverSecret(path string) error {
	if os.Geteuid() != 0 {
		return nil
	}
	account, err := user.Lookup(ServiceAccount)
	if err != nil {
		return fmt.Errorf("service account %q is not available: %w", ServiceAccount, err)
	}
	uid, err := strconv.Atoi(account.Uid)
	if err != nil {
		return fmt.Errorf("service account %q has user id %q: %w", ServiceAccount, account.Uid, err)
	}
	gid, err := strconv.Atoi(account.Gid)
	if err != nil {
		return fmt.Errorf("service account %q has group id %q: %w", ServiceAccount, account.Gid, err)
	}
	if err := os.Lchown(path, uid, gid); err != nil {
		return fmt.Errorf("hand secret over to %q: %w", ServiceAccount, err)
	}
	return nil
}

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
