package studio

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
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

// The tree always shows the node, and says whether the file is there yet -
// otherwise "нет узла" and "модуль пустой" would look the same.
func TestSessionModuleNodeReportsWhetherItExists(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	state := func() string {
		snapshot, err := workspace.Snapshot()
		if err != nil {
			t.Fatal(err)
		}
		for _, property := range snapshot.Tree.Children[0].Properties {
			if property.Name == "Состояние" {
				return property.Value
			}
		}
		t.Fatal("session module node has no state property")
		return ""
	}
	if state() != "не создан" {
		t.Fatalf("state before creation = %q", state())
	}
	if _, err := workspace.OpenSessionModule(); err != nil {
		t.Fatal(err)
	}
	if state() != "создан" {
		t.Fatalf("state after creation = %q", state())
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

// The root owns two modules, and both stand in the tree whether or not their
// file exists: a developer looking for "where does the application start" must
// find it there instead of having to know it can be created.
func TestBothRootModulesStandInTheTreeAndAreCreatedOnOpen(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	state := func(id string) string {
		snapshot, err := workspace.Snapshot()
		if err != nil {
			t.Fatal(err)
		}
		for _, child := range snapshot.Tree.Children {
			if child.ID != id {
				continue
			}
			for _, property := range child.Properties {
				if property.Name == "Состояние" {
					return property.Value
				}
			}
		}
		t.Fatalf("the tree has no node %s with a state", id)
		return ""
	}
	for id, file := range map[string]string{
		"session-module":     project.SessionModuleFile,
		"application-module": project.ApplicationModuleFile,
	} {
		if state(id) != "не создан" {
			t.Fatalf("%s before creation = %q", id, state(id))
		}
		opened, err := workspace.OpenRootModule(id)
		if err != nil {
			t.Fatal(err)
		}
		if opened.Path != file || opened.Language != "bsl" || opened.Content == "" {
			t.Fatalf("%s opened as %+v", id, opened)
		}
		if state(id) != "создан" {
			t.Fatalf("%s after creation = %q", id, state(id))
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
