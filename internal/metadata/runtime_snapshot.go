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
	Constants             []Constant                       `json:"constants,omitempty"`
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
	validated, err := NewCatalogSnapshotWithAccumulationRegisters(
		catalog.Project, catalog.Constants, catalog.Enumerations, catalog.DefinedTypes,
		catalog.Catalogs, catalog.Documents, catalog.InformationRegisters, catalog.AccumulationRegisters,
	)
	if err != nil {
		return RuntimeSnapshot{}, err
	}
	result := RuntimeSnapshot{
		Format: CurrentFormat, Project: validated.Project,
		Constants: validated.Constants, Enumerations: validated.Enumerations, DefinedTypes: validated.DefinedTypes,
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
	if len(result.Constants) == 0 {
		result.Constants = nil
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
	return NewCatalogSnapshotWithAccumulationRegisters(
		snapshot.Project, snapshot.Constants, snapshot.Enumerations, snapshot.DefinedTypes,
		snapshot.Catalogs, snapshot.Documents, snapshot.InformationRegisters, snapshot.AccumulationRegisters,
	)
}

// Validate checks both the metadata catalog and every embedded managed form.
func (snapshot RuntimeSnapshot) Validate() error {
	catalog, err := snapshot.Catalog()
	if err != nil {
		return err
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
