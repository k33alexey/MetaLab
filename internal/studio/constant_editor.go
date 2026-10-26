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
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// A constant has no window: its double click, «Изменить» and «Свойства»
// open its palette (docs/block-00/konstanta.md, «Окно редактирования»), so
// what Studio reads and writes of it is what the palette shows.

// ConstantEditorSource is a constant as its palette reads and writes it.
type ConstantEditorSource struct {
	Path            string             `json:"path"`
	Revision        string             `json:"revision"`
	Constant        metadata.Constant  `json:"constant"`
	TypeOffer       metadata.TypeOffer `json:"typeOffer"`
	Languages       []project.Language `json:"languages"`
	DefaultLanguage string             `json:"defaultLanguage"`
	KindTitle       string             `json:"kindTitle"`
	// Palette is the tabs of the palette of a constant and their rows, the
	// kind's (kinds.Palette).
	Palette []kinds.PaletteTab `json:"palette"`
	// Choices are what the windows «Выбор объекта» of the constant offer:
	// defaultForm - the common forms, one of which opens the value.
	Choices map[string][]ObjectChoiceBranch `json:"choices"`
}

func validateConstantEditorPath(relative string) error {
	return validateObjectEditorPath(relative, metadata.ConstantKind)
}

func (workspace *Workspace) ReadConstantEditor(relative string) (ConstantEditorSource, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	return workspace.readConstantEditorLocked(relative)
}

func (workspace *Workspace) readConstantEditorLocked(relative string) (ConstantEditorSource, error) {
	if err := validateConstantEditorPath(relative); err != nil {
		return ConstantEditorSource{}, err
	}
	file, err := workspace.readSource(relative)
	if err != nil {
		return ConstantEditorSource{}, err
	}
	configuration, err := project.ValidateLayout(workspace.root)
	if err != nil {
		return ConstantEditorSource{}, err
	}
	value, err := metadata.DecodeConstant(relative, strings.NewReader(file.Content), configuration)
	if err != nil {
		return ConstantEditorSource{}, err
	}
	loaded, err := metadata.Load(workspace.root)
	if err != nil {
		return ConstantEditorSource{}, err
	}
	offer, _ := loaded.TypeOffer(metadata.StoredFieldChoice)
	return ConstantEditorSource{
		Path: relative, Revision: file.Revision, Constant: value, TypeOffer: offer,
		Languages: editorLanguages(configuration), DefaultLanguage: configuration.DefaultLanguage,
		KindTitle: kinds.Must(metadata.ConstantKind).Title, Palette: kinds.Must(metadata.ConstantKind).Palette(),
		Choices: map[string][]ObjectChoiceBranch{"defaultForm": objectChoiceTree(loaded, []metadata.Kind{metadata.CommonFormKind})},
	}, nil
}

// SaveConstantEditor validates and writes an edited constant; a new name
// moves its folder with its modules (followNameLocked).
func (workspace *Workspace) SaveConstantEditor(relative string, value metadata.Constant, revision string) (ConstantEditorSource, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	opened, err := workspace.readConstantEditorLocked(relative)
	if err != nil {
		return ConstantEditorSource{}, err
	}
	if opened.Revision != revision {
		return ConstantEditorSource{}, ErrSourceChanged
	}
	if value.ID != opened.Constant.ID {
		return ConstantEditorSource{}, fmt.Errorf("constant UUID cannot be changed")
	}
	if value.Name != opened.Constant.Name {
		if err := workspace.objectNameErrorLocked(relative, "", value.Name); err != nil {
			return ConstantEditorSource{}, err
		}
	}
	configuration, err := project.ValidateLayout(workspace.root)
	if err != nil {
		return ConstantEditorSource{}, err
	}
	encoded, err := encodeConstant(value, configuration)
	if err != nil {
		return ConstantEditorSource{}, err
	}
	written, moveBack, err := workspace.followNameLocked(relative, opened.Constant.Name, value.Name)
	if err != nil {
		return ConstantEditorSource{}, err
	}
	if _, err := workspace.saveSourceLocked(written, encoded, revision); err != nil {
		return ConstantEditorSource{}, errors.Join(err, moveBack())
	}
	return workspace.readConstantEditorLocked(written)
}

// encodeConstant is the description of a constant as its file keeps it,
// refused where the model refuses it.
func encodeConstant(value metadata.Constant, configuration project.Project) (string, error) {
	var encoded bytes.Buffer
	if err := metadata.Encode(&encoded, value); err != nil {
		return "", err
	}
	if _, err := metadata.DecodeConstant("constant", bytes.NewReader(encoded.Bytes()), configuration); err != nil {
		return "", err
	}
	return encoded.String(), nil
}

// CreateConstant makes the next constant, «Константа1» and on, by the one
// way every object is made (CreateObject) and opens its palette.
func (workspace *Workspace) CreateConstant(name string) (ConstantEditorSource, error) {
	return createObjectAs(workspace, metadata.ConstantKind, name, workspace.readConstantEditorLocked)
}

// DeleteConstant removes a constant's folder - its description and modules -
// by the one order of steps of every deletion (deleteObjectLocked).
func (workspace *Workspace) DeleteConstant(relative, expectedRevision string) error {
	return workspace.deleteObjectFolder(metadata.ConstantKind, relative, expectedRevision, func() (uuid.UUID, string, error) {
		opened, err := workspace.readConstantEditorLocked(relative)
		return opened.Constant.ID, opened.Revision, err
	})
}

// constantEditorAddress is where the palette of a constant is read,
// written, made and deleted.
const constantEditorAddress = "/api/constant"

func registerConstantEditorRoutes(routes *http.ServeMux, workspace *Workspace) {
	routes.HandleFunc("GET "+constantEditorAddress, answerQuery(writeSourceError, func(request *http.Request) (any, error) {
		return workspace.ReadConstantEditor(request.URL.Query().Get("path"))
	}))
	registerObjectWindow(routes, constantEditorAddress, workspace.SaveConstantEditor, workspace.CreateConstant, workspace.DeleteConstant)
}
