package metadata

import (
	"testing"

	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// A migration plan names a table by its PostgreSQL name, and the developer
// reads it by the object's: the table of a catalog and of its table part are
// both translated.
//
// Defect caught: the tables left out of the translation, so a plan shows
// t_<uuid> where it could say Справочник.Товары.
func TestPhysicalNamesTranslateTables(t *testing.T) {
	t.Parallel()
	goods, part := uuid.MustNew(), uuid.MustNew()
	catalog := &Catalog{Catalogs: []CatalogDefinition{{ID: goods, Name: "Товары", TableParts: []TablePart{{ID: part, Name: "Состав"}}}}}
	names := catalog.PhysicalNames()
	for id, want := range map[uuid.UUID]string{goods: "Справочник.Товары", part: "Справочник.Товары.Состав"} {
		table, err := schemadiff.TableName(id)
		if err != nil {
			t.Fatal(err)
		}
		if names[table] != want {
			t.Fatalf("%s translated as %q, want %q", table, names[table], want)
		}
	}
}
