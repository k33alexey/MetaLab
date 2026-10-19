package metadata

import (
	"context"
	"testing"
	"time"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// A fixed code is padded to its width, and that is not a detail of storage: it
// is what the setting means, and a reader of the code gets the padded value
// because in the prototype the padded value is the code. A variable code of the
// same width comes back as it was typed.
//
// The second half is the trap this point walked into. Automatic numbering finds
// the highest code in use by matching the column against ^[0-9]+$ and taking
// the maximum. PostgreSQL pads character(n) physically, and a regular
// expression applied to that column sees the padding: "41" in a column nine
// wide fails the match, the highest code in use becomes invisible, and the next
// code starts again from one - onto a row that is already there. Checked
// against the real database, because guessing at bpchar semantics is exactly
// how this would have shipped broken.
func TestFixedLengthCodeIsPaddedAndStillNumberedIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool := integrationDatabase(t, ctx, 0)
	projectID := uuid.MustNew()
	fixedID, variableID := uuid.MustNew(), uuid.MustNew()

	catalog := &Catalog{
		Catalogs: []CatalogDefinition{
			{
				ID: fixedID, Name: "Фиксированные", DescriptionLength: 100,
				Code: CatalogCode{Type: StringType, Length: 9, Auto: true, Unique: true, FixedLength: true},
			},
			{
				ID: variableID, Name: "Переменные", DescriptionLength: 100,
				Code: CatalogCode{Type: StringType, Length: 9, Auto: true, Unique: true},
			},
		},
		catalogByName: map[string]int{"фиксированные": 0, "переменные": 1},
		catalogByID:   map[uuid.UUID]int{fixedID: 0, variableID: 1},
	}
	migrateIntegrationSchema(t, ctx, pool, catalog, projectID)
	// Automatic codes are allocated through ml_core.object_sequences, which is
	// not part of the application schema and has to be there before the first
	// one is asked for.
	repository, err := NewCatalogRepository(pool, catalog)
	if err != nil {
		t.Fatal(err)
	}

	// A code typed shorter than the width: padded in the fixed catalog, kept as
	// typed in the variable one.
	for name, want := range map[string]struct {
		catalog string
		code    string
	}{
		"фиксированный код дополняется":    {"фиксированные", "41       "},
		"переменный код остаётся как есть": {"переменные", "41"},
	} {
		t.Run(name, func(t *testing.T) {
			record, err := repository.New(ctx, want.catalog, nil)
			if err != nil {
				t.Fatal(err)
			}
			record.Code, record.Description = "41", "Сорок один"
			if err := repository.Save(ctx, record, nil); err != nil {
				t.Fatal(err)
			}
			loaded, err := repository.Get(ctx, record.Reference)
			if err != nil {
				t.Fatal(err)
			}
			if loaded.Code != want.code {
				t.Fatalf("code read back as %q, want %q", loaded.Code, want.code)
			}
		})
	}

	// Now the numbering. The fixed catalog holds "41" padded to nine; the next
	// automatic code has to be 42, not 1, or it lands on the row already there.
	record, err := repository.New(ctx, "фиксированные", nil)
	if err != nil {
		t.Fatal(err)
	}
	record.Description = "Следующий"
	if err := repository.Save(ctx, record, nil); err != nil {
		t.Fatalf("automatic code after a padded one: %v", err)
	}
	if record.Code != "000000042" {
		t.Fatalf("automatic code = %q, want 000000042: the padded code was invisible to the scan", record.Code)
	}
}
