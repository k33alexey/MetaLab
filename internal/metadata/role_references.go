package metadata

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// roleTarget describes the actual object shape. The bool says whether a field
// can be written; object operation checks remain separate from field checks.
type roleTarget struct {
	operations map[PermissionOperation]bool
	fields     map[string]bool
}

func (catalog *Catalog) permissionTarget(id uuid.UUID) (roleTarget, bool) {
	target := roleTarget{operations: map[PermissionOperation]bool{PermissionRead: true}, fields: map[string]bool{}}
	addAttributes := func(attributes []Attribute) {
		for _, attribute := range attributes {
			target.fields[attribute.ID.String()] = true
		}
	}
	addParts := func(parts []TablePart) {
		for _, part := range parts {
			target.fields[part.ID.String()] = true
			addAttributes(part.Attributes)
		}
	}
	if _, ok := catalog.constantByID[id]; ok {
		target.operations[PermissionUpdate] = true
		target.fields["value"] = true
	} else if _, ok := catalog.enumerationByID[id]; ok {
		// Enumeration values are metadata, never mutable application records.
		target.fields["ref"], target.fields["order"] = false, false
	} else if index, ok := catalog.catalogByID[id]; ok {
		item := catalog.Catalogs[index]
		target.operations[PermissionCreate], target.operations[PermissionUpdate], target.operations[PermissionDelete] = true, true, true
		target.fields = map[string]bool{"ref": false, "code": true, "description": true, "deletionmark": true, "version": false, "predefined": false, "predefineddataname": false}
		addAttributes(item.Attributes)
		addParts(item.TableParts)
	} else if index, ok := catalog.documentByID[id]; ok {
		item := catalog.Documents[index]
		target.operations[PermissionCreate], target.operations[PermissionUpdate], target.operations[PermissionDelete] = true, true, true
		if item.Posting {
			target.operations[PermissionPost], target.operations[PermissionUndoPosting] = true, true
		}
		target.fields = map[string]bool{"ref": false, "number": true, "date": true, "posted": false, "deletionmark": true, "version": false}
		addAttributes(item.Attributes)
		addParts(item.TableParts)
	} else if index, ok := catalog.informationRegisterByID[id]; ok {
		item := catalog.InformationRegisters[index]
		target.fields["recordid"] = false
		// Register changes are atomic record-set writes, including replacement
		// with an empty set; create/delete are not separate record operations.
		target.operations[PermissionUpdate] = true
		if item.Periodicity != InformationRegisterPeriodNone {
			target.fields["period"] = true
		}
		if item.WriteMode == InformationRegisterRecorder {
			target.fields["recorder"], target.fields["linenumber"], target.fields["active"] = true, false, true
		}
		addAttributes(informationRegisterFields(item))
	} else if index, ok := catalog.accumulationRegisterByID[id]; ok {
		item := catalog.AccumulationRegisters[index]
		target.operations[PermissionUpdate] = true
		target.fields = map[string]bool{"recordid": false, "period": true, "recorder": true, "linenumber": false, "active": true}
		if item.Kind == AccumulationRegisterBalance {
			target.fields["movementkind"] = true
		}
		addAttributes(accumulationRegisterFields(item))
	} else {
		return roleTarget{}, false
	}
	return target, true
}

func (catalog *Catalog) validateRoleReferences(root string) error {
	targets := map[uuid.UUID]roleTarget{}
	for _, role := range catalog.Roles {
		for _, permission := range role.Objects {
			target, ok := targets[permission.Object]
			if !ok {
				target, ok = catalog.permissionTarget(permission.Object)
				if !ok {
					return fmt.Errorf("role %s references unknown or unsupported object %s", role.Name, permission.Object)
				}
				targets[permission.Object] = target
			}
			for _, operation := range permission.Operations {
				if !target.operations[operation] {
					return fmt.Errorf("role %s: operation %s is unsupported for object %s", role.Name, operation, permission.Object)
				}
			}
			for _, field := range permission.Fields {
				writable, ok := target.fields[field.Field]
				if !ok {
					return fmt.Errorf("role %s: field %s does not belong to object %s", role.Name, field.Field, permission.Object)
				}
				for _, operation := range field.Operations {
					if operation == PermissionUpdate && !writable {
						return fmt.Errorf("role %s: field %s is read-only", role.Name, field.Field)
					}
				}
			}
			if err := catalog.validateRolePolicies(role, permission, target); err != nil {
				return err
			}
		}
	}
	if root == "" {
		return nil
	}
	return validateRoleCommands(catalog.Roles, func(id uuid.UUID) (ManagedForm, error) {
		return catalog.readRoleForm(root, id)
	})
}

// validateRolePolicies resolves every restriction of one object down to the rule
// it actually applies - inline or through a role template - and checks it against
// the real object and the declared session parameters. A template is role-scoped
// and may be reused across objects, so its field is checked per use, not once.
func (catalog *Catalog) validateRolePolicies(role RoleDefinition, permission ObjectPermission, target roleTarget) error {
	for _, policy := range permission.Policies {
		rule := policy.Rule
		if rule == nil {
			for index := range role.PolicyTemplates {
				if role.PolicyTemplates[index].Name == policy.Template {
					rule = &role.PolicyTemplates[index].Rule
					break
				}
			}
		}
		if rule == nil {
			return fmt.Errorf("role %s: policy template %s is not declared", role.Name, policy.Template)
		}
		for _, operation := range policy.Operations {
			if !target.operations[operation] {
				return fmt.Errorf("role %s: policy operation %s is unsupported for object %s", role.Name, operation, permission.Object)
			}
		}
		if _, ok := target.fields[rule.Field]; !ok {
			return fmt.Errorf("role %s: policy field %s does not belong to object %s", role.Name, rule.Field, permission.Object)
		}
		if rule.Parameter != "" {
			if _, ok := catalog.SessionParameter(rule.Parameter); !ok {
				return fmt.Errorf("role %s: policy references unknown session parameter %s", role.Name, rule.Parameter)
			}
		}
	}
	return nil
}

func (catalog *Catalog) readRoleForm(root string, id uuid.UUID) (ManagedForm, error) {
	path := filepath.Join(root, "forms", id.String()+".yaml")
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return ManagedForm{}, fmt.Errorf("form %s is missing or unsafe", id)
	}
	file, err := os.Open(path)
	if err != nil {
		return ManagedForm{}, err
	}
	defer file.Close()
	form, err := DecodeManagedForm(path, file, catalog.Project)
	if err != nil {
		return ManagedForm{}, err
	}
	if form.ID != id {
		return ManagedForm{}, fmt.Errorf("form UUID %s does not match filename UUID %s", form.ID, id)
	}
	return form, nil
}

func validateRoleCommands(roles []RoleDefinition, resolve func(uuid.UUID) (ManagedForm, error)) error {
	forms := map[uuid.UUID]map[uuid.UUID]bool{}
	for _, role := range roles {
		for _, permission := range role.Commands {
			commands, ok := forms[permission.Form]
			if !ok {
				form, err := resolve(permission.Form)
				if err != nil {
					return fmt.Errorf("role %s: %w", role.Name, err)
				}
				commands = make(map[uuid.UUID]bool, len(form.Commands))
				for _, command := range form.Commands {
					commands[command.ID] = true
				}
				forms[permission.Form] = commands
			}
			if !commands[permission.Command] {
				return fmt.Errorf("role %s: command %s does not belong to form %s", role.Name, permission.Command, permission.Form)
			}
		}
	}
	return nil
}
