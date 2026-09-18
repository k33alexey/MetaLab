package studio

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

type RoleSource struct {
	Path            string                    `json:"path"`
	Revision        string                    `json:"revision"`
	Role            metadata.RoleDefinition   `json:"role"`
	Schema          metadata.PermissionSchema `json:"schema"`
	Languages       []FormLanguage            `json:"languages"`
	DefaultLanguage string                    `json:"defaultLanguage"`
}

func validateRolePath(relative string) error {
	canonical, language, err := validateEditablePath(relative)
	if err != nil || language != "yaml" || !strings.HasPrefix(canonical, "metadata/roles/") {
		return ErrInvalidSourcePath
	}
	return nil
}

func (workspace *Workspace) ReadRole(relative string) (RoleSource, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	return workspace.readRoleLocked(relative)
}

func (workspace *Workspace) readRoleLocked(relative string) (RoleSource, error) {
	if err := validateRolePath(relative); err != nil {
		return RoleSource{}, err
	}
	file, err := workspace.readSource(relative)
	if err != nil {
		return RoleSource{}, err
	}
	manifest, err := project.ValidateLayout(workspace.root)
	if err != nil {
		return RoleSource{}, err
	}
	role, err := metadata.DecodeRole(relative, strings.NewReader(file.Content), manifest)
	if err != nil {
		return RoleSource{}, err
	}
	if path.Base(relative) != role.ID.String()+".yaml" {
		return RoleSource{}, fmt.Errorf("role UUID does not match filename")
	}
	schema, err := metadata.LoadPermissionSchema(workspace.root)
	if err != nil {
		return RoleSource{}, err
	}
	return RoleSource{Path: relative, Revision: file.Revision, Role: role, Schema: schema, Languages: roleLanguages(manifest), DefaultLanguage: manifest.DefaultLanguage}, nil
}

// RoleOverview is what the cross-role screens read. Opening one role at a time
// answers "what may this role do"; these screens answer the question a developer
// actually asks before a release - "who can reach this object, and under what
// restriction" - which no single role can show.
type RoleOverview struct {
	Roles           []RoleOverviewEntry       `json:"roles"`
	Schema          metadata.PermissionSchema `json:"schema"`
	Languages       []FormLanguage            `json:"languages"`
	DefaultLanguage string                    `json:"defaultLanguage"`
}

type RoleOverviewEntry struct {
	Path string                  `json:"path"`
	Role metadata.RoleDefinition `json:"role"`
}

// ReadAllRoles reads every role of the project. A role that does not decode is
// reported rather than skipped: silently omitting it would understate who has
// access, which is the one mistake this screen must not make.
func (workspace *Workspace) ReadAllRoles() (RoleOverview, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	manifest, err := project.ValidateLayout(workspace.root)
	if err != nil {
		return RoleOverview{}, err
	}
	entries, err := os.ReadDir(filepath.Join(workspace.root, "metadata", "roles"))
	if err != nil && !os.IsNotExist(err) {
		return RoleOverview{}, err
	}
	result := RoleOverview{Roles: []RoleOverviewEntry{}, Languages: roleLanguages(manifest), DefaultLanguage: manifest.DefaultLanguage}
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == ".gitkeep" {
			continue
		}
		relative := "metadata/roles/" + entry.Name()
		file, err := workspace.readSource(relative)
		if err != nil {
			return RoleOverview{}, err
		}
		role, err := metadata.DecodeRole(relative, strings.NewReader(file.Content), manifest)
		if err != nil {
			return RoleOverview{}, err
		}
		result.Roles = append(result.Roles, RoleOverviewEntry{Path: relative, Role: role})
	}
	sort.Slice(result.Roles, func(i, j int) bool {
		return strings.ToLower(result.Roles[i].Role.Name) < strings.ToLower(result.Roles[j].Role.Name)
	})
	if result.Schema, err = metadata.LoadPermissionSchema(workspace.root); err != nil {
		return RoleOverview{}, err
	}
	return result, nil
}

