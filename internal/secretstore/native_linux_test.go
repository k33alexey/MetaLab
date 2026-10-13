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

// Defect: install leaves the state directory, or a secrets folder root made
// before the account existed, to root, and the service cannot read its
// secrets; or follows a link planted in place of the directory. Needs root
// and the service account.
func TestLinuxServiceStateIsHandedToTheServiceAccount(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("handing a directory to another account needs root")
	}
	account, err := user.Lookup(ServiceAccount)
	if err != nil {
		t.Skipf("service account %q is absent: %v", ServiceAccount, err)
	}
	uid, _ := strconv.Atoi(account.Uid)
	state := filepath.Join(t.TempDir(), "metalab")
	secrets := filepath.Join(state, "secrets")
	if err := os.MkdirAll(secrets, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := claimServiceState(state); err != nil {
		t.Fatal(err)
	}
	for target, want := range map[string]os.FileMode{state: 0o750, secrets: 0o700} {
		information, err := os.Lstat(target)
		if err != nil {
			t.Fatal(err)
		}
		if owner := information.Sys().(*syscall.Stat_t).Uid; int(owner) != uid || information.Mode().Perm() != want {
			t.Fatalf("%s: user %d rights %o, want %s (%d) and %o", target, owner, information.Mode().Perm(), ServiceAccount, uid, want)
		}
	}

	elsewhere := t.TempDir()
	link := filepath.Join(t.TempDir(), "metalab")
	if err := os.Symlink(elsewhere, link); err != nil {
		t.Fatal(err)
	}
	if err := claimServiceState(link); err == nil {
		t.Fatal("a link in place of the state directory was claimed")
	}
	if information, _ := os.Stat(elsewhere); information.Sys().(*syscall.Stat_t).Uid == uint32(uid) {
		t.Fatal("the link's target was handed to the service account")
	}
}
