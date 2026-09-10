package studio

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/k33alexey/MetaLab/internal/bsl/compiler"
	"github.com/k33alexey/MetaLab/internal/bsl/vm"
	"github.com/k33alexey/MetaLab/internal/debugtarget"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestWorkspaceServerDebuggerUsesUnsavedSource(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	path, _ := project.ModulePath(uuid.MustNew())
	saved := "Функция Рассчитать(Значение)\nВозврат Значение;\nКонецФункции"
	writeBSLTestSource(t, root, path, saved)
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	unsaved := "Функция Рассчитать(Значение)\nРезультат = Значение + 2;\nВозврат Результат;\nКонецФункции"
	snapshot, err := workspace.StartDebug(studioDebugStart{
		Path: path, Content: unsaved, Routine: "Рассчитать", Arguments: []json.RawMessage{json.RawMessage("40")},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = workspace.StopDebug() }()
	if snapshot.State != vm.DebugPaused || snapshot.Frames[0].Location.Path != path {
		t.Fatalf("entry = %+v", snapshot)
	}
	value, err := workspace.EvaluateDebug(0, "Значение + 1")
	if err != nil || value.Value != "41" {
		t.Fatalf("EvaluateDebug() = %+v, %v", value, err)
	}
	session, _ := workspace.currentDebugSession()
	if _, err := workspace.DebugCommand(vm.DebugContinue); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	finished, err := session.Wait(ctx, snapshot.Sequence)
	if err != nil || finished.State != vm.DebugCompleted || finished.Result == nil || finished.Result.Value != "42" {
		t.Fatalf("finished = %+v, %v", finished, err)
	}
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil || string(content) != saved {
		t.Fatalf("debugger changed source: %q, %v", content, err)
	}
}

func TestStudioDebuggerHTTPAPIAndAssets(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	path, _ := project.ModulePath(uuid.MustNew())
	source := "Функция Запуск()\nВозврат 42;\nКонецФункции"
	writeBSLTestSource(t, root, path, source)
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(workspace)
	for _, target := range []string{"/", "/ui/debugger.js", "/ui/debugger.css", "/api/debug/targets"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, target, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("GET %s status=%d", target, response.Code)
		}
		if target == "/" && (!strings.Contains(response.Body.String(), `id="debug-panel"`) || !strings.Contains(response.Body.String(), `id="debug-target"`) || !strings.Contains(response.Body.String(), `/ui/debugger.js`)) {
			t.Fatal("Studio shell does not connect the server debugger")
		}
	}
	payload, _ := json.Marshal(studioDebugStart{Path: path, Content: source, Routine: "Запуск"})
	denied := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/debug/start", bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(denied, request)
	if denied.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF status = %d", denied.Code)
	}
	started := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/debug/start", bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-ML-CSRF", "1")
	handler.ServeHTTP(started, request)
	if started.Code != http.StatusOK || !strings.Contains(started.Body.String(), `"state":"paused"`) {
		t.Fatalf("start status=%d body=%s", started.Code, started.Body.String())
	}
	heartbeat := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/debug/heartbeat", nil)
	request.Header.Set("X-ML-CSRF", "1")
	handler.ServeHTTP(heartbeat, request)
	if heartbeat.Code != http.StatusOK || !strings.Contains(heartbeat.Body.String(), `"ok":true`) {
		t.Fatalf("heartbeat status=%d body=%s", heartbeat.Code, heartbeat.Body.String())
	}
	evaluated := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/debug/evaluate", strings.NewReader(`{"frame":0,"expression":"40 + 2"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-ML-CSRF", "1")
	handler.ServeHTTP(evaluated, request)
	if evaluated.Code != http.StatusOK || !strings.Contains(evaluated.Body.String(), `"value":"42"`) {
		t.Fatalf("evaluate status=%d body=%s", evaluated.Code, evaluated.Body.String())
	}
	stopped := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodDelete, "/api/debug/session", nil)
	request.Header.Set("X-ML-CSRF", "1")
	handler.ServeHTTP(stopped, request)
	if stopped.Code != http.StatusOK || !strings.Contains(stopped.Body.String(), `"state":"stopped"`) {
		t.Fatalf("stop status=%d body=%s", stopped.Code, stopped.Body.String())
	}
}

