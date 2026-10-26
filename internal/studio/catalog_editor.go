package studio

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/k33alexey/MetaLab/internal/kinds"
	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// CatalogEditorSource is the typed, revision-safe representation the
// visual catalog editor reads and writes, in place of raw YAML.
type CatalogEditorSource struct {
	Path            string                     `json:"path"`
	Revision        string                     `json:"revision"`
	Catalog         metadata.CatalogDefinition `json:"catalog"`
	TypeOffer       metadata.TypeOffer         `json:"typeOffer"`
	Languages       []project.Language         `json:"languages"`
	DefaultLanguage string                     `json:"defaultLanguage"`
	// KindTitle names one object of the kind in the window, from the
	// descriptor of the kind.
	KindTitle string `json:"kindTitle"`
	// Pages are the pages of the window, the kind's (kinds.WindowPages), and
	// WindowRows those of them built of the palette's rows.
	Pages      []kinds.Page                 `json:"pages"`
	WindowRows map[string][]kinds.WindowRow `json:"windowRows"`
	// Palette is the tabs of the palette of the catalog and their rows, the
	// kind's (kinds.Palette).
	Palette []kinds.PaletteTab `json:"palette"`
	// PartPalettes are the palettes of its parts: an attribute, a table part.
	PartPalettes map[string][]kinds.PaletteTab `json:"partPalettes"`
	// Choices are what the windows «Выбор объекта» of the catalog offer, by
	// the property they choose: owners - the objects of the five kinds that
	// may own it, basedOn - those of the eight that may be its basis.
	Choices map[string][]ObjectChoiceBranch `json:"choices"`
	// Forms are the forms the catalog keeps, with where each is described,
	// and CommandModules where the module of each command lies, by its
	// name: the window opens them.
	// Subsystems are the subsystems of the project, those the catalog is in
	// marked (its page «Подсистемы»), and FunctionalOptions the options of
	// the project, those whose content lists it marked (its page
	// «Функциональные опции»).
	Subsystems        []MemberPlace     `json:"subsystems"`
	FunctionalOptions []MemberPlace     `json:"functionalOptions"`
	Forms             []NamedPath       `json:"forms"`
	CommandModules    map[string]string `json:"commandModules"`
}

// NamedPath is a part of an object kept in a file of its own: its name and
// where the file lies.
type NamedPath struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func validateCatalogEditorPath(relative string) error {
	return validateObjectEditorPath(relative, metadata.CatalogKind)
}

// validateObjectEditorPath accepts the description of an object of a kind
// that keeps a folder named after it: metadata/<kind>/<Имя>/object.yaml.
func validateObjectEditorPath(relative string, kind metadata.Kind) error {
	canonical, language, err := validateEditablePath(relative)
	if err != nil || language != "yaml" {
		return ErrInvalidSourcePath
	}
	parts := strings.Split(canonical, "/")
	if len(parts) != 4 || parts[0] != "metadata" || parts[1] != string(kind) || parts[3] != project.ObjectMetadataFile {
		return ErrInvalidSourcePath
	}
	return nil
}

func (workspace *Workspace) ReadCatalogEditor(relative string) (CatalogEditorSource, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	return workspace.readCatalogEditorLocked(relative)
}

