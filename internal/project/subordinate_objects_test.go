package project

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// A table of an external data source is addressed like a catalog - a kind and a
// name - and its folder lies in its source's collection. Catches the address
// translated to the wrong folder, a name without its owner accepted, and an
// owner or a table name that is not an identifier let through into a path.
func TestSubordinateObjectIsAddressedByKindAndDottedName(t *testing.T) {
	t.Parallel()
	directory, err := ObjectDirectory("external-data-source-tables", "Склад.Товары")
	if err != nil || directory != "metadata/external-data-sources/Склад/tables/Товары" {
		t.Fatalf("the table's folder = %q, %v", directory, err)
	}
	for _, name := range []string{"Товары", "Склад.", ".Товары", "Склад.Това ры", "../Склад.Товары", "Склад/x.Товары"} {
		if directory, err := ObjectDirectory("external-data-source-tables", name); err == nil {
			t.Errorf("%q was accepted as %q", name, directory)
		}
	}
}

// Every path of an object folder reads back as the address it was built from,
// at either depth. Catches a parser that counts segments: it would read a
// table's module as belonging to the source, or the source's tables folder as
// an object of its own.
func TestSplitObjectPathReadsAPathBackAsItsAddress(t *testing.T) {
	t.Parallel()
	built := func(path string, err error) string {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return path
	}
	for _, test := range []struct {
		path, kind, name, rest string
	}{
		{built(ObjectMetadataPath("catalogs", "Товары")), "catalogs", "Товары", "object.yaml"},
		{built(ObjectFormModulePath("catalogs", "Товары", "ФормаЭлемента")), "catalogs", "Товары", "forms/ФормаЭлемента/МодульФормы.bsl"},
		{built(ObjectMetadataPath("external-data-sources", "Склад")), "external-data-sources", "Склад", "object.yaml"},
		{"metadata/external-data-sources/Склад/tables", "external-data-sources", "Склад", "tables"},
		{built(ObjectMetadataPath("external-data-source-tables", "Склад.Товары")), "external-data-source-tables", "Склад.Товары", "object.yaml"},
		{built(ObjectModulePath("external-data-source-tables", "Склад.Товары", ObjectModuleFile)), "external-data-source-tables", "Склад.Товары", ObjectModuleFile},
		{built(ObjectFormPath("external-data-source-tables", "Склад.Товары", "ФормаСписка")), "external-data-source-tables", "Склад.Товары", "forms/ФормаСписка/form.yaml"},
		{built(ObjectCommandModulePath("external-data-source-tables", "Склад.Товары", "Печать")), "external-data-source-tables", "Склад.Товары", "commands/Печать/МодульКоманды.bsl"},
		{built(ObjectDirectory("external-data-source-tables", "Склад.Товары")), "external-data-source-tables", "Склад.Товары", ""},
	} {
		kind, name, rest, ok := SplitObjectPath(test.path)
		if !ok || kind != test.kind || name != test.name || strings.Join(rest, "/") != test.rest {
			t.Errorf("%s read as %q %q %q %v, want %q %q %q", test.path, kind, name, strings.Join(rest, "/"), ok, test.kind, test.name, test.rest)
		}
	}
	for _, path := range []string{"modules/x.bsl", "metadata/roles/r.yaml", "metadata/unknown/Объект/object.yaml", "metadata/catalogs/Не так/object.yaml"} {
		if kind, name, _, ok := SplitObjectPath(path); ok {
			t.Errorf("%s was read as the object %s %s", path, kind, name)
		}
	}
}

// The walk of source files goes into the folders of subordinate objects the way
// it goes into a catalog's. Catches a table's module or form missing from what
// is compiled for the server, indexed by Studio and loaded by the debugger -
// all three take their list from this one walk.
func TestObjectFolderSourcePathsWalkSubordinateObjects(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, file := range []string{
		"metadata/external-data-sources/Склад/object.yaml",
		"metadata/external-data-sources/Склад/tables/Товары/object.yaml",
		"metadata/external-data-sources/Склад/tables/Товары/МодульОбъекта.bsl",
		"metadata/external-data-sources/Склад/tables/Товары/forms/ФормаСписка/form.yaml",
		"metadata/external-data-sources/Склад/tables/Товары/forms/ФормаСписка/МодульФормы.bsl",
		"metadata/external-data-sources/Склад/tables/Товары/commands/Печать/МодульКоманды.bsl",
		"metadata/external-data-sources/Склад/tables/Товары/templates/Макет/content.yaml",
		"metadata/external-data-sources/Склад/tables/Остатки/object.yaml",
	} {
		path := filepath.Join(root, filepath.FromSlash(file))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	paths, err := ObjectFolderSourcePaths(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"metadata/external-data-sources/Склад/object.yaml",
		"metadata/external-data-sources/Склад/tables/Товары/object.yaml",
		"metadata/external-data-sources/Склад/tables/Товары/МодульОбъекта.bsl",
		"metadata/external-data-sources/Склад/tables/Товары/forms/ФормаСписка/МодульФормы.bsl",
		"metadata/external-data-sources/Склад/tables/Товары/commands/Печать/МодульКоманды.bsl",
		"metadata/external-data-sources/Склад/tables/Остатки/object.yaml",
	} {
		if !slices.Contains(paths, want) {
			t.Errorf("the walk missed %s: %v", want, paths)
		}
	}
	for _, path := range paths {
		if strings.Contains(path, "/templates/") {
			t.Errorf("template content was listed as source: %s", path)
		}
	}
}
