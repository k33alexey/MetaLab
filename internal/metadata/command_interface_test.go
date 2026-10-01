package metadata

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// commandInterfaceBody is a subsystem whose command interface uses every
// section and every way of naming a command and a group the prototype writes.
func commandInterfaceBody(id, catalog, role, group, child string) string {
	return `format: 1
id: ` + id + `
name: Продажи
title: {ru: Продажи}
command_interface:
  commands_visibility:
    - {command: ` + commonCommandID + `, visibility: {common: false, roles: [{role: ` + role + `, visible: true}]}}
    - {object: ` + catalog + `, standard: open-list, visibility: {common: true}}
    - {written: "0:dd2b2f65-200c-417f-9c69-e6ba676d4f32", visibility: {common: false}}
  commands_placement:
    - {command: ` + commonCommandID + `, group: {group: ` + group + `}, placement: auto}
    - {object: ` + catalog + `, standard: create, group: {standard: navigation-panel-important}}
  commands_order:
    - {object: ` + catalog + `, standard: open-list, group: {standard: navigation-panel-ordinary}}
    - {written: "100:f432e4ac-daca-405e-b3bc-88db5d1077c3", group: {written: 00000000-0000-0000-0000-000000000000}}
  groups_order:
    - {standard: navigation-panel-ordinary}
    - {group: ` + group + `}
  subsystems_order: [` + child + `]
`
}

// The command interface of a section is carried whole: every section of it
// survives reading and writing back, in its order, with the notation the
// prototype wrote kept as written.
//
// Defect caught: a section of the command interface dropped at decode or at
// encode (192/84/100 subsystems keep one), and the order of a list changed -
// for an order, a changed order is a lost value.
func TestCommandInterfaceIsCarriedWhole(t *testing.T) {
	t.Parallel()
	id, catalog, role, group, child := uuid.MustNew(), uuid.MustNew(), uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	body := commandInterfaceBody(id.String(), catalog.String(), role.String(), group.String(), child.String())
	value, err := DecodeSubsystem("subsystem.yaml", strings.NewReader(body), metadataConfiguration())
	if err != nil {
		t.Fatal(err)
	}
	ci := value.CommandInterface
	if len(ci.CommandsVisibility) != 3 || len(ci.CommandsPlacement) != 2 || len(ci.CommandsOrder) != 2 ||
		len(ci.GroupsOrder) != 2 || !slices.Equal(ci.SubsystemsOrder, []uuid.UUID{child}) {
		t.Fatalf("command interface = %+v", ci)
	}
	if first := ci.CommandsVisibility[0]; first.Common || len(first.Roles) != 1 || first.Roles[0].Role != role || !first.Roles[0].Visible {
		t.Fatalf("visibility by role = %+v", first.InterfaceVisibility)
	}
	if second := ci.CommandsVisibility[1]; *second.Object != catalog || second.Standard != StandardOpenList || !second.Common {
		t.Fatalf("standard command = %+v", second)
	}
	if ci.CommandsOrder[1].Written != "100:f432e4ac-daca-405e-b3bc-88db5d1077c3" ||
		ci.CommandsOrder[1].Group.Written != "00000000-0000-0000-0000-000000000000" {
		t.Fatalf("notation of the prototype not kept as written: %+v", ci.CommandsOrder[1])
	}
	var encoded bytes.Buffer
	if err := Encode(&encoded, value); err != nil {
		t.Fatal(err)
	}
	again, err := DecodeSubsystem("subsystem.yaml", &encoded, metadataConfiguration())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(again.CommandInterface, value.CommandInterface) {
		t.Fatalf("round trip changed the command interface:\n%+v\n%+v", again.CommandInterface, value.CommandInterface)
	}
	// What a caller is handed is its own copy.
	original := *value.CommandInterface.CommandsVisibility[0].Command
	copied := cloneSubsystemDefinition(value)
	*copied.CommandInterface.CommandsVisibility[0].Command = uuid.MustNew()
	copied.CommandInterface.CommandsVisibility[0].Roles[0].Visible = false
	if *value.CommandInterface.CommandsVisibility[0].Command != original || !value.CommandInterface.CommandsVisibility[0].Roles[0].Visible {
		t.Fatal("the copy shares the command interface with the original")
	}
}

