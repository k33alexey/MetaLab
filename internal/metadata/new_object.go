package metadata

import (
	"bytes"
	"path"

	"github.com/k33alexey/MetaLab/internal/kinds"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// newObject is an object of a kind as «Добавить» makes it: the prototype's
// values of a new one, read off the inventory of block 00 (docs/block-00/
// <вид>.md, «Новый …»). The synonym is empty everywhere - the prototype's new
// object stands with none and its name shown in its place (konstanta.md,
// snapshot novaya-osnovnye) - and the name is the caller's.
type newObject func(id uuid.UUID, name string, configuration project.Project) Fresh

// Fresh is a new object: its description and the files it comes with, by
// where the project keeps them - the empty text of a common module, which
// the load requires and which opens at once (obshchij-modul.md), the
// content of a template loaded from a file (obshchij-maket.md).
type Fresh struct {
	Value any
	Files []FreshFile
}

// FreshFile is a file of a new object: where it lies and what it holds,
// nothing for an empty one.
type FreshFile struct {
	Path    string
	Content []byte
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
	// stil.md, «Новый стиль»: a style giving no item a value of its own -
	// each shows «стиль: <элемент>».
	StyleKind: func(id uuid.UUID, name string, _ project.Project) Fresh {
		return Fresh{Value: StyleDefinition{Format: CurrentFormat, ID: id, Name: name}}
	},
	// obshchij-modul.md, «Новый модуль»: «Сервер» alone. The text of the
	// module lies apart, named by an identifier of its own.
	CommonModuleKind: func(id uuid.UUID, name string, _ project.Project) Fresh {
		module := uuid.Derive(id, "module")
		text, _ := project.ModulePath(module)
		return Fresh{Value: CommonModuleDefinition{Format: CurrentFormat, ID: id, Name: name, Module: module, Server: true}, Files: []FreshFile{{Path: text}}}
	},
	// gruppa-komand.md, «Новая группа»: the navigation panel, shown «Авто».
	CommandGroupKind: func(id uuid.UUID, name string, _ project.Project) Fresh {
		return Fresh{Value: CommandGroupDefinition{Format: CurrentFormat, ID: id, Name: name, Category: NavigationPanelCategory, Representation: CommandAuto}}
	},
	// obshchaya-kartinka.md: the window opens on «Добавить», with no image -
	// the model takes a picture of none - and both availabilities off, as
	// the help gives them by default (ОбщаяКартинка, ДоступностьДляВыбора,
	// ДоступностьДляОформления).
	CommonPictureKind: func(id uuid.UUID, name string, _ project.Project) Fresh {
		return Fresh{Value: CommonPictureDefinition{Format: CurrentFormat, ID: id, Name: name}}
	},
	// obshchij-maket.md, «Конструктор макета»: a spreadsheet document, the
	// kind the constructor has marked; the constructor gives the synonym,
	// the comment, another kind and the content loaded from a file
	// (NewTemplate).
	CommonTemplateKind: func(id uuid.UUID, name string, _ project.Project) Fresh {
		return Fresh{Value: CommonTemplateDefinition{Format: CurrentFormat, ObjectTemplate: ObjectTemplate{ID: id, Name: name, Kind: SpreadsheetTemplate}}}
	},
	// ws-ssylka.md, «Добавление»: a reference is made by importing a
	// description, which the importer gives it with the address it came
	// from (the files of the new object); by itself it has neither.
	WSReferenceKind: func(id uuid.UUID, name string, _ project.Project) Fresh {
		return Fresh{Value: WSReferenceDefinition{Format: CurrentFormat, ID: id, Name: name}}
	},
	// websocket-klient.md, «Новый клиент»: the flags off, a timeout of 30
	// written as the prototype writes it, and the empty module, which opens
	// at once.
	WebSocketClientKind: func(id uuid.UUID, name string, _ project.Project) Fresh {
		timeout := defaultWebSocketTimeout
		module, _ := project.ObjectModulePath(string(WebSocketClientKind), name, string(kinds.ClientModule)+".bsl")
		return Fresh{Value: WebSocketClientDefinition{Format: CurrentFormat, ID: id, Name: name, Timeout: &timeout}, Files: []FreshFile{{Path: module}}}
	},
	// obshchij-rekvizit.md, «Новый реквизит»: a string of 10 of variable
	// length, no auto-use and no separation, not indexed, in the full-text
	// search and the history of data, the filling not checked, the quick
	// choice, the creation on input and the history of choice «Авто».
	CommonAttributeKind: func(id uuid.UUID, name string, _ project.Project) Fresh {
		return Fresh{Value: CommonAttributeDefinition{Format: CurrentFormat, ID: id, Name: name, Types: []Type{{Kind: StringType, Length: 10}},
			AutoUse: CommonAttributeAutoUseDontUse, DataSeparation: SeparationDontUse, Indexing: DontIndex,
			FullTextSearch: FullTextSearchUse, DataHistory: DataHistoryUse, FillChecking: DontCheckFilling,
			Choice: FieldChoice{QuickChoice: UsageAuto, CreateOnInput: UsageAuto, HistoryOnInput: ChoiceHistoryAuto}}}
	},
	// podsistema.md, «Новая подсистема»: in the command interface, not of
	// one command, in the contents of the help.
	SubsystemKind: func(id uuid.UUID, name string, _ project.Project) Fresh {
		return Fresh{Value: SubsystemDefinition{Format: CurrentFormat, ID: id, Name: name, IncludeInCommandInterface: true, IncludeHelpInContents: true}}
	},
	// paket-xdto.md, «Новый пакет»: the namespace
	// «http://www.sample-package.org» and the body saying the qualified
	// form of an element true, of an attribute false, in the prototype's
	// form beside the description.
	XDTOPackageKind: func(id uuid.UUID, name string, _ project.Project) Fresh {
		yes, no := true, false
		var body bytes.Buffer
		_ = EncodeXDTOContent(&body, XDTOContent{Namespace: newXDTONamespace, ElementFormQualified: &yes, AttributeFormQualified: &no})
		description, _ := project.ObjectMetadataPath(string(XDTOPackageKind), name)
		return Fresh{Value: XDTOPackageDefinition{Format: CurrentFormat, ID: id, Name: name, Namespace: newXDTONamespace},
			Files: []FreshFile{{Path: path.Join(path.Dir(description), XDTOPackageContentFile), Content: body.Bytes()}}}
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

// MakesNewObject says «Добавить» makes an object of the kind: its values
// of a new one are known (NewObject).
func MakesNewObject(kind Kind) bool {
	item, ok := kindInfoOf(kind)
	return ok && item.fresh != nil
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
