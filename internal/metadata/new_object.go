package metadata

import (
	"bytes"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// newObject is an object of a kind as «Добавить» makes it: the prototype's
// values of a new one, read off the inventory of block 00 (docs/block-00/
// <вид>.md, «Новый …»). The synonym is empty everywhere - the prototype's new
// object stands with none and its name shown in its place (konstanta.md,
// snapshot novaya-osnovnye) - and the name is the caller's.
type newObject func(id uuid.UUID, name string, configuration project.Project) Fresh

// Fresh is a new object: its description and the modules it comes with,
// empty, by where the project keeps them - a common module's text, which
// the load requires and which opens at once (obshchij-modul.md).
type Fresh struct {
	Value   any
	Modules []string
}

// kindNewObjectOf is the new object of every kind Studio makes, kept beside
// kindInfos rather than in each line of it, as kindUsesOf is; init puts it
// into kindInfo.fresh. A kind missing here is not made yet: its «Добавить»
// comes with its window or palette (owner, 26.10.2026), and so do its values.
// The functional option, the scheduled job and the common command wait for
// a question - the prototype makes a new one without the storage, the method
// and the group, and the model refuses each as «Проверка конфигурации»
// does (series 2.584, В1).
var kindNewObjectOf = map[Kind]newObject{
	// spravochnik.md, «Новый справочник»: a string code of 9, auto-numbered
	// and unique within the whole catalog, a description of 25 standing for
	// the catalog, the hierarchy off with the dialog's values of one -
	// folders and items, folders on top, two levels with no limit - and
	// subordination to items. The prototype writes these on every catalog,
	// owned and hierarchical or not (NoteInactiveHierarchy).
	CatalogKind: func(id uuid.UUID, name string, _ project.Project) Fresh {
		return Fresh{Value: CatalogDefinition{
			Format: CurrentFormat, ID: id, Name: name,
			ObjectChoice:      ObjectChoice{DefaultPresentation: PresentAsDescription},
			Code:              CatalogCode{Type: StringType, Length: 9, Auto: true, Unique: true, Series: WholeObjectSeries},
			DescriptionLength: 25,
			Hierarchy:         Hierarchy{Kind: FoldersAndItemsHierarchy, FoldersOnTop: true, LevelCount: 2},
			Subordination:     SubordinateToItems,
		}}
	},
	// konstanta.md, «Новая константа»: a string of 10 of variable length, the
	// managed lock, the history not used, the standard commands on, the
	// filling not checked, the quick choice and the history of choice left to
	// the platform.
	ConstantKind: func(id uuid.UUID, name string, _ project.Project) Fresh {
		return Fresh{Value: Constant{
			Format: CurrentFormat, ID: id, Name: name,
			Types:               []Type{{Kind: StringType, Length: 10}},
			DataLock:            project.ManagedDataLock,
			DataHistorySettings: DataHistorySettings{DataHistory: DataHistoryDontUse},
			UseStandardCommands: true,
			FillChecking:        DontCheckFilling,
			Choice:              FieldChoice{QuickChoice: UsageAuto, HistoryOnInput: ChoiceHistoryAuto},
		}}
	},
	RoleKind: func(id uuid.UUID, name string, configuration project.Project) Fresh {
		return Fresh{Value: NewRole(id, name, configuration.ID)}
	},
	// obshchij-modul.md, «Новый модуль»: «Сервер» alone. The text of the
	// module lies apart, named by an identifier of its own.
	CommonModuleKind: func(id uuid.UUID, name string, _ project.Project) Fresh {
		module := uuid.Derive(id, "module")
		text, _ := project.ModulePath(module)
		return Fresh{Value: CommonModuleDefinition{Format: CurrentFormat, ID: id, Name: name, Module: module, Server: true}, Modules: []string{text}}
	},
	// gruppa-komand.md, «Новая группа»: the navigation panel, shown «Авто».
	CommandGroupKind: func(id uuid.UUID, name string, _ project.Project) Fresh {
		return Fresh{Value: CommandGroupDefinition{Format: CurrentFormat, ID: id, Name: name, Category: NavigationPanelCategory, Representation: CommandAuto}}
	},
	// perechislenie.md, «Новое перечисление»: no values, the standard commands
	// off, chosen both ways with the quick choice on, the history «Авто».
	EnumerationKind: func(id uuid.UUID, name string, _ project.Project) Fresh {
		return Fresh{Value: Enumeration{
			Format: CurrentFormat, ID: id, Name: name,
			ChoiceMode: ChoiceBothWays, QuickChoice: true, ChoiceHistoryOnInput: ChoiceHistoryAuto,
		}}
	},
}

// NewObject is an object of a kind as «Добавить» makes it, with the
// prototype's values of a new one; false for a kind Studio does not make yet.
func NewObject(kind Kind, id uuid.UUID, name string, configuration project.Project) (Fresh, bool) {
	item, ok := kindInfoOf(kind)
	if !ok || item.fresh == nil {
		return Fresh{}, false
	}
	return item.fresh(id, name, configuration), true
}

// EncodeObject is the files of a description as the project keeps them: the
// description and, of a role, its grid of rights apart - what DecodeSource
// reads back.
func EncodeObject(value any) (description, rights []byte, err error) {
	if role, ok := value.(RoleDefinition); ok {
		return EncodeRole(role)
	}
	var content bytes.Buffer
	err = Encode(&content, value)
	return content.Bytes(), nil, err
}
