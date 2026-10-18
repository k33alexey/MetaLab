//go:build windows

package servicestate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/k33alexey/MetaLab/internal/ownerfile/ownerfiletest"
)

// Defect: the folder of the backups or the secrets keeps the rights of
// ProgramData, where every user may read and create, and the archives of
// every database are listed by them; or one left with those rights by an
// earlier run is reused as it is.
func TestPrepareOwnDirectoryClosesAFolderToOthers(t *testing.T) {
	existing := filepath.Join(t.TempDir(), "backups")
	if err := os.Mkdir(existing, 0o755); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(t.TempDir(), "MetaLab", "backups")
	for _, directory := range []string{existing, missing} {
		if err := PrepareOwnDirectory(directory); err != nil {
			t.Fatal(err)
		}
		ownerfiletest.AssertOwnerOnly(t, directory, true)
	}
}
