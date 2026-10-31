package studio

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/secretstore"
)

// TestAWebSocketClientIsMadeAndWrittenByItsPalette catches a client made or
// written otherwise than its palette shows (websocket-klient.md, «Новый
// клиент», «Палитра»): «Добавить» makes WebSocketКлиент1 with the flags off,
// a timeout of 30 and its empty module; the palette writes the address, the
// user, the headers and a timeout of 0 as typed; the password and a secret
// header, which lie in the protected store, stay as they were - a save that
// changes the secret header is refused.
func TestAWebSocketClientIsMadeAndWrittenByItsPalette(t *testing.T) {
	t.Parallel()
	workspace, err := Open(copyDemoProjectForStudioTest(t))
	if err != nil {
		t.Fatal(err)
	}
	made, err := workspace.CreateWebSocketClient("")
	if err != nil {
		t.Fatal(err)
	}
	client := made.Client
	if client.Name != "WebSocketКлиент1" || client.Predefined || client.AutoConnect || client.Timeout == nil || *client.Timeout != 30 {
		t.Fatalf("the new client is %+v", client)
	}
	module := filepath.Join(workspace.root, "metadata", "websocket-clients", "WebSocketКлиент1", "МодульКлиента.bsl")
	if content, err := os.ReadFile(module); err != nil || len(content) != 0 {
		t.Errorf("the module of the new client is %q (%v), want an empty file", content, err)
	}
	// The client as an import would bring it: a password and a secret
	// header in the store.
	stored := client
	stored.PasswordSet = true
	stored.Headers = []metadata.WebSocketHeader{{Name: "Authorization", Secret: true}}
	description := filepath.Join(workspace.root, filepath.FromSlash(made.Path))
	file, err := os.Create(description)
	if err != nil {
		t.Fatal(err)
	}
	if err := metadata.Encode(file, stored); err != nil {
		t.Fatal(err)
	}
	_ = file.Close()
	opened, err := workspace.ReadWebSocketClientEditor(made.Path)
	if err != nil {
		t.Fatal(err)
	}
	zero := 0
	edited := opened.Client
	edited.ServerURL, edited.User, edited.Timeout, edited.PasswordSet = "wss://example.org/feed", "склад", &zero, false
	edited.Headers = append(edited.Headers, metadata.WebSocketHeader{Name: "X-Version", Value: "2"})
	saved, err := workspace.SaveWebSocketClientEditor(made.Path, WebSocketClientEdit{WebSocketClientDefinition: edited}, opened.Revision)
	if err != nil {
		t.Fatal(err)
	}
	got := saved.Client
	if got.ServerURL != "wss://example.org/feed" || got.User != "склад" || got.Timeout == nil || *got.Timeout != 0 || !got.PasswordSet ||
		len(got.Headers) != 2 || !got.Headers[0].Secret || got.Headers[1].Value != "2" {
		t.Errorf("saved %+v", got)
	}
	changed := got
	changed.Headers = []metadata.WebSocketHeader{{Name: "Proxy-Authorization", Secret: true}, got.Headers[1]}
	if _, err := workspace.SaveWebSocketClientEditor(saved.Path, WebSocketClientEdit{WebSocketClientDefinition: changed}, saved.Revision); err == nil {
		t.Error("a save changing a secret header was taken")
	}
	if _, err := metadata.Load(workspace.root); err != nil {
		t.Fatalf("the project does not load: %v", err)
	}
}

// memorySecrets is a protected store in memory for the tests; failing
// makes the next write - Set or Delete - fail.
type memorySecrets struct {
	mu      sync.Mutex
	entries map[string]string
	failing bool
}

func (store *memorySecrets) Set(key, secret string) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.failing {
		store.failing = false
		return errors.New("the store refused")
	}
	store.entries[key] = secret
	return nil
}

func (store *memorySecrets) Get(key string) (string, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	secret, ok := store.entries[key]
	if !ok {
		return "", fmt.Errorf("%w: %s", secretstore.ErrNotFound, key)
	}
	return secret, nil
}

func (store *memorySecrets) Delete(key string) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.failing {
		store.failing = false
		return errors.New("the store refused")
	}
	delete(store.entries, key)
	return nil
}

