package metadata

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// WebServiceKind holds the services this configuration offers to the outside
// by a description of types: what can be called, what it takes and what it
// answers.
const WebServiceKind Kind = "web-services"

const (
	maxOperationsPerService   = 512
	maxParametersPerOperation = 64
)

// SessionReuseMode says whether calls into a service share one session.
//
// Reusing a session keeps what the previous call left - session parameters,
// caches, an open transaction's aftermath - and that is either the point of the
// service or its worst surprise, depending on the service. The prototype offers
// three answers and so do we: the choice belongs to the configuration.
type SessionReuseMode string

const (
	DoNotReuseSession     SessionReuseMode = "do-not-use"
	ReuseSession          SessionReuseMode = "use"
	ReuseSessionAutomatic SessionReuseMode = "auto"
)

// TransferDirection is which way one parameter of an operation travels.
type TransferDirection string

const (
	TransferIn    TransferDirection = "in"
	TransferOut   TransferDirection = "out"
	TransferInOut TransferDirection = "in-out"
)

// WebServiceParameter is one value an operation takes or gives back.
type WebServiceParameter struct {
	ID      uuid.UUID     `yaml:"id" json:"id"`
	Name    string        `yaml:"name" json:"name"`
	Title   LocalizedText `yaml:"title" json:"title"`
	Comment string        `yaml:"comment,omitempty" json:"comment,omitempty"`
	// Type is the type of the value, named the way the schema names it - with
	// the prefix of the namespace it comes from, as in xs:string. It is not one
	// of our own types: what travels between two systems is described by the
	// schema they agreed on, not by the model of either of them.
	Type string `yaml:"type" json:"type"`
	// Nillable allows the value to arrive empty. Empty is not the same as
	// absent, and the difference is the caller's to state.
	Nillable  bool              `yaml:"nillable,omitempty" json:"nillable,omitempty"`
	Direction TransferDirection `yaml:"direction,omitempty" json:"direction,omitempty"`
}

// WebServiceOperation is one thing the service can be asked to do.
type WebServiceOperation struct {
	ID      uuid.UUID     `yaml:"id" json:"id"`
	Name    string        `yaml:"name" json:"name"`
	Title   LocalizedText `yaml:"title" json:"title"`
	Comment string        `yaml:"comment,omitempty" json:"comment,omitempty"`
	// Procedure is what runs when the operation is called: a procedure of the
	// service's own module. The module lies in the service's folder under the
	// name of its role, so nothing here names the module - only the routine.
	Procedure string `yaml:"procedure" json:"procedure"`
	// ReturnType is what the operation answers with, named by the schema.
	ReturnType string `yaml:"return_type,omitempty" json:"returnType,omitempty"`
	Nillable   bool   `yaml:"nillable,omitempty" json:"nillable,omitempty"`
	// Transactioned runs the whole operation in one transaction: either
	// everything it did stays, or nothing does.
	Transactioned   bool                        `yaml:"transactioned,omitempty" json:"transactioned,omitempty"`
	DataLockControl project.DataLockControlMode `yaml:"data_lock_control,omitempty" json:"dataLockControl,omitempty"`
	Parameters      []WebServiceParameter       `yaml:"parameters,omitempty" json:"parameters,omitempty"`
}

// WebServiceDefinition is one service offered to the outside.
type WebServiceDefinition struct {
	Format  int           `yaml:"format" json:"format"`
	ID      uuid.UUID     `yaml:"id" json:"id"`
	Name    string        `yaml:"name" json:"name"`
	Title   LocalizedText `yaml:"title" json:"title"`
	Comment string        `yaml:"comment,omitempty" json:"comment,omitempty"`
	// Namespace is what the outside names this service's types by.
	Namespace string `yaml:"namespace" json:"namespace"`
	// Packages are the packages of exchanged types the service is described
	// by. They are named by identifier, like every reference between objects.
	Packages []uuid.UUID `yaml:"packages,omitempty" json:"packages,omitempty"`
	// DescriptorFile is the name the description of this service is published
	// under. It is a file name and not a path: where the file is published is
	// the business of the server, and a path here would be a decision taken in
	// the wrong place.
	DescriptorFile string           `yaml:"descriptor_file,omitempty" json:"descriptorFile,omitempty"`
	ReuseSessions  SessionReuseMode `yaml:"reuse_sessions,omitempty" json:"reuseSessions,omitempty"`
	// SessionMaxAge is how long a reused session is kept, in seconds. Zero
	// means the platform decides.
	SessionMaxAge int                   `yaml:"session_max_age,omitempty" json:"sessionMaxAge,omitempty"`
	Operations    []WebServiceOperation `yaml:"operations,omitempty" json:"operations,omitempty"`
}

