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

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestManagedFormWorkspaceRoundTripAndIdentity(t *testing.T) {
	t.Parallel()
	workspace, relative, form := createManagedFormSource(t)
	opened, err := workspace.ReadManagedForm(relative)
	if err != nil {
		t.Fatal(err)
	}
	if opened.Form.Name != form.Name || len(opened.Languages) != 1 || opened.Languages[0].Code != "ru" {
		t.Fatalf("opened form = %+v", opened)
	}
	opened.Form.Items[0].Children = append(opened.Form.Items[0].Children, metadata.ManagedFormElement{ID: uuid.MustNew(), Name: "Комментарий", Kind: metadata.FormElementField, Title: metadata.LocalizedText{"ru": "Комментарий"}})
	saved, err := workspace.SaveManagedForm(relative, opened.Form, opened.Revision)
	if err != nil || len(saved.Form.Items[0].Children) != 2 || saved.Revision == opened.Revision {
		t.Fatalf("saved form = %+v, error=%v", saved, err)
	}
	saved.Form.ID = uuid.MustNew()
	if _, err := workspace.SaveManagedForm(relative, saved.Form, saved.Revision); err == nil || !strings.Contains(err.Error(), "does not match filename") {
		t.Fatalf("identity error = %v", err)
	}
}

func TestManagedFormAPIIsTypedRevisionSafeAndCSRFProtected(t *testing.T) {
	t.Parallel()
	workspace, relative, _ := createManagedFormSource(t)
	handler := NewHandler(workspace)
	read := httptest.NewRecorder()
	handler.ServeHTTP(read, httptest.NewRequest(http.MethodGet, "/api/form?path="+relative, nil))
	var opened ManagedFormSource
	if read.Code != http.StatusOK || json.Unmarshal(read.Body.Bytes(), &opened) != nil {
		t.Fatalf("read status=%d body=%s", read.Code, read.Body.String())
	}
	opened.Form.Title["ru"] = "Изменённая форма"
	payload, err := json.Marshal(map[string]any{"path": relative, "expectedRevision": opened.Revision, "form": opened.Form})
	if err != nil {
		t.Fatal(err)
	}
	denied := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/form", bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(denied, request)
	if denied.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF status = %d", denied.Code)
	}
	saved := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPut, "/api/form", bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-ML-CSRF", "1")
	handler.ServeHTTP(saved, request)
	if saved.Code != http.StatusOK || !strings.Contains(saved.Body.String(), "Изменённая форма") {
		t.Fatalf("save status=%d body=%s", saved.Code, saved.Body.String())
	}
	conflict := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPut, "/api/form", bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-ML-CSRF", "1")
	handler.ServeHTTP(conflict, request)
	if conflict.Code != http.StatusConflict {
		t.Fatalf("stale revision status=%d body=%s", conflict.Code, conflict.Body.String())
	}
}

func TestManagedFormAPIRejectsNonFormAndUnknownJSON(t *testing.T) {
	t.Parallel()
	workspace, _, form := createManagedFormSource(t)
	handler := NewHandler(workspace)
	invalidPath := httptest.NewRecorder()
	handler.ServeHTTP(invalidPath, httptest.NewRequest(http.MethodGet, "/api/form?path=mlproject.yaml", nil))
	if invalidPath.Code != http.StatusBadRequest {
		t.Fatalf("non-form status = %d", invalidPath.Code)
	}
	payload := []byte(`{"path":"forms/invalid.yaml","expectedRevision":"x","form":{},"unknown":true}`)
	request := httptest.NewRequest(http.MethodPut, "/api/form", bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-ML-CSRF", "1")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("unknown JSON status = %d, form=%+v", response.Code, form)
	}
}

func TestStudioIncludesVisualManagedFormDesigner(t *testing.T) {
	t.Parallel()
	workspace, _, _ := createManagedFormSource(t)
	handler := NewHandler(workspace)
	for _, target := range []string{"/", "/ui/form-designer.js", "/ui/form-designer.css"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, target, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("GET %s status = %d", target, response.Code)
		}
		if target == "/" && (!strings.Contains(response.Body.String(), `id="form-designer"`) || !strings.Contains(response.Body.String(), `/api/form`)) {
			t.Fatalf("Studio shell does not connect the form designer")
		}
	}
}

func createManagedFormSource(t *testing.T) (*Workspace, string, metadata.ManagedForm) {
	t.Helper()
	root := createProject(t)
	form := metadata.ManagedForm{Format: metadata.CurrentFormat, ID: uuid.MustNew(), Name: "ФормаТовара", Title: metadata.LocalizedText{"ru": "Форма товара"}, Kind: metadata.ObjectForm}
	form.Items = []metadata.ManagedFormElement{{ID: uuid.MustNew(), Name: "ОсновнаяГруппа", Kind: metadata.FormElementGroup, Orientation: metadata.FormVertical, Children: []metadata.ManagedFormElement{{ID: uuid.MustNew(), Name: "Наименование", Kind: metadata.FormElementField, Title: metadata.LocalizedText{"ru": "Наименование"}}}}}
	relative, err := project.FormPath(form.ID)
	if err != nil {
		t.Fatal(err)
	}
	var source bytes.Buffer
	if err := metadata.Encode(&source, form); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(relative)), source.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	return workspace, relative, form
}
