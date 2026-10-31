package studio

import (
	"errors"
	"fmt"
	"net/http"
	"slices"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/secretstore"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// A WebSocket client has no window: its palette is all Studio shows of it
// (docs/block-00/websocket-klient.md, «Палитра»), «Добавить» makes one and
// opens its empty module at once. The password and the values of secret
// headers lie in the protected store of the system, not in the project
// (METADATA-OBJECTS.md, WebSocket-клиент): the palette writes the password
// there, the file saying only that there is one; a secret header is not
// changed here yet - its window is an item of its own.

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

// WebSocketClientEdit is a client as its palette writes it: the client and
// the password typed into it - none where the field was not touched, ""
// where it was cleared. The password goes to the protected store, never
// into the file.
type WebSocketClientEdit struct {
	metadata.WebSocketClientDefinition
	NewPassword *string `json:"newPassword,omitempty"`
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

// SaveWebSocketClientEditor writes a client as its palette edited it. A
// password typed is written into the protected store of the system under
// the client's entry (metadata.WebSocketPasswordKey) and the file says
// there is one; a password cleared takes the entry away and the file says
// there is none; untouched, it stays as it was. The store is written after
// the revision is checked and before the description, and a description
// that cannot be written puts the entry back as it stood. Every secret
// header stays as it was - its name, its place and that it is secret.
func (workspace *Workspace) SaveWebSocketClientEditor(relative string, edit WebSocketClientEdit, revision string) (WebSocketClientEditorSource, error) {
	return savePaletteObject(workspace, relative, revision, edit.WebSocketClientDefinition, workspace.readWebSocketClientEditorLocked, func(opened metadata.WebSocketClientDefinition, value *metadata.WebSocketClientDefinition) (func() error, error) {
		value.PasswordSet, value.Password = opened.PasswordSet, nil
		secret := func(header metadata.WebSocketHeader) bool { return !header.Secret }
		if !slices.Equal(slices.DeleteFunc(slices.Clone(value.Headers), secret), slices.DeleteFunc(slices.Clone(opened.Headers), secret)) {
			return nil, errors.New("a secret header is kept in the protected store of the system and is not changed here")
		}
		if edit.NewPassword == nil {
			return nil, nil
		}
		return workspace.putSecretLocked(metadata.WebSocketPasswordKey(workspace.configurationID, value.ID), *edit.NewPassword, &value.PasswordSet)
	})
}

// putSecretLocked writes secret into the protected store under key - ""
// takes the entry away - and says in set whether there is one. Answers how
// to put the entry back as it stood: its value before, or none. Without a
// store, or where the entry before cannot be read, nothing is written.
func (workspace *Workspace) putSecretLocked(key, secret string, set *bool) (func() error, error) {
	if workspace.secrets == nil {
		return nil, errNoSecretStore
	}
	before, err := workspace.secrets.Get(key)
	had := err == nil
	if err != nil && !errors.Is(err, secretstore.ErrNotFound) {
		return nil, err
	}
	putBack := func() error {
		if had {
			return workspace.secrets.Set(key, before)
		}
		return workspace.secrets.Delete(key)
	}
	if secret == "" {
		err = workspace.secrets.Delete(key)
	} else {
		err = workspace.secrets.Set(key, secret)
	}
	if err != nil {
		return nil, err
	}
	*set = secret != ""
	return putBack, nil
}

// errNoSecretStore refuses a secret where Studio is given no protected
// store of the system (SetSecretStore).
var errNoSecretStore = errors.New("the protected store of the system is not available: the secret is not written")

// CreateWebSocketClient makes the next client by the one way every object
// is made (CreateObject), its empty module with it.
func (workspace *Workspace) CreateWebSocketClient(name string) (WebSocketClientEditorSource, error) {
	return createObjectAs(workspace, metadata.WebSocketClientKind, name, nil, workspace.readWebSocketClientEditorLocked)
}

// DeleteWebSocketClient removes a client's folder - its description and
// module - by the one order of steps of every deletion, and then its
// secrets: the entries of its password and of its secret headers
// (metadata.WebSocketSecretKeys) go with it, the secret living with the
// object (METADATA-OBJECTS.md, «записи секретов в хранилище»). They go
// after the folder: an entry left by a failure is a secret nobody names,
// while the other order could leave a client marked with a password
// nowhere. Studio given no store has written none.
func (workspace *Workspace) DeleteWebSocketClient(relative, expectedRevision string) error {
	var keys []string
	err := deleteObjectFolder(workspace, metadata.WebSocketClientKind, relative, expectedRevision, func(relative string) (WebSocketClientEditorSource, error) {
		source, err := workspace.readWebSocketClientEditorLocked(relative)
		keys = metadata.WebSocketSecretKeys(workspace.configurationID, source.Client)
		return source, err
	})
	if err != nil {
		return err
	}
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	if workspace.secrets == nil {
		return nil
	}
	var failed []error
	for _, key := range keys {
		if err := workspace.secrets.Delete(key); err != nil {
			failed = append(failed, err)
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("the client is deleted, and its secrets stay in the protected store of the system: %w", errors.Join(failed...))
	}
	return nil
}

func registerWebSocketClientEditorRoutes(routes *http.ServeMux, workspace *Workspace) {
	registerObjectEditor(routes, webSocketClientEditorAddress, workspace.ReadWebSocketClientEditor, workspace.SaveWebSocketClientEditor, workspace.CreateWebSocketClient, workspace.DeleteWebSocketClient)
}
