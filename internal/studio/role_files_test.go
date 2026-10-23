package studio

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// savedRoleWithGrid is a role saved through Studio with a grid of rights and
// a restriction, and the path of its grid.
func savedRoleWithGrid(t *testing.T) (*Workspace, RoleSource, string) {
	t.Helper()
	workspace, object := roleWorkspace(t)
	created, err := workspace.CreateRole("Кладовщик")
	if err != nil {
		t.Fatal(err)
	}
	created.Role.Objects = []metadata.ObjectPermission{{Object: object.ID, Operations: []metadata.PermissionOperation{metadata.PermissionRead},
		Denied: []metadata.PermissionOperation{metadata.PermissionInteractiveDelete},
		Restrictions: []metadata.RightRestriction{{Right: metadata.PermissionRead, Fields: []string{object.Attributes[0].ID.String()},
			Condition: "ГДЕ Количество = 1"}}}}
	saved, err := workspace.SaveRole(created.Path, created.Role, created.Revision)
	if err != nil {
		t.Fatal(err)
	}
	return workspace, saved, filepath.Join(workspace.root, "metadata", "roles", saved.Role.ID.String(), metadata.RoleRightsFile)
}

// A save writes the grid beside the description, and the role read back -
// in Studio and by the loader - is the role saved.
//
// Defect caught: the grid not written, or written where the loader does not
// look, so that a saved role comes back without its rights.
func TestStudioSavesARoleAsTwoFiles(t *testing.T) {
	t.Parallel()
	workspace, saved, grid := savedRoleWithGrid(t)
	content, err := os.ReadFile(grid)
	if err != nil || !strings.Contains(string(content), `"interactive-delete"`) {
		t.Fatalf("the grid was not written: %q, %v", content, err)
	}
	catalog, err := metadata.Load(workspace.root)
	if err != nil {
		t.Fatal(err)
	}
	role, _ := catalog.Role("Кладовщик")
	if len(role.Objects) != 1 || !slices.Equal(role.Objects[0].Denied, saved.Role.Objects[0].Denied) || len(role.Objects[0].Restrictions) != 1 {
		t.Fatalf("the loader read another role: %+v", role.Objects)
	}
}

