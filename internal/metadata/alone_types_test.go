package metadata

import (
	"slices"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// aloneProbe is a type of the kind as a type description writes it: the kinds
// that name an object name one.
func aloneProbe(kind TypeKind) Type {
	item := Type{Kind: kind}
	switch kind {
	case StringType:
		item.Length = 10
	case NumberType:
		item.Precision = 10
	}
	if _, named := typeOwners[kind]; named {
		item.Reference = refID(uuid.MustNew().String())
	}
	return item
}

// Some types stand in a type description only by themselves, and which ones
// depends on the place: put beside a string they are refused, naming the type
// and the place; alone they are taken; the types beside which a composite is
// built in the place are taken beside a string. Both lists are written here
// from the opis (14.10.2026: opredelyaemyj-tip.md, «Не входят в составной
// тип»; METADATA-OBJECTS.md, «Типы, которые не входят в составной»), not read
// from the model.
//
// Defect caught: a constant «Булево + ХранилищеЗначения» or a defined type
// «Строка + Цвет» taken, which the designer cannot save; a type the designer
// puts in a composite - a value table in a defined type, Null in a session
// parameter, a manager of a chart of accounts - refused; a place that was not
// gone through in the designer - a form - refusing without a reason.
func TestSomeTypesStandOnlyByThemselves(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		place        typePlace
		alone, mixed []TypeKind
	}{
		{placeStored, []TypeKind{"value-storage", "uuid"},
			[]TypeKind{"number", "date", "boolean", "catalog", "any-ref", "defined-type"}},
		{placeDefinedType, []TypeKind{"value-storage", "uuid", "binary-data", "null-type", "dynamic-list",
			"spreadsheet-document", "formatted-document", "text-document", "picture", "color", "font",
			"graphical-schema", "geographical-schema", "chart", "gantt-chart", "dendrogram", "settings-composer",
			"standard-period", "catalog-manager", "document-manager", "enumeration-manager",
			"document-journal-manager", "information-register-manager", "accumulation-register-manager",
			"calculation-register-manager", "report-manager", "data-processor-manager", "filter-criterion-manager"},
			[]TypeKind{"number", "catalog", "value-table", "value-tree", "value-list", "formatted-string",
				"type-description", "standard-beginning-date", "fixed-structure", "fixed-array", "fixed-map",
				"catalog-object", "document-object", "information-register-record-set",
				"accumulation-register-record-set", "chart-of-characteristic-types-manager",
				"chart-of-accounts-manager", "chart-of-calculation-types-manager", "business-process-manager",
				"task-manager", "exchange-plan-manager", "accounting-register-manager", "constant-value-manager",
				"settings-storage-manager"}},
		{placeSessionParameter, nil, []TypeKind{"value-storage", "uuid", "binary-data", "null-type", "catalog", "any-ref"}},
		{placeFormAttribute, nil, []TypeKind{"uuid", "null-type", "dynamic-list", "value-table", "spreadsheet-document",
			"formatted-document", "text-document", "picture", "value-list", "type-description", "formatted-string"}},
		// The type of a filter criterion: a unique identifier alone (opis of
		// 23.10.2026, kriterij-otbora.md, «Досъёмка»).
		{placeFilterCriterion, []TypeKind{"uuid"}, []TypeKind{"catalog"}},
	} {
		where := placeName(test.place)
		for _, kind := range test.alone {
			beside := []Type{aloneProbe(StringType), aloneProbe(kind)}
			issues := validateTypesIn("types", beside, test.place)
			if len(issues) != 1 || !strings.Contains(issues[0], "types[1].kind "+string(kind)+" stands only by itself "+where) {
				t.Errorf("%s beside a string %s: %v", kind, where, issues)
			}
			if issues := validateTypesIn("types", []Type{aloneProbe(kind)}, test.place); len(issues) != 0 {
				t.Errorf("%s alone %s: %v", kind, where, issues)
			}
		}
		for _, kind := range test.mixed {
			if issues := validateTypesIn("types", []Type{aloneProbe(StringType), aloneProbe(kind)}, test.place); len(issues) != 0 {
				t.Errorf("%s beside a string %s: %v", kind, where, issues)
			}
		}
	}
}