// DecodeWebService reads and validates one web service.
func DecodeWebService(source string, reader io.Reader, configuration project.Project) (WebServiceDefinition, error) {
	var value WebServiceDefinition
	if err := decodeStrict(source, reader, &value); err != nil {
		return WebServiceDefinition{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, configuration)
	issues = append(issues, validateNamespace("namespace", value.Namespace, true)...)
	issues = append(issues, validateDescriptorFile("descriptor_file", value.DescriptorFile)...)
	switch value.ReuseSessions {
	case "", DoNotReuseSession, ReuseSession, ReuseSessionAutomatic:
	default:
		issues = append(issues, "reuse_sessions must be do-not-use, use or auto")
	}
	if value.SessionMaxAge < 0 {
		issues = append(issues, "session_max_age must not be negative")
	}
	seenPackages := map[uuid.UUID]bool{}
	for index, id := range value.Packages {
		where := fmt.Sprintf("packages[%d]", index)
		if id.IsZero() {
			issues = append(issues, where+" must be a non-zero UUID")
			continue
		}
		// A package named twice describes the service no better than once.
		if seenPackages[id] {
			issues = append(issues, where+" is named twice")
		}
		seenPackages[id] = true
	}
	issues = append(issues, validateOperations(value.Operations, configuration)...)
	if err := issuesError(source, value.Format, issues); err != nil {
		return WebServiceDefinition{}, err
	}
	return value, nil
}

// validateOperations checks what the service offers: each operation, each of
// its parameters, and that no two of either share a name or an identity.
func validateOperations(operations []WebServiceOperation, configuration project.Project) []string {
	if len(operations) > maxOperationsPerService {
		return []string{fmt.Sprintf("operations must not contain more than %d items", maxOperationsPerService)}
	}
	var issues []string
	names, ids := map[string]bool{}, map[uuid.UUID]bool{}
	for index, operation := range operations {
		where := fmt.Sprintf("operations[%d]", index)
		if operation.ID.IsZero() {
			issues = append(issues, where+".id must be a non-zero UUID")
		} else if ids[operation.ID] {
			issues = append(issues, where+".id is used twice")
		}
		ids[operation.ID] = true
		if !validIdentifier(operation.Name) || utf8.RuneCountInString(operation.Name) > 128 {
			issues = append(issues, where+".name must start with a letter and contain only letters or digits")
		} else if names[strings.ToLower(operation.Name)] {
			issues = append(issues, where+".name is used twice")
		}
		names[strings.ToLower(operation.Name)] = true
		issues = append(issues, validateTitle(where+".title", operation.Title, configuration)...)
		// An operation with no routine behind it answers a caller with
		// nothing, and does so only once the caller has already called.
		if !validIdentifier(operation.Procedure) || utf8.RuneCountInString(operation.Procedure) > 128 {
			issues = append(issues, where+".procedure must name a procedure of the service module")
		}
		issues = append(issues, validateSchemaType(where+".return_type", operation.ReturnType, false)...)
		// An operation that answers with nothing cannot answer with an empty
		// something: there is no value for "nillable" to be about.
		if operation.Nillable && operation.ReturnType == "" {
			issues = append(issues, where+".nillable says the answer may be empty, and the operation answers with nothing")
		}
		switch operation.DataLockControl {
		case "", project.AutomaticDataLock, project.ManagedDataLock, project.AutomaticAndManagedDataLock:
		default:
			issues = append(issues, where+".data_lock_control must be automatic, managed or automatic-and-managed")
		}
		issues = append(issues, validateParameters(where, operation.Parameters, configuration)...)
	}
	return issues
}

func validateParameters(owner string, parameters []WebServiceParameter, configuration project.Project) []string {
	if len(parameters) > maxParametersPerOperation {
		return []string{fmt.Sprintf("%s.parameters must not contain more than %d items", owner, maxParametersPerOperation)}
	}
	var issues []string
	names, ids := map[string]bool{}, map[uuid.UUID]bool{}
	for index, parameter := range parameters {
		where := fmt.Sprintf("%s.parameters[%d]", owner, index)
		if parameter.ID.IsZero() {
			issues = append(issues, where+".id must be a non-zero UUID")
		} else if ids[parameter.ID] {
			issues = append(issues, where+".id is used twice")
		}
		ids[parameter.ID] = true
		if !validIdentifier(parameter.Name) || utf8.RuneCountInString(parameter.Name) > 128 {
			issues = append(issues, where+".name must start with a letter and contain only letters or digits")
		} else if names[strings.ToLower(parameter.Name)] {
			issues = append(issues, where+".name is used twice")
		}
		names[strings.ToLower(parameter.Name)] = true
		issues = append(issues, validateTitle(where+".title", parameter.Title, configuration)...)
		issues = append(issues, validateSchemaType(where+".type", parameter.Type, true)...)
		switch parameter.Direction {
		case "", TransferIn, TransferOut, TransferInOut:
		default:
			issues = append(issues, where+".direction must be in, out or in-out")
		}
	}
	return issues
}

// validateSchemaType checks a type named by the schema: one word, possibly
// prefixed by the namespace it comes from, as in xs:string.
//
// What the word means is the schema's business, not ours. The types travelling
// between two systems are described by the schema they agreed on, and a check
// against our own type system would refuse every type we have not heard of -
// which is most of them, by design.
func validateSchemaType(path, value string, required bool) []string {
	if value == "" {
		if required {
			return []string{path + " must name the type of the value"}
		}
		return nil
	}
	return validateNamespace(path, value, required)
}

// validateDescriptorFile checks the name the description is published under: a
// file name, not a path. Where the file lies is the server's business, and a
// path written here would be a decision taken in the wrong place.
func validateDescriptorFile(path, value string) []string {
	if value == "" {
		return nil
	}
	if strings.ContainsAny(value, `/\`) {
		return []string{path + " must be a file name and not a path"}
	}
	return validateNamespace(path, value, false)
}

func cloneWebService(value WebServiceDefinition) WebServiceDefinition {
	value.Title = cloneTitle(value.Title)
	value.Packages = append([]uuid.UUID(nil), value.Packages...)
	operations := make([]WebServiceOperation, len(value.Operations))
	for index, operation := range value.Operations {
		operation.Title = cloneTitle(operation.Title)
		parameters := make([]WebServiceParameter, len(operation.Parameters))
		for position, parameter := range operation.Parameters {
			parameter.Title = cloneTitle(parameter.Title)
			parameters[position] = parameter
		}
		operation.Parameters = parameters
		operations[index] = operation
	}
	value.Operations = operations
	return value
}

// WebService returns one service by name, folded case.
func (catalog *Catalog) WebService(name string) (WebServiceDefinition, bool) {
	index, ok := catalog.webServiceByName[strings.ToLower(name)]
	if !ok {
		return WebServiceDefinition{}, false
	}
	return cloneWebService(catalog.WebServices[index]), true
}

// validateWebServices resolves what a service cannot resolve about itself: the
// packages it is described by.
//
// A service describing itself by a package that is not there describes itself
// by nothing, and the caller finds out at the moment of calling - which is the
// moment when the other side is already waiting.
func (catalog *Catalog) validateWebServices() error {
	for _, service := range catalog.WebServices {
		for _, id := range service.Packages {
			if _, ok := catalog.xdtoPackageByID[id]; !ok {
				return fmt.Errorf("web service %s is described by XDTO package %s, which is not in the configuration",
					service.Name, id)
			}
		}
	}
	return nil
}
