package metadata

import (
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/schemadiff"
)

const (
	ownerPartners  = "cb000000-0000-4000-8000-000000000001"
	ownerFiles     = "cb000000-0000-4000-8000-000000000002"
	ownerKinds     = "cb000000-0000-4000-8000-000000000003"
	ownerContracts = "cb000000-0000-4000-8000-000000000010"
	ownerDocument  = "cb000000-0000-4000-8000-000000000020"
	ownerMissing   = "cb000000-0000-4000-8000-0000000000ff"
)

// ownerProject writes what a subordinate catalog can belong to: a hierarchical
// catalog with folders, a flat one, and a chart of characteristic types.
func ownerProject(t *testing.T, subordinate string) string {
	t.Helper()
	root := metadataProject(t)
	writeMetadata(t, root, CatalogKind, ownerPartners, `format: 1
id: `+ownerPartners+`
name: Партнеры
title: {ru: Партнёры}
code: {type: string, length: 9, auto: true}
description_length: 150
hierarchy: {enabled: true, kind: folders-and-items}
`)
	catalogNamed(t, root, ownerFiles, "Файлы")
	writeMetadata(t, root, ChartOfCharacteristicTypesKind, ownerKinds, `format: 1
id: `+ownerKinds+`
name: ВидыСвойств
title: {ru: Виды свойств}
code: {type: string, length: 9}
description_length: 150
value_type: [{kind: string, length: 100}]
`)
	writeMetadata(t, root, DocumentKind, ownerDocument, `format: 1
id: `+ownerDocument+`
name: Заказ
title: {ru: Заказ}
number: {type: string, length: 9, auto: true, periodicity: none}
`)
	if subordinate != "" {
		writeMetadata(t, root, CatalogKind, ownerContracts, subordinate)
	}
	return root
}

// contractsOwnedBy writes the subordinate catalog. A body naming the code
// brings its own, because a description cannot say code twice.
func contractsOwnedBy(body string) string {
	code := "code: {type: string, length: 9, auto: true}\n"
	if strings.Contains(body, "code:") {
		code = ""
	}
	return `format: 1
id: ` + ownerContracts + `
name: Договоры
title: {ru: Договоры}
` + code + `description_length: 150
` + body
}

// A subordinate catalog says whom its rows belong to, and the table gets the
// column to put the owner in. One owner is a reference to a known object and
// carries a key into it; several are a composite reference and carry the
// object's identity beside the row's.
func TestSubordinateCatalogKeepsItsOwnersAndGetsAColumn(t *testing.T) {
	t.Parallel()
	root := ownerProject(t, contractsOwnedBy(`owners: [`+ownerPartners+`]
subordination: to-folders-and-items
`))
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	contracts, ok := catalog.CatalogDefinition("Договоры")
	switch {
	case !ok || len(contracts.Owners) != 1:
		t.Fatalf("the owners were lost: found=%v %+v", ok, contracts.Owners)
	case contracts.Subordination != SubordinateToFoldersAndItem:
		t.Fatalf("what an owner may be was lost: %+v", contracts.Subordination)
	}
	types := catalog.OwnerTypes(contracts)
	if len(types) != 1 || types[0].Kind != CatalogType || types[0].Reference.String() != ownerPartners {
		t.Fatalf("what the owner of a row may be was read wrong: %+v", types)
	}

	table, _, err := catalog.catalogTables(contracts)
	if err != nil {
		t.Fatal(err)
	}
	if !hasColumn(table.Columns, "owner") {
		t.Fatalf("a subordinate catalog got no column for its owner: %+v", table.Columns)
	}
	var keyed bool
	for _, constraint := range table.Constraints {
		if strings.Contains(constraint.Definition, "FOREIGN KEY (owner)") {
			keyed = true
		}
	}
	if !keyed {
		t.Fatalf("one owner is a reference to a known object and got no key: %+v", table.Constraints)
	}

	contracts.Owners[0] = mustUUID(t, ownerFiles)
	again, _ := catalog.CatalogDefinition("Договоры")
	if again.Owners[0].String() != ownerPartners {
		t.Fatal("the owners were handed out by reference")
	}
}

