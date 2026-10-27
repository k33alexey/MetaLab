package studio

import (
	"net/http"

	"github.com/k33alexey/MetaLab/internal/kinds"
	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// The window of a web service (docs/block-00/web-servis.md, «Окно»): the
// pages «Основные», «Подсистемы», «Операции», «Управление сеансами»,
// «Прочее», the kind's (kinds.WindowPages); its palette beside it; the
// window «Выбор пакетов XDTO» chooses its packages. The window choosing a
// type is an item of its own.
// «Добавить» waits for the answer on a new service of no namespace (series
// 2.584, В1).

// WebServiceEditorSource is a web service as its window and palette read
// and write it.
type WebServiceEditorSource struct {
	windowHead
	Service metadata.WebServiceDefinition `json:"service"`
	// Packages are the XDTO packages of the project in the order of the
	// tree: the field «Пакеты XDTO» names a package by its namespace, and
	// the window «Выбор пакетов XDTO» lists them after PlatformNamespaces,
	// the namespaces of the platform's own.
	Packages           []XDTOPackageChoice `json:"packages"`
	PlatformNamespaces []string            `json:"platformNamespaces"`
	// Types are the types a type of an operation or of a parameter is
	// chosen from, by namespace (Catalog.XDTOTypes).
	Types map[string][]string `json:"types"`
	// PartPalettes are the palettes of its operations and their parameters.
	PartPalettes map[string][]kinds.PaletteTab `json:"partPalettes"`
}

func (source WebServiceEditorSource) windowOf() (metadata.WebServiceDefinition, windowHead) {
	return source.Service, source.windowHead
}

func (WebServiceEditorSource) named(value metadata.WebServiceDefinition) (uuid.UUID, string) {
	return value.ID, value.Name
}

// webServiceShape is how the window names an object of its kind.
var webServiceShape = objectShape[metadata.WebServiceDefinition]{named: WebServiceEditorSource{}.named}

func (source WebServiceEditorSource) identity() (uuid.UUID, string) {
	return source.Service.ID, source.Revision
}

// XDTOPackageChoice is an XDTO package of the project as the choice of the
// packages of a web service names it: by its identifier and namespace.
type XDTOPackageChoice struct {
	ID        uuid.UUID `json:"id"`
	Namespace string    `json:"namespace"`
}

// WebServiceWindowValue is what the window of a web service writes: the
// service and, when its page «Подсистемы» was shown, the subsystems marked.
type WebServiceWindowValue struct {
	metadata.WebServiceDefinition
	Subsystems *[]uuid.UUID `json:"subsystems,omitempty"`
}

// webServiceEditorAddress is where the window of a web service is read and
// written.
const webServiceEditorAddress = "/api/web-service"

func (workspace *Workspace) ReadWebServiceEditor(relative string) (WebServiceEditorSource, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	return workspace.readWebServiceEditorLocked(relative)
}

func (workspace *Workspace) readWebServiceEditorLocked(relative string) (WebServiceEditorSource, error) {
	value, head, loaded, err := readObjectWindow(workspace, metadata.WebServiceKind, relative, metadata.DecodeWebService, webServiceShape)
	if err != nil {
		return WebServiceEditorSource{}, err
	}
	packages := []XDTOPackageChoice{}
	for _, object := range loaded.KindObjects(metadata.XDTOPackageKind) {
		item, _ := loaded.XDTOPackageByID(object.ID)
		packages = append(packages, XDTOPackageChoice{ID: item.ID, Namespace: item.Namespace})
	}
	return WebServiceEditorSource{windowHead: head, Service: value, Packages: packages, PlatformNamespaces: metadata.PlatformXDTONamespaces(), Types: loaded.XDTOTypes(),
		PartPalettes: map[string][]kinds.PaletteTab{"operation": kinds.PartPalette("webServiceOperation"), "parameter": kinds.PartPalette("webServiceParameter")}}, nil
}

// SaveWebServiceWindow writes the service and the subsystems its window
// marks as one save (saveObjectWindow).
func (workspace *Workspace) SaveWebServiceWindow(relative string, window WebServiceWindowValue, revision string) (WebServiceEditorSource, error) {
	return saveObjectWindowAs(workspace, relative, revision, window.WebServiceDefinition, windowMarks{Subsystems: window.Subsystems}, workspace.readWebServiceEditorLocked, webServiceShape)
}

// DeleteWebService removes a web service's folder - its description and
// module - by the one order of steps of every deletion.
func (workspace *Workspace) DeleteWebService(relative, expectedRevision string) error {
	return deleteObjectFolder(workspace, metadata.WebServiceKind, relative, expectedRevision, workspace.readWebServiceEditorLocked)
}

func registerWebServiceEditorRoutes(routes *http.ServeMux, workspace *Workspace) {
	registerObjectEditor(routes, webServiceEditorAddress, workspace.ReadWebServiceEditor, workspace.SaveWebServiceWindow, noNewObject[WebServiceEditorSource](metadata.WebServiceKind), workspace.DeleteWebService)
}