// A command or a group is named exactly one way, from the sets the prototype
// uses, and what is carried as written is one clean line.
//
// Defect caught: a reference that names nothing, or two things at once,
// accepted - the command interface would then be ambiguous for block 18 - and
// values outside the prototype's sets accepted.
func TestCommandInterfaceRefusesWhatNamesNothingOrTwoThings(t *testing.T) {
	t.Parallel()
	one, two := uuid.MustNew(), uuid.MustNew()
	for name, value := range map[string]CommandInterface{
		"command named no way":         {CommandsOrder: []InterfaceCommandOrder{{Group: InterfaceGroup{Standard: ActionsPanelTools}}}},
		"command named two ways":       {CommandsOrder: []InterfaceCommandOrder{{InterfaceCommand: InterfaceCommand{Command: &one, Written: "0"}, Group: InterfaceGroup{Standard: ActionsPanelTools}}}},
		"standard without object":      {CommandsOrder: []InterfaceCommandOrder{{InterfaceCommand: InterfaceCommand{Standard: StandardOpen}, Group: InterfaceGroup{Standard: ActionsPanelTools}}}},
		"unknown standard":             {CommandsOrder: []InterfaceCommandOrder{{InterfaceCommand: InterfaceCommand{Object: &one, Standard: "print"}, Group: InterfaceGroup{Standard: ActionsPanelTools}}}},
		"group named no way":           {CommandsOrder: []InterfaceCommandOrder{{InterfaceCommand: InterfaceCommand{Command: &one}}}},
		"group named two ways":         {GroupsOrder: []InterfaceGroup{{Standard: ActionsPanelTools, Group: &two}}},
		"unknown group kind":           {GroupsOrder: []InterfaceGroup{{Standard: "form-command-bar"}}},
		"unknown placement":            {CommandsPlacement: []InterfaceCommandPlacement{{InterfaceCommand: InterfaceCommand{Command: &one}, Group: InterfaceGroup{Group: &two}, Placement: "always"}}},
		"group written with a newline": {GroupsOrder: []InterfaceGroup{{Written: "0:\n1"}}},
		"command written with a space": {CommandsOrder: []InterfaceCommandOrder{{InterfaceCommand: InterfaceCommand{Written: " 0"}, Group: InterfaceGroup{Standard: ActionsPanelTools}}}},
		"subsystem ordered twice":      {SubsystemsOrder: []uuid.UUID{one, one}},
		"role named twice": {CommandsVisibility: []InterfaceCommandVisibility{{InterfaceCommand: InterfaceCommand{Command: &one},
			InterfaceVisibility: project.InterfaceVisibility{Roles: []project.RoleVisibility{{Role: two}, {Role: two, Visible: true}}}}}},
	} {
		if issues := validateCommandInterface("command_interface", value); len(issues) == 0 {
			t.Errorf("%s: accepted", name)
		}
	}
}

