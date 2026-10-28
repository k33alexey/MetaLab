package studio

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"

	"github.com/k33alexey/MetaLab/internal/atomicfile"
	"github.com/k33alexey/MetaLab/internal/kinds"
	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// The editor of an XDTO package (docs/block-00/paket-xdto.md, «Окно —
// редактор пакета», «Узлы редактора»): the tree of the body of the package -
// its root, the import directives, the properties, the value types and the
// object types, each in a folder of its own - with «Добавить ▾» by the node
// chosen, and the palette of the node chosen. The body is the model's
// (metadata.XDTOContent) and is written in the prototype's form beside the
// description; the namespace of the root is the package's own.

// XDTOPackageEditorSource is a package as its editor reads and writes it:
// the description, its body and the palettes of the nodes of the tree, by
// the kind of the node.
type XDTOPackageEditorSource struct {
	windowHead
	Package      metadata.XDTOPackageDefinition `json:"package"`
	Content      metadata.XDTOContent           `json:"content"`
	NodePalettes map[string][]kinds.PaletteTab  `json:"nodePalettes"`
	// Types are the types a type of a node is chosen from, by namespace,
	// and PlatformNamespaces the platform's namespaces the window lists
	// beside them - every one, as the window of a property of a package
	// lists them (paket-xdto.md, «Узлы редактора»).
	Types              map[string][]string `json:"types"`
	PlatformNamespaces []string            `json:"platformNamespaces"`
}

// XDTOPackageWindowValue is what the editor writes: the description and the
// body.
type XDTOPackageWindowValue struct {
	metadata.XDTOPackageDefinition
	Content metadata.XDTOContent `json:"content"`
}

func (source XDTOPackageEditorSource) windowOf() (metadata.XDTOPackageDefinition, windowHead) {
	return source.Package, source.windowHead
}

func (XDTOPackageEditorSource) named(value metadata.XDTOPackageDefinition) (uuid.UUID, string) {
	return value.ID, value.Name
}

// xdtoPackageShape is how the window names an object of its kind.
var xdtoPackageShape = objectShape[metadata.XDTOPackageDefinition]{named: XDTOPackageEditorSource{}.named}

// xdtoPackageEditorAddress is where the editor of a package is read and
// written.
const xdtoPackageEditorAddress = "/api/xdto-package"

// xdtoNodePalettes are the kinds of the nodes of the tree whose palettes the
// editor shows (kinds.PartPalette).
var xdtoNodePalettes = []string{"xdtoPackage", "xdtoImport", "xdtoProperty", "xdtoValueType", "xdtoObjectType", "xdtoPattern", "xdtoEnumeration"}

func (workspace *Workspace) ReadXDTOPackageEditor(relative string) (XDTOPackageEditorSource, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	return workspace.readXDTOPackageEditorLocked(relative)
}

func (workspace *Workspace) readXDTOPackageEditorLocked(relative string) (XDTOPackageEditorSource, error) {
	value, head, loaded, err := readObjectWindow(workspace, metadata.XDTOPackageKind, relative, metadata.DecodeXDTOPackage, xdtoPackageShape)
	if err != nil {
		return XDTOPackageEditorSource{}, err
	}
	content, ok := loaded.XDTOPackageContent(value.ID)
	if !ok {
		content = metadata.XDTOContent{Namespace: value.Namespace}
	}
	// The revision covers the body too: a body changed outside the editor
	// refuses its save as a changed description does.
	body, err := os.ReadFile(xdtoContentPath(workspace, relative))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return XDTOPackageEditorSource{}, err
	}
	revision := sha256.Sum256(append([]byte(head.Revision), body...))
	head.Revision = hex.EncodeToString(revision[:])
	palettes := make(map[string][]kinds.PaletteTab, len(xdtoNodePalettes))
	for _, kind := range xdtoNodePalettes {
		palettes[kind] = kinds.PartPalette(kind)
	}
	return XDTOPackageEditorSource{windowHead: head, Package: value, Content: content, NodePalettes: palettes, Types: loaded.XDTOTypes(),
		PlatformNamespaces: metadata.PlatformXDTONamespaces()}, nil
}

// xdtoContentPath is the file of the body of the package described at
// relative.
func xdtoContentPath(workspace *Workspace, relative string) string {
	return filepath.Join(workspace.root, filepath.FromSlash(path.Dir(relative)), metadata.XDTOPackageContentFile)
}

