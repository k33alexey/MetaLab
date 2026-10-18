//go:build !windows

package appconfig

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// Defect: a backup folder named in the settings is closed to its group, so
// the service account that reaches it through the group — as install checks —
// can no longer write; or its missing part is not created.
func TestPrepareBackupDirectoryLeavesAConfiguredFolderAsLaid(t *testing.T) {
	shared := filepath.Join(t.TempDir(), "shared")
	if err := os.Mkdir(shared, 0o770); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(shared, 0o770); err != nil {
		t.Fatal(err)
	}
	for directory, want := range map[string]os.FileMode{shared: 0o770, filepath.Join(shared, "a", "b"): 0o700} {
		settings := Default()
		settings.Backups.Directory = directory
		got, err := settings.PrepareBackupDirectory()
		if err != nil || got != directory {
			t.Fatalf("PrepareBackupDirectory() = %q, %v; want %q", got, err, directory)
		}
		information, err := os.Stat(directory)
		if err != nil {
			t.Fatal(err)
		}
		if information.Mode().Perm() != want {
			t.Fatalf("%s rights %o, want %o", directory, information.Mode().Perm(), want)
		}
	}
}

// Defect: the service's own backup folder is created with the rights the
// process happens to have, and the archives of every database are listed
// by other users. On macOS only: elsewhere the folder is the machine's.
func TestPrepareBackupDirectoryClosesTheServiceFolder(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("the service folder is the user's own on macOS only")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	directory, err := Default().PrepareBackupDirectory()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, "Library", "Application Support", "MetaLab", "backups"); directory != want {
		t.Fatalf("PrepareBackupDirectory() = %q, want %q", directory, want)
	}
	information, err := os.Stat(directory)
	if err != nil {
		t.Fatal(err)
	}
	if information.Mode().Perm() != 0o700 {
		t.Fatalf("%s rights %o, want 0700", directory, information.Mode().Perm())
	}
}
