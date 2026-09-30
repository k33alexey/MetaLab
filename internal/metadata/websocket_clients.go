package metadata

import (
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

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
const WebSocketClientKind Kind = "websocket-clients"

const (
	// defaultWebSocketTimeout is the connection timeout, in seconds, of a
	// client that does not name one.
	defaultWebSocketTimeout = 30
	// maxWebSocketTimeout bounds the timeout at a day: a connection that takes
	// longer than that to open is not being waited for, it is forgotten.
	maxWebSocketTimeout = 86400
	maxWebSocketHeaders = 64
	// maxWebSocketHeaderValue bounds one header value. Servers refuse the whole
	// request line past a few kilobytes, so a longer value is a mistake.
	maxWebSocketHeaderValue = 4096
	maxWebSocketUserLength  = 256
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
	// Headers are added to the GET request that opens the connection.
	Headers map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
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

// HeaderNames returns the names of the additional headers in a fixed order, so
// that whatever writes them writes the same request every time.
func (client WebSocketClientDefinition) HeaderNames() []string {
	names := make([]string, 0, len(client.Headers))
	for name := range client.Headers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// DecodeWebSocketClient reads and validates one WebSocket client.
func DecodeWebSocketClient(source string, reader io.Reader, configuration project.Project) (WebSocketClientDefinition, error) {
	var value WebSocketClientDefinition
	if err := decodeStrict(source, reader, &value); err != nil {
		return WebSocketClientDefinition{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, configuration)
	if value.Password != nil {
		issues = append(issues, "password is not kept in project files: it lies in the operating system's protected store, and the file says password_set: true")
	}
	issues = append(issues, validateWebSocketServerURL(value.ServerURL)...)
	issues = append(issues, validateWebSocketHeaders(value.Headers)...)
	if utf8.RuneCountInString(value.User) > maxWebSocketUserLength {
		issues = append(issues, fmt.Sprintf("user must not exceed %d characters", maxWebSocketUserLength))
	} else if strings.IndexFunc(value.User, unicode.IsControl) >= 0 {
		issues = append(issues, "user must be one line without control characters")
	}
	if value.Timeout != nil && (*value.Timeout < 0 || *value.Timeout > maxWebSocketTimeout) {
		issues = append(issues, fmt.Sprintf("timeout must be between 0 and %d seconds", maxWebSocketTimeout))
	}
	if err := issuesError(source, value.Format, issues); err != nil {
		return WebSocketClientDefinition{}, err
	}
	return value, nil
}

// validateWebSocketServerURL checks the address in form: absolute, with a
// scheme and a host, on one line. Which schemes the prototype accepts the help
// does not say, so the scheme is not narrowed down.
func validateWebSocketServerURL(value string) []string {
	if value == "" {
		return nil
	}
	if len([]rune(value)) > maxLocationURLLength {
		return []string{fmt.Sprintf("server_url must not exceed %d characters", maxLocationURLLength)}
	}
	if strings.IndexFunc(value, func(symbol rune) bool { return unicode.IsControl(symbol) || unicode.IsSpace(symbol) }) >= 0 {
		return []string{"server_url must be one line without spaces or control characters"}
	}
	address, err := url.Parse(value)
	if err != nil || address.Scheme == "" || address.Host == "" {
		return []string{"server_url must be an absolute address with a scheme and a host"}
	}
	return nil
}

// validateWebSocketHeaders checks the additional headers: each name is a word
// of HTTP, each value is one line, and no two names differ by case alone.
//
// The value is the part that matters. A line break in it would write into the
// handshake a header nobody declared - and the handshake is where the server
// decides who is calling.
func validateWebSocketHeaders(headers map[string]string) []string {
	if len(headers) > maxWebSocketHeaders {
		return []string{fmt.Sprintf("headers must not contain more than %d items", maxWebSocketHeaders)}
	}
	var issues []string
	seen := make(map[string]string, len(headers))
	names := make([]string, 0, len(headers))
	for name := range headers {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		value := headers[name]
		if !httpToken(name) {
			issues = append(issues, fmt.Sprintf("headers: %q is not a header name", name))
			continue
		}
		folded := strings.ToLower(name)
		if previous, taken := seen[folded]; taken {
			issues = append(issues, fmt.Sprintf("headers %q and %q are one header", previous, name))
		}
		seen[folded] = name
		if len(value) > maxWebSocketHeaderValue {
			issues = append(issues, fmt.Sprintf("headers.%s must not exceed %d bytes", name, maxWebSocketHeaderValue))
		} else if strings.IndexFunc(value, func(symbol rune) bool { return symbol != '\t' && unicode.IsControl(symbol) }) >= 0 {
			issues = append(issues, fmt.Sprintf("headers.%s must be one line without control characters", name))
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
	if value.Headers != nil {
		headers := make(map[string]string, len(value.Headers))
		for name, header := range value.Headers {
			headers[name] = header
		}
		value.Headers = headers
	}
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
