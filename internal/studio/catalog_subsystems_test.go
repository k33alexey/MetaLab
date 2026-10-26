package studio

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// A save of the window of a catalog writes the subsystems its page marks -
// a new member last, an unmarked one taken out - and a catalog refused after
// them puts every subsystem written back as it was - its content, in the
// canonical form every write of Studio gives a description. Defects
// caught: a mark not written into the subsystem, a member left behind, a
// refusal of the catalog leaving the subsystems changed for a catalog that
// was not saved.
func TestTheWindowOfACatalogWritesItsSubsystems(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	addTreeSubsystems(t, root)
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	const goods = "metadata/catalogs/Товары/object.yaml"
	sales := filepath.Join(root, "metadata", "subsystems", treeSalesSubsystem+".yaml")
	warehouse := filepath.Join(root, "metadata", "subsystems", treeWarehouseSubsystem+".yaml")
	read := func(path string) string {
		t.Helper()
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(content)
	}
	opened, err := workspace.ReadCatalogEditor(goods)
	if err != nil {
		t.Fatal(err)
	}
	var marked []string
	for _, place := range opened.Subsystems {
		if place.Member {
			marked = append(marked, place.Name)
		}
	}
	if !slices.Equal(marked, []string{"Склад"}) {
		t.Fatalf("Товары is shown in %v, want Склад", marked)
	}
	salesID, err := uuid.Parse(treeSalesSubsystem)
	if err != nil {
		t.Fatal(err)
	}
	only := []uuid.UUID{salesID}

	// Refused halfway: the folder of the catalog takes no file.
	folder := filepath.Join(root, "metadata", "catalogs", "Товары")
	if err := os.Chmod(folder, 0o555); err != nil {
		t.Fatal(err)
	}
	_, refused := workspace.SaveCatalogWindow(goods, CatalogWindowValue{CatalogDefinition: opened.Catalog, Subsystems: &only}, opened.Revision)
	if err := os.Chmod(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	if refused == nil {
		t.Fatal("the catalog was written into a folder that takes no file")
	}
	const member = "10000000-0000-4000-8000-000000000101"
	if strings.Contains(read(sales), member) || !strings.Contains(read(warehouse), member) {
		t.Errorf("the refusal left the subsystems changed:\n%s\n%s", read(sales), read(warehouse))
	}

	saved, err := workspace.SaveCatalogWindow(goods, CatalogWindowValue{CatalogDefinition: opened.Catalog, Subsystems: &only}, opened.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if text := read(sales); !strings.HasSuffix(strings.TrimSpace(text), member) {
		t.Errorf("Товары is not the last member of Продажи:\n%s", text)
	}
	if text := read(warehouse); strings.Contains(text, member) {
		t.Errorf("Товары is left in Склад:\n%s", text)
	}
	for _, place := range saved.Subsystems {
		if place.Member != (place.Name == "Продажи") {
			t.Errorf("after the save %s is shown marked %v", place.Name, place.Member)
		}
	}
}
