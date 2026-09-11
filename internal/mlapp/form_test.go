package mlapp

import (
	"testing"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestGeneratedObjectFormUsesStandardComponents(t *testing.T) {
	t.Parallel()
	descriptor := metadata.FormDescriptor{
		Kind: metadata.ObjectForm, ObjectKind: metadata.DocumentKind,
		ObjectID: uuid.MustNew(), ObjectName: "Продажа", Title: "Продажа", Generated: true,
		Fields: []metadata.FormField{
			{Name: "Date", Title: "Дата", Types: []metadata.Type{{Kind: metadata.DateType}}},
			{Name: "Posted", Title: "Проведён", Types: []metadata.Type{{Kind: metadata.BooleanType}}, ReadOnly: true},
		},
		TableParts: []metadata.FormTablePart{{Name: "Товары", Title: "Товары", Columns: []metadata.FormField{{Name: "Количество", Title: "Количество", Types: []metadata.Type{{Kind: metadata.NumberType}}}}}},
		Commands:   []metadata.FormCommand{{Name: "Save", Title: "Записать"}, {Name: "Close", Title: "Закрыть"}},
	}
	form, err := FormFromMetadata(descriptor, nil, "ru")
	if err != nil {
		t.Fatal(err)
	}
	if err := validateForm(form); err != nil {
		t.Fatal(err)
	}
	if len(form.Items) != 2 || form.Items[0].Kind != "group" || form.Items[1].Kind != "table" {
		t.Fatalf("items=%+v", form.Items)
	}
	if form.Items[0].Children[0].InputType != "datetime-local" || form.Items[0].Children[1].InputType != "checkbox" || form.Items[1].Children[0].DataPath != "Количество" {
		t.Fatalf("field components=%+v", form.Items)
	}
}

func TestGeneratedListAndChoiceFormsUseTableAndCommands(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		kind    metadata.FormKind
		command string
	}{
		{metadata.ListForm, "Create"},
		{metadata.ChoiceForm, "Choose"},
	} {
		descriptor := metadata.FormDescriptor{
			Kind: test.kind, ObjectKind: metadata.CatalogKind, ObjectID: uuid.MustNew(),
			ObjectName: "Товары", Title: "Товары", Generated: true,
			Fields:   []metadata.FormField{{Name: "Description", Title: "Наименование", Types: []metadata.Type{{Kind: metadata.StringType}}}},
			Commands: []metadata.FormCommand{{Name: test.command, Title: test.command}},
			List:     metadata.ListSettings{PageSize: 50, SearchFields: []string{"Description"}},
		}
		form, err := FormFromMetadata(descriptor, nil, "ru")
		if err != nil || len(form.Items) != 1 || form.Items[0].Kind != "table" || form.Commands[0].ID != test.command || form.List == nil || form.List.PageSize != 50 || !form.List.SearchEnabled || form.List.SearchFields[0].Name != "Description" || form.List.FilterFields[0].Title != "Наименование" {
			t.Fatalf("kind=%s form=%+v error=%v", test.kind, form, err)
		}
	}
}

func TestCustomFormKeepsLayoutBindingsAndLocalizedCommands(t *testing.T) {
	t.Parallel()
	formID, commandID, fieldID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	descriptor := metadata.FormDescriptor{
		Kind: metadata.ObjectForm, ObjectKind: metadata.CatalogKind, ObjectID: uuid.MustNew(),
		ObjectName: "Товары", Title: "Товары", SourceID: &formID,
	}
	source := metadata.ManagedForm{
		ID: formID, Kind: metadata.ObjectForm, Title: metadata.LocalizedText{"ru": "Карточка товара"},
		Commands: []metadata.ManagedFormCommand{{ID: commandID, Name: "Save", Title: metadata.LocalizedText{"ru": "Записать"}, Action: metadata.FormCommandSave}},
		Items:    []metadata.ManagedFormElement{{ID: fieldID, Name: "Наименование", Kind: metadata.FormElementField, DataPath: "Description", Command: nil}},
	}
	form, err := FormFromMetadata(descriptor, &source, "ru")
	if err != nil {
		t.Fatal(err)
	}
	if form.Title != "Карточка товара" || form.Commands[0].Title != "Записать" || form.Items[0].DataPath != "Description" {
		t.Fatalf("form=%+v", form)
	}
}
