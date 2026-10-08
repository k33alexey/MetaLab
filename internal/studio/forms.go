package studio

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/k33alexey/MetaLab/internal/bsl/syntax"
	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// ManagedFormSource is the typed, revision-safe representation consumed by Studio.
type ManagedFormSource struct {
	Path      string               `json:"path"`
	Revision  string               `json:"revision"`
	Form      metadata.ManagedForm `json:"form"`
	Languages []FormLanguage       `json:"languages"`
	DataPaths []FormDataPath       `json:"dataPaths"`
}

type FormLanguage struct {
	Code  string `json:"code"`
	Title string `json:"title"`
}

type FormDataPath struct {
	Path  string `json:"path"`
	Title string `json:"title"`
	Kind  string `json:"kind"`
}

// ManagedFormHandlerResult opens the generated or existing command handler.
type ManagedFormHandlerResult struct {
	Form     ManagedFormSource `json:"form"`
	Module   SourceFile        `json:"module"`
	Location StudioLocation    `json:"location"`
	Created  bool              `json:"created"`
}

func (workspace *Workspace) ReadManagedForm(relative string) (ManagedFormSource, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	return workspace.readManagedForm(relative)
}

func (workspace *Workspace) readManagedForm(relative string) (ManagedFormSource, error) {
	if err := validateFormPath(relative); err != nil {
		return ManagedFormSource{}, err
	}
	file, err := workspace.readSource(relative)
	if err != nil {
		return ManagedFormSource{}, err
	}
	configuration, err := project.ValidateLayout(workspace.root)
	if err != nil {
		return ManagedFormSource{}, err
	}
	form, err := metadata.DecodeManagedForm(relative, strings.NewReader(file.Content), configuration)
	if err != nil {
		return ManagedFormSource{}, err
	}
	if err := formAgreesWithItsPath(form, relative); err != nil {
		return ManagedFormSource{}, err
	}
	return ManagedFormSource{Path: relative, Revision: file.Revision, Form: form,
		Languages: roleLanguages(configuration), DataPaths: workspace.formDataPaths(relative, configuration)}, nil
}

func (workspace *Workspace) SaveManagedForm(relative string, form metadata.ManagedForm, expectedRevision string) (ManagedFormSource, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	return workspace.saveManagedFormLocked(relative, form, expectedRevision)
}

func (workspace *Workspace) saveManagedFormLocked(relative string, form metadata.ManagedForm, expectedRevision string) (ManagedFormSource, error) {
	if err := validateFormPath(relative); err != nil {
		return ManagedFormSource{}, err
	}
	configuration, err := project.ValidateLayout(workspace.root)
	if err != nil {
		return ManagedFormSource{}, err
	}
	if err := metadata.ValidateManagedForm(relative, form, configuration); err != nil {
		return ManagedFormSource{}, err
	}
	if err := formAgreesWithItsPath(form, relative); err != nil {
		return ManagedFormSource{}, err
	}
	var content bytes.Buffer
	if err := metadata.Encode(&content, form); err != nil {
		return ManagedFormSource{}, err
	}
	folders, err := workspace.planFormPictureFolders(relative, form, expectedRevision, configuration)
	if err != nil {
		return ManagedFormSource{}, err
	}
	if err := folders.setAside(); err != nil {
		return ManagedFormSource{}, err
	}
	if _, err := workspace.saveSourceLocked(relative, content.String(), expectedRevision); err != nil {
		folders.putBack()
		return ManagedFormSource{}, err
	}
	if err := folders.finish(); err != nil {
		return ManagedFormSource{}, err
	}
	return workspace.readManagedForm(relative)
}

