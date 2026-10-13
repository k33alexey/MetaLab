//go:build linux

package secretstore

import (
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
)

// Defect: the secret or its directory is open to other users, at once or
// after a directory that already existed with wider rights is reused.
func TestLinuxSecretsAreClosedToOthers(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "secrets")
	if err := os.Mkdir(directory, 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(directory, 0o777); err != nil {
		t.Fatal(err)
	}
	backend := fileBackend{directory: directory}
	if err := backend.Set(serviceName, "database.one.password", "password"); err != nil {
		t.Fatal(err)
	}
	_, path, _ := backend.secretPath(serviceName, "database.one.password")
	for target, want := range map[string]os.FileMode{directory: 0o700, path: 0o600} {
		information, err := os.Lstat(target)
		if err != nil {
			t.Fatal(err)
		}
		if got := information.Mode().Perm(); got != want {
			t.Fatalf("%s permissions = %o, want %o", target, got, want)
		}
	}
}

// Defect: a link planted in place of the directory sends the secret where
// the link points.
func TestLinuxSecretDirectoryLinkIsRefused(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "elsewhere")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "secrets")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := (fileBackend{directory: link}).Set(serviceName, "database.one.password", "password"); err == nil {
		t.Fatal("Set() through a linked directory succeeded")
	}
	if entries, _ := os.ReadDir(target); len(entries) != 0 {
		t.Fatalf("link target received %d entries", len(entries))
	}
}

// Defect: a secret written by root (an administrator) belongs to root, and
// the service reading it under its own account is refused. Needs root and
// the service account, as on a server where the service is installed.
func TestLinuxSecretsWrittenByRootBelongToTheServiceAccount(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("handing a secret to another account needs root")
	}
	account, err := user.Lookup(ServiceAccount)
	if err != nil {
		t.Skipf("service account %q is absent: %v", ServiceAccount, err)
	}
	uid, _ := strconv.Atoi(account.Uid)
	directory := filepath.Join(t.TempDir(), "secrets")
	backend := fileBackend{directory: directory}
	if err := backend.Set(serviceName, "database.one.password", "password"); err != nil {
		t.Fatal(err)
	}
	_, path, _ := backend.secretPath(serviceName, "database.one.password")
	for _, target := range []string{directory, path} {
		information, err := os.Lstat(target)
		if err != nil {
			t.Fatal(err)
		}
		if owner := information.Sys().(*syscall.Stat_t).Uid; int(owner) != uid {
			t.Fatalf("%s belongs to user %d, want %s (%d)", target, owner, ServiceAccount, uid)
		}
	}
}