// Several owners of different kinds at once - the demonstration configuration
// has such a catalog - and then no single table to key into.
func TestCatalogWithSeveralOwnersStoresWhichOneItIs(t *testing.T) {
	t.Parallel()
	root := ownerProject(t, contractsOwnedBy(`owners: [`+ownerFiles+`, `+ownerKinds+`]
subordination: to-items
code: {type: string, length: 9, auto: true, series: within-owner-subordination}
`))
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	contracts, _ := catalog.CatalogDefinition("Договоры")
	if len(contracts.Owners) != 2 || contracts.Code.Series != WithinOwnerSeries {
		t.Fatalf("two owners and their numbering were lost: %+v", contracts)
	}
	types := catalog.OwnerTypes(contracts)
	if len(types) != 2 || types[0].Kind != CatalogType || types[1].Kind != CharacteristicTypesType {
		t.Fatalf("the owner of a row may be either, and the types say %+v", types)
	}
	table, _, err := catalog.catalogTables(contracts)
	if err != nil {
		t.Fatal(err)
	}
	if !hasColumn(table.Columns, "owner_type") || !hasColumn(table.Columns, "owner_ref") {
		t.Fatalf("a composite owner got no columns to be composite in: %+v", table.Columns)
	}
	for _, constraint := range table.Constraints {
		if strings.Contains(constraint.Definition, "FOREIGN KEY (owner") {
			t.Fatalf("a composite owner got a key that can only point at one table: %+v", constraint)
		}
	}
}

func hasColumn(columns []schemadiff.Column, name string) bool {
	for _, column := range columns {
		if column.Name == name {
			return true
		}
	}
	return false
}

// Every setting here says something about an owner, and each of them says it
// about nothing when there is no owner, or about an owner that cannot answer.
func TestBrokenSubordinationIsRefused(t *testing.T) {
	t.Parallel()
	for name, broken := range map[string]struct{ body, want string }{
		"владельца нет в конфигурации": {`owners: [` + ownerMissing + `]`,
			"which is not an object that can own a catalog"},
		"владелец не может владеть": {`owners: [` + ownerDocument + `]`,
			"which is not an object that can own a catalog"},
		"подчинение группам у владельца без групп": {`owners: [` + ownerFiles + `]
subordination: to-folders`, "which has none"},
		"подчинение без владельца": {`subordination: to-items`,
			"subordination needs owners"},
		"нумерация по владельцу без владельца": {`code: {type: string, length: 9, auto: true, series: within-owner-subordination}`,
			"within-owner-subordination needs owners"},
		"подчинение неизвестно чему": {`owners: [` + ownerFiles + `]
subordination: подчинённым`, "must be to-items, to-folders or to-folders-and-items"},
		"один владелец дважды": {`owners: [` + ownerFiles + `, ` + ownerFiles + `]`,
			"owners[1] repeats"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := ownerProject(t, contractsOwnedBy(broken.body+"\n"))
			_, err := Load(root)
			if err == nil {
				t.Fatalf("%s: accepted", name)
			}
			if !strings.Contains(err.Error(), broken.want) {
				t.Fatalf("%s: refused for another reason: %v", name, err)
			}
		})
	}
}

// The range a code is numbered within is not the same set of answers for every
// kind: an owner is a catalog's alone, and two kinds have no such property at
// all.
func TestCodeSeriesBelongsToTheKindThatHasIt(t *testing.T) {
	t.Parallel()
	_, err := DecodeChartOfCharacteristicTypes("object.yaml", strings.NewReader(`format: 1
id: `+ownerKinds+`
name: ВидыСвойств
title: {ru: Виды свойств}
code: {type: string, length: 9, series: within-owner-subordination}
description_length: 150
value_type: [{kind: string, length: 100}]
`), metadataConfiguration())
	if err == nil || !strings.Contains(err.Error(), "belongs to a catalog") {
		t.Fatalf("a chart of characteristic types numbered within an owner it cannot have: %v", err)
	}

	_, err = DecodeExchangePlan("object.yaml", strings.NewReader(`format: 1
id: `+ownerKinds+`
name: Обмен
title: {ru: Обмен}
code: {type: string, length: 9, series: whole}
description_length: 150
content: [{kind: catalogs, object: `+ownerFiles+`, auto_record: allow}]
`), metadataConfiguration())
	if err == nil || !strings.Contains(err.Error(), "belongs to a kind that numbers within a range") {
		t.Fatalf("an exchange plan numbered within a range it does not have: %v", err)
	}
}
