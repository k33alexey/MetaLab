package studio

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
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

	"github.com/k33alexey/MetaLab/internal/atomicfile"
	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
	"github.com/k33alexey/MetaLab/internal/webpage"
)

type RoleSource struct {
	Path            string                    `json:"path"`
	Revision        string                    `json:"revision"`
	Role            metadata.RoleDefinition   `json:"role"`
	Schema          metadata.PermissionSchema `json:"schema"`
	Languages       []project.Language        `json:"languages"`
	DefaultLanguage string                    `json:"defaultLanguage"`
	roleTreeSource
}

// roleTreeSource is what the tree of rights of either window is drawn by
// beside the schema: the captions and the icons of the kinds of metadata and
// the places of their branches, with the branch «Общие» - from the one list
// Studio names, draws and places them by in the project tree.
type roleTreeSource struct {
	KindTitles map[string]string `json:"kindTitles"`
	KindIcons  map[string]string `json:"kindIcons"`
	KindPlaces map[string]string `json:"kindPlaces"`
	Common     CommonBranch      `json:"common"`
}

var roleTree = roleTreeSource{KindTitles: metadataTitles, KindIcons: metadataIcons, KindPlaces: branchPlaces, Common: commonBranch}

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

// RoleRevision is the revision of a role as ReadRole gives it - of its
// description and its grid of rights together - without reading the schema:
// what Studio compares to notice the role changed on disk.
func (workspace *Workspace) RoleRevision(relative string) (string, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	if err := validateRolePath(relative); err != nil {
		return "", err
	}
	files, err := workspace.readRoleFilesLocked(relative)
	if err != nil {
		return "", err
	}
	return files.revision(), nil
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
	return RoleSource{Path: relative, Revision: revision, Role: role, Schema: schema, Languages: editorLanguages(configuration), DefaultLanguage: configuration.DefaultLanguage, roleTreeSource: roleTree}, nil
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
	var rights io.Reader
	if grid != nil {
		rights = bytes.NewReader(grid)
	}
	value, described, err := metadata.DecodeSource(relative, bytes.NewReader(description), rights, configuration)
	if err != nil {
		return metadata.RoleDefinition{}, err
	}
	role, ok := value.(metadata.RoleDefinition)
	if !described || !ok {
		return metadata.RoleDefinition{}, ErrInvalidSourcePath
	}
	return role, nil
}

// writeRoleLocked writes a role as its two files, if both are still what the
// caller read, and answers with the files as they now lie.
func (workspace *Workspace) writeRoleLocked(relative string, role metadata.RoleDefinition, expectedRevision string) (roleFiles, error) {
	current, err := workspace.readRoleFilesLocked(relative)
	if err != nil {
		return roleFiles{}, err
	}
	if current.revision() != expectedRevision {
		return roleFiles{}, ErrSourceChanged
	}
	description, grid, err := metadata.EncodeRole(role)
	if err != nil {
		return roleFiles{}, err
	}
	return workspace.replaceRoleFilesLocked(relative, current, description, grid)
}

