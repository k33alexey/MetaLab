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
	// Pages are the pages of the window, the kind's (kinds.WindowPages).
	Pages []kinds.Page `json:"pages"`
	// OwnerChoice is what the window choosing the owners of the catalog
	// offers: the objects of the five kinds that may own it.
	OwnerChoice []ObjectChoiceBranch `json:"ownerChoice"`
}

func validateCatalogEditorPath(relative string) error {
	canonical, language, err := validateEditablePath(relative)
	if err != nil || language != "yaml" {
		return ErrInvalidSourcePath
	}
	parts := strings.Split(canonical, "/")
	if len(parts) != 4 || parts[0] != "metadata" || parts[1] != string(metadata.CatalogKind) || parts[3] != project.ObjectMetadataFile {
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
	return CatalogEditorSource{
		Path: relative, Revision: file.Revision, Catalog: value, TypeOffer: offer,
		OwnerChoice: objectChoiceTree(loaded, metadata.CatalogOwnerKinds()),
		Languages:   editorLanguages(configuration), DefaultLanguage: configuration.DefaultLanguage,
		KindTitle: kinds.Must(metadata.CatalogKind).Title, Pages: kinds.Must(metadata.CatalogKind).WindowPages(),
	}, nil
}

// SaveCatalogEditor validates and writes an edited catalog. Reference
// existence (enumeration/catalog/document ids used by composite attribute
// types) is checked the same way raw YAML edits are today — at the next
// full project Load, not here — so this is not a regression versus the
// plain text editor it replaces.
func (workspace *Workspace) SaveCatalogEditor(relative string, value metadata.CatalogDefinition, revision string) (CatalogEditorSource, error) {
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
	// The catalog keeps a folder named after it, so a new name moves the
	// folder - its forms and modules with it - and the description is
	// written there; a refusal puts the folder back.
	written, moveBack, err := workspace.followNameLocked(relative, opened.Catalog.Name, value.Name)
	if err == nil {
		var saved SourceFile
		if saved, err = workspace.saveSourceLocked(written, encoded.String(), revision); err == nil {
			opened.Path, opened.Catalog, opened.Revision = written, value, saved.Revision
			return opened, nil
		}
		err = errors.Join(err, moveBack())
	}
	return CatalogEditorSource{}, errors.Join(err, restoreRoles())
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
	relative, err := project.ObjectMetadataPath(string(metadata.CatalogKind), name)
	if err != nil {
		return CatalogEditorSource{}, err
	}
	directory := filepath.Join(workspace.root, "metadata", string(metadata.CatalogKind))
	if err := os.Mkdir(directory, 0o755); err != nil && !os.IsExist(err) {
		return CatalogEditorSource{}, err
	}
	info, err := os.Lstat(directory)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return CatalogEditorSource{}, ErrInvalidSourcePath
	}
	objectDirectory := filepath.Join(directory, name)
	if err := os.Mkdir(objectDirectory, 0o755); err != nil {
		return CatalogEditorSource{}, err
	}
	file, err := os.OpenFile(filepath.Join(workspace.root, filepath.FromSlash(relative)), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		_ = os.Remove(objectDirectory)
		return CatalogEditorSource{}, err
	}
	_, writeErr := file.Write(encoded.Bytes())
	if writeErr == nil {
		writeErr = file.Sync()
	}
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(filepath.Join(workspace.root, filepath.FromSlash(relative)))
		_ = os.Remove(objectDirectory)
		return CatalogEditorSource{}, fmt.Errorf("create catalog: write=%v, close=%v", writeErr, closeErr)
	}
	workspace.invalidateStudioIndexesLocked()
	// The new catalog stands last among the catalogs; an order that cannot be
	// written takes the catalog away again, as a refused grant does.
	restoreOrder, err := workspace.placeInOrderLocked(metadata.CatalogKind, id)
	if err != nil {
		_ = os.Remove(filepath.Join(workspace.root, filepath.FromSlash(relative)))
		_ = os.Remove(objectDirectory)
		workspace.invalidateStudioIndexesLocked()
		return CatalogEditorSource{}, fmt.Errorf("place new catalog in the order: %w", err)
	}
	// A refused grant takes the new catalog away again: the roles are left as
	// they were, the order is put back, and a catalog no role was told of is
	// not left behind.
	if _, err := workspace.applyRoleAutoGrantsLocked(func(role *metadata.RoleDefinition) bool {
		if !role.GrantNewObjectsByDefault {
			return false
		}
		return role.GrantObjectReadByDefault(id)
	}); err != nil {
		restoreErr := restoreOrder()
		_ = os.Remove(filepath.Join(workspace.root, filepath.FromSlash(relative)))
		_ = os.Remove(objectDirectory)
		workspace.invalidateStudioIndexesLocked()
		if restoreErr != nil {
			return CatalogEditorSource{}, fmt.Errorf("grant default access to new catalog: %w; restoring the order failed: %v", err, restoreErr)
		}
		return CatalogEditorSource{}, fmt.Errorf("grant default access to new catalog: %w", err)
	}
	// The new catalog opens as any catalog is read: one way to its window.
	return workspace.readCatalogEditorLocked(relative)
}

// DeleteCatalog permanently removes one catalog's whole per-object folder
// (its object.yaml plus any nested modules/forms), after confirming the
// caller still has the latest revision. Reference existence (other objects
// pointing at this catalog) is not checked here — same as SaveCatalogEditor,
// that is caught at the next full project Load, not at delete time.
func (workspace *Workspace) DeleteCatalog(relative, expectedRevision string) error {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	opened, err := workspace.readCatalogEditorLocked(relative)
	if err != nil {
		return err
	}
	if opened.Revision != expectedRevision {
		return ErrSourceChanged
	}
	return workspace.deleteObjectLocked(metadata.CatalogKind, opened.Catalog.ID, relative, func() error {
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
	registerObjectWindow(routes, catalogEditorAddress, workspace.SaveCatalogEditor, workspace.CreateCatalog, workspace.DeleteCatalog)
	routes.HandleFunc("POST "+catalogEditorAddress+"/owners", answerStudio(0, writeSourceError, func(input struct {
		Path   string                     `json:"path"`
		Object metadata.CatalogDefinition `json:"object"`
	}) (any, error) {
		return workspace.CatalogOwnersRefusal(input.Path, input.Object)
	}))
}
