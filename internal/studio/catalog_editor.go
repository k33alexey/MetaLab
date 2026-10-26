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
	// «Функциональные опции»), and ExchangePlans the exchange plans, those
	// registering its changes marked with their autoregistration (its page
	// «Обмен данными»).
	Subsystems        []MemberPlace `json:"subsystems"`
	FunctionalOptions []MemberPlace `json:"functionalOptions"`
	ExchangePlans     []MemberPlace `json:"exchangePlans"`
	// BasisFor are the objects entered on the basis of the catalog, by
	// their full names, but the catalog itself (its list «Является
	// основанием для»), and basisPlaces every object that may be.
	BasisFor    []MemberPlace `json:"basisFor"`
	basisPlaces []MemberPlace
	// Roles are the roles of the project with what each says about the
	// catalog alone, and RightsSchema the schema of its rights (its page
	// «Права»).
	Roles          []RolePlace               `json:"roles"`
	RightsSchema   metadata.PermissionSchema `json:"rightsSchema"`
	Forms          []NamedPath               `json:"forms"`
	CommandModules map[string]string         `json:"commandModules"`
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
	basis := basisPlaces(loaded, value.ID)
	roles, err := workspace.rolePlacesLocked(roleOrder(loaded), value.ID, configuration)
	if err != nil {
		return CatalogEditorSource{}, err
	}
	schema, err := objectRightsSchema(workspace.root, value.ID)
	if err != nil {
		return CatalogEditorSource{}, err
	}
	return CatalogEditorSource{
		BasisFor: members(basis), basisPlaces: basis, Roles: roles, RightsSchema: schema,
		Forms: forms, CommandModules: commandModules, Subsystems: subsystemPlaces(loaded, value.ID),
		FunctionalOptions: optionPlaces(loaded, metadata.CatalogKind, value.ID), ExchangePlans: exchangePlaces(loaded, value.ID),
		Path: relative, Revision: file.Revision, Catalog: value, TypeOffer: offer,
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
// the subsystems marked on its page «Подсистемы», the options marked on
// «Функциональные опции», the exchange plans marked on «Обмен данными»
// with their autoregistration and the objects listed «Является основанием
// для» on «Ввод на основании», and what the roles say about it on «Права» -
// none where a page is not given, which leaves those objects as they are.
type CatalogWindowValue struct {
	metadata.CatalogDefinition
	Subsystems        *[]uuid.UUID   `json:"subsystems,omitempty"`
	FunctionalOptions *[]uuid.UUID   `json:"functionalOptions,omitempty"`
	ExchangePlans     *[]MemberPlace `json:"exchangePlans,omitempty"`
	BasisFor          *[]uuid.UUID   `json:"basisFor,omitempty"`
	Roles             *[]RoleMark    `json:"roles,omitempty"`
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
	if window.ExchangePlans != nil {
		pages = append(pages, exchangeMembership(metadata.CatalogKind, opened.Catalog.ID, opened.ExchangePlans, *window.ExchangePlans))
	}
	if window.BasisFor != nil {
		pages = append(pages, basisMembership(opened.Catalog.ID, opened.basisPlaces, *window.BasisFor))
	}
	return pages
}

// SaveCatalogWindow writes the catalog and the content of the subsystems
// the options and the exchange plans its window marks, one write: a refusal of any of them
// puts back what was written before it - the roles, the subsystems, the
// options and the plans, the folder.
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
	// What the page «Права» changes in a role is checked before anything is
	// written.
	configuration, err := project.ValidateLayout(workspace.root)
	if err != nil {
		return CatalogEditorSource{}, err
	}
	var edits map[uuid.UUID]*metadata.ObjectPermission
	if window.Roles != nil {
		if edits, err = workspace.roleEditsLocked(value.ID, opened.Roles, *window.Roles, configuration); err != nil {
			return CatalogEditorSource{}, fmt.Errorf("the rights of the catalog: %w", err)
		}
	}
	// The roles go first, put back if the catalog is then refused: a
	// refusal halfway leaves neither a catalog the roles do not match nor
	// roles naming fields it has not. A role takes the change of the page
	// first, then that of the catalog's fields.
	added, removed := declaredFieldChanges(opened.Catalog, value)
	restoreRoles, err := workspace.applyRoleAutoGrantsLocked(func(role *metadata.RoleDefinition) bool {
		entry, edited := edits[role.ID]
		if edited {
			putPermission(role, value.ID, entry)
		}
		// A deleted field leaves every role, its rights and its
		// restrictions, as the prototype takes it off them (ACCESS-RIGHTS.md,
		// «Удаление реквизита, по которому стоит ограничение роли»).
		changed := role.DropFields(value.ID, removed) || edited
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
	// The subsystems, the options and the plans go after the roles and
	// before the catalog, put back with the roles if the catalog is refused.
	pages := window.memberships(opened)
	restoreMembership, err := workspace.writeMembershipLocked(pages)
	if err != nil {
		return CatalogEditorSource{}, errors.Join(fmt.Errorf("write the objects listing the catalog: %w", err), restoreRoles())
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
				markMembers(page)
			}
			opened.BasisFor = members(opened.basisPlaces)
			// The roles are read anew: the page and the fields change them,
			// and each has a revision of its own. A role that does not read
			// now stays as the window had it, and its next change is refused
			// as one of a role changed on disk.
			var order []uuid.UUID
			for _, place := range opened.Roles {
				order = append(order, place.Role.ID)
			}
			if roles, err := workspace.rolePlacesLocked(order, value.ID, configuration); err == nil {
				opened.Roles = roles
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

// CreateCatalog makes the next catalog by the one way every object is made
// (CreateObject) and opens it as any catalog is read.
func (workspace *Workspace) CreateCatalog(name string) (CatalogEditorSource, error) {
	return createObjectAs(workspace, metadata.CatalogKind, name, nil, workspace.readCatalogEditorLocked)
}

// DeleteCatalog permanently removes one catalog's whole per-object folder
// (its object.yaml plus any nested modules/forms), after confirming the
// caller still has the latest revision. Reference existence (other objects
// pointing at this catalog) is not checked here — same as SaveCatalogEditor,
// that is caught at the next full project Load, not at delete time.
func (workspace *Workspace) DeleteCatalog(relative, expectedRevision string) error {
	return deleteObjectFolder(workspace, metadata.CatalogKind, relative, expectedRevision, workspace.readCatalogEditorLocked)
}

// windowSource is what a window of an object reads: the object's
// identifier and the revision it was read at.
type windowSource interface {
	identity() (uuid.UUID, string)
}

func (source CatalogEditorSource) identity() (uuid.UUID, string) {
	return source.Catalog.ID, source.Revision
}
func (source ConstantEditorSource) identity() (uuid.UUID, string) {
	return source.Constant.ID, source.Revision
}
func (source PictureEditorSource) identity() (uuid.UUID, string) {
	return source.Picture.ID, source.Revision
}
func (source TemplateEditorSource) identity() (uuid.UUID, string) {
	return source.Template.ID, source.Revision
}

// deleteObjectFolder deletes an object of a kind that keeps a folder named
// after it - its description, modules, forms, content - by the one order of
// steps of every deletion (deleteObjectLocked), while the caller holds the
// revision read last. read reads the object as its window does.
func deleteObjectFolder[Window windowSource](workspace *Workspace, kind metadata.Kind, relative, expectedRevision string, read func(string) (Window, error)) error {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	opened, err := read(relative)
	if err != nil {
		return err
	}
	id, revision := opened.identity()
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
