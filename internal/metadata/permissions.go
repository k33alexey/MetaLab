package metadata

import (
	"errors"
	"fmt"
	"slices"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

const MaxAssignedRoles = 1024
const MaxEffectivePermissions = 100_000

var ErrPermissionDenied = errors.New("application permission denied")
var ErrInvalidRoleSelection = errors.New("invalid application role selection")

type permissionBits uint8

func operationBit(operation PermissionOperation) permissionBits {
	switch operation {
	case PermissionRead:
		return 1
	case PermissionCreate:
		return 2
	case PermissionUpdate:
		return 4
	case PermissionDelete:
		return 8
	case PermissionPost:
		return 16
	case PermissionUndoPosting:
		return 32
	}
	return 0
}

type objectPermissions struct {
	operations permissionBits
	fields     map[string]permissionBits
	parents    map[string]string
	rows       map[PermissionOperation]*rowFilter
}

// rowFilter describes which rows the assigned roles admit for one operation on
// one object. Alternatives are OR-ed, matching how the grants themselves
// combine: each restriction names an allowed set of rows, and holding more
// roles can only widen the union, never narrow it.
//
// unrestricted records that at least one granting role attached no restriction
// at all, which admits every row and makes the alternatives irrelevant. The zero
// value therefore means "restricted with no alternatives" - no rows - so an
// operation that was never granted cannot fall through as unrestricted.
type rowFilter struct {
	unrestricted bool
	alternatives []PolicyRule
}

// Permissions is an immutable union of explicitly assigned application roles.
// It has no system-administrator override. Nil, zero and empty policies deny
// everything. Build a new policy from the active publication when grants change;
// do not retain one across authenticated requests or publication versions.
type Permissions struct {
	project  uuid.UUID
	objects  map[uuid.UUID]objectPermissions
	commands map[CommandPermission]bool
}

// CompilePermissions consumes a validated catalog, never client-supplied role
// definitions. A missing role rejects the entire selection rather than silently
// running with a partial set. Result maps do not retain mutable catalog slices.
func CompilePermissions(catalog *Catalog, roleIDs []uuid.UUID) (*Permissions, error) {
	if catalog == nil || catalog.Project.ID.IsZero() || len(roleIDs) > MaxAssignedRoles {
		return nil, ErrInvalidRoleSelection
	}
	result := &Permissions{project: catalog.Project.ID, objects: map[uuid.UUID]objectPermissions{}, commands: map[CommandPermission]bool{}}
	seen := map[uuid.UUID]bool{}
	count := 0
	for _, id := range roleIDs {
		if id.IsZero() || seen[id] {
			return nil, ErrInvalidRoleSelection
		}
		seen[id] = true
		role, ok := catalog.RoleByID(id)
		if !ok {
			return nil, ErrInvalidRoleSelection
		}
		count += len(role.Objects) + len(role.Commands)
		for _, item := range role.Objects {
			count += len(item.Fields)
		}
		if count > MaxEffectivePermissions {
			return nil, fmt.Errorf("%w: combined permissions exceed %d", ErrInvalidRoleSelection, MaxEffectivePermissions)
		}
		for _, grant := range role.Objects {
			item, exists := result.objects[grant.Object]
			if !exists {
				item = objectPermissions{fields: map[string]permissionBits{}, parents: map[string]string{}, rows: map[PermissionOperation]*rowFilter{}}
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
				item.operations |= operationBit(operation)
				filter := item.rows[operation]
				if filter == nil {
					filter = &rowFilter{}
					item.rows[operation] = filter
				}
				rules, err := resolveRolePolicies(role, grant, operation)
				if err != nil {
					return nil, fmt.Errorf("%w: %w", ErrInvalidRoleSelection, err)
				}
				if len(rules) == 0 {
					// This role grants the operation outright, so no restriction
					// from any other role can take those rows away.
					filter.unrestricted = true
					continue
				}
				filter.alternatives = append(filter.alternatives, rules...)
			}
			for _, field := range grant.Fields {
				for _, operation := range field.Operations {
					item.fields[field.Field] |= operationBit(operation)
				}
			}
			result.objects[grant.Object] = item
		}
		for _, command := range role.Commands {
			result.commands[command] = true
		}
	}
	return result, nil
}

// resolveRolePolicies returns the rules one role applies to one operation on one
// object, with templates already resolved: a template is role-scoped, so after
// compilation nothing needs the role definition again.
func resolveRolePolicies(role RoleDefinition, grant ObjectPermission, operation PermissionOperation) ([]PolicyRule, error) {
	var rules []PolicyRule
	for _, policy := range grant.Policies {
		if !slices.Contains(policy.Operations, operation) {
			continue
		}
		if policy.Rule != nil {
			rules = append(rules, clonePolicyRule(*policy.Rule))
			continue
		}
		index := slices.IndexFunc(role.PolicyTemplates, func(item PolicyTemplate) bool { return item.Name == policy.Template })
		if index < 0 {
			return nil, fmt.Errorf("role %s references undeclared policy template %q", role.Name, policy.Template)
		}
		rules = append(rules, clonePolicyRule(role.PolicyTemplates[index].Rule))
	}
	return rules, nil
}

// RowFilter reports how rows must be restricted for one operation on one object.
// restricted=false means every row the grant covers is visible. restricted=true
// with no alternatives means no row is - which is also what an ungranted
// operation returns, so a caller that forgets to check the grant fails closed.
// Callers must still check the grant itself: this answers only "which rows".
func (permissions *Permissions) RowFilter(object uuid.UUID, operation PermissionOperation) (alternatives []PolicyRule, restricted bool) {
	if permissions == nil {
		return nil, true
	}
	filter := permissions.objects[object].rows[operation]
	if filter == nil {
		return nil, true
	}
	if filter.unrestricted {
		return nil, false
	}
	return filter.alternatives, true
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
	if operation == PermissionCreate {
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

func (permissions *Permissions) AllowsCommand(form, command uuid.UUID) bool {
	return permissions != nil && permissions.commands[CommandPermission{Form: form, Command: command}]
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

func (permissions *Permissions) RequireCommand(form, command uuid.UUID) error {
	if !permissions.AllowsCommand(form, command) {
		return ErrPermissionDenied
	}
	return nil
}
