package studio

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/metadata"
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

// The options of addCatalogOptions: Склады holds Товары itself, Артикулы
// only its attribute Артикул.
const (
	optionWarehouses = "30000000-0000-4000-8000-000000000401"
	optionArticles   = "30000000-0000-4000-8000-000000000402"
)

// addCatalogOptions gives the demo project two functional options kept in
// the attribute Артикул of Товары: Склады, whose content is Товары itself,
// and Артикулы, whose content is that attribute alone.
func addCatalogOptions(t *testing.T, root string) {
	t.Helper()
	directory := filepath.Join(root, "metadata", "functional-options")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	const goods, article = "10000000-0000-4000-8000-000000000101", "10000000-0000-4000-8000-000000001101"
	location := "location: {kind: catalogs, object: " + goods + ", element: " + article + "}\n"
	for id, body := range map[string]string{
		optionWarehouses: "format: 1\nid: " + optionWarehouses + "\nname: Склады\ntitle: {ru: Склады}\n" + location +
			"content:\n  - {kind: catalogs, object: " + goods + "}\n",
		optionArticles: "format: 1\nid: " + optionArticles + "\nname: Артикулы\ntitle: {ru: Артикулы}\n" + location +
			"content:\n  - {kind: catalogs, object: " + goods + ", element: " + article + "}\n",
	} {
		if err := os.WriteFile(filepath.Join(directory, id+".yaml"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// A save of the window of a catalog writes the options its page
// «Функциональные опции» marks: the catalog itself put last into the
// content of a marked option, beside the items naming its parts, and taken
// out of an unmarked one, its parts left there; a refusal of the options
// after the subsystems are written puts the subsystems back. Defects
// caught: a part of the catalog taken for the catalog - shown marked, or
// taken out with it; a mark not written into the option; an option refused
// leaving the subsystem written before it changed.
func TestTheWindowOfACatalogWritesItsOptions(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	addTreeSubsystems(t, root)
	addCatalogOptions(t, root)
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	const goods = "metadata/catalogs/Товары/object.yaml"
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
	var shown []string
	for _, place := range opened.FunctionalOptions {
		shown = append(shown, place.Name+map[bool]string{true: " ✓"}[place.Member])
	}
	if !slices.Equal(shown, []string{"Артикулы", "Склады ✓"}) {
		t.Fatalf("the options are shown as %v, want Артикулы, Склады ✓", shown)
	}
	articles, err := uuid.Parse(optionArticles)
	if err != nil {
		t.Fatal(err)
	}
	sales, err := uuid.Parse(treeSalesSubsystem)
	if err != nil {
		t.Fatal(err)
	}
	subsystems, options := []uuid.UUID{sales}, []uuid.UUID{articles}
	window := CatalogWindowValue{CatalogDefinition: opened.Catalog, Subsystems: &subsystems, FunctionalOptions: &options}
	salesFile := filepath.Join(root, "metadata", "subsystems", treeSalesSubsystem+".yaml")
	articlesFile := filepath.Join(root, "metadata", "functional-options", optionArticles+".yaml")
	warehousesFile := filepath.Join(root, "metadata", "functional-options", optionWarehouses+".yaml")
	before := map[string]string{salesFile: read(salesFile), articlesFile: read(articlesFile), warehousesFile: read(warehousesFile)}

	// Refused halfway: the subsystems are written, the folder of the options
	// takes no file.
	folder := filepath.Join(root, "metadata", "functional-options")
	if err := os.Chmod(folder, 0o555); err != nil {
		t.Fatal(err)
	}
	_, refused := workspace.SaveCatalogWindow(goods, window, opened.Revision)
	if err := os.Chmod(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	if refused == nil {
		t.Fatal("the options were written into a folder that takes no file")
	}
	const member = "10000000-0000-4000-8000-000000000101"
	if strings.Contains(read(salesFile), member) {
		t.Errorf("the refusal of the options left the subsystem changed:\n%s", read(salesFile))
	}
	for path, text := range before {
		if path != salesFile && read(path) != text {
			t.Errorf("the refusal left %s changed:\n%s", filepath.Base(path), read(path))
		}
	}

	saved, err := workspace.SaveCatalogWindow(goods, window, opened.Revision)
	if err != nil {
		t.Fatal(err)
	}
	text := read(articlesFile)
	text = text[strings.Index(text, "content:"):]
	part, whole := strings.Index(text, "element: 10000000-0000-4000-8000-000000001101"), strings.LastIndex(text, "object: "+member)
	if part < 0 || whole < part || strings.Count(text, "object: "+member) != 2 {
		t.Errorf("Товары is not put last into Артикулы beside its attribute:\n%s", text)
	}
	if text := read(warehousesFile); strings.Contains(text, "content") {
		t.Errorf("Товары is left in Склады:\n%s", text)
	}
	for _, place := range saved.FunctionalOptions {
		if place.Member != (place.Name == "Артикулы") {
			t.Errorf("after the save %s is shown marked %v", place.Name, place.Member)
		}
	}
	if _, err := metadata.Load(root); err != nil {
		t.Errorf("the options written do not load: %v", err)
	}
}
