package publication

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestSaveDataIntegration(t *testing.T) {
	databaseURL := os.Getenv("ML_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("ML_TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	// The demo project's own fixed UUID (examples/sales-and-warehouse/configuration.yaml).
	projectID, err := uuid.Parse("10000000-0000-4000-8000-000000000001")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupContext, "DROP SCHEMA IF EXISTS "+pgx.Identifier{schemadiff.ApplicationSchema}.Sanitize()+" CASCADE")
		_, _ = pool.Exec(cleanupContext, "DROP TABLE IF EXISTS ml_core.database_state")
		_, _ = pool.Exec(cleanupContext, "DELETE FROM ml_core.migration_journal WHERE project_id = $1", projectID.String())
		pool.Close()
	})
	if _, err := pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+pgx.Identifier{schemadiff.ApplicationSchema}.Sanitize()+" CASCADE"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "DROP TABLE IF EXISTS ml_core.database_state"); err != nil {
		t.Fatal(err)
	}

	root := demoProjectCopy(t)
	runSaveDataGit(t, root, "init", "-b", "main")
	runSaveDataGit(t, root, "config", "user.name", "MetaLab Test")
	runSaveDataGit(t, root, "config", "user.email", "metalab-test@example.invalid")
	runSaveDataGit(t, root, "add", "--all")
	runSaveDataGit(t, root, "commit", "-m", "Initial project")

	// A dirty Primary-mode project must be rejected before touching PostgreSQL.
	if err := os.WriteFile(filepath.Join(root, "README-dirty.md"), []byte("dirty"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := SaveData(ctx, pool, SaveDataRequest{Root: root, Mode: ActivationPrimary, Confirmed: true}); err != ErrDirtyPrimary {
		t.Fatalf("expected ErrDirtyPrimary for an uncommitted project, got %v", err)
	}
	if err := os.Remove(filepath.Join(root, "README-dirty.md")); err != nil {
		t.Fatal(err)
	}

	saved, migration, err := SaveData(ctx, pool, SaveDataRequest{Root: root, Mode: ActivationPrimary, Confirmed: true})
	if err != nil || migration.Status != "succeeded" || saved.MigrationID != migration.ID || saved.ProjectID != projectID {
		t.Fatalf("save data: saved=%+v migration=%+v err=%v", saved, migration, err)
	}

	snapshot, found, err := CurrentDatabaseState(ctx, pool)
	if err != nil || !found {
		t.Fatalf("current database state: found=%v err=%v", found, err)
	}
	if err := snapshot.Validate(); err != nil {
		t.Fatalf("saved snapshot does not validate: %v", err)
	}
	if len(snapshot.Modules) != 2 {
		t.Fatalf("expected 2 BSL modules saved with the snapshot, got %d", len(snapshot.Modules))
	}
	if len(snapshot.Documents) != 2 || len(snapshot.Catalogs) != 2 || len(snapshot.AccumulationRegisters) != 1 {
		t.Fatalf("saved snapshot structure mismatch: %+v", snapshot)
	}

	// Saving again with nothing changed must succeed and simply refresh state.
	resaved, _, err := SaveData(ctx, pool, SaveDataRequest{Root: root, Mode: ActivationPrimary, Confirmed: true})
	if err != nil || resaved.ProjectID != projectID {
		t.Fatalf("re-save data: %+v %v", resaved, err)
	}
}

func demoProjectCopy(t *testing.T) string {
	t.Helper()
	source, err := filepath.Abs(filepath.Join("..", "..", "examples", "sales-and-warehouse"))
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "project")
	if err := copyDirectory(source, destination); err != nil {
		t.Fatal(err)
	}
	return destination
}

func copyDirectory(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		defer input.Close()
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		output, err := os.Create(target)
		if err != nil {
			return err
		}
		defer output.Close()
		_, err = io.Copy(output, input)
		return err
	})
}

func runSaveDataGit(t *testing.T, root string, arguments ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, arguments...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", arguments, err, output)
	}
}
