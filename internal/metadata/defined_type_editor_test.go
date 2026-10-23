package metadata

import (
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// definedTypeSource is the description of a defined type holding the types
// written as YAML, "" for no line of types at all.
func definedTypeSource(id, types string) string {
	source := "format: 1\nid: " + id + "\nname: Значение\ntitle: {ru: Значение}\n"
	if types != "" {
		source += "types: " + types + "\n"
	}
	return source
}

// What the editor of the type of a defined type offers, the model takes in a
// defined type: every type of a form but the planner, which that editor does
// not offer, and no type at all - «Произвольный», the first line of the
// editor. The list is written here from the opis of the defined type
// (14.10.2026), not read from the model.
//
// Defect caught: a defined type the designer saves - a dynamic list, a colour,
// Null, «Произвольный» - refused on load; the planner let in, though the
// designer has no way to put it there; a type of a form taking a qualifier.
func TestADefinedTypeHoldsWhatItsEditorOffers(t *testing.T) {
	t.Parallel()
	configuration := catchupConfiguration()
	offered := []string{"dynamic-list", "formatted-string", "formatted-document", "text-document", "picture", "color",
		"font", "graphical-schema", "geographical-schema", "gantt-chart", "dendrogram", "null-type"}
	for _, kind := range offered {
		if _, err := DecodeDefinedType("defined-type.yaml", strings.NewReader(definedTypeSource(uuid.MustNew().String(), "[{kind: "+kind+"}]")), configuration); err != nil {
			t.Errorf("%s refused in a defined type: %v", kind, err)
		}
		_, err := DecodeDefinedType("defined-type.yaml", strings.NewReader(definedTypeSource(uuid.MustNew().String(), "[{kind: "+kind+", length: 10}]")), configuration)
		if err == nil || !strings.Contains(err.Error(), "types[0] has unsupported qualifiers") {
			t.Errorf("%s took a qualifier in a defined type: %v", kind, err)
		}
	}
	_, err := DecodeDefinedType("defined-type.yaml", strings.NewReader(definedTypeSource(uuid.MustNew().String(), "[{kind: planner}]")), configuration)
	if err == nil || !strings.Contains(err.Error(), "types[0].kind planner is a type of an attribute of a form and cannot stand in a defined type") {
		t.Errorf("a planner in a defined type: %v", err)
	}
	for name, types := range map[string]string{"an empty list": "[]", "no line of types": ""} {
		value, err := DecodeDefinedType("defined-type.yaml", strings.NewReader(definedTypeSource(uuid.MustNew().String(), types)), configuration)
		if err != nil {
			t.Errorf("«Произвольный» as %s refused: %v", name, err)
		} else if len(value.Types) != 0 {
			t.Errorf("«Произвольный» as %s read as %v", name, value.Types)
		}
	}
}

// A stored field given a defined type that holds a type of a form or no type
// at all loads, as the designer lets such a defined type be given to a stored
// attribute, and is refused when the database is built, by a message that
// names why - a type of a form beside a string as well, which would otherwise
// get one composite column without a word.
//
// Defect caught: a column built for a colour or a formatted string, a
// composite column taking one beside a string, and an arbitrary defined type
// refused with a message of no reason.
func TestAStoredFieldRefusesWhatOnlyADefinedTypeMayHold(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct{ types, want string }{
		"a colour":                     {"[{kind: color}]", "type color is a type of what a form shows and cannot be stored"},
		"Null":                         {"[{kind: null-type}]", "type null-type is a type of what a form shows and cannot be stored"},
		"a dynamic list":               {"[{kind: dynamic-list}]", "type dynamic-list is a type of what a form shows and cannot be stored"},
		"a string and a formatted one": {"[{kind: string, length: 10}, {kind: formatted-string}]", "type formatted-string is a type of what a form shows and cannot be stored"},
		"no type":                      {"[]", "the type is arbitrary"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			defined := uuid.MustNew().String()
			writeMetadata(t, root, DefinedTypeKind, defined, definedTypeSource(defined, test.types))
			catalogID := uuid.MustNew().String()
			writeMetadata(t, root, CatalogKind, catalogID, "format: 1\nid: "+catalogID+"\nname: Контрагенты\ntitle: {ru: Контрагенты}\ncode: {type: string, length: 9, auto: true}\ndescription_length: 150\n"+
				"attributes:\n  - {id: "+uuid.MustNew().String()+", name: Значение, title: {ru: Значение}, types: [{kind: defined-type, reference: "+defined+"}]}\n")
			catalog, err := Load(root)
			if err != nil {
				t.Fatalf("a stored field typed by the defined type was refused on load: %v", err)
			}
			if _, err := catalog.ApplicationSchema(); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("the schema: %v, want %q", err, test.want)
			}
		})
	}
}
