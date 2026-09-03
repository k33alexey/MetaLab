package studio

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestReadAndSaveSource(t *testing.T) {
	t.Parallel()

	workspace, relative, filePath := createModuleSource(t, "Переменная = 1;\n")
	opened, err := workspace.ReadSource(relative)
	if err != nil {
		t.Fatal(err)
	}
	if opened.Language != "bsl" || opened.Content != "Переменная = 1;\n" || len(opened.Revision) != 64 {
		t.Fatalf("opened source = %+v", opened)
	}
	saved, err := workspace.SaveSource(relative, "Переменная = 2;\n", opened.Revision)
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filePath)
	if err != nil || string(content) != saved.Content || saved.Revision == opened.Revision {
		t.Fatalf("saved=%+v disk=%q error=%v", saved, content, err)
	}
}

func TestSaveRejectsExternalChangeWithoutOverwritingIt(t *testing.T) {
	t.Parallel()

	workspace, relative, filePath := createModuleSource(t, "Исходный();\n")
	opened, err := workspace.ReadSource(relative)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filePath, []byte("ВнешнийРедактор();\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.SaveSource(relative, "Studio();\n", opened.Revision); !errors.Is(err, ErrSourceChanged) {
		t.Fatalf("SaveSource() error = %v, want ErrSourceChanged", err)
	}
	content, err := os.ReadFile(filePath)
	if err != nil || string(content) != "ВнешнийРедактор();\n" {
		t.Fatalf("external content = %q, error=%v", content, err)
	}
}

func TestConcurrentSavesHaveOneWinner(t *testing.T) {
	t.Parallel()

	workspace, relative, _ := createModuleSource(t, "Исходный();\n")
	opened, err := workspace.ReadSource(relative)
	if err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 2)
	var start sync.WaitGroup
	start.Add(1)
	for _, content := range []string{"Первый();\n", "Второй();\n"} {
		content := content
		go func() {
			start.Wait()
			_, saveErr := workspace.SaveSource(relative, content, opened.Revision)
			results <- saveErr
		}()
	}
	start.Done()
	successes, conflicts := 0, 0
	for range 2 {
		err := <-results
		if err == nil {
			successes++
		} else if errors.Is(err, ErrSourceChanged) {
			conflicts++
		} else {
			t.Fatalf("unexpected save error: %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes=%d conflicts=%d", successes, conflicts)
	}
}

func TestSourcePathsRejectTraversalAndSymlinks(t *testing.T) {
	t.Parallel()

	workspace, _, filePath := createModuleSource(t, "Тест();\n")
	for _, unsafe := range []string{"../mlproject.yaml", "/etc/passwd", `modules\\file.bsl`, "assets/file.txt"} {
		if _, err := workspace.ReadSource(unsafe); !errors.Is(err, ErrInvalidSourcePath) {
			t.Errorf("ReadSource(%q) error = %v", unsafe, err)
		}
	}
	if runtime.GOOS == "windows" {
		return
	}
	id := uuid.MustNew()
	relative, err := project.ModulePath(id)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filePath, filepath.Join(workspace.root, filepath.FromSlash(relative))); err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.ReadSource(relative); !errors.Is(err, ErrInvalidSourcePath) {
		t.Fatalf("symlink error = %v, want ErrInvalidSourcePath", err)
	}
}

func TestManifestSaveIsValidatedCanonicalAndIdentitySafe(t *testing.T) {
	t.Parallel()

	workspace, err := Open(createProject(t))
	if err != nil {
		t.Fatal(err)
	}
	opened, err := workspace.ReadSource(project.ManifestFile)
	if err != nil {
		t.Fatal(err)
	}
	updated := strings.Replace(opened.Content, "title: Продажи и склад", "title: Новое название", 1)
	saved, err := workspace.SaveSource(project.ManifestFile, updated, opened.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(saved.Content, "title: Новое название\n") {
		t.Fatalf("canonical manifest = %s", saved.Content)
	}
	changedID := strings.Replace(saved.Content, saved.Content[strings.Index(saved.Content, "id: ")+4:strings.Index(saved.Content, "id: ")+40], uuid.MustNew().String(), 1)
	if _, err := workspace.SaveSource(project.ManifestFile, changedID, saved.Revision); !errors.Is(err, project.ErrProjectIdentityChanged) {
		t.Fatalf("identity change error = %v", err)
	}
}

func TestSaveRejectsMalformedYAML(t *testing.T) {
	t.Parallel()

	root := createProject(t)
	id := uuid.MustNew()
	relative, err := project.FormPath(id)
	if err != nil {
		t.Fatal(err)
	}
	filePath := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.WriteFile(filePath, []byte("format: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	opened, err := workspace.ReadSource(relative)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.SaveSource(relative, "broken: [\n", opened.Revision); err == nil || !strings.Contains(err.Error(), relative) {
		t.Fatalf("malformed YAML error = %v", err)
	}
}

func TestFileAPIRequiresRevisionAndCSRF(t *testing.T) {
	t.Parallel()

	workspace, relative, _ := createModuleSource(t, "Тест();\n")
	handler := NewHandler(workspace)
	readResponse := httptest.NewRecorder()
	handler.ServeHTTP(readResponse, httptest.NewRequest(http.MethodGet, "/api/file?path="+url.QueryEscape(relative), nil))
	var opened SourceFile
	if readResponse.Code != http.StatusOK || json.Unmarshal(readResponse.Body.Bytes(), &opened) != nil {
		t.Fatalf("read status=%d body=%s", readResponse.Code, readResponse.Body.String())
	}
	notModified := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/file?path="+url.QueryEscape(relative), nil)
	request.Header.Set("If-None-Match", `"`+opened.Revision+`"`)
	handler.ServeHTTP(notModified, request)
	if notModified.Code != http.StatusNotModified {
		t.Fatalf("conditional read status = %d", notModified.Code)
	}
	payload, err := json.Marshal(map[string]string{"path": relative, "content": "Изменено();\n", "expectedRevision": opened.Revision})
	if err != nil {
		t.Fatal(err)
	}
	denied := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPut, "/api/file", bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(denied, request)
	if denied.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF status = %d", denied.Code)
	}
	saved := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPut, "/api/file", bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-ML-CSRF", "1")
	handler.ServeHTTP(saved, request)
	if saved.Code != http.StatusOK || !strings.Contains(saved.Body.String(), "Изменено") {
		t.Fatalf("save status=%d body=%s", saved.Code, saved.Body.String())
	}
}

func createModuleSource(t *testing.T, content string) (*Workspace, string, string) {
	t.Helper()
	root := createProject(t)
	relative, err := project.ModulePath(uuid.MustNew())
	if err != nil {
		t.Fatal(err)
	}
	filePath := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	return workspace, relative, filePath
}
