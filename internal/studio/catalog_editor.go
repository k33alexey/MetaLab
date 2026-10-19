package studio

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// loadTypeChoices is what the type picker of an attribute offers beside the
// primitive types: every reference a stored attribute may hold, and every
// defined type, each with the objects it may name. The model says which
// types those are (ReferenceTypeChoices), so the editor offers what the save
// accepts.
func loadTypeChoices(root string) ([]metadata.ReferenceTypeChoice, error) {
	catalog, err := metadata.Load(root)
	if err != nil {
		return nil, err
	}
	return catalog.ReferenceTypeChoices(), nil
}

// CatalogEditorSource is the typed, revision-safe representation the
// visual catalog editor reads and writes, in place of raw YAML.
type CatalogEditorSource struct {
	Path            string                         `json:"path"`
	Revision        string                         `json:"revision"`
	Catalog         metadata.CatalogDefinition     `json:"catalog"`
	TypeChoices     []metadata.ReferenceTypeChoice `json:"typeChoices"`
	Languages       []project.Language             `json:"languages"`
	DefaultLanguage string                         `json:"defaultLanguage"`
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
	choices, err := loadTypeChoices(workspace.root)
	if err != nil {
		return CatalogEditorSource{}, err
	}
	return CatalogEditorSource{
		Path: relative, Revision: file.Revision, Catalog: value, TypeChoices: choices,
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
	newFieldIDs := newCatalogFieldIDs(opened.Catalog, value)
	saved, err := workspace.saveSourceLocked(relative, encoded.String(), revision)
	if err != nil {
		return CatalogEditorSource{}, err
	}
	if len(newFieldIDs) > 0 {
		if err := workspace.applyRoleAutoGrantsLocked(func(role *metadata.RoleDefinition) bool {
			if !role.GrantNewFieldsByDefault {
				return false
			}
			changed := false
			for _, fieldID := range newFieldIDs {
				if role.GrantFieldReadByDefault(value.ID, fieldID.String()) {
					changed = true
				}
			}
			return changed
		}); err != nil {
			return CatalogEditorSource{}, fmt.Errorf("grant default access to new catalog fields: %w", err)
		}
	}
	opened.Catalog, opened.Revision = value, saved.Revision
	return opened, nil
}

// newCatalogFieldIDs collects the IDs of attributes and table parts (whole
// table parts, and their own nested attributes) present in after but not in
// before - a table part added earlier that only gained a new nested
// attribute counts too, since that attribute alone is separately grantable.
func newCatalogFieldIDs(before, after metadata.CatalogDefinition) []uuid.UUID {
	collect := func(value metadata.CatalogDefinition) map[uuid.UUID]bool {
		ids := make(map[uuid.UUID]bool)
		for _, attribute := range value.Attributes {
			ids[attribute.ID] = true
		}
		for _, part := range value.TableParts {
			ids[part.ID] = true
			for _, attribute := range part.Attributes {
				ids[attribute.ID] = true
			}
		}
		return ids
	}
	existing := collect(before)
	var result []uuid.UUID
	for id := range collect(after) {
		if !existing[id] {
			result = append(result, id)
		}
	}
	return result
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
	if err := workspace.applyRoleAutoGrantsLocked(func(role *metadata.RoleDefinition) bool {
		if !role.GrantNewObjectsByDefault {
			return false
		}
		return role.GrantObjectReadByDefault(id)
	}); err != nil {
		return CatalogEditorSource{}, fmt.Errorf("grant default access to new catalog: %w", err)
	}
	choices, err := loadTypeChoices(workspace.root)
	if err != nil {
		return CatalogEditorSource{}, err
	}
	return CatalogEditorSource{
		Path: relative, Revision: sourceFile(relative, "yaml", encoded.Bytes()).Revision, Catalog: value, TypeChoices: choices,
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
