package studio

import (
	"os"
	"path/filepath"
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
