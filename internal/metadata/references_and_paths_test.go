package metadata

import (
	"slices"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// References and paths the model could not express until 2.180: a search by a
// defined type, a link to a constant, a standard field of a document in a
// journal and a sequence, any register of a document in a sequence, any field
// of a calculation register as leading data, and the refinement of an
// accounting entry's period.

const (
	pathsDefinedType = "42000000-0000-4000-8000-000000000001"
	pathsAsset       = "42000000-0000-4000-8000-000000000002"
	pathsNumber      = "42000000-0000-4000-8000-000000000003"
	pathsOrder       = "42000000-0000-4000-8000-000000000004"
)

// pathsAssets writes a catalog searched by an attribute whose type is a
// defined type, and the defined type standing for types.
func pathsAssets(t *testing.T, root, types, indexing string) {
	t.Helper()
	writeMetadata(t, root, DefinedTypeKind, pathsDefinedType, "format: 1\nid: "+pathsDefinedType+
		"\nname: ИнвентарныйНомер\ntitle: {ru: Инвентарный номер}\ntypes: "+types+"\n")
	writeMetadata(t, root, CatalogKind, pathsAsset, "format: 1\nid: "+pathsAsset+"\nname: ОбъектыЭксплуатации\ntitle: {ru: Объекты}\n"+
		"code: {type: string, length: 9, auto: true}\ndescription_length: 150\n"+
		"attributes:\n  - {id: "+pathsNumber+", name: ИнвентарныйНомер, title: {ru: Номер}, types: [{kind: defined-type, reference: "+pathsDefinedType+"}]"+indexing+"}\n"+
		"input_by_string:\n  - {standard: Наименование}\n  - {attribute: ИнвентарныйНомер}\n")
}

// A searched attribute of a defined type is held to what the type stands for:
// a string or a number is searched by, anything else is not, and the index is
// asked for as of any other.
//
// Defect caught: the defined type refused for being a defined type (erp
// ОбъектыЭксплуатации lost whole), or accepted whatever it stands for, or
// accepted without an index.
func TestInputByStringTakesADefinedTypeForWhatItStandsFor(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	pathsAssets(t, root, "[{kind: string, length: 15}]", ", indexing: index")
	catalog, err := Load(root)
	if err != nil {
		t.Fatalf("a defined string of fifteen is refused: %v", err)
	}
	if notes := catalog.Notes(); len(notes) != 0 {
		t.Fatalf("a search by a defined string carries notes: %+v", notes)
	}
	for name, types := range map[string]string{
		"a date":      "[{kind: date, date_parts: date}]",
		"a composite": "[{kind: string, length: 15}, {kind: number, precision: 10}]",
		"a reference": "[{kind: catalog, reference: " + pathsAsset + "}]",
		"a boolean":   "[{kind: boolean}]",
	} {
		root := metadataProject(t)
		pathsAssets(t, root, types, ", indexing: index")
		if _, err := Load(root); err == nil || !strings.Contains(err.Error(), "input by string attribute ИнвентарныйНомер") {
			t.Errorf("a search by a defined type standing for %s: %v", name, err)
		}
	}
	root = metadataProject(t)
	pathsAssets(t, root, "[{kind: string, length: 15}]", "")
	if _, err := Load(root); err == nil || !strings.Contains(err.Error(), "must be indexed") {
		t.Fatalf("a search by a defined type without an index: %v", err)
	}
}

// The same on a document, which embeds its search in another group than a
// catalog does.
//
// Defect caught: the load-time check found the searched fields of one group
// and not of the other, so a document searched by a defined date loaded.
func TestADocumentSearchedByADefinedTypeIsCheckedToo(t *testing.T) {
	t.Parallel()
	for types, accepted := range map[string]bool{"[{kind: number, precision: 10}]": true, "[{kind: date, date_parts: date}]": false} {
		root := metadataProject(t)
		writeMetadata(t, root, DefinedTypeKind, pathsDefinedType, "format: 1\nid: "+pathsDefinedType+
			"\nname: НомерЗаказа\ntitle: {ru: Номер заказа}\ntypes: "+types+"\n")
		writeMetadata(t, root, DocumentKind, pathsOrder, "format: 1\nid: "+pathsOrder+"\nname: Заказ\ntitle: {ru: Заказ}\n"+
			"number: {type: string, length: 11, periodicity: year}\n"+
			"attributes:\n  - {id: "+pathsNumber+", name: НомерКлиента, title: {ru: Номер}, types: [{kind: defined-type, reference: "+pathsDefinedType+"}], indexing: index}\n"+
			"input_by_string:\n  - {attribute: НомерКлиента}\n")
		_, err := Load(root)
		if accepted && err != nil || !accepted && (err == nil || !strings.Contains(err.Error(), "input by string attribute НомерКлиента")) {
			t.Errorf("a document searched by a defined %s: err = %v", types, err)
		}
	}
}

// constantsLinked writes two constants, the second taking a choice parameter
// from the first - or from the constant named by target.
func constantsLinked(t *testing.T, root, target string) (string, string) {
	t.Helper()
	currencies, source, linked := uuid.MustNew().String(), uuid.MustNew().String(), uuid.MustNew().String()
	catalogNamed(t, root, currencies, "Валюты")
	writeMetadata(t, root, ConstantKind, source, "format: 1\nid: "+source+"\nname: ВалютаСебестоимости\ntitle: {ru: Валюта}\n"+
		"types: [{kind: catalog, reference: "+currencies+"}]\n")
	if target == "" {
		target = source
	}
	writeMetadata(t, root, ConstantKind, linked, "format: 1\nid: "+linked+"\nname: ВидЦеныПлановойСтоимости\ntitle: {ru: Вид цены}\n"+
		"types: [{kind: string, length: 10}]\n"+
		"choice:\n  parameter_links: [{name: Отбор.ВалютаЦены, source: {constant: "+target+"}, change: dont-change}]\n")
	return source, linked
}

// A constant takes a choice parameter from another constant, as erp's
// ВидЦеныПлановойСтоимостиМатериаловРабот does: the path is a constant by its
// identifier, carried, resolved at load, and copied rather than shared.
//
// Defect caught: the link refused as naming no field of the constant (erp's
// constant lost whole); a constant that is not there accepted silently
// instead of listed as unresolved; the path shared between a copy and the
// catalog.
func TestAConstantTakesAChoiceParameterFromAnotherConstant(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	source, _ := constantsLinked(t, root, "")
	catalog, err := Load(root)
	if err != nil {
		t.Fatalf("a link to another constant is refused: %v", err)
	}
	if notes, unresolved := catalog.Notes(), catalog.UnresolvedReferences(); len(notes) != 0 || len(unresolved) != 0 {
		t.Fatalf("a link to a constant that is there is noted: %+v %+v", notes, unresolved)
	}
	constant, ok := catalog.Constant("ВидЦеныПлановойСтоимости")
	if !ok || len(constant.Choice.ParameterLinks) != 1 {
		t.Fatalf("the link was not carried: %+v", constant.Choice)
	}
	link := constant.Choice.ParameterLinks[0]
	if link.Source.Constant == nil || link.Source.Constant.String() != source || link.Change != ValueChangeDontChange {
		t.Fatalf("the link lost its constant: %+v", link)
	}
	*constant.Choice.ParameterLinks[0].Source.Constant = uuid.UUID{}
	again, _ := catalog.Constant("ВидЦеныПлановойСтоимости")
	if again.Choice.ParameterLinks[0].Source.Constant.String() != source {
		t.Fatal("a copy of the constant shares its path with the catalog")
	}

	missing := uuid.MustNew().String()
	root = metadataProject(t)
	constantsLinked(t, root, missing)
	catalog, err = Load(root)
	if err != nil {
		t.Fatalf("a link to a deleted constant is refused, not carried: %v", err)
	}
	unresolved := catalog.UnresolvedReferences()
	if len(unresolved) != 1 || unresolved[0].ID.String() != missing || !strings.Contains(unresolved[0].Where, "Отбор.ВалютаЦены") {
		t.Fatalf("a link to a deleted constant is not listed: %+v", unresolved)
	}
}

// A path to a constant names the constant and nothing beside it.
//
// Defect caught: a path naming a constant and an attribute at once accepted,
// one of the two then silently winning; a zero constant accepted.
func TestAPathToAConstantNamesOnlyTheConstant(t *testing.T) {
	t.Parallel()
	for name, source := range map[string]string{
		"and an attribute":  "{constant: " + uuid.MustNew().String() + ", attribute: " + uuid.MustNew().String() + "}",
		"and a field":       "{constant: " + uuid.MustNew().String() + ", standard: Дата}",
		"and a table part":  "{constant: " + uuid.MustNew().String() + ", table_part: " + uuid.MustNew().String() + "}",
		"kept as written":   "{constant: " + uuid.MustNew().String() + ", unresolved: \"0\"}",
		"a zero identifier": "{constant: 00000000-0000-0000-0000-000000000000}",
	} {
		id := uuid.MustNew().String()
		body := "format: 1\nid: " + id + "\nname: Константа\ntitle: {ru: Константа}\ntypes: [{kind: string, length: 10}]\n" +
			"choice:\n  parameter_links: [{name: Отбор.Вид, source: " + source + "}]\n"
		if _, err := DecodeConstant("constant.yaml", strings.NewReader(body), metadataConfiguration()); err == nil {
			t.Errorf("a path naming a constant %s is accepted", name)
		}
	}
}

const (
	pathsDocument = "43000000-0000-4000-8000-000000000001"
	pathsOther    = "43000000-0000-4000-8000-000000000002"
	pathsJournal  = "43000000-0000-4000-8000-000000000003"
	pathsSequence = "43000000-0000-4000-8000-000000000004"
	pathsAuthor   = "43000000-0000-4000-8000-000000000005"
)

// pathsDocuments writes two documents; the first has an attribute Автор.
func pathsDocuments(t *testing.T, root string) {
	t.Helper()
	writeMetadata(t, root, DocumentKind, pathsDocument, "format: 1\nid: "+pathsDocument+"\nname: Заметка\ntitle: {ru: Заметка}\n"+
		"number: {type: string, length: 11, periodicity: year}\n"+
		"attributes:\n  - {id: "+pathsAuthor+", name: Автор, title: {ru: Автор}, types: [{kind: string, length: 10}]}\n")
	writeMetadata(t, root, DocumentKind, pathsOther, "format: 1\nid: "+pathsOther+"\nname: Письмо\ntitle: {ru: Письмо}\n"+
		"number: {type: string, length: 11, periodicity: year}\n")
}

func pathsJournalBody(documents, column string) string {
	return "format: 1\nid: " + pathsJournal + "\nname: Журнал\ntitle: {ru: Журнал}\ndocuments: [" + documents + "]\n" +
		"columns:\n  - {id: " + uuid.MustNew().String() + ", name: Графа, title: {ru: Графа}, " + column + "}\n"
}

// A journal column shows a standard field of a document beside an attribute
// of another, and a sequence dimension takes one: both carried by document
// and name, both noted.
//
// Defect caught: a standard field refused (the column could only name
// attributes); a column showing two fields of one document - one attribute
// and one standard - accepted; a field of a document the journal or the
// sequence does not list accepted; a name that is no standard field accepted.
func TestAJournalAndASequenceTakeAStandardFieldOfADocument(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	pathsDocuments(t, root)
	writeMetadata(t, root, DocumentJournalKind, pathsJournal, pathsJournalBody(pathsDocument+", "+pathsOther,
		"references: ["+pathsAuthor+"], standard_references: [{document: "+pathsOther+", standard: Дата}]"))
	writeMetadata(t, root, SequenceKind, pathsSequence, "format: 1\nid: "+pathsSequence+"\nname: Последовательность\ntitle: {ru: Последовательность}\n"+
		"documents: ["+pathsDocument+", "+pathsOther+"]\n"+
		"dimensions:\n  - {id: "+uuid.MustNew().String()+", name: Номер, title: {ru: Номер}, types: [{kind: string, length: 11}], "+
		"document_attributes: ["+pathsAuthor+"], document_standard_attributes: [{document: "+pathsOther+", standard: Number}]}\n")
	catalog, err := Load(root)
	if err != nil {
		t.Fatalf("a standard field of a document is refused: %v", err)
	}
	other, _ := uuid.Parse(pathsOther)
	journal, ok := catalog.DocumentJournal("Журнал")
	if !ok || len(journal.Columns) != 1 ||
		!slices.Equal(journal.Columns[0].StandardReferences, []DocumentStandardField{{Document: other, Standard: "Дата"}}) {
		t.Fatalf("the column lost its standard field: %+v", journal.Columns)
	}
	var noted []string
	for _, note := range catalog.Notes() {
		if note.Kind != NoteStandardFieldOfDocument {
			t.Fatalf("unexpected note %+v", note)
		}
		noted = append(noted, note.Written)
	}
	if !slices.Equal(noted, []string{"Дата", "Number"}) {
		t.Fatalf("notes = %v, want the journal's Дата and the sequence's Number", noted)
	}

	for name, column := range map[string]string{
		"two fields of one document": "references: [" + pathsAuthor + "], standard_references: [{document: " + pathsDocument + ", standard: Дата}]",
		"a document not listed":      "standard_references: [{document: " + uuid.MustNew().String() + ", standard: Дата}]",
		"a field named twice":        "standard_references: [{document: " + pathsOther + ", standard: Дата}, {document: " + pathsOther + ", standard: Date}]",
		"no such standard field":     "standard_references: [{document: " + pathsOther + ", standard: Автор}]",
		"no document":                "standard_references: [{standard: Дата}]",
	} {
		root := metadataProject(t)
		pathsDocuments(t, root)
		writeMetadata(t, root, DocumentJournalKind, pathsJournal, pathsJournalBody(pathsDocument+", "+pathsOther, column))
		if _, err := Load(root); err == nil {
			t.Errorf("a column with %s is accepted", name)
		}
	}
	// What the list itself must be is checked where it is read: a journal
	// column would catch a repeat by its one-field-per-document rule, a
	// sequence dimension has no such rule.
	for name, fields := range map[string]string{
		"a field named twice":    "[{document: " + pathsOther + ", standard: Дата}, {document: " + pathsOther + ", standard: Date}]",
		"no such standard field": "[{document: " + pathsOther + ", standard: Автор}]",
		"no document":            "[{standard: Дата}]",
	} {
		body := "format: 1\nid: " + pathsSequence + "\nname: Последовательность\ntitle: {ru: Последовательность}\n" +
			"dimensions:\n  - {id: " + uuid.MustNew().String() + ", name: Номер, title: {ru: Номер}, types: [{kind: string, length: 11}], " +
			"document_standard_attributes: " + fields + "}\n"
		if _, err := DecodeSequence("sequence.yaml", strings.NewReader(body), metadataConfiguration()); err == nil ||
			!strings.Contains(err.Error(), "document_standard_attributes[") {
			t.Errorf("a sequence dimension with %s: %v", name, err)
		}
	}
	root = metadataProject(t)
	pathsDocuments(t, root)
	writeMetadata(t, root, SequenceKind, pathsSequence, "format: 1\nid: "+pathsSequence+"\nname: Последовательность\ntitle: {ru: Последовательность}\n"+
		"documents: ["+pathsDocument+"]\n"+
		"dimensions:\n  - {id: "+uuid.MustNew().String()+", name: Номер, title: {ru: Номер}, types: [{kind: string, length: 11}], "+
		"document_standard_attributes: [{document: "+pathsOther+", standard: Номер}]}\n")
	if _, err := Load(root); err == nil || !strings.Contains(err.Error(), "does not follow") {
		t.Fatalf("a sequence taking a field of a document it does not follow: %v", err)
	}
}

// A sequence watches any register a document writes to: the demonstration
// configuration's watches an accounting register, and the help names no kind.
// Its dimension is matched against a dimension of that register.
//
// Defect caught: an accounting or calculation register refused as an
// unknown register; a dimension of it refused as a dimension of no register
// watched; a register that is not there accepted.
func TestASequenceWatchesAnAccountingRegister(t *testing.T) {
	t.Parallel()
	sequence := func(movement, dimension string) string {
		return "format: 1\nid: " + pathsSequence + "\nname: Последовательность\ntitle: {ru: Последовательность}\n" +
			"documents: [" + entriesDocument + "]\nmovements: [" + movement + "]\n" +
			"dimensions:\n  - {id: " + uuid.MustNew().String() + ", name: Организация, title: {ru: Организация}, " +
			"types: [{kind: catalog, reference: " + entriesCompanies + "}], register_dimensions: [" + dimension + "]}\n"
	}
	root := entriesProject(t, false, entriesStandardFields)
	writeMetadata(t, root, SequenceKind, pathsSequence, sequence(entriesRegister, entriesCompany))
	if _, err := Load(root); err != nil {
		t.Fatalf("a sequence watching an accounting register is refused: %v", err)
	}
	root = entriesProject(t, false, entriesStandardFields)
	writeMetadata(t, root, SequenceKind, pathsSequence, sequence(uuid.MustNew().String(), entriesCompany))
	if _, err := Load(root); err == nil || !strings.Contains(err.Error(), "unknown register") {
		t.Fatalf("a sequence watching a register that is not there: %v", err)
	}
	root = entriesProject(t, false, entriesStandardFields)
	writeMetadata(t, root, SequenceKind, pathsSequence, sequence(entriesRegister, entriesSum))
	if _, err := Load(root); err == nil || !strings.Contains(err.Error(), "not a dimension of any register") {
		t.Fatalf("a sequence matched against a resource: %v", err)
	}
}

// The same for a calculation register, the fourth kind a document writes to.
//
// Defect caught: the calculation register missing from the kinds a sequence
// may watch.
func TestASequenceWatchesACalculationRegister(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	pathsDocuments(t, root)
	chart := noteChart(t, root, ChartOfCalculationTypesKind, "")
	register, person := uuid.MustNew().String(), uuid.MustNew().String()
	writeMetadata(t, root, CalculationRegisterKind, register, "format: 1\nid: "+register+"\nname: Начисления\ntitle: {ru: Начисления}\n"+
		"chart_of_calculation_types: "+chart+"\nperiodicity: month\n"+
		"dimensions:\n  - {id: "+person+", name: Лицо, title: {ru: Лицо}, types: [{kind: string, length: 10}]}\n"+
		"resources:\n  - {id: "+uuid.MustNew().String()+", name: Сумма, title: {ru: Сумма}, types: [{kind: number, precision: 15, scale: 2}]}\n")
	writeMetadata(t, root, SequenceKind, pathsSequence, "format: 1\nid: "+pathsSequence+"\nname: Последовательность\ntitle: {ru: Последовательность}\n"+
		"documents: ["+pathsDocument+"]\nmovements: ["+register+"]\n"+
		"dimensions:\n  - {id: "+uuid.MustNew().String()+", name: Лицо, title: {ru: Лицо}, types: [{kind: string, length: 10}], "+
		"document_attributes: ["+pathsAuthor+"], register_dimensions: ["+person+"]}\n")
	if _, err := Load(root); err != nil {
		t.Fatalf("a sequence watching a calculation register is refused: %v", err)
	}
}

// Leading data are any field of a calculation register: a resource and an
// attribute are carried and noted, a dimension is carried without a note,
// and an identifier of no field is refused.
//
// Defect caught: a resource refused as leading data; accepted without a note;
// a dimension noted; a field of nothing accepted.
func TestLeadingDataAreAnyFieldOfACalculationRegister(t *testing.T) {
	t.Parallel()
	person, sum, basis := uuid.MustNew().String(), uuid.MustNew().String(), uuid.MustNew().String()
	write := func(leading string) string {
		root := metadataProject(t)
		chart := noteChart(t, root, ChartOfCalculationTypesKind, "")
		id := uuid.MustNew().String()
		writeMetadata(t, root, CalculationRegisterKind, id, "format: 1\nid: "+id+"\nname: Начисления\ntitle: {ru: Начисления}\n"+
			"chart_of_calculation_types: "+chart+"\nperiodicity: month\n"+
			"dimensions:\n  - {id: "+person+", name: Лицо, title: {ru: Лицо}, types: [{kind: string, length: 10}]}\n"+
			"resources:\n  - {id: "+sum+", name: Сумма, title: {ru: Сумма}, types: [{kind: number, precision: 15, scale: 2}]}\n"+
			"attributes:\n  - {id: "+basis+", name: Основание, title: {ru: Основание}, types: [{kind: string, length: 10}]}\n"+
			"recalculations:\n  - {id: "+uuid.MustNew().String()+", name: Перерасчет, title: {ru: Перерасчёт}, dimensions: [{id: "+uuid.MustNew().String()+
			", name: Лицо, title: {ru: Лицо}, register_dimension: "+person+", leading_data: ["+leading+"]}]}\n")
		return root
	}
	for leading, noted := range map[string][]string{
		person:                             nil,
		sum:                                {sum},
		sum + ", " + basis:                 {sum, basis},
		person + ", " + basis:              {basis},
		person + ", " + sum + ", " + basis: {sum, basis},
	} {
		catalog, err := Load(write(leading))
		if err != nil {
			t.Fatalf("leading data [%s] refused: %v", leading, err)
		}
		var got []string
		for _, note := range catalog.Notes() {
			if note.Kind != NoteLeadingDataNotDimension {
				t.Fatalf("unexpected note %+v", note)
			}
			got = append(got, note.Written)
		}
		if !slices.Equal(got, noted) {
			t.Errorf("leading data [%s]: notes %v, want %v", leading, got, noted)
		}
	}
	if _, err := Load(write(uuid.MustNew().String())); err == nil || !strings.Contains(err.Error(), "not a field of any calculation register") {
		t.Fatalf("leading data naming no field: %v", err)
	}
}

// An accounting entry has УточнениеПериода while the register keeps a
// refinement of the period: its description is accepted then, refused at a
// length of 0, and the name is the platform's whatever the length.
//
// Defect caught: the description refused on a register with a refinement
// (the field missing from the entry's standard fields); accepted on one
// without; an attribute of the register allowed to take the name.
func TestAnAccountingEntryRefinesItsPeriodWhenTheRegisterDoes(t *testing.T) {
	t.Parallel()
	described := entriesStandardFields + "\nstandard_attributes:\n  - {name: УточнениеПериода, title: {ru: Уточнение}}"
	root := entriesProject(t, false, "period_adjustment_length: 2\n"+described)
	catalog, err := Load(root)
	if err != nil {
		t.Fatalf("a description of the refinement is refused: %v", err)
	}
	if notes := catalog.Notes(); len(notes) != 0 {
		t.Fatalf("a described refinement carries notes: %+v", notes)
	}
	root = entriesProject(t, false, described)
	if _, err := Load(root); err == nil {
		t.Fatal("a description of the refinement on a register without one is accepted")
	}
	root = entriesProject(t, false, "period_adjustment_length: 2\n"+entriesStandardFields+
		"\nattributes:\n  - {id: "+uuid.MustNew().String()+", name: PeriodAdjustment, title: {ru: Уточнение}, types: [{kind: string, length: 10}]}")
	if _, err := Load(root); err == nil {
		t.Fatal("an attribute named PeriodAdjustment is accepted")
	}
	root = entriesProject(t, false, entriesStandardFields+
		"\nattributes:\n  - {id: "+uuid.MustNew().String()+", name: УточнениеПериода, title: {ru: Уточнение}, types: [{kind: string, length: 10}]}")
	if _, err := Load(root); err == nil {
		t.Fatal("an attribute named УточнениеПериода is accepted on a register without a refinement")
	}
}
