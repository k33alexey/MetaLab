package metadata

import (
	"strings"
	"testing"
)

// The three child objects this file is about were found by the sweep of
// subordinate objects, not by a test and not by reading the code: the sweep of
// composition until then compared only the properties of the kinds themselves,
// because the export's property index is collected per kind and knows nothing
// of what a dimension or a flag carries.
//
// What it found: an accounting flag of a chart of accounts carried three
// properties where the export writes twenty-six, an ext dimension accounting
// flag the same, an addressing attribute of a task four where the export writes
// twenty-nine, and a dimension of a sequence five of its six.

const (
	flagsChartID       = "a1000000-0000-4000-8000-000000000001"
	flagsKindsID       = "a1000000-0000-4000-8000-000000000002"
	flagsCurrencyID    = "a1000000-0000-4000-8000-000000000003"
	flagsQuantityID    = "a1000000-0000-4000-8000-000000000004"
	flagsSubcontoID    = "a1000000-0000-4000-8000-000000000005"
	flagsAttributeID   = "a1000000-0000-4000-8000-000000000006"
	flagsCurrencyRefID = "a1000000-0000-4000-8000-000000000007"
)

// chartWithFlags writes a chart of characteristic types for the analytics and a
// chart of accounts whose flags carry whatever the case under test says.
func chartWithFlags(t *testing.T, flags string) string {
	t.Helper()
	root := metadataProject(t)
	writeMetadata(t, root, CatalogKind, flagsCurrencyRefID, `format: 1
id: `+flagsCurrencyRefID+`
name: Валюты
title: {ru: Валюты}
code: {type: string, length: 3, auto: true}
description_length: 50
`)
	writeMetadata(t, root, ChartOfCharacteristicTypesKind, flagsKindsID, `format: 1
id: `+flagsKindsID+`
name: ВидыАналитики
title: {ru: Виды аналитики}
code: {type: string, length: 9, auto: true}
description_length: 100
value_type: [{kind: string, length: 100}]
`)
	writeMetadata(t, root, ChartOfAccountsKind, flagsChartID, `format: 1
id: `+flagsChartID+`
name: Основной
title: {ru: Основной}
code: {type: string, length: 5, auto: false}
description_length: 120
code_mask: "@@.@@"
order_length: 5
ext_dimension_types: `+flagsKindsID+`
max_ext_dimension_count: 3
attributes:
  - id: `+flagsAttributeID+`
    name: Валюта
    title: {ru: Валюта}
    types: [{kind: catalog, reference: `+flagsCurrencyRefID+`}]
`+flags)
	return root
}