func (workspace *Workspace) readCatalogEditorLocked(relative string) (CatalogEditorSource, error) {
	if err := validateCatalogEditorPath(relative); err != nil {
		return CatalogEditorSource{}, err
	}
	file, err := workspace.readSource(relative)
	if err != nil {
		return CatalogEditorSource{}, err
	}
	configuration, err := project.ValidateLayout(workspace.root)
	if err != nil {
		return CatalogEditorSource{}, err
	}
	value, err := metadata.DecodeCatalog(relative, strings.NewReader(file.Content), configuration)
	if err != nil {
		return CatalogEditorSource{}, err
	}
	// What the window «Редактирование типа данных» offers for an attribute,
	// a field the database stores, and the owners the window of the owners
	// offers, the model says with the objects of the project.
	loaded, err := metadata.Load(workspace.root)
	if err != nil {
		return CatalogEditorSource{}, err
	}
	offer, _ := loaded.TypeOffer(metadata.StoredFieldChoice)
	var forms []NamedPath
	for _, name := range loaded.ObjectFormNames(metadata.CatalogKind, value.Name) {
		if form, err := project.ObjectFormPath(string(metadata.CatalogKind), value.Name, name); err == nil {
			forms = append(forms, NamedPath{Name: name, Path: form})
		}
	}
	commandModules := map[string]string{}
	for _, command := range value.Commands {
		if module, err := project.ObjectCommandModulePath(string(metadata.CatalogKind), value.Name, command.Name); err == nil {
			commandModules[command.Name] = module
		}
	}
	return CatalogEditorSource{
		Forms: forms, CommandModules: commandModules, Subsystems: subsystemPlaces(loaded, value.ID),
		FunctionalOptions: optionPlaces(loaded, metadata.CatalogKind, value.ID),
		Path:              relative, Revision: file.Revision, Catalog: value, TypeOffer: offer,
		Choices: map[string][]ObjectChoiceBranch{
			"owners": objectChoiceTree(loaded, metadata.CatalogOwnerKinds()), "basedOn": objectChoiceTree(loaded, metadata.BasisKinds()),
		},
		Languages: editorLanguages(configuration), DefaultLanguage: configuration.DefaultLanguage,
		KindTitle: kinds.Must(metadata.CatalogKind).Title, Pages: kinds.Must(metadata.CatalogKind).WindowPages(), WindowRows: kinds.Must(metadata.CatalogKind).WindowRows(),
		Palette:      kinds.Must(metadata.CatalogKind).Palette(),
		PartPalettes: map[string][]kinds.PaletteTab{"attribute": kinds.PartPalette("attribute"), "tablePart": kinds.PartPalette("tablePart")},
	}, nil
}

// SaveCatalogEditor validates and writes an edited catalog. Reference
// existence (enumeration/catalog/document ids used by composite attribute
// types) is checked the same way raw YAML edits are today — at the next
// full project Load, not here — so this is not a regression versus the
// plain text editor it replaces.
func (workspace *Workspace) SaveCatalogEditor(relative string, value metadata.CatalogDefinition, revision string) (CatalogEditorSource, error) {
	return workspace.SaveCatalogWindow(relative, CatalogWindowValue{CatalogDefinition: value}, revision)
}

// CatalogWindowValue is what the window of a catalog writes: the catalog,
// the subsystems marked on its page «Подсистемы» and the options marked on
// «Функциональные опции» - none where a page is not given, which leaves
// those objects as they are.
type CatalogWindowValue struct {
	metadata.CatalogDefinition
	Subsystems        *[]uuid.UUID `json:"subsystems,omitempty"`
	FunctionalOptions *[]uuid.UUID `json:"functionalOptions,omitempty"`
}

// memberships are the pages of the window given, each with the objects it
// marks.
func (window CatalogWindowValue) memberships(opened CatalogEditorSource) []membership {
	var pages []membership
	if window.Subsystems != nil {
		pages = append(pages, subsystemMembership(opened.Catalog.ID, opened.Subsystems, *window.Subsystems))
	}
	if window.FunctionalOptions != nil {
		pages = append(pages, optionMembership(metadata.CatalogKind, opened.Catalog.ID, opened.FunctionalOptions, *window.FunctionalOptions))
	}
	return pages
}

