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
				if kind, known := catalog.kindOfObject(permission.Object); !ok && known {
					return fmt.Errorf("role %s gives rights on %s: the designer gives no rights on an object of the kind",
						role.Name, catalog.noteObjectName(kind, permission.Object))
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

// RoleNotRepaired refuses a role saved by an earlier version of ML and not
// repaired since: it gives or denies a right ML no longer keeps, or gives
// rights on commands of forms, which a role does not give
// (RoleDefinition.Commands). Such a role is not translated, it is repaired in
// Studio, and every load that runs it refuses it naming the role and the
// right (ACCESS-RIGHTS, «Роли, сохранённые прежней версией ML»): «Сохранить
// данные» by Error, ML App by the fields.
type RoleNotRepaired struct {
	Role string
	// Right is the right ML does not keep by the name a role keeps it under,
	// and RightName by its name in Russian, as ПравоДоступа knows it; both
	// are empty when the role gives rights on commands of forms.
	Right     PermissionOperation
	RightName string
	// Denies says the role denies the right rather than gives it.
	Denies bool
	// Commands is how many commands of forms the role gives rights on.
	Commands int
	why      string
}

func (refusal *RoleNotRepaired) Error() string {
	const repair = "; open the role in Studio and take away the rights it cannot keep («Убрать недоступные права»)"
	return refusal.Worded(
		"role %[1]s gives rights on %[2]d commands of forms, which a role does not give: whom a command of a form is available to is the command's own use"+repair,
		"role %[1]s gives the right %[2]s, which ML does not keep: %[4]s"+repair,
		"role %[1]s denies the right %[2]s, which ML does not keep: %[4]s"+repair)
}

// Worded words the refusal for its reader, the parts of it the same in
// every wording: commands takes the role and how many commands of forms it
// gives rights on; gives and denies take, by their places, the role, the
// right by the name a role keeps it under, by its Russian name, and why ML
// does not keep it.
func (refusal *RoleNotRepaired) Worded(commands, gives, denies string) string {
	if refusal.Right == "" {
		return fmt.Sprintf(commands, refusal.Role, refusal.Commands)
	}
	format := gives
	if refusal.Denies {
		format = denies
	}
	return fmt.Sprintf(format, refusal.Role, refusal.Right, refusal.RightName, refusal.why)
}

// validateDroppedRights refuses a role saved by an earlier version of ML
// (RoleNotRepaired).
func validateDroppedRights(role RoleDefinition) error {
	if len(role.Commands) > 0 {
		return &RoleNotRepaired{Role: role.Name, Commands: len(role.Commands)}
	}
	for _, permission := range role.Objects {
		for _, list := range []struct {
			denies bool
			rights []PermissionOperation
		}{{false, permission.Operations}, {true, permission.Denied}} {
			for _, operation := range list.rights {
				if dropped, ok := droppedRightOf(operation); ok {
					return &RoleNotRepaired{Role: role.Name, Right: operation, RightName: dropped.russian, Denies: list.denies, why: dropped.why}
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
	// A cube and a table of its dimension keep data and are read, yet the
	// panel of the restriction of their window stays shut with «Чтение»
	// marked (opis of 15.10.2026, block-00/kub.md): a kind whose restriction
	// is absent is restricted on no right.
	if kind, ok := catalog.kindOfObject(permission.Object); ok && len(permission.Restrictions) > 0 && usesOf(kind).restriction == absent {
		return fmt.Errorf("role %s restricts access to the data of %s: the designer restricts no access to an object of the kind",
			role.Name, catalog.noteObjectName(kind, permission.Object))
	}
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
