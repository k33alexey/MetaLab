package metadata

import (
	"slices"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/kinds"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// systemEnumerationsOfTheWindow are the enumerations the editor of a type offers
// in its folder «Системные перечисления», in its order (opis of 24.10.2026,
// obshchee.md, snapshots series-2.517/14 and 16), with the kind the project
// writes each by, the name of its type XDTO and the number of its values
// (help 8.3.27). Written here, not read from the model.
var systemEnumerationsOfTheWindow = []struct {
	name, kind, xdto string
	values           int
}{
	{"ВидДвиженияНакопления", "accumulation-record-kind", "AccumulationRecordType", 2},
	{"ВидСчета", "account-kind", "AccountType", 3},
	{"ВидДвиженияБухгалтерии", "accounting-record-kind", "AccountingRecordType", 2},
	{"ГоризонтальноеПоложение", "horizontal-align", "HorizontalAlign", 5},
	{"ВертикальноеПоложение", "vertical-align", "VerticalAlign", 3},
	{"РежимИзмененияРазмера", "size-change-mode", "SizeChangeMode", 2},
	{"ВидДанныхАнализа", "analysis-data-type", "AnalysisDataType", 2},
	{"МетодКластеризации", "clusterization-method", "ClusterizationMethod", 4},
	{"ТипЕдиницыИнтервалаВремениАнализаДанных", "data-analysis-time-interval-unit-type", "DataAnalysisTimeIntervalUnitType", 19},
	{"ТипЗаполненияТаблицыРезультатаАнализаДанных", "data-analysis-result-table-fill-type", "DataAnalysisResultTableFillType", 4},
	{"ТипИспользованияЧисловыхЗначенийАнализаДанных", "data-analysis-numeric-value-use-type", "DataAnalysisNumericValueUseType", 2},
	{"ТипИсточникаДанныхПоискаАссоциаций", "association-rules-data-source-type", "AssociationRulesDataSourceType", 2},
	{"ТипКолонкиАнализаДанныхДеревоРешений", "data-analysis-column-type-decision-tree", "DataAnalysisColumnTypeDecisionTree", 3},
	{"ТипКолонкиАнализаДанныхКластеризация", "data-analysis-column-type-clusterization", "DataAnalysisColumnTypeClusterization", 5},
	{"ТипКолонкиАнализаДанныхОбщаяСтатистика", "data-analysis-column-type-summary-statistics", "DataAnalysisColumnTypeSummaryStatistics", 2},
	{"ТипКолонкиАнализаДанныхПоискАссоциаций", "data-analysis-column-type-association-rules", "DataAnalysisColumnTypeAssociationRules", 3},
	{"ТипКолонкиАнализаДанныхПоискПоследовательностей", "data-analysis-column-type-sequential-patterns", "DataAnalysisColumnTypeSequentialPatterns", 4},
	{"ТипКолонкиМоделиПрогноза", "prediction-model-column-type", "PredictionModelColumnType", 3},
	{"ТипМерыРасстоянияАнализаДанных", "data-analysis-distance-metric-type", "DataAnalysisDistanceMetricType", 4},
	{"ТипОтсеченияПравилАссоциации", "association-rules-prune-type", "AssociationRulesPruneType", 2},
	{"ТипСтандартизацииАнализаДанных", "data-analysis-standardization-type", "DataAnalysisStandardizationType", 2},
	{"ТипУпрощенияДереваРешений", "decision-tree-simplification-type", "DecisionTreeSimplificationType", 2},
	{"ТипУпорядочиванияПравилАссоциацииАнализаДанных", "data-analysis-association-rules-order-type", "DataAnalysisAssociationRulesOrderType", 3},
	{"ТипУпорядочиванияШаблоновПоследовательностейАнализаДанных", "data-analysis-sequential-patterns-order-type", "DataAnalysisSequentialPatternsOrderType", 2},
}

// The list of the enumerations of the platform is the window's: every one, in
// its order, under the name of its type XDTO and with all its values.
//
// Defect caught: an enumeration the window offers left out - it would come
// back as a type of the platform by name with a note - or put in an order of
// its own, which the window of Studio would show; a name the export writes it
// by mistyped, so a type written by it is not known as its own; a value of the
// help lost.
func TestTheSystemEnumerationsAreTheWindows(t *testing.T) {
	t.Parallel()
	if len(kinds.SystemEnumerations) != len(systemEnumerationsOfTheWindow) {
		t.Fatalf("%d enumerations, the window offers %d", len(kinds.SystemEnumerations), len(systemEnumerationsOfTheWindow))
	}
	for index, want := range systemEnumerationsOfTheWindow {
		got := kinds.SystemEnumerations[index]
		if got.Name.Russian != want.name || got.Kind != want.kind || got.Name.English != want.xdto || len(got.Values) != want.values {
			t.Errorf("enumeration %d = %s %s %s with %d values, want %+v", index, got.Name.Russian, got.Kind, got.Name.English, len(got.Values), want)
		}
	}
}

// Each enumeration of the platform the window offers is a type of its own and
// stands where the window offers it: an attribute of a form, of a data
// processor and of a report and a defined type hold all 24, a session
// parameter the three of accounting, a stored field none (opis of 24.10.2026,
// «Системные перечисления по местам»); written by its name as a type of the
// platform it is refused and the kind to write is named; nothing about it is
// noted.
//
// Defect caught: an enumeration taken in a stored field or in a session
// parameter the window does not offer it in; one refused where the window
// offers it; one written by its name kept beside its own kind, a second
// spelling of one type; a note left on what the model knows.
func TestEachSystemEnumerationStandsWhereTheWindowOffersIt(t *testing.T) {
	t.Parallel()
	session := map[string]bool{"accumulation-record-kind": true, "account-kind": true, "accounting-record-kind": true}
	for _, enumeration := range systemEnumerationsOfTheWindow {
		item := []Type{{Kind: TypeKind(enumeration.kind)}}
		for _, place := range []typePlace{placeFormAttribute, placeRunningObject, placeDefinedType} {
			if issues := validateTypesIn("types", item, place); len(issues) != 0 {
				t.Errorf("%s %s: %v", enumeration.name, placeName(place), issues)
			}
		}
		for _, place := range []typePlace{placeStored, placeCommandParameter, placeFilterCriterion, placeExternalField} {
			if issues := validateTypesIn("types", item, place); len(issues) == 0 {
				t.Errorf("%s taken %s", enumeration.name, placeName(place))
			}
		}
		if issues := validateTypesIn("types", item, placeSessionParameter); (len(issues) == 0) != session[enumeration.kind] {
			t.Errorf("%s in a session parameter: %v", enumeration.name, issues)
		}
		byName := []Type{{Kind: PlatformType, Name: "d5p1:" + enumeration.xdto}}
		issues := validateTypesIn("types", byName, placeFormAttribute)
		if len(issues) != 1 || !strings.HasSuffix(issues[0], "is a type of its own: write it as kind "+enumeration.kind) {
			t.Errorf("%s written by its name: %v", enumeration.name, issues)
		}
	}
	if issues := validateTypesIn("types", []Type{{Kind: PlatformType, Name: "cfg:Filter"}}, placeFormAttribute); len(issues) != 0 {
		t.Errorf("a type of the platform the window does not offer as an enumeration was refused: %v", issues)
	}

	root := metadataProject(t)
	id := uuid.MustNew().String()
	writeMetadata(t, root, DefinedTypeKind, id, "format: 1\nid: "+id+"\nname: Положение\ntitle: {ru: Положение}\n"+
		"types: [{kind: vertical-align}, {kind: data-analysis-time-interval-unit-type}]\n")
	catalog, err := Load(root)
	if err != nil {
		t.Fatalf("a defined type of two enumerations of the platform was refused: %v", err)
	}
	if notes := catalog.Notes(); slices.ContainsFunc(notes, func(note Note) bool { return note.Kind == NotePlatformTypeByName }) {
		t.Errorf("an enumeration of the platform is noted: %+v", notes)
	}
}

// The values of the enumerations the model reads elsewhere - the alignment of
// a value of the settings of a composition and of a point of a route map, the
// type of account - are the help's, written as the project writes them.
//
// Defect caught: a value of the help missing in one of them (justify, the
// alignment of text across the width) or a value of another enumeration
// taken.
func TestTheValuesOfTheSystemEnumerationsAreTheHelps(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		what      string
		got, want []string
	}{
		{"horizontal alignment of a composition", horizontalAligns, []string{"auto", "center", "justify", "left", "right"}},
		{"horizontal alignment of a route", routeHorizontalAligns, []string{"auto", "center", "justify", "left", "right"}},
		{"vertical alignment of a route", routeVerticalAligns, []string{"bottom", "center", "top"}},
		{"type of account", accountTypes, []string{"active", "active-passive", "passive"}},
	} {
		got := slices.Sorted(slices.Values(test.got))
		if !slices.Equal(got, test.want) {
			t.Errorf("%s: %v, want %v", test.what, got, test.want)
		}
	}
}