// SaveCatalogWindow writes the catalog and the content of the subsystems
// and the options its window marks, one write: a refusal of any of them
// puts back what was written before it - the roles, the subsystems and the
// options, the folder.
func (workspace *Workspace) SaveCatalogWindow(relative string, window CatalogWindowValue, revision string) (CatalogEditorSource, error) {
	value := window.CatalogDefinition
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	opened, err := workspace.readCatalogEditorLocked(relative)
	if err != nil {
		return CatalogEditorSource{}, err
	}
	if opened.Revision != revision {
		return CatalogEditorSource{}, ErrSourceChanged
	}
	if value.ID != opened.Catalog.ID {
		return CatalogEditorSource{}, fmt.Errorf("catalog UUID cannot be changed")
	}
	if value.Name != opened.Catalog.Name {
		if err := workspace.objectNameErrorLocked(relative, "", value.Name); err != nil {
			return CatalogEditorSource{}, err
		}
	}
	var encoded bytes.Buffer
	if err := metadata.Encode(&encoded, value); err != nil {
		return CatalogEditorSource{}, err
	}
	// The roles go first, put back if the catalog is then refused: a
	// refusal halfway leaves neither a catalog the roles do not match nor
	// roles naming fields it has not.
	added, removed := declaredFieldChanges(opened.Catalog, value)
	restoreRoles, err := workspace.applyRoleAutoGrantsLocked(func(role *metadata.RoleDefinition) bool {
		// A deleted field leaves every role, its rights and its
		// restrictions, as the prototype takes it off them (ACCESS-RIGHTS.md,
		// «Удаление реквизита, по которому стоит ограничение роли»).
		changed := role.DropFields(value.ID, removed)
		if !role.GrantNewFieldsByDefault {
			return changed
		}
		for _, key := range added {
			// A line number is not a part a right is given to.
			if _, err := uuid.Parse(key); err == nil && role.GrantFieldReadByDefault(value.ID, key) {
				changed = true
			}
		}
		return changed
	})
	if err != nil {
		return CatalogEditorSource{}, fmt.Errorf("apply the catalog's fields to the roles: %w", err)
	}
	// The subsystems and the options go after the roles and before the
	// catalog, put back with the roles if the catalog is refused.
	pages := window.memberships(opened)
	restoreMembership, err := workspace.writeMembershipLocked(pages)
	if err != nil {
		return CatalogEditorSource{}, errors.Join(fmt.Errorf("write the catalog's subsystems and options: %w", err), restoreRoles())
	}
	// The catalog keeps a folder named after it, so a new name moves the
	// folder - its forms and modules with it - and the description is
	// written there; a refusal puts the folder back.
	written, moveBack, err := workspace.followNameLocked(relative, opened.Catalog.Name, value.Name)
	if err == nil {
		var saved SourceFile
		if saved, err = workspace.saveSourceLocked(written, encoded.String(), revision); err == nil {
			opened.Path, opened.Catalog, opened.Revision = written, value, saved.Revision
			for _, page := range pages {
				markMembers(page.places, page.marked)
			}
			return opened, nil
		}
		err = errors.Join(err, moveBack())
	}
	return CatalogEditorSource{}, errors.Join(err, restoreMembership(), restoreRoles())
}

// followNameLocked moves the folder of the object described at relative from
// its name before to its name after, and answers where its description lies
// then and how to move it back. An object of no folder of its own, and an
// unchanged name, stay where they are.
func (workspace *Workspace) followNameLocked(relative, before, after string) (string, func() error, error) {
	stay := func() error { return nil }
	kind, name, rest, ok := project.SplitObjectPath(relative)
	if !ok || before == after || name != before {
		return relative, stay, nil
	}
	moved, err := project.ObjectMetadataPath(kind, after)
	if err != nil || len(rest) != 1 {
		return relative, stay, err
	}
	from := filepath.Join(workspace.root, filepath.FromSlash(path.Dir(relative)))
	to := filepath.Join(workspace.root, filepath.FromSlash(path.Dir(moved)))
	if err := renameFolder(from, to); err != nil {
		return relative, stay, err
	}
	workspace.invalidateStudioIndexesLocked()
	return moved, func() error {
		workspace.invalidateStudioIndexesLocked()
		return renameFolder(to, from)
	}, nil
}