// planFormPictureFolders plans what the save does to the folders of the
// pictures of the form's elements, against the form on disk. Nothing is
// planned when the save is refused anyway for a changed revision - the
// folders would be put back, and a refused save is better not touching the
// disk at all - or when
// the form on disk does not read: then nobody knows which element a folder
// was.
func (workspace *Workspace) planFormPictureFolders(relative string, form metadata.ManagedForm, expectedRevision string,
	configuration project.Project) (pictureFolders, error) {
	current, err := workspace.readSource(relative)
	if err != nil || !strings.EqualFold(current.Revision, expectedRevision) {
		return pictureFolders{}, nil
	}
	before, err := metadata.DecodeManagedForm(relative, strings.NewReader(current.Content), configuration)
	if err != nil {
		return pictureFolders{}, nil
	}
	target, err := workspace.resolveExistingSource(relative)
	if err != nil {
		return pictureFolders{}, err
	}
	return planPictureFolders(filepath.Dir(target), before, form)
}

// EnsureManagedFormHandler creates a form module and command procedure when
// needed, then returns the exact source location for Studio navigation.
func (workspace *Workspace) EnsureManagedFormHandler(relative, expectedRevision string, commandID uuid.UUID) (ManagedFormHandlerResult, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	opened, err := workspace.readManagedForm(relative)
	if err != nil {
		return ManagedFormHandlerResult{}, err
	}
	if !strings.EqualFold(opened.Revision, expectedRevision) {
		return ManagedFormHandlerResult{}, ErrSourceChanged
	}
	var command *metadata.ManagedFormCommand
	for index := range opened.Form.Commands {
		if opened.Form.Commands[index].ID == commandID {
			command = &opened.Form.Commands[index]
			break
		}
	}
	if command == nil {
		return ManagedFormHandlerResult{}, fmt.Errorf("form command %s was not found", commandID)
	}
	if command.Action != metadata.FormCommandCustom {
		return ManagedFormHandlerResult{}, fmt.Errorf("form command %s does not use a BSL handler", command.Name)
	}

	// Every form keeps its module beside it now, under the name of its role,
	// and declares nothing: the file is the declaration, so whether there is a
	// module at all is answered by looking rather than by asking the form.
	modulePath, err := formModulePath(relative)
	if err != nil {
		return ManagedFormHandlerResult{}, err
	}
	module, err := workspace.readSource(modulePath)
	if err != nil {
		created, createErr := workspace.createFormModuleLocked(modulePath, formHandlerSource(command.Handler))
		if createErr != nil {
			return ManagedFormHandlerResult{}, createErr
		}
		location, locationErr := formHandlerLocation(created, command.Handler)
		if locationErr != nil {
			return ManagedFormHandlerResult{}, locationErr
		}
		return ManagedFormHandlerResult{Form: opened, Module: created, Location: location, Created: true}, nil
	}
	if location, found, findErr := findFormHandler(module, command.Handler); findErr != nil {
		return ManagedFormHandlerResult{}, findErr
	} else if found {
		return ManagedFormHandlerResult{Form: opened, Module: module, Location: location}, nil
	}
	separator := ""
	if strings.TrimSpace(module.Content) != "" {
		separator = "\n\n"
	}
	next := strings.TrimRight(module.Content, "\r\n") + separator + formHandlerSource(command.Handler)
	module, err = workspace.saveSourceLocked(modulePath, next, module.Revision)
	if err != nil {
		return ManagedFormHandlerResult{}, err
	}
	location, err := formHandlerLocation(module, command.Handler)
	if err != nil {
		return ManagedFormHandlerResult{}, err
	}
	return ManagedFormHandlerResult{Form: opened, Module: module, Location: location, Created: true}, nil
}

// formModulePath returns where one form's module lies: МодульФормы.bsl in the
// form's own folder, wherever that folder is. A common form and an object's
// form differ only in where the folder sits.
func formModulePath(relative string) (string, error) {
	if form, ok := objectFormName(relative); ok {
		kind, name, _, _ := project.SplitObjectPath(relative)
		return project.ObjectFormModulePath(kind, name, form)
	}
	parts := strings.Split(relative, "/")
	switch {
	case len(parts) == 4 && parts[0] == "metadata" && parts[1] == "common-forms" && parts[3] == project.FormMetadataFile:
		return project.CommonFormModulePath(parts[2])
	}
	return "", ErrInvalidSourcePath
}

