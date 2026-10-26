package studio

import (
	"fmt"
	"net/http"

	"github.com/k33alexey/MetaLab/internal/kinds"
	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// The window of a web service (docs/block-00/web-servis.md, «Окно»): the
// pages «Основные», «Подсистемы», «Операции», «Управление сеансами»,
// «Прочее», the kind's (kinds.WindowPages); its palette beside it. The
// windows choosing XDTO packages and types are an item of their own.
// «Добавить» waits for the answer on a new service of no namespace (series
// 2.584, В1).

// WebServiceEditorSource is a web service as its window and palette read
// and write it.
type WebServiceEditorSource struct {
	windowHead
	Service metadata.WebServiceDefinition `json:"service"`
	// Packages are the namespaces of the XDTO packages of the project by
	// their identifiers: the field «Пакеты XDTO» names a package by its
	// namespace.
	Packages map[uuid.UUID]string `json:"packages"`
	// PartPalettes are the palettes of its operations and their parameters.
	PartPalettes map[string][]kinds.PaletteTab `json:"partPalettes"`
}

func (source WebServiceEditorSource) windowOf() (metadata.WebServiceDefinition, windowHead) {
	return source.Service, source.windowHead
}

func (WebServiceEditorSource) named(value metadata.WebServiceDefinition) (uuid.UUID, string) {
	return value.ID, value.Name
}

func (source WebServiceEditorSource) identity() (uuid.UUID, string) {
	return source.Service.ID, source.Revision
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
	value, head, loaded, err := readObjectWindow(workspace, metadata.WebServiceKind, relative, metadata.DecodeWebService,
		func(value metadata.WebServiceDefinition) uuid.UUID { return value.ID })
	if err != nil {
		return WebServiceEditorSource{}, err
	}
	packages := map[uuid.UUID]string{}
	for _, item := range loaded.XDTOPackages {
		packages[item.ID] = item.Namespace
	}
	return WebServiceEditorSource{windowHead: head, Service: value, Packages: packages,
		PartPalettes: map[string][]kinds.PaletteTab{"operation": kinds.PartPalette("webServiceOperation"), "parameter": kinds.PartPalette("webServiceParameter")}}, nil
}

// SaveWebServiceWindow writes the service and the subsystems its window
// marks as one save (saveObjectWindow).
func (workspace *Workspace) SaveWebServiceWindow(relative string, window WebServiceWindowValue, revision string) (WebServiceEditorSource, error) {
	return saveObjectWindowAs(workspace, relative, revision, window.WebServiceDefinition, window.Subsystems, workspace.readWebServiceEditorLocked)
}

// DeleteWebService removes a web service's folder - its description and
// module - by the one order of steps of every deletion.
func (workspace *Workspace) DeleteWebService(relative, expectedRevision string) error {
	return deleteObjectFolder(workspace, metadata.WebServiceKind, relative, expectedRevision, workspace.readWebServiceEditorLocked)
}

func registerWebServiceEditorRoutes(routes *http.ServeMux, workspace *Workspace) {
	routes.HandleFunc("GET "+webServiceEditorAddress, answerQuery(writeSourceError, func(request *http.Request) (any, error) {
		return workspace.ReadWebServiceEditor(request.URL.Query().Get("path"))
	}))
	registerObjectWindow(routes, webServiceEditorAddress, workspace.SaveWebServiceWindow, func(string) (WebServiceEditorSource, error) {
		return WebServiceEditorSource{}, fmt.Errorf("%w: %s", ErrNoNewObject, metadata.WebServiceKind)
	}, workspace.DeleteWebService)
}
