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
	// Everything that can be read can also be shown, so View travels with Read;
	// which objects accept the other rights is decided per kind below, from
	// what the object actually has.
	target := roleTarget{operations: map[PermissionOperation]bool{PermissionRead: true, PermissionView: true}, fields: map[string]bool{}}
	addAttributes := func(attributes []Attribute) {
		for _, attribute := range attributes {
			target.fields[attribute.ID.String()] = true
		}
	}
	// The parent and the folder flag are standard fields where nesting exists:
	// moving a row to another parent is an edit like any other, and a role may
	// well be allowed to read a tree without rearranging it.
	addHierarchy := func(hierarchy Hierarchy) {
		if !hierarchy.Enabled {
			return
		}
		target.fields["parent"] = true
		if hierarchy.Kind == FoldersAndItemsHierarchy {
			target.fields["isfolder"] = false
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
		addHierarchy(item.Hierarchy)
		addAttributes(item.Attributes)
		addParts(item.TableParts)
	} else if index, ok := catalog.chartOfCharacteristicTypesByID[id]; ok {
		item := catalog.ChartsOfCharacteristicTypes[index]
		// A chart of characteristic types is a catalog of kinds of property,
		// and rights on it are a catalog's rights. Its own standard attribute
		// is the value type: a role may keep it out of reach while letting the
		// rest of the element be edited.
		target.operations[PermissionCreate], target.operations[PermissionUpdate], target.operations[PermissionDelete] = true, true, true
		target.fields = map[string]bool{"ref": false, "code": true, "description": true, "valuetype": true,
			"deletionmark": true, "version": false, "predefined": false, "predefineddataname": false}
		addHierarchy(item.Hierarchy)
		addAttributes(item.Attributes)
		addParts(item.TableParts)
	} else if index, ok := catalog.chartOfAccountsByID[id]; ok {
		item := catalog.ChartsOfAccounts[index]
		target.operations[PermissionCreate], target.operations[PermissionUpdate], target.operations[PermissionDelete] = true, true, true
		// The order is derived from the code, so it is readable but not
		// writable: letting a role grant what nobody may change would be a
		// right over nothing.
		target.fields = map[string]bool{"ref": false, "code": true, "description": true, "accountorder": false,
			"accountkind": true, "offbalance": true, "deletionmark": true, "version": false,
			"predefined": false, "predefineddataname": false}
		// Each declared flag is a field of its own, on the account and on the
		// line of analytics alike: an application may well let a role see the
		// accounts and not the flags its bookkeeping rests on.
		for _, flag := range item.AccountingFlags {
			target.fields[flag.ID.String()] = true
		}
		for _, flag := range item.ExtDimensionAccountingFlags {
			target.fields[flag.ID.String()] = true
		}
		// Accounts nest always, so the parent is a field of every chart.
		addHierarchy(Hierarchy{Enabled: true, Kind: ItemsHierarchy})
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
		// Totals are the pre-aggregated balances an accumulation register keeps
		// beside its movements. In the reference configuration this right sits
		// on information registers, because there the materialized slices are
		// the totals; ours are on accumulation registers, and slices are not
		// materialized at all - so the right guards what we actually have.
		if item.Kind == AccumulationRegisterBalance {
			target.operations[PermissionTotalsControl] = true
		}
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
				if role.PolicyTemplates[index].Name != policy.Template {
					continue
				}
				// The template's own field may be a parameter; what is checked
				// against this object is the rule after the restriction supplied
				// its fields, exactly as the compiled permission will apply it.
				resolved, err := substitutePolicyPlaceholders(role.PolicyTemplates[index].Rule, role.PolicyTemplates[index].Parameters, policy.Arguments)
				if err != nil {
					return fmt.Errorf("role %s: policy template %s: %w", role.Name, policy.Template, err)
				}
				rule = &resolved
				break
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
		if err := catalog.validatePolicyOperand(role, *rule); err != nil {
			return err
		}
		if rule.Subquery == nil {
			continue
		}
		// The subquery reads another object, so its own field and conditions are
		// checked against THAT object - a policy must not be able to name a field
		// of the restricted table and have it silently resolve elsewhere.
		source, ok := catalog.permissionTarget(rule.Subquery.Object)
		if !ok {
			return fmt.Errorf("role %s: policy subquery reads unknown or unsupported object %s", role.Name, rule.Subquery.Object)
		}
		if _, ok := source.fields[rule.Subquery.Field]; !ok {
			return fmt.Errorf("role %s: policy subquery field %s does not belong to object %s", role.Name, rule.Subquery.Field, rule.Subquery.Object)
		}
		for _, condition := range rule.Subquery.Where {
			if _, ok := source.fields[condition.Field]; !ok {
				return fmt.Errorf("role %s: policy subquery condition field %s does not belong to object %s", role.Name, condition.Field, rule.Subquery.Object)
			}
			if err := catalog.validatePolicyOperand(role, condition); err != nil {
				return err
			}
		}
	}
	return nil
}

// validatePolicyOperand checks what a rule compares against, independently of
// which object the field itself belongs to.
func (catalog *Catalog) validatePolicyOperand(role RoleDefinition, rule PolicyRule) error {
	if rule.Parameter == "" {
		return nil
	}
	if ReservedSessionParameter(rule.Parameter) {
		// The platform supplies this one, so a project never declares it.
		return nil
	}
	if _, ok := catalog.SessionParameter(rule.Parameter); !ok {
		return fmt.Errorf("role %s: policy references unknown session parameter %s", role.Name, rule.Parameter)
	}
	return nil
}

func (catalog *Catalog) readRoleForm(root string, id uuid.UUID) (ManagedForm, error) {
	path := filepath.Join(root, "metadata", "common-forms", id.String()+".yaml")
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
