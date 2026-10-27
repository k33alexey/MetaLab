package studio

import (
	"bytes"
	"errors"
	"net/http"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// The window of a subsystem (docs/block-00/podsistema.md, «Окно»; snapshots
// podsistema/04-07): «Основные» - the name, the synonym, the comment, the
// two flags, «Командный интерфейс», the explanation, the picture -
// «Функциональные опции» - the options of the project, marked where the
// subsystem is in their content - and «Прочее» - the help and «Включать в
// содержание справки»; its palette of three tabs. «Состав» - the tree of
// the objects with a box at each and the objects held below. The window
// «Командный интерфейс», «Все подсистемы» and the menu of a subsystem are
// items of their own.

// SubsystemEditorSource is a subsystem as its window and palette read and
// write it.
type SubsystemEditorSource struct {
	windowHead
	Subsystem metadata.SubsystemDefinition `json:"subsystem"`
	// Composition is the tree of the page «Состав»: the objects of the kinds
	// a subsystem's window offers (metadata.SubsystemCompositionKinds).
	Composition []ObjectChoiceBranch `json:"composition"`
	// Pictures name the pictures the field «Картинка» shows and its window
	// «Выбор картинки» offers.
	Pictures PictureChoice `json:"pictures"`
}

// SubsystemWindowValue is what the window of a subsystem writes: the
// subsystem and, when its page «Функциональные опции» was shown, the
// options marked.
type SubsystemWindowValue struct {
	metadata.SubsystemDefinition
	FunctionalOptions *[]uuid.UUID `json:"functionalOptions,omitempty"`
}

func (source SubsystemEditorSource) windowOf() (metadata.SubsystemDefinition, windowHead) {
	return source.Subsystem, source.windowHead
}

func (SubsystemEditorSource) named(value metadata.SubsystemDefinition) (uuid.UUID, string) {
	return value.ID, value.Name
}

// subsystemShape is how the window names an object of its kind.
var subsystemShape = objectShape[metadata.SubsystemDefinition]{named: SubsystemEditorSource{}.named}

func (source SubsystemEditorSource) identity() (uuid.UUID, string) {
	return source.Subsystem.ID, source.Revision
}

// subsystemEditorAddress is where the window of a subsystem is read and
// written.
const subsystemEditorAddress = "/api/subsystem"

func (workspace *Workspace) ReadSubsystemEditor(relative string) (SubsystemEditorSource, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	return workspace.readSubsystemEditorLocked(relative)
}

func (workspace *Workspace) readSubsystemEditorLocked(relative string) (SubsystemEditorSource, error) {
	value, head, loaded, err := readObjectWindow(workspace, metadata.SubsystemKind, relative, metadata.DecodeSubsystem, subsystemShape)
	if err != nil {
		return SubsystemEditorSource{}, err
	}
	return SubsystemEditorSource{windowHead: head, Subsystem: value, Composition: objectChoiceTree(loaded, metadata.SubsystemCompositionKinds()),
		Pictures: pictureChoice(loaded)}, nil
}

// SaveSubsystemWindow writes the subsystem and the options its window marks
// as one save (saveObjectWindow).
func (workspace *Workspace) SaveSubsystemWindow(relative string, window SubsystemWindowValue, revision string) (SubsystemEditorSource, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	written, err := saveObjectWindow(workspace, relative, revision, window.SubsystemDefinition,
		windowMarks{FunctionalOptions: window.FunctionalOptions, Kind: metadata.SubsystemKind},
		func(relative string) (metadata.SubsystemDefinition, windowHead, error) {
			opened, err := workspace.readSubsystemEditorLocked(relative)
			return opened.Subsystem, opened.windowHead, err
		}, subsystemShape)
	if err != nil {
		return SubsystemEditorSource{}, err
	}
	return workspace.readSubsystemEditorLocked(written)
}

// CreateSubsystem makes the next subsystem of the root - «Подсистема1»,
// in the command interface and in the contents of the help (podsistema.md,
// «Новая подсистема») - by the one way every object is made
// (CreateObject) and opens its window.
func (workspace *Workspace) CreateSubsystem(name string) (SubsystemEditorSource, error) {
	return createObjectAs(workspace, metadata.SubsystemKind, name, nil, workspace.readSubsystemEditorLocked)
}

// DeleteSubsystem removes the file of a subsystem by the one order of steps
// of every deletion.
func (workspace *Workspace) DeleteSubsystem(relative, expectedRevision string) error {
	return deleteObjectFolder(workspace, metadata.SubsystemKind, relative, expectedRevision, workspace.readSubsystemEditorLocked)
}

// subsystemChildAddress and subsystemMoveAddress are where a subsystem is
// made under another and moved.
const (
	subsystemChildAddress = "/api/subsystem/child"
	subsystemMoveAddress  = "/api/subsystem/move"
)

func registerSubsystemEditorRoutes(routes *http.ServeMux, workspace *Workspace) {
	routes.HandleFunc("POST "+subsystemChildAddress, answerStudio(0, writeSourceError, func(input SubsystemChild) (any, error) {
		return workspace.CreateSubsystemChild(input)
	}))
	routes.HandleFunc("POST "+subsystemMoveAddress, answerStudio(0, writeSourceError, func(input SubsystemMove) (any, error) {
		return workspace.MoveSubsystem(input)
	}))
	registerObjectEditor(routes, subsystemEditorAddress, workspace.ReadSubsystemEditor, workspace.SaveSubsystemWindow, workspace.CreateSubsystem,
		workspace.DeleteSubsystem)
}

// SubsystemChild asks for a subsystem under Parent - «Добавить ▸
// Подчиненная подсистема» of the menu of a subsystem.
type SubsystemChild struct {
	Parent uuid.UUID `json:"parent"`
}

// SubsystemMove moves the subsystem described at Path, read at Revision,
// under Parent - none, the root of the project («Переместить подсистему»).
type SubsystemMove struct {
	Path     string     `json:"path"`
	Revision string     `json:"revision"`
	Parent   *uuid.UUID `json:"parent,omitempty"`
}

// CreateSubsystemChild makes a subsystem under parent, named the next number
// of «Подсистема» among its siblings - a child of «Подсистема1» is
// «Подсистема1» again (podsistema.md, «Новая подсистема») - with the values
// of every new subsystem, and opens its window.
func (workspace *Workspace) CreateSubsystemChild(child SubsystemChild) (SubsystemEditorSource, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	parent, err := newObjectPath(metadata.SubsystemKind, child.Parent, "")
	if err != nil {
		return SubsystemEditorSource{}, err
	}
	if _, err := workspace.readSubsystemEditorLocked(parent); err != nil {
		return SubsystemEditorSource{}, err
	}
	// The node of a subsystem in the tree is named by its identifier.
	relative, err := workspace.createObjectInLocked(metadata.SubsystemKind, child.Parent.String(), "", func(fresh *metadata.Fresh) error {
		subsystem, _ := fresh.Value.(metadata.SubsystemDefinition)
		above := child.Parent
		subsystem.Parent = &above
		fresh.Value = subsystem
		return nil
	})
	if err != nil {
		return SubsystemEditorSource{}, err
	}
	return workspace.readSubsystemEditorLocked(relative)
}

// errSubsystemInsideItself refuses a subsystem moved under itself or under
// a subsystem of its own.
var errSubsystemInsideItself = errors.New("Подсистему нельзя переместить в неё саму или в подчиненную ей")

// MoveSubsystem puts a subsystem under another parent or the root: a name
// taken among the new siblings - «Имя не уникально!» - and a parent inside
// the subsystem itself are refused, and the subsystem stays where it was.
func (workspace *Workspace) MoveSubsystem(move SubsystemMove) (SubsystemEditorSource, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	opened, err := workspace.readSubsystemEditorLocked(move.Path)
	if err != nil {
		return SubsystemEditorSource{}, err
	}
	if opened.Revision != move.Revision {
		return SubsystemEditorSource{}, ErrSourceChanged
	}
	// The new parent must not lie inside the subsystem, and the name must be
	// free among its new siblings - both checked before anything is written.
	loaded, err := metadata.Load(workspace.root)
	if err != nil {
		return SubsystemEditorSource{}, err
	}
	for above := move.Parent; above != nil; {
		if *above == opened.Subsystem.ID {
			return SubsystemEditorSource{}, errSubsystemInsideItself
		}
		parent, ok := loaded.SubsystemByID(*above)
		if !ok {
			return SubsystemEditorSource{}, errSubsystemInsideItself
		}
		above = parent.Parent
	}
	group := branchOf(string(metadata.SubsystemKind))
	if move.Parent != nil {
		group = move.Parent.String()
	}
	if !sameParent(opened.Subsystem.Parent, move.Parent) {
		if err := workspace.objectNameErrorLocked("", group, opened.Subsystem.Name); err != nil {
			return SubsystemEditorSource{}, err
		}
	}
	moved := opened.Subsystem
	moved.Parent = move.Parent
	var encoded bytes.Buffer
	if err := metadata.Encode(&encoded, moved); err != nil {
		return SubsystemEditorSource{}, err
	}
	if _, err := workspace.saveSourceLocked(move.Path, encoded.String(), opened.Revision); err != nil {
		return SubsystemEditorSource{}, err
	}
	return workspace.readSubsystemEditorLocked(move.Path)
}

// sameParent says two parents of a subsystem are one - the root or the same
// subsystem.
func sameParent(one, other *uuid.UUID) bool {
	return one == nil && other == nil || one != nil && other != nil && *one == *other
}