// A constant «Булево + ХранилищеЗначения» is refused on reading, the example
// of the opis, and so is a filter criterion of an identifier and a reference
// (opis of 23.10.2026); and the window of the type marks the types that stand
// alone - two of a stored field, one of a criterion - from the same table, and
// nothing else.
//
// Defect caught: the rule known to validateTypesIn and missed by a reading
// that calls it another way; the window of Studio and the model disagreeing on
// what stands alone.
func TestAConstantRefusesValueStorageBesideAnotherType(t *testing.T) {
	t.Parallel()
	source := "format: 1\nid: " + uuid.MustNew().String() + "\nname: Настройка\ntitle: {ru: Настройка}\ntypes: [{kind: boolean}, {kind: value-storage}]\n"
	_, err := DecodeConstant("constant.yaml", strings.NewReader(source), constantConfiguration())
	if err == nil || !strings.Contains(err.Error(), "types[1].kind value-storage stands only by itself in a field the database stores") {
		t.Fatalf("a constant of a boolean and a value storage: %v", err)
	}
	criterion := "format: 1\nid: " + uuid.MustNew().String() + "\nname: Связанные\ntitle: {ru: Связанные}\ntypes: [{kind: uuid}, {kind: catalog, reference: " + uuid.MustNew().String() + "}]\n"
	if _, err := DecodeFilterCriterion("criterion.yaml", strings.NewReader(criterion), metadataConfiguration()); err == nil ||
		!strings.Contains(err.Error(), "types[0].kind uuid stands only by itself in the type of a filter criterion") {
		t.Fatalf("a criterion of an identifier and a reference: %v", err)
	}
	catalog, err := Load(compositeProject(t))
	if err != nil {
		t.Fatal(err)
	}
	for place, alone := range map[TypeChoicePlace][]TypeKind{
		StoredFieldChoice: {"value-storage", "uuid"}, FilterCriterionChoice: {"uuid"}, CharacteristicValueChoice: nil,
	} {
		offer, _ := catalog.TypeOffer(place)
		var marked []TypeKind
		for _, row := range offer.Rows {
			if row.Alone {
				marked = append(marked, row.Kind)
			}
		}
		if strings.Join(kindStrings(marked), ",") != strings.Join(kindStrings(alone), ",") {
			t.Errorf("%s marks %v alone, want %v", place, marked, alone)
		}
	}
}

func kindStrings(items []TypeKind) []string {
	var result []string
	for _, item := range items {
		result = append(result, string(item))
	}
	return result
}

// A manager that stands alone in a defined type is a type the kind holds.
//
// Defect caught: a manager put in kindAloneInDefinedType under a kind it is
// not of, which the type description would then never name.
func TestAManagerAloneIsTheKinds(t *testing.T) {
	t.Parallel()
	for _, item := range kindInfos {
		for _, kind := range item.aloneInDefinedType {
			if !slices.Contains(item.objectTypes, kind) {
				t.Errorf("%s: %s stands alone and is not a type of the kind", item.kind, kind)
			}
		}
	}
}

