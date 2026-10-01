package metadata

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// A document of a numerator numbers itself when it asks to: the numerator has
// no automatic numbering of its own, and taking its whole number, as reading
// once did, left every such document without a number for ever. The number
// written is of the numerator's shape - its length, here 11.
func TestADocumentOfANumeratorIsNumberedWhenWrittenIntegration(t *testing.T) {
	databaseURL := os.Getenv("ML_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("ML_TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	root := aroundDocumentsProject(t)
	writeMetadata(t, root, DocumentKind, aroundFirstDoc, `format: 1
id: `+aroundFirstDoc+`
name: ПоступлениеТоваров
title: {ru: Поступление товаров}
numerator: `+aroundNumerator+`
number: {type: string, length: 9, auto: true, periodicity: none}
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	projectID := uuid.MustNew()
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		_, _ = pool.Exec(cleanup, "DROP SCHEMA IF EXISTS "+pgx.Identifier{schemadiff.ApplicationSchema}.Sanitize()+" CASCADE")
		_, _ = pool.Exec(cleanup, "DELETE FROM ml_core.migration_journal WHERE project_id = $1", projectID.String())
		pool.Close()
	})
	if _, err := pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+pgx.Identifier{schemadiff.ApplicationSchema}.Sanitize()+" CASCADE"); err != nil {
		t.Fatal(err)
	}
	desired, err := catalog.ApplicationSchema()
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := schemadiff.Prepare(ctx, pool, desired)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := schemadiff.Execute(ctx, pool, schemadiff.MigrationRequest{
		ProjectID: projectID, PackageSHA256: repeatCatalogHex('a', 64), GitCommit: repeatCatalogHex('b', 40), Desired: desired,
		ExpectedPlanSHA256: prepared.SHA256, ExpectedSchemaSHA256: prepared.ActualSHA256, Confirmed: true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := EnsureObjectIntegrityStorage(ctx, pool); err != nil {
		t.Fatal(err)
	}
	repository, err := NewDocumentRepository(pool, catalog)
	if err != nil {
		t.Fatal(err)
	}
	record, err := repository.New(ctx, "ПоступлениеТоваров", nil)
	if err != nil {
		t.Fatal(err)
	}
	record.Date = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	if err := repository.Save(ctx, record, nil); err != nil {
		t.Fatal(err)
	}
	if len(record.Number) != 11 {
		t.Fatalf("the document of a numerator got the number %q, want one of the numerator's 11 characters", record.Number)
	}
}
