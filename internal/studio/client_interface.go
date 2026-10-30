package studio

import (
	"fmt"
	"net/http"
	"sort"

	"github.com/k33alexey/MetaLab/internal/behavior"
	"github.com/k33alexey/MetaLab/internal/kinds"
	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// «Интерфейс клиентского приложения» - the command of the menu of the root
// (docs/block-00/konfiguraciya.md, «Окна», «Досъёмка 27.10.2026»,
// «Предсъёмка 28.10.2026», «Досъёмка 29.10.2026, вечер»): the sides of the
// main window with their groups and panels, the panels of the platform at
// the right. The prototype's window is not modal and has no OK, so Studio
// stands it in the place of the open file, as the work area of the home
// page: «Сохранить» writes project.ClientInterfaceFile. What a command does
// to the interface the platform's behavior says (package behavior); the
// window sends it the interface it holds and shows what comes back.

// clientInterfaceAddress is where the window reads the interface and writes
// it; clientInterfaceEditAddress is where a command of the window is done.
const (
	clientInterfaceAddress     = "/api/project/client-interface"
	clientInterfaceEditAddress = "/api/project/client-interface/edit"
)

// ClientInterfaceWindow is what the window reads: the interface - empty
// where the project has no file of it - at what revision, none for no file;
// the panels of the platform in the order of the list at the right; which
// of them the sides show; the palettes of a panel standing on a side
// («Свойства: Стандартная панель») and of a panel of the list («Описание
// панели»).
type ClientInterfaceWindow struct {
	Revision string                   `json:"revision"`
	Value    metadata.ClientInterface `json:"value"`
	Panels   []metadata.StandardPanel `json:"panels"`
	Placed   []uuid.UUID              `json:"placed"`
	Palettes ClientInterfacePalettes  `json:"palettes"`
}

// ClientInterfacePalettes are the palettes of the window: the sides and the
// groups have none (series-2.595/57).
type ClientInterfacePalettes struct {
	Panel      []kinds.PaletteTab `json:"panel"`
	Definition []kinds.PaletteTab `json:"definition"`
}

// clientInterfacePalettes: a panel on a side shows «Стандартная панель»
// with «…» (predsyomka-1/42, dosyomka-2646/49); a panel of the list its name,
// unavailable, and - the panel of sections only - «Отображение», the five
// values of the help in the order of the palette's list
// (series-2.595/57, dosyomka-2646/56).
var clientInterfacePalettes = ClientInterfacePalettes{
	Panel: []kinds.PaletteTab{{Key: "main", Title: "Основные", Rows: []kinds.PaletteRow{
		{Key: "panel", Title: "Стандартная панель", Field: "standard-panel"},
	}}},
	Definition: []kinds.PaletteTab{{Key: "main", Title: "Основные", Rows: []kinds.PaletteRow{
		{Key: "name", Title: "Имя", Field: "text", Fixed: true},
		{Key: "representation", Title: "Отображение", Field: "choice", Default: string(metadata.SectionsPanelPictureAndText),
			Shown: []string{"panel=" + metadata.SectionsPanel.String()}, Choices: [][2]string{
				{string(metadata.SectionsPanelPicture), "Картинка"},
				{string(metadata.SectionsPanelPictureAndText), "Картинка и текст"},
				{string(metadata.SectionsPanelText), "Текст"},
				{string(metadata.SectionsPanelPictureOnLeftAndText), "Текст справа от картинки"},
				{string(metadata.SectionsPanelPictureOnTopAndText), "Текст под картинкой"},
			}},
	}}},
}

func (workspace *Workspace) ReadClientInterface() (ClientInterfaceWindow, error) {
	workspace.mu.Lock()
	defer workspace.mu.Unlock()
	return workspace.readClientInterfaceLocked()
}

func (workspace *Workspace) readClientInterfaceLocked() (ClientInterfaceWindow, error) {
	if _, err := project.ValidateLayout(workspace.root); err != nil {
		return ClientInterfaceWindow{}, err
	}
	loaded, err := metadata.Load(workspace.root)
	if err != nil {
		return ClientInterfaceWindow{}, err
	}
	// A project whose root writes no interface shows the platform's default
	// one, written only when the window saves an edit of it.
	window := ClientInterfaceWindow{Panels: metadata.StandardPanels, Palettes: clientInterfacePalettes, Value: metadata.DefaultClientInterface()}
	if loaded.ClientInterface != nil {
		window.Value = *loaded.ClientInterface
	}
	window.Placed = placedPanels(window.Value)
	if window.Revision, err = workspace.rootDescriptionRevisionLocked(project.ClientInterfaceFile); err != nil {
		return ClientInterfaceWindow{}, err
	}
	return window, nil
}

// SaveClientInterface writes the interface the window holds, at the revision
// read (saveRootDescription), defining every panel of the platform as the
// prototype's file does (DefineStandardPanels).
func (workspace *Workspace) SaveClientInterface(value metadata.ClientInterface, expectedRevision string) (ClientInterfaceWindow, error) {
	value.DefineStandardPanels()
	return saveRootDescription(workspace, project.ClientInterfaceFile, value, expectedRevision, workspace.readClientInterfaceLocked)
}

// ClientInterfaceEdit is a command of the window on the interface it holds:
// add-panel and add-group at At, remove and move (By -1 or 1) the element at
// At, choose the Panel at At, drop-panel - the Panel dragged from the list
// onto the group at At, representation of the panel of sections.
type ClientInterfaceEdit struct {
	Value               metadata.ClientInterface             `json:"value"`
	Operation           string                               `json:"operation"`
	At                  behavior.ClientInterfacePlace        `json:"at"`
	By                  int                                  `json:"by,omitempty"`
	Panel               uuid.UUID                            `json:"panel"`
	PanelRepresentation metadata.SectionsPanelRepresentation `json:"representation,omitempty"`
}

// ClientInterfaceEdited is the interface after a command, where the element
// it acted on stands then - the row the window makes current - and the
// panels the sides show.
type ClientInterfaceEdited struct {
	Value  metadata.ClientInterface      `json:"value"`
	At     behavior.ClientInterfacePlace `json:"at"`
	Placed []uuid.UUID                   `json:"placed"`
}

// EditClientInterface does a command of the window by the platform's
// behavior.
func EditClientInterface(edit ClientInterfaceEdit) (ClientInterfaceEdited, error) {
	value, at := edit.Value, edit.At
	var err error
	switch edit.Operation {
	case "add-panel":
		at, err = behavior.AddClientInterfacePanel(&value, at)
	case "add-group":
		at, err = behavior.AddClientInterfaceGroup(&value, at)
	case "remove":
		err = behavior.RemoveClientInterfaceElement(&value, at)
	case "move":
		at, _, err = behavior.MoveClientInterfaceElement(&value, at, edit.By)
	case "choose":
		err = behavior.ChooseClientInterfacePanel(&value, at, edit.Panel)
	case "drop-panel":
		at, err = behavior.DropClientInterfacePanel(&value, at, edit.Panel)
	case "representation":
		behavior.SetSectionsPanelRepresentation(&value, edit.PanelRepresentation)
	default:
		err = fmt.Errorf("the window of the client interface has no command %q", edit.Operation)
	}
	if err != nil {
		return ClientInterfaceEdited{}, err
	}
	return ClientInterfaceEdited{Value: value, At: at, Placed: placedPanels(value)}, nil
}

// placedPanels is behavior.PlacedClientInterfacePanels in the order of the
// panels of the platform, then the others the sides show by their
// identifiers.
func placedPanels(value metadata.ClientInterface) []uuid.UUID {
	placed := behavior.PlacedClientInterfacePanels(value)
	answer := []uuid.UUID{}
	for _, panel := range metadata.StandardPanels {
		if placed[panel.ID] {
			answer = append(answer, panel.ID)
			delete(placed, panel.ID)
		}
	}
	others := make([]uuid.UUID, 0, len(placed))
	for panel := range placed {
		others = append(others, panel)
	}
	sort.Slice(others, func(i, j int) bool { return others[i].String() < others[j].String() })
	return append(answer, others...)
}

func registerClientInterfaceRoutes(routes *http.ServeMux, workspace *Workspace) {
	registerRootDescriptionWindow(routes, clientInterfaceAddress, workspace.ReadClientInterface, workspace.SaveClientInterface)
	routes.HandleFunc("POST "+clientInterfaceEditAddress, answerStudio(0, writeSourceError, func(edit ClientInterfaceEdit) (any, error) {
		return EditClientInterface(edit)
	}))
}
