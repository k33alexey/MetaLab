package studio

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestProjectEditorAlwaysIncludesCanonicalEnglish(t *testing.T) {
	t.Parallel()
	root := createProject(t) // single-language ("ru") fixture project, no "en" at all
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	opened, err := workspace.ReadProjectEditor()
	if err != nil {
		t.Fatal(err)
	}
	english, ok := findLanguage(opened.Manifest.Languages, "en")
	if !ok || english.Name != englishLanguage.Name || english.Title != englishLanguage.Title || english.ID.IsZero() {
		t.Fatalf("English was not injected on read: %+v", opened.Manifest.Languages)
	}

	// Try to corrupt English (rename it) and remove the "ru" language while
	// setting the default language to the (attempted) corrupted English.
	corrupted := opened.Manifest
	corrupted.Languages = []project.Language{{ID: uuid.MustNew(), Name: "NotEnglish", Title: "Not English", Code: "en"}}
	corrupted.DefaultLanguage = "en"
	saved, err := workspace.SaveProjectEditor(corrupted, opened.Revision)
	if err != nil {
		t.Fatal(err)
	}
	english, ok = findLanguage(saved.Manifest.Languages, "en")
	if !ok || english.Name != englishLanguage.Name || english.Title != englishLanguage.Title {
		t.Fatalf("English was not restored to its canonical value: %+v", saved.Manifest.Languages)
	}
	// Личность языка не меняется от того, что кто-то поправил его заголовок:
	// при сохранении без идентификатора он берётся из уже сохранённого проекта.
	if english.ID.IsZero() {
		t.Fatalf("restored English lost its identity: %+v", english)
	}
	if len(saved.Manifest.Languages) != 1 {
		t.Fatalf("unexpected language list: %+v", saved.Manifest.Languages)
	}
}

func TestProjectEditorSaveConflictsAndIdentity(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	opened, err := workspace.ReadProjectEditor()
	if err != nil {
		t.Fatal(err)
	}
	updated := opened.Manifest
	updated.Title = "Новое название"
	saved, err := workspace.SaveProjectEditor(updated, opened.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Revision == opened.Revision || saved.Manifest.Title != "Новое название" {
		t.Fatalf("save did not persist edits: %+v", saved)
	}
	if _, err := workspace.SaveProjectEditor(updated, opened.Revision); !errors.Is(err, ErrSourceChanged) {
		t.Fatalf("stale save: %v", err)
	}
	invalid := saved.Manifest
	invalid.ID = uuid.MustNew()
	if _, err := workspace.SaveProjectEditor(invalid, saved.Revision); err == nil {
		t.Fatal("identity change accepted")
	}
	again, err := workspace.ReadProjectEditor()
	if err != nil || again.Revision != saved.Revision {
		t.Fatalf("invalid save modified file: %v", err)
	}
}

func TestProjectEditorRoutesValidateMutations(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(workspace)
	opened, err := workspace.ReadProjectEditor()
	if err != nil {
		t.Fatal(err)
	}
	manifest := opened.Manifest
	manifest.Title = "Через HTTP"
	encodedManifest, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	body := `{"expectedRevision":"` + opened.Revision + `","manifest":` + string(encodedManifest) + `}`
	for _, test := range []struct {
		body, contentType, csrf string
		status                  int
	}{
		{body, "application/json", "", http.StatusForbidden},
		{body, "text/plain", "1", http.StatusUnsupportedMediaType},
		{body + " {}", "application/json", "1", http.StatusBadRequest},
		{body, "application/json", "1", http.StatusOK},
	} {
		request := httptest.NewRequest(http.MethodPut, "http://localhost/api/project-manifest", strings.NewReader(test.body))
		request.Header.Set("Content-Type", test.contentType)
		request.Header.Set("X-ML-CSRF", test.csrf)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != test.status {
			t.Fatalf("PUT project-manifest: %d %s", response.Code, response.Body.String())
		}
	}
}

func TestProjectEditorUI(t *testing.T) {
	runNodeTest(t, "project-editor.test.mjs")
}

func TestStudioCommonUI(t *testing.T) {
	runNodeTest(t, "studio-common.test.mjs")
}

func runNodeTest(t *testing.T, script string) {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js is required for this test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, node, "--test", "../../scripts/"+script).CombinedOutput()
	if err != nil {
		t.Fatalf("%s: %v\n%s", script, err, output)
	}
}
