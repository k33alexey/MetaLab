package studio

import (
	"bytes"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/k33alexey/MetaLab/internal/kinds"
	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

const englishLanguageCode = "en"

// englishLanguage is the platform's baseline interchange language: BSL
// itself has an English keyword syntax, so every ML Project keeps it
// available regardless of which languages the project owner configures.
var englishLanguage = project.Language{
	Name: "English", Title: project.LocalizedText{englishLanguageCode: "English"}, Code: englishLanguageCode,
}

// canonicalEnglish keeps the identity a project already gave to English while
// restoring its name and title. Resetting the identity too would make the
// language a different object every time someone edited its title.
func canonicalEnglish(existing project.Language) project.Language {
	canonical := englishLanguage
	canonical.ID = existing.ID
	return canonical
}

// ProjectEditorSource is the root of the project as its palette reads and
// writes it, in place of raw configuration.yaml: the description, the
// palette's tabs and what its windows «Выбор объекта» offer.
type ProjectEditorSource struct {
	Path          string             `json:"path"`
	Revision      string             `json:"revision"`
	Configuration project.Project    `json:"configuration"`
	Palette       []kinds.PaletteTab `json:"palette"`
	// Choices are the objects a row of the palette names, by the key of the
	// row: the tree of the window choosing them and the names the row shows
	// (ObjectChoiceBranch).
	Choices map[string][]ObjectChoiceBranch `json:"choices"`
}

// rootChoices are what the rows of the palette of the root may name, by the
// key of the row: a role, a style, a language, a settings storage, a common
// form, an appearance template, and a dictionary of the full-text search -
// a common template or a constant.
//
// A configuration that does not fully load comes back with no choices
// rather than an error: the root is where a broken reference is repaired, so
// the one palette that can repair it must open even while something else in
// the project is mid-edit. What the root already names is shown regardless -
// a row shows an identifier it cannot find, which is the point of opening it
// in that state. The languages are the root's own and are offered always.
func rootChoices(root string, configuration project.Project) map[string][]ObjectChoiceBranch {
	language := kinds.Must(metadata.LanguageKind)
	languages := ObjectChoiceBranch{Title: language.Titles, One: language.Title, Icon: language.Icon}
	for _, item := range configuration.Languages {
		languages.Objects = append(languages.Objects, ObjectChoiceItem{ID: item.ID, Name: item.Name,
			FullName: language.Name.Russian + "." + item.Name, Icon: language.Icon})
	}
	choices := map[string][]ObjectChoiceBranch{"defaultLanguage": {languages}}
	loaded, err := metadata.Load(root)
	if err != nil {
		return choices
	}
	appearance := func(kind metadata.Kind, item *ObjectChoiceItem) bool {
		// A template of another kind describes nothing an appearance is made
		// of: a spreadsheet is a document, not a set of colours and fonts.
		at := slices.IndexFunc(loaded.CommonTemplates, func(template metadata.CommonTemplateDefinition) bool { return template.ID == item.ID })
		return at >= 0 && loaded.CommonTemplates[at].Kind == metadata.CompositionAppearance
	}
	choices["defaultRoles"] = objectChoiceTree(loaded, []metadata.Kind{metadata.RoleKind})
	choices["defaultStyle"] = objectChoiceTree(loaded, []metadata.Kind{metadata.StyleKind})
	choices["defaultReportAppearanceTemplate"] = shapedObjectChoiceTree(loaded, []metadata.Kind{metadata.CommonTemplateKind}, appearance)
	choices["additionalFullTextSearchDictionaries"] = objectChoiceTree(loaded, []metadata.Kind{metadata.CommonTemplateKind, metadata.ConstantKind})
	storages := objectChoiceTree(loaded, []metadata.Kind{metadata.SettingsStorageKind})
	forms := objectChoiceTree(loaded, []metadata.Kind{metadata.CommonFormKind})
	for _, tab := range kinds.RootPalette(nil) {
		for _, row := range tab.Rows {
			switch {
			case row.Field == kinds.FormField:
				choices[row.Key] = forms
			case row.Field == kinds.ObjectField && choices[row.Key] == nil:
				// Every other row of one object is a storage of settings.
				choices[row.Key] = storages
			}
		}
	}
	return choices
}

// rootPaletteRows is the palette of the root with the rows of its modules
// (rootModules), each opened by its file.
func rootPaletteRows() []kinds.PaletteTab {
	modules := make([]kinds.PaletteRow, 0, len(rootModules))
	for _, module := range rootModules {
		modules = append(modules, kinds.PaletteRow{Key: module.file, Title: module.title, Field: kinds.ModuleField, Important: true,
			Description: module.explanation})
	}
	return kinds.RootPalette(modules)
}

func (workspace *Workspace) ReadProjectEditor() (ProjectEditorSource, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	return workspace.readProjectEditorLocked()
}

func (workspace *Workspace) readProjectEditorLocked() (ProjectEditorSource, error) {
	file, err := workspace.readSource(project.ConfigurationFile)
	if err != nil {
		return ProjectEditorSource{}, err
	}
	configuration, err := project.DecodeSource(project.ConfigurationFile, strings.NewReader(file.Content))
	if err != nil {
		return ProjectEditorSource{}, err
	}
	configuration = withEnglishLanguage(configuration)
	return ProjectEditorSource{
		Path: project.ConfigurationFile, Revision: file.Revision,
		Configuration: configuration, Palette: rootPaletteRows(), Choices: rootChoices(workspace.root, configuration),
	}, nil
}

// withEnglishLanguage guarantees English is present and canonical — added
// back if missing, reset to its canonical values if a caller tried to
// change it. This is enforced only here (the visual project editor), not
// in the shared project.Validate rules used everywhere else, so it does
// not retroactively reject any existing ML Project.
func withEnglishLanguage(configuration project.Project) project.Project {
	configuration.Languages = append([]project.Language(nil), configuration.Languages...)
	for index, language := range configuration.Languages {
		if language.Code == englishLanguageCode {
			configuration.Languages[index] = canonicalEnglish(language)
			return configuration
		}
	}
	// English is added with an identity of its own: every language the editor
	// hands out is a complete object, whether the project had it or not.
	added := englishLanguage
	added.ID = uuid.Derive(configuration.ID, "language:"+englishLanguageCode)
	configuration.Languages = append([]project.Language{added}, configuration.Languages...)
	return configuration
}

// findLanguage returns the configured language with this code.
func findLanguage(languages []project.Language, code string) (project.Language, bool) {
	for _, language := range languages {
		if language.Code == code {
			return language, true
		}
	}
	return project.Language{}, false
}

// SaveProjectEditor validates and writes an edited project configuration.
func (workspace *Workspace) SaveProjectEditor(configuration project.Project, revision string) (ProjectEditorSource, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	opened, err := workspace.readProjectEditorLocked()
	if err != nil {
		return ProjectEditorSource{}, err
	}
	if opened.Revision != revision {
		return ProjectEditorSource{}, ErrSourceChanged
	}
	if configuration.ID != opened.Configuration.ID {
		return ProjectEditorSource{}, fmt.Errorf("project UUID cannot be changed")
	}
	configuration = withEnglishLanguage(configuration)
	// A language the browser did not send an identity for keeps the one the
	// project already has for that code, and only a genuinely new language gets
	// a new identity: editing a title must not replace the object.
	configuration.Languages = append([]project.Language(nil), configuration.Languages...)
	for index := range configuration.Languages {
		if !configuration.Languages[index].ID.IsZero() {
			continue
		}
		if previous, ok := findLanguage(opened.Configuration.Languages, configuration.Languages[index].Code); ok {
			configuration.Languages[index].ID = previous.ID
		}
	}
	configuration, err = project.EnsureLanguageIdentities(configuration)
	if err != nil {
		return ProjectEditorSource{}, err
	}
	configuration = withoutRemovedTranslations(configuration)
	var encoded bytes.Buffer
	if err := project.Encode(&encoded, configuration); err != nil {
		return ProjectEditorSource{}, err
	}
	saved, err := workspace.saveSourceLocked(project.ConfigurationFile, encoded.String(), revision)
	if err != nil {
		return ProjectEditorSource{}, err
	}
	opened.Configuration, opened.Revision = configuration, saved.Revision
	opened.Choices = rootChoices(workspace.root, configuration)
	return opened, nil
}

// withoutRemovedTranslations drops the texts of the configuration root written
// in languages the project no longer has.
//
// Removing a language removes what was written in it - that is what removing a
// language means, and keeping the text would leave the project refusing to
// save over a translation nobody can read any more. The synonym is the one
// text that cannot simply vanish, because a configuration without one is not
// valid: if its last translation went with the language, the project's own
// name stands in, in the language the project now reads in. The name is an
// identifier and says nothing in any particular language, so it is the one
// honest thing to put there.
func withoutRemovedTranslations(configuration project.Project) project.Project {
	kept := make(map[string]bool, len(configuration.Languages))
	for _, language := range configuration.Languages {
		kept[strings.ToLower(language.Code)] = true
	}
	prune := func(text project.LocalizedText) project.LocalizedText {
		if len(text) == 0 {
			return text
		}
		result := make(project.LocalizedText, len(text))
		for code, value := range text {
			if kept[strings.ToLower(code)] {
				result[code] = value
			}
		}
		if len(result) == 0 {
			return nil
		}
		return result
	}
	configuration.Title = prune(configuration.Title)
	if len(configuration.Title) == 0 {
		configuration.Title = project.LocalizedText{configuration.DefaultLanguage: configuration.Name}
	}
	configuration.BriefInformation = prune(configuration.BriefInformation)
	configuration.DetailedInformation = prune(configuration.DetailedInformation)
	configuration.Copyright = prune(configuration.Copyright)
	configuration.VendorAddress = prune(configuration.VendorAddress)
	configuration.InformationAddress = prune(configuration.InformationAddress)
	// A language's own synonym is a text of the configuration like any other,
	// so it loses the translations written in a language that is gone. What it
	// cannot lose is all of them: a language without a synonym is a blank line
	// in the list of languages, and its name stands in - the same answer the
	// root gives when its last translation leaves with a language.
	configuration.Languages = append([]project.Language(nil), configuration.Languages...)
	for index, language := range configuration.Languages {
		title := prune(language.Title)
		if len(title) == 0 {
			title = project.LocalizedText{language.Code: language.Name}
		}
		configuration.Languages[index].Title = title
	}
	return configuration
}

func registerProjectEditorRoutes(routes *http.ServeMux, workspace *Workspace) {
	routes.HandleFunc("GET /api/project-configuration", answerQuery(writeSourceError, func(*http.Request) (any, error) {
		return workspace.ReadProjectEditor()
	}))
	routes.HandleFunc("PUT /api/project-configuration", answerStudio(0, writeSourceError, func(input struct {
		ExpectedRevision string          `json:"expectedRevision"`
		Configuration    project.Project `json:"configuration"`
	}) (any, error) {
		return workspace.SaveProjectEditor(input.Configuration, input.ExpectedRevision)
	}))
}
