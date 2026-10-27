package metadata

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/k33alexey/MetaLab/internal/kinds"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

const SubsystemKind = kinds.Subsystem

// SubsystemDefinition groups other metadata objects into one functional
// section of ML App navigation. Membership and nesting are purely
// navigational: they do not affect execution, rights or storage.
type SubsystemDefinition struct {
	Format int           `yaml:"format" json:"format"`
	ID     uuid.UUID     `yaml:"id" json:"id"`
	Name   string        `yaml:"name" json:"name"`
	Title  LocalizedText `yaml:"title" json:"title"`
	// A subsystem is offered to a user and has a help topic, but no commands of
	// its own to offer: the help gives it the explanation and the help flag and
	// no standard commands - see object_presentation.go.
	ObjectExplanation     `yaml:",inline"`
	IncludeHelpInContents bool        `yaml:"include_help_in_contents,omitempty" json:"includeHelpInContents,omitempty"`
	Parent                *uuid.UUID  `yaml:"parent,omitempty" json:"parent,omitempty"`
	Members               []uuid.UUID `yaml:"members,omitempty" json:"members,omitempty"`
	// IncludeInCommandInterface decides whether the subsystem becomes a section
	// of the command interface at all. Without it a subsystem groups objects for
	// the developer and shows the user nothing.
	IncludeInCommandInterface bool `yaml:"include_in_command_interface,omitempty" json:"includeInCommandInterface,omitempty"`
	// Picture is the icon of the section.
	Picture *PictureReference `yaml:"picture,omitempty" json:"picture,omitempty"`
	// UseOneCommand puts the commands of the section behind one command instead
	// of listing them.
	UseOneCommand bool `yaml:"use_one_command,omitempty" json:"useOneCommand,omitempty"`
	// CommandInterface is what the section shows and in what order - see
	// CommandInterface.
	CommandInterface CommandInterface `yaml:"command_interface,omitempty" json:"commandInterface,omitempty"`
}

func DecodeSubsystem(source string, reader io.Reader, configuration project.Project) (SubsystemDefinition, error) {
	return decodeDescription(source, reader, configuration, validateSubsystem)
}

// ValidateSubsystem checks structure only; membership and parent existence
// are checked catalog-wide by validateSubsystemReferences.
func ValidateSubsystem(source string, value SubsystemDefinition, configuration project.Project) error {
	return checkDescription(source, value, configuration, validateSubsystem)
}

// validateSubsystem checks what a subsystem adds to the head every description has.
func validateSubsystem(value SubsystemDefinition, configuration project.Project) []string {
	var issues []string
	issues = append(issues, validateTitle("explanation", value.Explanation, configuration)...)
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
	return issues
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
// A member may be an object of any kind the prototype puts into a subsystem
// (kindUses.subsystem), another subsystem included - some forty kinds, common
// modules, roles and pictures the most; a language and a WebSocket client are
// refused, as the page «Подсистемы» of theirs is empty. A member naming nothing is carried,
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
			kind, ok := catalog.kindOfObject(member)
			if !ok {
				catalog.noteUnresolved("subsystem "+item.Name+" member", member)
				continue
			}
			if usesOf(kind).subsystem == absent {
				return fmt.Errorf("subsystem %s holds %s: the designer puts no object of the kind into a subsystem",
					item.Name, catalog.noteObjectName(kind, member))
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

// knownMetadataObject reports whether id is an object the configuration
// carries, of any kind.
func (catalog *Catalog) knownMetadataObject(id uuid.UUID) bool {
	_, ok := catalog.objects[id]
	return ok
}

// SubsystemFilter is what the filter by subsystems selects by (help, «Отбор
// по подсистемам»), for the tree of the configuration: every subsystem with
// the one it is nested in, which the window of the filter shows as a tree,
// every object lying inside another with its owner, as such an object passes
// with the one it lies in, and what each subsystem holds. The windows of
// rights read the same out of PermissionSchema.
type SubsystemFilter struct {
	Objects    []SubsystemFilterObject   `json:"objects"`
	Subsystems map[uuid.UUID][]uuid.UUID `json:"subsystems"`
}

// SubsystemFilterObject is a subsystem, or an object inside another, as the
// filter by subsystems knows it.
type SubsystemFilterObject struct {
	ID    uuid.UUID     `json:"id"`
	Owner *uuid.UUID    `json:"owner,omitempty"`
	Name  string        `json:"name"`
	Title LocalizedText `json:"title"`
}

// SubsystemFilter answers the filter by subsystems of the catalog: the
// subsystems by name, then the objects inside others by identifier.
func (catalog *Catalog) SubsystemFilter() SubsystemFilter {
	result := SubsystemFilter{Objects: []SubsystemFilterObject{}, Subsystems: catalog.subsystemMembers()}
	for _, item := range catalog.objectsToShow(SubsystemKind, nil) {
		result.Objects = append(result.Objects, SubsystemFilterObject{ID: item.ID, Owner: catalog.objects[item.ID].owner, Name: item.Name, Title: cloneTitle(item.Title)})
	}
	var inside []SubsystemFilterObject
	for id, object := range catalog.objects {
		if object.owner != nil && object.kind != SubsystemKind {
			inside = append(inside, SubsystemFilterObject{ID: id, Owner: object.owner, Name: object.name, Title: cloneTitle(object.title)})
		}
	}
	slices.SortFunc(inside, func(a, b SubsystemFilterObject) int { return strings.Compare(a.ID.String(), b.ID.String()) })
	result.Objects = append(result.Objects, inside...)
	return result
}

// subsystemMembers are the objects every subsystem holds, as it lists them,
// by the subsystem. A subsystem holding nothing is there too, with an empty
// list, as its subsystems beneath may hold something; a member naming a
// deleted object is carried.
func (catalog *Catalog) subsystemMembers() map[uuid.UUID][]uuid.UUID {
	result := map[uuid.UUID][]uuid.UUID{}
	for _, subsystem := range catalog.Subsystems {
		result[subsystem.ID] = append([]uuid.UUID{}, subsystem.Members...)
	}
	return result
}

// subsystemCompositionLeftOut are the kinds a subsystem may hold that the
// page «Состав» of its window does not offer (docs/block-00/podsistema.md,
// «Окно»: no subsystems, WS references, external sources of data - with
// their tables, cubes and tables of dimensions - and sequences).
var subsystemCompositionLeftOut = []Kind{SubsystemKind, WSReferenceKind, ExternalDataSourceKind, ExternalDataSourceTableKind,
	ExternalCubeKind, ExternalDimensionTableKind, SequenceKind}

// SubsystemCompositionKinds are the kinds the page «Состав» of the window
// of a subsystem offers: those a subsystem may hold, less those its window
// leaves out - 38 of the inventory and the numerators inside «Документы».
func SubsystemCompositionKinds() []Kind {
	var offered []Kind
	for _, info := range kindInfos {
		if info.uses.subsystem == present && !slices.Contains(subsystemCompositionLeftOut, info.kind) {
			offered = append(offered, info.kind)
		}
	}
	return offered
}
