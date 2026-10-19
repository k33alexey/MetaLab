package metadata

import (
	"context"
	"testing"
	"time"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// A document of a numerator numbers itself when it asks to: the numerator has
// no automatic numbering of its own, and taking its whole number, as reading
// once did, left every such document without a number for ever. The number
// written is of the numerator's shape - its length, here 11.
func TestADocumentOfANumeratorIsNumberedWhenWrittenIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool := integrationDatabase(t, ctx, 0)
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
	projectID := uuid.MustNew()
	migrateIntegrationSchema(t, ctx, pool, catalog, projectID)
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
