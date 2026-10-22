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

func (catalog *Catalog) validateRoleReferences() error {
	targets := map[uuid.UUID]rightsTarget{}
	for _, role := range catalog.Roles {
		if err := validateDroppedRights(role); err != nil {
			return err
		}
		for _, permission := range role.Objects {
			target, ok := targets[permission.Object]
			if !ok {
				target, ok = catalog.rightsTargetOf(permission.Object)
				if _, outside := catalog.outside[permission.Object]; !ok && outside {
					// Checked against its description when the project was
					// read; the snapshot carries the rights and runs none.
					continue
				}
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
	return nil
}

// validateDroppedRights refuses a role that gives or denies a right ML no
// longer keeps, naming the role and the right, and one giving rights on
// commands of forms, which a role does not give (RoleDefinition.Commands)
// (ACCESS-RIGHTS, «Роли, сохранённые прежней версией ML»): such a role is not
// translated, it is repaired in Studio.
func validateDroppedRights(role RoleDefinition) error {
	if len(role.Commands) > 0 {
		return fmt.Errorf("role %s gives rights on %d commands of forms, which a role does not give: whom a command of a form is available to is the command's own use; open the role in Studio and take away the rights it cannot keep («Убрать недоступные права»)",
			role.Name, len(role.Commands))
	}
	for _, permission := range role.Objects {
		for _, list := range []struct {
			verb   string
			rights []PermissionOperation
		}{{"gives", permission.Operations}, {"denies", permission.Denied}} {
			for _, operation := range list.rights {
				if dropped, ok := droppedRightOf(operation); ok {
					return fmt.Errorf("role %s %s the right %s, which ML does not keep: %s; open the role in Studio and take away the rights it cannot keep («Убрать недоступные права»)",
						role.Name, list.verb, operation, dropped.why)
				}
			}
		}
	}
	return nil
}

// validateRoleRestrictions checks the restrictions of one object against the
// object: the configurator offers the right restricted on it. A kind no
// export restricts is accepted and noted (restriction-without-sample), and
// so is a field the object does not have (restriction-absent-field): the
// prototype keeps one when a setting of the object takes it away, and its
// check of the configuration says nothing (ACCESS-RIGHTS.md, «Ограничение по
// полю, которого у объекта нет»).
func (catalog *Catalog) validateRoleRestrictions(role RoleDefinition, permission ObjectPermission, target rightsTarget) error {
	// A kind keeping no data is offered none of the rights restricted, so the
	// check of the right below refuses a restriction of it.
	for _, restriction := range permission.Restrictions {
		if !slices.Contains(target.rights, restriction.Right) {
			return fmt.Errorf("role %s: object %s has no right %s to restrict", role.Name, permission.Object, restriction.Right)
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
