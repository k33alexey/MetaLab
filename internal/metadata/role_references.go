package metadata

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/k33alexey/MetaLab/internal/kinds"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// roleTarget describes the actual object shape. The bool says whether a field
// can be written; object operation checks remain separate from field checks.
// rightsTarget is what a role may give rights on in one object: the rights
// the configurator offers on the object itself, and its parts it offers
// rights on, by the key a role keeps them under.
type rightsTarget struct {
	// rights is nil for a kind no export gives rights on: such an object is
	// carried with the note use-without-sample.
	rights []PermissionOperation
	parts  map[string]objectPart
}

// rightsTargetOf finds what a role may give rights on: the configuration
// itself, by the project's identifier, or an object of it, with the rights
// the configurator offers on it and on each of its parts (configurationRights,
// kindRightsOf, partsOf, partRightsOf).
func (catalog *Catalog) rightsTargetOf(id uuid.UUID) (rightsTarget, bool) {
	target := rightsTarget{parts: map[string]objectPart{}}
	if id == catalog.Project.ID {
		target.rights = configurationRights
		return target, true
	}
	kind, ok := catalog.kindOfObject(id)
	if !ok || !usesOf(kind).rights.accepted() {
		return rightsTarget{}, false
	}
	target.rights = usesOf(kind).offered
	found, _ := catalog.partsOf(kind, id)
	for _, part := range found {
		if len(partRightsOf(part.group)) > 0 {
			target.parts[part.key] = part
		}
	}
	return target, true
}

func (catalog *Catalog) validateRoleReferences(root string) error {
	targets := map[uuid.UUID]rightsTarget{}
	for _, role := range catalog.Roles {
		for _, permission := range role.Objects {
			target, ok := targets[permission.Object]
			if !ok {
				target, ok = catalog.rightsTargetOf(permission.Object)
				if !ok {
					return fmt.Errorf("role %s references unknown or unsupported object %s", role.Name, permission.Object)
				}
				targets[permission.Object] = target
			}
			for _, field := range permission.Fields {
				if _, ok := target.parts[field.Field]; !ok {
					return fmt.Errorf("role %s: field %s does not belong to object %s", role.Name, field.Field, permission.Object)
				}
			}
			if err := catalog.validateRoleRestrictions(role, permission, target); err != nil {
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

// validateRoleRestrictions checks the restrictions of one object against the
// object: the configurator offers the right restricted on it, and the fields
// named are its own. A kind no export restricts is accepted and
// noted (restriction-without-sample).
func (catalog *Catalog) validateRoleRestrictions(role RoleDefinition, permission ObjectPermission, target rightsTarget) error {
	if len(permission.Restrictions) == 0 {
		return nil
	}
	// A kind keeping no data is offered none of the rights restricted, so the
	// check of the right below refuses a restriction of it.
	kind, _ := catalog.kindOfObject(permission.Object)
	fields := map[string]bool{}
	for _, field := range catalog.restrictionFieldsOf(kind, permission.Object) {
		fields[field.key] = true
	}
	for _, restriction := range permission.Restrictions {
		if !slices.Contains(target.rights, restriction.Right) {
			return fmt.Errorf("role %s: object %s has no right %s to restrict", role.Name, permission.Object, restriction.Right)
		}
		for _, field := range restriction.Fields {
			if !fields[field] {
				return fmt.Errorf("role %s: restriction of %s names field %s, which object %s does not have", role.Name, restriction.Right, field, permission.Object)
			}
		}
	}
	return nil
}

// CommonFormKind holds the forms that belong to no object.
const CommonFormKind = kinds.CommonForm

// ReadCommonForms reads every common form of a project, sorted by name. The
// folder is the list of them: a common form belongs to no object, so nothing
// declares it anywhere else.
func ReadCommonForms(root string, configuration project.Project) ([]ManagedForm, error) {
	directory := filepath.Join(root, "metadata", string(CommonFormKind))
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
		if entry.IsDir() && entry.Type()&fs.ModeSymlink == 0 && entry.Name() == project.HelpDirectory {
			if err := validateHelpFolder("common form "+name, filepath.Join(directory, name, entry.Name())); err != nil {
				return ManagedForm{}, err
			}
			continue
		}
		// The pictures of the elements are checked once the form is read.
		if entry.Name() == project.FormItemsDirectory {
			continue
		}
		if entry.IsDir() || entry.Type()&fs.ModeSymlink != 0 ||
			(entry.Name() != project.FormMetadataFile && entry.Name() != project.FormModuleFile) {
			return ManagedForm{}, fmt.Errorf("keeps %q, and a form keeps only its description, its module, its help and the pictures of its elements", entry.Name())
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
	if err := agreesWithFolder("form", form.Name, name); err != nil {
		return ManagedForm{}, err
	}
	if err := validateFormItemPictureFiles(filepath.Join(directory, name), form); err != nil {
		return ManagedForm{}, err
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
