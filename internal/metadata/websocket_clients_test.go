package metadata

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
)

const (
	firstWebSocketClient  = "f7000000-0000-4000-8000-000000000001"
	secondWebSocketClient = "f7000000-0000-4000-8000-000000000002"
)

func writeWebSocketClient(t *testing.T, root, id, name, body string) {
	t.Helper()
	writeMetadata(t, root, WebSocketClientKind, id, `format: 1
id: `+id+`
name: `+name+`
title: {ru: `+name+`}
`+body)
}

// refusedWebSocketClient writes one client with the given body and returns the
// error its load must end with.
func refusedWebSocketClient(t *testing.T, body, what string) string {
	t.Helper()
	root := metadataProject(t)
	writeWebSocketClient(t, root, firstWebSocketClient, "Биржа", body)
	return loadRefused(t, root, what)
}

// A client carries every property the help lists for it, and is found by name
// and identifier. Catches a kind still read by identity alone - every line
// below would be refused as unknown - and a property that is read into the
// wrong field or not at all.
func TestWebSocketClientCarriesItsWholeComposition(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeWebSocketClient(t, root, firstWebSocketClient, "Биржа", `comment: Котировки
predefined: true
auto_connect: true
server_url: wss://quotes.example.org/stream
headers:
  - {name: X-Api-Version, value: "2"}
  - {name: Sec-WebSocket-Protocol, value: quotes.v2}
use_os_authentication: true
use_os_proxy: true
user: robot
password_set: true
timeout: 0
`)
	writeWebSocketClient(t, root, secondWebSocketClient, "Склад", "")
	module := filepath.Join(root, "metadata", string(WebSocketClientKind), "Биржа", project.ClientModuleFile)
	if err := os.WriteFile(module, []byte("Процедура ПриПолученииСообщения(Соединение, Сообщение)\nКонецПроцедуры\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	client, ok := catalog.WebSocketClient("биржа")
	if !ok {
		t.Fatal("the client is not found by name")
	}
	if !client.Predefined || !client.AutoConnect || !client.UseOSAuthentication || !client.UseOSProxy || !client.PasswordSet {
		t.Fatalf("a flag was lost: %+v", client)
	}
	if client.ServerURL != "wss://quotes.example.org/stream" || client.User != "robot" || client.Comment != "Котировки" {
		t.Fatalf("a text property was lost: %+v", client)
	}
	if !slices.Equal(client.Headers, []WebSocketHeader{{Name: "X-Api-Version", Value: "2"}, {Name: "Sec-WebSocket-Protocol", Value: "quotes.v2"}}) {
		t.Fatalf("the headers came back as %+v", client.Headers)
	}
	// A zero somebody wrote is carried as written; only an omitted timeout is
	// the default of thirty.
	if client.ConnectionTimeout() != 0 {
		t.Fatalf("an explicit zero timeout became %d", client.ConnectionTimeout())
	}
	plain, ok := catalog.WebSocketClientByID(catalog.WebSocketClients[1].ID)
	if !ok {
		t.Fatal("the client is not found by identifier")
	}
	if plain.Name != "Склад" || plain.ConnectionTimeout() != 30 || plain.PasswordSet {
		t.Fatalf("a client that names nothing came back as %+v with timeout %d", plain, plain.ConnectionTimeout())
	}
}

// A copy handed out is a copy. Catches a clone that copies the structure and
// shares the map of headers and the timeout, so that a caller changing what it
// was given changes the catalog for everybody.
func TestWebSocketClientIsHandedOutAsACopy(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeWebSocketClient(t, root, firstWebSocketClient, "Биржа", "headers: [{name: X-Api-Version, value: \"2\"}]\ntimeout: 10\n")
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	given, _ := catalog.WebSocketClient("Биржа")
	given.Headers[0].Value = "3"
	*given.Timeout = 99
	again, _ := catalog.WebSocketClient("Биржа")
	if again.Headers[0].Value != "2" || again.ConnectionTimeout() != 10 {
		t.Fatalf("changing a copy changed the catalog: %+v, timeout %d", again.Headers, again.ConnectionTimeout())
	}
}

// The password never lies in a project file, whatever the line says - even an
// empty one, because a line that is accepted empty is the line somebody fills
// in next. Catches a password that is read and kept, and one refused as merely
// unknown without saying where passwords go.
func TestWebSocketClientPasswordIsNotKeptInAFile(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"a password":        "password: s3cret\n",
		"an empty password": "password: \"\"\n",
		"beside the flag":   "password_set: true\npassword: s3cret\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			message := refusedWebSocketClient(t, body, "a password in the file")
			if !strings.Contains(message, "password is not kept in project files") || !strings.Contains(message, "password_set") {
				t.Fatalf("the error does not say where the password goes: %v", message)
			}
			if strings.Contains(message, "s3cret") {
				t.Fatalf("the error repeats the password: %v", message)
			}
		})
	}
}

