package studio

import (
	"bytes"
	"fmt"
	"os"
	"path"
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
	manifest, err := project.ValidateLayout(workspace.root)
	if err != nil {
		return ManagedFormSource{}, err
	}
	form, err := metadata.DecodeManagedForm(relative, strings.NewReader(file.Content), manifest)
	if err != nil {
		return ManagedFormSource{}, err
	}
	filenameID, _ := uuid.Parse(strings.TrimSuffix(path.Base(relative), ".yaml"))
	if form.ID != filenameID {
		return ManagedFormSource{}, fmt.Errorf("form UUID %s does not match filename UUID %s", form.ID, filenameID)
	}
	languages := make([]FormLanguage, len(manifest.Languages))
	for index, language := range manifest.Languages {
		title := language.Title
		if strings.TrimSpace(title) == "" {
			title = language.Name
		}
		languages[index] = FormLanguage{Code: language.Code, Title: title}
	}
	return ManagedFormSource{Path: relative, Revision: file.Revision, Form: form, Languages: languages, DataPaths: workspace.formDataPaths(form.ID, manifest)}, nil
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
	manifest, err := project.ValidateLayout(workspace.root)
	if err != nil {
		return ManagedFormSource{}, err
	}
	if err := metadata.ValidateManagedForm(relative, form, manifest); err != nil {
		return ManagedFormSource{}, err
	}
	filenameID, _ := uuid.Parse(strings.TrimSuffix(path.Base(relative), ".yaml"))
	if form.ID != filenameID {
		return ManagedFormSource{}, fmt.Errorf("form UUID %s does not match filename UUID %s", form.ID, filenameID)
	}
	var content bytes.Buffer
	if err := metadata.Encode(&content, form); err != nil {
		return ManagedFormSource{}, err
	}
	if _, err := workspace.saveSourceLocked(relative, content.String(), expectedRevision); err != nil {
		return ManagedFormSource{}, err
	}
	return workspace.readManagedForm(relative)
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

	if opened.Form.Module == nil {
		moduleID, idErr := uuid.New()
		if idErr != nil {
			return ManagedFormHandlerResult{}, idErr
		}
		opened.Form.Module = &moduleID
		modulePath, pathErr := project.ModulePath(moduleID)
		if pathErr != nil {
			return ManagedFormHandlerResult{}, pathErr
		}
		module, createErr := workspace.createFormModuleLocked(modulePath, formHandlerSource(command.Handler))
		if createErr != nil {
			return ManagedFormHandlerResult{}, createErr
		}
		saved, saveErr := workspace.saveManagedFormLocked(relative, opened.Form, opened.Revision)
		if saveErr != nil {
			_ = os.Remove(filepath.Join(workspace.root, filepath.FromSlash(modulePath)))
			return ManagedFormHandlerResult{}, saveErr
		}
		location, locationErr := formHandlerLocation(module, command.Handler)
		if locationErr != nil {
			return ManagedFormHandlerResult{}, locationErr
		}
		return ManagedFormHandlerResult{Form: saved, Module: module, Location: location, Created: true}, nil
	}

	modulePath, err := project.ModulePath(*opened.Form.Module)
	if err != nil {
		return ManagedFormHandlerResult{}, err
	}
	module, err := workspace.readSource(modulePath)
	if err != nil {
		return ManagedFormHandlerResult{}, fmt.Errorf("read form module: %w", err)
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

func (workspace *Workspace) createFormModuleLocked(relative, content string) (SourceFile, error) {
	canonical, language, err := validateEditablePath(relative)
	if err != nil || language != "bsl" {
		return SourceFile{}, ErrInvalidSourcePath
	}
	if len(content) > MaxEditableFileBytes {
		return SourceFile{}, fmt.Errorf("form module exceeds %d bytes", MaxEditableFileBytes)
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

func validateFormPath(relative string) error {
	canonical, language, err := validateEditablePath(relative)
	if err != nil || language != "yaml" || !strings.HasPrefix(canonical, "forms/") {
		return ErrInvalidSourcePath
	}
	return nil
}

func (workspace *Workspace) formDataPaths(formID uuid.UUID, manifest project.Project) []FormDataPath {
	catalog, err := metadata.Load(workspace.root)
	if err != nil {
		return nil
	}
	var result []FormDataPath
	appendField := func(prefix, name, title, kind string) {
		if strings.TrimSpace(title) == "" {
			title = name
		}
		result = append(result, FormDataPath{Path: prefix + "." + name, Title: title, Kind: kind})
	}
	for _, object := range catalog.Catalogs {
		kind, ok := referencedFormKind(object.Forms, formID)
		if !ok {
			continue
		}
		prefix := formDataPrefix(kind)
		appendField(prefix, "Код", "Код", "field")
		appendField(prefix, "Наименование", "Наименование", "field")
		for _, attribute := range object.Attributes {
			appendField(prefix, attribute.Name, attribute.Title.Resolve(manifest.DefaultLanguage, manifest.Languages), "field")
		}
		if kind == metadata.ObjectForm {
			for _, part := range object.TableParts {
				partPath := prefix + "." + part.Name
				appendField(prefix, part.Name, part.Title.Resolve(manifest.DefaultLanguage, manifest.Languages), "table")
				for _, attribute := range part.Attributes {
					appendField(partPath, attribute.Name, attribute.Title.Resolve(manifest.DefaultLanguage, manifest.Languages), "column")
				}
			}
		}
	}
	for _, object := range catalog.Documents {
		kind, ok := referencedFormKind(object.Forms, formID)
		if !ok {
			continue
		}
		prefix := formDataPrefix(kind)
		for _, system := range []struct{ name, title string }{{"Номер", "Номер"}, {"Дата", "Дата"}, {"Проведен", "Проведён"}} {
			appendField(prefix, system.name, system.title, "field")
		}
		for _, attribute := range object.Attributes {
			appendField(prefix, attribute.Name, attribute.Title.Resolve(manifest.DefaultLanguage, manifest.Languages), "field")
		}
		if kind == metadata.ObjectForm {
			for _, part := range object.TableParts {
				partPath := prefix + "." + part.Name
				appendField(prefix, part.Name, part.Title.Resolve(manifest.DefaultLanguage, manifest.Languages), "table")
				for _, attribute := range part.Attributes {
					appendField(partPath, attribute.Name, attribute.Title.Resolve(manifest.DefaultLanguage, manifest.Languages), "column")
				}
			}
		}
	}
	return result
}

func referencedFormKind(forms metadata.ObjectForms, id uuid.UUID) (metadata.FormKind, bool) {
	for _, item := range []struct {
		id   *uuid.UUID
		kind metadata.FormKind
	}{{forms.Object, metadata.ObjectForm}, {forms.List, metadata.ListForm}, {forms.Choice, metadata.ChoiceForm}} {
		if item.id != nil && *item.id == id {
			return item.kind, true
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
