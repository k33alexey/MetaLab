package metadata

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// TestANewObjectHasThePrototypesValues catches «Добавить» making an object
// with values of its own instead of the prototype's, and one the model
// itself would refuse to read back. The values are written out here from the
// inventory of block 00, not taken from kindNewObjectOf: spravochnik.md
// («Новый справочник»), konstanta.md, obshchij-modul.md, gruppa-komand.md,
// perechislenie.md.
func TestANewObjectHasThePrototypesValues(t *testing.T) {
	t.Parallel()
	configuration := project.Project{Format: 1, ID: uuid.MustNew(), Name: "Demo", Title: project.LocalizedText{"ru": "Demo"}, DefaultLanguage: "ru", Languages: []project.Language{{Code: "ru", Name: "Русский"}}}
	id := uuid.MustNew()
	module := uuid.Derive(id, "module")
	moduleText, _ := project.ModulePath(module)
	cases := []struct {
		kind    Kind
		want    any
		modules []string
	}{
		{CatalogKind, CatalogDefinition{
			Format: CurrentFormat, ID: id, Name: "Новый",
			ObjectChoice:      ObjectChoice{DefaultPresentation: "as-description"},
			Code:              CatalogCode{Type: "string", Length: 9, Auto: true, Unique: true, Series: "whole"},
			DescriptionLength: 25,
			Hierarchy:         Hierarchy{Enabled: false, Kind: "folders-and-items", FoldersOnTop: true, LimitLevels: false, LevelCount: 2},
			Subordination:     "to-items",
		}, nil},
		{ConstantKind, Constant{
			Format: CurrentFormat, ID: id, Name: "Новый",
			Types: []Type{{Kind: "string", Length: 10}}, DataLock: "managed",
			DataHistorySettings: DataHistorySettings{DataHistory: "dont-use"}, UseStandardCommands: true,
			FillChecking: "dont-check", Choice: FieldChoice{QuickChoice: "auto", HistoryOnInput: "auto"},
		}, nil},
		{CommonModuleKind, CommonModuleDefinition{Format: CurrentFormat, ID: id, Name: "Новый", Module: module, Server: true}, []string{moduleText}},
		{CommandGroupKind, CommandGroupDefinition{Format: CurrentFormat, ID: id, Name: "Новый", Category: "navigation-panel", Representation: "auto"}, nil},
		{EnumerationKind, Enumeration{
			Format: CurrentFormat, ID: id, Name: "Новый",
			ChoiceMode: "both-ways", QuickChoice: true, ChoiceHistoryOnInput: "auto", UseStandardCommands: false,
		}, nil},
	}
	for _, item := range cases {
		fresh, ok := NewObject(item.kind, id, "Новый", configuration)
		if !ok {
			t.Errorf("no new %s", item.kind)
			continue
		}
		if !reflect.DeepEqual(fresh.Value, item.want) || !reflect.DeepEqual(fresh.Modules, item.modules) {
			t.Errorf("the new %s is %+v with modules %v,\nwant %+v with %v", item.kind, fresh.Value, fresh.Modules, item.want, item.modules)
		}
	}
	// Every kind made is read back by the load's own reading, wherever its
	// kind keeps a description.
	for _, item := range kindInfos {
		fresh, ok := NewObject(item.kind, id, "Новый", configuration)
		if !ok {
			continue
		}
		description, rights, err := EncodeObject(fresh.Value)
		if err != nil {
			t.Fatal(err)
		}
		relative, _ := project.MetadataPath(string(item.kind), id)
		if keptInFolder(item.kind) {
			relative, _ = project.ObjectMetadataPath(string(item.kind), "Новый")
		}
		if _, _, err := DecodeSource(relative, bytes.NewReader(description), bytes.NewReader(rights), configuration); err != nil {
			t.Errorf("the new %s is refused: %v", item.kind, err)
		}
	}
}
