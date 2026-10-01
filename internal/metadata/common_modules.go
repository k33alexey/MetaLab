package metadata

import (
	"io"
	"strings"

	"github.com/k33alexey/MetaLab/internal/bsl/syntax"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

const CommonModuleKind Kind = "common-modules"

// CommonModuleDefinition is a named, cross-callable BSL module (modules/<Module>.bsl)
// exposed to every other module compiled alongside it. Unlike object/form
// modules its routines carry no per-routine &AtClient/&AtServer directives:
// Client/Server classify the whole module at once, matching how 1C common
// modules are authored. ServerCall and Privileged are modeled for Studio and
// future enforcement but are not yet separately enforced: any exported server
// routine is already remotely callable from client code (the compiler/VM
// already bridge that call automatically), and Privileged has no consumer
// yet in the role-based Permissions model from iteration 060.
type CommonModuleDefinition struct {
	Format     int           `yaml:"format"`
	ID         uuid.UUID     `yaml:"id"`
	Name       string        `yaml:"name"`
	Title      LocalizedText `yaml:"title"`
	Module     uuid.UUID     `yaml:"module"`
	Client     bool          `yaml:"client,omitempty"`
	Server     bool          `yaml:"server,omitempty"`
	ServerCall bool          `yaml:"server_call,omitempty"`
	Privileged bool          `yaml:"privileged,omitempty"`
	// Comment is for the developer and is not localized. All 739 common modules
	// of the demonstration configuration carry one.
	Comment string `yaml:"comment,omitempty"`
	// Global puts the exported procedures of this module into the global
	// context: they are called by name alone, without the module in front. It
	// is not decoration - code carried over from the prototype calls them that
	// way, and without this flag the call finds nothing.
	Global bool `yaml:"global,omitempty"`
	// ExternalConnection lets the procedures of this module be used over an
	// external connection.
	ExternalConnection bool `yaml:"external_connection,omitempty"`
	// ClientOrdinaryApplication is the prototype's flag of a module compiled
	// in the client of an ordinary application. ML has no ordinary
	// application, so the flag is carried and runs nothing: about 1600, 900
	// and 1000 modules of the configurations being moved have it, nearly all
	// beside the server flag, and dropping it would lose what the module was
	// written for.
	ClientOrdinaryApplication bool `yaml:"client_ordinary_application,omitempty"`
	// ReturnValuesReuse caches what the exported functions of this module
	// return. It is not a flag but a choice of three - see below.
	ReturnValuesReuse ReturnValuesReuse `yaml:"return_values_reuse,omitempty"`
}

// ReturnValuesReuse is how long the result of an exported function of a common
// module is kept: not at all, for the one server call, or for the session. The
// prototype has three values here and not two, and the difference is visible
// to an application: a function reused for the session keeps returning what it
// returned first, however the data has changed since.
type ReturnValuesReuse string

const (
	ReturnValuesReuseDontUse       ReturnValuesReuse = "dont-use"
	ReturnValuesReuseDuringRequest ReturnValuesReuse = "during-request"
	ReturnValuesReuseDuringSession ReturnValuesReuse = "during-session"
)

func validReturnValuesReuse(value ReturnValuesReuse) bool {
	switch value {
	case "", ReturnValuesReuseDontUse, ReturnValuesReuseDuringRequest, ReturnValuesReuseDuringSession:
		return true
	default:
		return false
	}
}

func DecodeCommonModule(source string, reader io.Reader, configuration project.Project) (CommonModuleDefinition, error) {
	var value CommonModuleDefinition
	if err := decodeStrict(source, reader, &value); err != nil {
		return CommonModuleDefinition{}, err
	}
	if err := ValidateCommonModule(source, value, configuration); err != nil {
		return CommonModuleDefinition{}, err
	}
	return value, nil
}

// ValidateCommonModule checks structure only; module source existence is
// checked catalog-wide by indexAndValidate (validateObjectSources).
func ValidateCommonModule(source string, value CommonModuleDefinition, configuration project.Project) error {
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, configuration)
	if value.Module.IsZero() {
		issues = append(issues, "module must be a non-zero UUID")
	}
	// Any one context is enough: the designer saves a module that is only
	// for the external connection, or only for the ordinary application
	// (checked by the owner, 01.10.2026).
	if !value.Client && !value.Server && !value.ExternalConnection && !value.ClientOrdinaryApplication {
		issues = append(issues, "at least one of client, server, external_connection or client_ordinary_application must be set")
	}
	if value.ServerCall && !value.Server {
		issues = append(issues, "server_call requires server")
	}
	if value.Privileged && !value.Server {
		issues = append(issues, "privileged requires server")
	}
	if !validReturnValuesReuse(value.ReturnValuesReuse) {
		issues = append(issues, "return_values_reuse must be dont-use, during-request or during-session")
	}
	return issuesError(source, value.Format, issues)
}

func (catalog *Catalog) CommonModule(name string) (CommonModuleDefinition, bool) {
	index, ok := catalog.commonModuleByName[strings.ToLower(name)]
	if !ok {
		return CommonModuleDefinition{}, false
	}
	return cloneCommonModuleDefinition(catalog.CommonModules[index]), true
}

func (catalog *Catalog) CommonModuleByID(id uuid.UUID) (CommonModuleDefinition, bool) {
	index, ok := catalog.commonModuleByID[id]
	if !ok {
		return CommonModuleDefinition{}, false
	}
	return cloneCommonModuleDefinition(catalog.CommonModules[index]), true
}

// CommonModuleByModuleID looks up a common module by its .bsl source UUID
// (the Module field), for callers that key compiled modules by source file.
func (catalog *Catalog) CommonModuleByModuleID(moduleID uuid.UUID) (CommonModuleDefinition, bool) {
	index, ok := catalog.commonModuleByModuleID[moduleID]
	if !ok {
		return CommonModuleDefinition{}, false
	}
	return cloneCommonModuleDefinition(catalog.CommonModules[index]), true
}

func cloneCommonModuleDefinition(value CommonModuleDefinition) CommonModuleDefinition {
	value.Title = cloneTitle(value.Title)
	return value
}

// DefaultContext maps a common module's flags to the compiler's per-module
// default execution context, so its routines need no per-routine directive to
// be classified correctly.
//
// The ordinary application's client is not a context of ML: beside any other
// flag it changes nothing, and a module of the server and the ordinary client
// stays a server module. A module of the ordinary client alone is compiled as
// a client module and called by nobody, exactly as the ordinary application
// module of the root is. A module of the external connection alone is a
// server module: the external connection runs on the server.
func (definition CommonModuleDefinition) DefaultContext() syntax.ExecutionContext {
	switch {
	case definition.Client && definition.Server:
		return syntax.ContextClientServer
	case definition.Client:
		return syntax.ContextClient
	case definition.Server, definition.ExternalConnection:
		return syntax.ContextServer
	case definition.ClientOrdinaryApplication:
		return syntax.ContextClient
	default:
		return syntax.ContextServer
	}
}