// declaredFieldChanges is the keys of the fields the catalog declares after
// the edit and not before - an attribute added to a table part added earlier
// counts too, since that attribute alone is separately grantable - and the
// keys of those it declared before and not after.
func declaredFieldChanges(before, after metadata.CatalogDefinition) (added, removed []string) {
	was := metadata.DeclaredFieldKeys(before.Attributes, before.TableParts)
	is := metadata.DeclaredFieldKeys(after.Attributes, after.TableParts)
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

func (workspace *Workspace) CreateCatalog(name string) (CatalogEditorSource, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	configuration, err := project.ValidateLayout(workspace.root)
	if err != nil {
		return CatalogEditorSource{}, err
	}
	id, err := uuid.New()
	if err != nil {
		return CatalogEditorSource{}, err
	}
	// «Добавить» names nothing: the object is the kind's next number.
	if name == "" {
		if name, err = workspace.newObjectNameLocked(string(metadata.CatalogKind)); err != nil {
			return CatalogEditorSource{}, err
		}
	}
	if err := workspace.objectNameErrorLocked("", branchOf(string(metadata.CatalogKind)), name); err != nil {
		return CatalogEditorSource{}, err
	}
	// A new object has no synonym: the prototype's new object stands with an
	// empty one (block-00, konstanta.md, snapshot novaya-osnovnye), its name
	// shown in its place, and the synonym follows the name typed after.
	value := metadata.CatalogDefinition{
		Format: metadata.CurrentFormat, ID: id, Name: name,
		Code: metadata.CatalogCode{Type: metadata.StringType, Length: 9, Auto: true, Unique: true},
		// The prototype's new catalog: a code of 9, a description of 25
		// (block-00, spravochnik.md, «Умолчания»).
		DescriptionLength: 25,
	}
	var encoded bytes.Buffer
	if err := metadata.Encode(&encoded, value); err != nil {
		return CatalogEditorSource{}, err
	}
	if _, err := metadata.DecodeCatalog("new catalog", bytes.NewReader(encoded.Bytes()), configuration); err != nil {
		return CatalogEditorSource{}, err
	}
	relative, err := workspace.createObjectLocked(metadata.CatalogKind, id, name, encoded.Bytes())
	if err != nil {
		return CatalogEditorSource{}, err
	}
	// The new catalog opens as any catalog is read: one way to its window.
	return workspace.readCatalogEditorLocked(relative)
}

// createObjectLocked writes the description of a new object of a kind that
// keeps a folder named after it, puts the object last among the objects of
// its kind, and gives it to the roles that are given every new object. A
// refusal on the way takes the object away again: no folder, no line of the
// order and no right of a role is left of it. Answers where the description
// lies.
func (workspace *Workspace) createObjectLocked(kind metadata.Kind, id uuid.UUID, name string, encoded []byte) (string, error) {
	relative, err := project.ObjectMetadataPath(string(kind), name)
	if err != nil {
		return "", err
	}
	directory := filepath.Join(workspace.root, "metadata", string(kind))
	if err := os.Mkdir(directory, 0o755); err != nil && !os.IsExist(err) {
		return "", err
	}
	info, err := os.Lstat(directory)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", ErrInvalidSourcePath
	}
	objectDirectory := filepath.Join(directory, name)
	if err := os.Mkdir(objectDirectory, 0o755); err != nil {
		return "", err
	}
	description := filepath.Join(workspace.root, filepath.FromSlash(relative))
	takeAway := func() {
		_ = os.Remove(description)
		_ = os.Remove(objectDirectory)
		workspace.invalidateStudioIndexesLocked()
	}
	file, err := os.OpenFile(description, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		_ = os.Remove(objectDirectory)
		return "", err
	}
	_, writeErr := file.Write(encoded)
	if writeErr == nil {
		writeErr = file.Sync()
	}
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		takeAway()
		return "", fmt.Errorf("create %s: write=%v, close=%v", kind, writeErr, closeErr)
	}
	workspace.invalidateStudioIndexesLocked()
	// The new object stands last among the objects of its kind; an order
	// that cannot be written takes the object away again, as a refused
	// grant does.
	restoreOrder, err := workspace.placeInOrderLocked(kind, id)
	if err != nil {
		takeAway()
		return "", fmt.Errorf("place new %s in the order: %w", kind, err)
	}
	// A refused grant takes the new object away again: the roles are left as
	// they were, the order is put back, and an object no role was told of is
	// not left behind.
	if _, err := workspace.applyRoleAutoGrantsLocked(func(role *metadata.RoleDefinition) bool {
		if !role.GrantNewObjectsByDefault {
			return false
		}
		return role.GrantObjectReadByDefault(id)
	}); err != nil {
		restoreErr := restoreOrder()
		takeAway()
		if restoreErr != nil {
			return "", fmt.Errorf("grant default access to new %s: %w; restoring the order failed: %v", kind, err, restoreErr)
		}
		return "", fmt.Errorf("grant default access to new %s: %w", kind, err)
	}
	return relative, nil
}

