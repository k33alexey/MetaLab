package metadata

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

const SubsystemKind Kind = "subsystems"

// SubsystemDefinition groups other metadata objects into one functional
// section of ML App navigation. Membership and nesting are purely
// navigational: they do not affect execution, rights or storage.
type SubsystemDefinition struct {
	Format int           `yaml:"format"`
	ID     uuid.UUID     `yaml:"id"`
	Name   string        `yaml:"name"`
	Title  LocalizedText `yaml:"title"`
	// A subsystem is offered to a user and has a help topic, but no commands of
	// its own to offer: the help gives it the explanation and the help flag and
	// no standard commands - see object_presentation.go.
	ObjectExplanation     `yaml:",inline"`
	IncludeHelpInContents bool        `yaml:"include_help_in_contents,omitempty"`
	Parent                *uuid.UUID  `yaml:"parent,omitempty"`
	Members               []uuid.UUID `yaml:"members,omitempty"`
	// IncludeInCommandInterface decides whether the subsystem becomes a section
	// of the command interface at all. Without it a subsystem groups objects for
	// the developer and shows the user nothing.
	IncludeInCommandInterface bool `yaml:"include_in_command_interface,omitempty"`
	// Picture is the icon of the section.
	Picture *PictureReference `yaml:"picture,omitempty"`
	// UseOneCommand puts the commands of the section behind one command instead
	// of listing them.
	UseOneCommand bool `yaml:"use_one_command,omitempty"`
	// CommandInterface is what the section shows and in what order - see
	// CommandInterface.
	CommandInterface CommandInterface `yaml:"command_interface,omitempty"`
}

func DecodeSubsystem(source string, reader io.Reader, configuration project.Project) (SubsystemDefinition, error) {
	var value SubsystemDefinition
	if err := decodeStrict(source, reader, &value); err != nil {
		return SubsystemDefinition{}, err
	}
	if err := ValidateSubsystem(source, value, configuration); err != nil {
		return SubsystemDefinition{}, err
	}
	return value, nil
}

// ValidateSubsystem checks structure only; membership and parent existence
// are checked catalog-wide by validateSubsystemReferences.
func ValidateSubsystem(source string, value SubsystemDefinition, configuration project.Project) error {
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, configuration)
	if len(value.Explanation) > 0 {
		issues = append(issues, validateTitle("explanation", value.Explanation, configuration)...)
	}
	issues = append(issues, validatePictureReference("picture", value.Picture)...)
	if value.Parent != nil && *value.Parent == value.ID {
		issues = append(issues, "parent must not reference the subsystem itself")
	}
	if value.Parent != nil && value.Parent.IsZero() {
		issues = append(issues, "parent must be a non-zero UUID")
	}
	issues = append(issues, validateCommandInterface("command_interface", value.CommandInterface)...)
	seen := make(map[uuid.UUID]bool, len(value.Members))
	for index, member := range value.Members {
		prefix := fmt.Sprintf("members[%d]", index)
		if member.IsZero() {
			issues = append(issues, prefix+" must be a non-zero UUID")
		}
		if seen[member] {
			issues = append(issues, prefix+" must be unique")
		}
		seen[member] = true
	}
	return issuesError(source, value.Format, issues)
}

// Subsystem returns one subsystem by its path, folded case: the name of a
// top-level subsystem, or the names from the top down joined by dots -
// "Администрирование.БазоваяФункциональность". A name alone does not find a
// nested one, because the same name may stand under several parents.
func (catalog *Catalog) Subsystem(name string) (SubsystemDefinition, bool) {
	index, ok := catalog.subsystemByName[strings.ToLower(name)]
	if !ok {
		return SubsystemDefinition{}, false
	}
	return cloneSubsystemDefinition(catalog.Subsystems[index]), true
}

func (catalog *Catalog) SubsystemByID(id uuid.UUID) (SubsystemDefinition, bool) {
	index, ok := catalog.subsystemByID[id]
	if !ok {
		return SubsystemDefinition{}, false
	}
	return cloneSubsystemDefinition(catalog.Subsystems[index]), true
}

func cloneSubsystemDefinition(value SubsystemDefinition) SubsystemDefinition {
	value.Title = cloneTitle(value.Title)
	if value.Parent != nil {
		parent := *value.Parent
		value.Parent = &parent
	}
	value.Members = slices.Clone(value.Members)
	value.Explanation = cloneTitle(value.Explanation)
	value.CommandInterface = cloneCommandInterface(value.CommandInterface)
	value.Picture = value.Picture.clone()
	return value
}

// validateSubsystemReferences checks that every parent resolves and that no
// subsystem is its own ancestor, builds the lookup by path, and notes the
// members that point at nothing.
//
// A member may be an object of any kind of the top level, another subsystem
// included - the prototype puts some forty kinds into a subsystem, common
// modules, roles and pictures the most. A member naming nothing is carried,
// not refused: the object was deleted and the subsystem kept it, which the
// prototype saves (70/74/189 in the configurations being moved, none of them
// resolving). It is listed for the import report - see UnresolvedReference.
func (catalog *Catalog) validateSubsystemReferences() error {
	for _, item := range catalog.Subsystems {
		if item.Parent != nil {
			if _, ok := catalog.subsystemByID[*item.Parent]; !ok {
				return fmt.Errorf("subsystem %s references unknown parent subsystem %s", item.Name, *item.Parent)
			}
		}
		for _, member := range item.Members {
			if !catalog.knownMetadataObject(member) {
				catalog.noteUnresolved("subsystem "+item.Name+" member", member)
			}
		}
	}
	for _, item := range catalog.Subsystems {
		visited := map[uuid.UUID]bool{item.ID: true}
		current := item.Parent
		for current != nil {
			if visited[*current] {
				return fmt.Errorf("subsystem %s has a cyclical parent chain", item.Name)
			}
			visited[*current] = true
			parent, ok := catalog.subsystemByID[*current]
			if !ok {
				break
			}
			current = catalog.Subsystems[parent].Parent
		}
	}
	catalog.subsystemByName = make(map[string]int, len(catalog.Subsystems))
	for index := range catalog.Subsystems {
		catalog.subsystemByName[strings.ToLower(catalog.subsystemPath(index))] = index
	}
	return nil
}

// subsystemPath is the names from the top down to one subsystem, joined by
// dots. The parent chain is known to be complete and acyclic when it is asked.
func (catalog *Catalog) subsystemPath(index int) string {
	path := catalog.Subsystems[index].Name
	for parent := catalog.Subsystems[index].Parent; parent != nil; {
		next := catalog.subsystemByID[*parent]
		path = catalog.Subsystems[next].Name + "." + path
		parent = catalog.Subsystems[next].Parent
	}
	return path
}

// knownMetadataObject reports whether id is an object of the top level the
// configuration carries, of any kind.
func (catalog *Catalog) knownMetadataObject(id uuid.UUID) bool {
	_, ok := catalog.objectKindByID[id]
	return ok
}
