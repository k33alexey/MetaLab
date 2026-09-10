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

	"github.com/k33alexey/MetaLab/internal/bsl/vm"
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
	for _, target := range []string{"/", "/ui/debugger.js", "/ui/debugger.css"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, target, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("GET %s status=%d", target, response.Code)
		}
		if target == "/" && (!strings.Contains(response.Body.String(), `id="debug-panel"`) || !strings.Contains(response.Body.String(), `/ui/debugger.js`)) {
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