// The revision a role is edited against covers its grid: a grid changed on
// disk meanwhile - by git, by another window - refuses the save.
//
// Defect caught: a revision of the description alone, under which a save
// silently overwrote rights somebody else had just changed.
func TestARoleSaveOverAChangedGridIsRefused(t *testing.T) {
	t.Parallel()
	workspace, saved, grid := savedRoleWithGrid(t)
	content, err := os.ReadFile(grid)
	if err != nil {
		t.Fatal(err)
	}
	changed := strings.Replace(string(content), `"interactive-delete"`, `"interactive-insert"`, 1)
	if err := os.WriteFile(grid, []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.SaveRole(saved.Path, saved.Role, saved.Revision); !errors.Is(err, ErrSourceChanged) {
		t.Fatalf("a save over a changed grid: %v", err)
	}
	if after, _ := os.ReadFile(grid); string(after) != changed {
		t.Fatal("the refused save still wrote the grid")
	}
}

// When the description cannot be written after the grid was, the grid is put
// back: a refused save leaves the role as it was.
//
// Defect caught: a new grid beside an old description - a role half saved.
func TestARoleSaveThatFailsHalfwayPutsTheGridBack(t *testing.T) {
	t.Parallel()
	workspace, saved, grid := savedRoleWithGrid(t)
	before, err := os.ReadFile(grid)
	if err != nil {
		t.Fatal(err)
	}
	roles := filepath.Join(workspace.root, "metadata", "roles")
	// The folder of roles refuses new files, so the description cannot be
	// replaced; the role's own folder still takes the grid.
	if err := os.Chmod(roles, 0o555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(roles, 0o755)
	changed := saved.Role
	changed.Objects = []metadata.ObjectPermission{{Object: saved.Role.Objects[0].Object, Operations: []metadata.PermissionOperation{metadata.PermissionView}}}
	if _, err := workspace.SaveRole(saved.Path, changed, saved.Revision); err == nil {
		t.Fatal("the save went through a folder that takes no files")
	}
	if after, _ := os.ReadFile(grid); string(after) != string(before) {
		t.Fatalf("the grid was left changed:\n%s", after)
	}
}

// Deleting a role removes its grid with it.
//
// Defect caught: a folder of no role left behind, which the loader refuses -
// deleting a role would break the project.
func TestDeletingARoleRemovesItsGrid(t *testing.T) {
	t.Parallel()
	workspace, saved, grid := savedRoleWithGrid(t)
	if err := workspace.DeleteRole(saved.Path, saved.Revision); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Dir(grid)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the role's folder outlived it: %v", err)
	}
	if _, err := metadata.Load(workspace.root); err != nil {
		t.Fatalf("the project does not load after the role was deleted: %v", err)
	}
}

// A delete refused halfway leaves the role as it was, rights and all.
//
// Defect caught: the grid removed before the rest failed, which left a role
// that loads and grants nothing (review of block 2в, 2.431).
func TestARoleDeleteThatFailsHalfwayKeepsItsRights(t *testing.T) {
	t.Parallel()
	workspace, saved, grid := savedRoleWithGrid(t)
	before, err := os.ReadFile(grid)
	if err != nil {
		t.Fatal(err)
	}
	roles := filepath.Join(workspace.root, "metadata", "roles")
	// The folder of roles lets nothing go: the grid inside the role's own
	// folder can be removed, the folder and the description cannot.
	if err := os.Chmod(roles, 0o555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(roles, 0o755)
	if err := workspace.DeleteRole(saved.Path, saved.Revision); err == nil {
		t.Fatal("the delete went through a folder that lets nothing go")
	}
	if after, err := os.ReadFile(grid); err != nil || string(after) != string(before) {
		t.Fatalf("the role lost its rights: %q, %v", after, err)
	}
	if _, err := os.Stat(filepath.Join(workspace.root, filepath.FromSlash(saved.Path))); err != nil {
		t.Fatalf("the role lost its description: %v", err)
	}
}

// The description edited as text is checked with the grid beside it and
// written back with its restrictions.
//
// Defect caught: the description rewritten from the role without its
// restrictions, which dropped them on every save of the text.
func TestEditingARoleDescriptionAsTextKeepsItsRestrictions(t *testing.T) {
	t.Parallel()
	workspace, saved, _ := savedRoleWithGrid(t)
	source, err := workspace.ReadSource(saved.Path)
	if err != nil {
		t.Fatal(err)
	}
	edited := strings.Replace(source.Content, "name: Кладовщик", "name: Кладовщик\ncomment: правлено текстом", 1)
	written, err := workspace.SaveSource(saved.Path, edited, source.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(written.Content, "restrictions:") || !strings.Contains(written.Content, "правлено текстом") {
		t.Fatalf("the text came back without its restriction or the edit:\n%s", written.Content)
	}
	again, err := workspace.ReadRole(saved.Path)
	if err != nil || len(again.Role.Objects) != 1 || len(again.Role.Objects[0].Restrictions) != 1 {
		t.Fatalf("the role after a text edit: %+v, %v", again.Role.Objects, err)
	}
}

// twoRolesWithGrids is a workspace with two roles saved with grids, as the
// window «Все роли» reads them.
func twoRolesWithGrids(t *testing.T) (*Workspace, []RoleOverviewEntry) {
	t.Helper()
	workspace, first, _ := savedRoleWithGrid(t)
	second, err := workspace.CreateRole("Продавец")
	if err != nil {
		t.Fatal(err)
	}
	second.Role.Objects = []metadata.ObjectPermission{{Object: first.Role.Objects[0].Object, Operations: []metadata.PermissionOperation{metadata.PermissionView}}}
	if _, err := workspace.SaveRole(second.Path, second.Role, second.Revision); err != nil {
		t.Fatal(err)
	}
	overview, err := workspace.ReadAllRoles()
	if err != nil {
		t.Fatal(err)
	}
	if len(overview.Roles) != 2 {
		t.Fatalf("roles: %+v", overview.Roles)
	}
	return workspace, overview.Roles
}

// roleFilesOf is what lies on disk for a role: its description and its grid.
func roleFilesOf(t *testing.T, workspace *Workspace, entry RoleOverviewEntry) string {
	t.Helper()
	description, err := os.ReadFile(filepath.Join(workspace.root, filepath.FromSlash(entry.Path)))
	if err != nil {
		t.Fatal(err)
	}
	grid, _ := os.ReadFile(filepath.Join(workspace.root, "metadata", "roles", entry.Role.ID.String(), metadata.RoleRightsFile))
	return string(description) + "\n--\n" + string(grid)
}

// untouched tells whether the description of a role is still the very file
// it was: a role written and then put back is the same text in another file.
func untouched(t *testing.T, workspace *Workspace, entry RoleOverviewEntry, before os.FileInfo) bool {
	t.Helper()
	after, err := os.Stat(filepath.Join(workspace.root, filepath.FromSlash(entry.Path)))
	return err == nil && os.SameFile(before, after)
}

// withRight is the role of entry with one more right on its first object.
func withRight(entry RoleOverviewEntry, right metadata.PermissionOperation) RoleSave {
	role := entry.Role
	role.Objects = slices.Clone(role.Objects)
	role.Objects[0].Operations = append(slices.Clone(role.Objects[0].Operations), right)
	return RoleSave{Path: entry.Path, ExpectedRevision: entry.Revision, Role: role}
}

// The window «Все роли» saves the roles it changed in one go, and answers
// with every role as it now lies, each with its new revision.
//
// Defect caught: one role written and the next lost, or a revision not
// renewed, so the next save from the window is refused as a conflict.
func TestAllRolesAreSavedTogether(t *testing.T) {
	t.Parallel()
	workspace, roles := twoRolesWithGrids(t)
	saved, err := workspace.SaveRoles([]RoleSave{withRight(roles[0], metadata.PermissionInputByString), withRight(roles[1], metadata.PermissionInputByString)})
	if err != nil {
		t.Fatal(err)
	}
	for index, entry := range saved.Roles {
		if !slices.Contains(entry.Role.Objects[0].Operations, metadata.PermissionInputByString) {
			t.Fatalf("role %q came back without the right: %+v", entry.Role.Name, entry.Role.Objects)
		}
		if entry.Revision == roles[index].Revision {
			t.Fatalf("role %q kept its old revision", entry.Role.Name)
		}
	}
	if _, err := workspace.SaveRoles([]RoleSave{withRight(saved.Roles[0], metadata.PermissionView)}); err != nil {
		t.Fatalf("a second save from the window: %v", err)
	}
}

// One role changed on disk since the window read it refuses the whole save:
// no role is written, the one checked first included.
//
// Defect caught: the roles before the stale one written, the rest refused -
// half of one edit saved.
func TestAllRolesSaveOverAChangedRoleWritesNothing(t *testing.T) {
	t.Parallel()
	workspace, roles := twoRolesWithGrids(t)
	before := roleFilesOf(t, workspace, roles[0])
	file, err := os.Stat(filepath.Join(workspace.root, filepath.FromSlash(roles[0].Path)))
	if err != nil {
		t.Fatal(err)
	}
	stale := withRight(roles[1], metadata.PermissionInputByString)
	stale.ExpectedRevision = roles[0].Revision
	_, err = workspace.SaveRoles([]RoleSave{withRight(roles[0], metadata.PermissionInputByString), stale})
	if !errors.Is(err, ErrSourceChanged) {
		t.Fatalf("a save over a changed role: %v", err)
	}
	if roleFilesOf(t, workspace, roles[0]) != before || !untouched(t, workspace, roles[0], file) {
		t.Fatal("the first role was written although the save was refused")
	}
	// A role the project would not take - here named as another - is refused
	// before anything is written, though nothing on disk changed.
	clash := withRight(roles[1], metadata.PermissionView)
	clash.Role.Name = roles[0].Role.Name
	beforeSecond := roleFilesOf(t, workspace, roles[1])
	if _, err := workspace.SaveRoles([]RoleSave{withRight(roles[0], metadata.PermissionInputByString), clash}); err == nil {
		t.Fatal("a role named as another was saved")
	}
	if roleFilesOf(t, workspace, roles[0]) != before || !untouched(t, workspace, roles[0], file) || roleFilesOf(t, workspace, roles[1]) != beforeSecond {
		t.Fatal("a refused save wrote a role")
	}
}

// «Все роли» changes rights and keeps names: a save renaming a role is
// refused whole, before anything is written.
//
// Defect caught: two roles of one save given one new name, each checked
// against the disk alone and both written - a project that no longer loads
// (review of block 2в, 2.431; the owner, 21.10.2026: a role is renamed in its
// own window).
func TestAllRolesSaveKeepsTheNames(t *testing.T) {
	t.Parallel()
	workspace, roles := twoRolesWithGrids(t)
	before := []string{roleFilesOf(t, workspace, roles[0]), roleFilesOf(t, workspace, roles[1])}
	first, second := withRight(roles[0], metadata.PermissionInputByString), withRight(roles[1], metadata.PermissionInputByString)
	first.Role.Name, second.Role.Name = "Кладовщик2", "Кладовщик2"
	if _, err := workspace.SaveRoles([]RoleSave{first, second}); err == nil {
		t.Fatal("two roles were renamed alike in one save")
	}
	if roleFilesOf(t, workspace, roles[0]) != before[0] || roleFilesOf(t, workspace, roles[1]) != before[1] {
		t.Fatal("a refused save wrote a role")
	}
}

// A write that fails on a later role puts the roles written before it back
// as they lay, byte for byte.
//
// Defect caught: the first role saved and the second not - the window shows
// a refused save, and the project keeps half of it.
func TestAllRolesSaveThatFailsHalfwayPutsTheRolesBack(t *testing.T) {
	t.Parallel()
	workspace, roles := twoRolesWithGrids(t)
	first, second := roles[0], roles[1]
	before := roleFilesOf(t, workspace, first)
	// The second role's folder takes no new files, so its grid cannot be
	// replaced; the first role is written whole before it.
	folder := filepath.Join(workspace.root, "metadata", "roles", second.Role.ID.String())
	if err := os.Chmod(folder, 0o555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(folder, 0o755)
	if _, err := workspace.SaveRoles([]RoleSave{withRight(first, metadata.PermissionInputByString), withRight(second, metadata.PermissionInputByString)}); err == nil {
		t.Fatal("the save went through a folder that takes no files")
	}
	if after := roleFilesOf(t, workspace, first); after != before {
		t.Fatalf("the first role was left changed:\n%s", after)
	}
	// A role named twice in one save is refused by its own revision the
	// second time, and the first write of it is put back.
	if err := os.Chmod(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.SaveRoles([]RoleSave{withRight(first, metadata.PermissionView), withRight(first, metadata.PermissionView)}); !errors.Is(err, ErrSourceChanged) || roleFilesOf(t, workspace, first) != before {
		t.Fatalf("a role saved twice in one save: %v", err)
	}
}

// The route of the window answers a conflict as a conflict, so Studio says
// a role changed outside it rather than showing a bare error.
func TestAllRolesRouteAnswersAConflict(t *testing.T) {
	t.Parallel()
	workspace, roles := twoRolesWithGrids(t)
	stale := withRight(roles[0], metadata.PermissionView)
	stale.ExpectedRevision = roles[1].Revision
	body, _ := json.Marshal(map[string]any{"roles": []RoleSave{stale}})
	request := httptest.NewRequest(http.MethodPut, "http://localhost/api/roles", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-ML-CSRF", "1")
	response := httptest.NewRecorder()
	NewHandler(workspace).ServeHTTP(response, request)
	if response.Code != http.StatusConflict {
		t.Fatalf("PUT roles over a changed role: %d %s", response.Code, response.Body.String())
	}
}

// The grid of a role is read only as a plain file: a link put in its place
// is refused rather than followed out of the project.
//
// Defect caught: Studio reading, and the next save writing, through a link
// to a file anywhere on the disk.
func TestAGridOfRightsThatIsALinkIsRefused(t *testing.T) {
	t.Parallel()
	workspace, saved, grid := savedRoleWithGrid(t)
	outside := filepath.Join(t.TempDir(), "rights.json")
	content, err := os.ReadFile(grid)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outside, content, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(grid); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, grid); err != nil {
		t.Skipf("no links here: %v", err)
	}
	if _, err := workspace.ReadRole(saved.Path); !errors.Is(err, ErrInvalidSourcePath) {
		t.Fatalf("a grid that is a link was read: %v", err)
	}
}

// TestEveryWriteOfARoleTakesAwayTheFieldsItsObjectsHaveNot catches a write
// of a role keeping in its restrictions a field the object has not now -
// Родитель and ЭтоГруппа of a catalog with no hierarchy -, where the
// prototype's takes it away at any write of the role, whatever right the
// write changes, and a role not written keeps it (configurator 23.10.2026,
// the ninth and the tenth passes, absent-25, v2-*; ACCESS-RIGHTS
// «Ограничение по полю, которого у объекта нет»). The roles are written past
// Studio, as a file edited by hand or a project loaded leaves them.
// Defects caught: the window of a role (SaveRole) or «Все роли» (SaveRoles)
// writing the field; a restriction naming only such fields kept, or turned
// into one on «Прочие поля»; the role answered as it was sent, not as it was
// written; a role not written changed.
func TestEveryWriteOfARoleTakesAwayTheFieldsItsObjectsHaveNot(t *testing.T) {
	t.Parallel()
	workspace, object := roleWorkspace(t)
	read := metadata.PermissionRead
	for _, name := range []string{"Первая", "Вторая", "Третья"} {
		role := metadata.RoleDefinition{Format: metadata.CurrentFormat, ID: uuid.MustNew(), Name: name, Title: metadata.LocalizedText{"ru": name},
			Objects: []metadata.ObjectPermission{{Object: object.ID, Operations: []metadata.PermissionOperation{read},
				Restrictions: []metadata.RightRestriction{{Right: read, Fields: []string{"ref", "parent"}, Condition: "ГДЕ А"},
					{Right: read, Fields: []string{"isfolder"}, Condition: "ГДЕ Б"}}}}}
		if err := metadata.WriteRole(workspace.root, role); err != nil {
			t.Fatal(err)
		}
	}
	overview, err := workspace.ReadAllRoles()
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]RoleOverviewEntry{}
	for _, entry := range overview.Roles {
		byName[entry.Role.Name] = entry
	}
	written := []metadata.RightRestriction{{Right: read, Fields: []string{"ref"}, Condition: "ГДЕ А"}}
	kept := []metadata.RightRestriction{{Right: read, Fields: []string{"ref", "parent"}, Condition: "ГДЕ А"},
		{Right: read, Fields: []string{"isfolder"}, Condition: "ГДЕ Б"}}
	same := func(left, right []metadata.RightRestriction) bool {
		return slices.EqualFunc(left, right, func(a, b metadata.RightRestriction) bool {
			return a.Right == b.Right && slices.Equal(a.Fields, b.Fields) && a.Condition == b.Condition
		})
	}
	// The window of a role writes another right of the object.
	first := withRight(byName["Первая"], metadata.PermissionInsert)
	answered, err := workspace.SaveRole(first.Path, first.Role, first.ExpectedRevision)
	if err != nil {
		t.Fatal(err)
	}
	if got := answered.Role.Objects[0].Restrictions; !same(got, written) {
		t.Errorf("the window of a role was answered %+v, want %+v", got, written)
	}
	// «Все роли» writes a right the restrictions do not touch.
	if _, err := workspace.SaveRoles([]RoleSave{withRight(byName["Вторая"], metadata.PermissionView)}); err != nil {
		t.Fatal(err)
	}
	again, err := workspace.ReadAllRoles()
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range again.Roles {
		want := map[string][]metadata.RightRestriction{"Первая": written, "Вторая": written, "Третья": kept}[entry.Role.Name]
		if got := entry.Role.Objects[0].Restrictions; !same(got, want) {
			t.Errorf("role %s lies with %+v, want %+v", entry.Role.Name, got, want)
		}
	}
}