// The password typed into the palette of a WebSocket client goes into the
// protected store of the system under «websocket.<project>.<client>.password»
// and the file says only password_set (METADATA-OBJECTS.md, WebSocket-клиент,
// «записи секретов в хранилище»; the owner, 31.10.2026, В3 of series 2.672);
// cleared, the entry goes and so does the mark; untouched, it stays. A save
// at an old revision writes nothing into the store; a store refusing writes
// nothing into the file; a description that cannot be written puts the
// entry back. The deletion of the client takes away its entries - the
// password and a secret header -, not those of another client. Without a
// store a password is refused. Defects caught: the password written into
// the file or not into the store; an entry under another name; the mark
// left after clearing; a store written by a save refused for its revision;
// a file marked by a store that refused; a new password left in the store
// by a failed save; the entries outliving the client or another client's
// taken; a password silently dropped without a store.
func TestThePasswordOfAWebSocketClientLiesInTheProtectedStore(t *testing.T) {
	t.Parallel()
	workspace, err := Open(copyDemoProjectForStudioTest(t))
	if err != nil {
		t.Fatal(err)
	}
	made, err := workspace.CreateWebSocketClient("")
	if err != nil {
		t.Fatal(err)
	}
	password := func(value string) *string { return &value }
	if _, err := workspace.SaveWebSocketClientEditor(made.Path, WebSocketClientEdit{WebSocketClientDefinition: made.Client, NewPassword: password("p")}, made.Revision); !errors.Is(err, errNoSecretStore) {
		t.Fatalf("a password without a store answered %v", err)
	}
	store := &memorySecrets{entries: map[string]string{}}
	workspace.SetSecretStore(store)
	configuration, err := os.ReadFile(filepath.Join(workspace.root, "configuration.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	projectID := ""
	for _, line := range strings.Split(string(configuration), "\n") {
		if value, ok := strings.CutPrefix(line, "id: "); ok {
			projectID = strings.TrimSpace(value)
		}
	}
	key := "websocket." + projectID + "." + made.Client.ID.String() + ".password"
	description := filepath.Join(workspace.root, filepath.FromSlash(made.Path))
	fileHolds := func(text string) bool {
		content, _ := os.ReadFile(description)
		return strings.Contains(string(content), text)
	}
	saved, err := workspace.SaveWebSocketClientEditor(made.Path, WebSocketClientEdit{WebSocketClientDefinition: made.Client, NewPassword: password("секрет 1")}, made.Revision)
	if err != nil || store.entries[key] != "секрет 1" || len(store.entries) != 1 || !saved.Client.PasswordSet || !fileHolds("password_set: true") || fileHolds("секрет") {
		t.Fatalf("the password typed: %v; the store %v; the client %+v", err, store.entries, saved.Client)
	}
	untouched := saved.Client
	untouched.User = "склад"
	saved, err = workspace.SaveWebSocketClientEditor(saved.Path, WebSocketClientEdit{WebSocketClientDefinition: untouched}, saved.Revision)
	if err != nil || store.entries[key] != "секрет 1" || !saved.Client.PasswordSet {
		t.Fatalf("an untouched password: %v; the store %v; the client %+v", err, store.entries, saved.Client)
	}
	if _, err := workspace.SaveWebSocketClientEditor(saved.Path, WebSocketClientEdit{WebSocketClientDefinition: saved.Client, NewPassword: password("другой")}, made.Revision); !errors.Is(err, ErrSourceChanged) || store.entries[key] != "секрет 1" {
		t.Fatalf("a save at an old revision: %v; the store %v", err, store.entries)
	}
	store.failing = true
	if _, err := workspace.SaveWebSocketClientEditor(saved.Path, WebSocketClientEdit{WebSocketClientDefinition: saved.Client, NewPassword: password("")}, saved.Revision); err == nil || store.entries[key] != "секрет 1" || !fileHolds("password_set: true") {
		t.Fatalf("a store refusing: %v; the store %v", err, store.entries)
	}
	// The folder of the client unwritable: the description is not written,
	// and the entry is put back.
	folder := filepath.Dir(description)
	if err := os.Chmod(folder, 0o555); err != nil {
		t.Fatal(err)
	}
	_, err = workspace.SaveWebSocketClientEditor(saved.Path, WebSocketClientEdit{WebSocketClientDefinition: saved.Client, NewPassword: password("новый")}, saved.Revision)
	_ = os.Chmod(folder, 0o755)
	if err == nil || store.entries[key] != "секрет 1" {
		t.Fatalf("a description not written: %v; the store %v", err, store.entries)
	}
	saved, err = workspace.SaveWebSocketClientEditor(saved.Path, WebSocketClientEdit{WebSocketClientDefinition: saved.Client, NewPassword: password("")}, saved.Revision)
	if _, kept := store.entries[key]; err != nil || kept || saved.Client.PasswordSet || fileHolds("password_set") {
		t.Fatalf("the password cleared: %v; the store %v; the client %+v", err, store.entries, saved.Client)
	}
	// The deletion takes the entries of the client away, a secret header's
	// too, and leaves another client's.
	other, err := workspace.CreateWebSocketClient("")
	if err != nil {
		t.Fatal(err)
	}
	saved, err = workspace.SaveWebSocketClientEditor(saved.Path, WebSocketClientEdit{WebSocketClientDefinition: saved.Client, NewPassword: password("p")}, saved.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.SaveWebSocketClientEditor(other.Path, WebSocketClientEdit{WebSocketClientDefinition: other.Client, NewPassword: password("q")}, other.Revision); err != nil {
		t.Fatal(err)
	}
	withHeader := saved.Client
	withHeader.Headers = []metadata.WebSocketHeader{{Name: "Authorization", Secret: true}}
	file, err := os.Create(description)
	if err != nil {
		t.Fatal(err)
	}
	if err := metadata.Encode(file, withHeader); err != nil {
		t.Fatal(err)
	}
	_ = file.Close()
	header := "websocket." + projectID + "." + made.Client.ID.String() + ".header.authorization"
	store.entries[header] = "Bearer x"
	opened, err := workspace.ReadWebSocketClientEditor(saved.Path)
	if err != nil {
		t.Fatal(err)
	}
	if err := workspace.DeleteWebSocketClient(opened.Path, opened.Revision); err != nil {
		t.Fatal(err)
	}
	otherKey := "websocket." + projectID + "." + other.Client.ID.String() + ".password"
	if len(store.entries) != 1 || store.entries[otherKey] != "q" {
		t.Fatalf("after the deletion the store holds %v", store.entries)
	}
}
