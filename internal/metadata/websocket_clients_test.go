package metadata

import (
	"os"
	"path/filepath"
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
  X-Api-Version: "2"
  Sec-WebSocket-Protocol: quotes.v2
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
	if client.Headers["X-Api-Version"] != "2" || client.Headers["Sec-WebSocket-Protocol"] != "quotes.v2" {
		t.Fatalf("the headers came back as %+v", client.Headers)
	}
	if names := client.HeaderNames(); strings.Join(names, ",") != "Sec-WebSocket-Protocol,X-Api-Version" {
		t.Fatalf("the headers are not in a fixed order: %v", names)
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
	writeWebSocketClient(t, root, firstWebSocketClient, "Биржа", "headers: {X-Api-Version: \"2\"}\ntimeout: 10\n")
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	given, _ := catalog.WebSocketClient("Биржа")
	given.Headers["X-Api-Version"] = "3"
	*given.Timeout = 99
	again, _ := catalog.WebSocketClient("Биржа")
	if again.Headers["X-Api-Version"] != "2" || again.ConnectionTimeout() != 10 {
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
		"a line break in a value": {"headers:\n  X-Tenant: \"a\\r\\nAuthorization: Basic eA==\"\n", "headers.X-Tenant must be one line"},
		"a name with a space":     {"headers:\n  \"X Tenant\": a\n", `"X Tenant" is not a header name`},
		"a name with a colon":     {"headers:\n  \"X-Tenant:\": a\n", `"X-Tenant:" is not a header name`},
		"names differing by case": {"headers:\n  X-Tenant: a\n  x-tenant: b\n", "are one header"},
		"a value past the bound":  {"headers:\n  X-Tenant: " + strings.Repeat("a", maxWebSocketHeaderValue+1) + "\n", "must not exceed"},
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
	writeWebSocketClient(t, root, firstWebSocketClient, "Биржа", "headers:\n  X-Tenant: \"a\\tb\"\n")
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
		"past the bound":      "server_url: wss://" + strings.Repeat("a", maxLocationURLLength) + "\n",
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
	for _, value := range []string{"-1", "86401"} {
		if message := refusedWebSocketClient(t, "timeout: "+value+"\n", "timeout "+value); !strings.Contains(message, "timeout must be between") {
			t.Fatalf("timeout %s: the error does not name the bound: %v", value, message)
		}
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
			message := refusedWebSocketClient(t, "headers:\n  "+name+": Bearer s3cret\n", "a credential in a header")
			if !strings.Contains(message, "headers."+name+" carries credentials") || !strings.Contains(message, "secret_headers") {
				t.Fatalf("the error does not say where the header goes: %v", message)
			}
			if strings.Contains(message, "s3cret") {
				t.Fatalf("the error repeats the credential: %v", message)
			}
		})
	}
}

// A header kept as a secret is named and not written out. Catches a name that
// is not a header, a name given twice, and a header that is both written out and
// declared a secret - which of the two would be sent nobody could say.
func TestWebSocketClientSecretHeadersAreNamedOnly(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeWebSocketClient(t, root, firstWebSocketClient, "Биржа", "secret_headers: [Authorization, X-Api-Key]\nheaders: {X-Api-Version: \"2\"}\n")
	catalog, err := Load(root)
	if err != nil {
		t.Fatalf("headers named as secrets were refused: %v", err)
	}
	client, _ := catalog.WebSocketClient("Биржа")
	if strings.Join(client.SecretHeaders, ",") != "Authorization,X-Api-Key" {
		t.Fatalf("the secret headers came back as %v", client.SecretHeaders)
	}
	client.SecretHeaders[0] = "Испорчено"
	if again, _ := catalog.WebSocketClient("Биржа"); again.SecretHeaders[0] != "Authorization" {
		t.Fatal("changing a copy changed the catalog")
	}
	for name, test := range map[string]struct{ body, says string }{
		"not a header name":        {"secret_headers: [\"X Api Key\"]\n", `"X Api Key" is not a header name`},
		"one name twice":           {"secret_headers: [X-Api-Key, x-api-key]\n", "names \"x-api-key\" twice"},
		"a secret written out too": {"secret_headers: [X-Api-Key]\nheaders: {x-api-key: abc}\n", "a header is either a secret or not"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if message := refusedWebSocketClient(t, test.body, name); !strings.Contains(message, test.says) {
				t.Fatalf("the error does not say what is wrong: %v", message)
			}
		})
	}
}
