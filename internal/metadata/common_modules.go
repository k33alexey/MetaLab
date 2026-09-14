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
}

func DecodeCommonModule(source string, reader io.Reader, manifest project.Project) (CommonModuleDefinition, error) {
	var value CommonModuleDefinition
	if err := decodeStrict(source, reader, &value); err != nil {
		return CommonModuleDefinition{}, err
	}
	if err := ValidateCommonModule(source, value, manifest); err != nil {
		return CommonModuleDefinition{}, err
	}
	return value, nil
}

// ValidateCommonModule checks structure only; module source existence is
// checked catalog-wide by indexAndValidate (validateObjectSources).
func ValidateCommonModule(source string, value CommonModuleDefinition, manifest project.Project) error {
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, manifest)
	if value.Module.IsZero() {
		issues = append(issues, "module must be a non-zero UUID")
	}
	if !value.Client && !value.Server {
		issues = append(issues, "at least one of client or server must be set")
	}
	if value.ServerCall && !value.Server {
		issues = append(issues, "server_call requires server")
	}
	if value.Privileged && !value.Server {
		issues = append(issues, "privileged requires server")
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

// DefaultContext maps a common module's Client/Server flags to the
// compiler's per-module default execution context, so its routines need no
// per-routine directive to be classified correctly.
func (definition CommonModuleDefinition) DefaultContext() syntax.ExecutionContext {
	switch {
	case definition.Client && definition.Server:
		return syntax.ContextClientServer
	case definition.Client:
		return syntax.ContextClient
	default:
		return syntax.ContextServer
	}
}
