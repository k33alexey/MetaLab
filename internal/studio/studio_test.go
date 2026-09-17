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
	metadataPath, err := project.ObjectMetadataPath("catalogs", catalogID)
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
	// "reports" (Компоновка данных) has no content here and is hidden: report
	// layouts belong to Отчёты/Обработки objects, which do not exist yet.
	if snapshot.Manifest.Name != "SalesDemo" || len(snapshot.Tree.Children) != len(project.RootDirectories())-1 {
		t.Fatalf("snapshot = %+v", snapshot)
	}
	if !treeContains(snapshot.Tree, metadataPath) || !treeContains(snapshot.Tree, modulePath) {
		t.Fatalf("tree does not contain created sources: %+v", snapshot.Tree)
	}
	if !treeContainsTitle(snapshot.Tree, "Контрагенты") {
		t.Fatalf("tree does not expose metadata title: %+v", snapshot.Tree)
	}
	if treeContains(snapshot.Tree, "reports") || treeContainsTitle(snapshot.Tree, "Компоновка данных") {
		t.Fatalf("empty report layouts branch should be hidden: %+v", snapshot.Tree)
	}
	if treeContains(snapshot.Tree, "metadata/folders") || treeContainsTitle(snapshot.Tree, "Каталоги Studio") {
		t.Fatalf("empty Studio folders branch should be hidden: %+v", snapshot.Tree)
	}
}

func TestWorkspaceTreeAlwaysShowsFixedGroupsForDocumentsAndRegisters(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	for _, kind := range []string{"documents", "information-registers", "accumulation-registers"} {
		id := uuid.MustNew()
		metadataPath, err := project.ObjectMetadataPath(kind, id)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, filepath.FromSlash(metadataPath))), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(metadataPath)), []byte("format: 1\nname: Тест\ntitle: {ru: Тест}\n"), 0o644); err != nil {
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
		objectNode, ok := findNodeByID(snapshot.Tree, id.String())
		if !ok {
			t.Fatalf("%s: object node not found: %+v", kind, snapshot.Tree)
		}
		wantGroups := []string{"Формы", "Команды", "Макеты"}
		if kind == "documents" {
			wantGroups = append(wantGroups, "Реквизиты", "Табличные части")
		} else {
			wantGroups = append(wantGroups, "Измерения", "Ресурсы", "Реквизиты")
		}
		for _, want := range wantGroups {
			if !treeContainsTitle(objectNode, want) {
				t.Fatalf("%s: missing always-visible group %q: %+v", kind, want, objectNode)
			}
		}
	}
}

func TestWorkspaceTreeExposesLanguagesAsOneNodeAfterStyles(t *testing.T) {
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
	metadataNode, ok := findNodeByID(snapshot.Tree, "metadata")
	if !ok {
		t.Fatalf("metadata node not found: %+v", snapshot.Tree)
	}
	var matches []int
	stylesIndex, constantsIndex := -1, -1
	for index, child := range metadataNode.Children {
		if child.ID == "metadata/languages" {
			matches = append(matches, index)
		}
		if child.ID == "metadata/styles" {
			stylesIndex = index
		}
		if child.ID == "metadata/constants" {
			constantsIndex = index
		}
	}
	if len(matches) != 1 {
		t.Fatalf("expected exactly one languages node, found %d: %+v", len(matches), metadataNode.Children)
	}
	languagesIndex := matches[0]
	if stylesIndex == -1 || constantsIndex == -1 || languagesIndex != stylesIndex+1 || constantsIndex != languagesIndex+1 {
		t.Fatalf("languages node must sit directly between styles and constants: styles=%d languages=%d constants=%d", stylesIndex, languagesIndex, constantsIndex)
	}
	languagesNode := metadataNode.Children[languagesIndex]
	if len(languagesNode.Children) != 1 || languagesNode.Children[0].ID != "language:ru" || languagesNode.Children[0].Path != project.ManifestFile {
		t.Fatalf("unexpected languages node children: %+v", languagesNode.Children)
	}
	// The group node itself must stay inert (no Path), matching every other
	// metadata-group node — only individual languages are openable.
	if languagesNode.Path == project.ManifestFile {
		t.Fatalf("the languages GROUP node must not itself be openable: %+v", languagesNode)
	}
}

func TestWorkspaceTreeAlwaysShowsNameNeverLocalizedTitle(t *testing.T) {
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
	// The tree is the developer's technical view (1C Configurator style) —
	// it must show the object's Имя (Name) even when a localized Заголовок
	// (Title) is configured, since that Title is what end users see, not
	// what a developer navigates the tree by.
	content := "format: 1\nid: " + id.String() + "\nname: Режим\ntitle: {uk: Режим роботи, ru: Рабочий режим}\ntypes: [{kind: boolean}]\n"
	if err := os.WriteFile(absolute, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := workspace.Snapshot()
	if err != nil || !treeContainsTitle(snapshot.Tree, "Режим") {
		t.Fatalf("tree does not show the object's Name: error=%v tree=%+v", err, snapshot.Tree)
	}
	if treeContainsTitle(snapshot.Tree, "Рабочий режим") {
		t.Fatalf("tree must not show the localized Title instead of Name: %+v", snapshot.Tree)
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
