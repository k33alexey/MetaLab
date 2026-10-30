package behavior

import (
	"bytes"
	"testing"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// The panels of the platform the tests place, by the identifiers the export
// writes (docs/block-00/konfiguraciya.md, «Досъёмка 29.10.2026, вечер»).
var (
	sectionsPanel = uuid.MustParse("b553047f-c9aa-4157-978d-448ecad24248")
	openPanel     = uuid.MustParse("cbab57f2-a0f3-4f0a-89ea-4cb19570ab75")
	historyPanel  = uuid.MustParse("c933ac92-92cd-459d-81cc-e0c8a83ced99")
	unknownPanel  = uuid.MustParse("8e10648b-f52d-4ec2-b4dd-87de33778d95")
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
	at, err = AddClientInterfaceGroup(&value, place("left", 0))
	if err != nil || len(at.Path) != 1 || at.Path[0] != 1 || value.Left[1].ID == nil || value.Left[1].Panel != nil {
		t.Fatalf("the group does not stand at the end of the side: %+v %+v %v", at, value.Left, err)
	}
	if _, err := AddClientInterfacePanel(&value, place("top", 0, 1, 0)); err == nil {
		t.Fatal("a panel was added to a panel")
	}
	if _, err := AddClientInterfacePanel(&value, place("middle")); err == nil {
		t.Fatal("a panel was added to a side the window has not")
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

// A panel dragged from the list onto a group stands at the end of the
// group, in a nameless group of its own, chosen, of an identifier of its own
// (series-2.595/71); a panel placed already, a zero panel, a drop onto a
// side or onto a panel are refused, the interface left as it was. Defects
// caught: the panel put first or bare into the group, left not chosen, a
// panel of the platform placed twice, a drop onto a side or a panel taken,
// a refusal leaving an empty panel behind.
func TestAPanelDraggedOntoAGroupStandsAtItsEnd(t *testing.T) {
	favourites := uuid.MustParse("13322b22-3960-4d68-93a6-fe2dd7f28ca3")
	value := erpInterface()
	at, err := DropClientInterfacePanel(&value, place("left", 0), favourites)
	if err != nil || at.Side != "left" || len(at.Path) != 3 || at.Path[0] != 0 || at.Path[1] != 1 || at.Path[2] != 0 {
		t.Fatalf("the panel dropped stands at %+v: %v", at, err)
	}
	group := value.Left[0].Items
	if len(group) != 2 || group[1].ID != nil || len(group[1].Items) != 1 {
		t.Fatalf("the group holds %+v", group)
	}
	dropped := group[1].Items[0]
	if dropped.Panel == nil || uuid.UUID(*dropped.Panel) != favourites || dropped.ID == nil || dropped.ID.IsZero() {
		t.Fatalf("the panel dropped is %+v", dropped)
	}
	if issue := metadataIssues(value); issue != "" {
		t.Fatalf("the model refuses the interface: %s", issue)
	}
	for name, drop := range map[string]struct {
		at    ClientInterfacePlace
		panel uuid.UUID
	}{
		"размещённая":     {place("top", 0), sectionsPanel},
		"нулевая":         {place("top", 0), uuid.UUID{}},
		"на сторону":      {place("right"), historyPanel},
		"на панель":       {place("top", 0, 1, 0), historyPanel},
		"мимо интерфейса": {place("top", 5), historyPanel},
	} {
		before := erpInterface()
		value := before
		value.Top = append([]metadata.ClientInterfaceItem(nil), before.Top...)
		value.Top[0].Items = append([]metadata.ClientInterfaceItem(nil), before.Top[0].Items...)
		if _, err := DropClientInterfacePanel(&value, drop.at, drop.panel); err == nil {
			t.Errorf("%s: the drop was taken", name)
		}
		if len(value.Top[0].Items) != 2 || len(value.Right) != 0 {
			t.Errorf("%s: the refusal changed the interface: %+v %+v", name, value.Top, value.Right)
		}
	}
}
