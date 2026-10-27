package metadata

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// A subsystem and a common attribute are written in JSON - the snapshot of
// a published version, the windows of Studio - under the names every other
// kind has: the YAML name in camel case (the owner, 27.10.2026, question
// В11 of series 2.595), and read back whole.
// A struct field - the presentation, the choice, the filling - is written
// always, as for every field of every kind.
// Defects caught: a field written under its Go name («IncludeInCommandInterface»)
// or lost on the way back.
func TestASubsystemAndACommonAttributeHaveTheJSONNamesOfEveryKind(t *testing.T) {
	id := uuid.MustNew()
	subsystem := SubsystemDefinition{Format: CurrentFormat, ID: id, Name: "Продажи", Title: LocalizedText{"ru": "Продажи"},
		IncludeHelpInContents: true, Parent: &id, Members: []uuid.UUID{id}, IncludeInCommandInterface: true, UseOneCommand: true,
		Picture: &PictureReference{Standard: "Документ"}}
	attribute := CommonAttributeDefinition{Format: CurrentFormat, ID: id, Name: "Организация", Title: LocalizedText{"ru": "Организация"},
		Types: []Type{{Kind: StringType, Length: 10}}, AutoUse: CommonAttributeAutoUse("use"), DataSeparationValue: &id,
		Content: []CommonAttributeContentItem{{Metadata: id, ConditionalSeparation: &ConditionalSeparation{Constant: &id}}}}
	keys := func(value any) []string {
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]any
		if err := json.Unmarshal(encoded, &fields); err != nil {
			t.Fatal(err)
		}
		var names []string
		for name := range fields {
			names = append(names, name)
		}
		sort.Strings(names)
		return names
	}
	if got, want := strings.Join(keys(subsystem), ","), "commandInterface,format,id,includeHelpInContents,includeInCommandInterface,members,name,parent,picture,title,useOneCommand"; got != want {
		t.Errorf("a subsystem: %s, want %s", got, want)
	}
	if got, want := strings.Join(keys(attribute), ","), "content,dataSeparationValue,format,id,name,title,types,autoUse,choice,filling,presentation"; got != sortedList(want) {
		t.Errorf("a common attribute: %s, want %s", got, sortedList(want))
	}
	item, _ := json.Marshal(attribute.Content[0])
	if string(item) != `{"metadata":"`+id.String()+`","conditionalSeparation":{"constant":"`+id.String()+`"}}` {
		t.Errorf("an item of the content: %s", item)
	}
	for _, value := range []any{subsystem, attribute} {
		encoded, _ := json.Marshal(value)
		back := reflect.New(reflect.TypeOf(value))
		if err := json.Unmarshal(encoded, back.Interface()); err != nil || !reflect.DeepEqual(back.Elem().Interface(), value) {
			t.Errorf("%T read back otherwise: %v", value, err)
		}
	}
}

func sortedList(list string) string {
	names := strings.Split(list, ",")
	sort.Strings(names)
	return strings.Join(names, ",")
}

// The page «Состав» of the window of a subsystem offers 38 kinds and the
// numerators inside «Документы» (docs/block-00/podsistema.md, «Окно»), the
// list written out here: none of the subsystems, the WS references, the
// WebSocket clients, the languages, the external sources of data and the
// sequences.
// Defects caught: a kind the window does not offer, or one it does left out.
func TestTheCompositionOfASubsystemOffersThePrototypesKinds(t *testing.T) {
	want := []Kind{RoleKind, ConstantKind, SessionParameterKind, CommonAttributeKind, CommonModuleKind, EventSubscriptionKind, EnumerationKind,
		ScheduledJobKind, CommandGroupKind, CommonCommandKind, CommonFormKind, CommonTemplateKind, XDTOPackageKind, WebServiceKind, HTTPServiceKind,
		CommonPictureKind, StyleItemKind, StyleKind, FunctionalOptionKind, FunctionalOptionParameterKind, SettingsStorageKind, FilterCriterionKind,
		DefinedTypeKind, CatalogKind, ChartOfCharacteristicTypesKind, ChartOfAccountsKind, ChartOfCalculationTypesKind, TaskKind,
		AccountingRegisterKind, CalculationRegisterKind, ReportKind, DataProcessorKind, NumeratorKind, DocumentJournalKind, ExchangePlanKind,
		BusinessProcessKind, DocumentKind, InformationRegisterKind, AccumulationRegisterKind}
	got := SubsystemCompositionKinds()
	sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })
	sort.Slice(want, func(i, j int) bool { return want[i] < want[j] })
	if !reflect.DeepEqual(got, want) {
		t.Errorf("the kinds of the composition: %v, want %v", got, want)
	}
}

// A document is written in JSON under the names every other kind has - the
// YAML name in camel case (the owner, 27.10.2026, question В11 of series
// 2.595) - and read back whole; a snapshot written before under the Go
// names reads the same, the names differing in the case of the first letter
// alone. The expected names are written from the YAML of a document.
// Defects caught: a field of a document written under its Go name
// («TableParts», «Number») - which the window of a document reads by the
// JSON name and would not find - or lost on the way back.
func TestADocumentHasTheJSONNamesOfEveryKind(t *testing.T) {
	id := uuid.MustNew()
	document := DocumentDefinition{Format: CurrentFormat, ID: id, Name: "Заказ", Title: LocalizedText{"ru": "Заказ"},
		Number:    DocumentNumber{Type: StringType, Length: 9, Auto: true, Unique: true, FixedLength: true, Periodicity: "year"},
		Numerator: &id, Movements: []uuid.UUID{id}, Attributes: []Attribute{{ID: id, Name: "Сумма"}},
		TableParts: []TablePart{{ID: id, Name: "Товары"}}, Commands: []ObjectCommand{{ID: id, Name: "Печать"}},
		Templates: []ObjectTemplate{{ID: id, Name: "Макет"}}}
	encoded, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"format", "id", "name", "title", "number", "numerator", "movements", "attributes", "tableParts", "commands", "templates", "posting"} {
		if _, ok := fields[name]; !ok {
			t.Errorf("a document has no JSON name %q: %s", name, encoded)
		}
	}
	var number map[string]any
	_ = json.Unmarshal(fields["number"], &number)
	if got := strings.Join(sortedKeys(number), ","); got != "auto,fixedLength,length,periodicity,type,unique" {
		t.Errorf("the number of a document: %s", got)
	}
	var back DocumentDefinition
	if err := json.Unmarshal(encoded, &back); err != nil || !reflect.DeepEqual(back, document) {
		t.Errorf("a document read back otherwise: %v", err)
	}
	old := strings.NewReplacer(`"tableParts"`, `"TableParts"`, `"number"`, `"Number"`, `"fixedLength"`, `"FixedLength"`).Replace(string(encoded))
	var before DocumentDefinition
	if err := json.Unmarshal([]byte(old), &before); err != nil || !reflect.DeepEqual(before, document) {
		t.Errorf("a snapshot under the Go names read otherwise: %v", err)
	}
}

func sortedKeys(fields map[string]any) []string {
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
