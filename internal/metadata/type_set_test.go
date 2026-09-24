package metadata

import (
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

const (
	setCatalogOne   = "5e700000-0000-4000-8000-000000000001"
	setCatalogTwo   = "5e700000-0000-4000-8000-000000000002"
	setHolder       = "5e700000-0000-4000-8000-000000000003"
	setChart        = "5e700000-0000-4000-8000-000000000004"
	setHolderColumn = "5e700000-0000-4000-8000-000000000010"
)

func mustUUID(t *testing.T, text string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(text)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func catalogNamed(t *testing.T, root, id, name string) {
	t.Helper()
	writeMetadata(t, root, CatalogKind, id, `format: 1
id: `+id+`
name: `+name+`
title: {ru: `+name+`}
code: {type: string, length: 9, auto: true}
description_length: 150
`)
}

// A set holding exactly one object today is still a set. Giving it that
// object's own column and a foreign key to that object's table would mean
// rebuilding the table the day a second object appears - and a set exists
// precisely so that nothing has to be touched that day.
func TestSetIsStoredAsCompositeEvenWithOneMember(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	catalogNamed(t, root, setCatalogOne, "Контрагенты")
	writeMetadata(t, root, CatalogKind, setHolder, `format: 1
id: `+setHolder+`
name: Задания
title: {ru: Задания}
code: {type: string, length: 9, auto: true}
description_length: 150
attributes:
  - id: `+setHolderColumn+`
    name: Предмет
    title: {ru: Предмет}
    types: [{kind: catalog-ref}]
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	storage, err := catalog.attributeStorage([]Type{{Kind: CatalogSet}})
	if err != nil {
		t.Fatal(err)
	}
	if !storage.composite || storage.sqlType != "jsonb" {
		t.Fatalf("a set with one member was given a column of its own: %+v", storage)
	}
	if storage.referenceObject != nil {
		t.Fatal("a set was given a foreign key to a single table")
	}
	// The same attribute declared by naming that one catalog does get its own
	// column: the difference between the two is the whole mechanism.
	id := catalog.Catalogs[0].ID
	direct, err := catalog.attributeStorage([]Type{{Kind: CatalogType, Reference: &id}})
	if err != nil {
		t.Fatal(err)
	}
	if direct.composite {
		t.Fatalf("a named catalog reference lost its own column: %+v", direct)
	}
}

// What a set contains is read off the configuration every time it is asked,
// never written down. An object added today is in the set today, without
// anybody reopening the attribute that uses it.
func TestSetWidensWithTheConfiguration(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	catalogNamed(t, root, setCatalogOne, "Контрагенты")
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	before, err := catalog.expandTypes([]Type{{Kind: CatalogSet}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != 1 {
		t.Fatalf("the set did not contain the one catalog there is: %+v", before)
	}

	catalogNamed(t, root, setCatalogTwo, "Организации")
	catalog, err = Load(root)
	if err != nil {
		t.Fatal(err)
	}
	after, err := catalog.expandTypes([]Type{{Kind: CatalogSet}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 2 {
		t.Fatalf("a catalog added to the project did not fall into the set: %+v", after)
	}
	// And the new catalog is accepted where the set is declared, which is what
	// "falls into the set" has to mean in practice.
	for _, item := range catalog.Catalogs {
		id := item.ID
		allowed, err := catalog.allowsObjectReference([]Type{{Kind: CatalogSet}}, CatalogType, id)
		if err != nil || !allowed {
			t.Fatalf("catalog %s is not in the set of all catalogs: %v", item.Name, err)
		}
	}
}

// Any reference means any: every reference kind the platform has, route points
// included, and one member per business process for them.
func TestAnyReferenceCoversEveryReferenceKind(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	catalogNamed(t, root, setCatalogOne, "Контрагенты")
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[TypeKind]bool{}
	for kind := range referenceSets {
		if kind == AnyReferenceSet {
			continue
		}
		for _, member := range catalog.setMembers(kind) {
			kinds[member.Kind] = true
		}
	}
	any := map[TypeKind]bool{}
	for _, member := range catalog.setMembers(AnyReferenceSet) {
		any[member.Kind] = true
	}
	for kind := range kinds {
		if !any[kind] {
			t.Fatalf("any reference does not cover %s", kind)
		}
	}
}

// A set and a concrete type are elements of one description and mix freely:
// the reference export carries dimensions declared by six sets at once and
// attributes declared by any reference together with a boolean, a string, a
// date and a number.
func TestSetsMixWithConcreteTypes(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	catalogNamed(t, root, setCatalogOne, "Контрагенты")
	writeMetadata(t, root, CatalogKind, setHolder, `format: 1
id: `+setHolder+`
name: Значения
title: {ru: Значения}
code: {type: string, length: 9, auto: true}
description_length: 150
attributes:
  - id: `+setHolderColumn+`
    name: Значение
    title: {ru: Значение}
    types:
      - {kind: any-ref}
      - {kind: boolean}
      - {kind: string, length: 100}
      - {kind: date, date_parts: date}
      - {kind: number, precision: 10, scale: 2}
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	var holder CatalogDefinition
	for _, item := range catalog.Catalogs {
		if item.Name == "Значения" {
			holder = item
		}
	}
	if holder.Name == "" {
		t.Fatal("the catalog did not load")
	}
	if len(holder.Attributes[0].Types) != 5 {
		t.Fatalf("the description lost elements: %+v", holder.Attributes[0].Types)
	}
	if _, err := catalog.normalizeTypes("attribute Значение", holder.Attributes[0].Types, Value{Kind: BooleanType, Data: "true"}); err != nil {
		t.Fatalf("a concrete member of a mixed description was refused: %v", err)
	}
	// An item of a catalog the set covers, named by the catalog it belongs to.
	item := mustUUID(t, "5e700000-0000-4000-8000-0000000000aa")
	for _, subject := range catalog.Catalogs {
		if subject.Name != "Контрагенты" {
			continue
		}
		if _, err := catalog.normalizeTypes("attribute Значение", holder.Attributes[0].Types,
			Value{Kind: CatalogType, Data: item.String(), Object: subject.ID}); err != nil {
			t.Fatalf("a reference covered by the set was refused: %v", err)
		}
	}
	// And a reference naming an object the configuration does not have is
	// refused by the registry, whatever the set would otherwise allow.
	if _, err := catalog.normalizeTypes("attribute Значение", holder.Attributes[0].Types,
		Value{Kind: CatalogType, Data: item.String(), Object: mustUUID(t, "5e700000-0000-4000-8000-0000000000bb")}); err == nil {
		t.Fatal("a reference to an object outside the configuration was accepted")
	}
}

// A characteristic set is read off its chart, and a chart is free to allow
// characteristics of another chart. What must never happen is a walk that does
// not end.
func TestCharacteristicSetFollowsItsChart(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	catalogNamed(t, root, setCatalogOne, "Контрагенты")
	writeMetadata(t, root, ChartOfCharacteristicTypesKind, setChart, `format: 1
id: `+setChart+`
name: ВидыСвойств
title: {ru: Виды свойств}
code: {type: string, length: 9, auto: true}
description_length: 150
value_type:
  - {kind: string, length: 50}
  - {kind: catalog, reference: `+setCatalogOne+`}
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	chart := catalog.ChartsOfCharacteristicTypes[0].ID
	resolved, err := catalog.expandTypes([]Type{{Kind: CharacteristicSet, Reference: &chart}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(resolved) != 2 {
		t.Fatalf("the set did not take the value types of its chart: %+v", resolved)
	}
}

// A description that cannot be read is refused where it is written, not where
// it is used.
func TestTypeSetsRefuseDescriptionsThatMeanNothing(t *testing.T) {
	t.Parallel()
	for name, types := range map[string]string{
		"набор с лишней ссылкой":        `[{kind: catalog-ref, reference: ` + setCatalogOne + `}]`,
		"характеристика без плана":      `[{kind: characteristic}]`,
		"характеристика чужого объекта": `[{kind: characteristic, reference: ` + setCatalogOne + `}]`,
		"набор с квалификатором":        `[{kind: any-ref, length: 10}]`,
		"набор дважды":                  `[{kind: catalog-ref}, {kind: catalog-ref}]`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			catalogNamed(t, root, setCatalogOne, "Контрагенты")
			writeMetadata(t, root, CatalogKind, setHolder, `format: 1
id: `+setHolder+`
name: Значения
title: {ru: Значения}
code: {type: string, length: 9, auto: true}
description_length: 150
attributes:
  - id: `+setHolderColumn+`
    name: Значение
    title: {ru: Значение}
    types: `+types+`
`)
			if _, err := Load(root); err == nil {
				t.Fatal("a description that means nothing was accepted")
			}
		})
	}
}

// A chart whose characteristics are typed by its own characteristics describes
// nothing at all, and the refusal comes at the chart, not at whoever uses it.
func TestChartCannotBeTypedByItsOwnCharacteristics(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, ChartOfCharacteristicTypesKind, setChart, `format: 1
id: `+setChart+`
name: ВидыСвойств
title: {ru: Виды свойств}
code: {type: string, length: 9, auto: true}
description_length: 150
value_type:
  - {kind: characteristic, reference: `+setChart+`}
`)
	if _, err := Load(root); err == nil {
		t.Fatal("a chart typed by itself was accepted")
	}
}
