package studio

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
)

// The configuration root owns exactly one session module, so opening it is what
// creates it: a developer looking for where session parameters get their values
// must find a module to write in, not an error about a missing file.
func TestOpenSessionModuleCreatesItOnceAndKeepsEdits(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	absolute := filepath.Join(root, project.SessionModuleFile)
	if _, err := os.Stat(absolute); !os.IsNotExist(err) {
		t.Fatalf("a new project already has a session module: %v", err)
	}
	created, err := workspace.OpenSessionModule()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(created.Content, "УстановкаПараметровСеанса") || created.Language != "bsl" {
		t.Fatalf("session module stub = %+v", created)
	}
	edited := "Процедура УстановкаПараметровСеанса(ИменаПараметровСеанса)\n\tПараметрыСеанса.ДоступныеСклады = \"Основной\";\nКонецПроцедуры\n"
	saved, err := workspace.SaveSource(created.Path, edited, created.Revision)
	if err != nil {
		t.Fatal(err)
	}
	again, err := workspace.OpenSessionModule()
	if err != nil {
		t.Fatal(err)
	}
	if again.Content != edited || again.Revision != saved.Revision {
		t.Fatalf("opening an existing session module replaced it: %+v", again)
	}
}

func TestSessionModuleRouteRequiresMutationHeaderAndReturnsTheFile(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(workspace)
	unguarded := httptest.NewRequest(http.MethodPost, "http://localhost/api/session-module", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, unguarded)
	if response.Code != http.StatusForbidden {
		t.Fatalf("POST without CSRF header: %d %s", response.Code, response.Body.String())
	}
	if _, err := os.Stat(filepath.Join(root, project.SessionModuleFile)); !os.IsNotExist(err) {
		t.Fatalf("a refused request still created the file: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "http://localhost/api/session-module", nil)
	request.Header.Set("X-ML-CSRF", "1")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("POST session module: %d %s", response.Code, response.Body.String())
	}
	var file SourceFile
	if err := json.Unmarshal(response.Body.Bytes(), &file); err != nil {
		t.Fatal(err)
	}
	if file.Path != project.SessionModuleFile || file.Language != "bsl" || file.Revision == "" {
		t.Fatalf("unexpected session module: %+v", file)
	}
}

// The session module must reach the compiled program under its canonical name,
// otherwise it is an editable file that never runs.
func TestSessionModuleIsCompiledIntoTheProjectProgram(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.OpenSessionModule(); err != nil {
		t.Fatal(err)
	}
	catalog, err := metadata.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	modules, err := metadata.LoadProjectModules(root, catalog)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, module := range modules {
		if module.Filename != project.SessionModuleFile {
			continue
		}
		found = true
		if module.Name != metadata.SessionModuleName {
			t.Fatalf("session module name = %q", module.Name)
		}
	}
	if !found {
		t.Fatalf("session module missing from %d project modules", len(modules))
	}
}

func TestRootModulesUI(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js is required for session module UI tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, node, "--test", "../../scripts/root-modules.test.mjs").CombinedOutput()
	if err != nil {
		t.Fatalf("root module UI tests: %v\n%s", err, output)
	}
}

// The root's modules are offered whether or not their file exists: a
// developer looking for "where does the application start" must find it in
// the root's menu instead of having to know it can be created. Opening makes
// the file with the module's empty routines, and opening again keeps what was
// written.
func TestRootModulesAreOfferedAndCreatedOnOpen(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	for id, file := range map[string]string{
		"session-module":     project.SessionModuleFile,
		"application-module": project.ApplicationModuleFile,
	} {
		if _, err := os.Stat(filepath.Join(root, file)); !os.IsNotExist(err) {
			t.Fatalf("%s exists before it is opened: %v", id, err)
		}
		opened, err := workspace.OpenRootModule(id)
		if err != nil {
			t.Fatal(err)
		}
		if opened.Path != file || opened.Language != "bsl" || opened.Content == "" {
			t.Fatalf("%s opened as %+v", id, opened)
		}
		if _, err := os.Stat(filepath.Join(root, file)); err != nil {
			t.Fatalf("%s not made by opening: %v", id, err)
		}
		// Opening again returns what is there rather than the stub again: the
		// second open must not overwrite what was written in between.
		again, err := workspace.OpenRootModule(id)
		if err != nil || again.Content != opened.Content {
			t.Fatalf("%s reopened as %+v (%v)", id, again, err)
		}
	}
	if _, err := workspace.OpenRootModule("common-module"); err == nil {
		t.Fatal("a module the root does not own was opened")
	}
}

