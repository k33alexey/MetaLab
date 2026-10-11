package studio

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
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
	// KindTitles are the captions of the kinds of metadata, from the one
	// list Studio names them by.
	KindTitles map[string]string `json:"kindTitles"`
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
	configuration, err := project.ValidateLayout(workspace.root)
	if err != nil {
		return RoleSource{}, err
	}
	role, revision, err := workspace.decodeRoleLocked(relative, configuration)
	if err != nil {
		return RoleSource{}, err
	}
	schema, err := metadata.LoadPermissionSchema(workspace.root)
	if err != nil {
		return RoleSource{}, err
	}
	return RoleSource{Path: relative, Revision: revision, Role: role, Schema: schema, Languages: roleLanguages(configuration), DefaultLanguage: configuration.DefaultLanguage, KindTitles: metadataTitles}, nil
}

// roleFiles is a role as it lies on disk: its description, and the grid of
// its rights, nil when the role keeps none.
type roleFiles struct {
	description SourceFile
	grid        []byte
}

// revision covers both files: a save over a grid somebody else changed is as
// much a lost edit as one over a changed description.
func (files roleFiles) revision() string {
	digest := sha256.New()
	digest.Write([]byte(files.description.Content))
	if files.grid != nil {
		digest.Write([]byte{0})
		digest.Write(files.grid)
	}
	return hex.EncodeToString(digest.Sum(nil))
}

// roleGridPath is where the grid of the role described at relative lies.
func (workspace *Workspace) roleGridPath(relative string) string {
	id := strings.TrimSuffix(path.Base(relative), ".yaml")
	return filepath.Join(workspace.root, "metadata", "roles", id, metadata.RoleRightsFile)
}

// readRoleFilesLocked reads both files of one role. A grid that is not a
// plain file - a link, a folder - is refused rather than followed.
func (workspace *Workspace) readRoleFilesLocked(relative string) (roleFiles, error) {
	description, err := workspace.readSource(relative)
	if err != nil {
		return roleFiles{}, err
	}
	gridPath := workspace.roleGridPath(relative)
	for _, step := range []string{filepath.Dir(gridPath), gridPath} {
		info, err := os.Lstat(step)
		if errors.Is(err, fs.ErrNotExist) {
			return roleFiles{description: description}, nil
		}
		if err != nil {
			return roleFiles{}, fmt.Errorf("inspect role rights: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 || step == gridPath && !info.Mode().IsRegular() {
			return roleFiles{}, fmt.Errorf("%w: the rights of a role are a plain file", ErrInvalidSourcePath)
		}
	}
	grid, err := os.ReadFile(gridPath)
	if err != nil {
		return roleFiles{}, fmt.Errorf("read role rights: %w", err)
	}
	return roleFiles{description: description, grid: grid}, nil
}

// decodeRoleLocked reads one role whole - every place in Studio that needs a
// role reads it here, so the two files are always read together.
func (workspace *Workspace) decodeRoleLocked(relative string, configuration project.Project) (metadata.RoleDefinition, string, error) {
	files, err := workspace.readRoleFilesLocked(relative)
	if err != nil {
		return metadata.RoleDefinition{}, "", err
	}
	role, err := decodeRoleFiles(relative, []byte(files.description.Content), files.grid, configuration)
	if err != nil {
		return metadata.RoleDefinition{}, "", err
	}
	return role, files.revision(), nil
}

func decodeRoleFiles(relative string, description, grid []byte, configuration project.Project) (metadata.RoleDefinition, error) {
	var gridReader io.Reader
	if grid != nil {
		gridReader = bytes.NewReader(grid)
	}
	role, err := metadata.DecodeRole(relative, bytes.NewReader(description), gridReader, configuration)
	if err != nil {
		return metadata.RoleDefinition{}, err
	}
	if path.Base(relative) != role.ID.String()+".yaml" {
		return metadata.RoleDefinition{}, fmt.Errorf("role UUID does not match filename")
	}
	return role, nil
}

// writeRoleLocked writes a role as its two files, if both are still what the
// caller read. The grid goes first and is put back if the description cannot
// be written, so a refused save leaves the role as it was.
func (workspace *Workspace) writeRoleLocked(relative string, role metadata.RoleDefinition, expectedRevision string) (string, error) {
	current, err := workspace.readRoleFilesLocked(relative)
	if err != nil {
		return "", err
	}
	if current.revision() != expectedRevision {
		return "", ErrSourceChanged
	}
	description, grid, err := metadata.EncodeRole(role)
	if err != nil {
		return "", err
	}
	gridPath := workspace.roleGridPath(relative)
	if err := writeRoleGrid(gridPath, grid); err != nil {
		return "", err
	}
	saved, err := workspace.saveSourceLocked(relative, string(description), current.description.Revision)
	if err != nil {
		if restoreErr := writeRoleGrid(gridPath, current.grid); restoreErr != nil {
			return "", fmt.Errorf("%w; restoring the rights failed: %v", err, restoreErr)
		}
		return "", err
	}
	return roleFiles{description: saved, grid: grid}.revision(), nil
}

// writeRoleGrid puts the grid in place through a temporary file, or removes
// it with its folder when the role keeps none.
func writeRoleGrid(gridPath string, grid []byte) error {
	directory := filepath.Dir(gridPath)
	if grid == nil {
		if err := os.Remove(gridPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("remove role rights: %w", err)
		}
		if err := os.Remove(directory); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("remove role folder: %w", err)
		}
		return nil
	}
	if err := os.Mkdir(directory, 0o755); err != nil && !errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("create role folder: %w", err)
	}
	if info, err := os.Lstat(directory); err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: a role's folder must be a plain folder", ErrInvalidSourcePath)
	}
	temporary, err := prepareStudioSource(directory, grid)
	if err != nil {
		return err
	}
	if err := replaceStudioFile(temporary, gridPath); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("replace role rights: %w", err)
	}
	return nil
}

