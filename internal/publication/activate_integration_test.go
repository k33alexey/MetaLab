package publication

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestAtomicPublicationActivationIntegration(t *testing.T) {
	databaseURL := os.Getenv("ML_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("ML_TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	schemaName := fmt.Sprintf("ml_publish_%x", time.Now().UnixNano())
	quotedSchema := pgx.Identifier{schemaName}.Sanitize()
	if _, err := pool.Exec(ctx, `DROP TABLE IF EXISTS ml_core.publication_state; DROP TABLE IF EXISTS ml_core.publication_versions`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupContext, "DROP SCHEMA IF EXISTS "+quotedSchema+" CASCADE")
		_, _ = pool.Exec(cleanupContext, `DROP TABLE IF EXISTS ml_core.publication_state; DROP TABLE IF EXISTS ml_core.publication_versions`)
		_, _ = pool.Exec(cleanupContext, `DROP FUNCTION IF EXISTS ml_core.reject_test_publication()`)
		pool.Close()
	})

	desired := schemadiff.Schema{Name: schemaName, Exists: true, Tables: []schemadiff.Table{{
		Name:        "t_demo",
		Columns:     []schemadiff.Column{{Name: "id", Type: "uuid", Nullable: false}},
		Constraints: []schemadiff.Constraint{{Name: "k_demo_primary", Type: "primary_key", Definition: "PRIMARY KEY (id)"}},
	}}}
	root := publicationProject(t)
	firstPackage := buildActivationPackage(t, root, "Первая", repeatedCommit('1'), false)
	firstPlan, err := schemadiff.Prepare(ctx, pool, desired)
	if err != nil {
		t.Fatal(err)
	}
	first, firstMigration, err := Activate(ctx, pool, ActivationRequest{
		PackagePath: firstPackage, Desired: desired, Prepared: firstPlan,
		ExpectedGitCommit: repeatedCommit('1'), Mode: ActivationPrimary, Confirmed: true,
	})
	if err != nil || first.Generation != 1 || firstMigration.Status != "succeeded" || first.MigrationID != firstMigration.ID {
		t.Fatalf("first=%+v migration=%+v error=%v", first, firstMigration, err)
	}
	current, found, err := Current(ctx, pool)
	if err != nil || !found || current.ID != first.ID || current.PackageSHA256 != first.PackageSHA256 {
		t.Fatalf("current=%+v found=%v error=%v", current, found, err)
	}

	secondPackage := buildActivationPackage(t, root, "Вторая", repeatedCommit('2'), false)
	secondPlan, err := schemadiff.Prepare(ctx, pool, desired)
	if err != nil {
		t.Fatal(err)
	}
	secondRequest := ActivationRequest{
		PackagePath: secondPackage, Desired: desired, Prepared: secondPlan,
		ExpectedGitCommit: repeatedCommit('2'), ExpectedActivePackageSHA256: first.PackageSHA256,
		Mode: ActivationPrimary, Confirmed: true,
	}
	type result struct {
		active ActiveVersion
		err    error
	}
	start := make(chan struct{})
	results := make(chan result, 2)
	for range 2 {
		go func() {
			<-start
			active, _, activateErr := Activate(ctx, pool, secondRequest)
			results <- result{active: active, err: activateErr}
		}()
	}
	close(start)
	var second ActiveVersion
	succeeded, rejected := 0, 0
	for range 2 {
		item := <-results
		if item.err == nil {
			succeeded++
			second = item.active
		} else if errors.Is(item.err, ErrAlreadyActive) || errors.Is(item.err, ErrStalePublication) {
			rejected++
		} else {
			t.Fatalf("concurrent Activate() error = %v", item.err)
		}
	}
	if succeeded != 1 || rejected != 1 || second.Generation != 2 {
		t.Fatalf("concurrent activation succeeded=%d rejected=%d second=%+v", succeeded, rejected, second)
	}

	dirtyPackage := buildActivationPackage(t, root, "Отладочная", repeatedCommit('3'), true)
	dirtyPlan, err := schemadiff.Prepare(ctx, pool, desired)
	if err != nil {
		t.Fatal(err)
	}
	dirtyRequest := ActivationRequest{
		PackagePath: dirtyPackage, Desired: desired, Prepared: dirtyPlan,
		ExpectedGitCommit: repeatedCommit('3'), ExpectedActivePackageSHA256: second.PackageSHA256,
		Mode: ActivationPrimary, Confirmed: true,
	}
	if _, _, err := Activate(ctx, pool, dirtyRequest); !errors.Is(err, ErrDirtyPrimary) {
		t.Fatalf("dirty primary Activate() error = %v", err)
	}
	dirtyRequest.Mode = ActivationDebug
	debugVersion, _, err := Activate(ctx, pool, dirtyRequest)
	if err != nil || debugVersion.Generation != 3 || !debugVersion.Dirty {
		t.Fatalf("debug=%+v error=%v", debugVersion, err)
	}

	if _, err := pool.Exec(ctx, `
CREATE OR REPLACE FUNCTION ml_core.reject_test_publication() RETURNS trigger
LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'test activation failure'; END $$;
CREATE TRIGGER reject_test_publication BEFORE INSERT ON ml_core.publication_versions
FOR EACH ROW EXECUTE FUNCTION ml_core.reject_test_publication()`); err != nil {
		t.Fatal(err)
	}
	changedDesired := desired
	changedDesired.Tables = append([]schemadiff.Table(nil), desired.Tables...)
	changedDesired.Tables[0].Columns = append([]schemadiff.Column(nil), desired.Tables[0].Columns...)
	changedDesired.Tables[0].Columns = append(changedDesired.Tables[0].Columns,
		schemadiff.Column{Name: "atomic_test", Type: "text", Nullable: true})
	rollbackPackage := buildActivationPackage(t, root, "Откат", repeatedCommit('4'), false)
	rollbackPlan, err := schemadiff.Prepare(ctx, pool, changedDesired)
	if err != nil {
		t.Fatal(err)
	}
	_, failedMigration, err := Activate(ctx, pool, ActivationRequest{
		PackagePath: rollbackPackage, Desired: changedDesired, Prepared: rollbackPlan,
		ExpectedGitCommit: repeatedCommit('4'), ExpectedActivePackageSHA256: debugVersion.PackageSHA256,
		Mode: ActivationPrimary, Confirmed: true,
	})
	if err == nil || failedMigration.Status != "failed" {
		t.Fatalf("failed atomic activation migration=%+v error=%v", failedMigration, err)
	}
	var atomicColumnExists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS(
		SELECT 1 FROM information_schema.columns WHERE table_schema = $1 AND table_name = 't_demo' AND column_name = 'atomic_test'
	)`, schemaName).Scan(&atomicColumnExists); err != nil || atomicColumnExists {
		t.Fatalf("failed activation changed schema: exists=%v error=%v", atomicColumnExists, err)
	}
	current, found, err = Current(ctx, pool)
	if err != nil || !found || current.ID != debugVersion.ID || current.Generation != 3 {
		t.Fatalf("failed activation changed active version: current=%+v found=%v error=%v", current, found, err)
	}
	if _, err := pool.Exec(ctx, `
