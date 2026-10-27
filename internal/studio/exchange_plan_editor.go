package studio

import (
	"net/http"

	"github.com/k33alexey/MetaLab/internal/kinds"
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
	ExchangePlan metadata.ExchangePlanDefinition `json:"exchangePlan"`
	TypeOffer    metadata.TypeOffer              `json:"typeOffer"`
	// PartPalettes are the palettes of its parts: an attribute, a table
	// part, a template.
	PartPalettes map[string][]kinds.PaletteTab `json:"partPalettes"`
	// Choices are what the window «Выбор объекта» of «Вводится на
	// основании» offers.
	Choices map[string][]ObjectChoiceBranch `json:"choices"`
}

// ExchangePlanWindowValue is what the window of an exchange plan writes:
// the plan and what its pages mark - none of a page not given.
type ExchangePlanWindowValue struct {
	metadata.ExchangePlanDefinition
	TemplateContents  []TemplateContent `json:"templateContents,omitempty"`
	Subsystems        *[]uuid.UUID      `json:"subsystems,omitempty"`
	FunctionalOptions *[]uuid.UUID      `json:"functionalOptions,omitempty"`
	BasisFor          *[]uuid.UUID      `json:"basisFor,omitempty"`
	Roles             *[]RoleMark       `json:"roles,omitempty"`
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
	// What the window «Редактирование типа данных» offers for an attribute
	// is a field the database stores.
	offer, _ := loaded.TypeOffer(metadata.StoredFieldChoice)
	return ExchangePlanEditorSource{windowHead: head, ExchangePlan: value, TypeOffer: offer,
		Choices: map[string][]ObjectChoiceBranch{"basedOn": objectChoiceTree(loaded, metadata.BasisKinds())},
		PartPalettes: map[string][]kinds.PaletteTab{"attribute": kinds.PartPalette("attribute"), "tablePart": kinds.PartPalette("tablePart"),
			"template": kinds.PartPalette("template")},
	}, nil
}

// SaveExchangePlanWindow writes the plan and what the pages of its window
// mark as one save (saveObjectWindow).
func (workspace *Workspace) SaveExchangePlanWindow(relative string, window ExchangePlanWindowValue, revision string) (ExchangePlanEditorSource, error) {
	return saveObjectWindowAs(workspace, relative, revision, window.ExchangePlanDefinition, windowMarks{
		Subsystems: window.Subsystems, FunctionalOptions: window.FunctionalOptions, BasisFor: window.BasisFor, Roles: window.Roles,
		TemplateContents: window.TemplateContents, Kind: metadata.ExchangePlanKind,
	}, workspace.readExchangePlanEditorLocked, exchangePlanShape)
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
