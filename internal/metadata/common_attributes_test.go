package metadata

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

func commonAttributeFixture(objects ...uuid.UUID) CommonAttributeDefinition {
	return CommonAttributeDefinition{
		Format: CurrentFormat, ID: uuid.MustNew(), Name: "ОтветственныйМенеджер", Title: LocalizedText{"ru": "Ответственный менеджер"},
		Types: []Type{{Kind: StringType, Length: 100}}, Objects: objects,
	}
}

func TestDecodeCommonAttributeStrictAndBounded(t *testing.T) {
	t.Parallel()
	attribute := commonAttributeFixture(uuid.MustNew(), uuid.MustNew())
	var encoded bytes.Buffer
	if err := Encode(&encoded, attribute); err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeCommonAttribute("common-attribute.yaml", bytes.NewReader(encoded.Bytes()), metadataManifest())
	if err != nil || decoded.ID != attribute.ID || len(decoded.Objects) != 2 {
		t.Fatalf("decode common attribute = %+v, %v", decoded, err)
	}
	tests := map[string]func(*CommonAttributeDefinition){
		"format":        func(a *CommonAttributeDefinition) { a.Format++ },
		"zero identity": func(a *CommonAttributeDefinition) { a.ID = uuid.UUID{} },
		"name":          func(a *CommonAttributeDefinition) { a.Name = "Invalid Name" },
		"language":      func(a *CommonAttributeDefinition) { a.Title = LocalizedText{"de": "Verantwortlich"} },
		"no objects":    func(a *CommonAttributeDefinition) { a.Objects = nil },
		"zero object":   func(a *CommonAttributeDefinition) { a.Objects = append(a.Objects, uuid.UUID{}) },
		"duplicate object": func(a *CommonAttributeDefinition) {
			a.Objects = append(a.Objects, a.Objects[0])
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			mutated := attribute
			mutated.Objects = append([]uuid.UUID{}, attribute.Objects...)
			mutate(&mutated)
			if err := ValidateCommonAttribute("common-attribute.yaml", mutated, metadataManifest()); err == nil {
				t.Fatalf("%s: accepted invalid common attribute", name)
			}
		})
	}
	if _, err := DecodeCommonAttribute("common-attribute.yaml", strings.NewReader(encoded.String()+"unknown: true\n"), metadataManifest()); err == nil {
		t.Fatal("accepted unknown field")
	}
}

func TestCatalogCommonAttributeLookupReturnsIsolatedCopies(t *testing.T) {
	t.Parallel()
	target := CatalogDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "Товары", Title: LocalizedText{"ru": "Товары"},
		Code: CatalogCode{Type: StringType, Length: 9}, DescriptionLength: 100}
	attribute := commonAttributeFixture(target.ID)
	catalog, err := NewCatalogSnapshotWithCommonAttributes(metadataManifest(), nil, nil, nil, []CatalogDefinition{target}, nil, nil, nil, nil, nil, nil, []CommonAttributeDefinition{attribute})
	if err != nil {
		t.Fatal(err)
	}
	byName, ok := catalog.CommonAttribute("ОтветственныйМенеджер")
	if !ok || byName.ID != attribute.ID {
		t.Fatalf("common attribute by name = %+v, %v", byName, ok)
	}
	byID, ok := catalog.CommonAttributeByID(attribute.ID)
	if !ok || byID.Name != "ОтветственныйМенеджер" {
		t.Fatalf("common attribute by id = %+v, %v", byID, ok)
	}
	byID.Objects[0] = uuid.MustNew()
	again, _ := catalog.CommonAttributeByID(attribute.ID)
	if again.Objects[0] != target.ID {
		t.Fatal("common attribute lookup exposed mutable metadata")
	}
}

func TestPropagateCommonAttributesAcrossObjectKinds(t *testing.T) {
	t.Parallel()
	catalogTarget := CatalogDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "Товары", Title: LocalizedText{"ru": "Товары"},
		Code: CatalogCode{Type: StringType, Length: 9}, DescriptionLength: 100}
	documentTarget := DocumentDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "Продажа", Title: LocalizedText{"ru": "Продажа"},
		Number: DocumentNumber{Type: StringType, Length: 9}}
	attribute := commonAttributeFixture(catalogTarget.ID, documentTarget.ID)

	catalog, err := NewCatalogSnapshotWithCommonAttributes(metadataManifest(), nil, nil, nil,
		[]CatalogDefinition{catalogTarget}, []DocumentDefinition{documentTarget}, nil, nil, nil, nil, nil, []CommonAttributeDefinition{attribute})
	if err != nil {
		t.Fatal(err)
	}
	propagatedCatalog, ok := catalog.CatalogDefinition("Товары")
	if !ok || len(propagatedCatalog.Attributes) != 1 || propagatedCatalog.Attributes[0].ID != attribute.ID || propagatedCatalog.Attributes[0].Name != attribute.Name {
		t.Fatalf("catalog did not receive the propagated attribute: %+v", propagatedCatalog.Attributes)
	}
	propagatedDocument, ok := catalog.DocumentDefinition("Продажа")
	if !ok || len(propagatedDocument.Attributes) != 1 || propagatedDocument.Attributes[0].ID != attribute.ID {
		t.Fatalf("document did not receive the propagated attribute: %+v", propagatedDocument.Attributes)
	}

	// indexAndValidate may run again on already-propagated data (RuntimeSnapshot
	// round-trips reconstruct a Catalog from already-validated definitions);
	// propagation must be idempotent, not duplicate the field on each pass.
	again, err := NewCatalogSnapshotWithCommonAttributes(metadataManifest(), nil, nil, nil,
		catalog.Catalogs, catalog.Documents, nil, nil, nil, nil, nil, catalog.CommonAttributes)
	if err != nil {
		t.Fatal(err)
	}
	revalidatedCatalog, _ := again.CatalogDefinition("Товары")
	if len(revalidatedCatalog.Attributes) != 1 {
		t.Fatalf("propagation duplicated the field across revalidation: %+v", revalidatedCatalog.Attributes)
	}
}

