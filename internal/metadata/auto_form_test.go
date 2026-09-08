package metadata

import (
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestAutomaticDocumentFormsAndCommands(t *testing.T) {
	t.Parallel()
	documentID, registerID := uuid.MustNew(), uuid.MustNew()
	attributeID, partID, columnID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	catalog := &Catalog{
		Project: project.Project{Languages: []project.Language{{Code: "ru"}, {Code: "uk"}, {Code: "en"}}},
		Documents: []DocumentDefinition{{
			ID: documentID, Name: "Продажа", Title: LocalizedText{"ru": "Продажа", "uk": "Продаж"}, Posting: true,
			Number:     DocumentNumber{Type: StringType, Length: 10},
			Attributes: []Attribute{{ID: attributeID, Name: "Контрагент", Title: LocalizedText{"ru": "Контрагент"}, Types: []Type{{Kind: StringType, Length: 100}}, Required: true}},
			TableParts: []TablePart{{ID: partID, Name: "Товары", Title: LocalizedText{"ru": "Товары"}, Attributes: []Attribute{{ID: columnID, Name: "Количество", Types: []Type{{Kind: NumberType, Precision: 15, Scale: 3}}}}}},
		}},
		AccumulationRegisters: []AccumulationRegisterDefinition{{ID: registerID, Recorders: []uuid.UUID{documentID}}},
		documentByName:        map[string]int{"продажа": 0}, documentByID: map[uuid.UUID]int{documentID: 0},
	}
	form, err := catalog.DocumentForm("продажа", ObjectForm, "uk")
	if err != nil {
		t.Fatal(err)
	}
	if !form.Generated || form.Title != "Продаж" || len(form.Fields) != 4 || len(form.TableParts) != 1 {
		t.Fatalf("form=%+v", form)
	}
	for _, command := range []string{"Save", "Post", "UndoPosting", "Movements", "SetDeletionMark"} {
		if !hasFormCommand(form.Commands, command) {
			t.Fatalf("automatic form has no %s command: %+v", command, form.Commands)
		}
	}
	list, err := catalog.DocumentForm("Продажа", ListForm, "en")
	if err != nil || !list.Generated || len(list.Fields) != 3 || !hasFormCommand(list.Commands, "Create") {
		t.Fatalf("list=%+v error=%v", list, err)
	}
}

func TestCustomFormSuppressesAutomaticLayout(t *testing.T) {
	t.Parallel()
	catalogID, formID := uuid.MustNew(), uuid.MustNew()
	catalog := &Catalog{
		Catalogs:      []CatalogDefinition{{ID: catalogID, Name: "Товары", Title: LocalizedText{"ru": "Товары"}, Forms: ObjectForms{Object: &formID}}},
		catalogByName: map[string]int{"товары": 0}, catalogByID: map[uuid.UUID]int{catalogID: 0},
	}
	form, err := catalog.CatalogForm("Товары", ObjectForm, "ru")
	if err != nil || form.Generated || form.SourceID == nil || *form.SourceID != formID || len(form.Fields) != 0 {
		t.Fatalf("form=%+v error=%v", form, err)
	}
	if _, err := catalog.CatalogForm("Товары", FormKind("invalid"), "ru"); err == nil {
		t.Fatal("invalid form kind was accepted")
	}
}

func hasFormCommand(commands []FormCommand, name string) bool {
	for _, command := range commands {
		if command.Name == name {
			return true
		}
	}
	return false
}

func TestBasicListPageValidation(t *testing.T) {
	t.Parallel()
	zero := uuid.UUID{}
	if err := validateBasicListPage(&zero, 20); err == nil {
		t.Fatal("zero cursor was accepted")
	}
	if err := validateBasicListPage(nil, 101); err == nil {
		t.Fatal("oversized page was accepted")
	}
	if err := validateBasicListPage(nil, 20); err != nil {
		t.Fatal(err)
	}
}