// The application module must reach the compiled program under its own name,
// otherwise it is an editable file that never runs. It compiles as client code
// because that is where an application starts and stops.
func TestApplicationModuleIsCompiledIntoTheProjectProgram(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	source := "Процедура ПриНачалеРаботыСистемы()\n\nКонецПроцедуры\n"
	if err := os.WriteFile(filepath.Join(root, project.ApplicationModuleFile), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	catalog, err := metadata.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	modules, err := metadata.LoadProjectModules(root, catalog)
	if err != nil {
		t.Fatal(err)
	}
	for _, module := range modules {
		if module.Filename != project.ApplicationModuleFile {
			continue
		}
		if module.Name != metadata.ApplicationModuleName {
			t.Fatalf("the application module compiles under %q", module.Name)
		}
		if module.Source != source {
			t.Fatalf("the application module reached the program changed: %q", module.Source)
		}
		return
	}
	t.Fatalf("the application module did not reach the program: %+v", modules)
}

// The three modules of the root open from the head of its menu, in the
// prototype's order - application, session, external connection - and are no
// rows of the tree (inventory of block 00, konfiguraciya.md, snapshot
// konfiguraciya/15-menyu). Defects caught: a module missing from the menu, in
// another order or with other words, a command the page cannot tell to open
// as a module of the root (the external connection module did not open from
// 2.38 to 2.296), or a module left a row of the tree.
func TestRootKeepsThreeModules(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := workspace.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	want := []NodeModule{
		{Command: "Открыть модуль приложения", Path: project.ApplicationModuleFile},
		{Command: "Открыть модуль сеанса", Path: project.SessionModuleFile},
		{Command: "Открыть модуль внешнего соединения", Path: project.ExternalConnectionModuleFile},
	}
	if !reflect.DeepEqual(snapshot.Tree.Modules, want) {
		t.Fatalf("the root offers %+v, want %+v", snapshot.Tree.Modules, want)
	}
	for _, module := range want {
		for _, node := range snapshot.Tree.Children {
			if node.Path == module.Path || strings.HasSuffix(node.ID, "-module") {
				t.Fatalf("module %s is a row of the tree: %+v", module.Path, node)
			}
		}
		if _, err := workspace.OpenModule(module.Path); err != nil {
			t.Fatalf("module %s does not open: %v", module.Path, err)
		}
	}
}

// The ordinary application module is not carried (owner, 14.10.2026,
// METADATA-OBJECTS «Только управляемое приложение»): a file of its name at the
// project root is no module - the tree does not show it, Studio does not open
// it, the program is not compiled from it. Defects caught: the module of an
// application ML does not have still offered in the tree, opened, or compiled
// as if it ran.
func TestOrdinaryApplicationModuleIsNotCarried(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	const file = "МодульОбычногоПриложения.bsl"
	writeBSLTestSource(t, root, file, "Процедура ПриНачалеРаботыСистемы()\nКонецПроцедуры\n")
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := workspace.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	for _, node := range snapshot.Tree.Children {
		if node.ID == "ordinary-application-module" || node.Path == file || node.Title == "Модуль обычного приложения" {
			t.Fatalf("the ordinary application module is in the tree: %+v", node)
		}
	}
	for _, module := range snapshot.Tree.Modules {
		if module.Path == file {
			t.Fatalf("the ordinary application module is offered by the root: %+v", module)
		}
	}
	if _, err := workspace.OpenRootModule("ordinary-application-module"); err == nil {
		t.Fatal("the ordinary application module opened as a module of the root")
	}
	catalog, err := metadata.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	modules, err := metadata.LoadProjectModules(root, catalog)
	if err != nil {
		t.Fatal(err)
	}
	for _, module := range modules {
		if module.Filename == file {
			t.Fatalf("the ordinary application module is compiled: %+v", module.Name)
		}
	}
}

// The pictures of the root are no rows of the tree: the prototype's tree
// has none (snapshot dosyomka-2651/05), the picture of the main section
// opens from the palette of the root, «Картинка основного раздела:
// Открыть». Defects caught: a row of the picture of the main section coming
// back; a folder of the logo or of the splash, which ML does not carry
// (owner, 15.10.2026), shown.
func TestRootPicturesAreNoRowsOfTheTree(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	for _, folder := range []string{"Логотип", "Заставка", project.MainSectionPictureDirectory} {
		directory := filepath.Join(root, folder)
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, "100.png"), []byte("image"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := workspace.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	for _, node := range snapshot.Tree.Children {
		if node.Kind != "group" && node.Kind != "metadata-group" {
			t.Errorf("the root holds a row %q (%s) of no branch", node.Title, node.ID)
		}
	}
}
