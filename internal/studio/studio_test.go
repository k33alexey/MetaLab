package studio

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
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
	commonModuleID := uuid.MustNew()
	commonModulePath, err := project.MetadataPath("common-modules", commonModuleID)
	if err != nil {
		t.Fatal(err)
	}
	commonModule := "format: 1\nid: " + commonModuleID.String() + "\nname: ОбщегоНазначения\ntitle: {ru: Общего назначения}\n" +
		"module: " + moduleID.String() + "\nserver: true\n"
	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, filepath.FromSlash(commonModulePath))), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(commonModulePath)), []byte(commonModule), 0o644); err != nil {
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
	// The top level is the configuration itself: the session module, the
	// "Общие" group, then the object kinds. Storage directories are not
	// branches - a module or a form is reached through the object owning it.
	wantTop := []string{
		"session-module", "metadata/common", "metadata/constants", "metadata/catalogs",
		"metadata/documents", "metadata/document-journals", "metadata/enumerations",
		"metadata/reports", "metadata/data-processors", "metadata/charts-of-characteristic-types",
		"metadata/charts-of-accounts", "metadata/information-registers",
		"metadata/accumulation-registers", "metadata/accounting-registers",
	}
	var top []string
	for _, child := range snapshot.Tree.Children {
		top = append(top, child.ID)
	}
	if snapshot.Manifest.Name != "SalesDemo" || !reflect.DeepEqual(top, wantTop) {
		t.Fatalf("top level = %v, want %v", top, wantTop)
	}
	if first := snapshot.Tree.Children[0]; first.Path != project.SessionModuleFile {
		t.Fatalf("session module node = %+v", first)
	}
	if !treeContains(snapshot.Tree, metadataPath) || !treeContains(snapshot.Tree, commonModulePath) {
		t.Fatalf("tree does not contain created sources: %+v", snapshot.Tree)
	}
	// The common module's own BSL hangs under the common module, which is the
	// only thing that can lead a developer to it.
	commonModuleNode, ok := findNodeByID(snapshot.Tree, commonModuleID.String())
	if !ok || len(commonModuleNode.Children) != 1 || commonModuleNode.Children[0].Path != modulePath {
		t.Fatalf("common module node = %+v found=%v", commonModuleNode, ok)
	}
	if !treeContainsTitle(snapshot.Tree, "Контрагенты") {
		t.Fatalf("tree does not expose metadata title: %+v", snapshot.Tree)
	}
	for _, title := range []string{"Модули", "Компоновка данных", "Ресурсы", "Метаданные"} {
		if treeContainsTitle(snapshot.Tree, title) {
			t.Fatalf("storage directory %q must not be a tree branch: %+v", title, snapshot.Tree)
		}
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
	metadataNode, ok := findNodeByID(snapshot.Tree, "metadata/common")
	if !ok {
		t.Fatalf("\"Общие\" node not found: %+v", snapshot.Tree)
	}
	var matches []int
	stylesIndex := -1
	for index, child := range metadataNode.Children {
		if child.ID == "metadata/languages" {
			matches = append(matches, index)
		}
		if child.ID == "metadata/styles" {
			stylesIndex = index
		}
	}
	if len(matches) != 1 {
		t.Fatalf("expected exactly one languages node, found %d: %+v", len(matches), metadataNode.Children)
	}
	languagesIndex := matches[0]
	if stylesIndex == -1 || languagesIndex != stylesIndex+1 || languagesIndex != len(metadataNode.Children)-1 {
		t.Fatalf("languages node must close \"Общие\" directly after styles: styles=%d languages=%d of %d", stylesIndex, languagesIndex, len(metadataNode.Children))
	}
	if _, ok := findNodeByID(snapshot.Tree, "metadata/constants"); !ok {
		t.Fatalf("constants must stand beside \"Общие\", not inside it: %+v", snapshot.Tree)
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
		DefaultLanguage: "ru", Languages: []project.Language{{ID: uuid.MustNew(), Name: "Русский", Title: "Русский", Code: "ru"}},
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
