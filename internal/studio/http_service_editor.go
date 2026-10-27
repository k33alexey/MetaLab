package studio

import (
	"net/http"

	"github.com/k33alexey/MetaLab/internal/kinds"
	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// The window of an HTTP service (docs/block-00/http-servis.md, «Окно»): the
// pages «Основные» - with the root -, «Подсистемы», «Шаблоны URL»,
// «Управление сеансами», «Прочее», the kind's; its palette beside it. The
// root is checked when its field is left, in the prototype's words. The
// page «Шаблоны URL» is an item of its own; «Добавить» waits for the answer
// on a new service of no root (series 2.584, В1).

// HTTPServiceEditorSource is an HTTP service as its window and palette read
// and write it.
type HTTPServiceEditorSource struct {
	windowHead
	Service metadata.HTTPServiceDefinition `json:"service"`
	// PartPalettes are the palettes of its templates and their methods.
	PartPalettes map[string][]kinds.PaletteTab `json:"partPalettes"`
}

func (source HTTPServiceEditorSource) windowOf() (metadata.HTTPServiceDefinition, windowHead) {
	return source.Service, source.windowHead
}

func (HTTPServiceEditorSource) named(value metadata.HTTPServiceDefinition) (uuid.UUID, string) {
	return value.ID, value.Name
}

// httpServiceShape is how the window names an object of its kind.
var httpServiceShape = objectShape[metadata.HTTPServiceDefinition]{named: HTTPServiceEditorSource{}.named}

func (source HTTPServiceEditorSource) identity() (uuid.UUID, string) {
	return source.Service.ID, source.Revision
}

// HTTPServiceWindowValue is what the window of an HTTP service writes: the
// service and, when its page «Подсистемы» was shown, the subsystems marked.
type HTTPServiceWindowValue struct {
	metadata.HTTPServiceDefinition
	Subsystems *[]uuid.UUID `json:"subsystems,omitempty"`
}

// rootURLRefusals are the refusals of a root in the prototype's words
// (snapshots http-servis/02, 08).
var rootURLRefusals = map[metadata.RootURLFault]string{
	metadata.RootURLEmpty:   "Корневой URL должен быть заполнен",
	metadata.RootURLInvalid: "Некорректный корневой URL",
}

// httpServiceEditorAddress is where the window of an HTTP service is read
// and written, and httpRootAddress where its root is checked.
const (
	httpServiceEditorAddress = "/api/http-service"
	httpRootAddress          = "/api/http-service/root"
)

func (workspace *Workspace) ReadHTTPServiceEditor(relative string) (HTTPServiceEditorSource, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	return workspace.readHTTPServiceEditorLocked(relative)
}

func (workspace *Workspace) readHTTPServiceEditorLocked(relative string) (HTTPServiceEditorSource, error) {
	value, head, _, err := readObjectWindow(workspace, metadata.HTTPServiceKind, relative, metadata.DecodeHTTPService, httpServiceShape)
	return HTTPServiceEditorSource{windowHead: head, Service: value,
		PartPalettes: map[string][]kinds.PaletteTab{"template": kinds.PartPalette("httpTemplate"), "method": kinds.PartPalette("httpMethod")}}, err
}

// SaveHTTPServiceWindow writes the service and the subsystems its window
// marks as one save (saveObjectWindow).
func (workspace *Workspace) SaveHTTPServiceWindow(relative string, window HTTPServiceWindowValue, revision string) (HTTPServiceEditorSource, error) {
	return saveObjectWindowAs(workspace, relative, revision, window.HTTPServiceDefinition, windowMarks{Subsystems: window.Subsystems}, workspace.readHTTPServiceEditorLocked, httpServiceShape)
}

// DeleteHTTPService removes an HTTP service's folder - its description and
// module - by the one order of steps of every deletion.
func (workspace *Workspace) DeleteHTTPService(relative, expectedRevision string) error {
	return deleteObjectFolder(workspace, metadata.HTTPServiceKind, relative, expectedRevision, workspace.readHTTPServiceEditorLocked)
}

func registerHTTPServiceEditorRoutes(routes *http.ServeMux, workspace *Workspace) {
	// The root typed is checked when its field is left, as a name is: the
	// refusal is the prototype's, none when it fits.
	routes.HandleFunc("POST "+httpRootAddress, answerStudio(studioMutationCeiling, writeSourceError, func(input struct {
		Value string `json:"value"`
	}) (any, error) {
		return NameAnswer{Refusal: rootURLRefusals[metadata.RootURLFaultOf(input.Value)]}, nil
	}))
	registerObjectEditor(routes, httpServiceEditorAddress, workspace.ReadHTTPServiceEditor, workspace.SaveHTTPServiceWindow, noNewObject[HTTPServiceEditorSource](metadata.HTTPServiceKind), workspace.DeleteHTTPService)
}
