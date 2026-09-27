package metadata

import (
	"strings"
	"testing"
)

const (
	searchCatalogID   = "f1500000-0000-4000-8000-000000000001"
	searchRegisterID  = "f1500000-0000-4000-8000-000000000002"
	searchCommonID    = "f1500000-0000-4000-8000-000000000003"
	searchAttributeID = "f1500000-0000-4000-8000-000000000010"
	searchDimensionID = "f1500000-0000-4000-8000-000000000011"
	searchResourceID  = "f1500000-0000-4000-8000-000000000012"
)

func searchCatalogBody(body string) string {
	return `format: 1
id: ` + searchCatalogID + `
name: Номенклатура
title: {ru: Номенклатура}
code: {type: string, length: 9, auto: true}
description_length: 150
attributes:
  - id: ` + searchAttributeID + `
    name: Артикул
    title: {ru: Артикул}
    types: [{kind: string, length: 20}]
    indexing: index
` + body
}

// The object's own flag, which the field's flag has been waiting for: a field
// is indexed inside an object, and an object outside the index has no inside.
func TestAnObjectSaysWhetherItIsInTheIndex(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]struct {
		value   string
		accepts bool
	}{
		"участвует":    {"use", true},
		"не участвует": {"dont-use", true},
		"третьего нет": {"auto", false},
		"и четвёртого": {"иногда", false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeMetadata(t, root, CatalogKind, searchCatalogID, searchCatalogBody("full_text_search: "+want.value+"\n"))
			catalog, err := Load(root)
			switch {
			case want.accepts && err != nil:
				t.Fatalf("%s: refused: %v", name, err)
			case !want.accepts:
				if err == nil || !strings.Contains(err.Error(), "full_text_search must be use or dont-use") {
					t.Fatalf("%s: %v", name, err)
				}
				return
			}
			definition, _ := catalog.CatalogDefinition("Номенклатура")
			if string(definition.FullTextSearch) != want.value {
				t.Fatalf("the flag was lost: %q", definition.FullTextSearch)
			}
		})
	}
}

// The pair that could not be checked until now. Input by string may be told to
// search the full-text index, and an object that is not in the index finds
// nothing that way - a setting that reads as working and does nothing.
//
// The other pair is left alone on purpose: a field flagged for the index
// inside an object that is not in it is written four hundred and eighty-four
// times in the demonstration configuration, and refusing it would refuse a
// real configuration.
func TestSearchingByAnObjectOutsideTheIndexIsRefused(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]struct {
		body    string
		accepts bool
	}{
		"вне индекса, но подбор ищет индексом": {`full_text_search: dont-use
full_text_search_on_input: use
`, false},
		"вне индекса и подбор индексом не ищет": {`full_text_search: dont-use
full_text_search_on_input: dont-use
`, true},
		"в индексе и ищет": {`full_text_search: use
full_text_search_on_input: use
`, true},
		"вне индекса, а реквизит в индексе — так пишет и выгрузка": {`full_text_search: dont-use
attributes:
  - id: ` + searchDimensionID + `
    name: Описание
    title: {ru: Описание}
    types: [{kind: string, length: 100}]
    full_text_search: use
`, true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			body := want.body
			if strings.Contains(body, "attributes:") {
				// The catalog body already declares one attribute, so this
				// case writes its own object instead of adding to it.
				writeMetadata(t, root, CatalogKind, searchCatalogID, `format: 1
id: `+searchCatalogID+`
name: Номенклатура
title: {ru: Номенклатура}
code: {type: string, length: 9, auto: true}
description_length: 150
`+body)
			} else {
				writeMetadata(t, root, CatalogKind, searchCatalogID, searchCatalogBody(body))
			}
			_, err := Load(root)
			switch {
			case want.accepts && err != nil:
				t.Fatalf("%s: refused: %v", name, err)
			case !want.accepts:
				if err == nil || !strings.Contains(err.Error(), "this object is not in the index") {
					t.Fatalf("%s: %v", name, err)
				}
			}
		})
	}
}

// Twelve kinds carry the object's flag, and a register is four of them: it
// keeps data of its own, so it is in the index or out of it like anything
// else. A common attribute carries the field's flag, as any field does.
func TestEveryKindThatKeepsDataSaysWhetherItIsIndexed(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, InformationRegisterKind, searchRegisterID, `format: 1
id: `+searchRegisterID+`
name: Цены
title: {ru: Цены}
write_mode: independent
periodicity: day
full_text_search: dont-use
dimensions:
  - {id: `+searchDimensionID+`, name: Товар, title: {ru: Товар}, types: [{kind: string, length: 20}]}
resources:
  - {id: `+searchResourceID+`, name: Цена, title: {ru: Цена}, types: [{kind: number, precision: 15, scale: 2}]}
`)
	writeMetadata(t, root, CommonAttributeKind, searchCommonID, `format: 1
id: `+searchCommonID+`
name: Организация
title: {ru: Организация}
types: [{kind: string, length: 50}]
full_text_search: use
objects: [`+searchRegisterID+`]
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	register, _ := catalog.InformationRegisterDefinition("Цены")
	if register.FullTextSearch != FullTextSearchDontUse {
		t.Fatalf("a register lost its flag: %q", register.FullTextSearch)
	}
	common, ok := catalog.CommonAttribute("Организация")
	if !ok || common.FullTextSearch != FullTextSearchUse {
		t.Fatalf("a common attribute lost its flag: %+v found=%v", common, ok)
	}
}
