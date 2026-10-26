package studio

import (
	"bytes"
	"fmt"
	"net/http"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// The window of a web service (docs/block-00/web-servis.md, «Окно»): the
// pages «Основные», «Подсистемы», «Операции», «Управление сеансами»,
// «Прочее», the kind's (kinds.WindowPages); its palette beside it. The page
// «Операции» and the windows choosing XDTO packages and types are items of
// their own. «Добавить» waits for the answer on a new service of no
// namespace (series 2.584, В1).

// WebServiceEditorSource is a web service as its window and palette read
// and write it.
type WebServiceEditorSource struct {
	objectHead
	Service metadata.WebServiceDefinition `json:"service"`
	// Subsystems are the subsystems of the project, those holding the
	// service marked (its page «Подсистемы»).
	Subsystems []MemberPlace `json:"subsystems"`
	// Packages are the namespaces of the XDTO packages of the project by
	// their identifiers: the field «Пакеты XDTO» names a package by its
	// namespace.
	Packages map[uuid.UUID]string `json:"packages"`
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
	value, head, err := readObjectDescription(workspace, metadata.WebServiceKind, relative, metadata.DecodeWebService)
	if err != nil {
		return WebServiceEditorSource{}, err
	}
	loaded, err := metadata.Load(workspace.root)
	if err != nil {
		return WebServiceEditorSource{}, err
	}
	packages := map[uuid.UUID]string{}
	for _, item := range loaded.XDTOPackages {
		packages[item.ID] = item.Namespace
	}
	return WebServiceEditorSource{objectHead: head, Service: value, Subsystems: subsystemPlaces(loaded, value.ID), Packages: packages}, nil
}

// SaveWebServiceWindow writes the service and the subsystems its window
// marks as one save (writeObjectWindowLocked).
func (workspace *Workspace) SaveWebServiceWindow(relative string, window WebServiceWindowValue, revision string) (WebServiceEditorSource, error) {
	value := window.WebServiceDefinition
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	opened, err := workspace.readWebServiceEditorLocked(relative)
	if err != nil {
		return WebServiceEditorSource{}, err
	}
	if opened.Revision != revision {
		return WebServiceEditorSource{}, ErrSourceChanged
	}
	if value.ID != opened.Service.ID {
		return WebServiceEditorSource{}, fmt.Errorf("web service UUID cannot be changed")
	}
	if value.Name != opened.Service.Name {
		if err := workspace.objectNameErrorLocked(relative, "", value.Name); err != nil {
			return WebServiceEditorSource{}, err
		}
	}
	var encoded bytes.Buffer
	if err := metadata.Encode(&encoded, value); err != nil {
		return WebServiceEditorSource{}, err
	}
	var pages []membership
	if window.Subsystems != nil {
		pages = append(pages, subsystemMembership(value.ID, opened.Subsystems, *window.Subsystems))
	}
	written, _, err := workspace.writeObjectWindowLocked(relative, opened.Service.Name, value.Name, encoded.String(), revision, pages, nil)
	if err != nil {
		return WebServiceEditorSource{}, err
	}
	return workspace.readWebServiceEditorLocked(written)
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
