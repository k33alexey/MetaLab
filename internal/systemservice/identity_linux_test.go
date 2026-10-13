//go:build linux

package systemservice

import (
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"
)

// fakeInstall records what preparing an install asked of the machine.
type fakeInstall struct {
	euid      int
	accounts  int
	claims    int
	added     bool
	checked   []string
	refuseFor string
}

func (fake *fakeInstall) host() installHost {
	return installHost{
		euid: func() int { return fake.euid },
		lookup: func(name string) (*user.User, error) {
			if name != "metalab" {
				return nil, errors.New("asked for another account: " + name)
			}
			if fake.accounts == 0 {
				return nil, user.UnknownUserError(name)
			}
			return &user.User{Username: name, Uid: "990", Gid: "990"}, nil
		},
		addAccount: func() error {
			fake.added = true
			fake.accounts++
			return nil
		},
		claimState: func() error {
			fake.claims++
			return nil
		},
		permits: func(account *user.User, path string, write bool) error {
			mode := "read"
			if write {
				mode = "write"
			}
			fake.checked = append(fake.checked, mode+" "+path)
			if path == fake.refuseFor {
				return errors.New("permission denied")
			}
			return nil
		},
	}
}

func writeSettings(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("version: 1\nlanguage: ru\nservice:\n  listen: 127.0.0.1:8090\n"+content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// Defect: install under another init system or without root goes on and
// leaves a user service, or a half-made account, behind.
func TestLinuxInstallNeedsSystemdAndRoot(t *testing.T) {
	path := writeSettings(t, "")
	for _, test := range []struct {
		name, platform string
		euid           int
		refusal        string
	}{
		{"sysv", "linux-system-v", 0, "systemd only"},
		{"user", systemdPlatform, 1000, "requires root"},
	} {
		fake := &fakeInstall{euid: test.euid}
		err := fake.host().prepare(test.platform, path)
		if err == nil || !strings.Contains(err.Error(), test.refusal) {
			t.Fatalf("%s: error = %v, want %q", test.name, err, test.refusal)
		}
		if fake.added || fake.claims != 0 || len(fake.checked) != 0 {
			t.Fatalf("%s: the machine was changed before the refusal: %+v", test.name, fake)
		}
	}
}

// Defect: a missing account is not created, an existing one is created
// again, or the state directory is not handed to it.
func TestLinuxInstallCreatesTheAccountOnlyWhenAbsent(t *testing.T) {
	path := writeSettings(t, "")
	for _, existing := range []int{0, 1} {
		fake := &fakeInstall{accounts: existing}
		if err := fake.host().prepare(systemdPlatform, path); err != nil {
			t.Fatal(err)
		}
		if fake.added != (existing == 0) || fake.accounts != 1 || fake.claims != 1 {
			t.Fatalf("existing=%d: %+v", existing, fake)
		}
		if len(fake.checked) != 1 || fake.checked[0] != "read "+path {
			t.Fatalf("existing=%d: checked %q, want the settings read", existing, fake.checked)
		}
	}
}

// Defect: install succeeds with settings or a backup folder the service
// cannot reach, and the service fails only after the next boot.
func TestLinuxInstallRefusesWhatTheAccountCannotReach(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(root, "archives", "daily")
	path := writeSettings(t, "backups:\n  directory: "+missing+"\n")

	fake := &fakeInstall{accounts: 1}
	if err := fake.host().prepare(systemdPlatform, path); err != nil {
		t.Fatal(err)
	}
	if strings.Join(fake.checked, "|") != "read "+path+"|write "+root {
		t.Fatalf("checked %q, want the settings and the nearest existing folder", fake.checked)
	}

	for _, refused := range []string{path, root} {
		fake = &fakeInstall{accounts: 1, refuseFor: refused}
		err := fake.host().prepare(systemdPlatform, path)
		if err == nil || !strings.Contains(err.Error(), refused) || !strings.Contains(err.Error(), "metalab") {
			t.Fatalf("refused %s: error = %v, want it named", refused, err)
		}
	}

	file := filepath.Join(root, "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	path = writeSettings(t, "backups:\n  directory: "+file+"\n")
	fake = &fakeInstall{accounts: 1}
	if err := fake.host().prepare(systemdPlatform, path); err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("backup folder that is a file: error = %v", err)
	}
}

// Defect: the access check judges by the file's own rights and misses a
// closed folder on the way, or lets through a file only root can read.
// Needs root to run the check under another account.
func TestLinuxAccountPermitsAsTheSystemJudges(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("running a check under another account needs root")
	}
	account, err := user.Lookup("nobody")
	if err != nil {
		t.Skipf("account nobody is absent: %v", err)
	}
	open := t.TempDir()
	// The test's folder and the one above it are closed to others by default.
	for _, folder := range []string{filepath.Dir(open), open} {
		if err := os.Chmod(folder, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	closed := filepath.Join(open, "closed")
	if err := os.Mkdir(closed, 0o700); err != nil {
		t.Fatal(err)
	}
	files := map[string]os.FileMode{
		filepath.Join(open, "public"):   0o644,
		filepath.Join(open, "private"):  0o600,
		filepath.Join(closed, "public"): 0o644,
	}
	for path, mode := range files {
		if err := os.WriteFile(path, nil, mode); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, mode); err != nil {
			t.Fatal(err)
		}
	}
	for _, test := range []struct {
		path    string
		write   bool
		allowed bool
	}{
		{filepath.Join(open, "public"), false, true},
		{filepath.Join(open, "private"), false, false},
		{filepath.Join(closed, "public"), false, false},
		{open, true, false},
	} {
		err := accountPermits(account, test.path, test.write)
		if (err == nil) != test.allowed {
			t.Fatalf("%s write=%v: error = %v, want allowed=%v", test.path, test.write, err, test.allowed)
		}
	}
	if err := os.Chmod(open, 0o777); err != nil {
		t.Fatal(err)
	}
	if err := accountPermits(account, open, true); err != nil {
		t.Fatalf("a folder open to everyone: %v", err)
	}
}
