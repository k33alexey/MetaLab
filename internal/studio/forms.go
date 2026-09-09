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
}

type FormLanguage struct {
	Code  string `json:"code"`
	Title string `json:"title"`
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
	return ManagedFormSource{Path: relative, Revision: file.Revision, Form: form, Languages: languages}, nil
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
