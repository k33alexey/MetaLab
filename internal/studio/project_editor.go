package studio

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/k33alexey/MetaLab/internal/atomicfile"
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
	// Picture is the picture of the main section as its window «Выбор
	// картинки» shows it.
	Picture RootPictureSource `json:"picture"`
	// description is the revision of configuration.yaml alone, which its
	// save checks; Revision holds the picture too.
	description string
}

// RootPictureSource is the picture of the main section, the root's own
// folder (project.MainSectionPictureDirectory), as its window shows it, the
// window of a common picture's picture (konfiguraciya.md, «Досъёмка
// 30.10.2026»; snapshots dosyomka-2651/07, 08): its images by name, its set
// of variants as the editor of a set shows it (pictureSet), and what that
// editor and «Выбрать из файла» offer.
type RootPictureSource struct {
	Images         []string                  `json:"images"`
	Set            []metadata.PictureVariant `json:"set,omitempty"`
	VariantChoices PictureVariantChoices     `json:"variantChoices"`
	Formats        []string                  `json:"formats"`
}

// RootPictureValue is what OK of «Выбор картинки» gave the picture of the
// main section, written by «Сохранить» of the root with configuration.yaml:
// a new image or a new set, as the window of a common picture gives them.
type RootPictureValue struct {
	Image *PictureImage       `json:"image,omitempty"`
	Set   []PictureSetVariant `json:"set,omitempty"`
}

// mainSectionPictureFile is the description of the picture of the main
// section beside its images: a picture of the root is read and written as
// a common picture is, by the path of its description.
var mainSectionPictureFile = path.Join(project.MainSectionPictureDirectory, project.RootPictureDescriptionFile)

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
	picture, digest, err := workspace.rootPictureLocked()
	if err != nil {
		return ProjectEditorSource{}, err
	}
	revision := sha256.New()
	revision.Write([]byte(file.Revision))
	revision.Write(digest)
	return ProjectEditorSource{
		Path: project.ConfigurationFile, Revision: hex.EncodeToString(revision.Sum(nil)), description: file.Revision,
		Configuration: configuration, Palette: rootPaletteRows(), Choices: rootChoices(workspace.root, configuration), Picture: picture,
	}, nil
}

// rootPictureLocked reads the picture of the main section and a digest of
// its images and its description: a picture changed outside Studio changes
// the revision of the root. A description that does not read leaves the
// images drawn by their names - the root is where a broken project is
// repaired, so its palette opens.
func (workspace *Workspace) rootPictureLocked() (RootPictureSource, []byte, error) {
	images, digest, err := workspace.pictureImagesLocked(mainSectionPictureFile)
	if err != nil {
		return RootPictureSource{}, nil, err
	}
	description, err := os.ReadFile(filepath.Join(workspace.root, filepath.FromSlash(mainSectionPictureFile)))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return RootPictureSource{}, nil, err
	}
	described, _ := metadata.ReadRootPictureImages(workspace.root, project.MainSectionPictureDirectory)
	return RootPictureSource{Images: images, Set: pictureSet(described.Variants, images), VariantChoices: pictureVariantChoices, Formats: metadata.PictureFormats()},
		append(digest, description...), nil
}

// ReadRootPictureImage answers an image of the picture of the main section,
// by its name in the folder, and its media type.
func (workspace *Workspace) ReadRootPictureImage(name string) ([]byte, string, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	return workspace.readPictureImageLocked(mainSectionPictureFile, name)
}

// rootPictureFilesLocked is what writing the picture the window gave makes
// of the folder of the picture of the main section: the files taken away
// and written - the images and the description, which keeps the
// transparency and says the variants, and is not written when it says
// neither (project.RootPictureDescriptionFile).
func (workspace *Workspace) rootPictureFilesLocked(opened RootPictureSource, picture RootPictureValue) ([]string, []pictureFile, error) {
	described, _ := metadata.ReadRootPictureImages(workspace.root, project.MainSectionPictureDirectory)
	variants, remove, write, err := workspace.pictureChangeLocked(mainSectionPictureFile, opened.Images, described.Variants, picture.Image, picture.Set)
	if err != nil {
		return nil, nil, err
	}
	described.Variants = variants
	if _, err := os.Stat(filepath.Join(workspace.root, filepath.FromSlash(mainSectionPictureFile))); err == nil {
		remove = append(remove, project.RootPictureDescriptionFile)
	}
	// A picture left with no image is no folder: its description goes too.
	kept := slices.DeleteFunc(slices.Clone(opened.Images), func(image string) bool { return slices.Contains(remove, image) })
	if len(kept)+len(write) > 0 && (described.LoadTransparent || described.TransparentPixel != nil || len(described.Variants) > 0) {
		var encoded bytes.Buffer
		if err := metadata.Encode(&encoded, described); err != nil {
			return nil, nil, err
		}
		write = append(write, pictureFile{project.RootPictureDescriptionFile, encoded.Bytes()})
	}
	return remove, write, nil
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

// SaveProjectEditor validates and writes an edited project configuration
// and, when «Выбор картинки» gave one, the picture of the main section, as
// one save: the description first, then the folder of the picture; a
// refusal there puts the description back as it was read and leaves the
// folder as it was (replacePictureImagesLocked).
func (workspace *Workspace) SaveProjectEditor(configuration project.Project, picture *RootPictureValue, revision string) (ProjectEditorSource, error) {
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
	var remove []string
	var write []pictureFile
	if picture != nil {
		if remove, write, err = workspace.rootPictureFilesLocked(opened.Picture, *picture); err != nil {
			return ProjectEditorSource{}, err
		}
	}
	original, err := workspace.readSource(project.ConfigurationFile)
	if err != nil {
		return ProjectEditorSource{}, err
	}
	if _, err := workspace.saveSourceLocked(project.ConfigurationFile, encoded.String(), opened.description); err != nil {
		return ProjectEditorSource{}, err
	}
	if picture != nil {
		if err := workspace.writeRootPictureLocked(remove, write); err != nil {
			restored := atomicfile.Write(filepath.Join(workspace.root, project.ConfigurationFile), ".ml-write-*", 0o644, atomicfile.Bytes([]byte(original.Content)))
			workspace.invalidateStudioIndexesLocked()
			return ProjectEditorSource{}, errors.Join(err, restored)
		}
	}
	return workspace.readProjectEditorLocked()
}

// writeRootPictureLocked replaces the files of the folder of the picture of
// the main section: the folder is made for the first image and taken away
// when the last one goes.
func (workspace *Workspace) writeRootPictureLocked(remove []string, write []pictureFile) error {
	directory := filepath.Join(workspace.root, project.MainSectionPictureDirectory)
	_, missing := os.Stat(directory)
	if len(write) > 0 {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			return err
		}
	}
	if err := workspace.replacePictureImagesLocked(mainSectionPictureFile, remove, write); err != nil {
		if missing != nil {
			_ = os.Remove(directory)
		}
		return err
	}
	if len(write) == 0 {
		// Empty now, or kept for files that are no images of it.
		_ = os.Remove(directory)
	}
	return nil
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
		ExpectedRevision string            `json:"expectedRevision"`
		Configuration    project.Project   `json:"configuration"`
		Picture          *RootPictureValue `json:"picture"`
	}) (any, error) {
		return workspace.SaveProjectEditor(input.Configuration, input.Picture, input.ExpectedRevision)
	}))
	routes.HandleFunc("GET /api/project/picture/image", answerPictureImage(func(request *http.Request) ([]byte, string, error) {
		return workspace.ReadRootPictureImage(request.URL.Query().Get("name"))
	}))
}
