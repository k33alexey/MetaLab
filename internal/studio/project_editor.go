package studio

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

const englishLanguageCode = "en"

// englishLanguage is the platform's baseline interchange language: BSL
// itself has an English keyword syntax, so every ML Project keeps it
// available regardless of which languages the project owner configures.
var englishLanguage = project.Language{Name: "English", Title: "English", Code: englishLanguageCode}

// canonicalEnglish keeps the identity a project already gave to English while
// restoring its name and title. Resetting the identity too would make the
// language a different object every time someone edited its title.
func canonicalEnglish(existing project.Language) project.Language {
	canonical := englishLanguage
	canonical.ID = existing.ID
	return canonical
}

// ProjectEditorSource is the typed, revision-safe representation the
// visual project editor reads and writes, in place of raw configuration.yaml.
type ProjectEditorSource struct {
	Path     string          `json:"path"`
	Revision string          `json:"revision"`
	Manifest project.Project `json:"manifest"`
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
	manifest, err := project.DecodeSource(project.ConfigurationFile, strings.NewReader(file.Content))
	if err != nil {
		return ProjectEditorSource{}, err
	}
	return ProjectEditorSource{Path: project.ConfigurationFile, Revision: file.Revision, Manifest: withEnglishLanguage(manifest)}, nil
}

// withEnglishLanguage guarantees English is present and canonical — added
// back if missing, reset to its canonical values if a caller tried to
// change it. This is enforced only here (the visual project editor), not
// in the shared project.Validate rules used everywhere else, so it does
// not retroactively reject any existing ML Project.
func withEnglishLanguage(manifest project.Project) project.Project {
	manifest.Languages = append([]project.Language(nil), manifest.Languages...)
	for index, language := range manifest.Languages {
		if language.Code == englishLanguageCode {
			manifest.Languages[index] = canonicalEnglish(language)
			return manifest
		}
	}
	// English is added with an identity of its own: every language the editor
	// hands out is a complete object, whether the project had it or not.
	added := englishLanguage
	added.ID = uuid.Derive(manifest.ID, "language:"+englishLanguageCode)
	manifest.Languages = append([]project.Language{added}, manifest.Languages...)
	return manifest
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

// SaveProjectEditor validates and writes an edited project manifest.
func (workspace *Workspace) SaveProjectEditor(manifest project.Project, revision string) (ProjectEditorSource, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	opened, err := workspace.readProjectEditorLocked()
	if err != nil {
		return ProjectEditorSource{}, err
	}
	if opened.Revision != revision {
		return ProjectEditorSource{}, ErrSourceChanged
	}
	if manifest.ID != opened.Manifest.ID {
		return ProjectEditorSource{}, fmt.Errorf("project UUID cannot be changed")
	}
	manifest = withEnglishLanguage(manifest)
	// A language the browser did not send an identity for keeps the one the
	// project already has for that code, and only a genuinely new language gets
	// a new identity: editing a title must not replace the object.
	manifest.Languages = append([]project.Language(nil), manifest.Languages...)
	for index := range manifest.Languages {
		if !manifest.Languages[index].ID.IsZero() {
			continue
		}
		if previous, ok := findLanguage(opened.Manifest.Languages, manifest.Languages[index].Code); ok {
			manifest.Languages[index].ID = previous.ID
		}
	}
	manifest, err = project.EnsureLanguageIdentities(manifest)
	if err != nil {
		return ProjectEditorSource{}, err
	}
	manifest = withoutRemovedTranslations(manifest)
	var encoded bytes.Buffer
	if err := project.Encode(&encoded, manifest); err != nil {
		return ProjectEditorSource{}, err
	}
	saved, err := workspace.saveSourceLocked(project.ConfigurationFile, encoded.String(), revision)
	if err != nil {
		return ProjectEditorSource{}, err
	}
	opened.Manifest, opened.Revision = manifest, saved.Revision
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
func withoutRemovedTranslations(manifest project.Project) project.Project {
	kept := make(map[string]bool, len(manifest.Languages))
	for _, language := range manifest.Languages {
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
	manifest.Title = prune(manifest.Title)
	if len(manifest.Title) == 0 {
		manifest.Title = project.LocalizedText{manifest.DefaultLanguage: manifest.Name}
	}
	manifest.BriefInformation = prune(manifest.BriefInformation)
	manifest.DetailedInformation = prune(manifest.DetailedInformation)
	manifest.Copyright = prune(manifest.Copyright)
	manifest.VendorAddress = prune(manifest.VendorAddress)
	manifest.InformationAddress = prune(manifest.InformationAddress)
	manifest.UpdateCatalogAddress = prune(manifest.UpdateCatalogAddress)
	return manifest
}

func registerProjectEditorRoutes(routes *http.ServeMux, workspace *Workspace) {
	routes.HandleFunc("GET /api/project-manifest", func(response http.ResponseWriter, _ *http.Request) {
		value, err := workspace.ReadProjectEditor()
		if err != nil {
			writeSourceError(response, err)
			return
		}
		writeStudioJSON(response, value)
	})
	routes.HandleFunc("PUT /api/project-manifest", func(response http.ResponseWriter, request *http.Request) {
		var input struct {
			ExpectedRevision string          `json:"expectedRevision"`
			Manifest         project.Project `json:"manifest"`
		}
		if !decodeStudioJSONRequest(response, request, &input) {
			return
		}
		value, err := workspace.SaveProjectEditor(input.Manifest, input.ExpectedRevision)
		if err != nil {
			writeSourceError(response, err)
			return
		}
		writeStudioJSON(response, value)
	})
}
