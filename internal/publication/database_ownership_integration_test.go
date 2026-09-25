package publication

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// A database belongs to one project, and applying another one to it has to be
// refused before the schema is read - not asked about. The plan such a save
// builds says "drop every table and create different ones", which reads as an
// ordinary request to lose objects, and the developer has just pressed save and
// is expecting to confirm something.
func TestSaveDataRefusesADatabaseOfAnotherProjectIntegration(t *testing.T) {
	databaseURL := os.Getenv("ML_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("ML_TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	projectID, err := uuid.Parse("10000000-0000-4000-8000-000000000001")
	if err != nil {
		t.Fatal(err)
	}
	clean := func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupContext, "DROP SCHEMA IF EXISTS "+pgx.Identifier{schemadiff.ApplicationSchema}.Sanitize()+" CASCADE")
		_, _ = pool.Exec(cleanupContext, "DROP TABLE IF EXISTS ml_core.database_state")
		_, _ = pool.Exec(cleanupContext, "DELETE FROM ml_core.migration_journal WHERE project_id = $1", projectID.String())
	}
	t.Cleanup(func() {
		clean()
		pool.Close()
	})
	clean()

	root := demoProjectCopy(t)
	runSaveDataGit(t, root, "init", "-b", "main")
	runSaveDataGit(t, root, "config", "user.name", "MetaLab Test")
	runSaveDataGit(t, root, "config", "user.email", "metalab-test@example.invalid")
	runSaveDataGit(t, root, "add", "--all")
	runSaveDataGit(t, root, "commit", "-m", "Initial project")

	// A free database takes any project, and this save binds it.
	if _, _, err := SaveData(ctx, pool, SaveDataRequest{Root: root, Mode: ActivationPrimary, Confirmed: true}); err != nil {
		t.Fatalf("first save into a free database: %v", err)
	}
	var ownerText, ownerName string
	if err := pool.QueryRow(ctx,
		"SELECT project_id::text, project_name FROM ml_core.database_state WHERE singleton").Scan(&ownerText, &ownerName); err != nil {
		t.Fatal(err)
	}
	if ownerText != projectID.String() || ownerName == "" {
		t.Fatalf("the database was bound to %q (%q)", ownerText, ownerName)
	}

	// Now the same database, a different project: the identifier in the
	// manifest is all that differs.
	other := demoProjectCopy(t)
	manifest := filepath.Join(other, "configuration.yaml")
	content, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	changed := strings.Replace(string(content), projectID.String(), "20000000-0000-4000-8000-000000000001", 1)
	changed = strings.Replace(changed, "name: ПродажиИСклад", "name: ДругойПроект", 1)
	if err := os.WriteFile(manifest, []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	runSaveDataGit(t, other, "init", "-b", "main")
	runSaveDataGit(t, other, "config", "user.name", "MetaLab Test")
	runSaveDataGit(t, other, "config", "user.email", "metalab-test@example.invalid")
	runSaveDataGit(t, other, "add", "--all")
	runSaveDataGit(t, other, "commit", "-m", "Another project")

	_, _, err = SaveData(ctx, pool, SaveDataRequest{Root: other, Mode: ActivationPrimary, Confirmed: true})
	if !errors.Is(err, ErrForeignDatabase) {
		t.Fatalf("a database of another project was accepted: %v", err)
	}
	// The refusal names all three: the project that is open, the project the
	// database belongs to, and the database itself.
	for _, name := range []string{"ДругойПроект", "20000000-0000-4000-8000-000000000001",
		"ПродажиИСклад", projectID.String(), "metalab"} {
		if !strings.Contains(err.Error(), name) {
			t.Fatalf("the refusal does not name %s: %v", name, err)
		}
	}

	// And it changed nothing: the binding is still the first project's.
	if err := pool.QueryRow(ctx,
		"SELECT project_id::text FROM ml_core.database_state WHERE singleton").Scan(&ownerText); err != nil {
		t.Fatal(err)
	}
	if ownerText != projectID.String() {
		t.Fatalf("the refused save rebound the database to %q", ownerText)
	}
}

// A database holding application tables with no record of what was applied to
// it is not free. Accepting it would mean planning to drop tables nobody can
// account for.
func TestSaveDataRefusesOccupiedDatabaseWithoutARecordIntegration(t *testing.T) {
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
	clean := func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupContext, "DROP SCHEMA IF EXISTS "+pgx.Identifier{schemadiff.ApplicationSchema}.Sanitize()+" CASCADE")
		_, _ = pool.Exec(cleanupContext, "DROP TABLE IF EXISTS ml_core.database_state")
	}
	t.Cleanup(func() {
		clean()
		pool.Close()
	})
	clean()

	// One table of ours, and no record of who put it there.
	if _, err := pool.Exec(ctx, "CREATE SCHEMA IF NOT EXISTS "+pgx.Identifier{schemadiff.ApplicationSchema}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		"CREATE TABLE "+pgx.Identifier{schemadiff.ApplicationSchema}.Sanitize()+".t_deadbeefdeadbeefdeadbeefdeadbeef (ref uuid)"); err != nil {
		t.Fatal(err)
	}

	root := demoProjectCopy(t)
	runSaveDataGit(t, root, "init", "-b", "main")
	runSaveDataGit(t, root, "config", "user.name", "MetaLab Test")
	runSaveDataGit(t, root, "config", "user.email", "metalab-test@example.invalid")
	runSaveDataGit(t, root, "add", "--all")
	runSaveDataGit(t, root, "commit", "-m", "Initial project")

	_, _, err = SaveData(ctx, pool, SaveDataRequest{Root: root, Mode: ActivationPrimary, Confirmed: true})
	if !errors.Is(err, ErrForeignDatabase) {
		t.Fatalf("an occupied database was accepted: %v", err)
	}

	// A table that is not ours, kept where the administrator keeps their own,
	// does not make the database occupied. Our schema is a different matter:
	// what lies inside ml_data is ours to manage, and the migration plan deals
	// with it - with consent, like any other loss.
	clean()
	if _, err := pool.Exec(ctx, "CREATE TABLE IF NOT EXISTS public.ml_admin_notes (note text)"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupContext, "DROP TABLE IF EXISTS public.ml_admin_notes")
	})
	if _, _, err := SaveData(ctx, pool, SaveDataRequest{Root: root, Mode: ActivationPrimary, Confirmed: true}); err != nil {
		t.Fatalf("a database holding somebody else's table was refused: %v", err)
	}
}
