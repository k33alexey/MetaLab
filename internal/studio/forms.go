package studio

import (
	"bytes"
	"fmt"
	"path"
	"strings"

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
	if _, err := workspace.SaveSource(relative, content.String(), expectedRevision); err != nil {
		return ManagedFormSource{}, err
	}
	return workspace.ReadManagedForm(relative)
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
