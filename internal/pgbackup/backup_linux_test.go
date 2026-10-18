//go:build linux

package pgbackup

import (
	"context"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"strconv"
	"syscall"
	"testing"

	"github.com/k33alexey/MetaLab/internal/pgtool"
	"github.com/k33alexey/MetaLab/internal/postgresconn"
	"github.com/k33alexey/MetaLab/internal/servicestate"
)

// Defect: an archive made by an administrator under root belongs to root,
// and the service restoring it under its own account cannot read it. Needs
// root and the service account, as on a server where the service is installed.
func TestLinuxArchiveWrittenByRootBelongsToTheServiceAccount(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("handing an archive to another account needs root")
	}
	account, err := user.Lookup(servicestate.Account)
	if err != nil {
		t.Skipf("service account %q is absent: %v", servicestate.Account, err)
	}
	uid, _ := strconv.Atoi(account.Uid)
	tool := &Tool{tools: pgtool.New("pg_dump", "pg_restore", func(_ context.Context, _ string, arguments, _ []string, _ io.Writer) error {
		return os.WriteFile(arguments[slices.Index(arguments, "--file")+1], []byte("archive-data"), 0o600)
	})}
	connection := postgresconn.Descriptor{
		Host: "localhost", Port: 5432, Database: "sales", User: "ml", SSLMode: "disable", SecretKey: "sales.password",
	}
	directory := t.TempDir()
	archive, err := tool.Create(context.Background(), connection, "secret", directory, "sales.mlbackup")
	if err != nil {
		t.Fatal(err)
	}
	information, err := os.Lstat(filepath.Join(directory, archive.FileName))
	if err != nil {
		t.Fatal(err)
	}
	if owner := information.Sys().(*syscall.Stat_t).Uid; int(owner) != uid || information.Mode().Perm() != 0o600 {
		t.Fatalf("archive: user %d rights %o, want %s (%d) and 0600", owner, information.Mode().Perm(), servicestate.Account, uid)
	}
}
