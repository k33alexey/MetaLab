package metadata

import (
	"errors"
	"slices"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

var ErrPermissionDenied = errors.New("application permission denied")
var ErrInvalidRoleSelection = errors.New("invalid application role selection")

type permissionBits uint16

// operationBit derives a right's bit from its position in objectOperations
// rather than from a list of its own. The two lists disagreeing is not a
// theoretical risk: adding a right to the model while this switch kept
// returning zero for it made the right silently ungrantable, which is how it
// was found.
func operationBit(operation PermissionOperation) permissionBits {
	index := slices.Index(objectOperations, operation)
	if index < 0 || index >= 16 {
		return 0
	}
	return 1 << index
}

// fieldOperations is what a right on a part of an object lets the runtime do
// with it. The prototype gives a part two rights of its own, view and edit,
// and the runtime asks of a field whether it may be read and whether it may be
// written. Any other right on a part is carried and does nothing yet.
var fieldOperations = map[PermissionOperation]PermissionOperation{PermissionView: PermissionRead, PermissionEdit: PermissionUpdate}

type objectPermissions struct {
	operations permissionBits
	fields     map[string]permissionBits
	parents    map[string]string
	// readsEveryRow records that at least one role grants reading the object
	// with no restriction of its own, which admits every row. Without it the
	// reading granted is restricted, and since conditions are not run yet a
	// restricted read admits no row: the zero value fails closed.
	readsEveryRow bool
}

// Permissions is an immutable union of explicitly assigned application roles.
// It has no system-administrator override. Nil, zero and empty policies deny
// everything. Build a new policy from the active publication when grants change;
// do not retain one across authenticated requests or publication versions.
type Permissions struct {
	project uuid.UUID
	objects map[uuid.UUID]objectPermissions
}

// CompilePermissions consumes a validated catalog, never client-supplied role
// definitions. A missing role rejects the entire selection rather than silently
// running with a partial set. Result maps do not retain mutable catalog slices.
//
// Neither the number of roles nor the permissions they add up to is bounded:
// the prototype bounds neither (help, ПользовательИнформационнойБазы.Роли),
// and erp alone has 1211 roles. A selection cannot outgrow the catalog, since
// a role is taken once and only if the catalog has it, and what is compiled is
// no larger than the roles the catalog already holds.
func CompilePermissions(catalog *Catalog, roleIDs []uuid.UUID) (*Permissions, error) {
	if catalog == nil || catalog.Project.ID.IsZero() {
		return nil, ErrInvalidRoleSelection
	}
	result := &Permissions{project: catalog.Project.ID, objects: map[uuid.UUID]objectPermissions{}}
	seen := map[uuid.UUID]bool{}
	for _, id := range roleIDs {
		if id.IsZero() || seen[id] {
			return nil, ErrInvalidRoleSelection
		}
		seen[id] = true
		role, ok := catalog.RoleByID(id)
		if !ok {
			return nil, ErrInvalidRoleSelection
		}
		for _, grant := range role.Objects {
			item, exists := result.objects[grant.Object]
			if !exists {
				item = objectPermissions{fields: map[string]permissionBits{}, parents: map[string]string{}}
				var parts []TablePart
				if index, ok := catalog.catalogByID[grant.Object]; ok {
					parts = catalog.Catalogs[index].TableParts
				} else if index, ok := catalog.documentByID[grant.Object]; ok {
					parts = catalog.Documents[index].TableParts
				}
				for _, part := range parts {
					for _, field := range part.Attributes {
						item.parents[field.ID.String()] = part.ID.String()
					}
				}
			}
			for _, operation := range grant.Operations {
				restricted := grant.restrictsData(operation)
				switch {
				case operation == PermissionRead:
					// Reading restricted is still reading: the rows the
					// condition admits, which until it is run are none.
					item.operations |= operationBit(operation)
					item.readsEveryRow = item.readsEveryRow || !restricted
				case !restricted:
					item.operations |= operationBit(operation)
				}
				// Adding, changing or deleting restricted gives nothing: there
				// is no row to admit a write by until the condition is run.
			}
			for _, field := range grant.Fields {
				for _, operation := range field.Operations {
					item.fields[field.Field] |= operationBit(fieldOperations[operation])
				}
			}
			result.objects[grant.Object] = item
		}
	}
	return result, nil
}

// ReadsEveryRow says whether the roles let every row of the object be read.
// False with reading granted is reading restricted, which admits no row until
// conditions are run (block 10); false without it is no reading at all, so a
// caller that forgets to check the grant fails closed. Callers must still
// check the grant itself: this answers only "which rows".
func (permissions *Permissions) ReadsEveryRow(object uuid.UUID) bool {
	return permissions != nil && permissions.objects[object].readsEveryRow
}

func (permissions *Permissions) ProjectID() uuid.UUID {
	if permissions == nil {
		return uuid.UUID{}
	}
	return permissions.project
}

func (permissions *Permissions) AllowsObject(object uuid.UUID, operation PermissionOperation) bool {
	if permissions == nil {
		return false
	}
	bit := operationBit(operation)
	return bit != 0 && permissions.objects[object].operations&bit != 0
}

// AllowsFields requires the object operation even for an empty field list (for
// example replacing a register with an empty record set). Create and update
// require the same field-edit grant, but remain distinct object operations.
// A table-part column additionally requires its parent table-part grant.
func (permissions *Permissions) AllowsFields(object uuid.UUID, operation PermissionOperation, fields ...string) bool {
	if !permissions.AllowsObject(object, operation) {
		return false
	}
	fieldOperation := operation
	if operation == PermissionInsert {
		fieldOperation = PermissionUpdate
	}
	if len(fields) != 0 && fieldOperation != PermissionRead && fieldOperation != PermissionUpdate {
		return false
	}
	item, bit := permissions.objects[object], operationBit(fieldOperation)
	for _, field := range fields {
		if item.fields[field]&bit == 0 {
			return false
		}
		if parent := item.parents[field]; parent != "" && item.fields[parent]&bit == 0 {
			return false
		}
	}
	return true
}

func (permissions *Permissions) RequireObject(object uuid.UUID, operation PermissionOperation) error {
	if !permissions.AllowsObject(object, operation) {
		return ErrPermissionDenied
	}
	return nil
}

func (permissions *Permissions) RequireFields(object uuid.UUID, operation PermissionOperation, fields ...string) error {
	if !permissions.AllowsFields(object, operation, fields...) {
		return ErrPermissionDenied
	}
	return nil
}
