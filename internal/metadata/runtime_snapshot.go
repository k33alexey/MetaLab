package metadata

import (
	"fmt"
	"sort"

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
}

// NewRuntimeSnapshot creates an isolated, deterministic publication snapshot.
func NewRuntimeSnapshot(catalog *Catalog, forms []ManagedForm) (RuntimeSnapshot, error) {
	if catalog == nil {
		return RuntimeSnapshot{}, fmt.Errorf("runtime metadata catalog is required")
	}
	validated, err := NewCatalogSnapshotWithEventSubscriptions(
		catalog.Project, catalog.Constants, catalog.Enumerations, catalog.DefinedTypes,
		catalog.Catalogs, catalog.Documents, catalog.InformationRegisters, catalog.AccumulationRegisters, catalog.Roles, catalog.Subsystems, catalog.SessionParameters, catalog.CommonAttributes, catalog.CommonModules, catalog.EventSubscriptions,
	)
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
		Forms: make([]ManagedForm, len(forms)),
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
	if err := result.validateRoleCommands(); err != nil {
		return RuntimeSnapshot{}, err
	}
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

// Catalog reconstructs indexed immutable metadata from a published snapshot.
func (snapshot RuntimeSnapshot) Catalog() (*Catalog, error) {
	if snapshot.Format != CurrentFormat {
		return nil, fmt.Errorf("unsupported runtime metadata format %d", snapshot.Format)
	}
	catalog, err := NewCatalogSnapshotWithEventSubscriptions(
		snapshot.Project, snapshot.Constants, snapshot.Enumerations, snapshot.DefinedTypes,
		snapshot.Catalogs, snapshot.Documents, snapshot.InformationRegisters, snapshot.AccumulationRegisters, snapshot.Roles, snapshot.Subsystems, snapshot.SessionParameters, snapshot.CommonAttributes, snapshot.CommonModules, snapshot.EventSubscriptions,
	)
	if err != nil {
		return nil, err
	}
	if err := snapshot.validateRoleCommands(); err != nil {
		return nil, err
	}
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

func (snapshot RuntimeSnapshot) validateRoleCommands() error {
	forms := make(map[uuid.UUID]int, len(snapshot.Forms))
	for index, form := range snapshot.Forms {
		if _, exists := forms[form.ID]; exists {
			return fmt.Errorf("duplicate runtime form UUID %s", form.ID)
		}
		forms[form.ID] = index
	}
	return validateRoleCommands(snapshot.Roles, func(id uuid.UUID) (ManagedForm, error) {
		index, ok := forms[id]
		if !ok {
			return ManagedForm{}, fmt.Errorf("unknown runtime form %s", id)
		}
		return snapshot.Forms[index], nil
	})
}

// Form returns an isolated managed form by stable UUID.
func (snapshot RuntimeSnapshot) Form(id uuid.UUID) (ManagedForm, bool) {
	index := sort.Search(len(snapshot.Forms), func(index int) bool { return snapshot.Forms[index].ID.String() >= id.String() })
	if index >= len(snapshot.Forms) || snapshot.Forms[index].ID != id {
		return ManagedForm{}, false
	}
	return cloneRuntimeForm(snapshot.Forms[index]), true
}

func cloneRuntimeForm(value ManagedForm) ManagedForm {
	result := value
	result.Title = cloneTitle(value.Title)
	result.Commands = make([]ManagedFormCommand, len(value.Commands))
	for index, command := range value.Commands {
		result.Commands[index] = command
		result.Commands[index].Title = cloneTitle(command.Title)
	}
	result.Items = cloneRuntimeFormElements(value.Items)
	return result
}

func cloneRuntimeFormElements(values []ManagedFormElement) []ManagedFormElement {
	result := make([]ManagedFormElement, len(values))
	for index, value := range values {
		result[index] = value
		result[index].Title = cloneTitle(value.Title)
		if value.Command != nil {
			command := *value.Command
			result[index].Command = &command
		}
		result[index].Children = cloneRuntimeFormElements(value.Children)
	}
	return result
}
