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

// ServiceStateDirectory is ML Service's own directory on Linux: the service
// account owns it, it is the account's home, and the secrets lie in it.
const ServiceStateDirectory = "/var/lib/metalab"

// linuxSecretDirectory is the service state directory's secrets folder.
const linuxSecretDirectory = ServiceStateDirectory + "/secrets"

// A server has no unlocked Secret Service keyring, and a key kept on the same
// machine protects nothing root cannot read, so a secret is stored as is and
// the rights on the file and the directory keep it from other users.
const secretExtension = ".secret"

func secretDirectory() (string, error) { return linuxSecretDirectory, nil }

// ClaimServiceState gives the service account its state directory and the
// secrets directory in it, closed to others. Run by root when the service is
// installed: a directory left by an earlier run belongs to root otherwise —
// a secret refused for want of the account leaves the secrets folder behind.
func ClaimServiceState() error { return claimServiceState(ServiceStateDirectory) }

func claimServiceState(directory string) error {
	if err := os.Mkdir(directory, 0o750); err != nil && !os.IsExist(err) {
		return fmt.Errorf("create service directory: %w", err)
	}
	information, err := os.Lstat(directory)
	if err != nil {
		return fmt.Errorf("read service directory: %w", err)
	}
	if information.Mode().Type() != os.ModeDir {
		return fmt.Errorf("service directory %s is not a directory", directory)
	}
	if err := os.Chmod(directory, 0o750); err != nil {
		return fmt.Errorf("restrict service directory: %w", err)
	}
	if err := handOverSecret(directory); err != nil {
		return err
	}
	return prepareSecretDirectory(filepath.Join(directory, filepath.Base(linuxSecretDirectory)))
}

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
	uid, gid, err := AccountIDs(account)
	if err != nil {
		return err
	}
	if err := os.Lchown(path, uid, gid); err != nil {
		return fmt.Errorf("hand %s over to %q: %w", path, ServiceAccount, err)
	}
	return nil
}

// AccountIDs returns the numeric user and group of an account.
func AccountIDs(account *user.User) (int, int, error) {
	uid, err := strconv.Atoi(account.Uid)
	if err != nil {
		return 0, 0, fmt.Errorf("account %q has user id %q: %w", account.Username, account.Uid, err)
	}
	gid, err := strconv.Atoi(account.Gid)
	if err != nil {
		return 0, 0, fmt.Errorf("account %q has group id %q: %w", account.Username, account.Gid, err)
	}
	return uid, gid, nil
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
