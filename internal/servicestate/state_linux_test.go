//go:build linux

package servicestate

import (
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
)

// Defect: install leaves the state directory, or a folder of secrets or
// backups root made before the account existed, to root, and the service
// cannot read its secrets or write its archives; or follows a link planted
// in place of the directory. Needs root and the service account.
func TestLinuxServiceStateIsHandedToTheServiceAccount(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("handing a directory to another account needs root")
	}
	account, err := user.Lookup(Account)
	if err != nil {
		t.Skipf("service account %q is absent: %v", Account, err)
	}
	uid, _ := strconv.Atoi(account.Uid)
	state := filepath.Join(t.TempDir(), "metalab")
	secrets := filepath.Join(state, "secrets")
	backups := filepath.Join(state, "backups")
	if err := os.MkdirAll(secrets, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(backups, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := claim(state); err != nil {
		t.Fatal(err)
	}
	for target, want := range map[string]os.FileMode{state: 0o750, secrets: 0o700, backups: 0o700} {
		information, err := os.Lstat(target)
		if err != nil {
			t.Fatal(err)
		}
		if owner := information.Sys().(*syscall.Stat_t).Uid; int(owner) != uid || information.Mode().Perm() != want {
			t.Fatalf("%s: user %d rights %o, want %s (%d) and %o", target, owner, information.Mode().Perm(), Account, uid, want)
		}
	}

	elsewhere := t.TempDir()
	link := filepath.Join(t.TempDir(), "metalab")
	if err := os.Symlink(elsewhere, link); err != nil {
		t.Fatal(err)
	}
	if err := claim(link); err == nil {
		t.Fatal("a link in place of the state directory was claimed")
	}
	if information, _ := os.Stat(elsewhere); information.Sys().(*syscall.Stat_t).Uid == uint32(uid) {
		t.Fatal("the link's target was handed to the service account")
	}
}