func (workspace *Workspace) createFormModuleLocked(relative, content string) (SourceFile, error) {
	canonical, language, err := validateEditablePath(relative)
	if err != nil || language != "bsl" {
		return SourceFile{}, ErrInvalidSourcePath
	}
	target := filepath.Join(workspace.root, filepath.FromSlash(canonical))
	file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return SourceFile{}, fmt.Errorf("create form module: %w", err)
	}
	failed := true
	defer func() {
		_ = file.Close()
		if failed {
			_ = os.Remove(target)
		}
	}()
	if _, err := file.WriteString(content); err != nil {
		return SourceFile{}, fmt.Errorf("write form module: %w", err)
	}
	if err := file.Sync(); err != nil {
		return SourceFile{}, fmt.Errorf("sync form module: %w", err)
	}
	if err := file.Close(); err != nil {
		return SourceFile{}, fmt.Errorf("close form module: %w", err)
	}
	failed = false
	workspace.invalidateStudioIndexesLocked()
	return sourceFile(canonical, language, []byte(content)), nil
}

func formHandlerSource(name string) string {
	return "&НаКлиенте\nПроцедура " + name + "(Команда)\n\nКонецПроцедуры\n"
}

func formHandlerLocation(module SourceFile, name string) (StudioLocation, error) {
	location, found, err := findFormHandler(module, name)
	if err != nil {
		return StudioLocation{}, err
	}
	if !found {
		return StudioLocation{}, fmt.Errorf("created form handler %s was not found", name)
	}
	return location, nil
}

func findFormHandler(module SourceFile, name string) (StudioLocation, bool, error) {
	parsed, diagnostics := syntax.Parse(module.Path, module.Content)
	if len(diagnostics) != 0 {
		return StudioLocation{}, false, fmt.Errorf("form module has syntax errors: %s", diagnostics[0].Error())
	}
	for _, routine := range parsed.Routines {
		if !strings.EqualFold(routine.Name, name) {
			continue
		}
		if routine.Function {
			return StudioLocation{}, false, fmt.Errorf("form handler %s conflicts with a function", name)
		}
		return StudioLocation{Path: module.Path, Range: editorRange(routine.SourceSpan), Kind: "procedure", Preview: routine.Name}, true, nil
	}
	return StudioLocation{}, false, nil
}

// formAgreesWithItsPath checks a form against where it lies. Both kinds of
// form lie the same way - in a folder named after the form - so the name is
// what has to agree. The identifier inside is free: it is what roles, the
// portal and ML App refer to the form by, not what finds the file.
func formAgreesWithItsPath(form metadata.ManagedForm, relative string) error {
	name, ok := formNameFromPath(relative)
	if !ok {
		return ErrInvalidSourcePath
	}
	if !strings.EqualFold(form.Name, name) {
		return fmt.Errorf("form %s does not match the folder %s it lies in", form.Name, name)
	}
	return nil
}

// formNameFromPath reads the form's name out of where the file lies, for
// either kind of form.
func formNameFromPath(relative string) (string, bool) {
	if form, ok := objectFormName(relative); ok {
		return form, true
	}
	parts := strings.Split(relative, "/")
	switch {
	case len(parts) == 4 && parts[0] == "metadata" && parts[1] == "common-forms" && parts[3] == project.FormMetadataFile:
		return parts[2], true
	}
	return "", false
}

func validateFormPath(relative string) error {
	canonical, language, err := validateEditablePath(relative)
	if err != nil || language != "yaml" {
		return ErrInvalidSourcePath
	}
	if _, ok := formNameFromPath(canonical); !ok {
		return ErrInvalidSourcePath
	}
	return nil
}