func TestWorkspaceDebugsRegisteredUserSessionTarget(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	manifest, err := project.ValidateLayout(root)
	if err != nil {
		t.Fatal(err)
	}
	path, _ := project.ModulePath(uuid.MustNew())
	source := "Функция Рассчитать(Значение)\nРезультат = Значение + 2;\nВозврат Результат;\nКонецФункции"
	writeBSLTestSource(t, root, path, source)
	program, diagnostics := compiler.CompileSource(path, source)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	machine, err := vm.New(program)
	if err != nil {
		t.Fatal(err)
	}
	endpoint, err := debugtarget.NewVMEndpoint(machine.NewContext())
	if err != nil {
		t.Fatal(err)
	}
	registry := debugtarget.NewRegistry()
	databaseID, sessionID := uuid.MustNew(), uuid.MustNew()
	registration, target, err := registry.Register(debugtarget.Descriptor{
		Kind: debugtarget.UserSession, ProjectID: manifest.ID, DatabaseID: databaseID, SessionID: &sessionID, Name: "alexey · Chrome",
	}, endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer registration.Close()
	workspace, err := OpenForDatabaseWithDebugTargets(root, databaseID, registry)
	if err != nil {
		t.Fatal(err)
	}
	targets, err := workspace.DebugTargets()
	if err != nil || len(targets) != 1 || targets[0].ID != target.ID {
		t.Fatalf("DebugTargets() = %+v, %v", targets, err)
	}
	started, err := workspace.StartDebug(studioDebugStart{
		TargetID: target.ID.String(), Path: path, Content: source, Routine: "Рассчитать",
		Arguments: []json.RawMessage{json.RawMessage("40")},
	})
	if err != nil {
		t.Fatal(err)
	}
	paused := waitWorkspaceDebugState(t, workspace, started, vm.DebugPaused)
	value, err := workspace.EvaluateDebug(0, "Значение + 1")
	if err != nil || value.Value != "41" {
		t.Fatalf("EvaluateDebug() = %+v, %v", value, err)
	}
	if _, err := workspace.DebugCommand(vm.DebugContinue); err != nil {
		t.Fatal(err)
	}
	completed := waitWorkspaceDebugState(t, workspace, paused, vm.DebugCompleted)
	if completed.Result == nil || completed.Result.Value != "42" {
		t.Fatalf("completed = %+v", completed)
	}
}

func TestWorkspaceRejectsDebugTargetFromAnotherProject(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	path, _ := project.ModulePath(uuid.MustNew())
	source := "Функция Запуск()\nВозврат 42;\nКонецФункции"
	writeBSLTestSource(t, root, path, source)
	program, diagnostics := compiler.CompileSource(path, source)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	machine, err := vm.New(program)
	if err != nil {
		t.Fatal(err)
	}
	endpoint, err := debugtarget.NewVMEndpoint(machine.NewContext())
	if err != nil {
		t.Fatal(err)
	}
	registry := debugtarget.NewRegistry()
	databaseID, jobID := uuid.MustNew(), uuid.MustNew()
	registration, target, err := registry.Register(debugtarget.Descriptor{
		Kind: debugtarget.BackgroundJob, ProjectID: uuid.MustNew(), DatabaseID: databaseID, JobID: &jobID, Name: "Foreign job",
	}, endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer registration.Close()
	workspace, err := OpenWithDebugTargets(root, registry)
	if err != nil {
		t.Fatal(err)
	}
	if targets, err := workspace.DebugTargets(); err != nil || len(targets) != 0 {
		t.Fatalf("foreign DebugTargets() = %+v, %v", targets, err)
	}
	_, err = workspace.StartDebug(studioDebugStart{TargetID: target.ID.String(), Path: path, Content: source, Routine: "Запуск"})
	if err == nil || !strings.Contains(err.Error(), "another ML Project") {
		t.Fatalf("StartDebug() error = %v", err)
	}
}

func TestWorkspaceFiltersAndRejectsDebugTargetFromAnotherDatabase(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	manifest, err := project.ValidateLayout(root)
	if err != nil {
		t.Fatal(err)
	}
	path, _ := project.ModulePath(uuid.MustNew())
	source := "Функция Запуск()\nВозврат 42;\nКонецФункции"
	writeBSLTestSource(t, root, path, source)
	program, diagnostics := compiler.CompileSource(path, source)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	machine, err := vm.New(program)
	if err != nil {
		t.Fatal(err)
	}
	endpoint, err := debugtarget.NewVMEndpoint(machine.NewContext())
	if err != nil {
		t.Fatal(err)
	}
	registry := debugtarget.NewRegistry()
	foreignDatabaseID, currentDatabaseID, sessionID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	registration, target, err := registry.Register(debugtarget.Descriptor{
		Kind: debugtarget.UserSession, ProjectID: manifest.ID, DatabaseID: foreignDatabaseID, SessionID: &sessionID, Name: "Foreign database session",
	}, endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer registration.Close()
	workspace, err := OpenForDatabaseWithDebugTargets(root, currentDatabaseID, registry)
	if err != nil {
		t.Fatal(err)
	}
	if targets, err := workspace.DebugTargets(); err != nil || len(targets) != 0 {
		t.Fatalf("foreign database DebugTargets() = %+v, %v", targets, err)
	}
	_, err = workspace.StartDebug(studioDebugStart{TargetID: target.ID.String(), Path: path, Content: source, Routine: "Запуск"})
	if err == nil || !strings.Contains(err.Error(), "another ML database") {
		t.Fatalf("StartDebug() error = %v", err)
	}
}

func waitWorkspaceDebugState(t *testing.T, workspace *Workspace, initial vm.DebugSnapshot, wanted vm.DebugState) vm.DebugSnapshot {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	snapshot := initial
	for snapshot.State != wanted {
		select {
		case <-ctx.Done():
			t.Fatalf("wait for debug state %s: last=%+v", wanted, snapshot)
		case <-time.After(time.Millisecond):
			var err error
			snapshot, err = workspace.DebugSnapshot()
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	return snapshot
}

func TestDecodeDebugArgumentPreservesNumbersAndRejectsObjects(t *testing.T) {
	t.Parallel()
	value, err := decodeDebugArgument(json.RawMessage(`[1.25,true,null]`), 0)
	if err != nil {
		t.Fatal(err)
	}
	first, ok := value.ArrayElement(0)
	if !ok || first.String() != "1.25" {
		t.Fatalf("decoded argument = %v", value)
	}
	if _, err := decodeDebugArgument(json.RawMessage(`{"unsafe":true}`), 0); err == nil {
		t.Fatal("decodeDebugArgument() accepted an object")
	}
}
