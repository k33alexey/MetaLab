package studio

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/metadata"
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
		Policies: []metadata.AccessPolicy{{Operations: []metadata.PermissionOperation{metadata.PermissionRead},
			Rule: &metadata.PolicyRule{Field: object.Attributes[0].ID.String(), Operator: metadata.PolicyEqual, Values: []metadata.Value{{Kind: metadata.NumberType, Data: "1"}}}}}}}
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
	if len(role.Objects) != 1 || !slices.Equal(role.Objects[0].Denied, saved.Role.Objects[0].Denied) || len(role.Objects[0].Policies) != 1 {
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
	if err != nil || len(again.Role.Objects) != 1 || len(again.Role.Objects[0].Policies) != 1 {
		t.Fatalf("the role after a text edit: %+v, %v", again.Role.Objects, err)
	}
}