// replaceRoleFilesLocked puts a description and a grid in place of the files
// current names. The grid goes first and is put back if the description
// cannot be written, so a refused write leaves the role as it was.
func (workspace *Workspace) replaceRoleFilesLocked(relative string, current roleFiles, description, grid []byte) (roleFiles, error) {
	gridPath := workspace.roleGridPath(relative)
	if err := writeRoleGrid(gridPath, grid); err != nil {
		return roleFiles{}, err
	}
	saved, err := workspace.saveSourceLocked(relative, string(description), current.description.Revision)
	if err != nil {
		if restoreErr := writeRoleGrid(gridPath, current.grid); restoreErr != nil {
			return roleFiles{}, fmt.Errorf("%w; restoring the rights failed: %v", err, restoreErr)
		}
		return roleFiles{}, err
	}
	return roleFiles{description: saved, grid: grid}, nil
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
	if err := atomicfile.Write(gridPath, ".ml-write-*", 0o644, atomicfile.Bytes(grid)); err != nil {
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
	Languages       []project.Language        `json:"languages"`
	DefaultLanguage string                    `json:"defaultLanguage"`
	roleTreeSource
}

type RoleOverviewEntry struct {
	Path string `json:"path"`
	// Revision is the role's as ReadRole gives it: what a save from the
	// window «Все роли» is checked against.
	Revision string                  `json:"revision"`
	Role     metadata.RoleDefinition `json:"role"`
}

// ReadAllRoles reads every role of the project. A role that does not decode is
// reported rather than skipped: silently omitting it would understate who has
// access, which is the one mistake this screen must not make.
func (workspace *Workspace) ReadAllRoles() (RoleOverview, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	return workspace.readAllRolesLocked()
}

func (workspace *Workspace) readAllRolesLocked() (RoleOverview, error) {
	configuration, err := project.ValidateLayout(workspace.root)
	if err != nil {
		return RoleOverview{}, err
	}
	descriptions, err := workspace.roleDescriptions()
	if err != nil {
		return RoleOverview{}, err
	}
	result := RoleOverview{Roles: []RoleOverviewEntry{}, Languages: editorLanguages(configuration), DefaultLanguage: configuration.DefaultLanguage, roleTreeSource: roleTree}
	for _, relative := range descriptions {
		role, revision, err := workspace.decodeRoleLocked(relative, configuration)
		if err != nil {
			return RoleOverview{}, err
		}
		result.Roles = append(result.Roles, RoleOverviewEntry{Path: relative, Revision: revision, Role: role})
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
	opened, err := workspace.checkRoleSaveLocked(RoleSave{Path: relative, ExpectedRevision: revision, Role: role})
	if err != nil {
		return RoleSource{}, err
	}
	// Written as every role is (writeRolesLocked), and answered as written.
	if err := metadata.DropAbsentRestrictionFields(workspace.root, &role); err != nil {
		return RoleSource{}, err
	}
	saved, err := workspace.writeRoleLocked(relative, role, revision)
	if err != nil {
		return RoleSource{}, err
	}
	opened.Role, opened.Revision = role, saved.revision()
	return opened, nil
}

// RoleSave is one role to write: where it lies, the revision it was read at
// and what it is to become.
type RoleSave struct {
	Path             string                  `json:"path"`
	ExpectedRevision string                  `json:"expectedRevision"`
	Role             metadata.RoleDefinition `json:"role"`
}

// checkRoleSaveLocked refuses a save the role cannot take: a role changed
// since it was read, another identifier, a role the project would not load,
// a name another role has.
func (workspace *Workspace) checkRoleSaveLocked(save RoleSave) (RoleSource, error) {
	opened, err := workspace.readRoleLocked(save.Path)
	if err != nil {
		return RoleSource{}, err
	}
	if opened.Revision != save.ExpectedRevision {
		return RoleSource{}, ErrSourceChanged
	}
	if save.Role.ID != opened.Role.ID {
		return RoleSource{}, fmt.Errorf("role UUID cannot be changed")
	}
	if err := metadata.ValidateProjectRole(workspace.root, save.Role); err != nil {
		return RoleSource{}, err
	}
	// A name the save leaves as it was is where it stood: «Все роли» saves
	// many roles at once and would weigh each against the whole tree.
	if save.Role.Name != opened.Role.Name {
		if err := workspace.objectNameErrorLocked(save.Path, "", save.Role.Name); err != nil {
			return RoleSource{}, err
		}
	}
	return opened, nil
}

// SaveRoles writes the roles the window «Все роли» changed - all of them or
// none: every one is checked before the first is written, and the roles
// written before one that fails are put back as they lay.
func (workspace *Workspace) SaveRoles(saves []RoleSave) (RoleOverview, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	for _, save := range saves {
		opened, err := workspace.checkRoleSaveLocked(save)
		if err != nil {
			return RoleOverview{}, fmt.Errorf("role %q: %w", save.Role.Name, err)
		}
		// «Все роли» changes rights, not names (the owner, 21.10.2026): a
		// role is renamed in its own window, one at a time. Two roles of one
		// save renamed alike would each pass the check against the disk and
		// leave a project that does not load.
		if save.Role.Name != opened.Role.Name {
			return RoleOverview{}, fmt.Errorf("role %q: all roles are saved with their names, a role is renamed in its own window", opened.Role.Name)
		}
	}
	if _, err := workspace.writeRolesLocked(saves); err != nil {
		return RoleOverview{}, err
	}
	return workspace.readAllRolesLocked()
}

// writeRolesLocked writes the roles in order, each without the fields of
// restrictions its objects have not now, as SaveRole writes one
// (metadata.DropAbsentRestrictionFields). A role refused puts back the
// ones written before it, so a refused save leaves every role as it was;
// after a save that went through, the answer puts them all back, for a
// caller whose own step after the roles is refused.
func (workspace *Workspace) writeRolesLocked(saves []RoleSave) (func() error, error) {
	type written struct {
		path            string
		before, current roleFiles
	}
	roles := make([]*metadata.RoleDefinition, len(saves))
	for index := range saves {
		roles[index] = &saves[index].Role
	}
	if err := metadata.DropAbsentRestrictionFields(workspace.root, roles...); err != nil {
		return nil, err
	}
	var done []written
	restore := func() error {
		for index := len(done) - 1; index >= 0; index-- {
			item := done[index]
			if _, err := workspace.replaceRoleFilesLocked(item.path, item.current, []byte(item.before.description.Content), item.before.grid); err != nil {
				return fmt.Errorf("restoring role %q failed: %w", item.path, err)
			}
		}
		return nil
	}
	for _, save := range saves {
		before, err := workspace.readRoleFilesLocked(save.Path)
		if err == nil {
			var current roleFiles
			if current, err = workspace.writeRoleLocked(save.Path, save.Role, save.ExpectedRevision); err == nil {
				done = append(done, written{save.Path, before, current})
				continue
			}
		}
		if restoreErr := restore(); restoreErr != nil {
			return nil, fmt.Errorf("role %q: %w; %v", save.Role.Name, err, restoreErr)
		}
		return nil, fmt.Errorf("role %q: %w", save.Role.Name, err)
	}
	return restore, nil
}

// DeleteRole permanently removes one role - its description and its grid - after confirming the
// caller still has the latest revision. Reference existence (e.g. a common
// module still granting this role) is not checked here — same as SaveRole,
// that is caught at the next full project Load, not at delete time.
//
// The grid goes first, as a save writes it first, and comes back when the
// rest cannot follow: a delete refused halfway leaves the role as it was, not
// a description with no rights, which loads as a role that grants nothing.
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
	current, err := workspace.readRoleFilesLocked(relative)
	if err != nil {
		return err
	}
	// The role leaves the order first, and comes back to it when the rest
	// cannot follow, as its rights do.
	restoreOrder, err := workspace.dropFromOrderLocked(metadata.RoleKind, opened.Role.ID)
	if err != nil {
		return fmt.Errorf("delete role %q: %w", relative, err)
	}
	gridPath := workspace.roleGridPath(relative)
	err = writeRoleGrid(gridPath, nil)
	if err == nil {
		err = os.Remove(filepath.Join(workspace.root, filepath.FromSlash(relative)))
	}
	if err != nil {
		restoreErr := writeRoleGrid(gridPath, current.grid)
		if orderErr := restoreOrder(); restoreErr == nil {
			restoreErr = orderErr
		}
		if restoreErr != nil {
			return fmt.Errorf("delete role %q: %w; restoring the role failed: %v", relative, err, restoreErr)
		}
		return fmt.Errorf("delete role %q: %w", relative, err)
	}
	workspace.invalidateStudioIndexesLocked()
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
	if err := workspace.objectNameErrorLocked("", branchOf(string(metadata.RoleKind)), name); err != nil {
		return RoleSource{}, err
	}
	// No synonym, as a new catalog has none (CreateCatalog).
	role := metadata.RoleDefinition{Format: metadata.CurrentFormat, ID: id, Name: name}
	if err := metadata.ValidateRole("new role", role, configuration); err != nil {
		return RoleSource{}, err
	}
	schema, err := metadata.LoadPermissionSchema(workspace.root)
	if err != nil {
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
	// The new role stands last among the roles; an order that cannot be
	// written takes the role away again.
	if _, err := workspace.placeInOrderLocked(metadata.RoleKind, id); err != nil {
		_ = os.Remove(filepath.Join(workspace.root, filepath.FromSlash(relative)))
		workspace.invalidateStudioIndexesLocked()
		return RoleSource{}, fmt.Errorf("place new role in the order: %w", err)
	}
	return RoleSource{Path: relative, Revision: sourceFile(relative, "yaml", content.Bytes()).Revision, Role: role, Schema: schema, Languages: editorLanguages(configuration), DefaultLanguage: configuration.DefaultLanguage, roleTreeSource: roleTree}, nil
}

// applyRoleAutoGrantsLocked visits every role file and lets mutate decide
// whether to change it (returning true if it did) - used right after a new
// metadata object is created (GrantNewObjectsByDefault), a new attribute/
// table part is added to an existing one (GrantNewFieldsByDefault) or one is
// deleted. Every role is read and changed before any is written, and the
// roles are written by writeRolesLocked: a refusal leaves them all as they
// were, and the answer puts them back for a caller whose own write after
// them is refused. The caller must already hold workspace.mu; this only
// uses the already-locked read/save helpers, never the exported
// (re-locking) ones.
func (workspace *Workspace) applyRoleAutoGrantsLocked(mutate func(*metadata.RoleDefinition) bool) (func() error, error) {
	unchanged := func() error { return nil }
	descriptions, err := workspace.roleDescriptions()
	if err != nil || len(descriptions) == 0 {
		return unchanged, err
	}
	configuration, err := project.ValidateLayout(workspace.root)
	if err != nil {
		return unchanged, err
	}
	var saves []RoleSave
	for _, relative := range descriptions {
		role, revision, err := workspace.decodeRoleLocked(relative, configuration)
		if err != nil {
			return unchanged, err
		}
		if mutate(&role) {
			saves = append(saves, RoleSave{Path: relative, Role: role, ExpectedRevision: revision})
		}
	}
	return workspace.writeRolesLocked(saves)
}

// editorLanguages are the configured languages as an editor of Studio gets
// them: whole, with the synonym in every language. The page names a language
// by its synonym in the language Studio edits in, as the prototype does
// (languageTitle, studio-common.js; the owner's decision of 19.10.2026), so
// the name it is shown by changes with that language and is not resolved
// here.
func editorLanguages(configuration project.Project) []project.Language {
	if configuration.Languages == nil {
		return []project.Language{}
	}
	return configuration.Languages
}

func registerRoleRoutes(routes *http.ServeMux, workspace *Workspace) {
	routes.HandleFunc("GET /api/roles", answerQuery(writeSourceError, func(*http.Request) (any, error) {
		return workspace.ReadAllRoles()
	}))
	routes.HandleFunc("PUT /api/roles", answerStudio(0, writeSourceError, func(input struct {
		Roles []RoleSave `json:"roles"`
	}) (any, error) {
		return workspace.SaveRoles(input.Roles)
	}))
	routes.HandleFunc("GET /api/role", func(response http.ResponseWriter, request *http.Request) {
		path := request.URL.Query().Get("path")
		// A role unchanged since the caller read it is answered without the
		// schema, which is the whole project read again.
		if known := request.Header.Get("If-None-Match"); known != "" {
			revision, err := workspace.RoleRevision(path)
			if err != nil {
				writeSourceError(response, err)
				return
			}
			response.Header().Set("ETag", `"`+revision+`"`)
			if known == `"`+revision+`"` {
				response.WriteHeader(http.StatusNotModified)
				return
			}
		}
		role, err := workspace.ReadRole(path)
		if err != nil {
			writeSourceError(response, err)
			return
		}
		response.Header().Set("ETag", `"`+role.Revision+`"`)
		webpage.JSON(response, http.StatusOK, role)
	})
	routes.HandleFunc("PUT /api/role", answerStudio(0, writeSourceError, func(input struct {
		Path             string                  `json:"path"`
		ExpectedRevision string                  `json:"expectedRevision"`
		Role             metadata.RoleDefinition `json:"role"`
	}) (any, error) {
		return workspace.SaveRole(input.Path, input.Role, input.ExpectedRevision)
	}))
	routes.HandleFunc("POST /api/role", answerStudio(0, writeSourceError, func(input struct {
		Name string `json:"name"`
	}) (any, error) {
		return workspace.CreateRole(input.Name)
	}))
	routes.HandleFunc("DELETE /api/role", answerRemoval(writeSourceError, func(request *http.Request) error {
		return workspace.DeleteRole(request.URL.Query().Get("path"), request.URL.Query().Get("expectedRevision"))
	}))
}
