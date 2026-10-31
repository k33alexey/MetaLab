package behavior

import (
	"bytes"
	"slices"
	"testing"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// The panels of the platform the tests place, by the identifiers the export
// writes (docs/block-00/konfiguraciya.md, «Досъёмка 29.10.2026, вечер»).
var (
	sectionsPanel   = uuid.MustParse("b553047f-c9aa-4157-978d-448ecad24248")
	openPanel       = uuid.MustParse("cbab57f2-a0f3-4f0a-89ea-4cb19570ab75")
	historyPanel    = uuid.MustParse("c933ac92-92cd-459d-81cc-e0c8a83ced99")
	favouritesPanel = uuid.MustParse("13322b22-3960-4d68-93a6-fe2dd7f28ca3")
	unknownPanel    = uuid.MustParse("8e10648b-f52d-4ec2-b4dd-87de33778d95")
)

func panelItem(panel uuid.UUID) metadata.ClientInterfaceItem {
	id := uuid.MustNew()
	chosen := metadata.PanelID(panel)
	return metadata.ClientInterfaceItem{ID: &id, Panel: &chosen}
}

func wrapped(panel uuid.UUID) metadata.ClientInterfaceItem {
	return metadata.ClientInterfaceItem{Items: []metadata.ClientInterfaceItem{panelItem(panel)}}
}

// erpInterface is the interface of erp: at the top a group of the panel the
// platform's list does not have and the panel of open windows, at the left a
// group of the panel of sections - each panel of a group in a nameless group
// of its own.
func erpInterface() metadata.ClientInterface {
	top, left := uuid.MustNew(), uuid.MustNew()
	return metadata.ClientInterface{
		Top:  []metadata.ClientInterfaceItem{{ID: &top, Items: []metadata.ClientInterfaceItem{wrapped(unknownPanel), wrapped(openPanel)}}},
		Left: []metadata.ClientInterfaceItem{{ID: &left, Items: []metadata.ClientInterfaceItem{wrapped(sectionsPanel)}}},
	}
}

// metadataIssues is what the model says of value written and read back as
// the project keeps it, "" for nothing.
func metadataIssues(value metadata.ClientInterface) string {
	var encoded bytes.Buffer
	if err := metadata.Encode(&encoded, value); err != nil {
		return err.Error()
	}
	if _, err := metadata.DecodeRootDescription(project.ClientInterfaceFile, &encoded); err != nil {
		return err.Error()
	}
	return ""
}

func place(side string, path ...int) ClientInterfacePlace {
	return ClientInterfacePlace{Side: side, Path: path}
}

// A panel added to a group stands in a nameless group of its own at the end
// of the group, a panel added to a side stands on the side itself, a group at
// the end of the side; each has an identifier of its own and the panel none
// chosen (export after the edit of 29.10.2026). Defects caught: the panel
// put into the group bare, beside the wrappers, the export never writes; a
// panel of a side wrapped; a panel added with no identifier, or with a panel.
func TestAPanelAndAGroupAreAddedWhereThePrototypePutsThem(t *testing.T) {
	value := erpInterface()
	at, err := AddClientInterfacePanel(&value, place("top", 0))
	if err != nil || at.Side != "top" || len(at.Path) != 3 || at.Path[0] != 0 || at.Path[1] != 2 || at.Path[2] != 0 {
		t.Fatalf("the panel of the group stands at %+v: %v", at, err)
	}
	wrapper := value.Top[0].Items[2]
	if wrapper.ID != nil || wrapper.Panel != nil || len(wrapper.Items) != 1 {
		t.Fatalf("the panel added to the group is not wrapped alone: %+v", wrapper)
	}
	added := wrapper.Items[0]
	if added.ID == nil || added.ID.IsZero() || added.Panel == nil || added.Panel.Chosen() {
		t.Fatalf("the panel added is not a panel none chosen of an identifier: %+v", added)
	}
	at, err = AddClientInterfacePanel(&value, place("right"))
	if err != nil || len(at.Path) != 1 || at.Path[0] != 0 || len(value.Right) != 1 || value.Right[0].Panel == nil || len(value.Right[0].Items) != 0 {
		t.Fatalf("the panel of the side does not stand on the side: %+v %+v %v", at, value.Right, err)
	}
	at, err = AddClientInterfaceGroup(&value, place("left"))
	if err != nil || len(at.Path) != 1 || at.Path[0] != 1 || value.Left[1].ID == nil || value.Left[1].Panel != nil {
		t.Fatalf("the group does not stand at the end of the side: %+v %+v %v", at, value.Left, err)
	}
	if _, err := AddClientInterfacePanel(&value, place("middle")); err == nil {
		t.Fatal("a panel was added to a side the window has not")
	}
	if issues := metadataIssues(value); issues != "" {
		t.Fatalf("what was added is refused by the model: %s", issues)
	}
}

// «Добавить» at an element puts the new one before it (konfiguraciya.md,
// «Досъёмка 29.10.2026, ночь»; dosyomka-2649/02-09): a panel at a panel of a
// side - before it on the side, bare; at a panel of a group - before it in
// the group, in a nameless group of its own; a group at a group or a panel
// of a side - before it on the side; a group at a panel of a group is
// refused, as «Тип элемента» offers it no group there. Defects caught: the
// new one put after the current or at the end, a panel of a side wrapped, a
// panel of a group left bare, a group put into a group, the place answered
// not the new one's.
func TestWhatIsAddedAtAnElementStandsBeforeIt(t *testing.T) {
	value := metadata.ClientInterface{Top: []metadata.ClientInterfaceItem{panelItem(sectionsPanel), panelItem(openPanel)}}
	at, err := AddClientInterfacePanel(&value, place("top", 1))
	if err != nil || len(at.Path) != 1 || at.Path[0] != 1 || len(value.Top) != 3 || value.Top[1].Panel == nil || value.Top[1].Panel.Chosen() ||
		*value.Top[2].Panel != metadata.PanelID(openPanel) {
		t.Fatalf("the panel at a panel of the side stands at %+v: %+v %v", at, value.Top, err)
	}
	at, err = AddClientInterfaceGroup(&value, place("top", 0))
	if err != nil || len(at.Path) != 1 || at.Path[0] != 0 || value.Top[0].ID == nil || value.Top[0].Panel != nil || len(value.Top) != 4 {
		t.Fatalf("the group at a panel of the side stands at %+v: %+v %v", at, value.Top, err)
	}
	value = erpInterface()
	at, err = AddClientInterfacePanel(&value, place("top", 0, 1, 0))
	if err != nil || len(at.Path) != 3 || at.Path[0] != 0 || at.Path[1] != 1 || at.Path[2] != 0 {
		t.Fatalf("the panel at a panel of the group stands at %+v: %v", at, err)
	}
	group := value.Top[0].Items
	if len(group) != 3 || group[1].ID != nil || len(group[1].Items) != 1 || group[1].Items[0].Panel.Chosen() || *group[2].Items[0].Panel != metadata.PanelID(openPanel) {
		t.Fatalf("the group holds %+v", group)
	}
	at, err = AddClientInterfaceGroup(&value, place("left", 0))
	if err != nil || len(at.Path) != 1 || at.Path[0] != 0 || value.Left[0].ID == nil || len(value.Left[0].Items) != 0 || len(value.Left[1].Items) != 1 {
		t.Fatalf("the group at a group stands at %+v: %+v %v", at, value.Left, err)
	}
	before := len(value.Top[0].Items)
	if _, err := AddClientInterfaceGroup(&value, place("top", 0, 1, 0)); err == nil || len(value.Top[0].Items) != before || len(value.Top) != 1 {
		t.Fatalf("a group was added at a panel of a group: %v %+v", err, value.Top)
	}
	if issues := metadataIssues(value); issues != "" {
		t.Fatalf("what was added is refused by the model: %s", issues)
	}
}

// The arrows move a panel of a group among the panels of the group - its
// wrapper goes with it - and stop at the ends: the first up and the last
// down do nothing (series-2.595/71). Defects caught: the panel moved out of
// its wrapper, the wrapper left empty; the arrows going round; the place
// answered not following the panel.
func TestThePanelsOfAGroupMoveWithinIt(t *testing.T) {
	value := erpInterface()
	at, moved, err := MoveClientInterfaceElement(&value, place("top", 0, 1, 0), -1)
	if err != nil || !moved || at.Path[1] != 0 || at.Path[2] != 0 {
		t.Fatalf("the panel of open windows did not go up: %+v %v %v", at, moved, err)
	}
	if *value.Top[0].Items[0].Items[0].Panel != metadata.PanelID(openPanel) || *value.Top[0].Items[1].Items[0].Panel != metadata.PanelID(unknownPanel) {
		t.Fatalf("the wrappers did not change places: %+v", value.Top[0].Items)
	}
	if _, moved, _ := MoveClientInterfaceElement(&value, at, -1); moved {
		t.Fatal("the first panel of the group went up")
	}
	if _, moved, _ := MoveClientInterfaceElement(&value, place("top", 0, 1, 0), 1); moved {
		t.Fatal("the last panel of the group went down")
	}
	if _, moved, _ := MoveClientInterfaceElement(&value, place("top", 0), 1); moved {
		t.Fatal("the only group of the side moved")
	}
	if _, _, err := MoveClientInterfaceElement(&value, place("top"), 1); err == nil {
		t.Fatal("a side moved")
	}
}

// Deleting a panel of a group deletes its wrapper; deleting a group deletes
// what it holds; a side is not deleted. Defects caught: an empty nameless
// group left in the group, which the window does not show and the file
// keeps.
func TestADeletedPanelTakesItsWrapper(t *testing.T) {
	value := erpInterface()
	if err := RemoveClientInterfaceElement(&value, place("top", 0, 0, 0)); err != nil {
		t.Fatal(err)
	}
	if len(value.Top[0].Items) != 1 || *value.Top[0].Items[0].Items[0].Panel != metadata.PanelID(openPanel) {
		t.Fatalf("the panel went and its wrapper stayed: %+v", value.Top[0].Items)
	}
	if err := RemoveClientInterfaceElement(&value, place("left", 0)); err != nil || len(value.Left) != 0 {
		t.Fatalf("the group did not go with its panel: %+v %v", value.Left, err)
	}
	if err := RemoveClientInterfaceElement(&value, place("left")); err == nil {
		t.Fatal("a side was deleted")
	}
	if err := RemoveClientInterfaceElement(&value, place("top", 0, 5, 0)); err == nil {
		t.Fatal("nothing standing there was deleted")
	}
}

// A panel chosen is a panel of the platform no other element shows: the
// choice offers the placed ones unavailable (series-2.595/55). Defects
// caught: one panel shown twice; a group given a panel; the zero chosen.
func TestAPanelIsChosenOnce(t *testing.T) {
	value := erpInterface()
	at, _ := AddClientInterfacePanel(&value, place("bottom"))
	if err := ChooseClientInterfacePanel(&value, at, openPanel); err == nil {
		t.Fatal("the panel of open windows was chosen twice")
	}
	if err := ChooseClientInterfacePanel(&value, at, uuid.UUID{}); err == nil {
		t.Fatal("no panel was chosen")
	}
	if err := ChooseClientInterfacePanel(&value, place("top", 0), historyPanel); err == nil {
		t.Fatal("a group was given a panel")
	}
	if err := ChooseClientInterfacePanel(&value, at, historyPanel); err != nil || *value.Bottom[0].Panel != metadata.PanelID(historyPanel) {
		t.Fatalf("the panel of history was not chosen: %+v %v", value.Bottom, err)
	}
	if err := ChooseClientInterfacePanel(&value, at, historyPanel); err != nil {
		t.Fatalf("a panel chosen again for itself was refused: %v", err)
	}
	placed := PlacedClientInterfacePanels(value)
	if len(placed) != 4 || !placed[historyPanel] || !placed[unknownPanel] || placed[uuid.UUID{}] {
		t.Fatalf("the panels placed are told wrong: %v", placed)
	}
}

// The representation of the panel of sections lies on its definition, made
// where there is none; «Картинка и текст», the default, is written as none.
// Defects caught: a second definition of the panel; the default written,
// which the export leaves out.
func TestTheRepresentationLiesOnTheDefinitionOfThePanelOfSections(t *testing.T) {
	value := erpInterface()
	SetSectionsPanelRepresentation(&value, metadata.SectionsPanelText)
	SetSectionsPanelRepresentation(&value, metadata.SectionsPanelPictureOnTopAndText)
	if len(value.Panels) != 1 || value.Panels[0].ID != sectionsPanel || value.Panels[0].PanelRepresentation != metadata.SectionsPanelPictureOnTopAndText {
		t.Fatalf("the representation is not on one definition: %+v", value.Panels)
	}
	SetSectionsPanelRepresentation(&value, metadata.SectionsPanelPictureAndText)
	if value.Panels[0].PanelRepresentation != "" {
		t.Fatalf("the default is written: %+v", value.Panels)
	}
}

// A panel dragged from the list stands where «Добавить» puts one at the row
// it is dropped onto (konfiguraciya.md, «Досъёмка 31.10.2026,
// перетаскивание»; series-2.595/71, dosyomka-2667/20-25): onto a group at
// its end in a nameless group of its own, onto a side at its end on the
// side itself, onto a panel of a group before it in a nameless group of its
// own, onto a panel of a side before it on the side; chosen, of an
// identifier of its own. A panel placed already puts a panel none chosen
// there and stays where it stood. A zero panel and a row not there are
// refused, the interface left as it was. Defects caught: the panel put
// first or bare into the group, wrapped on a side, after the panel dropped
// onto, left not chosen; a panel of the platform placed twice or moved from
// its place; a drop onto a side or a panel refused; a refusal leaving an
// empty panel behind.
func TestAPanelDraggedFromTheListStandsWhereAddPutsOne(t *testing.T) {
	favourites := favouritesPanel
	chosen := func(item metadata.ClientInterfaceItem, panel uuid.UUID) bool {
		return item.Panel != nil && uuid.UUID(*item.Panel) == panel && item.ID != nil && !item.ID.IsZero() && len(item.Items) == 0
	}
	value := erpInterface()
	at, err := DropClientInterfacePanel(&value, place("left", 0), favourites)
	if err != nil || !slices.Equal(at.Path, []int{0, 1, 0}) || at.Side != "left" {
		t.Fatalf("the panel dropped onto the group stands at %+v: %v", at, err)
	}
	if group := value.Left[0].Items; len(group) != 2 || group[1].ID != nil || len(group[1].Items) != 1 || !chosen(group[1].Items[0], favourites) {
		t.Fatalf("the group holds %+v", group)
	}
	value = erpInterface()
	at, err = DropClientInterfacePanel(&value, place("right"), favourites)
	if err != nil || at.Side != "right" || !slices.Equal(at.Path, []int{0}) || len(value.Right) != 1 || !chosen(value.Right[0], favourites) {
		t.Fatalf("the panel dropped onto a side stands at %+v: %v; the side holds %+v", at, err, value.Right)
	}
	at, err = DropClientInterfacePanel(&value, place("right", 0), historyPanel)
	if err != nil || !slices.Equal(at.Path, []int{0}) || len(value.Right) != 2 || !chosen(value.Right[0], historyPanel) || !chosen(value.Right[1], favourites) {
		t.Fatalf("the panel dropped onto a panel of a side stands at %+v: %v; the side holds %+v", at, err, value.Right)
	}
	value = erpInterface()
	at, err = DropClientInterfacePanel(&value, place("top", 0, 1, 0), historyPanel)
	if group := value.Top[0].Items; err != nil || !slices.Equal(at.Path, []int{0, 1, 0}) || len(group) != 3 || group[1].ID != nil || len(group[1].Items) != 1 ||
		!chosen(group[1].Items[0], historyPanel) || !chosen(group[2].Items[0], openPanel) {
		t.Fatalf("the panel dropped onto a panel of a group stands at %+v: %v; the group holds %+v", at, err, value.Top[0].Items)
	}
	// Placed already: the panel of sections dropped onto the right side.
	value = erpInterface()
	at, err = DropClientInterfacePanel(&value, place("right"), sectionsPanel)
	if err != nil || !slices.Equal(at.Path, []int{0}) || len(value.Right) != 1 || value.Right[0].Panel == nil || value.Right[0].Panel.Chosen() || value.Right[0].ID == nil {
		t.Fatalf("a placed panel dropped puts %+v at %+v: %v", value.Right, at, err)
	}
	if left := value.Left[0].Items; len(left) != 1 || !chosen(left[0].Items[0], sectionsPanel) {
		t.Fatalf("the placed panel moved: %+v", left)
	}
	if issue := metadataIssues(value); issue != "" {
		t.Fatalf("the model refuses the interface: %s", issue)
	}
	for name, drop := range map[string]struct {
		at    ClientInterfacePlace
		panel uuid.UUID
	}{
		"нулевая":         {place("top", 0), uuid.UUID{}},
		"мимо интерфейса": {place("top", 5), historyPanel},
		"мимо стороны":    {place("middle"), historyPanel},
	} {
		value := erpInterface()
		if _, err := DropClientInterfacePanel(&value, drop.at, drop.panel); err == nil {
			t.Errorf("%s: the drop was taken", name)
		}
		if len(value.Top[0].Items) != 2 || len(value.Right) != 0 {
			t.Errorf("%s: the refusal changed the interface: %+v %+v", name, value.Top, value.Right)
		}
	}
}

// A panel of the tree dragged moves, keeping its identifier, from where it
// stood with its nameless group (konfiguraciya.md, «Досъёмка 31.10.2026,
// перетаскивание»; dosyomka-2667/26-28): onto a side to its end on the side
// itself, onto a panel before it - in its group, wrapped; onto itself it
// stays; a side dragged is refused, the interface left as it was. Defects
// caught: the panel left at the old place or its empty wrapper left; the
// panel wrapped on a side or bare in a group; the place of the target taken
// before the panel went - one place off in the same list; a new
// identifier; a side moved.
func TestAPanelOfTheTreeDraggedMoves(t *testing.T) {
	value := erpInterface()
	open := *value.Top[0].Items[1].Items[0].ID
	at, err := MoveClientInterfaceElementTo(&value, place("top", 0, 1, 0), place("right"))
	if err != nil || at.Side != "right" || !slices.Equal(at.Path, []int{0}) || len(value.Right) != 1 || *value.Right[0].ID != open || len(value.Right[0].Items) != 0 {
		t.Fatalf("the panel of a group dropped onto a side stands at %+v: %v; the side holds %+v", at, err, value.Right)
	}
	if len(value.Top[0].Items) != 1 {
		t.Fatalf("the group keeps %+v", value.Top[0].Items)
	}
	at, err = MoveClientInterfaceElementTo(&value, place("right", 0), place("left", 0, 0, 0))
	if left := value.Left[0].Items; err != nil || !slices.Equal(at.Path, []int{0, 0, 0}) || at.Side != "left" || len(value.Right) != 0 || len(left) != 2 ||
		left[0].ID != nil || *left[0].Items[0].ID != open || uuid.UUID(*left[1].Items[0].Panel) != sectionsPanel {
		t.Fatalf("the panel of a side dropped onto a panel of a group stands at %+v: %v; the group holds %+v", at, err, value.Left[0].Items)
	}
	// In the same group: the last panel before the first, the first onto the
	// last - before it, one place off once the first went.
	value = erpInterface()
	unknown := *value.Top[0].Items[0].Items[0].ID
	at, err = MoveClientInterfaceElementTo(&value, place("top", 0, 0, 0), place("top", 0, 1, 0))
	if group := value.Top[0].Items; err != nil || !slices.Equal(at.Path, []int{0, 0, 0}) || len(group) != 2 || *group[0].Items[0].ID != unknown {
		t.Fatalf("the first panel dropped onto the next stands at %+v: %v; the group holds %+v", at, err, value.Top[0].Items)
	}
	value.Top = append(value.Top, panelItem(historyPanel), panelItem(favouritesPanel))
	history := *value.Top[1].ID
	at, err = MoveClientInterfaceElementTo(&value, place("top", 1), place("top", 2))
	if err != nil || !slices.Equal(at.Path, []int{1}) || *value.Top[1].ID != history {
		t.Fatalf("a panel of a side dropped onto the next stands at %+v: %v", at, err)
	}
	at, err = MoveClientInterfaceElementTo(&value, place("top", 2), place("top", 1))
	if err != nil || !slices.Equal(at.Path, []int{1}) || uuid.UUID(*value.Top[1].Panel) != favouritesPanel || *value.Top[2].ID != history {
		t.Fatalf("a panel of a side dropped onto the one before stands at %+v: %v; the side holds %+v", at, err, value.Top)
	}
	at, err = MoveClientInterfaceElementTo(&value, place("top", 1), place("top", 1))
	if err != nil || !slices.Equal(at.Path, []int{1}) || len(value.Top) != 3 {
		t.Fatalf("a panel dropped onto itself goes to %+v: %v", at, err)
	}
	if issue := metadataIssues(value); issue != "" {
		t.Fatalf("the model refuses the interface: %s", issue)
	}
	for name, move := range map[string][2]ClientInterfacePlace{
		"сторона":         {place("top"), place("right")},
		"мимо интерфейса": {place("top", 0, 1, 0), place("top", 7)},
		"мимо стороны":    {place("top", 0, 1, 0), place("middle")},
	} {
		value := erpInterface()
		if _, err := MoveClientInterfaceElementTo(&value, move[0], move[1]); err == nil {
			t.Errorf("%s: the drop was taken", name)
		}
		if len(value.Top[0].Items) != 2 || len(value.Left[0].Items) != 1 || len(value.Right) != 0 {
			t.Errorf("%s: the refusal changed the interface: %+v %+v %+v", name, value.Top, value.Left, value.Right)
		}
	}
}

// A panel of the tree dragged onto a group goes to the end of the group, in a
// nameless group of its own, from where it stood - its own group too; a group
// dragged goes onto a side to its end, onto a group or a panel of a side
// before it, onto a panel of a group nowhere - the drop changes nothing
// (konfiguraciya.md, «Досъёмка 31.10.2026, перетаскивание», В2 of series
// 2.672; configurator-2026-10-31/vopr-2672/04-13). The group keeps its
// identifier and its panels. A side, a nameless group and a drop onto a
// nameless group are refused, the interface left as it was. Defects caught:
// a panel onto a group refused, or put bare or first into it; the panel
// left at the old place; a group refused, put after the target or one place
// off once it went from before it; a group put into a group; a group losing
// what it holds; a nameless group moved.
func TestAGroupAndAPanelOntoAGroupDraggedMove(t *testing.T) {
	value := erpInterface()
	open := *value.Top[0].Items[1].Items[0].ID
	at, err := MoveClientInterfaceElementTo(&value, place("top", 0, 1, 0), place("left", 0))
	if left := value.Left[0].Items; err != nil || at.Side != "left" || !slices.Equal(at.Path, []int{0, 1, 0}) || len(left) != 2 ||
		left[1].ID != nil || len(left[1].Items) != 1 || *left[1].Items[0].ID != open || len(value.Top[0].Items) != 1 {
		t.Fatalf("a panel dropped onto another group stands at %+v: %v; the groups hold %+v and %+v", at, err, value.Top[0].Items, value.Left[0].Items)
	}
	value = erpInterface()
	unknown, open := *value.Top[0].Items[0].Items[0].ID, *value.Top[0].Items[1].Items[0].ID
	at, err = MoveClientInterfaceElementTo(&value, place("top", 0, 0, 0), place("top", 0))
	if group := value.Top[0].Items; err != nil || !slices.Equal(at.Path, []int{0, 1, 0}) || len(group) != 2 || *group[1].Items[0].ID != unknown || *group[0].Items[0].ID != open {
		t.Fatalf("a panel dropped onto its own group stands at %+v: %v; the group holds %+v", at, err, value.Top[0].Items)
	}
	value = erpInterface()
	value.Top = append(value.Top, panelItem(historyPanel))
	first, second, history := *value.Top[0].ID, *value.Left[0].ID, *value.Top[1].ID
	ids := func() []uuid.UUID {
		var answer []uuid.UUID
		for _, item := range value.Top {
			answer = append(answer, *item.ID)
		}
		return answer
	}
	at, err = MoveClientInterfaceElementTo(&value, place("left", 0), place("top"))
	if err != nil || !slices.Equal(at.Path, []int{2}) || at.Side != "top" || len(value.Left) != 0 || !slices.Equal(ids(), []uuid.UUID{first, history, second}) ||
		len(value.Top[2].Items) != 1 || uuid.UUID(*value.Top[2].Items[0].Items[0].Panel) != sectionsPanel {
		t.Fatalf("a group dropped onto a side stands at %+v: %v; the top holds %v", at, err, ids())
	}
	at, err = MoveClientInterfaceElementTo(&value, place("top", 2), place("top", 0))
	if err != nil || !slices.Equal(at.Path, []int{0}) || !slices.Equal(ids(), []uuid.UUID{second, first, history}) {
		t.Fatalf("a group dropped onto a group stands at %+v: %v; the top holds %v", at, err, ids())
	}
	at, err = MoveClientInterfaceElementTo(&value, place("top", 0), place("top", 2))
	if err != nil || !slices.Equal(at.Path, []int{1}) || !slices.Equal(ids(), []uuid.UUID{first, second, history}) {
		t.Fatalf("a group dropped onto a panel of the side after it stands at %+v: %v; the top holds %v", at, err, ids())
	}
	for name, target := range map[string]ClientInterfacePlace{"на панель группы": place("top", 0, 0, 0), "на свою панель": place("top", 1, 0, 0), "на себя": place("top", 1)} {
		at, err = MoveClientInterfaceElementTo(&value, place("top", 1), target)
		if err != nil || !slices.Equal(at.Path, []int{1}) || !slices.Equal(ids(), []uuid.UUID{first, second, history}) || len(value.Top[0].Items) != 2 {
			t.Fatalf("%s: the group went to %+v: %v; the top holds %v", name, at, err, ids())
		}
	}
	if issue := metadataIssues(value); issue != "" {
		t.Fatalf("the model refuses the interface: %s", issue)
	}
	for name, move := range map[string][2]ClientInterfacePlace{
		"сторона":              {place("top"), place("right")},
		"безымянная группа":    {place("top", 0, 1), place("right")},
		"на безымянную группу": {place("left", 0, 0, 0), place("top", 0, 1)},
		"мимо интерфейса":      {place("top", 0), place("top", 7)},
		"мимо стороны":         {place("top", 0), place("middle")},
	} {
		value := erpInterface()
		if _, err := MoveClientInterfaceElementTo(&value, move[0], move[1]); err == nil {
			t.Errorf("%s: the drop was taken", name)
		}
		if len(value.Top) != 1 || len(value.Top[0].Items) != 2 || len(value.Left[0].Items) != 1 || len(value.Right) != 0 {
			t.Errorf("%s: the refusal changed the interface: %+v %+v %+v", name, value.Top, value.Left, value.Right)
		}
	}
}
