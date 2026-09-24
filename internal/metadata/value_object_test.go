package metadata

import (
	"encoding/json"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

const (
	twoCatalogsFirst  = "7a100000-0000-4000-8000-000000000001"
	twoCatalogsSecond = "7a100000-0000-4000-8000-000000000002"
	twoCatalogsHolder = "7a100000-0000-4000-8000-000000000003"
	twoCatalogsColumn = "7a100000-0000-4000-8000-000000000010"
)

func twoCatalogsProject(t *testing.T) *Catalog {
	t.Helper()
	root := metadataProject(t)
	catalogNamed(t, root, twoCatalogsFirst, "Контрагенты")
	catalogNamed(t, root, twoCatalogsSecond, "Организации")
	writeMetadata(t, root, CatalogKind, twoCatalogsHolder, `format: 1
id: `+twoCatalogsHolder+`
name: Договоры
title: {ru: Договоры}
code: {type: string, length: 9, auto: true}
description_length: 150
attributes:
  - id: `+twoCatalogsColumn+`
    name: Сторона
    title: {ru: Сторона}
    types:
      - {kind: catalog, reference: `+twoCatalogsFirst+`}
      - {kind: catalog, reference: `+twoCatalogsSecond+`}
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	return catalog
}

func catalogIDNamed(t *testing.T, catalog *Catalog, name string) (id uuid.UUID) {
	t.Helper()
	for _, item := range catalog.Catalogs {
		if item.Name == name {
			return item.ID
		}
	}
	t.Fatalf("no catalog named %s", name)
	return
}

// A composite column holds references to two catalogs. Which of them a stored
// value came from used to be decided by whichever type of the description
// matched by kind first - so an item of the second catalog was handed back as
// an item of the first. Nothing refused, nothing warned: the wrong object came
// back wearing the right shape.
func TestCompositeReferenceKeepsTheObjectItCameFrom(t *testing.T) {
	t.Parallel()
	catalog := twoCatalogsProject(t)
	second := catalogIDNamed(t, catalog, "Организации")
	item := mustUUID(t, "7a100000-0000-4000-8000-0000000000aa")

	var types []Type
	for _, definition := range catalog.Catalogs {
		if definition.Name == "Договоры" {
			types = definition.Attributes[0].Types
		}
	}
	normalized, err := catalog.normalizeTypes("attribute Сторона", types, Value{Kind: CatalogType, Data: item.String(), Object: second})
	if err != nil {
		t.Fatal(err)
	}
	if normalized.Object != second {
		t.Fatalf("the value was resolved against another catalog: %+v", normalized)
	}

	// And it survives the round trip through storage, because that is where it
	// was being lost.
	storage, err := catalog.attributeStorage(types)
	if err != nil {
		t.Fatal(err)
	}
	if !storage.composite {
		t.Fatal("two catalogs must share one composite column")
	}
	encoded, err := databaseAttributeValue(storage, normalized)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := decodeDatabaseAttribute(storage, json.RawMessage(encoded.(string)))
	if err != nil {
		t.Fatal(err)
	}
	if restored.Object != second || restored.Data != item.String() {
		t.Fatalf("the object was lost in storage: %+v", restored)
	}
}

// A value naming an object the description does not allow is refused, and a
// value naming no object at all is refused rather than guessed at.
func TestReferenceValueMustNameAnAllowedObject(t *testing.T) {
	t.Parallel()
	catalog := twoCatalogsProject(t)
	item := mustUUID(t, "7a100000-0000-4000-8000-0000000000aa")
	first := catalogIDNamed(t, catalog, "Контрагенты")
	types := []Type{{Kind: CatalogType, Reference: &first}}

	if _, err := catalog.normalizeTypes("attribute Сторона", types, Value{Kind: CatalogType, Data: item.String()}); err == nil {
		t.Fatal("a reference that names no object was accepted")
	}
	second := catalogIDNamed(t, catalog, "Организации")
	if _, err := catalog.normalizeTypes("attribute Сторона", types, Value{Kind: CatalogType, Data: item.String(), Object: second}); err == nil {
		t.Fatal("a reference to a catalog the attribute does not allow was accepted")
	}
	outside := mustUUID(t, "7a100000-0000-4000-8000-0000000000bb")
	if _, err := catalog.normalizeTypes("attribute Сторона", []Type{{Kind: CatalogType, Reference: &outside}},
		Value{Kind: CatalogType, Data: item.String(), Object: outside}); err == nil {
		t.Fatal("a reference to an object outside the configuration was accepted")
	}
}

// A column declared by one object does not repeat that object in every row -
// it is restored from the description on read, and a value that carries no
// object is still spelled the way it always was.
func TestSingleTypedColumnRestoresItsObject(t *testing.T) {
	t.Parallel()
	catalog := twoCatalogsProject(t)
	first := catalogIDNamed(t, catalog, "Контрагенты")
	storage, err := catalog.attributeStorage([]Type{{Kind: CatalogType, Reference: &first}})
	if err != nil {
		t.Fatal(err)
	}
	if storage.composite {
		t.Fatal("one named catalog must keep its own column")
	}
	item := mustUUID(t, "7a100000-0000-4000-8000-0000000000aa")
	encoded, err := json.Marshal(item.String())
	if err != nil {
		t.Fatal(err)
	}
	restored, err := decodeDatabaseAttribute(storage, encoded)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Object != first {
		t.Fatalf("the column did not restore the object it is declared by: %+v", restored)
	}
	plain, err := json.Marshal(Value{Kind: StringType, Data: "текст"})
	if err != nil {
		t.Fatal(err)
	}
	if string(plain) != `{"kind":"string","data":"текст"}` {
		t.Fatalf("a value without an object changed how it is stored: %s", plain)
	}
}
