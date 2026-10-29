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
		file, err := workspace.readSource(project.HomePageFile)
		if err != nil {
			return HomePageWindow{}, err
		}
		window.Revision = file.Revision
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
		for _, form := range loaded.ObjectFormNames(kind, item.Name) {
			id, _ := loaded.ObjectFormID(kind, item.Name, form)
			full := item.FullName + ".Форма." + form
			names[id] = full
			item.Items = append(item.Items, ObjectChoiceItem{ID: id, Name: form, FullName: full, Icon: formIconOf})
		}
		return len(item.Items) > 0
	})
	return tree, names
}

// SaveHomePage writes the page the window holds, at the revision read - none
// for a project that had no page, whose file it makes - checked as the
// loader reads it (validateYAMLSource).
func (workspace *Workspace) SaveHomePage(page metadata.HomePage, expectedRevision string) (HomePageWindow, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	var encoded bytes.Buffer
	if err := metadata.Encode(&encoded, page); err != nil {
		return HomePageWindow{}, err
	}
	if _, err := workspace.readSource(project.HomePageFile); !errors.Is(err, ErrSourceNotFound) {
		if err != nil {
			return HomePageWindow{}, err
		}
		// A page read where there was none is made: one made meanwhile
		// is somebody else's.
		if expectedRevision == "" {
			return HomePageWindow{}, ErrSourceChanged
		}
		if _, err := workspace.saveSourceLocked(project.HomePageFile, encoded.String(), expectedRevision); err != nil {
			return HomePageWindow{}, err
		}
		return workspace.readHomePageLocked()
	}
	if expectedRevision != "" {
		return HomePageWindow{}, ErrSourceChanged
	}
	content, err := workspace.validateYAMLSource(project.HomePageFile, encoded.Bytes())
	if err != nil {
		return HomePageWindow{}, err
	}
	if err := writeNewFile(filepath.Join(workspace.root, project.HomePageFile), content); errors.Is(err, fs.ErrExist) {
		return HomePageWindow{}, ErrSourceChanged
	} else if err != nil {
		return HomePageWindow{}, err
	}
	workspace.invalidateStudioIndexesLocked()
	return workspace.readHomePageLocked()
}

func registerHomePageRoutes(routes *http.ServeMux, workspace *Workspace) {
	routes.HandleFunc("GET "+homePageAddress, answerQuery(writeSourceError, func(*http.Request) (any, error) {
		return workspace.ReadHomePage()
	}))
	routes.HandleFunc("PUT "+homePageAddress, answerStudio(0, writeSourceError, func(input struct {
		Page             metadata.HomePage `json:"page"`
		ExpectedRevision string            `json:"expectedRevision"`
	}) (any, error) {
		return workspace.SaveHomePage(input.Page, input.ExpectedRevision)
	}))
}
