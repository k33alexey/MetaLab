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

// TypeChoice is one referenceable metadata object shown in the catalog
// editor's type picker.
type TypeChoice struct {
	ID    uuid.UUID              `json:"id"`
	Name  string                 `json:"name"`
	Title metadata.LocalizedText `json:"title"`
}

// TypeChoices lists every object an attribute's composite type can
// reference, grouped by kind.
type TypeChoices struct {
	Enumerations []TypeChoice `json:"enumerations"`
	DefinedTypes []TypeChoice `json:"definedTypes"`
	Catalogs     []TypeChoice `json:"catalogs"`
	Documents    []TypeChoice `json:"documents"`
}

func loadTypeChoices(root string) (TypeChoices, error) {
	catalog, err := metadata.Load(root)
	if err != nil {
		return TypeChoices{}, err
	}
	choices := TypeChoices{}
	for _, item := range catalog.Enumerations {
		choices.Enumerations = append(choices.Enumerations, TypeChoice{ID: item.ID, Name: item.Name, Title: item.Title})
	}
	for _, item := range catalog.DefinedTypes {
		choices.DefinedTypes = append(choices.DefinedTypes, TypeChoice{ID: item.ID, Name: item.Name, Title: item.Title})
	}
	for _, item := range catalog.Catalogs {
		choices.Catalogs = append(choices.Catalogs, TypeChoice{ID: item.ID, Name: item.Name, Title: item.Title})
	}
	for _, item := range catalog.Documents {
		choices.Documents = append(choices.Documents, TypeChoice{ID: item.ID, Name: item.Name, Title: item.Title})
	}
	return choices, nil
}

// CatalogEditorSource is the typed, revision-safe representation the
// visual catalog editor reads and writes, in place of raw YAML.
type CatalogEditorSource struct {
	Path            string                     `json:"path"`
	Revision        string                     `json:"revision"`
	Catalog         metadata.CatalogDefinition `json:"catalog"`
	TypeChoices     TypeChoices                `json:"typeChoices"`
	Languages       []FormLanguage             `json:"languages"`
	DefaultLanguage string                     `json:"defaultLanguage"`
}

func validateCatalogEditorPath(relative string) error {
	canonical, language, err := validateEditablePath(relative)
	if err != nil || language != "yaml" {
		return ErrInvalidSourcePath
	}
	parts := strings.Split(canonical, "/")
	if len(parts) != 4 || parts[0] != "metadata" || parts[1] != "catalogs" || parts[3] != "object.yaml" {
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
		Languages: roleLanguages(configuration), DefaultLanguage: configuration.DefaultLanguage,
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
	entries, err := os.ReadDir(filepath.Join(workspace.root, "metadata", "catalogs"))
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
	relative, err := project.ObjectMetadataPath("catalogs", name)
	if err != nil {
		return CatalogEditorSource{}, err
	}
	directory := filepath.Join(workspace.root, "metadata", "catalogs")
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
		Languages: roleLanguages(configuration), DefaultLanguage: configuration.DefaultLanguage,
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
	routes.HandleFunc("GET /api/catalog", func(response http.ResponseWriter, request *http.Request) {
		value, err := workspace.ReadCatalogEditor(request.URL.Query().Get("path"))
		if err != nil {
			writeSourceError(response, err)
			return
		}
		writeStudioJSON(response, value)
	})
	routes.HandleFunc("PUT /api/catalog", func(response http.ResponseWriter, request *http.Request) {
		var input struct {
			Path             string                     `json:"path"`
			ExpectedRevision string                     `json:"expectedRevision"`
			Catalog          metadata.CatalogDefinition `json:"catalog"`
		}
		if !decodeStudioJSONRequest(response, request, &input) {
			return
		}
		value, err := workspace.SaveCatalogEditor(input.Path, input.Catalog, input.ExpectedRevision)
		if err != nil {
			writeSourceError(response, err)
			return
		}
		writeStudioJSON(response, value)
	})
	routes.HandleFunc("POST /api/catalog", func(response http.ResponseWriter, request *http.Request) {
		var input struct {
			Name string `json:"name"`
		}
		if !decodeStudioJSONRequest(response, request, &input) {
			return
		}
		value, err := workspace.CreateCatalog(input.Name)
		if err != nil {
			writeSourceError(response, err)
			return
		}
		writeStudioJSON(response, value)
	})
	routes.HandleFunc("DELETE /api/catalog", func(response http.ResponseWriter, request *http.Request) {
		if !validateStudioMutation(response, request) {
			return
		}
		if err := workspace.DeleteCatalog(request.URL.Query().Get("path"), request.URL.Query().Get("expectedRevision")); err != nil {
			writeSourceError(response, err)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
}
