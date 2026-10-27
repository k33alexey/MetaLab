package studio

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/k33alexey/MetaLab/internal/kinds"
	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// windowHead is what the window of an object reads beside the object: the
// head of its palette, the subsystems of the project, those holding the
// object marked (its page «Подсистемы»), and what the other pages the
// kind's window has show of the objects around it (readObjectPages) -
// none of a page the window has not.
type windowHead struct {
	objectHead
	Subsystems []MemberPlace `json:"subsystems"`
	// FunctionalOptions are the options of the project marked where the
	// object is in their content - the page «Функциональные опции».
	FunctionalOptions []MemberPlace `json:"functionalOptions,omitempty"`
	// ExchangePlans are the exchange plans, those registering the changes
	// of the object marked with their autoregistration - «Обмен данными».
	ExchangePlans []MemberPlace `json:"exchangePlans,omitempty"`
	// BasisFor are the objects entered on the basis of the object, by their
	// full names, but the object itself - the list «Является основанием
	// для» of «Ввод на основании» - and basisPlaces every object that may be.
	BasisFor    []MemberPlace `json:"basisFor,omitempty"`
	basisPlaces []MemberPlace
	// Roles are the roles of the project with what each says about the
	// object alone, and RightsSchema the schema of its rights - «Права».
	Roles        []RolePlace                `json:"roles,omitempty"`
	RightsSchema *metadata.PermissionSchema `json:"rightsSchema,omitempty"`
	// Forms are the forms the object keeps with where each is described -
	// «Формы» - and CommandModules where the module of each of its
	// commands lies, by its name - «Команды»: the window opens them.
	Forms          []NamedPath       `json:"forms,omitempty"`
	CommandModules map[string]string `json:"commandModules,omitempty"`
}

// objectShape is what the window of an object knows of an object of its
// kind beyond the kind's descriptor: its identifier and name and, of a
// kind whose window has the pages, the names of its commands, the keys of
// the fields it declares - a right is given to each - and its templates.
// A kind without them leaves them nil.
type objectShape[T any] struct {
	named     func(T) (uuid.UUID, string)
	commands  func(T) []string
	fields    func(T) []string
	templates func(T) []metadata.ObjectTemplate
}

// referenceShape is the shape of an object of a kind stored as a reference
// with parts - a catalog, an exchange plan: named names it, parts answers
// its commands, attributes, table parts and templates.
func referenceShape[T any](named func(T) (uuid.UUID, string),
	parts func(T) ([]metadata.ObjectCommand, []metadata.Attribute, []metadata.TablePart, []metadata.ObjectTemplate)) objectShape[T] {
	return objectShape[T]{
		named: named,
		commands: func(value T) []string {
			commands, _, _, _ := parts(value)
			names := make([]string, 0, len(commands))
			for _, command := range commands {
				names = append(names, command.Name)
			}
			return names
		},
		fields: func(value T) []string {
			_, attributes, tableParts, _ := parts(value)
			return metadata.DeclaredFieldKeys(attributes, tableParts)
		},
		templates: func(value T) []metadata.ObjectTemplate {
			_, _, _, templates := parts(value)
			return templates
		},
	}
}

// windowMarks are what the pages of a window that edit other objects mark,
// of an object of Kind: «Подсистемы», «Функциональные опции», «Обмен
// данными», «Является основанием для» of «Ввод на основании», «Права» -
// none where the page was not shown - and the contents loaded from files
// into templates the window added, by their identifiers.
type windowMarks struct {
	Subsystems, FunctionalOptions *[]uuid.UUID
	ExchangePlans                 *[]MemberPlace
	BasisFor                      *[]uuid.UUID
	Roles                         *[]RoleMark
	TemplateContents              []TemplateContent
	Kind                          metadata.Kind
}

// memberships are the pages of the window given, each with the objects it
// marks, for the object id whose window read head.
func (marks windowMarks) memberships(id uuid.UUID, head windowHead) []membership {
	var pages []membership
	if marks.Subsystems != nil {
		pages = append(pages, subsystemMembership(id, head.Subsystems, *marks.Subsystems))
	}
	if marks.FunctionalOptions != nil {
		pages = append(pages, optionMembership(marks.Kind, id, head.FunctionalOptions, *marks.FunctionalOptions))
	}
	if marks.ExchangePlans != nil {
		pages = append(pages, exchangeMembership(marks.Kind, id, head.ExchangePlans, *marks.ExchangePlans))
	}
	if marks.BasisFor != nil {
		pages = append(pages, basisMembership(id, head.basisPlaces, *marks.BasisFor))
	}
	return pages
}

