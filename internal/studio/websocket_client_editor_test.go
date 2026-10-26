package studio

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/k33alexey/MetaLab/internal/metadata"
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
	saved, err := workspace.SaveWebSocketClientEditor(made.Path, edited, opened.Revision)
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
	if _, err := workspace.SaveWebSocketClientEditor(saved.Path, changed, saved.Revision); err == nil {
		t.Error("a save changing a secret header was taken")
	}
	if _, err := metadata.Load(workspace.root); err != nil {
		t.Fatalf("the project does not load: %v", err)
	}
}
