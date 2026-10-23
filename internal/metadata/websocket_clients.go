package metadata

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"

	"github.com/k33alexey/MetaLab/internal/kinds"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// WebSocketClientKind holds the lasting connections this configuration opens to
// somebody else's WebSocket server.
//
// The object is a template and not the connection itself, the way a scheduled
// job is: the clients that run are records of the infobase. A predefined one is
// created by the platform from this object and cannot be deleted; others are
// created by application code from the same object under a key of their own.
// What this object says is what such a client starts with.
const WebSocketClientKind = kinds.WebSocketClient

const (
	// defaultWebSocketTimeout is the connection timeout, in seconds, of a
	// client that does not name one.
	defaultWebSocketTimeout = 30
)

// WebSocketClientDefinition is one WebSocket client.
type WebSocketClientDefinition struct {
	Format  int           `yaml:"format" json:"format"`
	ID      uuid.UUID     `yaml:"id" json:"id"`
	Name    string        `yaml:"name" json:"name"`
	Title   LocalizedText `yaml:"title" json:"title"`
	Comment string        `yaml:"comment,omitempty" json:"comment,omitempty"`
	// Predefined says the platform creates the client by itself from this
	// object. Such a client may be changed and not deleted.
	Predefined bool `yaml:"predefined,omitempty" json:"predefined,omitempty"`
	// AutoConnect connects the client when the application starts and
	// connects it again whenever the connection drops.
	AutoConnect bool `yaml:"auto_connect,omitempty" json:"autoConnect,omitempty"`
	// ServerURL is the server the client connects to. It is not required: a
	// client created by code may be given its address there.
	ServerURL string `yaml:"server_url,omitempty" json:"serverUrl,omitempty"`
	// Headers are added to the GET request that opens the connection, in the
	// order the designer keeps them: its editor «Настройка заголовков» moves
	// a row up and down, and the export writes a list (opis of 15.10.2026,
	// websocket-klient.md). A header whose value is a credential is secret:
	// its value is kept where the password is, not here, and it keeps its
	// place in the list all the same. Three names always carry credentials
	// and are refused unless secret - see credentialHeaders; any other header
	// may be secret by choice, an API key for one, because a name cannot tell
	// a key from a version number.
	Headers []WebSocketHeader `yaml:"headers,omitempty" json:"headers,omitempty"`
	// UseOSAuthentication signs in to the server as the operating system
	// account. With no user and password it is the account of whatever
	// process serves the connection.
	UseOSAuthentication bool `yaml:"use_os_authentication,omitempty" json:"useOsAuthentication,omitempty"`
	// UseOSProxy takes the proxy settings of the computer the connection is
	// opened from.
	UseOSProxy bool `yaml:"use_os_proxy,omitempty" json:"useOsProxy,omitempty"`
	// User is the name the client signs in to the server with. It is not a
	// secret and lies in the file, as it does in the prototype.
	User string `yaml:"user,omitempty" json:"user,omitempty"`
	// PasswordSet says the client has a password. The password itself is not
	// here and never is: it is kept in the operating system's protected store
	// under the project and this object's identifier, and on the server in
	// the store of secrets. The prototype keeps it in the configuration, and
	// for us that would be a password in the clear in every clone of the
	// repository.
	PasswordSet bool `yaml:"password_set,omitempty" json:"passwordSet,omitempty"`
	// Password exists only to refuse it with an explanation. Without it the
	// line would be refused as an unknown property, which is true and tells
	// nobody where the password went.
	Password *string `yaml:"password,omitempty" json:"-"`
	// Timeout is the connection timeout in seconds. It is a pointer because
	// the default is thirty and not zero: an omitted line is thirty, and a
	// zero somebody wrote is carried as written - what zero means to the
	// prototype the help does not say, and putting thirty in its place would
	// lose the value.
	Timeout *int `yaml:"timeout,omitempty" json:"timeout,omitempty"`
}

// ConnectionTimeout is the timeout the client connects with, in seconds.
func (client WebSocketClientDefinition) ConnectionTimeout() int {
	if client.Timeout == nil {
		return defaultWebSocketTimeout
	}
	return *client.Timeout
}

// WebSocketHeader is one header of the handshake: its name, and its value or,
// for a secret one, the mark that the value lies in the store of secrets.
type WebSocketHeader struct {
	Name   string `yaml:"name" json:"name"`
	Value  string `yaml:"value,omitempty" json:"value,omitempty"`
	Secret bool   `yaml:"secret,omitempty" json:"secret,omitempty"`
}

// DecodeWebSocketClient reads and validates one WebSocket client.
func DecodeWebSocketClient(source string, reader io.Reader, configuration project.Project) (WebSocketClientDefinition, error) {
	return decodeDescription(source, reader, configuration, func(value WebSocketClientDefinition, configuration project.Project) []string {
		var issues []string
		if value.Password != nil {
			issues = append(issues, "password is not kept in project files: it lies in the operating system's protected store, and the file says password_set: true")
		}
		issues = append(issues, validateWebSocketServerURL(value.ServerURL)...)
		issues = append(issues, validateWebSocketHeaders(value.Headers)...)
		if strings.IndexFunc(value.User, unicode.IsControl) >= 0 {
			issues = append(issues, "user must be one line without control characters")
		}
		if value.Timeout != nil && *value.Timeout < 0 {
			issues = append(issues, "timeout must not be negative")
		}
		return issues
	})
}

