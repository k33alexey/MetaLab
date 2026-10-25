package metadata

import (
	"fmt"
	"sort"

	"github.com/k33alexey/MetaLab/internal/bsl/syntax"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// RuntimeSnapshot is the validated metadata subset persisted with a published
// version and consumed by ML Service without access to Studio project files.
type RuntimeSnapshot struct {
	Format                int                              `json:"format"`
	Project               project.Project                  `json:"project"`
	Roles                 []RoleDefinition                 `json:"roles,omitempty"`
	Subsystems            []SubsystemDefinition            `json:"subsystems,omitempty"`
	Constants             []Constant                       `json:"constants,omitempty"`
	SessionParameters     []SessionParameter               `json:"sessionParameters,omitempty"`
	CommonAttributes      []CommonAttributeDefinition      `json:"commonAttributes,omitempty"`
	CommonModules         []CommonModuleDefinition         `json:"commonModules,omitempty"`
	EventSubscriptions    []EventSubscriptionDefinition    `json:"eventSubscriptions,omitempty"`
	Enumerations          []Enumeration                    `json:"enumerations,omitempty"`
	DefinedTypes          []DefinedTypeObject              `json:"definedTypes,omitempty"`
	Catalogs              []CatalogDefinition              `json:"catalogs,omitempty"`
	Documents             []DocumentDefinition             `json:"documents,omitempty"`
	InformationRegisters  []InformationRegisterDefinition  `json:"informationRegisters,omitempty"`
	AccumulationRegisters []AccumulationRegisterDefinition `json:"accumulationRegisters,omitempty"`
	Forms                 []ManagedForm                    `json:"forms,omitempty"`
	// ObjectForms carries the forms each object keeps, resolved to their
	// identifiers. On disk an object's forms are the folders beside it, and a
	// slot names one of them; a published application has no folders, so what
	// the folders said has to travel with it or every custom form silently
	// becomes a generated one.
	ObjectForms []RuntimeObjectForm `json:"objectForms,omitempty"`
	// Modules holds every project BSL module's source text. ML Service
	// compiles BSL exclusively from here - it never reads project files
	// from disk, so it can run on a different machine than Studio.
	Modules []RuntimeModule `json:"modules,omitempty"`
	// Outside are the objects of the project a role gives rights on or a
	// document writes into whose kind the snapshot holds no description of -
	// a report, a chart, a register of calculation or accounting. Execution
	// does not know them yet: the rights and the movements are carried and not
	// executed, not refused (BLOCKS 2в, «Сохранить данные» принимает роль с
	// правами на любой вид). They were checked against their descriptions
	// when the project was read.
	Outside []RuntimeObject `json:"outside,omitempty"`
}

// RuntimeObject is an object of the project by its identifier and kind.
type RuntimeObject struct {
	ID   uuid.UUID `json:"id"`
	Kind Kind      `json:"kind"`
}

// outsideOf is the objects the roles and the documents of the catalog name,
// by their kind in the project: those of them the snapshot holds are its own,
// and the check of a role or a movement looks here only for what it does not
// find among them.
func outsideOf(catalog *Catalog) map[uuid.UUID]Kind {
	named := map[uuid.UUID]Kind{}
	name := func(id uuid.UUID) {
		if kind, ok := catalog.kindOfObject(id); ok {
			named[id] = kind
		}
	}
	for _, role := range catalog.Roles {
		for _, permission := range role.Objects {
			name(permission.Object)
		}
	}
	for _, document := range catalog.Documents {
		for _, id := range document.Movements {
			name(id)
		}
	}
	return named
}

// outsideMap is the snapshot's Outside as catalogOf takes it.
func (snapshot RuntimeSnapshot) outsideMap() map[uuid.UUID]Kind {
	result := make(map[uuid.UUID]Kind, len(snapshot.Outside))
	for _, object := range snapshot.Outside {
		result[object.ID] = object.Kind
	}
	return result
}

// RuntimeObjectForm is one form an object keeps, named as its folder was and
// carrying the identifier that form declared inside itself.
type RuntimeObjectForm struct {
	ObjectKind Kind      `json:"objectKind"`
	Object     string    `json:"object"`
	Name       string    `json:"name"`
	ID         uuid.UUID `json:"id"`
}

// RuntimeModule is one compilable BSL module, persisted as source text
// (not bytecode) so it survives compiler version changes.
type RuntimeModule struct {
	Name                string                  `json:"name"`
	Filename            string                  `json:"filename"`
	Source              string                  `json:"source"`
	PredefinedVariables []string                `json:"predefinedVariables,omitempty"`
	ContextMethods      []string                `json:"contextMethods,omitempty"`
	DefaultContext      syntax.ExecutionContext `json:"defaultContext,omitempty"`
}

// NewRuntimeSnapshot creates an isolated, deterministic publication snapshot.
func NewRuntimeSnapshot(catalog *Catalog, forms []ManagedForm) (RuntimeSnapshot, error) {
	if catalog == nil {
		return RuntimeSnapshot{}, fmt.Errorf("runtime metadata catalog is required")
	}
	named := outsideOf(catalog)
	validated, err := RuntimeSnapshot{Project: catalog.Project, Constants: catalog.Constants, Enumerations: catalog.Enumerations,
		DefinedTypes: catalog.DefinedTypes, Catalogs: catalog.Catalogs, Documents: catalog.Documents,
		InformationRegisters: catalog.InformationRegisters, AccumulationRegisters: catalog.AccumulationRegisters,
		Roles: catalog.Roles, Subsystems: catalog.Subsystems, SessionParameters: catalog.SessionParameters,
		CommonAttributes: catalog.CommonAttributes, CommonModules: catalog.CommonModules,
		EventSubscriptions: catalog.EventSubscriptions}.catalogOf(named)
	if err != nil {
		return RuntimeSnapshot{}, err
	}
	result := RuntimeSnapshot{
		Format: CurrentFormat, Project: validated.Project,
		Roles:      validated.Roles,
		Subsystems: validated.Subsystems,
		Constants:  validated.Constants, SessionParameters: validated.SessionParameters, CommonAttributes: validated.CommonAttributes, CommonModules: validated.CommonModules, EventSubscriptions: validated.EventSubscriptions, Enumerations: validated.Enumerations, DefinedTypes: validated.DefinedTypes,
		Catalogs: validated.Catalogs, Documents: validated.Documents,
		InformationRegisters: validated.InformationRegisters, AccumulationRegisters: validated.AccumulationRegisters,
		Forms: make([]ManagedForm, len(forms)), ObjectForms: catalog.ObjectForms(),
	}
	seen := make(map[uuid.UUID]bool, len(forms))
	for index, form := range forms {
		if seen[form.ID] {
			return RuntimeSnapshot{}, fmt.Errorf("duplicate runtime form UUID %s", form.ID)
		}
		if err := ValidateManagedForm("runtime form "+form.Name, form, validated.Project); err != nil {
			return RuntimeSnapshot{}, err
		}
		seen[form.ID] = true
		result.Forms[index] = cloneRuntimeForm(form)
	}
	for id, kind := range named {
		if _, own := validated.kindOfObject(id); !own {
			result.Outside = append(result.Outside, RuntimeObject{ID: id, Kind: kind})
		}
	}
	sort.Slice(result.Outside, func(left, right int) bool {
		return result.Outside[left].ID.String() < result.Outside[right].ID.String()
	})
	if len(result.Subsystems) == 0 {
		result.Subsystems = nil
	}
	if len(result.Constants) == 0 {
		result.Constants = nil
	}
	if len(result.SessionParameters) == 0 {
		result.SessionParameters = nil
	}
	if len(result.CommonAttributes) == 0 {
		result.CommonAttributes = nil
	}
	if len(result.CommonModules) == 0 {
		result.CommonModules = nil
	}
	if len(result.EventSubscriptions) == 0 {
		result.EventSubscriptions = nil
	}
	if len(result.Enumerations) == 0 {
		result.Enumerations = nil
	}
	if len(result.DefinedTypes) == 0 {
		result.DefinedTypes = nil
	}
	if len(result.Catalogs) == 0 {
		result.Catalogs = nil
	}
	if len(result.Documents) == 0 {
		result.Documents = nil
	}
	if len(result.InformationRegisters) == 0 {
		result.InformationRegisters = nil
	}
	if len(result.AccumulationRegisters) == 0 {
		result.AccumulationRegisters = nil
	}
	if len(result.Forms) == 0 {
		result.Forms = nil
	}
	sort.Slice(result.Forms, func(left, right int) bool { return result.Forms[left].ID.String() < result.Forms[right].ID.String() })
	return result, nil
}

// WithModules attaches BSL module source text to an already-built snapshot.
// Kept as a separate step from NewRuntimeSnapshot so callers that have no
// BSL to compile yet (most existing tests, the retiring package pipeline)
// are unaffected.
func (snapshot RuntimeSnapshot) WithModules(modules []RuntimeModule) (RuntimeSnapshot, error) {
	seen := make(map[string]bool, len(modules))
	for _, module := range modules {
		if module.Name == "" || module.Filename == "" {
			return RuntimeSnapshot{}, fmt.Errorf("runtime module requires a name and filename")
		}
		if seen[module.Name] {
			return RuntimeSnapshot{}, fmt.Errorf("duplicate runtime module name %q", module.Name)
		}
		seen[module.Name] = true
	}
	snapshot.Modules = append([]RuntimeModule(nil), modules...)
	if len(snapshot.Modules) == 0 {
		snapshot.Modules = nil
	}
	sort.Slice(snapshot.Modules, func(left, right int) bool { return snapshot.Modules[left].Name < snapshot.Modules[right].Name })
	return snapshot, nil
}

// Catalog reconstructs indexed immutable metadata from a published snapshot.
func (snapshot RuntimeSnapshot) Catalog() (*Catalog, error) {
	if snapshot.Format != CurrentFormat {
		return nil, fmt.Errorf("unsupported runtime metadata format %d", snapshot.Format)
	}
	catalog, err := snapshot.catalogOf(snapshot.outsideMap())
	if err != nil {
		return nil, err
	}
	if err := snapshot.validateFormIDs(); err != nil {
		return nil, err
	}
	catalog.indexRuntimeObjectForms(snapshot.ObjectForms)
	catalog.keepRuntimeFormAttributeNames(snapshot.Forms, snapshot.ObjectForms)
	return catalog, nil
}

// Validate checks both the metadata catalog and every embedded managed form.
func (snapshot RuntimeSnapshot) Validate() error {
	catalog, err := snapshot.Catalog()
	if err != nil {
		return err
	}
	for index := 1; index < len(snapshot.Roles); index++ {
		if snapshot.Roles[index-1].ID.String() >= snapshot.Roles[index].ID.String() {
			return fmt.Errorf("runtime roles must have unique UUIDs in stable order")
		}
	}
	for index, module := range snapshot.Modules {
		if module.Name == "" || module.Filename == "" {
			return fmt.Errorf("runtime module requires a name and filename")
		}
		if index > 0 && snapshot.Modules[index-1].Name >= module.Name {
			return fmt.Errorf("runtime modules must have unique names in stable order")
		}
	}
	var previous string
	seen := make(map[uuid.UUID]bool, len(snapshot.Forms))
	for _, form := range snapshot.Forms {
		current := form.ID.String()
		if seen[form.ID] || previous != "" && current <= previous {
			return fmt.Errorf("runtime forms must have unique UUIDs in stable order")
		}
		if err := ValidateManagedForm("runtime form "+form.Name, form, catalog.Project); err != nil {
			return err
		}
		seen[form.ID], previous = true, current
	}
	return nil
}

// validateFormIDs refuses two forms of the configuration a database holds
// under one identifier: Form finds a form by it.
func (snapshot RuntimeSnapshot) validateFormIDs() error {
	forms := make(map[uuid.UUID]bool, len(snapshot.Forms))
	for _, form := range snapshot.Forms {
		if forms[form.ID] {
			return fmt.Errorf("duplicate runtime form UUID %s", form.ID)
		}
		forms[form.ID] = true
	}
	return nil
}

// Form returns an isolated managed form by stable UUID.
func (snapshot RuntimeSnapshot) Form(id uuid.UUID) (ManagedForm, bool) {
	index := sort.Search(len(snapshot.Forms), func(index int) bool { return snapshot.Forms[index].ID.String() >= id.String() })
	if index >= len(snapshot.Forms) || snapshot.Forms[index].ID != id {
		return ManagedForm{}, false
	}
	return cloneRuntimeForm(snapshot.Forms[index]), true
}