// The defect this catches: an accounting flag carried an identifier, a name and
// a synonym, and nothing else. The prototype writes twenty-six properties on
// every one of them - the format the checkbox is shown in, its tooltip, its
// filling value, its choice form, its parameter links - and a configuration
// imported through the old structure lost all twenty-three of them silently,
// which is the import report's third category.
func TestAccountingFlagCarriesTheWholeFieldPalette(t *testing.T) {
	t.Parallel()
	root := chartWithFlags(t, `accounting_flags:
  - id: `+flagsCurrencyID+`
    name: Валютный
    title: {ru: Валютный}
    comment: Учёт в валюте
    types: [{kind: boolean}]
    fill_checking: show-error
    data_history: use
    filling: {value: {kind: boolean, data: "true"}, from_filling_value: true}
    presentation:
      tooltip: {ru: По счёту ведётся валютный учёт}
      format: {ru: "БЛ=Да; Нет"}
      mark_negatives: true
    choice:
      quick_choice: use
      folders_and_items: items
      parameters:
        - {name: Отбор.Действует, values: [{kind: boolean, data: "true"}]}
      parameter_links:
        - {name: Отбор.Валюта, source: {attribute: `+flagsAttributeID+`}}
ext_dimension_accounting_flags:
  - id: `+flagsSubcontoID+`
    name: Суммовой
    title: {ru: Суммовой}
    comment: Суммовой учёт по аналитике
    presentation: {tooltip: {ru: Суммовой учёт}}
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	chart, ok := catalog.ChartOfAccounts("Основной")
	if !ok {
		t.Fatal("the chart of accounts did not load")
	}
	flag := chart.AccountingFlags[0]
	if flag.Comment != "Учёт в валюте" || flag.FillChecking != ShowFillingError || flag.DataHistory != UsageUse {
		t.Fatalf("flag = %+v", flag)
	}
	if flag.Filling.Value == nil || flag.Filling.Value.Data != "true" || !flag.Filling.FromFillingValue {
		t.Fatalf("flag filling = %+v", flag.Filling)
	}
	if flag.Presentation.ToolTip["ru"] != "По счёту ведётся валютный учёт" || !flag.Presentation.MarkNegatives {
		t.Fatalf("flag presentation = %+v", flag.Presentation)
	}
	if flag.Choice.QuickChoice != UsageUse || len(flag.Choice.Parameters) != 1 || len(flag.Choice.ParameterLinks) != 1 {
		t.Fatalf("flag choice = %+v", flag.Choice)
	}
	if ext := chart.ExtDimensionAccountingFlags[0]; ext.Comment == "" || ext.Presentation.ToolTip["ru"] != "Суммовой учёт" {
		t.Fatalf("ext dimension flag = %+v", ext)
	}

	// A copy shares nothing with the original: the chart comes out of the
	// catalog cloned, and a caller that changes what it got must not change
	// what the next caller gets. Before this the clone copied the title alone,
	// and every new property would have been shared.
	again, _ := catalog.ChartOfAccounts("Основной")
	again.AccountingFlags[0].Presentation.ToolTip["ru"] = "changed"
	again.AccountingFlags[0].Choice.Parameters[0].Name = "changed"
	fresh, _ := catalog.ChartOfAccounts("Основной")
	if fresh.AccountingFlags[0].Presentation.ToolTip["ru"] != "По счёту ведётся валютный учёт" ||
		fresh.AccountingFlags[0].Choice.Parameters[0].Name != "Отбор.Действует" {
		t.Fatal("the copy shared its presentation or its choice with the catalog")
	}
}

// The defect this catches: with the whole palette of a field on an accounting
// flag, three properties come along that a flag does not have - indexing,
// full-text search and «использование». The export writes none of the three on
// a flag and the syntax assistant lists none of them among the properties of
// ПризнакУчетаПланаСчетов, so a flag that carried them would read as a setting
// and change nothing: an index that is not built, a search that finds no text,
// a division between items and folders where a chart of accounts has neither.
//
// The fourth case is the type. «Тип: Булево» is what the help says of a flag's
// value, and a flag declared as a string would be a column of the accounts
// table that the movement check cannot read.
func TestAccountingFlagRefusesWhatAFlagHasNot(t *testing.T) {
	t.Parallel()
	cases := map[string]struct{ flags, message string }{
		"индексирование": {`accounting_flags:
  - {id: ` + flagsCurrencyID + `, name: Валютный, title: {ru: Валютный}, indexing: index}`, "indexing"},
		"полнотекстовый поиск": {`accounting_flags:
  - {id: ` + flagsCurrencyID + `, name: Валютный, title: {ru: Валютный}, full_text_search: use}`, "full_text_search"},
		"использование": {`accounting_flags:
  - {id: ` + flagsCurrencyID + `, name: Валютный, title: {ru: Валютный}, use: for-item}`, "use"},
		"тип не булев": {`accounting_flags:
  - {id: ` + flagsCurrencyID + `, name: Валютный, title: {ru: Валютный}, types: [{kind: string, length: 10}]}`, "types"},
		"два типа": {`accounting_flags:
  - {id: ` + flagsCurrencyID + `, name: Валютный, title: {ru: Валютный}, types: [{kind: boolean}, {kind: string, length: 10}]}`, "types"},
		"признак учёта субконто индексируется": {`ext_dimension_accounting_flags:
  - {id: ` + flagsSubcontoID + `, name: Суммовой, title: {ru: Суммовой}, indexing: index}`, "indexing"},
		"связь параметра выбора в никуда": {`accounting_flags:
  - id: ` + flagsCurrencyID + `
    name: Валютный
    title: {ru: Валютный}
    choice:
      parameter_links:
        - {name: Отбор.Валюта, source: {attribute: ` + flagsSubcontoID + `}}`, "parameter_links"},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := Load(chartWithFlags(t, testCase.flags))
			if err == nil || !strings.Contains(err.Error(), testCase.message) {
				t.Fatalf("err = %v, want one naming %s", err, testCase.message)
			}
		})
	}
}

// An accounting flag with no type at all is the ordinary case: the type has one
// possible value, and a line that can only say one thing is noise in every
// chart of accounts. This says so out loud, because the refusal above is close
// enough to it that one could be written into the other by accident.
func TestAccountingFlagWithoutATypeIsBoolean(t *testing.T) {
	t.Parallel()
	catalog, err := Load(chartWithFlags(t, `accounting_flags:
  - {id: `+flagsCurrencyID+`, name: Валютный, title: {ru: Валютный}}`))
	if err != nil {
		t.Fatal(err)
	}
	chart, _ := catalog.ChartOfAccounts("Основной")
	if types := accountingFlagTypes(chart.AccountingFlags[0]); !isBooleanType(types) {
		t.Fatalf("types = %+v, want boolean alone", types)
	}
}

const (
	addressRoleCatalogID = "a2000000-0000-4000-8000-000000000001"
	addressRegisterID    = "a2000000-0000-4000-8000-000000000002"
	addressRoleDimension = "a2000000-0000-4000-8000-000000000003"
	addressPerformer     = "a2000000-0000-4000-8000-000000000004"
	addressTaskID        = "a2000000-0000-4000-8000-000000000005"
	addressRoleAttribute = "a2000000-0000-4000-8000-000000000006"
	addressTaskAttribute = "a2000000-0000-4000-8000-000000000007"
)

// taskWithAddressing writes the register the addressing is resolved by and a
// task whose addressing attributes carry whatever the case under test says.
func taskWithAddressing(t *testing.T, attributes string) string {
	t.Helper()
	root := metadataProject(t)
	writeMetadata(t, root, CatalogKind, addressRoleCatalogID, `format: 1
id: `+addressRoleCatalogID+`
name: РолиИсполнителей
title: {ru: Роли исполнителей}
code: {type: string, length: 9, auto: true}
description_length: 100
`)
	writeMetadata(t, root, InformationRegisterKind, addressRegisterID, `format: 1
id: `+addressRegisterID+`
name: ИсполнителиЗадач
title: {ru: Исполнители задач}
write_mode: independent
periodicity: none
dimensions:
  - id: `+addressRoleDimension+`
    name: РольИсполнителя
    title: {ru: Роль исполнителя}
    types: [{kind: catalog, reference: `+addressRoleCatalogID+`}]
resources:
  - id: `+addressPerformer+`
    name: Исполнитель
    title: {ru: Исполнитель}
    types: [{kind: catalog, reference: `+addressRoleCatalogID+`}]
`)
	writeMetadata(t, root, TaskKind, addressTaskID, `format: 1
id: `+addressTaskID+`
name: ЗадачаИсполнителя
title: {ru: Задача исполнителя}
number: {type: string, length: 14, auto: true, unique: true, periodicity: none}
description_length: 150
addressing: `+addressRegisterID+`
main_addressing_attribute: РольИсполнителя
attributes:
  - id: `+addressTaskAttribute+`
    name: Предмет
    title: {ru: Предмет}
    types: [{kind: catalog, reference: `+addressRoleCatalogID+`}]
`+attributes)
	return root
}

// The defect this catches: an addressing attribute carried an identifier, a
// name, a synonym, a type and its dimension. The prototype gives it everything
// a catalog attribute has - the syntax assistant's «ОбъектМетаданных:
// РеквизитАдресации» lists twenty-seven properties, the export writes
// twenty-nine - and the four we kept were all a migrated task would have.
//
// Indexing and full-text search matter here beyond bookkeeping: an addressing
// attribute is what a task is looked for by, and both are written on the
// demonstration configuration's own attributes.
func TestAddressingAttributeCarriesTheWholeFieldPalette(t *testing.T) {
	t.Parallel()
	root := taskWithAddressing(t, `addressing_attributes:
  - id: `+addressRoleAttribute+`
    name: РольИсполнителя
    title: {ru: Роль исполнителя}
    types: [{kind: catalog, reference: `+addressRoleCatalogID+`}]
    dimension: `+addressRoleDimension+`
    comment: Роль, которой адресована задача
    indexing: index
    full_text_search: use
    data_history: use
    fill_checking: show-error
    filling: {from_filling_value: true}
    presentation:
      tooltip: {ru: Кому адресована задача}
    choice:
      quick_choice: use
      folders_and_items: items
      parameter_links:
        - {name: Отбор.Предмет, source: {attribute: `+addressTaskAttribute+`}}
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	task, ok := catalog.Task("ЗадачаИсполнителя")
	if !ok {
		t.Fatal("the task did not load")
	}
	attribute := task.AddressingAttributes[0]
	if attribute.Comment == "" || attribute.Indexing != IndexField || attribute.FullTextSearch != UsageUse {
		t.Fatalf("addressing attribute = %+v", attribute)
	}
	if attribute.DataHistory != UsageUse || attribute.FillChecking != ShowFillingError || !attribute.Filling.FromFillingValue {
		t.Fatalf("addressing attribute storage = %+v", attribute)
	}
	if attribute.Presentation.ToolTip["ru"] != "Кому адресована задача" {
		t.Fatalf("addressing attribute tooltip = %+v", attribute.Presentation.ToolTip)
	}
	if attribute.Choice.QuickChoice != UsageUse || len(attribute.Choice.ParameterLinks) != 1 {
		t.Fatalf("addressing attribute choice = %+v", attribute.Choice)
	}
	// What it always had is still there.
	if attribute.Dimension == nil || attribute.Dimension.String() != addressRoleDimension {
		t.Fatalf("addressing attribute lost its dimension: %+v", attribute.Dimension)
	}

	again, _ := catalog.Task("ЗадачаИсполнителя")
	again.AddressingAttributes[0].Presentation.ToolTip["ru"] = "changed"
	fresh, _ := catalog.Task("ЗадачаИсполнителя")
	if fresh.AddressingAttributes[0].Presentation.ToolTip["ru"] != "Кому адресована задача" {
		t.Fatal("the copy shared its presentation with the catalog")
	}
}

// The defect this catches: of everything a catalog attribute carries, one thing
// an addressing attribute does not is «использование» - the division between
// the fields of items and the fields of folders. A task has neither, so the
// setting would be read by nothing. And a choice link on an addressing
// attribute now has to point at a field of the same task: carrying the link and
// never resolving it would leave a broken reference to be found at run time.
func TestAddressingAttributeRefusesWhatItHasNot(t *testing.T) {
	t.Parallel()
	cases := map[string]struct{ attributes, message string }{
		"использование": {`addressing_attributes:
  - {id: ` + addressRoleAttribute + `, name: РольИсполнителя, title: {ru: Роль}, types: [{kind: catalog, reference: ` + addressRoleCatalogID + `}], dimension: ` + addressRoleDimension + `, use: for-item}`, "use"},
		"связь параметра выбора в никуда": {`addressing_attributes:
  - id: ` + addressRoleAttribute + `
    name: РольИсполнителя
    title: {ru: Роль}
    types: [{kind: catalog, reference: ` + addressRoleCatalogID + `}]
    dimension: ` + addressRoleDimension + `
    choice:
      parameter_links:
        - {name: Отбор.Предмет, source: {attribute: ` + addressRegisterID + `}}`, "parameter_links"},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := Load(taskWithAddressing(t, testCase.attributes))
			if err == nil || !strings.Contains(err.Error(), testCase.message) {
				t.Fatalf("err = %v, want one naming %s", err, testCase.message)
			}
		})
	}
}

