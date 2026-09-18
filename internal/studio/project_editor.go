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
// visual project editor reads and writes, in place of raw mlproject.yaml.
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
	file, err := workspace.readSource(project.ManifestFile)
	if err != nil {
		return ProjectEditorSource{}, err
	}
	manifest, err := project.DecodeSource(project.ManifestFile, strings.NewReader(file.Content))
	if err != nil {
		return ProjectEditorSource{}, err
	}
	return ProjectEditorSource{Path: project.ManifestFile, Revision: file.Revision, Manifest: withEnglishLanguage(manifest)}, nil
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
	var encoded bytes.Buffer
	if err := project.Encode(&encoded, manifest); err != nil {
		return ProjectEditorSource{}, err
	}
	saved, err := workspace.saveSourceLocked(project.ManifestFile, encoded.String(), revision)
	if err != nil {
		return ProjectEditorSource{}, err
	}
	opened.Manifest, opened.Revision = manifest, saved.Revision
	return opened, nil
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
