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
	if !hasSchemaColumn(main, "ref", "uuid", false) || !hasSchemaColumn(main, "deletion_mark", "boolean", false) ||
		!hasSchemaColumn(main, "predefined_name", "character varying(128)", true) || !hasSchemaColumn(main, attributeName, "uuid", true) {
		t.Fatalf("main table = %+v", main)
	}
	if !hasSchemaIndex(main, attributeName) || !hasSchemaIndex(main, "deletion_mark") || len(main.Constraints) != 4 {
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
	if !hasSchemaIndex(table, "code") || !hasSchemaIndex(table, "deletion_mark") {
		t.Fatalf("code indexes = %+v", table.Indexes)
	}
}

func TestDocumentApplicationSchemaContainsLifecycleColumns(t *testing.T) {
	t.Parallel()
	documentID := parseTestUUID(t, "50000000-0000-4000-8000-000000000001")
	attributeID := parseTestUUID(t, "50000000-0000-4000-8000-000000000002")
	partID := parseTestUUID(t, "50000000-0000-4000-8000-000000000003")
	catalog := &Catalog{Documents: []DocumentDefinition{{
		ID: documentID, Name: "Продажа",
		Number:     DocumentNumber{Type: StringType, Length: 11, Unique: true, Periodicity: NumberPeriodYear},
		Attributes: []Attribute{{ID: attributeID, Name: "Комментарий", Types: []Type{{Kind: StringType, Length: 100}}}},
		TableParts: []TablePart{{ID: partID, Name: "Товары"}},
	}}}
	schema, err := catalog.ApplicationSchema()
	if err != nil {
		t.Fatal(err)
	}
	tableName, _ := PhysicalDocumentTable(documentID)
	table := schemaTable(t, schema, tableName)
	if !hasSchemaColumn(table, "number", "character varying(11)", false) ||
		!hasSchemaColumn(table, "number_period", "integer", false) ||
		!hasSchemaColumn(table, "date", "timestamp with time zone", false) ||
		!hasSchemaColumn(table, "posted", "boolean", false) || !hasSchemaColumn(table, "deletion_mark", "boolean", false) {
		t.Fatalf("document columns = %+v", table.Columns)
	}
	if len(table.Constraints) != 2 || !hasSchemaIndex(table, "date DESC") || !hasSchemaIndex(table, "deletion_mark") {
		t.Fatalf("document constraints/indexes = %+v / %+v", table.Constraints, table.Indexes)
	}
	partName, _ := PhysicalDocumentTable(partID)
	if part := schemaTable(t, schema, partName); !hasSchemaColumn(part, "owner_ref", "uuid", false) {
		t.Fatalf("document table part = %+v", part)
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

func hasSchemaIndex(table schemadiff.Table, firstKey string) bool {
	for _, index := range table.Indexes {
		if len(index.Keys) > 0 && index.Keys[0] == firstKey {
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
