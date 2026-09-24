package metadata

import (
	"testing"
)

const (
	exchangePlanID    = "e0000000-0000-4000-8000-000000000001"
	exchangedCatalog  = "e0000000-0000-4000-8000-000000000002"
	exchangedConstant = "e0000000-0000-4000-8000-000000000003"
	nodeHolderCatalog = "e0000000-0000-4000-8000-000000000004"
)

func exchangedObjects(t *testing.T, root string) {
	t.Helper()
	writeMetadata(t, root, CatalogKind, exchangedCatalog, `format: 1
id: `+exchangedCatalog+`
name: Контрагенты
title: {ru: Контрагенты}
code: {type: string, length: 9, auto: true}
description_length: 150
`)
	writeMetadata(t, root, ConstantKind, exchangedConstant, `format: 1
id: `+exchangedConstant+`
name: ВалютаУчета
title: {ru: Валюта учёта}
types: [{kind: string, length: 3}]
`)
}

// A plan carries three things nothing else does: what is registered, whether
// the platform registers it by itself, and a flag we do not implement but must
// not lose. A node carries which base it is and what the two sides have
// exchanged.
func TestLoadExchangePlanWithContent(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	exchangedObjects(t, root)
	writeMetadata(t, root, ExchangePlanKind, exchangePlanID, `format: 1
id: `+exchangePlanID+`
name: ОбменСФилиалами
title: {ru: Обмен с филиалами}
code: {type: string, length: 36, auto: false}
description_length: 150
include_extensions: true
distributed_info_base: true
content:
  - {kind: catalogs, object: `+exchangedCatalog+`, auto_record: allow}
  - {kind: constants, object: `+exchangedConstant+`, auto_record: deny}
`)
	// A node of the plan is a value like any other reference.
	writeMetadata(t, root, CatalogKind, nodeHolderCatalog, `format: 1
id: `+nodeHolderCatalog+`
name: НастройкиОбмена
title: {ru: Настройки обмена}
code: {type: string, length: 9, auto: true}
description_length: 150
attributes:
  - id: e0000000-0000-4000-8000-000000000010
    name: Узел
    title: {ru: Узел}
    types: [{kind: exchange-plan, reference: `+exchangePlanID+`}]
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	plan, ok := catalog.ExchangePlan("ОбменСФилиалами")
	if !ok {
		t.Fatal("the exchange plan did not load")
	}
	if len(plan.Content) != 2 {
		t.Fatalf("the content was lost: %+v", plan.Content)
	}
	if plan.Content[0].Auto != AutoRecordAllow || plan.Content[1].Auto != AutoRecordDeny {
		t.Fatalf("automatic registration is not a detail and was lost: %+v", plan.Content)
	}
	if !plan.IncludeExtensions {
		t.Fatal("whether extensions travel to the nodes was lost")
	}
	// Carried, shown, not acted upon - and above all not dropped in silence.
	if !plan.DistributedInfoBase {
		t.Fatal("the distributed base flag was dropped instead of being carried unimplemented")
	}

	schema, err := catalog.ApplicationSchema()
	if err != nil {
		t.Fatal(err)
	}
	nodeTable, err := PhysicalCatalogTable(plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, table := range schema.Tables {
		if table.Name != nodeTable {
			continue
		}
		found = true
		columns := map[string]bool{}
		for _, column := range table.Columns {
			columns[column.Name] = true
		}
		for _, column := range []string{"code", "description", "this_node", "sent_no", "received_no"} {
			if !columns[column] {
				t.Fatalf("the node table has no %s", column)
			}
		}
		// One base is one node. The database says so, not a comment.
		var guarded bool
		for _, index := range table.Indexes {
			if index.Unique && index.Predicate == "this_node" {
				guarded = true
			}
		}
		if !guarded {
			t.Fatal("nothing stops two nodes from both being this base")
		}
	}
	if !found {
		t.Fatal("the plan has no table of nodes")
	}
}

// Everything refused here is refused because it would otherwise be discovered
// as missing data on the far side of an exchange, long after the cause.
func TestExchangePlanRefusesContentThatRegistersNothing(t *testing.T) {
	t.Parallel()
	for name, content := range map[string]string{
		"объекта нет в проекте": `
content:
  - {kind: catalogs, object: e0000000-0000-4000-8000-0000000000ff}`,
		"объект назван не тем видом": `
content:
  - {kind: documents, object: ` + exchangedCatalog + `}`,
		"один объект дважды": `
content:
  - {kind: catalogs, object: ` + exchangedCatalog + `, auto_record: allow}
  - {kind: catalogs, object: ` + exchangedCatalog + `, auto_record: deny}`,
		"план в собственном составе": `
content:
  - {kind: catalogs, object: ` + exchangePlanID + `}`,
		"вид, изменения которого не регистрируются": `
content:
  - {kind: enumerations, object: ` + exchangedCatalog + `}`,
		"вид, который ещё не описан": `
content:
  - {kind: sequences, object: ` + exchangedCatalog + `}`,
		"неизвестный признак регистрации": `
content:
  - {kind: catalogs, object: ` + exchangedCatalog + `, auto_record: sometimes}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			exchangedObjects(t, root)
			writeMetadata(t, root, ExchangePlanKind, exchangePlanID, `format: 1
id: `+exchangePlanID+`
name: ОбменСФилиалами
title: {ru: Обмен с филиалами}
code: {type: string, length: 36, auto: false}
description_length: 150`+content+`
`)
			if _, err := Load(root); err == nil {
				t.Fatal("content that registers nothing was accepted")
			}
		})
	}
}

// The standard attributes of a node belong to the platform. An application
// attribute taking one of their names would shadow the platform's own record
// of the exchange.
func TestExchangePlanKeepsItsStandardAttributes(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"ЭтотУзел", "ThisNode", "НомерОтправленного", "ReceivedNo"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeMetadata(t, root, ExchangePlanKind, exchangePlanID, `format: 1
id: `+exchangePlanID+`
name: ОбменСФилиалами
title: {ru: Обмен с филиалами}
code: {type: string, length: 36, auto: false}
description_length: 150
attributes:
  - id: e0000000-0000-4000-8000-000000000020
    name: `+name+`
    title: {ru: Занятое имя}
    types: [{kind: string, length: 10}]
`)
			if _, err := Load(root); err == nil {
				t.Fatalf("an attribute named %s shadowed a standard one", name)
			}
		})
	}
}
