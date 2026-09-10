package studio

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestWorkspaceSnapshotBuildsCanonicalTree(t *testing.T) {
	t.Parallel()

	root := createProject(t)
	catalogID, moduleID := uuid.MustNew(), uuid.MustNew()
	metadataPath, err := project.MetadataPath("catalogs", catalogID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, filepath.FromSlash(metadataPath))), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(metadataPath)), []byte("format: 1\nname: Контрагенты\ntitle: Контрагенты\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	modulePath, err := project.ModulePath(moduleID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(modulePath)), []byte("Процедура Тест()\nКонецПроцедуры\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := workspace.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Manifest.Name != "SalesDemo" || len(snapshot.Tree.Children) != len(project.RootDirectories()) {
		t.Fatalf("snapshot = %+v", snapshot)
	}
	if !treeContains(snapshot.Tree, metadataPath) || !treeContains(snapshot.Tree, modulePath) {
		t.Fatalf("tree does not contain created sources: %+v", snapshot.Tree)
	}
	if !treeContainsTitle(snapshot.Tree, "Контрагенты") {
		t.Fatalf("tree does not expose metadata title: %+v", snapshot.Tree)
	}
}

func TestWorkspaceTreeUsesLocalizedMetadataTitle(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	id := uuid.MustNew()
	relative, err := project.MetadataPath("constants", id)
	if err != nil {
		t.Fatal(err)
	}
	absolute := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "format: 1\nid: " + id.String() + "\nname: Режим\ntitle: {uk: Режим роботи, ru: Рабочий режим}\ntypes: [{kind: boolean}]\n"
	if err := os.WriteFile(absolute, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := workspace.Snapshot()
	if err != nil || !treeContainsTitle(snapshot.Tree, "Рабочий режим") {
		t.Fatalf("localized tree error=%v tree=%+v", err, snapshot.Tree)
	}
}

func TestStudioHandlerServesShellAndSnapshot(t *testing.T) {
	t.Parallel()

	workspace, err := Open(createProject(t))
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(workspace)
	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/", nil))
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "ML Studio") || !strings.Contains(page.Body.String(), "bsl-editor.js") ||
		!strings.Contains(page.Body.String(), "data-bsl-action=\"definition\"") || !strings.Contains(page.Body.String(), "/api/search?query=") ||
		!strings.Contains(page.Body.String(), "data-bsl-action=\"help\"") || !strings.Contains(page.Body.String(), "/api/bsl/help?query=") ||
		!strings.Contains(page.Body.String(), "tests.js") || !strings.Contains(page.Body.String(), "id=\"tests-open\"") ||
		page.Header().Get("Content-Security-Policy") == "" {
		t.Fatalf("page status=%d headers=%v body=%s", page.Code, page.Header(), page.Body.String())
	}
	asset := httptest.NewRecorder()
	handler.ServeHTTP(asset, httptest.NewRequest(http.MethodGet, "/ui/bsl-editor.js", nil))
	if asset.Code != http.StatusOK || !strings.Contains(asset.Body.String(), "createBSLEditor") ||
		!strings.Contains(asset.Body.String(), "/api/bsl/complete") || !strings.Contains(asset.Body.String(), "completion-item") ||
		!strings.Contains(asset.Body.String(), "/api/bsl/navigate") || !strings.Contains(asset.Body.String(), "/api/bsl/rename") ||
		!strings.Contains(asset.Body.String(), "/api/bsl/help/resolve") ||
		!strings.Contains(asset.Header().Get("Content-Type"), "javascript") {
		t.Fatalf("editor asset status=%d headers=%v", asset.Code, asset.Header())
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/project", nil))
	var snapshot Snapshot
	if response.Code != http.StatusOK {
		t.Fatalf("snapshot status=%d body=%s", response.Code, response.Body.String())
	}
	if err := json.Unmarshal(response.Body.Bytes(), &snapshot); err != nil || snapshot.Manifest.Title != "Продажи и склад" {
		t.Fatalf("snapshot=%+v error=%v", snapshot, err)
	}
}

func TestWorkspaceDiscoversTestProceduresAndServesTestAPI(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	id := uuid.MustNew()
	relative, err := project.TestPath(id)
	if err != nil {
		t.Fatal(err)
	}
	content := "Процедура Проверить() Экспорт\nКонецПроцедуры\nФункция НеТест() Экспорт\nВозврат Истина;\nКонецФункции\n"
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(relative)), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	cases, err := workspace.TestCases()
	if err != nil || len(cases) != 1 || cases[0].Routine != "Проверить" || cases[0].Path != relative {
		t.Fatalf("cases=%+v error=%v", cases, err)
	}
	snapshot, err := workspace.Snapshot()
	if err != nil || !treeContainsTitle(snapshot.Tree, "Проверить") {
		t.Fatalf("test tree error=%v tree=%+v", err, snapshot.Tree)
	}
	handler := NewHandler(workspace)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/tests", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Проверить") {
		t.Fatalf("test API status=%d body=%s", response.Code, response.Body.String())
	}
	run := httptest.NewRequest(http.MethodPost, "/api/tests/run", bytes.NewBufferString(`{}`))
	run.Header.Set("Content-Type", "application/json")
	run.Header.Set("X-ML-CSRF", "1")
	runResponse := httptest.NewRecorder()
	handler.ServeHTTP(runResponse, run)
	if runResponse.Code != http.StatusConflict || !strings.Contains(runResponse.Body.String(), "Debug database") {
		t.Fatalf("test run status=%d body=%s", runResponse.Code, runResponse.Body.String())
	}
}

func TestWorkspaceRejectsUnexpectedSourceFile(t *testing.T) {
	t.Parallel()

	root := createProject(t)
	if err := os.WriteFile(filepath.Join(root, "modules", "manual.bsl"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.Snapshot(); err == nil || !strings.Contains(err.Error(), "UUID") {
		t.Fatalf("Snapshot() error = %v", err)
	}
}

func TestOpenRejectsIncompleteProject(t *testing.T) {
	t.Parallel()

	if _, err := Open(t.TempDir()); err == nil {
		t.Fatal("Open() accepted a directory without an ML Project")
	}
}

func createProject(t testing.TB) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "SalesDemo")
	manifest := project.Project{
		Format: project.CurrentFormat, ID: uuid.MustNew(), Name: "SalesDemo", Title: "Продажи и склад",
		DefaultLanguage: "ru", Languages: []project.Language{{Name: "Русский", Title: "Русский", Code: "ru"}},
	}
	if err := project.Initialize(root, manifest); err != nil {
		t.Fatal(err)
	}
	return root
}

func treeContains(node Node, sourcePath string) bool {
	if node.Path == filepath.ToSlash(sourcePath) {
		return true
	}
	for _, child := range node.Children {
		if treeContains(child, sourcePath) {
			return true
		}
	}
	return false
}

func treeContainsTitle(node Node, title string) bool {
	if node.Title == title {
		return true
	}
	for _, child := range node.Children {
		if treeContainsTitle(child, title) {
			return true
		}
	}
	return false
}
