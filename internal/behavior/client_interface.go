// Package behavior is how the platform changes what the model holds: where an
// element goes when it is added, what goes with it when it is deleted, how it
// moves. One function a behavior, called by the server of Studio and by the
// application alike; the shells only show what it answers and which of its
// commands are available (CLAUDE.md, «Три слоя», owner 29.10.2026).
package behavior

import (
	"errors"
	"fmt"
	"slices"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// The window «Интерфейс клиентского приложения» (docs/block-00/
// konfiguraciya.md, «Окна», «Досъёмка 27.10.2026», «Предсъёмка 28.10.2026»,
// «Досъёмка 29.10.2026, вечер»; snapshots konfiguraciya/23,
// series-2.595/53-57, 71, 72, predsyomka-1/40-44, dosyomka-2646/45-62) shows
// the four sides of the main window, the groups on them and the panels; a
// group shows its panels and not the nameless groups the prototype wraps
// each of them in (every export: a group of an identifier holds nameless
// groups of one panel each). What the window moves and deletes is what it
// shows: a panel of a group goes with its wrapper.

// ClientInterfaceSides are the sides of the main window in the order the
// window shows them, by the keys the model writes them.
var ClientInterfaceSides = []string{"top", "bottom", "left", "right"}

// ClientInterfacePlace is where an element of the client interface stands:
// its side and the indices of the items down to it, none for the side
// itself.
type ClientInterfacePlace struct {
	Side string `json:"side"`
	Path []int  `json:"path,omitempty"`
}

var errNoClientInterfaceElement = errors.New("no element of the client interface stands there")

// side answers the items of a side of value, nil for a side it has no key of.
func side(value *metadata.ClientInterface, name string) *[]metadata.ClientInterfaceItem {
	switch name {
	case "top":
		return &value.Top
	case "bottom":
		return &value.Bottom
	case "left":
		return &value.Left
	case "right":
		return &value.Right
	}
	return nil
}

// container answers the list holding the element at place and its index in
// it.
func container(value *metadata.ClientInterface, place ClientInterfacePlace) (*[]metadata.ClientInterfaceItem, int, error) {
	list := side(value, place.Side)
	if list == nil || len(place.Path) == 0 {
		return nil, 0, errNoClientInterfaceElement
	}
	for depth, index := range place.Path {
		if index < 0 || index >= len(*list) {
			return nil, 0, errNoClientInterfaceElement
		}
		if depth == len(place.Path)-1 {
			return list, index, nil
		}
		if (*list)[index].Panel != nil {
			return nil, 0, errNoClientInterfaceElement
		}
		list = &(*list)[index].Items
	}
	return nil, 0, errNoClientInterfaceElement
}

// unit answers the place of what the window moves and deletes for the
// element at place: the element, or the nameless group wrapping it alone,
// and that one's wrapper in turn.
func unit(value *metadata.ClientInterface, place ClientInterfacePlace) ClientInterfacePlace {
	for len(place.Path) > 1 {
		parent := ClientInterfacePlace{Side: place.Side, Path: place.Path[:len(place.Path)-1]}
		list, index, err := container(value, parent)
		if err != nil {
			break
		}
		wrapper := (*list)[index]
		if wrapper.ID != nil || len(wrapper.Items) != 1 {
			break
		}
		place = parent
	}
	return place
}

func newID() *uuid.UUID {
	id := uuid.MustNew()
	return &id
}

func withPath(place ClientInterfacePlace, path ...int) ClientInterfacePlace {
	return ClientInterfacePlace{Side: place.Side, Path: append(append([]int(nil), place.Path...), path...)}
}

// AddClientInterfacePanel adds a standard panel not yet chosen («Стандартная
// панель: Не выбрана», metadata.PanelID) where the prototype puts it
// (konfiguraciya.md, «Досъёмка 29.10.2026, ночь»; dosyomka-2649/02-07): at
// a side, on the side itself after its elements; at a group, in a nameless
// group of its own at the end of the group (export after the edit of
// 29.10.2026); at a panel, before it - on its side, or in its group in a
// nameless group of its own. Every panel added gets an identifier of its
// own, as the prototype gives one. Answers where the panel stands.
func AddClientInterfacePanel(value *metadata.ClientInterface, at ClientInterfacePlace) (ClientInterfacePlace, error) {
	return placePanel(value, at, metadata.ClientInterfaceItem{ID: newID(), Panel: &metadata.PanelID{}})
}

// placePanel puts the panel item where «Добавить» puts a panel at at
// (AddClientInterfacePanel) - and a drop onto the row at at with it
// (konfiguraciya.md, «Досъёмка 31.10.2026, перетаскивание»). Answers where
// it stands.
func placePanel(value *metadata.ClientInterface, at ClientInterfacePlace, panel metadata.ClientInterfaceItem) (ClientInterfacePlace, error) {
	if len(at.Path) == 0 {
		list := side(value, at.Side)
		if list == nil {
			return ClientInterfacePlace{}, errNoClientInterfaceElement
		}
		*list = append(*list, panel)
		return withPath(at, len(*list)-1), nil
	}
	list, index, err := container(value, at)
	if err != nil {
		return ClientInterfacePlace{}, err
	}
	if item := &(*list)[index]; item.Panel == nil {
		if item.ID == nil {
			return ClientInterfacePlace{}, fmt.Errorf("a panel is added to a side, a group or a panel")
		}
		item.Items = append(item.Items, metadata.ClientInterfaceItem{Items: []metadata.ClientInterfaceItem{panel}})
		return withPath(at, len(item.Items)-1, 0), nil
	}
	// At a panel: before what stands for it - itself on a side, its nameless
	// group in a group.
	before := unit(value, at)
	list, index, _ = container(value, before)
	parent := ClientInterfacePlace{Side: at.Side, Path: before.Path[:len(before.Path)-1]}
	if len(before.Path) == len(at.Path) && len(at.Path) == 1 {
		*list = slices.Insert(*list, index, panel)
		return withPath(parent, index), nil
	}
	*list = slices.Insert(*list, index, metadata.ClientInterfaceItem{Items: []metadata.ClientInterfaceItem{panel}})
	return withPath(parent, index, 0), nil
}

// AddClientInterfaceGroup adds a group of an identifier of its own: at a
// side, at the end of the side (series-2.595/54); at a group or a panel of
// the side, before it on the side (dosyomka-2649/08, 09). A group goes into
// no group: at a panel of a group «Тип элемента» offers the panel alone
// (dosyomka-2649/05). Answers where it stands.
func AddClientInterfaceGroup(value *metadata.ClientInterface, at ClientInterfacePlace) (ClientInterfacePlace, error) {
	list := side(value, at.Side)
	if list == nil {
		return ClientInterfacePlace{}, errNoClientInterfaceElement
	}
	group := metadata.ClientInterfaceItem{ID: newID()}
	if len(at.Path) == 0 {
		*list = append(*list, group)
		return ClientInterfacePlace{Side: at.Side, Path: []int{len(*list) - 1}}, nil
	}
	if _, _, err := container(value, at); err != nil {
		return ClientInterfacePlace{}, err
	}
	before := unit(value, at)
	if len(before.Path) != 1 {
		return ClientInterfacePlace{}, fmt.Errorf("a group is added to a side, before a group or a panel of the side")
	}
	*list = slices.Insert(*list, before.Path[0], group)
	return before, nil
}

// RemoveClientInterfaceElement deletes the element at place - a panel with
// the nameless group wrapping it, a group with what it holds; a side is not
// deleted.
func RemoveClientInterfaceElement(value *metadata.ClientInterface, place ClientInterfacePlace) error {
	list, index, err := container(value, unit(value, place))
	if err != nil {
		return err
	}
	*list = append((*list)[:index], (*list)[index+1:]...)
	return nil
}

// MoveClientInterfaceElement moves the element at place one place up (by -1)
// or down (by 1) among the elements of its side or group, nothing at the
// ends: the arrows never take it out of its group (series-2.595/71). Answers
// where it stands then and whether it moved.
func MoveClientInterfaceElement(value *metadata.ClientInterface, place ClientInterfacePlace, by int) (ClientInterfacePlace, bool, error) {
	moved := unit(value, place)
	list, index, err := container(value, moved)
	if err != nil {
		return place, false, err
	}
	to := index + by
	if (by != -1 && by != 1) || to < 0 || to >= len(*list) {
		return place, false, nil
	}
	(*list)[index], (*list)[to] = (*list)[to], (*list)[index]
	depth := len(moved.Path) - 1
	result := withPath(place)
	result.Path[depth] = to
	return result, true, nil
}

// ChooseClientInterfacePanel makes the panel at place show panel: a panel of
// the platform not placed elsewhere - «Выбор стандартной панели» leaves OK
// unavailable on a panel placed already (series-2.595/55).
func ChooseClientInterfacePanel(value *metadata.ClientInterface, place ClientInterfacePlace, panel uuid.UUID) error {
	list, index, err := container(value, place)
	if err != nil {
		return err
	}
	item := &(*list)[index]
	if item.Panel == nil {
		return fmt.Errorf("a group shows no panel")
	}
	if panel.IsZero() {
		return fmt.Errorf("no panel is chosen")
	}
	if *item.Panel != metadata.PanelID(panel) && PlacedClientInterfacePanels(*value)[panel] {
		return fmt.Errorf("the panel stands in the interface already")
	}
	*item.Panel = metadata.PanelID(panel)
	return nil
}

// DropClientInterfacePanel puts a panel of the platform dragged from the list
// of the window onto the row at at - a side, a group or a panel - where
// «Добавить» puts a panel at it: on a side at its end on the side itself, on
// a group at its end in a nameless group of its own, on a panel before it
// (konfiguraciya.md, «Досъёмка 31.10.2026, перетаскивание»; series-2.595/71,
// dosyomka-2667/20, 22, 23). A panel free stands there chosen; a panel
// placed already neither moves nor stands twice - a standard panel none
// chosen stands there instead, the placed one staying where it stood
// (dosyomka-2667/24, 25; one panel of the platform stands once, the owner,
// 29.10.2026). A zero panel and a row that is not there are refused,
// nothing changed. Answers where the panel dropped stands.
func DropClientInterfacePanel(value *metadata.ClientInterface, at ClientInterfacePlace, panel uuid.UUID) (ClientInterfacePlace, error) {
	if panel.IsZero() {
		return ClientInterfacePlace{}, fmt.Errorf("no panel is dropped")
	}
	if PlacedClientInterfacePanels(*value)[panel] {
		return AddClientInterfacePanel(value, at)
	}
	chosen := metadata.PanelID(panel)
	return placePanel(value, at, metadata.ClientInterfaceItem{ID: newID(), Panel: &chosen})
}

// MoveClientInterfacePanelTo moves the panel at from - a row of the tree
// dragged - onto the row at to: onto a side at its end on the side itself,
// onto a panel before it, on its side or in its group in a nameless group
// of its own; the panel goes from where it stood, with the nameless group
// wrapping it (konfiguraciya.md, «Досъёмка 31.10.2026, перетаскивание»;
// dosyomka-2667/26-28). It keeps its identifier and its panel. A panel
// dropped onto itself stays. A group dragged and a drop onto a group are
// not shot and refused, as is a place that is not there; nothing is
// changed by a refusal. Answers where the panel stands.
func MoveClientInterfacePanelTo(value *metadata.ClientInterface, from, to ClientInterfacePlace) (ClientInterfacePlace, error) {
	list, index, err := container(value, from)
	if err != nil {
		return ClientInterfacePlace{}, err
	}
	moved := (*list)[index]
	if moved.Panel == nil {
		return ClientInterfacePlace{}, fmt.Errorf("a panel is dragged, not a group")
	}
	if len(to.Path) > 0 {
		target, at, err := container(value, to)
		if err != nil {
			return ClientInterfacePlace{}, err
		}
		if (*target)[at].Panel == nil {
			return ClientInterfacePlace{}, fmt.Errorf("a panel is dropped onto a side or a panel")
		}
	} else if side(value, to.Side) == nil {
		return ClientInterfacePlace{}, errNoClientInterfaceElement
	}
	if to.Side == from.Side && slices.Equal(to.Path, from.Path) {
		return from, nil
	}
	gone := unit(value, from)
	if err := RemoveClientInterfaceElement(value, from); err != nil {
		return ClientInterfacePlace{}, err
	}
	// What stood after the unit taken away in the same list stands one
	// place before.
	if to.Side == gone.Side && len(to.Path) >= len(gone.Path) {
		depth := len(gone.Path) - 1
		if slices.Equal(to.Path[:depth], gone.Path[:depth]) && to.Path[depth] > gone.Path[depth] {
			to = withPath(to)
			to.Path[depth]--
		}
	}
	return placePanel(value, to, moved)
}

// PlacedClientInterfacePanels answers the panels the sides show, the ones
// the list of the window marks.
func PlacedClientInterfacePanels(value metadata.ClientInterface) map[uuid.UUID]bool {
	placed := map[uuid.UUID]bool{}
	var walk func([]metadata.ClientInterfaceItem)
	walk = func(items []metadata.ClientInterfaceItem) {
		for _, item := range items {
			if item.Panel != nil && item.Panel.Chosen() {
				placed[uuid.UUID(*item.Panel)] = true
			}
			walk(item.Items)
		}
	}
	for _, name := range ClientInterfaceSides {
		walk(*side(&value, name))
	}
	return placed
}

// SetSectionsPanelRepresentation sets how the panel of sections is shown
// («Описание панели», «Отображение»): on its definition, which it makes where
// the interface has none; «Картинка и текст», the default, is written as
// none, as the export leaves it out.
func SetSectionsPanelRepresentation(value *metadata.ClientInterface, representation metadata.SectionsPanelRepresentation) {
	if representation == metadata.SectionsPanelPictureAndText {
		representation = ""
	}
	for index := range value.Panels {
		if value.Panels[index].ID == metadata.SectionsPanel {
			value.Panels[index].PanelRepresentation = representation
			return
		}
	}
	value.Panels = append(value.Panels, metadata.ClientInterfacePanel{ID: metadata.SectionsPanel, PanelRepresentation: representation})
}