// roleDescriptions lists the description of every role, in the order of the
// folder. Folders - the grids - are not roles.
func (workspace *Workspace) roleDescriptions() ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(workspace.root, "metadata", "roles"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var result []string
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == ".gitkeep" {
			continue
		}
		result = append(result, "metadata/roles/"+entry.Name())
	}
	return result, nil
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
	KindTitles      map[string]string         `json:"kindTitles"`
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
	configuration, err := project.ValidateLayout(workspace.root)
	if err != nil {
		return RoleOverview{}, err
	}
	descriptions, err := workspace.roleDescriptions()
	if err != nil {
		return RoleOverview{}, err
	}
	result := RoleOverview{Roles: []RoleOverviewEntry{}, Languages: roleLanguages(configuration), DefaultLanguage: configuration.DefaultLanguage, KindTitles: metadataTitles}
	for _, relative := range descriptions {
		role, _, err := workspace.decodeRoleLocked(relative, configuration)
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
	saved, err := workspace.writeRoleLocked(relative, role, revision)
	if err != nil {
		return RoleSource{}, err
	}
	opened.Role, opened.Revision = role, saved
	return opened, nil
}

// DeleteRole permanently removes one role - its description and its grid - after confirming the
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
	if err := writeRoleGrid(workspace.roleGridPath(relative), nil); err != nil {
		return fmt.Errorf("delete role %q: %w", relative, err)
	}
	if err := os.Remove(filepath.Join(workspace.root, filepath.FromSlash(relative))); err != nil {
		return fmt.Errorf("delete role %q: %w", relative, err)
	}
	workspace.invalidateStudioIndexesLocked()
	return nil
}

func (workspace *Workspace) checkRoleNameLocked(role metadata.RoleDefinition) error {
	descriptions, err := workspace.roleDescriptions()
	if err != nil || len(descriptions) == 0 {
		return err
	}
	configuration, err := project.ValidateLayout(workspace.root)
	if err != nil {
		return err
	}
	for _, relative := range descriptions {
		if path.Base(relative) == role.ID.String()+".yaml" {
			continue
		}
		other, _, err := workspace.decodeRoleLocked(relative, configuration)
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
	configuration, err := project.ValidateLayout(workspace.root)
	if err != nil {
		return RoleSource{}, err
	}
	id, err := uuid.New()
	if err != nil {
		return RoleSource{}, err
	}
	role := metadata.RoleDefinition{Format: metadata.CurrentFormat, ID: id, Name: name, Title: metadata.LocalizedText{configuration.DefaultLanguage: name}}
	if err := metadata.ValidateRole("new role", role, configuration); err != nil {
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
	// A new role says nothing about any object, so it has no grid.
	description, _, err := metadata.EncodeRole(role)
	if err != nil {
		return RoleSource{}, err
	}
	content := bytes.NewBuffer(description)
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
	return RoleSource{Path: relative, Revision: sourceFile(relative, "yaml", content.Bytes()).Revision, Role: role, Schema: schema, Languages: roleLanguages(configuration), DefaultLanguage: configuration.DefaultLanguage, KindTitles: metadataTitles}, nil
}

// applyRoleAutoGrantsLocked visits every role file and lets mutate decide
// whether to change it (returning true if it did) - used right after a new
// metadata object is created (GrantNewObjectsByDefault) or a new attribute/
// table part is added to an existing one (GrantNewFieldsByDefault). The
// caller must already hold workspace.mu; this only uses the already-locked
// read/save helpers, never the exported (re-locking) ones.
func (workspace *Workspace) applyRoleAutoGrantsLocked(mutate func(*metadata.RoleDefinition) bool) error {
	descriptions, err := workspace.roleDescriptions()
	if err != nil || len(descriptions) == 0 {
		return err
	}
	configuration, err := project.ValidateLayout(workspace.root)
	if err != nil {
		return err
	}
	for _, relative := range descriptions {
		role, revision, err := workspace.decodeRoleLocked(relative, configuration)
		if err != nil {
			return err
		}
		if !mutate(&role) {
			continue
		}
		if _, err := workspace.writeRoleLocked(relative, role, revision); err != nil {
			return err
		}
	}
	return nil
}

// roleLanguages names every configured language for the browser: its code, and
// the synonym to show it by.
//
// The synonym of a language is localized like every other, so it is resolved -
// in that language itself first, because a language names itself in itself, and
// a list where one language is written in another reads as a mistake. A
// language with no synonym at all is shown by its name: an empty chip in a list
// of languages is worse than a technical one.
func roleLanguages(configuration project.Project) []FormLanguage {
	result := make([]FormLanguage, len(configuration.Languages))
	for index, language := range configuration.Languages {
		title := language.Title.Resolve(language.Code, configuration.DefaultLanguage, configuration.Languages)
		if title == "" {
			title = language.Name
		}
		result[index] = FormLanguage{Code: language.Code, Title: title}
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
	decoder := json.NewDecoder(request.Body)
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
