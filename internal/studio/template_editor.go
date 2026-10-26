package studio

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/k33alexey/MetaLab/internal/kinds"
	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
)

// A common template has no window of its own: a double click and «Открыть
// макет» open the editor of its content, «Свойства» its palette
// (docs/block-00/obshchij-maket.md, «Окно»). «Добавить» opens the
// constructor of a template, which names it, says what it holds and may
// load its content from a file; the kind is not changed after.

// TemplateEditorSource is a common template as its palette reads and
// writes it.
type TemplateEditorSource struct {
	Path            string                            `json:"path"`
	Revision        string                            `json:"revision"`
	Template        metadata.CommonTemplateDefinition `json:"template"`
	Languages       []project.Language                `json:"languages"`
	DefaultLanguage string                            `json:"defaultLanguage"`
	KindTitle       string                            `json:"kindTitle"`
	// Palette is the tabs of the palette of a template and their rows, the
	// kind's (kinds.Palette); its row «Тип макета» lists the kinds the
	// constructor offers.
	Palette []kinds.PaletteTab `json:"palette"`
}

// TemplateDraft is what the constructor of a template makes: the name, the
// synonym, the comment, the kind and, for binary data and an external
// component, the content loaded from a file.
type TemplateDraft struct {
	Name    string                 `json:"name"`
	Title   metadata.LocalizedText `json:"title,omitempty"`
	Comment string                 `json:"comment,omitempty"`
	Kind    metadata.TemplateKind  `json:"kind"`
	Content []byte                 `json:"content,omitempty"`
}

// templateEditorAddress is where the palette of a template is read and
// written, and templateConstructorAddress where the constructor makes one.
const (
	templateEditorAddress      = "/api/template"
	templateConstructorAddress = "/api/template/new"
)

func (workspace *Workspace) ReadTemplateEditor(relative string) (TemplateEditorSource, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	return workspace.readTemplateEditorLocked(relative)
}

func (workspace *Workspace) readTemplateEditorLocked(relative string) (TemplateEditorSource, error) {
	if err := validateObjectEditorPath(relative, metadata.CommonTemplateKind); err != nil {
		return TemplateEditorSource{}, err
	}
	file, err := workspace.readSource(relative)
	if err != nil {
		return TemplateEditorSource{}, err
	}
	configuration, err := project.ValidateLayout(workspace.root)
	if err != nil {
		return TemplateEditorSource{}, err
	}
	value, err := metadata.DecodeCommonTemplate(relative, strings.NewReader(file.Content), configuration)
	if err != nil {
		return TemplateEditorSource{}, err
	}
	return TemplateEditorSource{
		Path: relative, Revision: file.Revision, Template: value,
		Languages: editorLanguages(configuration), DefaultLanguage: configuration.DefaultLanguage,
		KindTitle: kinds.Must(metadata.CommonTemplateKind).Title, Palette: kinds.Must(metadata.CommonTemplateKind).Palette(),
	}, nil
}

// SaveTemplateEditor writes the name, the synonym and the comment of a
// template; its kind stays as it was made, and a new name moves its folder
// with its content (followNameLocked).
func (workspace *Workspace) SaveTemplateEditor(relative string, value metadata.CommonTemplateDefinition, revision string) (TemplateEditorSource, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	opened, err := workspace.readTemplateEditorLocked(relative)
	if err != nil {
		return TemplateEditorSource{}, err
	}
	if opened.Revision != revision {
		return TemplateEditorSource{}, ErrSourceChanged
	}
	if value.ID != opened.Template.ID {
		return TemplateEditorSource{}, fmt.Errorf("template UUID cannot be changed")
	}
	if value.Kind != opened.Template.Kind {
		return TemplateEditorSource{}, fmt.Errorf("the kind of a template is not changed once it is made")
	}
	if value.Name != opened.Template.Name {
		if err := workspace.objectNameErrorLocked(relative, "", value.Name); err != nil {
			return TemplateEditorSource{}, err
		}
	}
	var encoded bytes.Buffer
	if err := metadata.Encode(&encoded, value); err != nil {
		return TemplateEditorSource{}, err
	}
	written, moveBack, err := workspace.followNameLocked(relative, opened.Template.Name, value.Name)
	if err != nil {
		return TemplateEditorSource{}, err
	}
	if _, err := workspace.saveSourceLocked(written, encoded.String(), revision); err != nil {
		return TemplateEditorSource{}, errors.Join(err, moveBack())
	}
	return workspace.readTemplateEditorLocked(written)
}