func (workspace *Workspace) SaveRole(relative string, role metadata.RoleDefinition, revision string) (RoleSource, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	opened, err := workspace.readRoleLocked(relative)
	if err != nil {
		return RoleSource{}, err
	}
	if opened.Revision != revision {
		return RoleSource{}, ErrSourceChanged
	}
	if role.ID != opened.Role.ID {
		return RoleSource{}, fmt.Errorf("role UUID cannot be changed")
	}
	if err := metadata.ValidateProjectRole(workspace.root, role); err != nil {
		return RoleSource{}, err
	}
	if err := workspace.checkRoleNameLocked(role); err != nil {
		return RoleSource{}, err
	}
	var encoded bytes.Buffer
	if err := metadata.Encode(&encoded, role); err != nil {
		return RoleSource{}, err
	}
	saved, err := workspace.saveSourceLocked(relative, encoded.String(), revision)
	if err != nil {
		return RoleSource{}, err
	}
	opened.Role, opened.Revision = role, saved.Revision
	return opened, nil
}

// DeleteRole permanently removes one role's YAML file, after confirming the
// caller still has the latest revision. Reference existence (e.g. a common
// module still granting this role) is not checked here — same as SaveRole,
// that is caught at the next full project Load, not at delete time.
func (workspace *Workspace) DeleteRole(relative, expectedRevision string) error {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	opened, err := workspace.readRoleLocked(relative)
	if err != nil {
		return err
	}
	if opened.Revision != expectedRevision {
		return ErrSourceChanged
	}
	if err := os.Remove(filepath.Join(workspace.root, filepath.FromSlash(relative))); err != nil {
		return fmt.Errorf("delete role %q: %w", relative, err)
	}
	workspace.invalidateStudioIndexesLocked()
	return nil
}

func (workspace *Workspace) checkRoleNameLocked(role metadata.RoleDefinition) error {
	entries, err := os.ReadDir(filepath.Join(workspace.root, "metadata", "roles"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	manifest, err := project.ValidateLayout(workspace.root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() == ".gitkeep" || entry.Name() == role.ID.String()+".yaml" {
			continue
		}
		file, err := workspace.readSource("metadata/roles/" + entry.Name())
		if err != nil {
			return err
		}
		other, err := metadata.DecodeRole(file.Path, strings.NewReader(file.Content), manifest)
		if err != nil {
			return err
		}
		if strings.EqualFold(other.Name, role.Name) {
			return fmt.Errorf("role %q already exists", role.Name)
		}
	}
	return nil
}

func (workspace *Workspace) CreateRole(name string) (RoleSource, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	manifest, err := project.ValidateLayout(workspace.root)
	if err != nil {
		return RoleSource{}, err
	}
	id, err := uuid.New()
	if err != nil {
		return RoleSource{}, err
	}
	role := metadata.RoleDefinition{Format: metadata.CurrentFormat, ID: id, Name: name, Title: metadata.LocalizedText{manifest.DefaultLanguage: name}}
	if err := metadata.ValidateRole("new role", role, manifest); err != nil {
		return RoleSource{}, err
	}
	schema, err := metadata.LoadPermissionSchema(workspace.root)
	if err != nil {
		return RoleSource{}, err
	}
	if err := workspace.checkRoleNameLocked(role); err != nil {
		return RoleSource{}, err
	}
	relative, _ := project.MetadataPath("roles", id)
	// Metadata root was checked by ValidateLayout. Do not follow an existing
	// role-directory symlink when creating the first source.
	directory := filepath.Join(workspace.root, "metadata", "roles")
	if err := os.Mkdir(directory, 0o755); err != nil && !os.IsExist(err) {
		return RoleSource{}, err
	}
	info, err := os.Lstat(directory)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return RoleSource{}, ErrInvalidSourcePath
	}
	var content bytes.Buffer
	if err := metadata.Encode(&content, role); err != nil {
		return RoleSource{}, err
	}
	file, err := os.OpenFile(filepath.Join(workspace.root, filepath.FromSlash(relative)), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return RoleSource{}, err
	}
	_, writeErr := file.Write(content.Bytes())
	if writeErr == nil {
		writeErr = file.Sync()
	}
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(filepath.Join(workspace.root, filepath.FromSlash(relative)))
		return RoleSource{}, fmt.Errorf("create role: write=%v, close=%v", writeErr, closeErr)
	}
	workspace.invalidateStudioIndexesLocked()
	return RoleSource{Path: relative, Revision: sourceFile(relative, "yaml", content.Bytes()).Revision, Role: role, Schema: schema, Languages: roleLanguages(manifest), DefaultLanguage: manifest.DefaultLanguage}, nil
}

