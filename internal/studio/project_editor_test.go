package studio

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
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

// defaultsWorkspace writes a project holding one of everything the root's
// defaults can name: a style, a role, a common form, a common template of the
// kind an appearance is made of beside one of another kind, and a settings
// storage.
func defaultsWorkspace(t *testing.T) (*Workspace, map[string]uuid.UUID) {
	t.Helper()
	root := createProject(t)
	written := map[string]uuid.UUID{}
	write := func(key, relative, content string, id uuid.UUID) {
		t.Helper()
		absolute := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(absolute, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		written[key] = id
	}
	base := func(id uuid.UUID, name string) string {
		return "format: 1\nid: " + id.String() + "\nname: " + name + "\ntitle: {ru: " + name + "}\n"
	}
	style, role := uuid.MustNew(), uuid.MustNew()
	form, appearance, spreadsheet, storage := uuid.MustNew(), uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	write("style", "metadata/styles/"+style.String()+".yaml", base(style, "ОсновнойСтиль"), style)
	write("role", "metadata/roles/"+role.String()+".yaml", base(role, "ПолныеПрава"), role)
	write("form", "metadata/common-forms/ФормаОтчета/form.yaml", base(form, "ФормаОтчета")+"kind: common\n", form)
	write("appearance", "metadata/common-templates/ОформлениеОтчетов/object.yaml",
		base(appearance, "ОформлениеОтчетов")+"kind: composition-appearance\n", appearance)
	write("spreadsheet", "metadata/common-templates/ПечатнаяФорма/object.yaml",
		base(spreadsheet, "ПечатнаяФорма")+"kind: spreadsheet\n", spreadsheet)
	write("storage", "metadata/settings-storages/ХранилищеНастроек/object.yaml", base(storage, "ХранилищеНастроек"), storage)
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	return workspace, written
}

// The editor offers what each default may name, and offers only that: an
// identifier typed by hand is an identifier misspelled sooner or later, and an
// appearance is not made out of a spreadsheet.
func TestProjectEditorOffersWhatTheDefaultsMayName(t *testing.T) {
	t.Parallel()
	workspace, written := defaultsWorkspace(t)
	opened, err := workspace.ReadProjectEditor()
	if err != nil {
		t.Fatal(err)
	}
	holds := func(choices []DefaultChoice, id uuid.UUID) bool {
		for _, choice := range choices {
			if choice.ID == id {
				return choice.Name != ""
			}
		}
		return false
	}
	switch {
	case !holds(opened.Defaults.Styles, written["style"]):
		t.Fatalf("the style is not offered: %+v", opened.Defaults.Styles)
	case !holds(opened.Defaults.Roles, written["role"]):
		t.Fatalf("the role is not offered: %+v", opened.Defaults.Roles)
	case !holds(opened.Defaults.CommonForms, written["form"]):
		t.Fatalf("the common form is not offered: %+v", opened.Defaults.CommonForms)
	case !holds(opened.Defaults.SettingsStorages, written["storage"]):
		t.Fatalf("the settings storage is not offered: %+v", opened.Defaults.SettingsStorages)
	case !holds(opened.Defaults.AppearanceTemplates, written["appearance"]):
		t.Fatalf("the appearance template is not offered: %+v", opened.Defaults.AppearanceTemplates)
	case holds(opened.Defaults.AppearanceTemplates, written["spreadsheet"]):
		t.Fatalf("a spreadsheet was offered as an appearance: %+v", opened.Defaults.AppearanceTemplates)
	}
}

// Every default survives being saved through the editor and comes back naming
// what it named, the roles in the order they were granted.
func TestProjectEditorSavesTheDefaultsOfTheRoot(t *testing.T) {
	t.Parallel()
	workspace, written := defaultsWorkspace(t)
	opened, err := workspace.ReadProjectEditor()
	if err != nil {
		t.Fatal(err)
	}
	style, form, appearance, storage := written["style"], written["form"], written["appearance"], written["storage"]
	updated := opened.Configuration
	updated.DefaultStyle = &style
	updated.DefaultInterface = "ОсновнойИнтерфейс"
	updated.DefaultRoles = []uuid.UUID{written["role"]}
	updated.DefaultReportForm = &form
	updated.DefaultSearchForm = &form
	updated.DefaultReportAppearanceTemplate = &appearance
	updated.FormDataSettingsStorage = &storage
	if _, err := workspace.SaveProjectEditor(updated, opened.Revision); err != nil {
		t.Fatal(err)
	}
	reopened, err := workspace.ReadProjectEditor()
	if err != nil {
		t.Fatal(err)
	}
	saved := reopened.Configuration
	switch {
	case saved.DefaultStyle == nil || *saved.DefaultStyle != style:
		t.Fatalf("the default style was lost: %+v", saved.DefaultStyle)
	case saved.DefaultInterface != "ОсновнойИнтерфейс":
		t.Fatalf("the default interface was lost: %q", saved.DefaultInterface)
	case len(saved.DefaultRoles) != 1 || saved.DefaultRoles[0] != written["role"]:
		t.Fatalf("the default roles were lost: %+v", saved.DefaultRoles)
	case saved.DefaultReportForm == nil || *saved.DefaultReportForm != form:
		t.Fatalf("the default report form was lost: %+v", saved.DefaultReportForm)
	case saved.DefaultSearchForm == nil || *saved.DefaultSearchForm != form:
		t.Fatalf("one form standing in two defaults was lost: %+v", saved.DefaultSearchForm)
	case saved.DefaultReportAppearanceTemplate == nil || *saved.DefaultReportAppearanceTemplate != appearance:
		t.Fatalf("the appearance template was lost: %+v", saved.DefaultReportAppearanceTemplate)
	case saved.FormDataSettingsStorage == nil || *saved.FormDataSettingsStorage != storage:
		t.Fatalf("the form data storage was lost: %+v", saved.FormDataSettingsStorage)
	}
}

// A default naming an object the configuration no longer has is kept and shown,
// not quietly dropped: it is the only trace of what was meant, and the root is
// the one place it can be repaired.
func TestProjectEditorKeepsADefaultPointingAtNothing(t *testing.T) {
	t.Parallel()
	workspace, _ := defaultsWorkspace(t)
	opened, err := workspace.ReadProjectEditor()
	if err != nil {
		t.Fatal(err)
	}
	stranger := uuid.MustNew()
	updated := opened.Configuration
	updated.DefaultStyle = &stranger
	if _, err := workspace.SaveProjectEditor(updated, opened.Revision); err != nil {
		t.Fatal(err)
	}
	reopened, err := workspace.ReadProjectEditor()
	if err != nil {
		t.Fatal(err)
	}
	if reopened.Configuration.DefaultStyle == nil || *reopened.Configuration.DefaultStyle != stranger {
		t.Fatalf("the broken default was dropped: %+v", reopened.Configuration.DefaultStyle)
	}
	for _, choice := range reopened.Defaults.Styles {
		if choice.ID == stranger {
			t.Fatal("a style nobody wrote was offered as a choice")
		}
	}
}

// The settings of the root that name no other object are saved through the
// editor, and the dictionaries of the full-text search are offered with the
// kind each of them is: a common template and a constant are both dictionaries
// and are not the same thing.
func TestProjectEditorSavesTheSettingsOfTheRoot(t *testing.T) {
	t.Parallel()
	workspace, written := defaultsWorkspace(t)
	opened, err := workspace.ReadProjectEditor()
	if err != nil {
		t.Fatal(err)
	}
	offered := map[project.DictionaryKind]bool{}
	for _, choice := range opened.Defaults.Dictionaries {
		if choice.ID == written["appearance"] || choice.ID == written["spreadsheet"] {
			offered[choice.Kind] = true
		}
	}
	if !offered[project.TemplateDictionary] {
		t.Fatalf("a common template was not offered as a dictionary: %+v", opened.Defaults.Dictionaries)
	}

	updated := opened.Configuration
	updated.DataLockControl = project.ManagedDataLock
	updated.ObjectAutonumeration = project.KeepAutonumber
	updated.ScriptVariant = project.RussianScript
	updated.NamePrefix = "бсп"
	updated.AdditionalFullTextSearchDictionaries = []project.DictionaryReference{
		{Kind: project.TemplateDictionary, Object: written["spreadsheet"]},
	}
	if _, err := workspace.SaveProjectEditor(updated, opened.Revision); err != nil {
		t.Fatal(err)
	}
	reopened, err := workspace.ReadProjectEditor()
	if err != nil {
		t.Fatal(err)
	}
	saved := reopened.Configuration
	switch {
	case saved.DataLockControl != project.ManagedDataLock:
		t.Fatalf("the data lock mode was lost: %q", saved.DataLockControl)
	case saved.ObjectAutonumeration != project.KeepAutonumber:
		t.Fatalf("the autonumeration mode was lost: %q", saved.ObjectAutonumeration)
	case saved.ScriptVariant != project.RussianScript:
		t.Fatalf("the script variant was lost: %q", saved.ScriptVariant)
	case saved.NamePrefix != "бсп":
		t.Fatalf("the name prefix was lost: %q", saved.NamePrefix)
	case len(saved.AdditionalFullTextSearchDictionaries) != 1 ||
		saved.AdditionalFullTextSearchDictionaries[0].Object != written["spreadsheet"]:
		t.Fatalf("the dictionary was lost: %+v", saved.AdditionalFullTextSearchDictionaries)
	}
}
