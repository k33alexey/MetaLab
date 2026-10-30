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
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/k33alexey/MetaLab/internal/metadata"
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
	if !ok || english.Name != englishLanguage.Name || !reflect.DeepEqual(english.Title, englishLanguage.Title) || english.ID.IsZero() {
		t.Fatalf("English was not injected on read: %+v", opened.Configuration.Languages)
	}

	// Try to corrupt English (rename it) and remove the "ru" language while
	// setting the default language to the (attempted) corrupted English.
	corrupted := opened.Configuration
	corrupted.Languages = []project.Language{{ID: uuid.MustNew(), Name: "NotEnglish", Title: project.LocalizedText{"en": "Not English"}, Code: "en"}}
	corrupted.DefaultLanguage = "en"
	saved, err := workspace.SaveProjectEditor(corrupted, nil, opened.Revision)
	if err != nil {
		t.Fatal(err)
	}
	english, ok = findLanguage(saved.Configuration.Languages, "en")
	if !ok || english.Name != englishLanguage.Name || !reflect.DeepEqual(english.Title, englishLanguage.Title) {
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
	saved, err := workspace.SaveProjectEditor(updated, nil, opened.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Revision == opened.Revision || saved.Configuration.Title["ru"] != "Новое название" {
		t.Fatalf("save did not persist edits: %+v", saved)
	}
	if _, err := workspace.SaveProjectEditor(updated, nil, opened.Revision); !errors.Is(err, ErrSourceChanged) {
		t.Fatalf("stale save: %v", err)
	}
	invalid := saved.Configuration
	invalid.ID = uuid.MustNew()
	if _, err := workspace.SaveProjectEditor(invalid, nil, saved.Revision); err == nil {
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
	saved, err := workspace.SaveProjectEditor(updated, nil, opened.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Configuration.Title["ru"] != "Продажи" || saved.Configuration.Copyright["ru"] != "© Пример" {
		t.Fatalf("texts were lost while every language was still there: %+v", saved.Configuration)
	}

	// Now Russian goes, and with it everything written in Russian. English
	// stays, so the synonym keeps its English translation.
	withoutRussian := saved.Configuration
	withoutRussian.Languages = []project.Language{{Name: "English", Title: project.LocalizedText{"en": "English"}, Code: "en"}}
	withoutRussian.DefaultLanguage = "en"
	saved, err = workspace.SaveProjectEditor(withoutRussian, nil, saved.Revision)
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
	updated.Languages = []project.Language{{Name: "English", Title: project.LocalizedText{"en": "English"}, Code: "en"}}
	updated.DefaultLanguage = "en"
	saved, err := workspace.SaveProjectEditor(updated, nil, opened.Revision)
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
	holds := func(key string, id uuid.UUID) bool {
		item := choiceItem(opened.Choices[key], id)
		return item != nil && item.Name != ""
	}
	switch {
	case !holds("defaultStyle", written["style"]):
		t.Fatalf("the style is not offered: %+v", opened.Choices["defaultStyle"])
	case !holds("defaultRoles", written["role"]):
		t.Fatalf("the role is not offered: %+v", opened.Choices["defaultRoles"])
	case !holds("defaultSearchForm", written["form"]):
		t.Fatalf("the common form is not offered: %+v", opened.Choices["defaultSearchForm"])
	case !holds("formDataSettingsStorage", written["storage"]), !holds("urlExternalDataStorage", written["storage"]):
		t.Fatalf("the settings storage is not offered: %+v", opened.Choices)
	case holds("formDataSettingsStorage", written["style"]):
		t.Fatalf("a style was offered as a storage: %+v", opened.Choices["formDataSettingsStorage"])
	case !holds("defaultReportAppearanceTemplate", written["appearance"]):
		t.Fatalf("the appearance template is not offered: %+v", opened.Choices["defaultReportAppearanceTemplate"])
	case holds("defaultReportAppearanceTemplate", written["spreadsheet"]):
		t.Fatalf("a spreadsheet was offered as an appearance: %+v", opened.Choices["defaultReportAppearanceTemplate"])
	case !holds("defaultLanguage", opened.Configuration.Languages[0].ID):
		t.Fatalf("the languages are not offered: %+v", opened.Choices["defaultLanguage"])
	}
}

// choiceItem finds an object of a tree of «Выбор объекта» by its identifier.
func choiceItem(branches []ObjectChoiceBranch, id uuid.UUID) *ObjectChoiceItem {
	for _, branch := range branches {
		for index := range branch.Objects {
			if branch.Objects[index].ID == id {
				return &branch.Objects[index]
			}
		}
		if item := choiceItem(branch.Children, id); item != nil {
			return item
		}
	}
	return nil
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
	updated.DefaultRoles = []uuid.UUID{written["role"]}
	updated.DefaultReportForm = &form
	updated.DefaultSearchForm = &form
	updated.DefaultReportAppearanceTemplate = &appearance
	updated.FormDataSettingsStorage = &storage
	if _, err := workspace.SaveProjectEditor(updated, nil, opened.Revision); err != nil {
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
	if _, err := workspace.SaveProjectEditor(updated, nil, opened.Revision); err != nil {
		t.Fatal(err)
	}
	reopened, err := workspace.ReadProjectEditor()
	if err != nil {
		t.Fatal(err)
	}
	if reopened.Configuration.DefaultStyle == nil || *reopened.Configuration.DefaultStyle != stranger {
		t.Fatalf("the broken default was dropped: %+v", reopened.Configuration.DefaultStyle)
	}
	if choiceItem(reopened.Choices["defaultStyle"], stranger) != nil {
		t.Fatal("a style nobody wrote was offered as a choice")
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
	if choiceItem(opened.Choices["additionalFullTextSearchDictionaries"], written["spreadsheet"]) == nil {
		t.Fatalf("a common template was not offered as a dictionary: %+v", opened.Choices["additionalFullTextSearchDictionaries"])
	}

	updated := opened.Configuration
	updated.DataLockControl = project.ManagedDataLock
	updated.ScriptVariant = project.RussianScript
	updated.NamePrefix = "бсп"
	updated.AdditionalFullTextSearchDictionaries = []project.DictionaryReference{
		{Kind: project.TemplateDictionary, Object: written["spreadsheet"]},
	}
	if _, err := workspace.SaveProjectEditor(updated, nil, opened.Revision); err != nil {
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
	case saved.ScriptVariant != project.RussianScript:
		t.Fatalf("the script variant was lost: %q", saved.ScriptVariant)
	case saved.NamePrefix != "бсп":
		t.Fatalf("the name prefix was lost: %q", saved.NamePrefix)
	case len(saved.AdditionalFullTextSearchDictionaries) != 1 ||
		saved.AdditionalFullTextSearchDictionaries[0].Object != written["spreadsheet"]:
		t.Fatalf("the dictionary was lost: %+v", saved.AdditionalFullTextSearchDictionaries)
	}
}

// Whether the help of the root stands in the contents is saved through the
// editor and read back unchanged.
func TestProjectEditorSavesTheHelpFlagOfTheRoot(t *testing.T) {
	t.Parallel()
	workspace, _ := defaultsWorkspace(t)
	opened, err := workspace.ReadProjectEditor()
	if err != nil {
		t.Fatal(err)
	}
	updated := opened.Configuration
	updated.IncludeHelpInContents = true
	if _, err := workspace.SaveProjectEditor(updated, nil, opened.Revision); err != nil {
		t.Fatal(err)
	}
	reopened, err := workspace.ReadProjectEditor()
	if err != nil {
		t.Fatal(err)
	}
	if !reopened.Configuration.IncludeHelpInContents {
		t.Fatalf("the flag was lost: %+v", reopened.Configuration)
	}
}

// Removing a language removes what was written in it, including the synonyms of
// the languages that stay. What cannot vanish is a language's synonym as a
// whole: its name stands in, the same answer the root gives for itself.
func TestRemovingALanguageLeavesEveryLanguageNamed(t *testing.T) {
	t.Parallel()
	workspace, err := Open(createProject(t))
	if err != nil {
		t.Fatal(err)
	}
	opened, err := workspace.ReadProjectEditor()
	if err != nil {
		t.Fatal(err)
	}
	// Русский is named only in Ukrainian, which is about to leave.
	updated := opened.Configuration
	updated.Languages = append([]project.Language(nil), updated.Languages...)
	for index := range updated.Languages {
		if updated.Languages[index].Code == "ru" {
			updated.Languages[index].Title = project.LocalizedText{"uk": "Російська"}
		}
	}
	updated.Languages = append(updated.Languages,
		project.Language{Name: "Українська", Title: project.LocalizedText{"uk": "Українська"}, Code: "uk"})
	saved, err := workspace.SaveProjectEditor(updated, nil, opened.Revision)
	if err != nil {
		t.Fatal(err)
	}

	withoutUkrainian := saved.Configuration
	withoutUkrainian.Languages = nil
	for _, language := range saved.Configuration.Languages {
		if language.Code != "uk" {
			withoutUkrainian.Languages = append(withoutUkrainian.Languages, language)
		}
	}
	saved, err = workspace.SaveProjectEditor(withoutUkrainian, nil, saved.Revision)
	if err != nil {
		t.Fatal(err)
	}
	for _, language := range saved.Configuration.Languages {
		if len(language.Title) == 0 {
			t.Fatalf("a language lost its synonym entirely: %+v", language)
		}
		if _, orphaned := language.Title["uk"]; orphaned {
			t.Fatalf("a synonym in a removed language survived: %+v", language)
		}
		if language.Code == "ru" && language.Title["ru"] != language.Name {
			t.Fatalf("the synonym did not fall back to the name: %+v", language)
		}
	}
}

// The picture of the main section is written by «Сохранить» of the root
// with configuration.yaml, as «Выбор картинки» gave it (konfiguraciya.md,
// «Досъёмка 30.10.2026»): an image is its base 100.png in a folder made for
// it; a set writes its variants into picture.yaml, which keeps the
// transparency it said; a picture changed outside Studio is a conflict;
// «Очистить» takes the folder away. A refusal halfway - the folder cannot
// take the image - puts configuration.yaml back as it was read. Defects
// caught: the picture written apart from the description or not at all; the
// variants not described or the transparency lost; a folder of no image
// left; an outside change overwritten; a half-done save leaving the new
// description.
func TestTheRootWritesThePictureOfTheMainSectionWithItsDescription(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	folder := filepath.Join(root, project.MainSectionPictureDirectory)
	opened, err := workspace.ReadProjectEditor()
	if err != nil {
		t.Fatal(err)
	}
	if len(opened.Picture.Images) != 0 || len(opened.Picture.Formats) == 0 {
		t.Fatalf("a project of no picture reads %+v", opened.Picture)
	}
	image := pngImage(t, 16, 16, 120)
	updated := opened.Configuration
	updated.Vendor = "Поставщик"
	saved, err := workspace.SaveProjectEditor(updated, &RootWindowsValue{Picture: &RootPictureValue{Image: &PictureImage{Format: "png", Content: image}}}, opened.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if written, err := os.ReadFile(filepath.Join(folder, "100.png")); err != nil || string(written) != string(image) || saved.Configuration.Vendor != "Поставщик" {
		t.Fatalf("the image and the description were not written together: %v, vendor %q", err, saved.Configuration.Vendor)
	}
	if !reflect.DeepEqual(saved.Picture.Images, []string{"100.png"}) {
		t.Fatalf("the picture reads %+v after its save", saved.Picture)
	}

	// The transparency said outside the window stays; a set describes its
	// variants beside it.
	if err := os.WriteFile(filepath.Join(folder, project.RootPictureDescriptionFile), []byte("load_transparent: true\ntransparent_pixel: {x: 1, y: 2}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.SaveProjectEditor(saved.Configuration, nil, saved.Revision); !errors.Is(err, ErrSourceChanged) {
		t.Fatalf("a picture changed outside Studio was not a conflict: %v", err)
	}
	reread, err := workspace.ReadProjectEditor()
	if err != nil {
		t.Fatal(err)
	}
	set := []PictureSetVariant{{PictureVariant: metadata.PictureVariant{File: "100.png", Density: 100}},
		{PictureVariant: metadata.PictureVariant{Density: 200}, Image: &PictureImage{Format: "png", Content: pngImage(t, 32, 32, 60)}}}
	saved, err = workspace.SaveProjectEditor(reread.Configuration, &RootWindowsValue{Picture: &RootPictureValue{Set: set}}, reread.Revision)
	if err != nil {
		t.Fatal(err)
	}
	described, err := metadata.ReadRootPictureImages(root, project.MainSectionPictureDirectory)
	if err != nil {
		t.Fatal(err)
	}
	want := []metadata.PictureVariant{{File: "100.png", Density: 100}, {File: "200.png", Density: 200}}
	if !described.LoadTransparent || described.TransparentPixel == nil || *described.TransparentPixel != (metadata.PicturePixel{X: 1, Y: 2}) || !reflect.DeepEqual(described.Variants, want) {
		t.Fatalf("picture.yaml says %+v, want the transparency kept and the variants %+v", described, want)
	}

	// A refusal halfway: a folder where the new image goes.
	before, err := os.ReadFile(filepath.Join(root, project.ConfigurationFile))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(folder, "100.jpg"), 0o755); err != nil {
		t.Fatal(err)
	}
	reread, err = workspace.ReadProjectEditor()
	if err != nil {
		t.Fatal(err)
	}
	refused := reread.Configuration
	refused.Vendor = "Другой"
	if _, err := workspace.SaveProjectEditor(refused, &RootWindowsValue{Picture: &RootPictureValue{Image: &PictureImage{Format: "jpg", Content: []byte("jpeg")}}}, reread.Revision); err == nil {
		t.Fatal("an image the folder cannot take was written")
	}
	if after, err := os.ReadFile(filepath.Join(root, project.ConfigurationFile)); err != nil || string(after) != string(before) {
		t.Fatalf("a refused save left configuration.yaml changed: %v", err)
	}
	if err := os.Remove(filepath.Join(folder, "100.jpg")); err != nil {
		t.Fatal(err)
	}

	// «Очистить»: no image, no folder.
	reread, err = workspace.ReadProjectEditor()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.SaveProjectEditor(reread.Configuration, &RootWindowsValue{Picture: &RootPictureValue{Image: &PictureImage{}}}, reread.Revision); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(folder); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a cleared picture left its folder: %v", err)
	}
}

// The command interface of the main section is written by «Сохранить» of
// the root with configuration.yaml, as one save (the owner's decision of
// 29.10.2026, В7 of series 2.646): a file made where there was none, at the
// revision the window read; a file changed outside Studio is a conflict and
// leaves configuration.yaml as it was; a refusal of the picture after it
// puts both back. Defects caught: the main section written apart from the
// description; a conflict or a refusal leaving the description or the main
// section half written.
func TestTheRootWritesTheMainSectionWithItsDescription(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	read := func(name string) string {
		t.Helper()
		content, err := os.ReadFile(filepath.Join(root, name))
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		return string(content)
	}
	said := func(visible bool) metadata.CommandInterface {
		return metadata.CommandInterface{CommandsVisibility: []metadata.InterfaceCommandVisibility{{
			InterfaceCommand: metadata.InterfaceCommand{Written: "0:11111111-1111-4111-8111-111111111111"}, InterfaceVisibility: project.InterfaceVisibility{Common: visible}}}}
	}
	opened, err := workspace.ReadProjectEditor()
	if err != nil {
		t.Fatal(err)
	}
	updated := opened.Configuration
	updated.Vendor = "Поставщик"
	saved, err := workspace.SaveProjectEditor(updated, &RootWindowsValue{MainSection: &MainSectionValue{Value: said(true)}}, opened.Revision)
	if err != nil {
		t.Fatal(err)
	}
	section := read(project.MainSectionCommandInterfaceFile)
	if !strings.Contains(section, "common: true") || saved.Configuration.Vendor != "Поставщик" {
		t.Fatalf("the main section %q and the vendor %q were not written together", section, saved.Configuration.Vendor)
	}
	description := read(project.ConfigurationFile)

	// A conflict: the window read no file, one is there now.
	conflicted := saved.Configuration
	conflicted.Vendor = "Другой"
	if _, err := workspace.SaveProjectEditor(conflicted, &RootWindowsValue{MainSection: &MainSectionValue{Value: said(false)}}, saved.Revision); !errors.Is(err, ErrSourceChanged) {
		t.Fatalf("a main section changed meanwhile: %v", err)
	}
	if read(project.ConfigurationFile) != description || read(project.MainSectionCommandInterfaceFile) != section {
		t.Fatal("a conflict of the main section left the root changed")
	}

	// A refusal of the picture after the main section: its folder takes no
	// file.
	window, err := workspace.ReadMainSection()
	if err != nil {
		t.Fatal(err)
	}
	folder := filepath.Join(root, project.MainSectionPictureDirectory)
	if err := os.MkdirAll(folder, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(folder, 0o755) })
	reread, err := workspace.ReadProjectEditor()
	if err != nil {
		t.Fatal(err)
	}
	refused := reread.Configuration
	refused.Vendor = "Третий"
	if _, err := workspace.SaveProjectEditor(refused, &RootWindowsValue{MainSection: &MainSectionValue{Value: said(false), ExpectedRevision: window.Revision},
		Picture: &RootPictureValue{Image: &PictureImage{Format: "png", Content: pngImage(t, 4, 4, 1)}}}, reread.Revision); err == nil {
		t.Fatal("a picture the folder cannot take was written")
	}
	if read(project.ConfigurationFile) != description || read(project.MainSectionCommandInterfaceFile) != section {
		t.Fatalf("a refused picture left the root changed:\n%s", read(project.MainSectionCommandInterfaceFile))
	}
}

// «Состав» of the root goes through the palette of the root as it is: a save
// of another property keeps it in the description (obshchee.md, «Окно поля
// „Подсистемы“»: the export keeps Content of the root). Defect caught: the
// window writing the root without what it does not show.
func TestTheRootKeepsItsContentThroughItsPalette(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	catalog := uuid.MustNew()
	writeProjectFile(t, root, "metadata/catalogs/Товары/object.yaml", "format: 1\nid: "+catalog.String()+"\nname: Товары\ntitle: {ru: Товары}\n"+
		"code: {type: string, length: 9}\ndescription_length: 25\n")
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	opened, err := workspace.ReadProjectEditor()
	if err != nil {
		t.Fatal(err)
	}
	marked := opened.Configuration
	marked.Content = []uuid.UUID{catalog}
	saved, err := workspace.SaveProjectEditor(marked, nil, opened.Revision)
	if err != nil {
		t.Fatal(err)
	}
	renamed := saved.Configuration
	renamed.Title = project.LocalizedText{"ru": "Новое название"}
	if saved, err = workspace.SaveProjectEditor(renamed, nil, saved.Revision); err != nil {
		t.Fatal(err)
	}
	again, err := workspace.ReadProjectEditor()
	if err != nil || len(again.Configuration.Content) != 1 || again.Configuration.Content[0] != catalog {
		t.Fatalf("the content read back is %v (%v)", again.Configuration.Content, err)
	}
}
