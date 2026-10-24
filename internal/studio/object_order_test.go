package studio

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// catalogTitles is the titles of the catalogs the tree shows, in its order.
func catalogTitles(t *testing.T, workspace *Workspace) []string {
	t.Helper()
	snapshot, err := workspace.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	node, ok := findNodeByID(snapshot.Tree, "metadata/catalogs")
	if !ok {
		t.Fatal("no branch of catalogs")
	}
	return titlesOf(node.Children)
}

// keptOrder is the order of a kind the root keeps, and whether it keeps one.
func keptOrder(t *testing.T, root, kind string) ([]uuid.UUID, bool) {
	t.Helper()
	configuration, err := project.ValidateLayout(root)
	if err != nil {
		t.Fatal(err)
	}
	order, kept := configuration.ObjectsOrder[kind]
	return order, kept
}

// The tree shows the objects of a kind in the order the root keeps, and a kind
// with no order kept by name; a new object stands last, the first one made
// starting the order from what the tree showed (owner, 24.10.2026). Defects
// caught: the tree by the alphabet whatever is kept; a new object put by
// name; an order started without the objects already there.
func TestTheTreeShowsTheKeptOrderAndANewObjectLast(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	// Two catalogs as if added by hand: the root keeps no order of catalogs.
	gamma, err := workspace.CreateCatalog("Гамма")
	if err != nil {
		t.Fatal(err)
	}
	alpha, err := workspace.CreateCatalog("Альфа")
	if err != nil {
		t.Fatal(err)
	}
	saveOrder(t, root, nil)
	if got := catalogTitles(t, workspace); !slices.Equal(got, []string{"Альфа", "Гамма"}) {
		t.Fatalf("with no order kept the tree shows %v, want by name", got)
	}

	beta, err := workspace.CreateCatalog("Бета")
	if err != nil {
		t.Fatal(err)
	}
	if got := catalogTitles(t, workspace); !slices.Equal(got, []string{"Альфа", "Гамма", "Бета"}) {
		t.Fatalf("the tree shows %v, want the new catalog last", got)
	}
	order, kept := keptOrder(t, root, "catalogs")
	if want := []uuid.UUID{alpha.Catalog.ID, gamma.Catalog.ID, beta.Catalog.ID}; !kept || !slices.Equal(order, want) {
		t.Fatalf("the root keeps %v (%v), want %v", order, kept, want)
	}

	// An order written by hand is what the tree shows.
	saveOrder(t, root, map[string][]uuid.UUID{"catalogs": {beta.Catalog.ID, gamma.Catalog.ID, alpha.Catalog.ID}})
	if got := catalogTitles(t, workspace); !slices.Equal(got, []string{"Бета", "Гамма", "Альфа"}) {
		t.Fatalf("the tree shows %v, want the order kept", got)
	}
}

// A deleted object leaves the order with it: a line left naming nothing would
// refuse the project. A new role stands last among the roles, and a deleted
// one leaves the order too. Defect caught: the line left behind.
func TestADeletedObjectLeavesTheOrder(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	first, err := workspace.CreateCatalog("Первый")
	if err != nil {
		t.Fatal(err)
	}
	second, err := workspace.CreateCatalog("Второй")
	if err != nil {
		t.Fatal(err)
	}
	if err := workspace.DeleteCatalog(first.Path, first.Revision); err != nil {
		t.Fatal(err)
	}
	if order, _ := keptOrder(t, root, "catalogs"); !slices.Equal(order, []uuid.UUID{second.Catalog.ID}) {
		t.Fatalf("the order of catalogs is %v after a delete", order)
	}

	role, err := workspace.CreateRole("Кладовщик")
	if err != nil {
		t.Fatal(err)
	}
	if order, _ := keptOrder(t, root, "roles"); !slices.Contains(order, role.Role.ID) || order[len(order)-1] != role.Role.ID {
		t.Fatalf("the new role is not last in %v", order)
	}
	if err := workspace.DeleteRole(role.Path, role.Revision); err != nil {
		t.Fatal(err)
	}
	if order, _ := keptOrder(t, root, "roles"); slices.Contains(order, role.Role.ID) {
		t.Fatalf("the deleted role is still in %v", order)
	}
}

