package metadata

import (
	"strings"
	"testing"
)

const (
	exchangePlanID    = "e0000000-0000-4000-8000-000000000001"
	exchangedCatalog  = "e0000000-0000-4000-8000-000000000002"
	exchangedConstant = "e0000000-0000-4000-8000-000000000003"
	nodeHolderCatalog = "e0000000-0000-4000-8000-000000000004"
	exchangedEnum     = "e0000000-0000-4000-8000-000000000006"
	exchangedEnumItem = "e0000000-0000-4000-8000-000000000007"
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
	writeMetadata(t, root, EnumerationKind, exchangedEnum, `format: 1
id: `+exchangedEnum+`
name: ВидыДоговоров
title: {ru: Виды договоров}
values:
  - {id: `+exchangedEnumItem+`, name: Поставка, title: {ru: Поставка}}
`)
}

// A plan carries two things nothing else does: what is registered and whether
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
  - {kind: enumerations, object: ` + exchangedEnum + `}`,
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

// An exchange plan registers changes of accounting and calculation registers,
// of sequences and of recalculations too - eleven plans of the configurations
// being moved hold one of them, and acc registers a recalculation.
//
// Defect caught: a plan holding any of the four refused at load, as if the
// object it names were missing from the project.
func TestExchangePlanRegistersRegistersSequencesAndRecalculations(t *testing.T) {
	t.Parallel()
	plan := func(content string) string {
		return `format: 1
id: ` + exchangePlanID + `
name: ОбменСФилиалами
title: {ru: Обмен с филиалами}
code: {type: string, length: 36, auto: false}
description_length: 150
content:
` + content
	}
	for name, project := range map[string]func(t *testing.T) (string, string){
		"регистр расчёта и перерасчёт": func(t *testing.T) (string, string) {
			return calculationProject(t, calcRegisterBody),
				"  - {kind: calculation-registers, object: " + calcRegister + "}\n  - {kind: recalculations, object: " + calcRecalc + "}\n"
		},
		"регистр бухгалтерии": func(t *testing.T) (string, string) {
			return entriesProject(t, true, `resources:
  - {id: `+entriesSum+`, name: Сумма, title: {ru: Сумма}, types: [{kind: number, precision: 15, scale: 2}]}`),
				"  - {kind: accounting-registers, object: " + entriesRegister + "}\n"
		},
		"последовательность": func(t *testing.T) (string, string) {
			root := aroundDocumentsProject(t)
			writeMetadata(t, root, SequenceKind, aroundSequence, "format: 1\nid: "+aroundSequence+"\nname: Движение\ntitle: {ru: Движение}\ndocuments: ["+aroundFirstDoc+"]\n")
			return root, "  - {kind: sequences, object: " + aroundSequence + "}\n"
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root, content := project(t)
			writeMetadata(t, root, ExchangePlanKind, exchangePlanID, plan(content))
			catalog, err := Load(root)
			if err != nil {
				t.Fatalf("refused: %v", err)
			}
			if found, ok := catalog.ExchangePlan("ОбменСФилиалами"); !ok || len(found.Content) == 0 {
				t.Fatalf("content = %+v", found.Content)
			}
		})
	}
	// A recalculation the project does not have is still refused.
	root := calculationProject(t, calcRegisterBody)
	writeMetadata(t, root, ExchangePlanKind, exchangePlanID, plan("  - {kind: recalculations, object: "+calcRegister+"}\n"))
	if _, err := Load(root); err == nil {
		t.Fatal("a recalculation the project does not have was accepted")
	}
}

// An exchange plan in the content of a plan is refused, another plan as well
// as the plan itself: the window «Состав» of a plan offers no exchange plan
// at all (opis of 24.10.2026, plan-obmena.md, «Досъёмка»). Defects caught:
// another plan carried with a note, as it was before 2.522, and the plan's
// own refusal lost.
func TestExchangePlanTakesNoPlanIntoItsContent(t *testing.T) {
	t.Parallel()
	other := "e0000000-0000-4000-8000-000000000005"
	plan := func(content string) string {
		return "format: 1\nid: " + exchangePlanID + "\nname: ОбменСФилиалами\ntitle: {ru: ОбменСФилиалами}\n" +
			"code: {type: string, length: 36, auto: false}\ndescription_length: 150\n" + content
	}
	for name, test := range map[string]struct{ object, want string }{
		"другой план": {other, "content[0].kind is not a kind whose changes can be registered"},
		"сам план":    {exchangePlanID, "content[0] puts the plan into its own content"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeExchangePlan("object.yaml", strings.NewReader(plan("content:\n  - {kind: exchange-plans, object: "+test.object+"}\n")),
				metadataConfiguration())
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want one saying %q", err, test.want)
			}
		})
	}
}

// «Включать расширения конфигурации» is not carried: extensions of the
// configuration are refused whole (owner, 30.09.2026, and the audit of
// 15.10.2026, exclusions-audit.md, question 5). A project naming it is
// refused as naming a property the model does not keep.
//
// Defect caught: the flag kept in the model after the mechanism it serves was
// refused.
func TestAnExchangePlanCarriesNoFlagOfExtensions(t *testing.T) {
	t.Parallel()
	_, err := DecodeExchangePlan("object.yaml", strings.NewReader("format: 1\nid: "+exchangePlanID+
		"\nname: Филиалы\ntitle: {ru: Филиалы}\ncode: {type: string, length: 9}\ndescription_length: 100\ninclude_extensions: true\n"), metadataConfiguration())
	if err == nil || !strings.Contains(err.Error(), "include_extensions") {
		t.Fatalf("the flag of extensions: %v", err)
	}
}
