package studio

import (
	"bytes"
	"errors"
	"io/fs"
	"net/http"
	"path/filepath"

	"github.com/k33alexey/MetaLab/internal/kinds"
	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// «Рабочая область начальной страницы» - the command of the menu of the root
// (docs/block-00/konfiguraciya.md, «Окна», «Досъёмка 27.10.2026»,
// «Предсъёмка 28.10.2026»; snapshots konfiguraciya/18-20,
// series-2.595/41-49, predsyomka-1/33): the forms of the home page in their
// columns, the template of the page and where it shows the command interface
// of the main section. The prototype's window is not modal and has no OK, so
// Studio stands it in the place of the open file: «Сохранить» writes the page
// into its own file beside the root's description (project.HomePageFile).

// homePageAddress is where the window reads the home page and writes it.
const homePageAddress = "/api/project/home-page"

// HomePageWindow is what the window reads: the page - a project without one
// answers an empty page of the template the configurator writes for an
// untouched one, two columns of equal width (mdclasses, HomePageWorkArea.xml;
// question В1 of series 2.646) - at what revision, none for no file; the
// tree of «Выбор управляемой формы»; the full names of the forms of the
// project by their identifiers; the roles of «Настройка видимости».
type HomePageWindow struct {
	Revision string               `json:"revision"`
	Page     metadata.HomePage    `json:"page"`
	Forms    []ObjectChoiceBranch `json:"forms"`
	Names    map[uuid.UUID]string `json:"names"`
	Roles    []SectionRole        `json:"roles"`
}

func (workspace *Workspace) ReadHomePage() (HomePageWindow, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	return workspace.readHomePageLocked()
}

func (workspace *Workspace) readHomePageLocked() (HomePageWindow, error) {
	configuration, err := project.ValidateLayout(workspace.root)
	if err != nil {
		return HomePageWindow{}, err
	}
	loaded, err := metadata.Load(workspace.root)
	if err != nil {
		return HomePageWindow{}, err
	}
	window := HomePageWindow{Page: metadata.HomePage{Template: metadata.TwoColumnsEqualWidthHomePage},
		Roles: projectRoles(loaded, configuration.DefaultLanguage)}
	if loaded.HomePage != nil {
		window.Page = *loaded.HomePage
	}
	if window.Revision, err = workspace.rootDescriptionRevisionLocked(project.HomePageFile); err != nil {
		return HomePageWindow{}, err
	}
	window.Forms, window.Names = homePageFormTree(loaded)
	return window, nil
}

// homePageFormTree is the tree of «Выбор управляемой формы» (snapshot
// series-2.595/42 - the branches stand at the top, no root above them): the
// branches of the project tree that hold forms, in its
// order - «Общие» with the common forms and the kinds of it that keep forms -
// a common form chosen itself, any other object by the forms it keeps, in
// the order of the project tree; an object of no form is left out (question
// В2 of series 2.646). Answers it and the full names of the forms
// (ОбщаяФорма.<имя>, <Вид>.<объект>.Форма.<имя> - the help's ПолноеИмя),
// which the columns of the window show.
func homePageFormTree(loaded *metadata.Catalog) ([]ObjectChoiceBranch, map[uuid.UUID]string) {
	var offered []metadata.Kind
	for _, descriptor := range kinds.Branches() {
		if descriptor.Kind == metadata.CommonFormKind || descriptor.Layout().Forms {
			offered = append(offered, descriptor.Kind)
		}
	}
	names := map[uuid.UUID]string{}
	formIconOf := kinds.Must(metadata.CommonFormKind).Icon
	tree := shapedObjectChoiceTree(loaded, offered, func(kind metadata.Kind, item *ObjectChoiceItem) bool {
		if kind == metadata.CommonFormKind {
			names[item.ID] = item.FullName
			return true
		}
		// Under the object the branch «Формы», as the prototype's window
		// draws it (konfiguraciya.md, «Досъёмка 29.10.2026, вечер»), the
		// forms in the order the model keeps them.
		forms := ObjectChoiceItem{Name: "Формы", Icon: formIcon}
		for _, form := range loaded.ObjectFormNames(kind, item.Name) {
			id, _ := loaded.ObjectFormID(kind, item.Name, form)
			full := item.FullName + ".Форма." + form
			names[id] = full
			forms.Items = append(forms.Items, ObjectChoiceItem{ID: id, Name: form, FullName: full, Icon: formIconOf})
		}
		if len(forms.Items) > 0 {
			item.Items = []ObjectChoiceItem{forms}
		}
		return len(forms.Items) > 0
	})
	return tree, names
}

// SaveHomePage writes the page the window holds, at the revision read
// (saveRootDescription).
func (workspace *Workspace) SaveHomePage(page metadata.HomePage, expectedRevision string) (HomePageWindow, error) {
	return saveRootDescription(workspace, project.HomePageFile, page, expectedRevision, workspace.readHomePageLocked)
}

// saveRootDescription writes value into a description of the root's
// interface (saveRootDescriptionLocked) and reads the window of it again.
func saveRootDescription[W any](workspace *Workspace, file string, value any, expectedRevision string, read func() (W, error)) (W, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	if err := workspace.saveRootDescriptionLocked(file, value, expectedRevision); err != nil {
		var none W
		return none, err
	}
	return read()
}

// registerRootDescriptionWindow serves the window of a description of the
// root's interface at address: GET reads it, PUT writes {value,
// expectedRevision} - the value a T - and answers it read again.
func registerRootDescriptionWindow[T, W any](routes *http.ServeMux, address string, read func() (W, error), save func(T, string) (W, error)) {
	routes.HandleFunc("GET "+address, answerQuery(writeSourceError, func(*http.Request) (any, error) {
		return read()
	}))
	routes.HandleFunc("PUT "+address, answerStudio(0, writeSourceError, func(input struct {
		Value            T      `json:"value"`
		ExpectedRevision string `json:"expectedRevision"`
	}) (any, error) {
		return save(input.Value, input.ExpectedRevision)
	}))
}

// saveRootDescriptionLocked writes one of the descriptions of the root's
// interface (project.RootDescriptionFiles) at the revision read - none for a
// project that had no such file, which it makes - checked as the loader
// reads it (validateYAMLSource). A file made meanwhile is somebody else's.
func (workspace *Workspace) saveRootDescriptionLocked(file string, value any, expectedRevision string) error {
	var encoded bytes.Buffer
	if err := metadata.Encode(&encoded, value); err != nil {
		return err
	}
	if _, err := workspace.readSource(file); !errors.Is(err, ErrSourceNotFound) {
		if err != nil {
			return err
		}
		if expectedRevision == "" {
			return ErrSourceChanged
		}
		_, err := workspace.saveSourceLocked(file, encoded.String(), expectedRevision)
		return err
	}
	if expectedRevision != "" {
		return ErrSourceChanged
	}
	content, err := workspace.validateYAMLSource(file, encoded.Bytes())
	if err != nil {
		return err
	}
	if err := writeNewFile(filepath.Join(workspace.root, file), content); errors.Is(err, fs.ErrExist) {
		return ErrSourceChanged
	} else if err != nil {
		return err
	}
	workspace.invalidateStudioIndexesLocked()
	return nil
}

// rootDescriptionRevisionLocked is the revision of a description of the
// root's interface, "" where the project has none.
func (workspace *Workspace) rootDescriptionRevisionLocked(file string) (string, error) {
	source, err := workspace.readSource(file)
	if errors.Is(err, ErrSourceNotFound) {
		return "", nil
	}
	return source.Revision, err
}

func registerHomePageRoutes(routes *http.ServeMux, workspace *Workspace) {
	registerRootDescriptionWindow(routes, homePageAddress, workspace.ReadHomePage, workspace.SaveHomePage)
}
