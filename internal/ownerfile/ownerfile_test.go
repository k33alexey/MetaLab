//go:build !windows

package ownerfile

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// Not parallel: the process umask is cleared so that the permissions checked
// below come from this package and not from the umask of whoever runs tests.
func clearUmask(t *testing.T) {
	previous := syscall.Umask(0)
	t.Cleanup(func() { syscall.Umask(previous) })
}

func permissions(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode().Perm()
}

// Defect: a secret file is created readable by group or others and is only
// closed after the secret has been written into it.
func TestFilesAreOwnerOnlyBeforeAnythingIsWritten(t *testing.T) {
	clearUmask(t)
	directory := t.TempDir()

	file, err := Create(filepath.Join(directory, "pgpass"))
	if err != nil {
		t.Fatal(err)
	}
	if got := permissions(t, file.Name()); got != 0o600 {
		t.Fatalf("Create permissions = %v, want 0600", got)
	}
	_ = file.Close()

	temporary, err := CreateTemp(directory, ".secret-*")
	if err != nil {
		t.Fatal(err)
	}
	if got := permissions(t, temporary.Name()); got != 0o600 {
		t.Fatalf("CreateTemp permissions = %v, want 0600", got)
	}
	_ = temporary.Close()

	private, err := MkdirTemp(directory, "metalab-*")
	if err != nil {
		t.Fatal(err)
	}
	if got := permissions(t, private); got != 0o700 {
		t.Fatalf("MkdirTemp permissions = %v, want 0700", got)
	}
}

// Defect: Create opens a file someone else laid at the path beforehand and
// writes the secret into a file whose rights that someone chose.
func TestCreateRefusesAnExistingFile(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "pgpass")
	if err := os.WriteFile(path, []byte("planted"), 0o666); err != nil {
		t.Fatal(err)
	}
	if _, err := Create(path); !errors.Is(err, os.ErrExist) {
		t.Fatalf("Create over an existing file = %v, want os.ErrExist", err)
	}
	if err := Mkdir(filepath.Dir(path)); !errors.Is(err, os.ErrExist) {
		t.Fatalf("Mkdir over an existing directory = %v, want os.ErrExist", err)
	}
	if content, _ := os.ReadFile(path); string(content) != "planted" {
		t.Fatalf("existing file was changed: %q", content)
	}
}

// Defect: temporary names repeat, so a second secret file lands on the first,
// or the pattern is not kept, or a pattern leads out of the directory.
func TestTemporaryNamesAreFreshAndKeepThePattern(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	seen := map[string]bool{}
	for range 20 {
		file, err := CreateTemp(directory, ".backup-*.partial")
		if err != nil {
			t.Fatal(err)
		}
		name := filepath.Base(file.Name())
		_ = file.Close()
		if seen[name] || !strings.HasPrefix(name, ".backup-") || !strings.HasSuffix(name, ".partial") || len(name) == len(".backup-.partial") {
			t.Fatalf("temporary name %q (seen before: %v)", name, seen[name])
		}
		seen[name] = true
	}
	if _, err := CreateTemp(directory, "../escape-*"); err == nil {
		t.Fatal("pattern with a path separator was accepted")
	}
	if _, err := MkdirTemp(directory, "a/b-*"); err == nil {
		t.Fatal("directory pattern with a path separator was accepted")
	}
}

// Defect: Restrict leaves an existing path open, or makes a directory
// unusable by giving it file permissions.
func TestRestrictClosesExistingPathsToOthers(t *testing.T) {
	t.Parallel()
	directory := filepath.Join(t.TempDir(), "secrets")
	if err := os.Mkdir(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(directory, "secret")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(file, 0o644); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]os.FileMode{directory: 0o700, file: 0o600} {
		if err := Restrict(path); err != nil {
			t.Fatal(err)
		}
		if got := permissions(t, path); got != want {
			t.Fatalf("Restrict(%s) permissions = %v, want %v", filepath.Base(path), got, want)
		}
	}
	if err := Restrict(filepath.Join(directory, "absent")); err == nil {
		t.Fatal("Restrict of an absent path succeeded")
	}
}