// A step refused halfway leaves the project as it was. A creation whose order
// cannot be written takes the new catalog away; a delete whose folder does not
// go puts the line back in the order. Defects caught: a catalog left with no
// place in the order, and an order naming a catalog that is still there but
// lost its line.
func TestAnOrderStepRefusedHalfwayLeavesTheProjectAsItWas(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("a folder without write permission does not refuse root")
	}
	root := createProject(t)
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	kept, err := workspace.CreateCatalog("Остаётся")
	if err != nil {
		t.Fatal(err)
	}
	before, _ := keptOrder(t, root, "catalogs")

	// The root cannot be written: the order is refused, and so is the catalog.
	if err := os.Chmod(root, 0o555); err != nil {
		t.Fatal(err)
	}
	_, createErr := workspace.CreateCatalog("Лишний")
	if err := os.Chmod(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if createErr == nil {
		t.Fatal("a catalog whose order could not be written was made")
	}
	if _, err := os.Stat(filepath.Join(root, "metadata", "catalogs", "Лишний")); !os.IsNotExist(err) {
		t.Fatalf("the refused catalog was left behind: %v", err)
	}
	if after, _ := keptOrder(t, root, "catalogs"); !slices.Equal(after, before) {
		t.Fatalf("the order changed from %v to %v", before, after)
	}

	// The folder of catalogs cannot be changed: the catalog does not go, and
	// its line comes back.
	catalogs := filepath.Join(root, "metadata", "catalogs")
	if err := os.Chmod(catalogs, 0o555); err != nil {
		t.Fatal(err)
	}
	deleteErr := workspace.DeleteCatalog(kept.Path, kept.Revision)
	if err := os.Chmod(catalogs, 0o755); err != nil {
		t.Fatal(err)
	}
	if deleteErr == nil {
		t.Fatal("a catalog whose folder could not go was deleted")
	}
	if after, _ := keptOrder(t, root, "catalogs"); !slices.Equal(after, before) {
		t.Fatalf("the order is %v after a refused delete, want %v", after, before)
	}
}

// A catalog whose grant to the roles is refused after its place in the order
// was written leaves that place too: the order comes back as it was. Defect
// caught: the catalog taken away and its line left naming nothing, which
// refuses the project.
func TestARefusedGrantPutsTheOrderBack(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("a folder without write permission does not refuse root")
	}
	root := createProject(t)
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.CreateCatalog("Первый"); err != nil {
		t.Fatal(err)
	}
	role, err := workspace.CreateRole("ВсёНовое")
	if err != nil {
		t.Fatal(err)
	}
	role.Role.GrantNewObjectsByDefault = true
	if _, err := workspace.SaveRole(role.Path, role.Role, role.Revision); err != nil {
		t.Fatal(err)
	}
	before, _ := keptOrder(t, root, "catalogs")
	roles := filepath.Join(root, "metadata", "roles")
	if err := os.Chmod(roles, 0o555); err != nil {
		t.Fatal(err)
	}
	_, createErr := workspace.CreateCatalog("Второй")
	if err := os.Chmod(roles, 0o755); err != nil {
		t.Fatal(err)
	}
	if createErr == nil {
		t.Fatal("a catalog whose grant could not be written was made")
	}
	if _, err := os.Stat(filepath.Join(root, "metadata", "catalogs", "Второй")); !os.IsNotExist(err) {
		t.Fatalf("the refused catalog was left behind: %v", err)
	}
	if after, _ := keptOrder(t, root, "catalogs"); !slices.Equal(after, before) {
		t.Fatalf("the order is %v after a refused grant, want %v", after, before)
	}
}

// saveOrder writes the order the root keeps.
func saveOrder(t *testing.T, root string, order map[string][]uuid.UUID) {
	t.Helper()
	configuration, err := project.ValidateLayout(root)
	if err != nil {
		t.Fatal(err)
	}
	configuration.ObjectsOrder = order
	if err := project.SaveConfiguration(root, configuration); err != nil {
		t.Fatal(err)
	}
}