// applyRoleAutoGrantsLocked visits every role file and lets mutate decide
// whether to change it (returning true if it did) - used right after a new
// metadata object is created (GrantNewObjectsByDefault) or a new attribute/
// table part is added to an existing one (GrantNewFieldsByDefault). The
// caller must already hold workspace.mu; this only uses the already-locked
// read/save helpers, never the exported (re-locking) ones.
func (workspace *Workspace) applyRoleAutoGrantsLocked(mutate func(*metadata.RoleDefinition) bool) error {
	entries, err := os.ReadDir(filepath.Join(workspace.root, "metadata", "roles"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	manifest, err := project.ValidateLayout(workspace.root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() == ".gitkeep" || entry.IsDir() {
			continue
		}
		relative := "metadata/roles/" + entry.Name()
		file, err := workspace.readSource(relative)
		if err != nil {
			return err
		}
		role, err := metadata.DecodeRole(relative, strings.NewReader(file.Content), manifest)
		if err != nil {
			return err
		}
		if !mutate(&role) {
			continue
		}
		var encoded bytes.Buffer
		if err := metadata.Encode(&encoded, role); err != nil {
			return err
		}
		if _, err := workspace.saveSourceLocked(relative, encoded.String(), file.Revision); err != nil {
			return err
		}
	}
	return nil
}

func roleLanguages(manifest project.Project) []FormLanguage {
	result := make([]FormLanguage, len(manifest.Languages))
	for index, language := range manifest.Languages {
		result[index] = FormLanguage{Code: language.Code, Title: language.Title}
	}
	return result
}

func registerRoleRoutes(routes *http.ServeMux, workspace *Workspace) {
	routes.HandleFunc("GET /api/roles", func(response http.ResponseWriter, request *http.Request) {
		overview, err := workspace.ReadAllRoles()
		if err != nil {
			writeSourceError(response, err)
			return
		}
		writeStudioJSON(response, overview)
	})
	routes.HandleFunc("GET /api/role", func(response http.ResponseWriter, request *http.Request) {
		role, err := workspace.ReadRole(request.URL.Query().Get("path"))
		if err != nil {
			writeSourceError(response, err)
			return
		}
		writeStudioJSON(response, role)
	})
	routes.HandleFunc("PUT /api/role", func(response http.ResponseWriter, request *http.Request) {
		var input struct {
			Path             string                  `json:"path"`
			ExpectedRevision string                  `json:"expectedRevision"`
			Role             metadata.RoleDefinition `json:"role"`
		}
		if !decodeStudioJSONRequest(response, request, &input) {
			return
		}
		role, err := workspace.SaveRole(input.Path, input.Role, input.ExpectedRevision)
		if err != nil {
			writeSourceError(response, err)
			return
		}
		writeStudioJSON(response, role)
	})
	routes.HandleFunc("POST /api/role", func(response http.ResponseWriter, request *http.Request) {
		var input struct {
			Name string `json:"name"`
		}
		if !decodeStudioJSONRequest(response, request, &input) {
			return
		}
		role, err := workspace.CreateRole(input.Name)
		if err != nil {
			writeSourceError(response, err)
			return
		}
		writeStudioJSON(response, role)
	})
	routes.HandleFunc("DELETE /api/role", func(response http.ResponseWriter, request *http.Request) {
		if !validateStudioMutation(response, request) {
			return
		}
		if err := workspace.DeleteRole(request.URL.Query().Get("path"), request.URL.Query().Get("expectedRevision")); err != nil {
			writeSourceError(response, err)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
}

func decodeStudioJSONRequest(response http.ResponseWriter, request *http.Request, target any) bool {
	if !validateStudioMutation(response, request) {
		return false
	}
	if !strings.HasPrefix(request.Header.Get("Content-Type"), "application/json") {
		http.Error(response, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
		return false
	}
	decoder := json.NewDecoder(http.MaxBytesReader(response, request.Body, 2*MaxEditableFileBytes+(64<<10)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		http.Error(response, "Invalid request", http.StatusBadRequest)
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		http.Error(response, "Invalid request", http.StatusBadRequest)
		return false
	}
	return true
}
