package studio

import (
	"net/http"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// A constant has no window: its double click, «Изменить» and «Свойства»
// open its palette (docs/block-00/konstanta.md, «Окно редактирования»), so
// what Studio reads and writes of it is what the palette shows.

// ConstantEditorSource is a constant as its palette reads and writes it.
type ConstantEditorSource struct {
	objectHead
	Constant  metadata.Constant  `json:"constant"`
	TypeOffer metadata.TypeOffer `json:"typeOffer"`
	// Choices are what the windows «Выбор объекта» of the constant offer:
	// defaultForm - the common forms, one of which opens the value.
	Choices map[string][]ObjectChoiceBranch `json:"choices"`
}

func (source ConstantEditorSource) paletteOf() (metadata.Constant, objectHead) {
	return source.Constant, source.objectHead
}
func (ConstantEditorSource) named(value metadata.Constant) (uuid.UUID, string) {
	return value.ID, value.Name
}

func (workspace *Workspace) ReadConstantEditor(relative string) (ConstantEditorSource, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	return workspace.readConstantEditorLocked(relative)
}

func (workspace *Workspace) readConstantEditorLocked(relative string) (ConstantEditorSource, error) {
	value, head, err := readObjectDescription(workspace, metadata.ConstantKind, relative, metadata.DecodeConstant)
	if err != nil {
		return ConstantEditorSource{}, err
	}
	loaded, err := metadata.Load(workspace.root)
	if err != nil {
		return ConstantEditorSource{}, err
	}
	offer, _ := loaded.TypeOffer(metadata.StoredFieldChoice)
	return ConstantEditorSource{objectHead: head, Constant: value, TypeOffer: offer,
		Choices: map[string][]ObjectChoiceBranch{"defaultForm": objectChoiceTree(loaded, []metadata.Kind{metadata.CommonFormKind})},
	}, nil
}

// SaveConstantEditor validates and writes an edited constant; a new name
// moves its folder with its modules (followNameLocked).
func (workspace *Workspace) SaveConstantEditor(relative string, value metadata.Constant, revision string) (ConstantEditorSource, error) {
	return savePaletteObject(workspace, relative, revision, value, workspace.readConstantEditorLocked, nil)
}

// CreateConstant makes the next constant, «Константа1» and on, by the one
// way every object is made (CreateObject) and opens its palette.
func (workspace *Workspace) CreateConstant(name string) (ConstantEditorSource, error) {
	return createObjectAs(workspace, metadata.ConstantKind, name, nil, workspace.readConstantEditorLocked)
}

// DeleteConstant removes a constant's folder - its description and modules -
// by the one order of steps of every deletion (deleteObjectLocked).
func (workspace *Workspace) DeleteConstant(relative, expectedRevision string) error {
	return deleteObjectFolder(workspace, metadata.ConstantKind, relative, expectedRevision, workspace.readConstantEditorLocked)
}

// constantEditorAddress is where the palette of a constant is read,
// written, made and deleted.
const constantEditorAddress = "/api/constant"

func registerConstantEditorRoutes(routes *http.ServeMux, workspace *Workspace) {
	registerObjectEditor(routes, constantEditorAddress, workspace.ReadConstantEditor, workspace.SaveConstantEditor, workspace.CreateConstant, workspace.DeleteConstant)
}
