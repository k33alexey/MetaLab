package studio

import (
	"fmt"
	"reflect"
	"slices"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// The page «Права» of the window of an object shows the roles of the project
// with what each says about the object alone: its rights and its
// restrictions of access to data (docs/block-00/spravochnik.md, page 13;
// snapshot okno-13). What a role says is the role's, so a change of the page
// is an edit of the role, written with the object by «Сохранить» and taken
// back with it.

// RolePlace is a role as the page shows it - what it says about the object
// alone, its flags and templates as they are - with where it lies and the
// revision it was read at.
type RolePlace struct {
	Path     string                  `json:"path"`
	Revision string                  `json:"revision"`
	Role     metadata.RoleDefinition `json:"role"`
}

// RoleMark is what the window writes of one role: what it says about the
// object, nothing where it says nothing, and the revision of the role the
// window read.
type RoleMark struct {
	ID         uuid.UUID                  `json:"id"`
	Revision   string                     `json:"revision"`
	Permission *metadata.ObjectPermission `json:"permission,omitempty"`
}

// roleOrder is the roles of the project in the order of the tree.
func roleOrder(loaded *metadata.Catalog) []uuid.UUID {
	var order []uuid.UUID
	for _, item := range loaded.KindObjects(metadata.RoleKind) {
		order = append(order, item.ID)
	}
	return order
}

// rolePlacesLocked are the roles of order, each with what it says about the
// object alone.
func (workspace *Workspace) rolePlacesLocked(order []uuid.UUID, object uuid.UUID, configuration project.Project) ([]RolePlace, error) {
	places := []RolePlace{}
	for _, id := range order {
		relative, err := project.MetadataPath(string(metadata.RoleKind), id)
		if err != nil {
			return nil, err
		}
		role, revision, err := workspace.decodeRoleLocked(relative, configuration)
		if err != nil {
			return nil, err
		}
		role.Objects = slices.DeleteFunc(role.Objects, func(entry metadata.ObjectPermission) bool { return entry.Object != object })
		role.Commands = nil
		places = append(places, RolePlace{Path: relative, Revision: revision, Role: role})
	}
	return places, nil
}

// objectRightsSchema is the schema of the rights of the object alone: what
// the model of a role needs to show and change them.
func objectRightsSchema(root string, object uuid.UUID) (metadata.PermissionSchema, error) {
	schema, err := metadata.LoadPermissionSchema(root)
	if err != nil {
		return metadata.PermissionSchema{}, err
	}
	schema.Objects = slices.DeleteFunc(schema.Objects, func(item metadata.PermissionObject) bool { return item.ID != object })
	schema.Parts = map[uuid.UUID][]string{object: schema.Parts[object]}
	schema.Folders, schema.Branches, schema.Subsystems, schema.PartGroups = nil, nil, nil, nil
	return schema, nil
}

// permissionOf is what a role says about the object, nil for nothing.
func permissionOf(role metadata.RoleDefinition, object uuid.UUID) *metadata.ObjectPermission {
	index := slices.IndexFunc(role.Objects, func(entry metadata.ObjectPermission) bool { return entry.Object == object })
	if index < 0 {
		return nil
	}
	return &role.Objects[index]
}

// samePermission tells two entries of a role about one object that say the
// same: an empty list is no list, and an entry saying nothing is none.
func samePermission(a, b *metadata.ObjectPermission) bool {
	return reflect.DeepEqual(plainPermission(a), plainPermission(b))
}

func plainPermission(entry *metadata.ObjectPermission) metadata.ObjectPermission {
	if entry == nil {
		return metadata.ObjectPermission{}
	}
	plain := metadata.ObjectPermission{Object: entry.Object}
	if len(entry.Operations) > 0 {
		plain.Operations = entry.Operations
	}
	if len(entry.Denied) > 0 {
		plain.Denied = entry.Denied
	}
	for _, field := range entry.Fields {
		if len(field.Operations) > 0 || len(field.Denied) > 0 {
			plain.Fields = append(plain.Fields, metadata.FieldPermission{Field: field.Field, Operations: nonEmpty(field.Operations), Denied: nonEmpty(field.Denied)})
		}
	}
	for _, restriction := range entry.Restrictions {
		restriction.Fields = nonEmpty(restriction.Fields)
		plain.Restrictions = append(plain.Restrictions, restriction)
	}
	if plain.Operations == nil && plain.Denied == nil && plain.Fields == nil && plain.Restrictions == nil {
		return metadata.ObjectPermission{}
	}
	return plain
}

func nonEmpty[T any](items []T) []T {
	if len(items) == 0 {
		return nil
	}
	return items
}

// putPermission makes the role say about the object what entry says, in
// the place its entry had, last when it had none; an entry saying nothing
// takes the role's entry away.
func putPermission(role *metadata.RoleDefinition, object uuid.UUID, entry *metadata.ObjectPermission) {
	index := slices.IndexFunc(role.Objects, func(item metadata.ObjectPermission) bool { return item.Object == object })
	plain := plainPermission(entry)
	switch {
	case plain.Object.IsZero() && index >= 0:
		role.Objects = slices.Delete(role.Objects, index, index+1)
	case plain.Object.IsZero():
	case index >= 0:
		plain.Object = object
		role.Objects[index] = plain
	default:
		plain.Object = object
		role.Objects = append(role.Objects, plain)
	}
}

// roleEditsLocked are the entries the page changes, by role: each role
// changed is checked before anything is written - it must lie as the window
// read it, and say with the change what a role may say. A role saying of the
// object what the page says is not among them. current are the roles as
// they lie now.
func (workspace *Workspace) roleEditsLocked(object uuid.UUID, current []RolePlace, marks []RoleMark, configuration project.Project) (map[uuid.UUID]*metadata.ObjectPermission, error) {
	edits := map[uuid.UUID]*metadata.ObjectPermission{}
	for _, mark := range marks {
		index := slices.IndexFunc(current, func(place RolePlace) bool { return place.Role.ID == mark.ID })
		if index < 0 {
			return nil, fmt.Errorf("role %s is not in the project", mark.ID)
		}
		place := current[index]
		if samePermission(permissionOf(place.Role, object), mark.Permission) {
			continue
		}
		if mark.Permission != nil && mark.Permission.Object != object {
			return nil, fmt.Errorf("role %s: the page writes the rights of %s alone", place.Role.Name, object)
		}
		// A role changed since the window read it - in its own window, in
		// «Все роли», by hand - is not overwritten.
		if place.Revision != mark.Revision {
			return nil, fmt.Errorf("role %s: %w", place.Role.Name, ErrSourceChanged)
		}
		role, _, err := workspace.decodeRoleLocked(place.Path, configuration)
		if err != nil {
			return nil, err
		}
		putPermission(&role, object, mark.Permission)
		if err := metadata.ValidateProjectRole(workspace.root, role); err != nil {
			return nil, fmt.Errorf("role %s: %w", place.Role.Name, err)
		}
		edits[mark.ID] = mark.Permission
	}
	return edits, nil
}
