package metadata

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
)

const (
	aroundNumerator = "d0c00000-0000-4000-8000-000000000001"
	aroundFirstDoc  = "d0c00000-0000-4000-8000-000000000002"
	aroundSecondDoc = "d0c00000-0000-4000-8000-000000000003"
	aroundSequence  = "d0c00000-0000-4000-8000-000000000004"
	aroundJournal   = "d0c00000-0000-4000-8000-000000000005"
	aroundRegister  = "d0c00000-0000-4000-8000-000000000006"
	aroundGoods     = "d0c00000-0000-4000-8000-000000000007"

	aroundFirstGoods   = "d0c00000-0000-4000-8000-000000000010"
	aroundSecondGoods  = "d0c00000-0000-4000-8000-000000000011"
	aroundFirstParty   = "d0c00000-0000-4000-8000-000000000012"
	aroundSecondParty  = "d0c00000-0000-4000-8000-000000000013"
	aroundFirstLines   = "d0c00000-0000-4000-8000-000000000014"
	aroundSecondLines  = "d0c00000-0000-4000-8000-000000000015"
	aroundDimension    = "d0c00000-0000-4000-8000-000000000016"
	aroundRegisterGood = "d0c00000-0000-4000-8000-000000000017"
	aroundRegisterQty  = "d0c00000-0000-4000-8000-000000000018"
	aroundColumn       = "d0c00000-0000-4000-8000-000000000019"
)

// Two kinds of document drawing numbers from one run, a sequence keeping a
// boundary per product rather than one for everything, and a journal showing
// both kinds under common headings.
func aroundDocumentsProject(t *testing.T) string {
	t.Helper()
	root := metadataProject(t)
	catalogNamed(t, root, aroundGoods, "Номенклатура")
	writeMetadata(t, root, NumeratorKind, aroundNumerator, `format: 1
id: `+aroundNumerator+`
name: СквознаяНумерация
title: {ru: Сквозная нумерация}
number: {type: string, length: 11, unique: true, periodicity: year}
`)
	for _, document := range []struct{ id, name, goods, party, lines string }{
		{aroundFirstDoc, "ПоступлениеТоваров", aroundFirstGoods, aroundFirstParty, aroundFirstLines},
		{aroundSecondDoc, "РеализацияТоваров", aroundSecondGoods, aroundSecondParty, aroundSecondLines},
	} {
		writeMetadata(t, root, DocumentKind, document.id, `format: 1
id: `+document.id+`
name: `+document.name+`
title: {ru: `+document.name+`}
numerator: `+aroundNumerator+`
posting: {allowed: true}
attributes:
  - id: `+document.party+`
    name: Контрагент
    title: {ru: Контрагент}
    types: [{kind: catalog, reference: `+aroundGoods+`}]
table_parts:
  - id: `+document.lines+`
    name: Товары
    title: {ru: Товары}
    attributes:
      - id: `+document.goods+`
        name: Номенклатура
        title: {ru: Номенклатура}
        types: [{kind: catalog, reference: `+aroundGoods+`}]
`)
	}
	writeMetadata(t, root, AccumulationRegisterKind, aroundRegister, `format: 1
id: `+aroundRegister+`
name: ОстаткиТоваров
title: {ru: Остатки товаров}
kind: balance
dimensions:
  - id: `+aroundRegisterGood+`
    name: Номенклатура
    title: {ru: Номенклатура}
    types: [{kind: catalog, reference: `+aroundGoods+`}]
resources:
  - id: `+aroundRegisterQty+`
    name: Количество
    title: {ru: Количество}
    types: [{kind: number, precision: 15, scale: 3}]
`)
	return root
}