// A header is a word of HTTP and a line of text. Catches, above all, a value
// with a line break - it would write into the handshake a header nobody
// declared - and two names the protocol reads as one.
func TestWebSocketClientHeadersAreCheckedForTheHandshake(t *testing.T) {
	t.Parallel()
	cases := map[string]struct{ body, says string }{
		"a line break in a value": {"headers:\n  - {name: X-Tenant, value: \"a\\r\\nAuthorization: Basic eA==\"}\n", "headers[0] X-Tenant must be one line"},
		// The break first: the check looks from the first symbol (mutant
		// of 2.556, `>= 0` to `> 0`).
		"a line break first":      {"headers:\n  - {name: X-Tenant, value: \"\\r\\nAuthorization: Basic eA==\"}\n", "headers[0] X-Tenant must be one line"},
		"a name with a space":     {"headers:\n  - {name: \"X Tenant\", value: a}\n", `"X Tenant" is not a header name`},
		"a name with a colon":     {"headers:\n  - {name: \"X-Tenant:\", value: a}\n", `"X-Tenant:" is not a header name`},
		"names differing by case": {"headers:\n  - {name: X-Tenant, value: a}\n  - {name: x-tenant, value: b}\n", `headers[1]: "X-Tenant" and "x-tenant" are one header`},
	}
	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if message := refusedWebSocketClient(t, test.body, name); !strings.Contains(message, test.says) {
				t.Fatalf("the error does not say what is wrong: %v", message)
			}
		})
	}
	// A tab is allowed inside a value: it is whitespace to HTTP, not the end
	// of a line.
	root := metadataProject(t)
	writeWebSocketClient(t, root, firstWebSocketClient, "Биржа", "headers:\n  - {name: X-Tenant, value: \"a\\tb\"}\n")
	if _, err := Load(root); err != nil {
		t.Fatalf("a tab inside a header value was refused: %v", err)
	}
}

// The server address is checked in form and nothing more. Catches a space or a
// line break, which no connection could open, and a check that demands more
// than the platform does: an export of the platform itself keeps an address of
// three letters with no scheme and no host, and it has to be carried.
func TestWebSocketClientServerAddressIsCheckedInFormOnly(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"a space in the path": "server_url: wss://quotes.example.org/the stream\n",
		"a line break inside": "server_url: \"wss://quotes.example.org/\\nstream\"\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if message := refusedWebSocketClient(t, body, name); !strings.Contains(message, "server_url") {
				t.Fatalf("the error does not name the property: %v", message)
			}
		})
	}
	for _, address := range []string{"soc", "/stream", "quotes.example.org", "wss://quotes.example.org/stream"} {
		root := metadataProject(t)
		writeWebSocketClient(t, root, firstWebSocketClient, "Биржа", "server_url: "+address+"\n")
		if _, err := Load(root); err != nil {
			t.Errorf("the address %q the platform itself writes was refused: %v", address, err)
		}
	}
}

// The timeout is never negative and never absurd. Catches a check on one side
// only.
func TestWebSocketClientTimeoutIsBounded(t *testing.T) {
	t.Parallel()
	if message := refusedWebSocketClient(t, "timeout: -1\n", "timeout -1"); !strings.Contains(message, "timeout must not be negative") {
		t.Fatalf("a negative timeout: %v", message)
	}
}

// The user name is a line of text. Catches a control character that would
// reach the authentication header.
func TestWebSocketClientUserIsOneLine(t *testing.T) {
	t.Parallel()
	if message := refusedWebSocketClient(t, "user: \"robot\\n\"\n", "a user with a line break"); !strings.Contains(message, "user must be one line") {
		t.Fatalf("the error does not name the property: %v", message)
	}
}