func TestPropagateCommonAttributesRejectsInvalidTargets(t *testing.T) {
	t.Parallel()
	t.Run("unknown object", func(t *testing.T) {
		attribute := commonAttributeFixture(uuid.MustNew())
		if _, err := NewCatalogSnapshotWithCommonAttributes(metadataManifest(), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, []CommonAttributeDefinition{attribute}); err == nil || !strings.Contains(err.Error(), "unknown object") {
			t.Fatalf("unknown object error = %v", err)
		}
	})
	t.Run("name collision", func(t *testing.T) {
		target := CatalogDefinition{
			Format: CurrentFormat, ID: uuid.MustNew(), Name: "Товары", Title: LocalizedText{"ru": "Товары"},
			Code: CatalogCode{Type: StringType, Length: 9}, DescriptionLength: 100,
			Attributes: []Attribute{{ID: uuid.MustNew(), Name: "ОтветственныйМенеджер", Title: LocalizedText{"ru": "Ответственный"}, Types: []Type{{Kind: StringType, Length: 50}}}},
		}
		attribute := commonAttributeFixture(target.ID)
		if _, err := NewCatalogSnapshotWithCommonAttributes(metadataManifest(), nil, nil, nil, []CatalogDefinition{target}, nil, nil, nil, nil, nil, nil, []CommonAttributeDefinition{attribute}); err == nil || !strings.Contains(err.Error(), "collides") {
			t.Fatalf("name collision error = %v", err)
		}
	})
}

func TestPropagatedCommonAttributeReachesApplicationSchema(t *testing.T) {
	t.Parallel()
	target := CatalogDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "Товары", Title: LocalizedText{"ru": "Товары"},
		Code: CatalogCode{Type: StringType, Length: 9}, DescriptionLength: 100}
	attribute := commonAttributeFixture(target.ID)
	catalog, err := NewCatalogSnapshotWithCommonAttributes(metadataManifest(), nil, nil, nil, []CatalogDefinition{target}, nil, nil, nil, nil, nil, nil, []CommonAttributeDefinition{attribute})
	if err != nil {
		t.Fatal(err)
	}
	schema, err := catalog.ApplicationSchema()
	if err != nil {
		t.Fatal(err)
	}
	tableName, _ := PhysicalCatalogTable(target.ID)
	columnName, _ := PhysicalAttributeColumn(attribute.ID)
	table := schemaTable(t, schema, tableName)
	if !hasSchemaColumn(table, columnName, "character varying(100)", true) {
		t.Fatalf("propagated common attribute did not reach the generated schema: %+v", table.Columns)
	}
}

func TestLoadPropagatesCommonAttributesFromProjectFiles(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	if err := os.MkdirAll(filepath.Join(root, "metadata", string(CommonAttributeKind)), 0o755); err != nil {
		t.Fatal(err)
	}
	catalogTargetID := uuid.MustNew()
	writeMetadata(t, root, CatalogKind, catalogTargetID.String(), "format: 1\nid: "+catalogTargetID.String()+
		"\nname: Товары\ntitle: {ru: Товары}\ncode: {type: string, length: 9}\ndescription_length: 100\n")
	commonAttributeID := uuid.MustNew()
	writeMetadata(t, root, CommonAttributeKind, commonAttributeID.String(), "format: 1\nid: "+commonAttributeID.String()+
		"\nname: ОтветственныйМенеджер\ntitle: {ru: Ответственный менеджер}\ntypes: [{kind: string, length: 100}]\nobjects: ["+catalogTargetID.String()+"]\n")
	catalog, err := load(root, true)
	if err != nil {
		t.Fatalf("Load() with a valid common attribute = %v", err)
	}
	propagated, ok := catalog.CatalogDefinition("Товары")
	if !ok || len(propagated.Attributes) != 1 || propagated.Attributes[0].ID != commonAttributeID {
		t.Fatalf("common attribute was not physically propagated into the catalog: %+v", propagated.Attributes)
	}

	unknownID := uuid.MustNew()
	if err := os.WriteFile(filepath.Join(root, "metadata", string(CommonAttributeKind), commonAttributeID.String()+".yaml"),
		[]byte("format: 1\nid: "+commonAttributeID.String()+
			"\nname: ОтветственныйМенеджер\ntitle: {ru: Ответственный менеджер}\ntypes: [{kind: string, length: 100}]\nobjects: ["+unknownID.String()+"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := load(root, true); err == nil || !strings.Contains(err.Error(), "unknown object") {
		t.Fatalf("common attribute referencing an unknown object was accepted: %v", err)
	}
}