DROP TRIGGER reject_test_publication ON ml_core.publication_versions;
DROP FUNCTION ml_core.reject_test_publication()`); err != nil {
		t.Fatal(err)
	}

	otherRoot := publicationProject(t)
	otherPackage := buildActivationPackage(t, otherRoot, "Другой", repeatedCommit('5'), false)
	otherPlan, err := schemadiff.Prepare(ctx, pool, desired)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := Activate(ctx, pool, ActivationRequest{
		PackagePath: otherPackage, Desired: desired, Prepared: otherPlan,
		ExpectedGitCommit: repeatedCommit('5'), ExpectedActivePackageSHA256: debugVersion.PackageSHA256,
		Mode: ActivationPrimary, Confirmed: true,
	}); !errors.Is(err, ErrProjectMismatch) {
		t.Fatalf("other-project Activate() error = %v", err)
	}

	if _, err := pool.Exec(ctx, "ALTER TABLE "+quotedSchema+`.t_demo ADD COLUMN external text`); err != nil {
		t.Fatal(err)
	}
	fourthPackage := buildActivationPackage(t, root, "Четвёртая", repeatedCommit('6'), false)
	driftPlan, err := schemadiff.Prepare(ctx, pool, desired)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := Activate(ctx, pool, ActivationRequest{
		PackagePath: fourthPackage, Desired: desired, Prepared: driftPlan,
		ExpectedGitCommit: repeatedCommit('6'), ExpectedActivePackageSHA256: debugVersion.PackageSHA256,
		Mode: ActivationPrimary, Confirmed: true, AllowDestructive: true,
	}); !errors.Is(err, ErrSchemaDiverged) {
		t.Fatalf("diverged-schema Activate() error = %v", err)
	}
	var externalExists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS(
		SELECT 1 FROM information_schema.columns WHERE table_schema = $1 AND table_name = 't_demo' AND column_name = 'external'
	)`, schemaName).Scan(&externalExists); err != nil || !externalExists {
		t.Fatalf("divergent column was unexpectedly changed: exists=%v error=%v", externalExists, err)
	}

	history, err := ListVersions(ctx, pool, 10)
	if err != nil || len(history) != 3 || history[0].ID != debugVersion.ID {
		t.Fatalf("history=%+v error=%v", history, err)
	}
}

func buildActivationPackage(t *testing.T, root, source, commit string, dirty bool) string {
	t.Helper()
	moduleID := uuid.MustNew()
	modulePath, err := project.ModulePath(moduleID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(modulePath)), []byte(source+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	packagePath := filepath.Join(t.TempDir(), source+PackageExtension)
	if _, err := BuildFile(context.Background(), root, packagePath, SourceState{GitCommit: commit, Dirty: dirty}); err != nil {
		t.Fatal(err)
	}
	return packagePath
}

func repeatedCommit(symbol byte) string {
	value := make([]byte, 40)
	for index := range value {
		value[index] = symbol
	}
	return string(value)
}
