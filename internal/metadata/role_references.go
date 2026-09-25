package metadata

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/k33alexey/MetaLab/internal/project"
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
	} else if index, ok := catalog.chartOfCalculationTypesByID[id]; ok {
		item := catalog.ChartsOfCalculationTypes[index]
		target.operations[PermissionCreate], target.operations[PermissionUpdate], target.operations[PermissionDelete] = true, true, true
		target.fields = map[string]bool{"ref": false, "code": true, "description": true,
			"deletionmark": true, "version": false, "predefined": false, "predefineddataname": false}
		if item.ActionPeriodUse {
			target.fields["actionperiodisbase"] = true
		}
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
	} else if index, ok := catalog.businessProcessByID[id]; ok {
		item := catalog.BusinessProcesses[index]
		target.operations[PermissionCreate], target.operations[PermissionUpdate], target.operations[PermissionDelete] = true, true, true
		// Started and completed are the platform's own record of where the
		// process is: a role may read them, never write them by hand.
		target.fields = map[string]bool{"ref": false, "number": true, "date": true, "deletionmark": true,
			"version": false, "started": false, "completed": false, "headtask": false}
		addAttributes(item.Attributes)
		addParts(item.TableParts)
	} else if index, ok := catalog.taskByID[id]; ok {
		item := catalog.Tasks[index]
		target.operations[PermissionCreate], target.operations[PermissionUpdate], target.operations[PermissionDelete] = true, true, true
		// Where the task stands is the platform's record too; what it is
		// addressed to is the application's, and a role may well be kept away
		// from it - seeing the performers of every task is not for everyone.
		target.fields = map[string]bool{"ref": false, "number": true, "date": true, "description": true,
			"deletionmark": true, "version": false, "executed": true, "businessprocess": false, "routepoint": false}
		addAttributes(addressingAsAttributes(item))
		addAttributes(item.Attributes)
		addParts(item.TableParts)
	} else if index, ok := catalog.exchangePlanByID[id]; ok {
		item := catalog.ExchangePlans[index]
		target.operations[PermissionCreate], target.operations[PermissionUpdate], target.operations[PermissionDelete] = true, true, true
		// Which node is this base and what the sides have exchanged is the
		// platform's own record: a role reads it and never sets it by hand,
		// because a node that declares itself this one breaks every exchange
		// that touches it.
		target.fields = map[string]bool{"ref": false, "code": true, "description": true, "deletionmark": true,
			"version": false, "thisnode": false, "sentno": false, "receivedno": false}
		addAttributes(item.Attributes)
		addParts(item.TableParts)
	} else if index, ok := catalog.sequenceByID[id]; ok {
		item := catalog.Sequences[index]
		// A sequence is read and it is rebuilt; there is nothing in it to
		// create or delete by hand, because every record of it belongs to the
		// document that made it.
		target.operations[PermissionUpdate] = true
		target.fields = map[string]bool{"period": false, "recorder": false}
		for _, dimension := range item.Dimensions {
			target.fields[dimension.ID.String()] = false
		}
	} else if _, ok := catalog.documentJournalByID[id]; ok {
		// A journal has no data of its own: it shows documents, and what a
		// role may see in it is decided by the rights on those documents. So
		// it carries the one right that is its own - whether it is visible at
		// all - and no fields.
		target.fields = map[string]bool{}
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
	} else if index, ok := catalog.accountingRegisterByID[id]; ok {
		item := catalog.AccountingRegisters[index]
		target.operations[PermissionUpdate] = true
		// Totals of an accounting register are the balances by account, and
		// they are exactly what a role may be kept away from managing.
		target.operations[PermissionTotalsControl] = true
		target.fields = map[string]bool{"recordid": false, "period": true, "recorder": true, "linenumber": false, "active": true}
		for _, field := range accountingRegisterFields(item) {
			target.fields[field.ID.String()] = true
		}
		addAttributes(item.Attributes)
	} else if index, ok := catalog.reportByID[id]; ok {
		item := catalog.Reports[index]
		// A report keeps no data, so there is nothing in it to create or
		// delete. What a role is given or denied is the report itself.
		target.fields = map[string]bool{}
		addAttributes(item.Attributes)
		addParts(item.TableParts)
	} else if index, ok := catalog.dataProcessorByID[id]; ok {
		item := catalog.DataProcessors[index]
		target.fields = map[string]bool{}
		addAttributes(item.Attributes)
		addParts(item.TableParts)
	} else if index, ok := catalog.calculationRegisterByID[id]; ok {
		item := catalog.CalculationRegisters[index]
		target.operations[PermissionUpdate] = true
		// Which kind of accrual a record is, and whether it reverses an
		// earlier one, are the platform's own account of the calculation: a
		// role reads them and does not set them by hand.
		target.fields = map[string]bool{"recordid": false, "period": true, "recorder": true,
			"linenumber": false, "active": true, "calculationtype": true, "reversing": false}
		if item.ActionPeriod {
			target.fields["actionperiodstart"], target.fields["actionperiodend"] = true, true
		}
		if item.BasePeriod {
			target.fields["baseperiodstart"], target.fields["baseperiodend"] = true, true
		}
		addAttributes(calculationRegisterFields(item))
		addAttributes(item.Attributes)
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
	// The forms are read once, not once per reference: a common form is found
	// by reading the folders now, and a configuration has as many roles
	// referring to forms as it has rights to grant.
	//
	// They are read whether or not any role refers to one, because the folder
	// is the whole declaration of a common form - if nothing read it, a form
	// with a broken folder would load as a configuration without that form.
	forms, err := ReadCommonForms(root, catalog.Project)
	if err != nil {
		return err
	}
	byID := make(map[uuid.UUID]ManagedForm, len(forms))
	for _, form := range forms {
		byID[form.ID] = form
	}
	return validateRoleCommands(catalog.Roles, func(id uuid.UUID) (ManagedForm, error) {
		form, ok := byID[id]
		if !ok {
			return ManagedForm{}, fmt.Errorf("form %s is missing or unsafe", id)
		}
		return form, nil
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

// ReadCommonForms reads every common form of a project, sorted by name. The
// folder is the list of them: a common form belongs to no object, so nothing
// declares it anywhere else.
func ReadCommonForms(root string, configuration project.Project) ([]ManagedForm, error) {
	directory := filepath.Join(root, "metadata", "common-forms")
	entries, err := os.ReadDir(directory)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read common forms: %w", err)
	}
	var result []ManagedForm
	seen := map[uuid.UUID]string{}
	for _, entry := range entries {
		if entry.Name() == ".gitkeep" {
			continue
		}
		if !entry.IsDir() || entry.Type()&fs.ModeSymlink != 0 {
			return nil, fmt.Errorf("common form %q must be a folder", entry.Name())
		}
		if project.ObjectName(entry.Name()) != nil {
			return nil, fmt.Errorf("common form folder %q is not a name", entry.Name())
		}
		form, err := readCommonForm(directory, entry.Name(), configuration)
		if err != nil {
			return nil, fmt.Errorf("common form %s: %w", entry.Name(), err)
		}
		if previous, exists := seen[form.ID]; exists {
			return nil, fmt.Errorf("common forms %s and %s share one identifier", previous, form.Name)
		}
		seen[form.ID] = form.Name
		result = append(result, form)
	}
	sort.Slice(result, func(left, right int) bool {
		return strings.ToLower(result[left].Name) < strings.ToLower(result[right].Name)
	})
	return result, nil
}

// readCommonForm reads one common form and checks its folder: the form calls
// itself what the folder is called, and the folder holds the form and the
// module that runs it, nothing else.
func readCommonForm(directory, name string, configuration project.Project) (ManagedForm, error) {
	entries, err := os.ReadDir(filepath.Join(directory, name))
	if err != nil {
		return ManagedForm{}, err
	}
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&fs.ModeSymlink != 0 ||
			(entry.Name() != project.FormMetadataFile && entry.Name() != project.FormModuleFile) {
			return ManagedForm{}, fmt.Errorf("keeps %q, and a form keeps only its description and its module", entry.Name())
		}
	}
	path := filepath.Join(directory, name, project.FormMetadataFile)
	file, err := os.Open(path)
	if err != nil {
		return ManagedForm{}, fmt.Errorf("has no %s", project.FormMetadataFile)
	}
	defer file.Close()
	form, err := DecodeManagedForm(path, file, configuration)
	if err != nil {
		return ManagedForm{}, err
	}
	if !strings.EqualFold(form.Name, name) {
		return ManagedForm{}, fmt.Errorf("calls itself %s, and lies in a folder called %s", form.Name, name)
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