// formDataPaths lists the data a form of a catalog or a document shows,
// for the designer to offer: the object's own fields, its attributes and,
// on the form of the object, its table parts with their columns. It reads
// the description of the object the form lies in and nothing else: opening
// a form is held to 300 ms however large the project (ML-STUDIO.md,
// «Отзывчивость»), and reading the whole project took a second for every
// two thousand objects beside it (2.242). Which of the object's forms is
// open is told by its name, which the form shares with its folder
// (formAgreesWithItsPath); a form no slot of the object names, a common
// form, or an object whose description does not read gets none.
func (workspace *Workspace) formDataPaths(relative string, configuration project.Project) []FormDataPath {
	kind, object, rest, ok := project.SplitObjectPath(relative)
	if !ok || len(rest) != 3 || rest[0] != "forms" {
		return nil
	}
	description, err := project.ObjectMetadataPath(kind, object)
	if err != nil {
		return nil
	}
	file, err := workspace.readSource(description)
	if err != nil {
		return nil
	}
	return workspace.formDataPathsFrom(relative, file.Content, configuration)
}

// formDataPathsFrom lists the data a form offers from the description of
// the object it lies in, given as text.
func (workspace *Workspace) formDataPathsFrom(relative, content string, configuration project.Project) []FormDataPath {
	kind, object, rest, ok := project.SplitObjectPath(relative)
	if !ok || len(rest) != 3 || rest[0] != "forms" {
		return nil
	}
	form := rest[1]
	description, err := project.ObjectMetadataPath(kind, object)
	if err != nil {
		return nil
	}
	var result []FormDataPath
	appendField := func(prefix, name string, title metadata.LocalizedText, kind string) {
		text := title.Resolve(configuration.DefaultLanguage, configuration.DefaultLanguage, configuration.Languages)
		if strings.TrimSpace(text) == "" {
			text = name
		}
		result = append(result, FormDataPath{Path: prefix + "." + name, Title: text, Kind: kind})
	}
	appendObject := func(slots metadata.ObjectForms, own []struct{ name, title string }, attributes []metadata.Attribute, parts []metadata.TablePart) {
		formKind, ok := slotOf(slots, form)
		if !ok {
			return
		}
		prefix := formDataPrefix(formKind)
		for _, field := range own {
			appendField(prefix, field.name, metadata.LocalizedText{configuration.DefaultLanguage: field.title}, "field")
		}
		for _, attribute := range attributes {
			appendField(prefix, attribute.Name, attribute.Title, "field")
		}
		if formKind != metadata.ObjectForm {
			return
		}
		for _, part := range parts {
			appendField(prefix, part.Name, part.Title, "table")
			for _, attribute := range part.Attributes {
				appendField(prefix+"."+part.Name, attribute.Name, attribute.Title, "column")
			}
		}
	}
	switch metadata.Kind(kind) {
	case metadata.CatalogKind:
		// The roles a folder brings are not among the three the data paths
		// are built for: a folder form shows a folder, which holds almost none
		// of what an item holds.
		catalog, err := metadata.DecodeCatalog(description, strings.NewReader(content), configuration)
		if err != nil {
			return nil
		}
		appendObject(catalog.Forms.ObjectForms, []struct{ name, title string }{{"Код", "Код"}, {"Наименование", "Наименование"}},
			catalog.Attributes, catalog.TableParts)
	case metadata.DocumentKind:
		document, err := metadata.DecodeDocument(description, strings.NewReader(content), configuration)
		if err != nil {
			return nil
		}
		appendObject(document.Forms, []struct{ name, title string }{{"Номер", "Номер"}, {"Дата", "Дата"}, {"Проведен", "Проведён"}},
			document.Attributes, document.TableParts)
	}
	return result
}

// slotOf says which of an object's three slots names the open form, by the
// name the form shares with its folder.
func slotOf(slots metadata.ObjectForms, form string) (metadata.FormKind, bool) {
	for _, slot := range []struct {
		name string
		kind metadata.FormKind
	}{{slots.Object, metadata.ObjectForm}, {slots.List, metadata.ListForm}, {slots.Choice, metadata.ChoiceForm}} {
		if slot.name != "" && strings.EqualFold(slot.name, form) {
			return slot.kind, true
		}
	}
	return "", false
}

func formDataPrefix(kind metadata.FormKind) string {
	if kind == metadata.ObjectForm {
		return "Объект"
	}
	return "Список"
}
