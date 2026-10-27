package studio

import (
	"net/http"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// A common attribute has no window: its palette is what Studio reads and
// writes of it (docs/block-00/obshchij-rekvizit.md, «Палитра»); the window
// «Состав» and the menu are an item of their own.

// CommonAttributeEditorSource is a common attribute as its palette reads
// and writes it, with the types a field the database stores may have.
type CommonAttributeEditorSource struct {
	objectHead
	Attribute metadata.CommonAttributeDefinition `json:"attribute"`
	TypeOffer metadata.TypeOffer                 `json:"typeOffer"`
}

func (source CommonAttributeEditorSource) paletteOf() (metadata.CommonAttributeDefinition, objectHead) {
	return source.Attribute, source.objectHead
}

func (CommonAttributeEditorSource) named(value metadata.CommonAttributeDefinition) (uuid.UUID, string) {
	return value.ID, value.Name
}

func (source CommonAttributeEditorSource) identity() (uuid.UUID, string) {
	return source.Attribute.ID, source.Revision
}

// commonAttributeEditorAddress is where the palette of a common attribute
// is read, written, made and deleted.
const commonAttributeEditorAddress = "/api/common-attribute"

func (workspace *Workspace) ReadCommonAttributeEditor(relative string) (CommonAttributeEditorSource, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	return workspace.readCommonAttributeEditorLocked(relative)
}

func (workspace *Workspace) readCommonAttributeEditorLocked(relative string) (CommonAttributeEditorSource, error) {
	value, head, err := readObjectDescription(workspace, metadata.CommonAttributeKind, relative, metadata.DecodeCommonAttribute)
	if err != nil {
		return CommonAttributeEditorSource{}, err
	}
	loaded, err := metadata.Load(workspace.root)
	if err != nil {
		return CommonAttributeEditorSource{}, err
	}
	offer, _ := loaded.TypeOffer(metadata.StoredFieldChoice)
	return CommonAttributeEditorSource{objectHead: head, Attribute: value, TypeOffer: offer}, nil
}

// SaveCommonAttributeEditor validates and writes an edited common attribute.
func (workspace *Workspace) SaveCommonAttributeEditor(relative string, value metadata.CommonAttributeDefinition, revision string) (CommonAttributeEditorSource, error) {
	return savePaletteObject(workspace, relative, revision, value, workspace.readCommonAttributeEditorLocked, nil)
}

// CreateCommonAttribute makes the next common attribute, «ОбщийРеквизит1»
// (obshchij-rekvizit.md, «Новый реквизит»), by the one way every object is
// made and opens its palette.
func (workspace *Workspace) CreateCommonAttribute(name string) (CommonAttributeEditorSource, error) {
	return createObjectAs(workspace, metadata.CommonAttributeKind, name, nil, workspace.readCommonAttributeEditorLocked)
}

// DeleteCommonAttribute removes a common attribute by the one order of
// steps of every deletion.
func (workspace *Workspace) DeleteCommonAttribute(relative, expectedRevision string) error {
	return deleteObjectFolder(workspace, metadata.CommonAttributeKind, relative, expectedRevision, workspace.readCommonAttributeEditorLocked)
}

func registerCommonAttributeEditorRoutes(routes *http.ServeMux, workspace *Workspace) {
	registerObjectEditor(routes, commonAttributeEditorAddress, workspace.ReadCommonAttributeEditor, workspace.SaveCommonAttributeEditor,
		workspace.CreateCommonAttribute, workspace.DeleteCommonAttribute)
}