// validateWebSocketServerURL checks the address in form only: one line with
// no spaces or control characters in it.
//
// Not as an absolute address. It was, and an export of the platform itself
// refuted it: a client there keeps a server address of three letters, no
// scheme and no host. What the prototype makes of such an address the help
// does not say - it may well be completed at run time - and refusing it would
// refuse a configuration the platform wrote.
func validateWebSocketServerURL(value string) []string {
	if strings.IndexFunc(value, func(symbol rune) bool { return unicode.IsControl(symbol) || unicode.IsSpace(symbol) }) >= 0 {
		return []string{"server_url must be one line without spaces or control characters"}
	}
	return nil
}

// credentialHeaders are the headers whose value is a credential by what the
// protocol says they are: who is calling, who is calling through a proxy, and
// the session the server handed out. Their value in a project file is a
// password in the clear in every clone of the repository, the same as the
// password property would be.
var credentialHeaders = []string{"authorization", "proxy-authorization", "cookie"}

// validateWebSocketHeaders checks the additional headers: each name is a word
// of HTTP, each value is one line, no two names differ by case alone, and a
// secret one has no value in the file.
//
// The value is the part that matters. A line break in it would write into the
// handshake a header nobody declared - and the handshake is where the server
// decides who is calling.
func validateWebSocketHeaders(headers []WebSocketHeader) []string {
	var issues []string
	seen := make(map[string]string, len(headers))
	for index, header := range headers {
		where := fmt.Sprintf("headers[%d]", index)
		if !httpToken(header.Name) {
			issues = append(issues, fmt.Sprintf("%s: %q is not a header name", where, header.Name))
			continue
		}
		folded := strings.ToLower(header.Name)
		if previous, taken := seen[folded]; taken {
			issues = append(issues, fmt.Sprintf("%s: %q and %q are one header", where, previous, header.Name))
		}
		seen[folded] = header.Name
		// The value is not repeated in a message about a secret or a
		// credential, for the reason it is refused: it is a credential.
		switch {
		case header.Secret && header.Value != "":
			issues = append(issues, fmt.Sprintf("%s %s is secret and has a value in the file: its value lies in the store of secrets", where, header.Name))
		case !header.Secret && slices.Contains(credentialHeaders, folded):
			issues = append(issues, fmt.Sprintf("%s %s carries credentials and is not kept in project files: its value lies in the store of secrets, and the file marks it secret", where, header.Name))
		case strings.IndexFunc(header.Value, func(symbol rune) bool { return symbol != '\t' && unicode.IsControl(symbol) }) >= 0:
			issues = append(issues, fmt.Sprintf("%s %s must be one line without control characters", where, header.Name))
		}
	}
	return issues
}

// httpToken reports whether a header name is a token of HTTP: visible ASCII
// without separators.
func httpToken(value string) bool {
	if value == "" {
		return false
	}
	for index := 0; index < len(value); index++ {
		symbol := value[index]
		if symbol <= ' ' || symbol >= 0x7f || strings.IndexByte(`"(),/:;<=>?@[\]{}`, symbol) >= 0 {
			return false
		}
	}
	return true
}

func cloneWebSocketClient(value WebSocketClientDefinition) WebSocketClientDefinition {
	value.Title = cloneTitle(value.Title)
	value.Headers = slices.Clone(value.Headers)
	if value.Timeout != nil {
		timeout := *value.Timeout
		value.Timeout = &timeout
	}
	return value
}

// WebSocketClient returns one WebSocket client by name, folded case.
func (catalog *Catalog) WebSocketClient(name string) (WebSocketClientDefinition, bool) {
	index, ok := catalog.webSocketClientByName[strings.ToLower(name)]
	if !ok {
		return WebSocketClientDefinition{}, false
	}
	return cloneWebSocketClient(catalog.WebSocketClients[index]), true
}

// WebSocketClientByID returns one WebSocket client by identifier.
func (catalog *Catalog) WebSocketClientByID(id uuid.UUID) (WebSocketClientDefinition, bool) {
	index, ok := catalog.webSocketClientByID[id]
	if !ok {
		return WebSocketClientDefinition{}, false
	}
	return cloneWebSocketClient(catalog.WebSocketClients[index]), true
}

// validateWebSocketClientFiles checks the folder of every client: its
// description, and its module when it has one, and nothing else. A second
// module beside the first under another name would be code nobody runs that
// reads like the code that does.
func (catalog *Catalog) validateWebSocketClientFiles(root string) error {
	if root == "" {
		return nil
	}
	allowed, _ := project.NamedFolderFiles(string(WebSocketClientKind))
	for _, item := range catalog.WebSocketClients {
		directory := filepath.Join(root, "metadata", string(WebSocketClientKind), item.Name)
		entries, err := os.ReadDir(directory)
		if err != nil {
			return fmt.Errorf("WebSocket client %s: %w", item.Name, err)
		}
		for _, entry := range entries {
			if entry.IsDir() || entry.Type()&fs.ModeSymlink != 0 || !slices.Contains(allowed, entry.Name()) {
				return fmt.Errorf("WebSocket client %s keeps %q, and a client keeps its description and its module",
					item.Name, entry.Name())
			}
		}
	}
	return nil
}
