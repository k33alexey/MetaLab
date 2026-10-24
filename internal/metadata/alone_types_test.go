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

// The types of the platform known by name that the editor of a defined type
// takes only by themselves - the PDF document, the line, the border and the
// six parts of a chart (opis of 14.10.2026, opredelyaemyj-tip.md) - are
// refused beside another type, whatever prefix of the space the export chose
// (d5p1, d7p1, opis of 23.10.2026); alone they are taken, and another type of
// the platform beside a string is taken as before. The names are written here.
// Defects caught: a name compared with its prefix, so the export's own choice
// lets it through; a part of a chart missed; Отбор refused.
func TestPlatformTypesByNameStandAloneInADefinedType(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"pdfdoc:PDFDocument", "v8ui:Line", "v8ui:Border", "d5p1:ChartLabelArea", "d7p1:ChartScale",
		"d5p1:ChartAxis", "d5p1:ChartColorPaletteDescription", "d5p1:ChartReferenceLine", "d5p1:ChartReferenceBand"} {
		item := Type{Kind: PlatformType, Name: name}
		issues := validateTypesIn("types", []Type{{Kind: StringType, Length: 10}, item}, placeDefinedType)
		if len(issues) != 1 || !strings.Contains(issues[0], "types[1] "+name+" stands only by itself in a defined type") {
			t.Errorf("%s beside a string: %v", name, issues)
		}
		if issues := validateTypesIn("types", []Type{item}, placeDefinedType); len(issues) != 0 {
			t.Errorf("%s alone: %v", name, issues)
		}
		if issues := validateTypesIn("types", []Type{{Kind: StringType, Length: 10}, item}, placeRunningObject); len(issues) != 0 {
			t.Errorf("%s beside a string in a data processor: %v", name, issues)
		}
	}
	if issues := validateTypesIn("types", []Type{{Kind: StringType, Length: 10}, {Kind: PlatformType, Name: "cfg:Filter"}}, placeDefinedType); len(issues) != 0 {
		t.Errorf("a filter beside a string: %v", issues)
	}
}
