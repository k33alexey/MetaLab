package studio

import (
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

func TestStudioHandlerServesShellAndSnapshot(t *testing.T) {
	t.Parallel()

	workspace, err := Open(createProject(t))
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(workspace)
	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/", nil))
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "ML Studio") || page.Header().Get("Content-Security-Policy") == "" {
		t.Fatalf("page status=%d headers=%v body=%s", page.Code, page.Header(), page.Body.String())
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

func createProject(t *testing.T) string {
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