// SaveXDTOPackageWindow writes the body and the description of a package as
// one save, under the one lock of the workspace: the editor read last at
// revision (description and body) must be the one on disk; the body is
// written in the prototype's form first and put back as it was when the
// description is refused. The namespace of the body is the package's.
func (workspace *Workspace) SaveXDTOPackageWindow(relative string, window XDTOPackageWindowValue, revision string) (XDTOPackageEditorSource, error) {
	window.Content.Namespace = window.Namespace
	var body bytes.Buffer
	if err := metadata.EncodeXDTOContent(&body, window.Content); err != nil {
		return XDTOPackageEditorSource{}, err
	}
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	opened, err := workspace.readXDTOPackageEditorLocked(relative)
	if err != nil {
		return XDTOPackageEditorSource{}, err
	}
	if opened.Revision != revision {
		return XDTOPackageEditorSource{}, ErrSourceChanged
	}
	description, err := workspace.readSource(relative)
	if err != nil {
		return XDTOPackageEditorSource{}, err
	}
	target := xdtoContentPath(workspace, relative)
	before, readErr := os.ReadFile(target)
	if readErr != nil && !errors.Is(readErr, fs.ErrNotExist) {
		return XDTOPackageEditorSource{}, readErr
	}
	if err := atomicfile.Write(target, ".ml-write-*", 0o644, atomicfile.Bytes(body.Bytes())); err != nil {
		return XDTOPackageEditorSource{}, err
	}
	putBack := func() error {
		if errors.Is(readErr, fs.ErrNotExist) {
			return os.Remove(target)
		}
		return atomicfile.Write(target, ".ml-write-*", 0o644, atomicfile.Bytes(before))
	}
	// The description is compared at its own revision: the body is written
	// by now, and a name of the package refused moves nothing.
	written, err := saveObjectWindow(workspace, relative, description.Revision, window.XDTOPackageDefinition, windowMarks{},
		func(string) (metadata.XDTOPackageDefinition, windowHead, error) {
			head := opened.windowHead
			head.Revision = description.Revision
			return opened.Package, head, nil
		}, xdtoPackageShape)
	if err != nil {
		return XDTOPackageEditorSource{}, errors.Join(err, putBack())
	}
	return workspace.readXDTOPackageEditorLocked(written)
}

// CreateXDTOPackage makes the next package - «ПакетXDTO1» of the
// namespace «http://www.sample-package.org» with its body (paket-xdto.md,
// «Новый пакет») - by the one way every object is made (CreateObject) and
// opens its editor.
func (workspace *Workspace) CreateXDTOPackage(name string) (XDTOPackageEditorSource, error) {
	return createObjectAs(workspace, metadata.XDTOPackageKind, name, nil, workspace.readXDTOPackageEditorLocked)
}

// DeleteXDTOPackage removes the folder of a package by the one order of
// steps of every deletion.
func (workspace *Workspace) DeleteXDTOPackage(relative, expectedRevision string) error {
	return deleteObjectFolder(workspace, metadata.XDTOPackageKind, relative, expectedRevision, workspace.readXDTOPackageEditorLocked)
}

// XDTOSchemaExport is what «Экспорт XML-схемы…» answers: the schema of the
// package, or the issues that keep it from being written.
type XDTOSchemaExport struct {
	Schema []byte                     `json:"schema,omitempty"`
	Issues []metadata.XDTOSchemaIssue `json:"issues,omitempty"`
}

// ExportXDTOSchema writes the XML schema of the package described at
// relative (metadata.ExportXDTOSchema): of the body its editor holds, given
// as content - the prototype exports what the editor shows, saved or not -
// or, with none, of the body on disk, as «Экспорт XML-схемы…» of the tree.
// Its types are checked against the other packages of the project as they
// are saved.
func (workspace *Workspace) ExportXDTOSchema(relative string, content *metadata.XDTOContent) (XDTOSchemaExport, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	opened, err := workspace.readXDTOPackageEditorLocked(relative)
	if err != nil {
		return XDTOSchemaExport{}, err
	}
	loaded, err := metadata.Load(workspace.root)
	if err != nil {
		return XDTOSchemaExport{}, err
	}
	body := opened.Content
	if content != nil {
		body = *content
		body.Namespace = opened.Package.Namespace
	}
	schema, issues := metadata.ExportXDTOSchema(body, loaded)
	return XDTOSchemaExport{Schema: schema, Issues: issues}, nil
}

func registerXDTOPackageEditorRoutes(routes *http.ServeMux, workspace *Workspace) {
	routes.HandleFunc("POST "+xdtoPackageEditorAddress+"/schema", answerStudio(0, writeSourceError, func(input struct {
		Path    string                `json:"path"`
		Content *metadata.XDTOContent `json:"content"`
	}) (any, error) {
		return workspace.ExportXDTOSchema(input.Path, input.Content)
	}))
	registerObjectEditor(routes, xdtoPackageEditorAddress, workspace.ReadXDTOPackageEditor, workspace.SaveXDTOPackageWindow, workspace.CreateXDTOPackage,
		workspace.DeleteXDTOPackage)
}

func (source XDTOPackageEditorSource) identity() (uuid.UUID, string) {
	return source.Package.ID, source.Revision
}
