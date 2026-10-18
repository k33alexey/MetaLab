//go:build !windows

package servicestate

import (
	"os"
	"path/filepath"
	"testing"
)

// Defect: a folder of the service's own that already existed with wider
// rights is reused as it is, and an archive or a secret in it is listed by
// other users; or one missing with its parent is not created.
func TestPrepareOwnDirectoryClosesAFolderToOthers(t *testing.T) {
	existing := filepath.Join(t.TempDir(), "backups")
	if err := os.Mkdir(existing, 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(existing, 0o777); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(t.TempDir(), "MetaLab", "backups")
	for _, directory := range []string{existing, missing} {
		if err := PrepareOwnDirectory(directory); err != nil {
			t.Fatal(err)
		}
		information, err := os.Lstat(directory)
		if err != nil {
			t.Fatal(err)
		}
		if got := information.Mode().Perm(); !information.IsDir() || got != 0o700 {
			t.Fatalf("%s: directory %v, rights %o, want a directory with 0700", directory, information.IsDir(), got)
		}
	}
}

// Defect: a link planted in place of the folder sends what is written into
// it where the link points, and the target's rights are changed.
func TestPrepareOwnDirectoryRefusesALink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "elsewhere")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "backups")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := PrepareOwnDirectory(link); err == nil {
		t.Fatal("PrepareOwnDirectory() through a link succeeded")
	}
	if information, _ := os.Stat(target); information.Mode().Perm() != 0o755 {
		t.Fatalf("the link's target rights became %o", information.Mode().Perm())
	}
}
