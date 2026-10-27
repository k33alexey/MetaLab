package studio

import (
	"net/http"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// The window of an exchange plan (docs/block-00/plan-obmena.md, «Окно»;
// snapshots plan-obmena/01, 02, 10-13): a catalog's without «Иерархия»,
// «Владельцы», «Нумерация» and «Обмен данными», read and written by the
// common part of an object's window (readObjectWindow, saveObjectWindow).
// The window «Состав» and the menu of a plan are items of their own.

// ExchangePlanEditorSource is an exchange plan as its window and palette
// read and write it.
type ExchangePlanEditorSource struct {
	windowHead
	referenceParts
	ExchangePlan metadata.ExchangePlanDefinition `json:"exchangePlan"`
}

// ExchangePlanWindowValue is what the window of an exchange plan writes:
// the plan and what its pages mark - none of a page not given.
type ExchangePlanWindowValue struct {
	metadata.ExchangePlanDefinition
	pageMarks
}

func (source ExchangePlanEditorSource) windowOf() (metadata.ExchangePlanDefinition, windowHead) {
	return source.ExchangePlan, source.windowHead
}

func (ExchangePlanEditorSource) named(value metadata.ExchangePlanDefinition) (uuid.UUID, string) {
	return value.ID, value.Name
}

func (source ExchangePlanEditorSource) identity() (uuid.UUID, string) {
	return source.ExchangePlan.ID, source.Revision
}

// exchangePlanShape is how the window names a plan and what its pages know
// of it: its commands, the fields a right is given to, its templates.
var exchangePlanShape = referenceShape(ExchangePlanEditorSource{}.named, func(value metadata.ExchangePlanDefinition) ([]metadata.ObjectCommand, []metadata.Attribute, []metadata.TablePart, []metadata.ObjectTemplate) {
	return value.Commands, value.Attributes, value.TableParts, value.Templates
})

// exchangePlanEditorAddress is where the window of an exchange plan is
// read and written.
const exchangePlanEditorAddress = "/api/exchange-plan"

func (workspace *Workspace) ReadExchangePlanEditor(relative string) (ExchangePlanEditorSource, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	return workspace.readExchangePlanEditorLocked(relative)
}

func (workspace *Workspace) readExchangePlanEditorLocked(relative string) (ExchangePlanEditorSource, error) {
	value, head, loaded, err := readObjectWindow(workspace, metadata.ExchangePlanKind, relative, metadata.DecodeExchangePlan, exchangePlanShape)
	if err != nil {
		return ExchangePlanEditorSource{}, err
	}
	return ExchangePlanEditorSource{windowHead: head, ExchangePlan: value,
		referenceParts: referencePartsOf(loaded, map[string][]metadata.Kind{"basedOn": metadata.BasisKinds()})}, nil
}

// SaveExchangePlanWindow writes the plan and what the pages of its window
// mark as one save (saveObjectWindow).
func (workspace *Workspace) SaveExchangePlanWindow(relative string, window ExchangePlanWindowValue, revision string) (ExchangePlanEditorSource, error) {
	return saveObjectWindowAs(workspace, relative, revision, window.ExchangePlanDefinition, window.of(metadata.ExchangePlanKind),
		workspace.readExchangePlanEditorLocked, exchangePlanShape)
}

// CreateExchangePlan makes the next exchange plan with the prototype's
// values (plan-obmena.md, «Новый план») by the one way every object is
// made (CreateObject) and opens its window.
func (workspace *Workspace) CreateExchangePlan(name string) (ExchangePlanEditorSource, error) {
	return createObjectAs(workspace, metadata.ExchangePlanKind, name, nil, workspace.readExchangePlanEditorLocked)
}

// DeleteExchangePlan removes an exchange plan's folder by the one order of
// steps of every deletion.
func (workspace *Workspace) DeleteExchangePlan(relative, expectedRevision string) error {
	return deleteObjectFolder(workspace, metadata.ExchangePlanKind, relative, expectedRevision, workspace.readExchangePlanEditorLocked)
}

func registerExchangePlanEditorRoutes(routes *http.ServeMux, workspace *Workspace) {
	registerObjectEditor(routes, exchangePlanEditorAddress, workspace.ReadExchangePlanEditor, workspace.SaveExchangePlanWindow,
		workspace.CreateExchangePlan, workspace.DeleteExchangePlan)
}
