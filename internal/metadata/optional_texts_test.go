package metadata

import (
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// An optional text - a tooltip, an explanation, a presentation - may be left
// empty, but what is written in it is checked like any other text: a key that
// cannot be a language code is refused. Each place checks its texts on its own
// path, so each one is tried.
func TestAnOptionalTextWithABrokenLanguageCodeIsRefused(t *testing.T) {
	t.Parallel()
	const broken = `{"r u": Подсказка}`
	catalog := func(extra string) func(t *testing.T) error {
		return func(t *testing.T) error {
			id := uuid.MustNew().String()
			_, err := DecodeCatalog("catalog.yaml", strings.NewReader("format: 1\nid: "+id+"\nname: Товары\ntitle: {ru: Товары}\n"+
				"code: {type: string, length: 9, auto: true}\ndescription_length: 150\n"+extra), metadataConfiguration())
			return err
		}
	}
	tests := []struct {
		name string
		try  func(t *testing.T) error
		want string
	}{
		{"presentation of an object", catalog("object_presentation: " + broken + "\n"), "object_presentation.r u is not a language code"},
		{"tooltip of an attribute", catalog("attributes:\n  - {id: " + uuid.MustNew().String() + ", name: Цена, title: {ru: Цена}, types: [{kind: boolean}], presentation: {tooltip: " + broken + "}}\n"),
			"presentation.tooltip.r u is not a language code"},
		{"tooltip of a command", catalog("commands:\n  - {id: " + uuid.MustNew().String() + ", name: Подбор, title: {ru: Подбор}, tooltip: " + broken + "}\n"),
			"tooltip.r u is not a language code"},
		{"explanation of a filter criterion", func(t *testing.T) error {
			id := uuid.MustNew().String()
			_, err := DecodeFilterCriterion("criterion.yaml", strings.NewReader("format: 1\nid: "+id+"\nname: Связанные\ntitle: {ru: Связанные}\ntypes: []\n"+
				"explanation: "+broken+"\n"), metadataConfiguration())
			return err
		}, "explanation.r u is not a language code"},
		{"tooltip of a command group", func(t *testing.T) error {
			id := uuid.MustNew().String()
			_, err := DecodeCommandGroup("group.yaml", strings.NewReader("format: 1\nid: "+id+"\nname: Обмены\ntitle: {ru: Обмены}\n"+
				"category: actions-panel\ntooltip: "+broken+"\n"), metadataConfiguration())
			return err
		}, "tooltip.r u is not a language code"},
		{"explanation of a form", func(t *testing.T) error {
			form := ManagedForm{Format: CurrentFormat, ID: uuid.MustNew(), Name: "Форма", Title: LocalizedText{"ru": "Форма"}, Kind: ObjectForm,
				Explanation: LocalizedText{"r u": "Подсказка"}}
			return ValidateManagedForm("form.yaml", form, managedFormConfiguration())
		}, "explanation.r u is not a language code"},
		{"title of a form element", func(t *testing.T) error {
			form := ManagedForm{Format: CurrentFormat, ID: uuid.MustNew(), Name: "Форма", Title: LocalizedText{"ru": "Форма"}, Kind: ObjectForm,
				Items: []ManagedFormElement{{ID: uuid.MustNew(), Name: "Группа", Kind: FormElementUsualGroup, Orientation: FormVertical, Title: LocalizedText{"r u": "Группа"}}}}
			return ValidateManagedForm("form.yaml", form, managedFormConfiguration())
		}, "title.r u is not a language code"},
		{"tooltip of a route point", func(t *testing.T) error {
			_, err := Load(routeOnlyProcess(t, `
  points:
    - id: b0000000-0000-4000-8000-000000000030
      name: Старт
      kind: start
      look: {tooltip: `+broken+`}
    - {id: b0000000-0000-4000-8000-000000000034, name: Завершение, kind: completion}
  transitions:
    - {from: Старт, to: Завершение}
`))
			return err
		}, "tooltip.r u is not a language code"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if err := test.try(t); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want it to say %q", err, test.want)
			}
		})
	}
}