// DeleteCatalog permanently removes one catalog's whole per-object folder
// (its object.yaml plus any nested modules/forms), after confirming the
// caller still has the latest revision. Reference existence (other objects
// pointing at this catalog) is not checked here — same as SaveCatalogEditor,
// that is caught at the next full project Load, not at delete time.
func (workspace *Workspace) DeleteCatalog(relative, expectedRevision string) error {
	return workspace.deleteObjectFolder(metadata.CatalogKind, relative, expectedRevision, func() (uuid.UUID, string, error) {
		opened, err := workspace.readCatalogEditorLocked(relative)
		return opened.Catalog.ID, opened.Revision, err
	})
}

// deleteObjectFolder deletes an object of a kind that keeps a folder named
// after it - its description, modules, forms - by the one order of steps of
// every deletion (deleteObjectLocked), while the caller holds the revision
// read last. read reads the object as its window does: its identifier and
// revision.
func (workspace *Workspace) deleteObjectFolder(kind metadata.Kind, relative, expectedRevision string, read func() (uuid.UUID, string, error)) error {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	id, revision, err := read()
	if err != nil {
		return err
	}
	if revision != expectedRevision {
		return ErrSourceChanged
	}
	return workspace.deleteObjectLocked(kind, id, relative, func() error {
		return os.RemoveAll(filepath.Join(workspace.root, filepath.FromSlash(path.Dir(relative))))
	})
}

// ownerRefusals are the prototype's words at «ОК» of the window choosing the
// owners of a catalog (docs/block-00/spravochnik.md, «Проверки»; snapshots
// vladelcy-otkaz-*): the one of a catalog with predefined items, and the one
// of an owner that is the catalog itself or closes a ring, naming the owner.
var ownerRefusals = map[metadata.OwnerFault]string{
	metadata.OwnerOfPredefined: "Справочнику, имеющему предопределенные данные, нельзя назначить владельца!",
	metadata.OwnerItself:       "Данный справочник не может быть установлен в качестве владельца! (%s)",
	metadata.OwnerInRing:       "Данный справочник не может быть установлен в качестве владельца! (%s)",
}

// CatalogOwnersRefusal answers the prototype's refusal of the owners the
// window of the catalog at relative holds - catalog, unsaved - against the
// owners of every other catalog of the project, or nothing.
func (workspace *Workspace) CatalogOwnersRefusal(relative string, catalog metadata.CatalogDefinition) (NameAnswer, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	if err := validateCatalogEditorPath(relative); err != nil {
		return NameAnswer{}, err
	}
	loaded, err := metadata.Load(workspace.root)
	if err != nil {
		return NameAnswer{}, err
	}
	fault, owner := loaded.OwnersFault(catalog)
	if fault == metadata.OwnerFits {
		return NameAnswer{}, nil
	}
	if strings.Contains(ownerRefusals[fault], "%s") {
		return NameAnswer{Refusal: fmt.Sprintf(ownerRefusals[fault], owner)}, nil
	}
	return NameAnswer{Refusal: ownerRefusals[fault]}, nil
}

// catalogEditorAddress is where the window of a catalog is read, written,
// made and deleted.
const catalogEditorAddress = "/api/catalog"

func registerCatalogEditorRoutes(routes *http.ServeMux, workspace *Workspace) {
	routes.HandleFunc("GET "+catalogEditorAddress, answerQuery(writeSourceError, func(request *http.Request) (any, error) {
		return workspace.ReadCatalogEditor(request.URL.Query().Get("path"))
	}))
	registerObjectWindow(routes, catalogEditorAddress, workspace.SaveCatalogWindow, workspace.CreateCatalog, workspace.DeleteCatalog)
	routes.HandleFunc("POST "+catalogEditorAddress+"/owners", answerStudio(0, writeSourceError, func(input struct {
		Path   string                     `json:"path"`
		Object metadata.CatalogDefinition `json:"object"`
	}) (any, error) {
		return workspace.CatalogOwnersRefusal(input.Path, input.Object)
	}))
}
