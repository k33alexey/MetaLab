package metadata

import (
	"fmt"
	"io"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// HTTPServiceKind holds the addresses an outside system reaches this
// configuration at without a description of types: a path, a verb, a handler.
const HTTPServiceKind Kind = "http-services"

const (
	maxTemplatesPerService = 256
	maxMethodsPerTemplate  = 32
)

// httpMethods are the verbs a method may answer. The list is closed and holds
// the verbs of HTTP together with those WebDAV adds, plus "ANY" for a handler
// that answers whatever arrives.
//
// Closed on purpose, unlike the type names of a web service: a verb is not an
// agreement between two systems but a word of the protocol itself, and a verb
// nobody can send is a handler nobody will ever reach.
var httpMethods = []string{
	"ANY", "GET", "PUT", "POST", "DELETE", "PATCH", "MERGE", "CONNECT", "HEAD", "OPTIONS", "TRACE",
	"PROPFIND", "PROPPATCH", "MKCOL", "COPY", "MOVE", "LOCK", "UNLOCK",
}

// HTTPServiceMethod is one verb answered at one address.
type HTTPServiceMethod struct {
	ID      uuid.UUID     `yaml:"id" json:"id"`
	Name    string        `yaml:"name" json:"name"`
	Title   LocalizedText `yaml:"title" json:"title"`
	Comment string        `yaml:"comment,omitempty" json:"comment,omitempty"`
	// Method is the verb, written the way the protocol writes it.
	Method string `yaml:"method" json:"method"`
	// Handler is the routine of the service's module that answers. The module
	// lies in the service's folder under the name of its role, so only the
	// routine is named here.
	Handler string `yaml:"handler" json:"handler"`
}

// URLTemplate is one address of the service, relative to its root.
type URLTemplate struct {
	ID      uuid.UUID     `yaml:"id" json:"id"`
	Name    string        `yaml:"name" json:"name"`
	Title   LocalizedText `yaml:"title" json:"title"`
	Comment string        `yaml:"comment,omitempty" json:"comment,omitempty"`
	// Template is the path, with parameters in braces and a star for the rest
	// of it: /bill/{Версия}/*. It is written as the caller writes it, because
	// it is matched against what the caller sent.
	Template string              `yaml:"template" json:"template"`
	Methods  []HTTPServiceMethod `yaml:"methods,omitempty" json:"methods,omitempty"`
}

// HTTPServiceDefinition is one entry point for an outside system.
type HTTPServiceDefinition struct {
	Format  int           `yaml:"format" json:"format"`
	ID      uuid.UUID     `yaml:"id" json:"id"`
	Name    string        `yaml:"name" json:"name"`
	Title   LocalizedText `yaml:"title" json:"title"`
	Comment string        `yaml:"comment,omitempty" json:"comment,omitempty"`
	// RootURL is the first segment of every address this service answers at.
	// It is one segment and not a path: the rest is what the templates say,
	// and a root spelling out several segments would put half of an address in
	// one place and half in another.
	RootURL       string           `yaml:"root_url" json:"rootUrl"`
	ReuseSessions SessionReuseMode `yaml:"reuse_sessions,omitempty" json:"reuseSessions,omitempty"`
	// SessionMaxAge is how long a reused session is kept, in seconds. Zero
	// means the platform decides.
	SessionMaxAge int           `yaml:"session_max_age,omitempty" json:"sessionMaxAge,omitempty"`
	Templates     []URLTemplate `yaml:"templates,omitempty" json:"templates,omitempty"`
}

// DecodeHTTPService reads and validates one HTTP service.
func DecodeHTTPService(source string, reader io.Reader, configuration project.Project) (HTTPServiceDefinition, error) {
	var value HTTPServiceDefinition
	if err := decodeStrict(source, reader, &value); err != nil {
		return HTTPServiceDefinition{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, configuration)
	issues = append(issues, validateRootURL(value.RootURL)...)
	switch value.ReuseSessions {
	case "", DoNotReuseSession, ReuseSession, ReuseSessionAutomatic:
	default:
		issues = append(issues, "reuse_sessions must be do-not-use, use or auto")
	}
	if value.SessionMaxAge < 0 {
		issues = append(issues, "session_max_age must not be negative")
	}
	issues = append(issues, validateURLTemplates(value.Templates, configuration)...)
	if err := issuesError(source, value.Format, issues); err != nil {
		return HTTPServiceDefinition{}, err
	}
	return value, nil
}

// validateRootURL checks the first segment of every address of the service.
func validateRootURL(value string) []string {
	if value == "" {
		return []string{"root_url must name the address the service answers at"}
	}
	if utf8.RuneCountInString(value) > 128 {
		return []string{"root_url must not exceed 128 characters"}
	}
	if strings.ContainsAny(value, "/?#") {
		return []string{"root_url must be one segment of the address and not a path"}
	}
	for _, symbol := range value {
		if unicode.IsSpace(symbol) || !unicode.IsPrint(symbol) {
			return []string{"root_url must not contain spaces or control characters"}
		}
	}
	return nil
}

// validateURLTemplates checks the addresses of one service: each template, its
// methods, and that no two of them answer the same call.
func validateURLTemplates(templates []URLTemplate, configuration project.Project) []string {
	if len(templates) > maxTemplatesPerService {
		return []string{fmt.Sprintf("templates must not contain more than %d items", maxTemplatesPerService)}
	}
	var issues []string
	names, ids, paths := map[string]bool{}, map[uuid.UUID]bool{}, map[string]bool{}
	for index, template := range templates {
		where := fmt.Sprintf("templates[%d]", index)
		if template.ID.IsZero() {
			issues = append(issues, where+".id must be a non-zero UUID")
		} else if ids[template.ID] {
			issues = append(issues, where+".id is used twice")
		}
		ids[template.ID] = true
		if !validIdentifier(template.Name) || utf8.RuneCountInString(template.Name) > 128 {
			issues = append(issues, where+".name must start with a letter and contain only letters or digits")
		} else if names[strings.ToLower(template.Name)] {
			issues = append(issues, where+".name is used twice")
		}
		names[strings.ToLower(template.Name)] = true
		issues = append(issues, validateTitle(where+".title", template.Title, configuration)...)
		issues = append(issues, validateTemplatePath(where+".template", template.Template)...)
		// Two templates on one path are two answers to one call, and which one
		// answers is decided by the order they happen to be read in.
		if paths[template.Template] {
			issues = append(issues, where+".template answers the same address as another template")
		}
		paths[template.Template] = true
		issues = append(issues, validateHTTPMethods(where, template.Methods, configuration)...)
	}
	return issues
}

// validateTemplatePath checks one address: it starts where the root ends, and
// what it holds is written the way the caller writes it.
func validateTemplatePath(path, value string) []string {
	if value == "" {
		return []string{path + " must say which address it answers"}
	}
	if utf8.RuneCountInString(value) > 512 {
		return []string{path + " must not exceed 512 characters"}
	}
	if !strings.HasPrefix(value, "/") {
		return []string{path + " must begin with a slash: it continues the root address"}
	}
	for _, symbol := range value {
		if unicode.IsSpace(symbol) || !unicode.IsPrint(symbol) {
			return []string{path + " must not contain spaces or control characters"}
		}
	}
	// A parameter is a name in braces. An unclosed brace is a parameter that
	// matches nothing, and the address then answers no call at all.
	depth := 0
	for _, symbol := range value {
		switch symbol {
		case '{':
			depth++
			if depth > 1 {
				return []string{path + " opens a parameter inside a parameter"}
			}
		case '}':
			depth--
			if depth < 0 {
				return []string{path + " closes a parameter that was never opened"}
			}
		}
	}
	if depth != 0 {
		return []string{path + " leaves a parameter unclosed"}
	}
	return nil
}

func validateHTTPMethods(owner string, methods []HTTPServiceMethod, configuration project.Project) []string {
	if len(methods) > maxMethodsPerTemplate {
		return []string{fmt.Sprintf("%s.methods must not contain more than %d items", owner, maxMethodsPerTemplate)}
	}
	var issues []string
	names, ids, verbs := map[string]bool{}, map[uuid.UUID]bool{}, map[string]bool{}
	for index, method := range methods {
		where := fmt.Sprintf("%s.methods[%d]", owner, index)
		if method.ID.IsZero() {
			issues = append(issues, where+".id must be a non-zero UUID")
		} else if ids[method.ID] {
			issues = append(issues, where+".id is used twice")
		}
		ids[method.ID] = true
		if !validIdentifier(method.Name) || utf8.RuneCountInString(method.Name) > 128 {
			issues = append(issues, where+".name must start with a letter and contain only letters or digits")
		} else if names[strings.ToLower(method.Name)] {
			issues = append(issues, where+".name is used twice")
		}
		names[strings.ToLower(method.Name)] = true
		issues = append(issues, validateTitle(where+".title", method.Title, configuration)...)
		if !slices.Contains(httpMethods, method.Method) {
			issues = append(issues, where+".method must be a verb of the protocol, written in capitals")
			continue
		}
		// One verb answered twice at one address is one handler nobody calls,
		// and which of the two it is nothing says.
		if verbs[method.Method] {
			issues = append(issues, where+".method answers "+method.Method+" at an address that already answers it")
		}
		verbs[method.Method] = true
		if !validIdentifier(method.Handler) || utf8.RuneCountInString(method.Handler) > 128 {
			issues = append(issues, where+".handler must name a procedure of the service module")
		}
	}
	return issues
}

func cloneHTTPService(value HTTPServiceDefinition) HTTPServiceDefinition {
	value.Title = cloneTitle(value.Title)
	templates := make([]URLTemplate, len(value.Templates))
	for index, template := range value.Templates {
		template.Title = cloneTitle(template.Title)
		methods := make([]HTTPServiceMethod, len(template.Methods))
		for position, method := range template.Methods {
			method.Title = cloneTitle(method.Title)
			methods[position] = method
		}
		template.Methods = methods
		templates[index] = template
	}
	value.Templates = templates
	return value
}

// HTTPService returns one service by name, folded case.
func (catalog *Catalog) HTTPService(name string) (HTTPServiceDefinition, bool) {
	index, ok := catalog.httpServiceByName[strings.ToLower(name)]
	if !ok {
		return HTTPServiceDefinition{}, false
	}
	return cloneHTTPService(catalog.HTTPServices[index]), true
}

// validateHTTPServices checks what no service can check about itself: that no
// two of them answer at the same root address.
//
// Two services on one root are two answers to every call that reaches it, and
// the caller gets whichever the server resolved first - an outcome that looks
// like the service working, until it is the other one that answers.
func (catalog *Catalog) validateHTTPServices() error {
	roots := make(map[string]string, len(catalog.HTTPServices))
	for _, service := range catalog.HTTPServices {
		root := strings.ToLower(service.RootURL)
		if previous, taken := roots[root]; taken {
			return fmt.Errorf("HTTP services %s and %s both answer at %s", previous, service.Name, service.RootURL)
		}
		roots[root] = service.Name
	}
	return nil
}