const (
	commentGoodsID     = "a3000000-0000-4000-8000-000000000001"
	commentDocumentID  = "a3000000-0000-4000-8000-000000000002"
	commentDocGoods    = "a3000000-0000-4000-8000-000000000003"
	commentRegisterID  = "a3000000-0000-4000-8000-000000000004"
	commentRegGoods    = "a3000000-0000-4000-8000-000000000005"
	commentRegAmount   = "a3000000-0000-4000-8000-000000000006"
	commentSequenceID  = "a3000000-0000-4000-8000-000000000007"
	commentDimensionID = "a3000000-0000-4000-8000-000000000008"
)

// The defect this catches: a dimension of a sequence carried five of the six
// properties the export writes on one, and the missing one was the comment. It
// is a small loss and a loss all the same - the developer's note on why the
// boundary is kept by this value disappears on import, and nobody notices,
// because everything else about the sequence arrives intact.
func TestSequenceDimensionKeepsItsComment(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, CatalogKind, commentGoodsID, `format: 1
id: `+commentGoodsID+`
name: Номенклатура
title: {ru: Номенклатура}
code: {type: string, length: 9, auto: true}
description_length: 100
`)
	writeMetadata(t, root, DocumentKind, commentDocumentID, `format: 1
id: `+commentDocumentID+`
name: Расход
title: {ru: Расход}
number: {type: string, length: 9, auto: true, unique: true, periodicity: year}
attributes:
  - id: `+commentDocGoods+`
    name: Номенклатура
    title: {ru: Номенклатура}
    types: [{kind: catalog, reference: `+commentGoodsID+`}]
`)
	writeMetadata(t, root, AccumulationRegisterKind, commentRegisterID, `format: 1
id: `+commentRegisterID+`
name: ОстаткиТоваров
title: {ru: Остатки товаров}
kind: balance
dimensions:
  - id: `+commentRegGoods+`
    name: Номенклатура
    title: {ru: Номенклатура}
    types: [{kind: catalog, reference: `+commentGoodsID+`}]
resources:
  - id: `+commentRegAmount+`
    name: Количество
    title: {ru: Количество}
    types: [{kind: number, precision: 15, scale: 3}]
`)
	writeMetadata(t, root, SequenceKind, commentSequenceID, `format: 1
id: `+commentSequenceID+`
name: ДвижениеТоваров
title: {ru: Движение товаров}
documents: [`+commentDocumentID+`]
movements: [`+commentRegisterID+`]
dimensions:
  - id: `+commentDimensionID+`
    name: Номенклатура
    title: {ru: Номенклатура}
    comment: Граница держится по номенклатуре
    types: [{kind: catalog, reference: `+commentGoodsID+`}]
    document_attributes: [`+commentDocGoods+`]
    register_dimensions: [`+commentRegGoods+`]
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	sequence, ok := catalog.Sequence("ДвижениеТоваров")
	if !ok {
		t.Fatal("the sequence did not load")
	}
	if got := sequence.Dimensions[0].Comment; got != "Граница держится по номенклатуре" {
		t.Fatalf("comment = %q", got)
	}
}

// The defect this catches: the column of an addressing attribute was indexed
// unconditionally, because the attribute had no indexing of its own to read.
// Now that it has one, a setting the developer writes and the schema overrules
// would be worse than no setting at all - it reads as an answer and is not one.
//
// Nothing is lost by reading it: every addressing attribute of the export asks
// for the index, so a migrated configuration gets exactly the index it had.
func TestAddressingAttributeIndexFollowsItsSetting(t *testing.T) {
	t.Parallel()
	root := taskWithAddressing(t, `addressing_attributes:
  - id: `+addressRoleAttribute+`
    name: РольИсполнителя
    title: {ru: Роль исполнителя}
    types: [{kind: catalog, reference: `+addressRoleCatalogID+`}]
    dimension: `+addressRoleDimension+`
    indexing: index
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	task, _ := catalog.Task("ЗадачаИсполнителя")
	column, err := PhysicalAttributeColumn(task.AddressingAttributes[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	indexed := func(definition TaskDefinition) bool {
		table, _, err := catalog.taskTables(definition)
		if err != nil {
			t.Fatal(err)
		}
		for _, index := range table.Indexes {
			for _, key := range index.Keys {
				if key == column {
					return true
				}
			}
		}
		return false
	}
	if !indexed(task) {
		t.Fatal("the addressing attribute asked for an index and did not get one")
	}
	// And the other way round: a field that asks for nothing gets nothing. This
	// is the half that the old code could not fail, because it never asked.
	task.AddressingAttributes[0].Indexing = ""
	if indexed(task) {
		t.Fatal("the addressing attribute was indexed although it asked for no index")
	}
}

// The defect this catches: a column of a document journal carried six of the
// seven properties «ОбъектМетаданных: Графа» has, and the missing one was the
// comment - the same loss as the sequence dimension's, found by the same pass
// once it was widened to the child objects nobody had compared yet. The export
// writes it on all 27 columns of the demonstration configuration.
func TestJournalColumnKeepsItsComment(t *testing.T) {
	t.Parallel()
	const (
		journalPartyID    = "a4000000-0000-4000-8000-000000000001"
		journalDocumentID = "a4000000-0000-4000-8000-000000000002"
		journalDocParty   = "a4000000-0000-4000-8000-000000000003"
		journalID         = "a4000000-0000-4000-8000-000000000004"
		journalColumnID   = "a4000000-0000-4000-8000-000000000005"
	)
	root := metadataProject(t)
	writeMetadata(t, root, CatalogKind, journalPartyID, `format: 1
id: `+journalPartyID+`
name: Контрагенты
title: {ru: Контрагенты}
code: {type: string, length: 9, auto: true}
description_length: 100
`)
	writeMetadata(t, root, DocumentKind, journalDocumentID, `format: 1
id: `+journalDocumentID+`
name: Расход
title: {ru: Расход}
number: {type: string, length: 9, auto: true, unique: true, periodicity: year}
attributes:
  - id: `+journalDocParty+`
    name: Контрагент
    title: {ru: Контрагент}
    types: [{kind: catalog, reference: `+journalPartyID+`}]
`)
	writeMetadata(t, root, DocumentJournalKind, journalID, `format: 1
id: `+journalID+`
name: СкладскиеДокументы
title: {ru: Складские документы}
documents: [`+journalDocumentID+`]
columns:
  - id: `+journalColumnID+`
    name: Контрагент
    title: {ru: Контрагент}
    comment: Показывает контрагента любого из документов журнала
    references: [`+journalDocParty+`]
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	journal, ok := catalog.DocumentJournal("СкладскиеДокументы")
	if !ok {
		t.Fatal("the journal did not load")
	}
	if got := journal.Columns[0].Comment; got != "Показывает контрагента любого из документов журнала" {
		t.Fatalf("comment = %q", got)
	}
}
