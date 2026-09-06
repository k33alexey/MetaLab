package metadata

import (
	"testing"

	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestCatalogApplicationSchemaUsesStableUUIDNames(t *testing.T) {
	t.Parallel()
	catalogID := parseTestUUID(t, "40000000-0000-4000-8000-000000000001")
	attributeID := parseTestUUID(t, "40000000-0000-4000-8000-000000000002")
	partID := parseTestUUID(t, "40000000-0000-4000-8000-000000000003")
	partAttributeID := parseTestUUID(t, "40000000-0000-4000-8000-000000000004")
	catalog := &Catalog{
		Catalogs: []CatalogDefinition{{
			ID: catalogID, Name: "Контрагенты",
			Code: CatalogCode{Type: StringType, Length: 9, Unique: true}, DescriptionLength: 250,
			Attributes: []Attribute{{
				ID: attributeID, Name: "Родитель", Indexed: true,
				Types: []Type{{Kind: CatalogType, Reference: &catalogID}},
			}},
			TableParts: []TablePart{{
				ID: partID, Name: "Контакты", Attributes: []Attribute{{
					ID: partAttributeID, Name: "Телефон", Required: true,
					Types: []Type{{Kind: StringType, Length: 32}},
				}},
			}},
		}},
		catalogByID: map[uuid.UUID]int{catalogID: 0},
	}
	schema, err := catalog.ApplicationSchema()
	if err != nil {
		t.Fatal(err)
	}
	if schema.Name != schemadiff.ApplicationSchema || len(schema.Tables) != 2 {
		t.Fatalf("schema = %+v", schema)
	}
	mainName, _ := PhysicalCatalogTable(catalogID)
	attributeName, _ := PhysicalAttributeColumn(attributeID)
	main := schemaTable(t, schema, mainName)
	if !hasSchemaColumn(main, "ref", "uuid", false) || !hasSchemaColumn(main, attributeName, "uuid", true) {
		t.Fatalf("main table = %+v", main)
	}
	if len(main.Indexes) != 1 || main.Indexes[0].Keys[0] != attributeName || len(main.Constraints) != 3 {
		t.Fatalf("main indexes/constraints = %+v / %+v", main.Indexes, main.Constraints)
	}
	partName, _ := PhysicalCatalogTable(partID)
	part := schemaTable(t, schema, partName)
	if !hasSchemaColumn(part, "owner_ref", "uuid", false) || !hasSchemaColumn(part, "line_no", "integer", false) {
		t.Fatalf("table part = %+v", part)
	}
}

func TestCompositeAttributeUsesJSONB(t *testing.T) {
	t.Parallel()
	catalog := &Catalog{}
	storage, err := catalog.attributeStorage([]Type{{Kind: StringType}, {Kind: BooleanType}})
	if err != nil || storage.sqlType != "jsonb" || !storage.composite {
		t.Fatalf("storage=%+v error=%v", storage, err)
	}
}

func TestNonUniqueCatalogCodeIsIndexed(t *testing.T) {
	t.Parallel()
	catalogID := parseTestUUID(t, "40000000-0000-4000-8000-000000000010")
	catalog := &Catalog{Catalogs: []CatalogDefinition{{
		ID: catalogID, Name: "Теги", Code: CatalogCode{Type: StringType, Length: 9}, DescriptionLength: 100,
	}}}
	schema, err := catalog.ApplicationSchema()
	if err != nil {
		t.Fatal(err)
	}
	tableName, _ := PhysicalCatalogTable(catalogID)
	table := schemaTable(t, schema, tableName)
	if len(table.Indexes) != 1 || len(table.Indexes[0].Keys) != 1 || table.Indexes[0].Keys[0] != "code" {
		t.Fatalf("code indexes = %+v", table.Indexes)
	}
}

func schemaTable(t *testing.T, schema schemadiff.Schema, name string) schemadiff.Table {
	t.Helper()
	for _, table := range schema.Tables {
		if table.Name == name {
			return table
		}
	}
	t.Fatalf("table %s not found", name)
	return schemadiff.Table{}
}

func hasSchemaColumn(table schemadiff.Table, name, sqlType string, nullable bool) bool {
	for _, column := range table.Columns {
		if column.Name == name && column.Type == sqlType && column.Nullable == nullable {
			return true
		}
	}
	return false
}

func parseTestUUID(t *testing.T, value string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
