package platform

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/k33alexey/MetaLab/internal/appconfig"
	"github.com/k33alexey/MetaLab/internal/ownerfile"
)

// TestStartingRemovesThePasswordsAKilledProcessLeft catches the platform
// starting with the password files of a tool killed mid-copy left open in
// the temporary folder: New removes every such folder whose lock no one
// holds (postgresconn.RemoveAbandonedPasswordFiles), so a password written
// out for pg_dump lives no longer than the process that wrote it.
func TestStartingRemovesThePasswordsAKilledProcessLeft(t *testing.T) {
	temporary := t.TempDir()
	for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(name, temporary)
	}
	left := filepath.Join(temporary, "metalab-pgpass-killed")
	if err := ownerfile.Mkdir(left); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"lock", "pgpass"} {
		file, err := ownerfile.Create(filepath.Join(left, name))
		if err != nil {
			t.Fatal(err)
		}
		_ = file.Close()
	}
	runtime := New(context.Background(), appconfig.Default(), nil)
	defer runtime.Close()
	if _, err := os.Stat(left); !os.IsNotExist(err) {
		t.Fatalf("the password folder a killed process left is still there: %v", err)
	}
}