// CreateTemplate makes the template the constructor describes by the one
// way every object is made (CreateObject). The constructor offers no Active
// document - it is greyed in the prototype's on macOS, and Studio edits
// none anywhere - and loads a file only for binary data and an external
// component (obshchij-maket.md, «Команды»).
func (workspace *Workspace) CreateTemplate(draft TemplateDraft) (TemplateEditorSource, error) {
	if draft.Kind == metadata.ActiveDocument {
		return TemplateEditorSource{}, fmt.Errorf("an Active document is not made by the constructor")
	}
	if len(draft.Content) > 0 && draft.Kind != metadata.BinaryTemplate && draft.Kind != metadata.AddInTemplate {
		return TemplateEditorSource{}, fmt.Errorf("a file is loaded only into binary data and an external component")
	}
	return createObjectAs(workspace, metadata.CommonTemplateKind, draft.Name, func(fresh *metadata.Fresh) error {
		template, _ := fresh.Value.(metadata.CommonTemplateDefinition)
		template.Title, template.Comment, template.Kind = draft.Title, draft.Comment, draft.Kind
		fresh.Value = template
		if len(draft.Content) > 0 {
			content, err := project.CommonTemplateContentPath(template.Name, metadata.TemplateContentFile(draft.Kind))
			if err != nil {
				return err
			}
			fresh.Files = append(fresh.Files, metadata.FreshFile{Path: content, Content: draft.Content})
		}
		return nil
	}, workspace.readTemplateEditorLocked)
}

// DeleteTemplate removes a common template's folder - its description and
// content - by the one order of steps of every deletion.
func (workspace *Workspace) DeleteTemplate(relative, expectedRevision string) error {
	return deleteObjectFolder(workspace, metadata.CommonTemplateKind, relative, expectedRevision, workspace.readTemplateEditorLocked)
}

func registerTemplateEditorRoutes(routes *http.ServeMux, workspace *Workspace) {
	routes.HandleFunc("GET "+templateEditorAddress, answerQuery(writeSourceError, func(request *http.Request) (any, error) {
		return workspace.ReadTemplateEditor(request.URL.Query().Get("path"))
	}))
	// What the constructor offers before any template is there to read it
	// off: the name of a new template and the kinds.
	routes.HandleFunc("GET "+templateConstructorAddress, answerQuery(writeSourceError, func(*http.Request) (any, error) {
		name, _ := kinds.Must(metadata.CommonTemplateKind).NewName()
		return struct {
			Name  string      `json:"name"`
			Kinds [][2]string `json:"kinds"`
		}{name, kinds.TemplateKinds()}, nil
	}))
	routes.HandleFunc("POST "+templateConstructorAddress, answerStudio(0, writeSourceError, func(draft TemplateDraft) (any, error) {
		return workspace.CreateTemplate(draft)
	}))
	// «Добавить» of the tree opens the constructor, which writes at its
	// own address; a template named alone is made spreadsheet.
	registerObjectWindow(routes, templateEditorAddress, workspace.SaveTemplateEditor, func(name string) (TemplateEditorSource, error) {
		return createObjectAs(workspace, metadata.CommonTemplateKind, name, nil, workspace.readTemplateEditorLocked)
	}, workspace.DeleteTemplate)
}