// The folder holds the description and the module and nothing else. Catches a
// second module under another name - code nobody runs that reads like the code
// that does.
func TestWebSocketClientFolderHoldsNothingElse(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeWebSocketClient(t, root, firstWebSocketClient, "Биржа", "")
	stray := filepath.Join(root, "metadata", string(WebSocketClientKind), "Биржа", "МодульСервиса.bsl")
	if err := os.WriteFile(stray, []byte("\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if message := loadRefused(t, root, "a second module"); !strings.Contains(message, `"МодульСервиса.bsl"`) {
		t.Fatalf("the error does not name the file: %v", message)
	}
}

// Reading is strict for this kind as for every other, and the client lies in a
// folder of its own name. Catches a decoder without strictness and a loader
// that never compares the folder with the name.
func TestWebSocketClientIsReadStrictlyFromAFolderOfItsName(t *testing.T) {
	t.Parallel()
	if message := refusedWebSocketClient(t, "infobase_user: robot\n", "a property nobody described"); !strings.Contains(message, "infobase_user") {
		t.Fatalf("the error does not name the property: %v", message)
	}
	root := metadataProject(t)
	writeWebSocketClient(t, root, firstWebSocketClient, "Биржа", "")
	from := filepath.Join(root, "metadata", string(WebSocketClientKind), "Биржа")
	if err := os.Rename(from, filepath.Join(filepath.Dir(from), "Котировки")); err != nil {
		t.Fatal(err)
	}
	if message := loadRefused(t, root, "a client in a folder of another name"); !strings.Contains(message, "lies in a folder called Котировки") {
		t.Fatalf("the error does not say what is wrong: %v", message)
	}
}

// The three headers that carry credentials by what the protocol says they are
// never lie in a project file, whatever their case. Catches each of the three
// accepted and kept - a password in the clear under another name - and a
// message that repeats the value it refuses.
func TestWebSocketClientCredentialHeadersAreNotKeptInAFile(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"Authorization", "proxy-authorization", "COOKIE"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			message := refusedWebSocketClient(t, "headers:\n  - {name: "+name+", value: Bearer s3cret}\n", "a credential in a header")
			if !strings.Contains(message, "headers[0] "+name+" carries credentials") || !strings.Contains(message, "marks it secret") {
				t.Fatalf("the error does not say where the header goes: %v", message)
			}
			if strings.Contains(message, "s3cret") {
				t.Fatalf("the error repeats the credential: %v", message)
			}
		})
	}
}

// A header kept as a secret has no value in the file and keeps its place in
// the list. Catches a secret that carries its value all the same, and a
// secret header put apart from the others, losing the order the designer
// keeps.
func TestWebSocketClientSecretHeadersKeepTheirPlace(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeWebSocketClient(t, root, firstWebSocketClient, "Биржа", "headers:\n  - {name: X-Api-Version, value: \"2\"}\n"+
		"  - {name: Authorization, secret: true}\n  - {name: X-Api-Key, secret: true}\n  - {name: Accept, value: text}\n")
	catalog, err := Load(root)
	if err != nil {
		t.Fatalf("headers marked secret were refused: %v", err)
	}
	client, _ := catalog.WebSocketClient("Биржа")
	want := []WebSocketHeader{{Name: "X-Api-Version", Value: "2"}, {Name: "Authorization", Secret: true},
		{Name: "X-Api-Key", Secret: true}, {Name: "Accept", Value: "text"}}
	if !slices.Equal(client.Headers, want) {
		t.Fatalf("the headers came back as %+v", client.Headers)
	}
	message := refusedWebSocketClient(t, "headers:\n  - {name: X-Api-Key, value: s3cret, secret: true}\n", "a secret with its value")
	if !strings.Contains(message, "headers[0] X-Api-Key is secret and has a value in the file") || strings.Contains(message, "s3cret") {
		t.Fatalf("the error does not say what is wrong, or repeats the value: %v", message)
	}
}

// The headers come back, and are written back, in the order of the file - not
// sorted, as the dictionary they once were wrote them. Catches the order lost
// on the way in or on the way out.
func TestWebSocketClientHeadersKeepTheirOrder(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeWebSocketClient(t, root, firstWebSocketClient, "Биржа", "headers:\n  - {name: X-Zeta, value: \"1\"}\n"+
		"  - {name: Accept, value: \"2\"}\n  - {name: X-Middle, value: \"3\"}\n")
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	client, _ := catalog.WebSocketClient("Биржа")
	order := func(headers []WebSocketHeader) string {
		names := make([]string, len(headers))
		for index, header := range headers {
			names[index] = header.Name
		}
		return strings.Join(names, ",")
	}
	if got := order(client.Headers); got != "X-Zeta,Accept,X-Middle" {
		t.Fatalf("the headers were read as %s", got)
	}
	var written bytes.Buffer
	if err := Encode(&written, client); err != nil {
		t.Fatal(err)
	}
	again, err := DecodeWebSocketClient("client.yaml", bytes.NewReader(written.Bytes()), metadataConfiguration())
	if err != nil {
		t.Fatal(err)
	}
	if got := order(again.Headers); got != "X-Zeta,Accept,X-Middle" {
		t.Fatalf("the headers were written as %s:\n%s", got, written.String())
	}
}
