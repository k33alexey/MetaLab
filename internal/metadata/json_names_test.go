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