func TestLoadNumeratorSequenceAndJournal(t *testing.T) {
	t.Parallel()
	root := aroundDocumentsProject(t)
	writeMetadata(t, root, SequenceKind, aroundSequence, `format: 1
id: `+aroundSequence+`
name: ДвижениеТоваров
title: {ru: Движение товаров}
move_boundary_on_posting: true
documents: [`+aroundFirstDoc+`, `+aroundSecondDoc+`]
movements: [`+aroundRegister+`]
dimensions:
  - id: `+aroundDimension+`
    name: Номенклатура
    title: {ru: Номенклатура}
    types: [{kind: catalog, reference: `+aroundGoods+`}]
    document_attributes: [`+aroundFirstGoods+`, `+aroundSecondGoods+`]
    register_dimensions: [`+aroundRegisterGood+`]
`)
	writeMetadata(t, root, DocumentJournalKind, aroundJournal, `format: 1
id: `+aroundJournal+`
name: СкладскиеДокументы
title: {ru: Складские документы}
documents: [`+aroundFirstDoc+`, `+aroundSecondDoc+`]
columns:
  - id: `+aroundColumn+`
    name: Контрагент
    title: {ru: Контрагент}
    indexing: index
    references: [`+aroundFirstParty+`, `+aroundSecondParty+`]
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}

	// Both documents number from one run, and neither declares a number.
	for _, name := range []string{"ПоступлениеТоваров", "РеализацияТоваров"} {
		var document DocumentDefinition
		for _, item := range catalog.Documents {
			if item.Name == name {
				document = item
			}
		}
		if document.Numerator == nil {
			t.Fatalf("%s lost the numerator it shares", name)
		}
		if document.Number.Length != 11 || document.Number.Periodicity != NumberPeriodYear || !document.Number.Unique {
			t.Fatalf("%s did not take its number from the numerator: %+v", name, document.Number)
		}
	}

	sequence, ok := catalog.Sequence("ДвижениеТоваров")
	if !ok {
		t.Fatal("the sequence did not load")
	}
	if !sequence.MoveBoundaryOnPosting || len(sequence.Documents) != 2 || len(sequence.Movements) != 1 {
		t.Fatalf("the sequence lost what it follows: %+v", sequence)
	}
	// A dimension taken from the lines of both documents and matched against
	// the register: that mapping is the whole of what a dimension is.
	if len(sequence.Dimensions) != 1 || len(sequence.Dimensions[0].DocumentAttributes) != 2 ||
		len(sequence.Dimensions[0].RegisterDimensions) != 1 {
		t.Fatalf("the dimension lost its maps: %+v", sequence.Dimensions)
	}

	journal, ok := catalog.DocumentJournal("СкладскиеДокументы")
	if !ok {
		t.Fatal("the journal did not load")
	}
	if len(journal.Documents) != 2 || len(journal.Columns) != 1 || len(journal.Columns[0].References) != 2 {
		t.Fatalf("the journal lost what it shows: %+v", journal)
	}

	schema, err := catalog.ApplicationSchema()
	if err != nil {
		t.Fatal(err)
	}
	sequenceTable, err := PhysicalCatalogTable(sequence.ID)
	if err != nil {
		t.Fatal(err)
	}
	journalTable, err := PhysicalCatalogTable(journal.ID)
	if err != nil {
		t.Fatal(err)
	}
	var foundSequence bool
	for _, table := range schema.Tables {
		if table.Name == journalTable {
			t.Fatal("a journal stores no data of its own, so it must have no table")
		}
		if table.Name != sequenceTable {
			continue
		}
		foundSequence = true
		columns := map[string]bool{}
		for _, column := range table.Columns {
			columns[column.Name] = true
		}
		for _, column := range []string{"period", "recorder_type", "recorder_ref"} {
			if !columns[column] {
				t.Fatalf("the sequence table has no %s", column)
			}
		}
	}
	if !foundSequence {
		t.Fatal("the sequence has no table of records")
	}
}

// Every pointer refused here is one that would otherwise fail silently: a
// boundary that never moves, a column that shows nothing, a number nobody set.
func TestAroundDocumentsRefusesPointersThatLeadNowhere(t *testing.T) {
	t.Parallel()
	for name, broken := range map[string]struct {
		kind Kind
		body string
	}{
		"последовательность по неизвестному документу": {SequenceKind, `
documents: [d0c00000-0000-4000-8000-0000000000ff]
dimensions: []`},
		"измерение от реквизита чужого документа": {SequenceKind, `
documents: [` + aroundFirstDoc + `]
movements: [` + aroundRegister + `]
dimensions:
  - {id: ` + aroundDimension + `, name: Номенклатура, title: {ru: Номенклатура}, types: [{kind: catalog, reference: ` + aroundGoods + `}], document_attributes: [` + aroundSecondGoods + `], register_dimensions: [` + aroundRegisterGood + `]}`},
		"измерение, сопоставленное не с измерением регистра": {SequenceKind, `
documents: [` + aroundFirstDoc + `]
movements: [` + aroundRegister + `]
dimensions:
  - {id: ` + aroundDimension + `, name: Номенклатура, title: {ru: Номенклатура}, types: [{kind: catalog, reference: ` + aroundGoods + `}], document_attributes: [` + aroundFirstGoods + `], register_dimensions: [` + aroundRegisterQty + `]}`},
		"журнал показывает чужой реквизит": {DocumentJournalKind, `
documents: [` + aroundFirstDoc + `]
columns:
  - {id: ` + aroundColumn + `, name: Контрагент, title: {ru: Контрагент}, references: [` + aroundSecondParty + `]}`},
		"графа показывает два реквизита одного документа": {DocumentJournalKind, `
documents: [` + aroundFirstDoc + `]
columns:
  - {id: ` + aroundColumn + `, name: Контрагент, title: {ru: Контрагент}, references: [` + aroundFirstParty + `, ` + aroundFirstGoods + `]}`},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := aroundDocumentsProject(t)
			id, title := aroundSequence, "Движение товаров"
			if broken.kind == DocumentJournalKind {
				id, title = aroundJournal, "Складские документы"
			}
			writeMetadata(t, root, broken.kind, id, `format: 1
id: `+id+`
name: Проверка
title: {ru: `+title+`}`+broken.body+`
`)
			if _, err := Load(root); err == nil {
				t.Fatal("a pointer that leads nowhere was accepted")
			}
		})
	}
}

// A document with a numerator still carries its own number: the prototype
// writes it beside the numerator on every such document, all eighteen in the
// configurations being moved. The numerator decides the numbering, and the
// document's own number is checked like any other - carried is not unchecked -
// and a numerator does not decide what the document alone decides.
func TestNumberComesFromExactlyOnePlace(t *testing.T) {
	t.Parallel()
	t.Run("документ несёт своё рядом с общим", func(t *testing.T) {
		t.Parallel()
		root := aroundDocumentsProject(t)
		writeMetadata(t, root, DocumentKind, aroundFirstDoc, `format: 1
id: `+aroundFirstDoc+`
name: ПоступлениеТоваров
title: {ru: Поступление товаров}
numerator: `+aroundNumerator+`
number: {type: string, length: 9, auto: true, periodicity: none}
`)
		catalog, err := Load(root)
		if err != nil {
			t.Fatalf("a document declaring a number beside its numerator was refused: %v", err)
		}
		document, _ := catalog.DocumentDefinition("ПоступлениеТоваров")
		if document.Number.Length != 11 || document.Number.Periodicity != NumberPeriodYear {
			t.Fatalf("the document's own number decided the numbering: %+v", document.Number)
		}
		// A numerator has no automatic numbering: that one setting stays the
		// document's, and taking the numerator's whole number lost it.
		if !document.Number.Auto {
			t.Fatalf("the document's own automatic numbering was lost to the numerator: %+v", document.Number)
		}
	})
	t.Run("своё рядом с общим проверяется", func(t *testing.T) {
		t.Parallel()
		root := aroundDocumentsProject(t)
		writeMetadata(t, root, DocumentKind, aroundFirstDoc, `format: 1
id: `+aroundFirstDoc+`
name: ПоступлениеТоваров
title: {ru: Поступление товаров}
numerator: `+aroundNumerator+`
number: {type: boolean, length: 9, periodicity: none}
`)
		if _, err := Load(root); err == nil {
			t.Fatal("a broken number beside a numerator was accepted")
		}
	})
	t.Run("нумератор решает за документ", func(t *testing.T) {
		t.Parallel()
		root := metadataProject(t)
		writeMetadata(t, root, NumeratorKind, aroundNumerator, `format: 1
id: `+aroundNumerator+`
name: СквознаяНумерация
title: {ru: Сквозная нумерация}
number: {type: string, length: 11, auto: true, periodicity: none}
`)
		if _, err := Load(root); err == nil {
			t.Fatal("a numerator claiming automatic numbering was accepted")
		}
	})
	t.Run("неизвестный нумератор", func(t *testing.T) {
		t.Parallel()
		root := aroundDocumentsProject(t)
		writeMetadata(t, root, DocumentKind, aroundFirstDoc, `format: 1
id: `+aroundFirstDoc+`
name: ПоступлениеТоваров
title: {ru: Поступление товаров}
numerator: d0c00000-0000-4000-8000-0000000000ff
`)
		if _, err := Load(root); err == nil {
			t.Fatal("a document numbered by nothing was accepted")
		}
	})
}

// A sequence dimension taken from no attribute of any document is filled by
// code, and the prototype saves it: the one sequence of acc has two such
// dimensions, set by an event subscription. The sequence keeps its record set
// module in its own folder, and only that module.
//
// Defect caught: the dimension refused at load (the whole of acc with it); the
// record set module of a sequence having nowhere to lie (it was a single file);
// and a module of a role a sequence does not have accepted beside it.
func TestSequenceDimensionMayBeFilledByCodeAndTheSequenceKeepsItsModule(t *testing.T) {
	t.Parallel()
	root := aroundDocumentsProject(t)
	writeMetadata(t, root, SequenceKind, aroundSequence, `format: 1
id: `+aroundSequence+`
name: ДокументыОрганизаций
title: {ru: Документы организаций}
documents: [`+aroundFirstDoc+`]
dimensions:
  - {id: `+aroundDimension+`, name: Номенклатура, title: {ru: Номенклатура}, types: [{kind: catalog, reference: `+aroundGoods+`}]}
`)
	folder := filepath.Join(root, "metadata", string(SequenceKind), "ДокументыОрганизаций")
	module := "Процедура ПередЗаписью(Отказ, Замещение)\nКонецПроцедуры\n"
	if err := os.WriteFile(filepath.Join(folder, project.RecordSetModuleFile), []byte(module), 0o644); err != nil {
		t.Fatal(err)
	}
	catalog, err := Load(root)
	if err != nil {
		t.Fatalf("a dimension filled by code, or the record set module, refused: %v", err)
	}
	if sequence, ok := catalog.Sequence("ДокументыОрганизаций"); !ok || len(sequence.Dimensions) != 1 || len(sequence.Dimensions[0].DocumentAttributes) != 0 {
		t.Fatalf("sequence = %+v, %v", sequence, ok)
	}

	if err := os.WriteFile(filepath.Join(folder, project.ObjectModuleFile), []byte(module), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root); err == nil {
		t.Fatal("an object module accepted beside a sequence, which has no object")
	}
}
