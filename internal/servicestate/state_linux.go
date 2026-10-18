//go:build linux

package servicestate

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
)

// Account is the account ML Service runs under on Linux; its secrets and
// backups belong to it (PLATFORM-MODEL.md, «Секреты в Linux»).
const Account = "metalab"

// StateDirectory is ML Service's own directory on Linux: the service account
// owns it and has it for its home.
const StateDirectory = "/var/lib/metalab"

// Directory is the service's own directory.
func Directory() (string, error) { return StateDirectory, nil }

// Claim gives the service account its state directory and the folders of
// the secrets and the backups in it, closed to others. Run by root when the
// service is installed: a folder left by an earlier run belongs to root
// otherwise — a secret refused for want of the account leaves the secrets
// folder behind.
func Claim() error { return claim(StateDirectory) }

func claim(directory string) error {
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
	if err := HandOver(directory); err != nil {
		return err
	}
	for _, name := range []string{secretsFolder, backupsFolder} {
		if err := PrepareOwnDirectory(filepath.Join(directory, name)); err != nil {
			return err
		}
	}
	return nil
}

// HandOver gives a path root writes to the service account: root reads
// everything anyway, and the service must read what an administrator wrote —
// a secret, or an archive it restores. Any other writer is the service
// account itself or is refused by the rights.
func HandOver(path string) error {
	if os.Geteuid() != 0 {
		return nil
	}
	account, err := user.Lookup(Account)
	if err != nil {
		return fmt.Errorf("service account %q is not available: %w", Account, err)
	}
	uid, gid, err := AccountIDs(account)
	if err != nil {
		return err
	}
	if err := os.Lchown(path, uid, gid); err != nil {
		return fmt.Errorf("hand %s over to %q: %w", path, Account, err)
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