// What the command interface names is resolved against the configuration, and
// what names nothing is carried and listed, not refused. The prototype's own
// notation is not a reference and is not listed.
//
// Defect caught: a stale command, object, group, role or section refusing the
// configuration - about a thousand references of the configurations being
// moved point at deleted things - and a stale reference carried without a
// trace; and the opposite, a live reference taken for a stale one.
func TestCommandInterfaceListsWhatPointsAtNothing(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	for _, directory := range []string{"subsystems", "roles"} {
		if err := os.MkdirAll(filepath.Join(root, "metadata", directory), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeCommonCommand(t, root, "Открыть", "format: 1\nid: "+commonCommandID+"\nname: Открыть\ntitle: {ru: Открыть}\n", true)
	writeCommandGroup(t, root, commandGroupID, "Цены")
	objectCommandID, otherID := uuid.MustNew(), uuid.MustNew()
	writeMetadata(t, root, CatalogKind, catalogID, "format: 1\nid: "+catalogID+"\nname: Товары\ntitle: {ru: Товары}\ncode: {type: string, length: 9}\ndescription_length: 100\n"+
		"commands:\n  - {id: "+objectCommandID.String()+", name: Подбор, title: {ru: Подбор}}\n")
	writeCommandModule(t, root, CatalogKind, "Товары", "Подбор")
	// A command of an object, not only a common one, is found by its identifier.
	writeMetadata(t, root, SubsystemKind, otherID.String(), "format: 1\nid: "+otherID.String()+"\nname: Закупки\ntitle: {ru: Закупки}\n"+
		"command_interface:\n  commands_visibility:\n    - {command: "+objectCommandID.String()+", visibility: {common: true}}\n")
	roleID, subsystemID, childID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	if err := os.WriteFile(filepath.Join(root, "metadata", "roles", roleID.String()+".yaml"),
		[]byte("format: 1\nid: "+roleID.String()+"\nname: Продавец\ntitle: {ru: Продавец}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeMetadata(t, root, SubsystemKind, childID.String(), "format: 1\nid: "+childID.String()+"\nname: Розница\ntitle: {ru: Розница}\nparent: "+subsystemID.String()+"\n")
	writeMetadata(t, root, SubsystemKind, subsystemID.String(), commandInterfaceBody(subsystemID.String(), catalogID, roleID.String(), commandGroupID, childID.String()))
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if unresolved := catalog.UnresolvedReferences(); len(unresolved) != 0 {
		t.Fatalf("live references taken for stale ones: %+v", unresolved)
	}

	staleCommand, staleObject, staleRole, staleGroup, staleChild := uuid.MustNew(), uuid.MustNew(), uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	writeMetadata(t, root, SubsystemKind, subsystemID.String(), `format: 1
id: `+subsystemID.String()+`
name: Продажи
title: {ru: Продажи}
command_interface:
  commands_visibility:
    - {command: `+staleCommand.String()+`, visibility: {common: false, roles: [{role: `+staleRole.String()+`, visible: true}]}}
  commands_order:
    - {object: `+staleObject.String()+`, standard: open-list, group: {group: `+staleGroup.String()+`}}
  subsystems_order: [`+staleChild.String()+`]
`)
	catalog, err = Load(root)
	if err != nil {
		t.Fatalf("stale references refused: %v", err)
	}
	var listed []uuid.UUID
	for _, item := range catalog.UnresolvedReferences() {
		listed = append(listed, item.ID)
	}
	want := []uuid.UUID{staleCommand, staleRole, staleObject, staleGroup, staleChild}
	if !slices.Equal(listed, want) {
		t.Fatalf("listed %v, want %v", listed, want)
	}
}

// The root's half of the command interface - the order and the visibility of
// the sections - lies on the root, is carried and is resolved the same way.
//
// Defect caught: the order of the sections lost (the root keeps it in all
// three configurations, 22/14/12 sections), a section ordered twice accepted,
// and a stale section or role refusing the configuration.
func TestRootOrdersAndHidesTheSections(t *testing.T) {
	t.Parallel()
	sales, purchases, role := uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	configuration := metadataConfiguration()
	configuration.SubsystemsOrder = []uuid.UUID{purchases, sales}
	configuration.SubsystemsVisibility = []project.SubsystemVisibility{{Subsystem: sales,
		InterfaceVisibility: project.InterfaceVisibility{Common: true, Roles: []project.RoleVisibility{{Role: role, Visible: false}}}}}
	if err := configuration.Validate(); err != nil {
		t.Fatal(err)
	}
	twice := configuration
	twice.SubsystemsOrder = []uuid.UUID{sales, sales}
	if err := twice.Validate(); err == nil || !strings.Contains(err.Error(), "subsystems_order[1]") {
		t.Fatalf("a section ordered twice: %v", err)
	}

	root := filepath.Join(t.TempDir(), "project")
	if err := project.Initialize(root, configuration); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "metadata", string(SubsystemKind)), 0o755); err != nil {
		t.Fatal(err)
	}
	writeMetadata(t, root, SubsystemKind, sales.String(), "format: 1\nid: "+sales.String()+"\nname: Продажи\ntitle: {ru: Продажи}\n")
	catalog, err := Load(root)
	if err != nil {
		t.Fatalf("stale section or role refused: %v", err)
	}
	if !slices.Equal(catalog.Project.SubsystemsOrder, []uuid.UUID{purchases, sales}) {
		t.Fatalf("order of the sections = %v", catalog.Project.SubsystemsOrder)
	}
	var listed []uuid.UUID
	for _, item := range catalog.UnresolvedReferences() {
		listed = append(listed, item.ID)
	}
	if !slices.Equal(listed, []uuid.UUID{purchases, role}) {
		t.Fatalf("listed %v, want the missing section and the missing role", listed)
	}
}
