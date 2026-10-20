package metadata

import (
	"maps"
	"slices"
	"strings"
	"unicode"

	"github.com/k33alexey/MetaLab/internal/kinds"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// PermissionSchema is editor data, not an authenticated runtime catalog.
type PermissionSchema struct {
	// Rights names every right a role can carry, in the prototype's order,
	// as the configurator writes them: the one place the editors take their
	// captions from.
	Rights  []PermissionRight  `json:"rights"`
	Objects []PermissionObject `json:"objects"`
	// Parts is everything a role may give rights on - the configuration, by
	// the project's identifier, and every object of it - with the keys of
	// its parts. The editor shows fewer objects than this; what it does not
	// show it must neither lose nor take for something deleted.
	Parts map[uuid.UUID][]string `json:"parts"`
	// PartGroups are the folders of parts in the order of the tree, with
	// their captions.
	PartGroups []PermissionPartGroup `json:"partGroups"`
	// Subsystems are the objects every subsystem holds, as it lists them, by
	// the subsystem: the filter by subsystems of the windows of rights
	// selects by them (help, «Отбор по подсистемам»; subsystemMembers).
	Subsystems map[uuid.UUID][]uuid.UUID `json:"subsystems"`
	// DroppedRights are the rights ML no longer keeps with the one standing
	// for each, "" for none: a role saved before keeps them until
	// «Убрать недоступные права» puts these in their place.
	DroppedRights map[PermissionOperation]PermissionOperation `json:"droppedRights"`
	// RestrictionRules are how each right a restriction may be on is
	// restricted: the editor offers by them what saving keeps.
	RestrictionRules []RestrictionRule `json:"restrictionRules"`
}

// PermissionRight is a right as the window of a role lists it: its caption,
// and, for the right chosen in the list, its description with the name of
// the right in the language under it.
type PermissionRight struct {
	Key         PermissionOperation `json:"key"`
	Title       string              `json:"title"`
	Description string              `json:"description"`
	Name        string              `json:"name"`
}

// nameInWords turns an identifier into words, the way the configurator
// shows a name it has no synonym for: ПометкаУдаления as Пометка удаления.
func nameInWords(name string) string {
	words := []rune(identifierWords(name))
	if len(words) > 0 {
		words[0] = unicode.ToUpper(words[0])
	}
	return string(words)
}

// PermissionObject is one object the editor gives rights on: the
// configuration itself or an object of it.
type PermissionObject struct {
	ID   uuid.UUID `json:"id"`
	Kind Kind      `json:"kind"`
	// Owner is the object this one lies inside and is shown under: the
	// subsystem of a nested one, the register of a recalculation, the source
	// of a table or a cube, the cube of a dimension table.
	Owner *uuid.UUID `json:"owner,omitempty"`
	// Root is the configuration itself, known by the project's identifier.
	Root  bool          `json:"root,omitempty"`
	Name  string        `json:"name"`
	Title LocalizedText `json:"title"`
	// Rights are the rights the configurator offers on the object, in the
	// prototype's order: what the editor shows.
	Rights []PermissionOperation `json:"rights"`
	// Operations are those of them this platform acts on.
	Operations []PermissionOperation `json:"operations"`
	Fields     []PermissionField     `json:"fields"`
	// RestrictionFields are the fields a restriction of reading may name
	// (restrictionFieldsOf). They are not the parts rights are given on: a
	// restriction names the version of the data and the line number of a
	// table part too, which have no rights of their own.
	RestrictionFields []PermissionField `json:"restrictionFields"`
	// RestrictedRights are those of the rights a restriction of access to
	// data may be put on: none on an object that keeps no data.
	RestrictedRights []PermissionOperation `json:"restrictedRights"`
}

// PermissionField is one part of an object: under Fields, with the folder it
// is shown in and the rights the configurator offers on it; under
// RestrictionFields, a field a restriction names.
type PermissionField struct {
	Key        string                `json:"key"`
	Name       string                `json:"name"`
	Title      LocalizedText         `json:"title,omitempty"`
	Parent     string                `json:"parent,omitempty"`
	Group      PartGroup             `json:"group,omitempty"`
	Operations []PermissionOperation `json:"operations"`
}

// PermissionPartGroup is a folder of parts, by its caption and the icon the
// folder and its parts are drawn with, as in the configuration tree.
type PermissionPartGroup struct {
	Key   PartGroup `json:"key"`
	Title string    `json:"title"`
	Icon  string    `json:"icon"`
}

// LoadPermissionSchema deliberately ignores existing role references so a
// developer can repair them visually. It never returns executable metadata.
func LoadPermissionSchema(root string) (PermissionSchema, error) {
	catalog, err := load(root, false)
	if err != nil {
		return PermissionSchema{}, err
	}
	result := PermissionSchema{Objects: []PermissionObject{}, RestrictionRules: slices.Clone(restrictionRules), DroppedRights: DroppedRights()}
	for _, right := range roleRights {
		result.Rights = append(result.Rights, PermissionRight{Key: right.operation, Title: right.title, Description: right.description, Name: right.russian})
	}
	result.Parts = map[uuid.UUID][]string{}
	for _, id := range append(slices.Collect(maps.Keys(catalog.objects)), catalog.Project.ID) {
		if target, ok := catalog.rightsTargetOf(id); ok {
			keys := slices.Sorted(maps.Keys(target.parts))
			if keys == nil {
				keys = []string{}
			}
			result.Parts[id] = keys
		}
	}
	// A configuration without common forms has no directory for them, the
	// same as any other metadata kind nobody has used yet.
	forms, err := ReadCommonForms(root, catalog.Project)
	if err != nil {
		return PermissionSchema{}, err
	}
	appendObject := func(kind Kind, id uuid.UUID, name string, title LocalizedText) {
		offered, _ := catalog.rightsTargetOf(id)
		object := PermissionObject{ID: id, Kind: kind, Owner: catalog.objects[id].owner, Name: name, Title: cloneTitle(title),
			Rights: offered.rights, Operations: []PermissionOperation{}, Fields: []PermissionField{},
			RestrictionFields: []PermissionField{}, RestrictedRights: []PermissionOperation{}}
		for _, operation := range objectOperations {
			if slices.Contains(offered.rights, operation) {
				object.Operations = append(object.Operations, operation)
			}
		}
		if usesOf(kind).restriction.accepted() {
			for _, rule := range restrictionRules {
				if slices.Contains(offered.rights, rule.Right) {
					object.RestrictedRights = append(object.RestrictedRights, rule.Right)
				}
			}
			for _, field := range catalog.restrictionFieldsOf(kind, id) {
				object.RestrictionFields = append(object.RestrictionFields, PermissionField{Key: field.key, Name: field.name,
					Title: cloneTitle(field.title), Parent: field.parent, Operations: []PermissionOperation{}})
			}
		}
		// Every part of the object, from the one walk of its composition, with
		// the rights the configurator offers on a part of its sort in an
		// object of the kind.
		parts, _ := catalog.partsOf(kind, id)
		for _, part := range parts {
			rights := slices.Clone(offeredPartRights(kind, part.group))
			if rights == nil {
				rights = []PermissionOperation{}
			}
			field := PermissionField{Key: part.key, Name: part.name, Title: cloneTitle(part.title), Parent: part.parent,
				Group: part.group, Operations: rights}
			object.Fields = append(object.Fields, field)
		}
		result.Objects = append(result.Objects, object)
	}
	// The configuration itself comes first, as the root of the tree.
	configuration, _ := catalog.rightsTargetOf(catalog.Project.ID)
	result.Objects = append(result.Objects, PermissionObject{ID: catalog.Project.ID, Root: true,
		Name: catalog.Project.Name, Title: cloneTitle(catalog.Project.Title), Rights: configuration.rights,
		Operations: []PermissionOperation{}, Fields: []PermissionField{}, RestrictionFields: []PermissionField{},
		RestrictedRights: []PermissionOperation{}})
	// Then every object a role is offered rights on, kind by kind in the
	// order of the tree. A common form is read apart from the catalog.
	formTitles := map[uuid.UUID]LocalizedText{}
	for _, form := range forms {
		formTitles[form.ID] = form.Title
	}
	// The objects that lie inside others - a recalculation, a table, a cube
	// and a table of its dimension - come after them, each naming its owner.
	shown := kinds.Branches()
	for _, descriptor := range kinds.All() {
		if descriptor.Layout().Owner != "" {
			shown = append(shown, descriptor)
		}
	}
	for _, descriptor := range shown {
		if usesOf(descriptor.Kind).offered == nil {
			continue
		}
		for _, item := range catalog.objectsToShow(descriptor.Kind, formTitles) {
			appendObject(descriptor.Kind, item.ID, item.Name, item.Title)
		}
	}
	result.Subsystems = catalog.subsystemMembers()
	for _, item := range partGroups {
		result.PartGroups = append(result.PartGroups, PermissionPartGroup{Key: item.group, Title: item.title, Icon: item.icon})
	}
	return result, nil
}

// objectsToShow lists the objects of a kind by name: from the kind's list in
// the catalog, or, for a kind read apart from it - a common form - from the
// index of identifiers, titled by what was read.
func (catalog *Catalog) objectsToShow(kind Kind, titles map[uuid.UUID]LocalizedText) []ReferenceTypeObject {
	var result []ReferenceTypeObject
	if _, ok := catalog.objectsOf(kind); ok {
		result = catalog.referenceTypeObjects(kind)
	} else {
		for id, object := range catalog.objects {
			if object.kind != kind {
				continue
			}
			// An object inside another is indexed under its owner's name too,
			// and shown under its own beneath the owner.
			item := ReferenceTypeObject{ID: id, Name: object.name, Title: titles[id]}
			if object.owner != nil {
				item.Name, item.Title = object.name[strings.LastIndex(object.name, ".")+1:], object.title
			}
			result = append(result, item)
		}
	}
	slices.SortFunc(result, func(a, b ReferenceTypeObject) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return result
}

// ValidateProjectRole checks the edited role against current objects and forms,
// without requiring other, possibly unfinished roles to have valid references.
func ValidateProjectRole(root string, role RoleDefinition) error {
	catalog, err := load(root, false)
	if err != nil {
		return err
	}
	catalog.Roles = []RoleDefinition{cloneRole(role)}
	return catalog.indexAndValidate(root)
}
