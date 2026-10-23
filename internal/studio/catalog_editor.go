package studio

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// loadTypeOffer is what the window «Редактирование типа данных» offers for
// an attribute of a catalog, a field the database stores. The model says it
// (TypeOffer), with the objects of the project its rows name, so the editor
// offers what the save accepts.
func loadTypeOffer(root string) (metadata.TypeOffer, error) {
	catalog, err := metadata.Load(root)
	if err != nil {
		return metadata.TypeOffer{}, err
	}
	offer, _ := catalog.TypeOffer(metadata.StoredFieldChoice)
	return offer, nil
}

// CatalogEditorSource is the typed, revision-safe representation the
// visual catalog editor reads and writes, in place of raw YAML.
type CatalogEditorSource struct {
	Path            string                     `json:"path"`
	Revision        string                     `json:"revision"`
	Catalog         metadata.CatalogDefinition `json:"catalog"`
	TypeOffer       metadata.TypeOffer         `json:"typeOffer"`
	Languages       []project.Language         `json:"languages"`
	DefaultLanguage string                     `json:"defaultLanguage"`
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
	offer, err := loadTypeOffer(workspace.root)
	if err != nil {
		return CatalogEditorSource{}, err
	}
	return CatalogEditorSource{
		Path: relative, Revision: file.Revision, Catalog: value, TypeOffer: offer,
		Languages: editorLanguages(configuration), DefaultLanguage: configuration.DefaultLanguage,
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
	saved, err := workspace.saveSourceLocked(relative, encoded.String(), revision)
	if err != nil {
		if restoreErr := restoreRoles(); restoreErr != nil {
			return CatalogEditorSource{}, fmt.Errorf("%w; %v", err, restoreErr)
		}
		return CatalogEditorSource{}, err
	}
	opened.Catalog, opened.Revision = value, saved.Revision
	return opened, nil
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

func (workspace *Workspace) checkCatalogNameLocked(value metadata.CatalogDefinition) error {
	entries, err := os.ReadDir(filepath.Join(workspace.root, "metadata", string(metadata.CatalogKind)))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	configuration, err := project.ValidateLayout(workspace.root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		// The folder is named by the object, so the object's own folder is the
		// one bearing its name - that is the one a rename is allowed to keep.
		if !entry.IsDir() || entry.Name() == value.Name {
			continue
		}
		file, err := workspace.readSource("metadata/catalogs/" + entry.Name() + "/object.yaml")
		if err != nil {
			return err
		}
		other, err := metadata.DecodeCatalog(file.Path, strings.NewReader(file.Content), configuration)
		if err != nil {
			return err
		}
		if strings.EqualFold(other.Name, value.Name) {
			return fmt.Errorf("catalog %q already exists", value.Name)
		}
	}
	return nil
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
	value := metadata.CatalogDefinition{
		Format: metadata.CurrentFormat, ID: id, Name: name, Title: metadata.LocalizedText{configuration.DefaultLanguage: name},
		Code:              metadata.CatalogCode{Type: metadata.StringType, Length: 9, Auto: true, Unique: true},
		DescriptionLength: 64,
	}
	var encoded bytes.Buffer
	if err := metadata.Encode(&encoded, value); err != nil {
		return CatalogEditorSource{}, err
	}
	if _, err := metadata.DecodeCatalog("new catalog", bytes.NewReader(encoded.Bytes()), configuration); err != nil {
		return CatalogEditorSource{}, err
	}
	if err := workspace.checkCatalogNameLocked(value); err != nil {
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
	// A refused grant takes the new catalog away again: the roles are left as
	// they were, and a catalog no role was told of is not left behind.
	if _, err := workspace.applyRoleAutoGrantsLocked(func(role *metadata.RoleDefinition) bool {
		if !role.GrantNewObjectsByDefault {
			return false
		}
		return role.GrantObjectReadByDefault(id)
	}); err != nil {
		_ = os.Remove(filepath.Join(workspace.root, filepath.FromSlash(relative)))
		_ = os.Remove(objectDirectory)
		workspace.invalidateStudioIndexesLocked()
		return CatalogEditorSource{}, fmt.Errorf("grant default access to new catalog: %w", err)
	}
	offer, err := loadTypeOffer(workspace.root)
	if err != nil {
		return CatalogEditorSource{}, err
	}
	return CatalogEditorSource{
		Path: relative, Revision: sourceFile(relative, "yaml", encoded.Bytes()).Revision, Catalog: value, TypeOffer: offer,
		Languages: editorLanguages(configuration), DefaultLanguage: configuration.DefaultLanguage,
	}, nil
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
	objectDirectory := filepath.Join(workspace.root, filepath.FromSlash(path.Dir(relative)))
	if err := os.RemoveAll(objectDirectory); err != nil {
		return fmt.Errorf("delete catalog %q: %w", relative, err)
	}
	workspace.invalidateStudioIndexesLocked()
	return nil
}

func registerCatalogEditorRoutes(routes *http.ServeMux, workspace *Workspace) {
	routes.HandleFunc("GET /api/catalog", answerQuery(writeSourceError, func(request *http.Request) (any, error) {
		return workspace.ReadCatalogEditor(request.URL.Query().Get("path"))
	}))
	routes.HandleFunc("PUT /api/catalog", answerStudio(0, writeSourceError, func(input struct {
		Path             string                     `json:"path"`
		ExpectedRevision string                     `json:"expectedRevision"`
		Catalog          metadata.CatalogDefinition `json:"catalog"`
	}) (any, error) {
		return workspace.SaveCatalogEditor(input.Path, input.Catalog, input.ExpectedRevision)
	}))
	routes.HandleFunc("POST /api/catalog", answerStudio(0, writeSourceError, func(input struct {
		Name string `json:"name"`
	}) (any, error) {
		return workspace.CreateCatalog(input.Name)
	}))
	routes.HandleFunc("DELETE /api/catalog", answerRemoval(writeSourceError, func(request *http.Request) error {
		return workspace.DeleteCatalog(request.URL.Query().Get("path"), request.URL.Query().Get("expectedRevision"))
	}))
}
