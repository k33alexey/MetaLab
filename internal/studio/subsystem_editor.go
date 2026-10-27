package studio

import (
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
	value, head, loaded, err := readObjectWindow(workspace, metadata.SubsystemKind, relative, metadata.DecodeSubsystem,
		func(value metadata.SubsystemDefinition) uuid.UUID { return value.ID })
	if err != nil {
		return SubsystemEditorSource{}, err
	}
	head.FunctionalOptions = optionPlaces(loaded, metadata.SubsystemKind, value.ID)
	return SubsystemEditorSource{windowHead: head, Subsystem: value, Composition: objectChoiceTree(loaded, metadata.SubsystemCompositionKinds())}, nil
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
		}, SubsystemEditorSource{}.named)
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

func registerSubsystemEditorRoutes(routes *http.ServeMux, workspace *Workspace) {
	registerObjectEditor(routes, subsystemEditorAddress, workspace.ReadSubsystemEditor, workspace.SaveSubsystemWindow, workspace.CreateSubsystem,
		workspace.DeleteSubsystem)
}