// The types of the platform the window offers by rows of their own - the PDF
// document, the line, the border and the six parts of a chart - are kinds of
// their own and stand where the window offers them: a defined type holds all
// nine, each only by itself (opis of 14.10.2026, opredelyaemyj-tip.md, «Не
// входят в составной тип»); an attribute of a form, of a data processor and of
// a report holds the PDF document, beside other types too (obshchee.md,
// «Состав по местам»), and none of the eight; no other place holds any. Written
// by the name of its type XDTO, with whatever prefix of the space the export
// chose (d5p1, d7p1, opis of 23.10.2026), each is refused and the kind to
// write is named; nothing about it is noted. The kinds, the names and the
// places are written here from the opis and the help (8.3.27), not read from
// the model.
//
// Defects caught: one of the nine taken beside a string in a defined type, or
// taken in a place the window does not offer it in; the PDF document refused
// in a form; a name compared with its prefix, so the export's own choice lets
// a second spelling of one type through; a part of a chart missed; Отбор
// refused; a note left on what the model knows.
func TestTheTypesOfTheWindowsRowsAreKindsOfTheirOwn(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		kind, written string
		inMemory      bool
	}{
		{"pdf-document", "pdfdoc:PDFDocument", true},
		{"line", "v8ui:Line", false},
		{"border", "v8ui:Border", false},
		{"chart-label-area", "d5p1:ChartLabelArea", false},
		{"chart-scale", "d7p1:ChartScale", false},
		{"chart-axis", "ChartAxis", false},
		{"chart-color-palette-description", "d5p1:ChartColorPaletteDescription", false},
		{"chart-reference-line", "d5p1:ChartReferenceLine", false},
		{"chart-reference-band", "d5p1:ChartReferenceBand", false},
	} {
		item := Type{Kind: TypeKind(test.kind)}
		beside := []Type{{Kind: StringType, Length: 10}, item}
		if issues := validateTypesIn("types", []Type{item}, placeDefinedType); len(issues) != 0 {
			t.Errorf("%s alone in a defined type: %v", test.kind, issues)
		}
		if issues := validateTypesIn("types", beside, placeDefinedType); len(issues) != 1 ||
			!strings.Contains(issues[0], "types[1].kind "+test.kind+" stands only by itself in a defined type") {
			t.Errorf("%s beside a string in a defined type: %v", test.kind, issues)
		}
		for _, place := range []typePlace{placeFormAttribute, placeRunningObject} {
			if issues := validateTypesIn("types", beside, place); (len(issues) == 0) != test.inMemory {
				t.Errorf("%s beside a string %s: %v", test.kind, placeName(place), issues)
			}
		}
		for _, place := range []typePlace{placeStored, placeSessionParameter, placeCommandParameter, placeFilterCriterion, placeExternalField} {
			if issues := validateTypesIn("types", []Type{item}, place); len(issues) == 0 {
				t.Errorf("%s taken %s", test.kind, placeName(place))
			}
		}
		for _, place := range []typePlace{placeDefinedType, placeFormAttribute, placeRunningObject} {
			issues := validateTypesIn("types", []Type{{Kind: PlatformType, Name: test.written}}, place)
			if len(issues) != 1 || !strings.HasSuffix(issues[0], test.written+" is a type of its own: write it as kind "+test.kind) {
				t.Errorf("%s written by its name %s: %v", test.kind, placeName(place), issues)
			}
		}
	}
	if issues := validateTypesIn("types", []Type{{Kind: StringType, Length: 10}, {Kind: PlatformType, Name: "cfg:Filter"}}, placeDefinedType); len(issues) != 0 {
		t.Errorf("a filter beside a string: %v", issues)
	}

	root := metadataProject(t)
	for name, kind := range map[string]string{"Ось": "chart-axis", "Документ": "pdf-document"} {
		id := uuid.MustNew().String()
		writeMetadata(t, root, DefinedTypeKind, id, "format: 1\nid: "+id+"\nname: "+name+"\ntitle: {ru: "+name+"}\ntypes: [{kind: "+kind+"}]\n")
	}
	catalog, err := Load(root)
	if err != nil {
		t.Fatalf("a defined type of a part of a chart and of a PDF document was refused: %v", err)
	}
	if notes := catalog.Notes(); slices.ContainsFunc(notes, func(note Note) bool { return note.Kind == NotePlatformTypeByName }) {
		t.Errorf("a type of the window's rows is noted: %+v", notes)
	}
}
