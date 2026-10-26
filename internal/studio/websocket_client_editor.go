package studio

import (
	"errors"
	"net/http"
	"slices"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// A WebSocket client has no window: its palette is all Studio shows of it
// (docs/block-00/websocket-klient.md, «Палитра»), «Добавить» makes one and
// opens its empty module at once. The password and the values of secret
// headers lie in the protected store of the system, not in the project;
// Studio does not write that store yet (series 2.584, В4), so a save keeps
// them as they were.

// WebSocketClientEditorSource is a WebSocket client as its palette reads and
// writes it.
type WebSocketClientEditorSource struct {
	objectHead
	Client metadata.WebSocketClientDefinition `json:"client"`
}

func (source WebSocketClientEditorSource) paletteOf() (metadata.WebSocketClientDefinition, objectHead) {
	return source.Client, source.objectHead
}
func (WebSocketClientEditorSource) named(value metadata.WebSocketClientDefinition) (uuid.UUID, string) {
	return value.ID, value.Name
}

func (source WebSocketClientEditorSource) identity() (uuid.UUID, string) {
	return source.Client.ID, source.Revision
}

// webSocketClientEditorAddress is where the palette of a WebSocket client is
// read and written.
const webSocketClientEditorAddress = "/api/websocket-client"

func (workspace *Workspace) ReadWebSocketClientEditor(relative string) (WebSocketClientEditorSource, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	return workspace.readWebSocketClientEditorLocked(relative)
}

func (workspace *Workspace) readWebSocketClientEditorLocked(relative string) (WebSocketClientEditorSource, error) {
	value, head, err := readObjectDescription(workspace, metadata.WebSocketClientKind, relative, metadata.DecodeWebSocketClient)
	return WebSocketClientEditorSource{objectHead: head, Client: value}, err
}

// SaveWebSocketClientEditor writes a client as its palette edited it. The
// password stays as it was, and so does every secret header - its name, its
// place and that it is secret: its value is not in the project to change.
func (workspace *Workspace) SaveWebSocketClientEditor(relative string, value metadata.WebSocketClientDefinition, revision string) (WebSocketClientEditorSource, error) {
	return savePaletteObject(workspace, relative, revision, value, workspace.readWebSocketClientEditorLocked, func(opened metadata.WebSocketClientDefinition, value *metadata.WebSocketClientDefinition) error {
		value.PasswordSet, value.Password = opened.PasswordSet, nil
		secret := func(header metadata.WebSocketHeader) bool { return !header.Secret }
		if !slices.Equal(slices.DeleteFunc(slices.Clone(value.Headers), secret), slices.DeleteFunc(slices.Clone(opened.Headers), secret)) {
			return errors.New("a secret header is kept in the protected store of the system and is not changed here")
		}
		return nil
	})
}

// CreateWebSocketClient makes the next client by the one way every object
// is made (CreateObject), its empty module with it.
func (workspace *Workspace) CreateWebSocketClient(name string) (WebSocketClientEditorSource, error) {
	return createObjectAs(workspace, metadata.WebSocketClientKind, name, nil, workspace.readWebSocketClientEditorLocked)
}

// DeleteWebSocketClient removes a client's folder - its description and
// module - by the one order of steps of every deletion.
func (workspace *Workspace) DeleteWebSocketClient(relative, expectedRevision string) error {
	return deleteObjectFolder(workspace, metadata.WebSocketClientKind, relative, expectedRevision, workspace.readWebSocketClientEditorLocked)
}

func registerWebSocketClientEditorRoutes(routes *http.ServeMux, workspace *Workspace) {
	registerObjectEditor(routes, webSocketClientEditorAddress, workspace.ReadWebSocketClientEditor, workspace.SaveWebSocketClientEditor, workspace.CreateWebSocketClient, workspace.DeleteWebSocketClient)
}
