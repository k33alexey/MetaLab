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
	return value
}

// validateSubsystemReferences checks that every parent and member reference
// resolves to a real object and that no subsystem is its own ancestor.
func (catalog *Catalog) validateSubsystemReferences() error {
	for _, item := range catalog.Subsystems {
		if item.Parent != nil {
			if _, ok := catalog.subsystemByID[*item.Parent]; !ok {
				return fmt.Errorf("subsystem %s references unknown parent subsystem %s", item.Name, *item.Parent)
			}
		}
		for _, member := range item.Members {
			if !catalog.knownMetadataObject(member) {
				return fmt.Errorf("subsystem %s references unknown object %s", item.Name, member)
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
	return nil
}

// knownMetadataObject reports whether id belongs to any currently supported
// metadata kind that a subsystem may list as a member.
func (catalog *Catalog) knownMetadataObject(id uuid.UUID) bool {
	if _, ok := catalog.catalogByID[id]; ok {
		return true
	}
	if _, ok := catalog.documentByID[id]; ok {
		return true
	}
	if _, ok := catalog.enumerationByID[id]; ok {
		return true
	}
	if _, ok := catalog.constantByID[id]; ok {
		return true
	}
	if _, ok := catalog.informationRegisterByID[id]; ok {
		return true
	}
	if _, ok := catalog.accumulationRegisterByID[id]; ok {
		return true
	}
	return false
}