// readObjectWindow reads the description of an object of a kind that has a
// window (readObjectDescription) with the subsystems holding it and what
// the other pages of the kind's window show (readObjectPages), and
// answers the project loaded for what else the window reads of it.
func readObjectWindow[T any](workspace *Workspace, kind metadata.Kind, relative string,
	decode func(string, io.Reader, project.Project) (T, error), shape objectShape[T]) (T, windowHead, *metadata.Catalog, error) {
	value, head, err := readObjectDescription(workspace, kind, relative, decode)
	if err != nil {
		return value, windowHead{}, nil, err
	}
	loaded, err := metadata.Load(workspace.root)
	if err != nil {
		return value, windowHead{}, nil, err
	}
	id, _ := shape.named(value)
	window := windowHead{objectHead: head, Subsystems: subsystemPlaces(loaded, id)}
	if err := readObjectPages(workspace, &window, loaded, kind, value, shape); err != nil {
		return value, windowHead{}, nil, err
	}
	return value, window, loaded, nil
}

// readObjectPages reads what the pages the window of an object of kind
// has show of the objects around it (kinds.WindowPages): the options, the
// plans, the bases, the roles, the forms, the modules of the commands.
func readObjectPages[T any](workspace *Workspace, head *windowHead, loaded *metadata.Catalog, kind metadata.Kind, value T, shape objectShape[T]) error {
	id, name := shape.named(value)
	for _, page := range kinds.Must(kind).WindowPages() {
		switch page.Key {
		case "functional-options":
			head.FunctionalOptions = optionPlaces(loaded, kind, id)
		case "exchange":
			head.ExchangePlans = exchangePlaces(loaded, id)
		case "based-on":
			head.basisPlaces = basisPlaces(loaded, id)
			head.BasisFor = members(head.basisPlaces)
		case "rights":
			configuration, err := project.ValidateLayout(workspace.root)
			if err != nil {
				return err
			}
			if head.Roles, err = workspace.rolePlacesLocked(roleOrder(loaded), id, configuration); err != nil {
				return err
			}
			schema, err := objectRightsSchema(workspace.root, id)
			if err != nil {
				return err
			}
			head.RightsSchema = &schema
		case "forms":
			for _, form := range loaded.ObjectFormNames(kind, name) {
				if path, err := project.ObjectFormPath(string(kind), name, form); err == nil {
					head.Forms = append(head.Forms, NamedPath{Name: form, Path: path})
				}
			}
		case "commands":
			head.CommandModules = map[string]string{}
			for _, command := range shape.commands(value) {
				if module, err := project.ObjectCommandModulePath(string(kind), name, command); err == nil {
					head.CommandModules[command] = module
				}
			}
		}
	}
	return nil
}

// paletteSource is what the reading of the palette of an object of a kind
// without a window answers: the object with the head of its palette, and
// how an object of its kind is named.
type paletteSource[T any] interface {
	paletteOf() (T, objectHead)
	named(T) (uuid.UUID, string)
}

// savePaletteObject writes an object its palette edited: the object read
// last at revision (read) must be the same object, a new name is checked in
// its branch and moves its folder (followNameLocked); keep, when given,
// puts back what the palette does not change - the address of a reference,
// the password of a client - or refuses the edit. Answers the object as
// the palette reads it after.
func savePaletteObject[S paletteSource[T], T any](workspace *Workspace, relative, revision string, value T,
	read func(string) (S, error), keep func(opened T, value *T) error) (S, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	var none S
	opened, err := read(relative)
	if err != nil {
		return none, err
	}
	object, head := opened.paletteOf()
	if head.Revision != revision {
		return none, ErrSourceChanged
	}
	id, name := opened.named(value)
	openedID, openedName := opened.named(object)
	if id != openedID {
		return none, fmt.Errorf("the identifier of an object cannot be changed")
	}
	if name != openedName {
		if err := workspace.objectNameErrorLocked(relative, "", name); err != nil {
			return none, err
		}
	}
	if keep != nil {
		if err := keep(object, &value); err != nil {
			return none, err
		}
	}
	var encoded bytes.Buffer
	if err := metadata.Encode(&encoded, value); err != nil {
		return none, err
	}
	written, moveBack, err := workspace.followNameLocked(relative, openedName, name)
	if err != nil {
		return none, err
	}
	if _, err := workspace.saveSourceLocked(written, encoded.String(), revision); err != nil {
		return none, errors.Join(err, moveBack())
	}
	return read(written)
}

// objectWindowSource is what the reading of the window of an object answers: the
// object with the head of its window, and how an object of its kind is
// named - by its identifier and its name.
type objectWindowSource[T any] interface {
	windowOf() (T, windowHead)
	named(T) (uuid.UUID, string)
}

