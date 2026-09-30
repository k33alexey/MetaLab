package studio

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
)

// writeProjectFile writes one file of a project, making its folders.
func writeProjectFile(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A WS reference, a WebSocket client and an HTTP service each keep a folder,
// and the tree shows each of them in its branch with what it holds. Catches a
// kind that moved to a folder and fell out of the tree, a module hanging under
// its object without a name to click on, and a service description shown as
// if it were source.
func TestNamedFolderKindsStandInTheTreeWithTheirModules(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	writeProjectFile(t, root, "metadata/ws-references/Склад/object.yaml",
		"format: 1\nid: f9000000-0000-4000-8000-000000000001\nname: Склад\ntitle: {ru: Склад}\n")
	writeProjectFile(t, root, "metadata/ws-references/Склад/definition.xml",
		`<definitions xmlns="http://schemas.xmlsoap.org/wsdl/"/>`)
	writeProjectFile(t, root, "metadata/websocket-clients/Биржа/object.yaml",
		"format: 1\nid: f9000000-0000-4000-8000-000000000002\nname: Биржа\ntitle: {ru: Биржа}\n")
	writeProjectFile(t, root, "metadata/websocket-clients/Биржа/"+project.ClientModuleFile,
		"Процедура ПриПолученииСообщения(Соединение, Сообщение)\nКонецПроцедуры\n")
	writeProjectFile(t, root, "metadata/http-services/Обмен/object.yaml",
		"format: 1\nid: f9000000-0000-4000-8000-000000000003\nname: Обмен\ntitle: {ru: Обмен}\nroot_url: exchange\n")
	writeProjectFile(t, root, "metadata/http-services/Обмен/"+project.ServiceModuleFile, "\n")

	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := workspace.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	reference, ok := findNodeByID(snapshot.Tree, "metadata/ws-references/Склад/object.yaml")
	if !ok || reference.Title != "Склад" {
		t.Fatalf("the WS reference is not in the tree: %+v", reference)
	}
	if len(reference.Children) != 0 {
		t.Fatalf("the service description hangs under the reference as source: %+v", reference.Children)
	}
	for path, title := range map[string]string{
		"metadata/websocket-clients/Биржа/" + project.ClientModuleFile: "Модуль клиента",
		"metadata/http-services/Обмен/" + project.ServiceModuleFile:    "Модуль сервиса",
	} {
		module, ok := findNodeByID(snapshot.Tree, path)
		if !ok || module.Title != title {
			t.Fatalf("the module %s stands in the tree as %+v", path, module)
		}
		if _, err := workspace.ReadSource(path); err != nil {
			t.Fatalf("the module %s does not open: %v", path, err)
		}
	}
	if _, err := workspace.ReadSource("metadata/ws-references/Склад/definition.xml"); err == nil {
		t.Fatal("the service description opens as source")
	}
}

// Every module a kind with a named folder may keep has a name in the tree.
// Catches the next kind that gains a module and hangs it under its object as
// a node with no title - which is how the modules of HTTP and web services
// stood until a WebSocket client made the gap visible.
func TestEveryNamedFolderModuleHasATitle(t *testing.T) {
	t.Parallel()
	for _, kind := range project.NamedFolderKinds() {
		files, _ := project.NamedFolderFiles(kind)
		for _, file := range files {
			if filepath.Ext(file) == ".bsl" && namedFolderFileTitles[file] == "" {
				t.Errorf("%s keeps %s, and the tree has no name for it", kind, file)
			}
		}
	}
}

// A source shows its tables, and a table shows what a catalog shows: its
// forms, commands, templates and modules. Catches the tables missing from the
// tree - which they were - a table's files that do not open, and a table's
// description saved without the strict reading every description gets.
func TestExternalDataSourceTablesStandInTheTreeAndOpen(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	const table = "metadata/external-data-sources/Склад/tables/Товары"
	writeProjectFile(t, root, "metadata/external-data-sources/Склад/object.yaml",
		"format: 1\nid: f9000000-0000-4000-8000-000000000011\nname: Склад\ntitle: {ru: Склад}\n")
	const description = "format: 1\nid: f9000000-0000-4000-8000-000000000012\nname: Товары\ntitle: {ru: Товары}\n" +
		"name_in_data_source: dbo.Goods\ndata_type: non-object\n"
	writeProjectFile(t, root, table+"/object.yaml", description)
	writeProjectFile(t, root, "metadata/external-data-sources/Пустой/object.yaml",
		"format: 1\nid: f9000000-0000-4000-8000-000000000013\nname: Пустой\ntitle: {ru: Пустой}\n")

	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := workspace.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	tables, ok := findNodeByID(snapshot.Tree, "metadata/external-data-sources/Склад/tables")
	if !ok || tables.Title != "Таблицы" || len(tables.Children) != 1 || tables.Children[0].Title != "Товары" ||
		tables.Children[0].Path != table+"/object.yaml" || tables.Children[0].Kind != "external-data-source-tables" {
		t.Fatalf("the tables of the source stand in the tree as %+v", tables)
	}
	groups := map[string]bool{}
	for _, child := range tables.Children[0].Children {
		groups[child.Title] = true
	}
	for _, group := range []string{"Формы", "Команды", "Макеты"} {
		if !groups[group] {
			t.Errorf("the table has no %s group: %+v", group, tables.Children[0].Children)
		}
	}
	// A source with no tables still shows the branch, empty.
	if empty, ok := findNodeByID(snapshot.Tree, "metadata/external-data-sources/Пустой/tables"); !ok || len(empty.Children) != 0 {
		t.Fatalf("a source without tables stands in the tree as %+v %v", empty, ok)
	}

	opened, err := workspace.ReadSource(table + "/object.yaml")
	if err != nil {
		t.Fatalf("the table's description does not open: %v", err)
	}
	if _, err := workspace.SaveSource(table+"/object.yaml", description+"unknown_property: 1\n", opened.Revision); err == nil ||
		!strings.Contains(err.Error(), "unknown_property") {
		t.Fatalf("a table's description was saved without being read strictly: %v", err)
	}
	if _, err := workspace.SaveSource(table+"/object.yaml", strings.Replace(description, "name: Товары", "name: Остатки", 1), opened.Revision); err == nil {
		t.Fatal("a table's description was saved under the name of another folder")
	}
	for _, path := range []string{table + "/МодульОбъекта.bsl", table + "/forms/ФормаСписка/form.yaml", table + "/commands/Печать/МодульКоманды.bsl"} {
		if _, _, err := validateEditablePath(path); err != nil {
			t.Errorf("%s is not a path Studio edits: %v", path, err)
		}
	}
	if module, err := formModulePath(table + "/forms/ФормаСписка/form.yaml"); err != nil || module != table+"/forms/ФормаСписка/МодульФормы.bsl" {
		t.Fatalf("the module of a table's form is %q, %v", module, err)
	}
}

// A source shows its cubes, and a cube shows its dimension tables beneath its
// own forms and modules, two levels below the source. Catches the cubes missing
// from the tree, the dimension tables' folder refused as a stray folder of the
// cube, and a cube's description saved without the strict reading.
func TestExternalDataSourceCubesStandInTheTree(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	const cube = "metadata/external-data-sources/Склад/cubes/Продажи"
	writeProjectFile(t, root, "metadata/external-data-sources/Склад/object.yaml",
		"format: 1\nid: f9000000-0000-4000-8000-000000000021\nname: Склад\ntitle: {ru: Склад}\n")
	const description = "format: 1\nid: f9000000-0000-4000-8000-000000000022\nname: Продажи\ntitle: {ru: Продажи}\nname_in_data_source: Sales\n"
	writeProjectFile(t, root, cube+"/object.yaml", description)
	writeProjectFile(t, root, cube+"/МодульМенеджера.bsl", "\n")
	writeProjectFile(t, root, cube+"/dimension-tables/Товары/object.yaml",
		"format: 1\nid: f9000000-0000-4000-8000-000000000023\nname: Товары\ntitle: {ru: Товары}\nname_in_data_source: Goods\n")

	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := workspace.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	cubes, ok := findNodeByID(snapshot.Tree, "metadata/external-data-sources/Склад/cubes")
	if !ok || cubes.Title != "Кубы" || len(cubes.Children) != 1 || cubes.Children[0].Path != cube+"/object.yaml" {
		t.Fatalf("the cubes of the source stand in the tree as %+v", cubes)
	}
	dimensions, ok := findNodeByID(snapshot.Tree, cube+"/dimension-tables")
	if !ok || dimensions.Title != "Таблицы измерений" || len(dimensions.Children) != 1 ||
		dimensions.Children[0].Path != cube+"/dimension-tables/Товары/object.yaml" {
		t.Fatalf("the dimension tables of the cube stand in the tree as %+v", dimensions)
	}
	if module, ok := findNodeByID(snapshot.Tree, cube+"/МодульМенеджера.bsl"); !ok || module.Title != "Модуль менеджера" {
		t.Fatalf("the cube's module stands in the tree as %+v", module)
	}
	opened, err := workspace.ReadSource(cube + "/object.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.SaveSource(cube+"/object.yaml", description+"measures: []\n", opened.Revision); err == nil || !strings.Contains(err.Error(), "measures") {
		t.Fatalf("a cube's description was saved without being read strictly: %v", err)
	}
}
