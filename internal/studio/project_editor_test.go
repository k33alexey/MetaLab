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
	english, ok := findLanguage(opened.Configuration.Languages, "en")
	if !ok || english.Name != englishLanguage.Name || english.Title != englishLanguage.Title || english.ID.IsZero() {
		t.Fatalf("English was not injected on read: %+v", opened.Configuration.Languages)
	}

	// Try to corrupt English (rename it) and remove the "ru" language while
	// setting the default language to the (attempted) corrupted English.
	corrupted := opened.Configuration
	corrupted.Languages = []project.Language{{ID: uuid.MustNew(), Name: "NotEnglish", Title: "Not English", Code: "en"}}
	corrupted.DefaultLanguage = "en"
	saved, err := workspace.SaveProjectEditor(corrupted, opened.Revision)
	if err != nil {
		t.Fatal(err)
	}
	english, ok = findLanguage(saved.Configuration.Languages, "en")
	if !ok || english.Name != englishLanguage.Name || english.Title != englishLanguage.Title {
		t.Fatalf("English was not restored to its canonical value: %+v", saved.Configuration.Languages)
	}
	// Личность языка не меняется от того, что кто-то поправил его заголовок:
	// при сохранении без идентификатора он берётся из уже сохранённого проекта.
	if english.ID.IsZero() {
		t.Fatalf("restored English lost its identity: %+v", english)
	}
	if len(saved.Configuration.Languages) != 1 {
		t.Fatalf("unexpected language list: %+v", saved.Configuration.Languages)
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
	updated := opened.Configuration
	updated.Title = project.LocalizedText{"ru": "Новое название"}
	saved, err := workspace.SaveProjectEditor(updated, opened.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Revision == opened.Revision || saved.Configuration.Title["ru"] != "Новое название" {
		t.Fatalf("save did not persist edits: %+v", saved)
	}
	if _, err := workspace.SaveProjectEditor(updated, opened.Revision); !errors.Is(err, ErrSourceChanged) {
		t.Fatalf("stale save: %v", err)
	}
	invalid := saved.Configuration
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
	configuration := opened.Configuration
	configuration.Title = project.LocalizedText{"ru": "Через HTTP"}
	encodedConfiguration, err := json.Marshal(configuration)
	if err != nil {
		t.Fatal(err)
	}
	body := `{"expectedRevision":"` + opened.Revision + `","configuration":` + string(encodedConfiguration) + `}`
	for _, test := range []struct {
		body, contentType, csrf string
		status                  int
	}{
		{body, "application/json", "", http.StatusForbidden},
		{body, "text/plain", "1", http.StatusUnsupportedMediaType},
		{body + " {}", "application/json", "1", http.StatusBadRequest},
		{body, "application/json", "1", http.StatusOK},
	} {
		request := httptest.NewRequest(http.MethodPut, "http://localhost/api/project-configuration", strings.NewReader(test.body))
		request.Header.Set("Content-Type", test.contentType)
		request.Header.Set("X-ML-CSRF", test.csrf)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != test.status {
			t.Fatalf("PUT project-configuration: %d %s", response.Code, response.Body.String())
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

// Removing a language removes what was written in it: keeping the text would
// leave the project refusing to save over a translation nobody can read. The
// synonym is the one text that cannot vanish, because a configuration without
// one is not valid.
func TestRemovingALanguageTakesItsTextsWithIt(t *testing.T) {
	t.Parallel()
	root := createProject(t) // single-language ("ru") fixture, English injected on read
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	opened, err := workspace.ReadProjectEditor()
	if err != nil {
		t.Fatal(err)
	}
	updated := opened.Configuration
	updated.Title = project.LocalizedText{"ru": "Продажи", "en": "Sales"}
	updated.Copyright = project.LocalizedText{"ru": "© Пример"}
	saved, err := workspace.SaveProjectEditor(updated, opened.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Configuration.Title["ru"] != "Продажи" || saved.Configuration.Copyright["ru"] != "© Пример" {
		t.Fatalf("texts were lost while every language was still there: %+v", saved.Configuration)
	}

	// Now Russian goes, and with it everything written in Russian. English
	// stays, so the synonym keeps its English translation.
	withoutRussian := saved.Configuration
	withoutRussian.Languages = []project.Language{{Name: "English", Title: "English", Code: "en"}}
	withoutRussian.DefaultLanguage = "en"
	saved, err = workspace.SaveProjectEditor(withoutRussian, saved.Revision)
	if err != nil {
		t.Fatal(err)
	}
	switch {
	case len(saved.Configuration.Title) != 1 || saved.Configuration.Title["en"] != "Sales":
		t.Fatalf("the synonym did not lose the language that went: %+v", saved.Configuration.Title)
	case len(saved.Configuration.Copyright) != 0:
		t.Fatalf("a text stayed in a language the project no longer has: %+v", saved.Configuration.Copyright)
	}
}

// A synonym written only in the language that was removed cannot simply
// vanish: the project's own name stands in, in the language it now reads in.
func TestASynonymLeftWithNoLanguageFallsBackToTheName(t *testing.T) {
	t.Parallel()
	workspace, err := Open(createProject(t))
	if err != nil {
		t.Fatal(err)
	}
	opened, err := workspace.ReadProjectEditor()
	if err != nil {
		t.Fatal(err)
	}
	updated := opened.Configuration
	updated.Title = project.LocalizedText{"ru": "Продажи"}
	updated.Languages = []project.Language{{Name: "English", Title: "English", Code: "en"}}
	updated.DefaultLanguage = "en"
	saved, err := workspace.SaveProjectEditor(updated, opened.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Configuration.Title["en"] != saved.Configuration.Name {
		t.Fatalf("the synonym did not fall back to the name: %+v", saved.Configuration.Title)
	}
}