// saveObjectWindowAs writes the object a window gives with what its pages
// mark (saveObjectWindow) under the lock and reads it back as the window
// reads it (read).
func saveObjectWindowAs[S objectWindowSource[T], T any](workspace *Workspace, relative, revision string, value T, marks windowMarks,
	read func(string) (S, error), shape objectShape[T]) (S, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	written, err := saveObjectWindow(workspace, relative, revision, value, marks, func(relative string) (T, windowHead, error) {
		opened, err := read(relative)
		object, head := opened.windowOf()
		return object, head, err
	}, shape)
	if err != nil {
		var none S
		return none, err
	}
	return read(written)
}

// saveObjectWindow writes an object of a window and what its pages mark as
// one save: the object read last at revision is opened by read, shape
// names an object of its kind by its identifier and name. What the page
// «Права» changes in the roles is checked before anything is written; the
// roles go first - the page's change, then the fields the object declares
// anew or no more (shape.fields) - and are put back if anything after them
// is refused; then the objects the other pages mark, the contents of new
// templates and the object itself (writeObjectWindowLocked). Answers where
// the description lies after.
func saveObjectWindow[T any](workspace *Workspace, relative, revision string, value T, marks windowMarks,
	read func(string) (T, windowHead, error), shape objectShape[T]) (string, error) {
	opened, head, err := read(relative)
	if err != nil {
		return "", err
	}
	if head.Revision != revision {
		return "", ErrSourceChanged
	}
	id, name := shape.named(value)
	openedID, openedName := shape.named(opened)
	if id != openedID {
		return "", fmt.Errorf("the identifier of an object cannot be changed")
	}
	if name != openedName {
		if err := workspace.objectNameErrorLocked(relative, "", name); err != nil {
			return "", err
		}
	}
	var encoded bytes.Buffer
	if err := metadata.Encode(&encoded, value); err != nil {
		return "", err
	}
	restoreRoles, err := roleChangesOfWindow(workspace, id, head, marks, shape, opened, value)
	if err != nil {
		return "", err
	}
	var files []newFile
	if shape.templates != nil {
		if files, err = templateContentFiles(marks.Kind, name, shape.templates(opened), shape.templates(value), marks.TemplateContents); err != nil {
			return "", errors.Join(err, restoreRoles())
		}
	}
	written, _, err := workspace.writeObjectWindowLocked(relative, openedName, name, encoded.String(), revision, marks.memberships(id, head), files)
	if err != nil {
		return "", errors.Join(err, restoreRoles())
	}
	return written, nil
}

// roleChangesOfWindowLocked writes into the roles what the page «Права» of
// the window of the object id changes (marks.Roles), checked first, then
// what the fields the object declares anew or no more change: a deleted
// field leaves every role, its rights and its restrictions, as the
// prototype takes it off them (ACCESS-RIGHTS.md, «Удаление реквизита, по
// которому стоит ограничение роли»), and a role giving new fields by
// default reads a new one. Answers how to put the roles back; nothing to
// put back for a window of neither.
func roleChangesOfWindow[T any](workspace *Workspace, id uuid.UUID, head windowHead, marks windowMarks, shape objectShape[T], opened, value T) (func() error, error) {
	if marks.Roles == nil && shape.fields == nil {
		return func() error { return nil }, nil
	}
	configuration, err := project.ValidateLayout(workspace.root)
	if err != nil {
		return nil, err
	}
	var edits map[uuid.UUID]*metadata.ObjectPermission
	if marks.Roles != nil {
		if edits, err = workspace.roleEditsLocked(id, head.Roles, *marks.Roles, configuration); err != nil {
			return nil, fmt.Errorf("the rights of the object: %w", err)
		}
	}
	var added, removed []string
	if shape.fields != nil {
		added, removed = fieldChanges(shape.fields(opened), shape.fields(value))
	}
	restore, err := workspace.applyRoleAutoGrantsLocked(func(role *metadata.RoleDefinition) bool {
		entry, edited := edits[role.ID]
		if edited {
			putPermission(role, id, entry)
		}
		changed := role.DropFields(id, removed) || edited
		if !role.GrantNewFieldsByDefault {
			return changed
		}
		for _, key := range added {
			// A line number is not a part a right is given to.
			if _, err := uuid.Parse(key); err == nil && role.GrantFieldReadByDefault(id, key) {
				changed = true
			}
		}
		return changed
	})
	if err != nil {
		return nil, fmt.Errorf("apply the object's fields to the roles: %w", err)
	}
	return restore, nil
}

// fieldChanges is the keys of the fields an object declares after an edit
// (is) and not before (was) - an attribute added to a table part added
// earlier counts too, since that attribute alone is separately grantable -
// and the keys of those it declared before and not after.
func fieldChanges(was, is []string) (added, removed []string) {
	for _, key := range is {
		if !slices.Contains(was, key) {
			added = append(added, key)
		}
	}
	for _, key := range was {
		if !slices.Contains(is, key) {
			removed = append(removed, key)
		}
	}
	return added, removed
}
