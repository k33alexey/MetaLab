package studio

import (
	"net/http"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// The window of a document (docs/block-00/dokument.md, «Окно»; snapshots
// dokument/01-03, 09-13): a catalog's window with the pages, the rows and
// the palette of a document's kind, read and written by the common part of
// an object's window (readObjectWindow, saveObjectWindow). «Движения»,
// «Последовательности», «Журналы», a new document and the menu of a
// document are items of their own.

// DocumentEditorSource is a document as its window and palette read and
// write it.
type DocumentEditorSource struct {
	windowHead
	referenceParts
	Document metadata.DocumentDefinition `json:"document"`
}

// DocumentWindowValue is what the window of a document writes: the document
// and what its pages mark.
type DocumentWindowValue struct {
	metadata.DocumentDefinition
	pageMarks
}

func (source DocumentEditorSource) windowOf() (metadata.DocumentDefinition, windowHead) {
	return source.Document, source.windowHead
}

func (DocumentEditorSource) named(value metadata.DocumentDefinition) (uuid.UUID, string) {
	return value.ID, value.Name
}

func (source DocumentEditorSource) identity() (uuid.UUID, string) {
	return source.Document.ID, source.Revision
}

// documentShape is how the window names a document and what its pages know
// of it: its commands, the fields a right is given to, its templates.
var documentShape = referenceShape(DocumentEditorSource{}.named, func(value metadata.DocumentDefinition) ([]metadata.ObjectCommand, []metadata.Attribute, []metadata.TablePart, []metadata.ObjectTemplate) {
	return value.Commands, value.Attributes, value.TableParts, value.Templates
})

// documentEditorAddress is where the window of a document is read and
// written.
const documentEditorAddress = "/api/document"

func (workspace *Workspace) ReadDocumentEditor(relative string) (DocumentEditorSource, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	return workspace.readDocumentEditorLocked(relative)
}

func (workspace *Workspace) readDocumentEditorLocked(relative string) (DocumentEditorSource, error) {
	value, head, loaded, err := readObjectWindow(workspace, metadata.DocumentKind, relative, metadata.DecodeDocument, documentShape)
	if err != nil {
		return DocumentEditorSource{}, err
	}
	// The windows «Выбор объекта» of the bases and of the numerator, which
	// shows the numerator chosen by its name.
	return DocumentEditorSource{windowHead: head, Document: value,
		referenceParts: referencePartsOf(loaded, map[string][]metadata.Kind{"basedOn": metadata.BasisKinds(), "numerator": {metadata.NumeratorKind}})}, nil
}

// SaveDocumentWindow writes the document and what the pages of its window
// mark as one save (saveObjectWindow).
func (workspace *Workspace) SaveDocumentWindow(relative string, window DocumentWindowValue, revision string) (DocumentEditorSource, error) {
	return saveObjectWindowAs(workspace, relative, revision, window.DocumentDefinition, window.of(metadata.DocumentKind),
		workspace.readDocumentEditorLocked, documentShape)
}

// DeleteDocument removes a document's folder by the one order of steps of
// every deletion.
func (workspace *Workspace) DeleteDocument(relative, expectedRevision string) error {
	return deleteObjectFolder(workspace, metadata.DocumentKind, relative, expectedRevision, workspace.readDocumentEditorLocked)
}

func registerDocumentEditorRoutes(routes *http.ServeMux, workspace *Workspace) {
	// A new document with the prototype's values is an item of its own.
	registerObjectEditor(routes, documentEditorAddress, workspace.ReadDocumentEditor, workspace.SaveDocumentWindow,
		noNewObject[DocumentEditorSource](metadata.DocumentKind), workspace.DeleteDocument)
}
