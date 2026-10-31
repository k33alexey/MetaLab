package studio

import (
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"

	"github.com/k33alexey/MetaLab/internal/kinds"
	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// A common template has no window of its own: a double click and «Открыть
// макет» open the editor of its content, «Свойства» its palette
// (docs/block-00/obshchij-maket.md, «Окно»). «Добавить» opens the
// constructor of a template, which names it, says what it holds and may
// load its content from a file; the kind is not changed after.

// TemplateEditorSource is a common template as its palette reads and
// writes it; the palette's row «Тип макета» lists the kinds the constructor
// offers.
type TemplateEditorSource struct {
	objectHead
	Template metadata.CommonTemplateDefinition `json:"template"`
}

// objectHead is what every palette of an object reads beside the object:
// where its description lies and at what revision, the languages of the
// project, what one object of the kind is called, and the kind's palette.
type objectHead struct {
	Path            string             `json:"path"`
	Revision        string             `json:"revision"`
	Languages       []project.Language `json:"languages"`
	DefaultLanguage string             `json:"defaultLanguage"`
	KindTitle       string             `json:"kindTitle"`
	// Project is the name of the project, the root of the tree of the
	// window «Подсистемы» of the field «Подсистемы».
	Project string `json:"project"`
	// MemberWord names the object in the captions of its pages «Подсистемы»
	// and «Функциональные опции» (kinds.Descriptor.MemberWord).
	MemberWord string             `json:"memberWord"`
	Palette    []kinds.PaletteTab `json:"palette"`
	// Pages are the pages of the window of an object of a kind that has one,
	// and WindowRows those of them built of the palette's rows.
	Pages      []kinds.Page                 `json:"pages,omitempty"`
	WindowRows map[string][]kinds.WindowRow `json:"windowRows,omitempty"`
}

// readObjectDescription reads the description of an object of a kind that
// keeps a folder named after it, as the load reads it (decode), with the
// head of its palette.
func readObjectDescription[T any](workspace *Workspace, kind metadata.Kind, relative string,
	decode func(string, io.Reader, project.Project) (T, error)) (T, objectHead, error) {
	var none T
	if err := validateObjectEditorPath(relative, kind); err != nil {
		return none, objectHead{}, err
	}
	file, err := workspace.readSource(relative)
	if err != nil {
		return none, objectHead{}, err
	}
	configuration, err := project.ValidateLayout(workspace.root)
	if err != nil {
		return none, objectHead{}, err
	}
	value, err := decode(relative, strings.NewReader(file.Content), configuration)
	if err != nil {
		return none, objectHead{}, err
	}
	return value, objectHead{Path: relative, Revision: file.Revision, Languages: editorLanguages(configuration), DefaultLanguage: configuration.DefaultLanguage, Project: configuration.Name,
		KindTitle: kinds.Must(kind).Title, MemberWord: kinds.Must(kind).MemberWord(), Palette: kinds.Must(kind).Palette(), Pages: kinds.Must(kind).WindowPages(), WindowRows: kinds.Must(kind).WindowRows()}, nil
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

func (source TemplateEditorSource) paletteOf() (metadata.CommonTemplateDefinition, objectHead) {
	return source.Template, source.objectHead
}
func (TemplateEditorSource) named(value metadata.CommonTemplateDefinition) (uuid.UUID, string) {
	return value.ID, value.Name
}

func (workspace *Workspace) ReadTemplateEditor(relative string) (TemplateEditorSource, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	return workspace.readTemplateEditorLocked(relative)
}

func (workspace *Workspace) readTemplateEditorLocked(relative string) (TemplateEditorSource, error) {
	value, head, err := readObjectDescription(workspace, metadata.CommonTemplateKind, relative, metadata.DecodeCommonTemplate)
	return TemplateEditorSource{objectHead: head, Template: value}, err
}

// SaveTemplateEditor writes the name, the synonym and the comment of a
// template; its kind stays as it was made, and a new name moves its folder
// with its content (followNameLocked).
func (workspace *Workspace) SaveTemplateEditor(relative string, value metadata.CommonTemplateDefinition, revision string) (TemplateEditorSource, error) {
	return savePaletteObject(workspace, relative, revision, value, workspace.readTemplateEditorLocked, func(opened metadata.CommonTemplateDefinition, value *metadata.CommonTemplateDefinition) (func() error, error) {
		if value.Kind != opened.Kind {
			return nil, fmt.Errorf("the kind of a template is not changed once it is made")
		}
		return nil, nil
	})
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

// TemplateContent is the content loaded from a file into a template a
// window added: the template by its identifier and the bytes.
type TemplateContent struct {
	ID      uuid.UUID `json:"id"`
	Content []byte    `json:"content"`
}

// templateContentFiles are the files of the contents loaded into the
// templates an object of a kind named name adds: each must be a template
// it did not have before (were), one of binary data or an external
// component, as the constructor loads a file into those alone.
func templateContentFiles(kind metadata.Kind, name string, were, are []metadata.ObjectTemplate, contents []TemplateContent) ([]newFile, error) {
	var files []newFile
	for _, content := range contents {
		at := slices.IndexFunc(are, func(template metadata.ObjectTemplate) bool { return template.ID == content.ID })
		if at < 0 || slices.ContainsFunc(were, func(template metadata.ObjectTemplate) bool { return template.ID == content.ID }) {
			return nil, fmt.Errorf("content is loaded only into a template the window adds")
		}
		template := are[at]
		if template.Kind != metadata.BinaryTemplate && template.Kind != metadata.AddInTemplate {
			return nil, fmt.Errorf("a file is loaded only into binary data and an external component")
		}
		relative, err := project.ObjectTemplateContentPath(string(kind), name, template.Name, metadata.TemplateContentFile(template.Kind))
		if err != nil {
			return nil, err
		}
		files = append(files, newFile{relative: relative, content: content.Content})
	}
	return files, nil
}

// DeleteTemplate removes a common template's folder - its description and
// content - by the one order of steps of every deletion.
func (workspace *Workspace) DeleteTemplate(relative, expectedRevision string) error {
	return deleteObjectFolder(workspace, metadata.CommonTemplateKind, relative, expectedRevision, workspace.readTemplateEditorLocked)
}

func registerTemplateEditorRoutes(routes *http.ServeMux, workspace *Workspace) {
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
	registerObjectEditor(routes, templateEditorAddress, workspace.ReadTemplateEditor, workspace.SaveTemplateEditor, func(name string) (TemplateEditorSource, error) {
		return createObjectAs(workspace, metadata.CommonTemplateKind, name, nil, workspace.readTemplateEditorLocked)
	}, workspace.DeleteTemplate)
}
