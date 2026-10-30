package studio

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// The forms of an object stand in the developer's order its description
// keeps (METADATA-OBJECTS.md, «Формат порядка в описании объекта»; the
// prototype shows them in its own order, досъёмка 29.10.2026, вечер) in the
// tree, the window of the object and the choice of a form of the home page,
// those the order leaves out after the others by name; the palette writes
// the description back with its order. Defects caught: a place showing the
// forms by name or as the folder lists them, the order lost on a save of
// the palette.
func TestTheFormsOfAnObjectStandInTheirOrder(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	catalog, first, second, third := uuid.MustNew(), uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	writeProjectFile(t, root, "metadata/catalogs/Товары/object.yaml", "format: 1\nid: "+catalog.String()+"\nname: Товары\ntitle: {ru: Товары}\n"+
		"code: {type: string, length: 9}\ndescription_length: 25\norder: {forms: ["+third.String()+", "+first.String()+"]}\n")
	for id, name := range map[uuid.UUID]string{first: "ФормаЭлемента", second: "ФормаВыбора", third: "ФормаСписка"} {
		writeProjectFile(t, root, "metadata/catalogs/Товары/forms/"+name+"/form.yaml",
			"format: 1\nid: "+id.String()+"\nname: "+name+"\ntitle: {ru: "+name+"}\nkind: list\n")
	}
	want := []string{"ФормаСписка", "ФормаЭлемента", "ФормаВыбора"}
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := workspace.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	forms, ok := findNodeByID(snapshot.Tree, catalog.String()+":forms")
	if !ok {
		t.Fatal("the catalog stands without «Формы»")
	}
	var tree []string
	for _, form := range forms.Children {
		tree = append(tree, form.Title)
	}
	if !slices.Equal(tree, want) {
		t.Errorf("the tree shows the forms %v, want %v", tree, want)
	}
	source, err := workspace.ReadCatalogEditor("metadata/catalogs/Товары/object.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var window []string
	for _, form := range source.Forms {
		window = append(window, form.Name)
	}
	if !slices.Equal(window, want) {
		t.Errorf("the window shows the forms %v, want %v", window, want)
	}
	page, err := workspace.ReadHomePage()
	if err != nil {
		t.Fatal(err)
	}
	var choice []string
	var collect func(groups []ObjectChoiceBranch)
	collect = func(groups []ObjectChoiceBranch) {
		for _, group := range groups {
			collect(group.Children)
			for _, object := range group.Objects {
				for _, branch := range object.Items {
					for _, form := range branch.Items {
						if object.Name == "Товары" {
							choice = append(choice, form.Name)
						}
					}
				}
			}
		}
	}
	collect(page.Forms)
	if !slices.Equal(choice, want) {
		t.Errorf("the choice of a form of the home page shows %v, want %v", choice, want)
	}
	source.Catalog.Comment = "Правка палитры"
	if _, err := workspace.SaveCatalogEditor("metadata/catalogs/Товары/object.yaml", source.Catalog, source.Revision); err != nil {
		t.Fatal(err)
	}
	written, err := os.ReadFile(filepath.Join(root, "metadata", "catalogs", "Товары", "object.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(written), "order:\n  forms:\n    - "+third.String()+"\n    - "+first.String()+"\n") {
		t.Errorf("the palette wrote the description as\n%s", written)
	}
}
