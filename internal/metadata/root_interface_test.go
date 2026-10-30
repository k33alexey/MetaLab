package metadata

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

const (
	rootInterfaceForm  = "5b000000-0000-4000-8000-0000000000f1"
	rootInterfaceRole  = "5b000000-0000-4000-8000-0000000000f2"
	rootInterfaceGroup = "5b000000-0000-4000-8000-0000000000f3"
	// The page names a form of an object as well as a common one.
	rootInterfaceCatalog    = "5b000000-0000-4000-8000-0000000000f4"
	rootInterfaceObjectForm = "5b000000-0000-4000-8000-0000000000f5"
)

// rootInterfaceProject is a project with a common form and a role the home
// page can name.
func rootInterfaceProject(t *testing.T) string {
	t.Helper()
	root := metadataProject(t)
	writeCommonForm(t, root, "РабочийСтол", "format: 1\nid: "+rootInterfaceForm+"\nname: РабочийСтол\ntitle: {ru: Рабочий стол}\nkind: common\n")
	writeMetadata(t, root, RoleKind, rootInterfaceRole, "format: 1\nid: "+rootInterfaceRole+"\nname: Менеджер\ntitle: {ru: Менеджер}\n")
	writeMetadata(t, root, CatalogKind, rootInterfaceCatalog, "format: 1\nid: "+rootInterfaceCatalog+"\nname: Заметки\ntitle: {ru: Заметки}\n"+
		"code: {type: string, length: 9}\ndescription_length: 150\n")
	writeObjectForm(t, root, CatalogKind, "Заметки", "МоиЗаметки", rootInterfaceObjectForm)
	return root
}

// The root carries what the application shows before anything is opened:
// the forms of the home page with their heights and who sees them, the panels
// of the main window and the commands of the main section. Defect caught: none
// of the three had a place, and the home page of every configuration moved
// arrived empty; the representation of the panel of sections the export
// writes (spr) had no place, and a panel added and not chosen - the zero
// identifier the export writes - was refused (opis of 29.10.2026).
//
// The vendor's support settings are not carried (owner, 15.10.2026): a file of
// their name at the project root is nothing the load reads, so even a folder
// standing there is no error.
func TestRootCarriesItsInterface(t *testing.T) {
	t.Parallel()
	root := rootInterfaceProject(t)
	writeText(t, root, "", project.HomePageFile, `template: two-columns-variable-width
left:
  - {form: `+rootInterfaceForm+`, height: 10, visibility: {common: false, roles: [{role: `+rootInterfaceRole+`, visible: true}]}}
right:
  - {form: `+rootInterfaceObjectForm+`, height: 100, visibility: {common: true}}
`)
	writeText(t, root, "", project.ClientInterfaceFile, `top:
  - id: 1b4042b7-6a75-4b3a-a7cd-e5306844686c
    items:
      - items:
          - {id: 1c6905f0-bef8-4e56-958c-25c457268b87, panel: 8e10648b-f52d-4ec2-b4dd-87de33778d95}
left:
  - {id: 49451a1e-9d1c-420b-b1e9-dd0e9c09e93b, panel: b553047f-c9aa-4157-978d-448ecad24248, height: 1}
bottom:
  - {id: bf5d77c2-c159-4afc-9f96-de953b83dfd6, panel: not-chosen}
  - {id: c0e6bbc8-b074-4d27-a123-9fc84db93826, panel: 00000000-0000-0000-0000-000000000000}
panels:
  - {id: b553047f-c9aa-4157-978d-448ecad24248, representation: picture-on-top-and-text}
  - {id: cbab57f2-a0f3-4f0a-89ea-4cb19570ab75}
`)
	writeText(t, root, "", project.MainSectionCommandInterfaceFile, `groups_order:
  - {standard: navigation-panel-important}
commands_visibility:
  - {written: "0", visibility: {common: true}}
`)
	if err := os.MkdirAll(filepath.Join(root, "РодительскиеКонфигурации.bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	catalog, err := Load(root)
	if err != nil {
		t.Fatalf("the root's interface was refused: %v", err)
	}
	page := catalog.HomePage
	switch {
	case page == nil || page.Template != TwoColumnsVariableWidthHomePage:
		t.Fatalf("the home page was lost: %+v", page)
	case len(page.Left) != 1 || page.Left[0].Height != 10 || page.Left[0].Visibility.Common ||
		len(page.Left[0].Visibility.Roles) != 1 || !page.Left[0].Visibility.Roles[0].Visible:
		t.Fatalf("a form of the left column lost what it said: %+v", page.Left)
	case len(page.Right) != 1 || page.Right[0].Height != 100:
		t.Fatalf("the right column was lost: %+v", page.Right)
	}
	panels := catalog.ClientInterface
	switch {
	case panels == nil || len(panels.Top) != 1 || len(panels.Top[0].Items) != 1 || len(panels.Top[0].Items[0].Items) != 1:
		t.Fatalf("the groups of the top were lost: %+v", panels)
	case panels.Top[0].Items[0].Items[0].Panel == nil || panels.Top[0].Items[0].Items[0].Panel.String() != "8e10648b-f52d-4ec2-b4dd-87de33778d95":
		t.Fatalf("the panel inside the groups was lost: %+v", panels.Top)
	case len(panels.Left) != 1 || panels.Left[0].Height != 1 || len(panels.Panels) != 2:
		t.Fatalf("the left side or the panels defined were lost: %+v", panels)
	case len(panels.Bottom) != 2 || panels.Bottom[0].Panel == nil || panels.Bottom[0].Panel.Chosen() || panels.Bottom[1].Panel == nil || panels.Bottom[1].Panel.Chosen():
		t.Fatalf("the panel not chosen was lost or taken for a group: %+v", panels.Bottom)
	case panels.Panels[0].PanelRepresentation != "picture-on-top-and-text" || panels.Panels[1].PanelRepresentation != "":
		t.Fatalf("the representation of the panel of sections was lost: %+v", panels.Panels)
	}
	commands := catalog.MainSectionCommandInterface
	if len(commands.GroupsOrder) != 1 || len(commands.CommandsVisibility) != 1 {
		t.Fatalf("the main section's command interface was lost: %+v", commands)
	}
	for _, note := range catalog.Notes() {
		if strings.Contains(note.Where, "home page") || strings.Contains(note.Where, "main section") {
			t.Errorf("a reference that resolves is noted: %+v", note)
		}
	}
}

// A form, a role, a command or a group the configuration no longer has is an
// error of the project: the home pages of erp and sb name 1 and 6 forms that
// are gone, the one of acc a role that is gone, and the import does not carry
// them. The strict load refuses the project naming the place; the reading for
// editing opens it and lists the reference to be fixed (owner, 05.10.2026). Defect caught: the
// root's interface naming nothing the check would ever find.
func TestRootInterfaceNamesWhatPointsAtNothing(t *testing.T) {
	t.Parallel()
	root := rootInterfaceProject(t)
	goneForm, goneRole, goneCommand := uuid.MustNew().String(), uuid.MustNew().String(), uuid.MustNew().String()
	writeText(t, root, "", project.HomePageFile, "template: one-column\nleft:\n"+
		"  - {form: "+goneForm+", visibility: {common: true, roles: [{role: "+goneRole+", visible: false}]}}\n")
	writeText(t, root, "", project.MainSectionCommandInterfaceFile, "commands_order:\n"+
		"  - {command: "+goneCommand+", group: {group: "+rootInterfaceGroup+"}}\n")
	noted := map[string]string{}
	for _, item := range unresolvedOf(t, root) {
		noted[item.ID.String()] = item.Where
	}
	for id, where := range map[string]string{
		goneForm:           "the project's home page left form",
		goneRole:           "the project's home page left role",
		goneCommand:        "the project's main section command interface command",
		rootInterfaceGroup: "the project's main section command interface group",
	} {
		if noted[id] != where {
			t.Errorf("%s is not noted at %q: %v", id, where, noted)
		}
	}
}

// Each description is checked for what its format can hold. Defects caught:
// a template nobody offers, a right column on a page of one, a panel holding
// items, a height on a group, the order of the sections on the main section,
// a form with no identifier, a description that is a folder, a representation
// the help does not list or on a panel other than that of sections, a panel
// defined with no identifier.
func TestRootInterfaceIsCheckedForShape(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		file, content, refusal string
	}{
		"шаблон не из трёх": {project.HomePageFile, "template: three-columns\n",
			"template must be one-column, two-columns-equal-width or two-columns-variable-width"},
		"чужое место командного интерфейса": {project.HomePageFile, "template: one-column\nmain_section_commands: left\n",
			"main_section_commands must be top, bottom or do-not-use"},
		"правая колонка у одной": {project.HomePageFile, "template: one-column\nright:\n  - {form: " + rootInterfaceForm + ", visibility: {common: true}}\n",
			"right belongs to a page of two columns"},
		"форма без идентификатора": {project.HomePageFile, "template: one-column\nleft:\n  - {form: 00000000-0000-0000-0000-000000000000, visibility: {common: true}}\n",
			"left[0].form must be a non-zero UUID"},
		"роль дважды": {project.HomePageFile, "template: one-column\nleft:\n  - {form: " + rootInterfaceForm + ", visibility: {common: true, roles: [{role: " +
			rootInterfaceRole + ", visible: true}, {role: " + rootInterfaceRole + ", visible: false}]}}\n", "role must be unique"},
		"панель с содержимым": {project.ClientInterfaceFile, "top:\n  - {panel: " + rootInterfaceForm + ", items: [{panel: " + rootInterfaceRole + "}]}\n",
			"top[0] is a panel and holds no items"},
		"высота у группы": {project.ClientInterfaceFile, "left:\n  - {height: 2, items: [{panel: " + rootInterfaceRole + "}]}\n",
			"left[0].height belongs to a panel"},
		"порядок разделов у основного раздела": {project.MainSectionCommandInterfaceFile, "subsystems_order: [" + rootInterfaceForm + "]\n",
			"subsystems_order lies on the project, not on its main section"},
		"неизвестное поле": {project.ClientInterfaceFile, "middle: []\n", "field middle not found"},
		"отображение не из пяти": {project.ClientInterfaceFile, "panels:\n  - {id: b553047f-c9aa-4157-978d-448ecad24248, representation: picture-under-text}\n",
			"panels[0].representation must be picture, picture-and-text, picture-on-top-and-text, picture-on-left-and-text or text"},
		"отображение не у панели разделов": {project.ClientInterfaceFile, "panels:\n  - {id: cbab57f2-a0f3-4f0a-89ea-4cb19570ab75, representation: text}\n",
			"panels[0].representation belongs to the panel of sections"},
		"определение панели без идентификатора": {project.ClientInterfaceFile, "panels:\n  - {representation: text}\n",
			"panels[0].id must be a non-zero UUID"},
		"панель дважды на разных сторонах": {project.ClientInterfaceFile, "top:\n  - {panel: b553047f-c9aa-4157-978d-448ecad24248}\n" +
			"left:\n  - {items: [{items: [{panel: b553047f-c9aa-4157-978d-448ecad24248}]}]}\n",
			"left[0].items[0].items[0].panel b553047f-c9aa-4157-978d-448ecad24248 stands at top[0] already"},
		"панель дважды в группе": {project.ClientInterfaceFile, "top:\n  - {panel: not-chosen}\n  - {panel: cbab57f2-a0f3-4f0a-89ea-4cb19570ab75}\n" +
			"  - {panel: cbab57f2-a0f3-4f0a-89ea-4cb19570ab75}\n", "top[2].panel cbab57f2-a0f3-4f0a-89ea-4cb19570ab75 stands at top[1] already"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := rootInterfaceProject(t)
			writeText(t, root, "", test.file, test.content)
			_, err := Load(root)
			if err == nil || !strings.Contains(err.Error(), test.refusal) || !strings.Contains(err.Error(), test.file) {
				t.Fatalf("refused for another reason or not at all: %v", err)
			}
		})
	}
	for _, file := range project.RootDescriptionFiles() {
		t.Run("папка вместо "+file, func(t *testing.T) {
			t.Parallel()
			root := rootInterfaceProject(t)
			if err := os.MkdirAll(filepath.Join(root, file), 0o755); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(root); err == nil || !strings.Contains(err.Error(), "which must be a file") {
				t.Fatalf("refused for another reason or not at all: %v", err)
			}
		})
	}
}

// TestTheHomePageKeepsWhereItShowsTheMainSection catches the place of the
// command interface of the main section on the home page lost or read into
// another: the window of the work area gives «Сверху», «Снизу» and «Не
// использовать», the last written by leaving it out
// (docs/block-00/konfiguraciya.md).
func TestTheHomePageKeepsWhereItShowsTheMainSection(t *testing.T) {
	t.Parallel()
	for written, want := range map[string]MainSectionCommandsPlace{
		"":                                    "",
		"main_section_commands: top\n":        "top",
		"main_section_commands: bottom\n":     "bottom",
		"main_section_commands: do-not-use\n": "do-not-use",
	} {
		root := rootInterfaceProject(t)
		writeText(t, root, "", project.HomePageFile, "template: two-columns-equal-width\n"+written)
		catalog, err := Load(root)
		if err != nil {
			t.Fatalf("%q refused: %v", written, err)
		}
		if catalog.HomePage == nil || catalog.HomePage.MainSectionCommands != want {
			t.Errorf("%q read as %+v, want %q", written, catalog.HomePage, want)
		}
	}
}

// A configuration without them opens on an empty page: none of the four is
// required. Defect caught: a missing description refused the project.
func TestRootInterfaceIsOptional(t *testing.T) {
	t.Parallel()
	catalog, err := Load(rootInterfaceProject(t))
	if err != nil {
		t.Fatal(err)
	}
	if catalog.HomePage != nil || catalog.ClientInterface != nil || !catalog.MainSectionCommandInterface.IsEmpty() {
		t.Fatalf("a root without its interface came back with one: %+v %+v", catalog.HomePage, catalog.ClientInterface)
	}
}

// A panel not chosen is written as the project keeps it and read back a
// panel not chosen. Defect caught (found by 2.650): the zero identifier,
// which uuid.UUID neither writes to JSON nor keeps in YAML (it says IsZero
// and omitempty leaves it out), turned the panel into an empty group on the
// first write and failed every answer of Studio carrying it.
func TestAPanelNotChosenIsWrittenAndReadBack(t *testing.T) {
	t.Parallel()
	id := uuid.MustNew()
	value := ClientInterface{Bottom: []ClientInterfaceItem{{ID: &id, Panel: &PanelID{}}}}
	var written bytes.Buffer
	if err := Encode(&written, value); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(written.String(), "panel: not-chosen") {
		t.Fatalf("the panel not chosen is written as %q", written.String())
	}
	read, err := DecodeRootDescription(project.ClientInterfaceFile, &written)
	if err != nil {
		t.Fatal(err)
	}
	if bottom := read.(ClientInterface).Bottom; len(bottom) != 1 || bottom[0].Panel == nil || bottom[0].Panel.Chosen() {
		t.Fatalf("the panel not chosen came back as %+v", bottom)
	}
	answer, err := json.Marshal(value)
	if err != nil || !strings.Contains(string(answer), `"panel":"not-chosen"`) {
		t.Fatalf("the panel not chosen is not answered: %s %v", answer, err)
	}
}

// Panels not chosen may stand anywhere, as many as there are: the window
// holds them while it is edited and the application shows none
// (dosyomka-2649/12, the configurator's check passes two). Defect caught:
// the refusal of a panel of the platform twice taking the panels not chosen
// for one panel.
func TestPanelsNotChosenStandAsManyAsThereAre(t *testing.T) {
	t.Parallel()
	root := rootInterfaceProject(t)
	writeText(t, root, "", project.ClientInterfaceFile, "bottom:\n  - {panel: not-chosen}\n  - {panel: not-chosen}\n"+
		"  - {items: [{items: [{panel: not-chosen}]}]}\n")
	if _, err := Load(root); err != nil {
		t.Fatalf("the panels not chosen were refused: %v", err)
	}
}

// The default interface is the prototype's window of a configuration that
// never wrote one (dosyomka-2649/01): at the top the panel of sections and
// that of the functions of the current section on the side itself, each of
// an identifier of its own, the other sides empty, the five panels of the
// platform defined; the definitions of a file are completed in the order of
// the export (panelDef), keeping the representation and a panel the platform
// does not list. Defects caught: the default empty or in a group, panels with
// no identifier, a definition dropped or its representation lost, the
// definitions out of the export's order.
func TestTheDefaultClientInterfaceAndItsDefinitions(t *testing.T) {
	t.Parallel()
	value := DefaultClientInterface()
	if len(value.Top) != 2 || len(value.Left)+len(value.Right)+len(value.Bottom) != 0 {
		t.Fatalf("the default is %+v", value)
	}
	for index, want := range []string{"b553047f-c9aa-4157-978d-448ecad24248", "b2735bd3-d822-4430-ba59-c9e869693b24"} {
		item := value.Top[index]
		if item.Panel == nil || item.Panel.String() != want || item.ID == nil || item.ID.IsZero() || len(item.Items) != 0 {
			t.Errorf("the top[%d] of the default is %+v", index, item)
		}
	}
	if issues := validateClientInterface(value); len(issues) != 0 {
		t.Errorf("the default is refused: %v", issues)
	}
	unknown := uuid.MustParse("8e10648b-f52d-4ec2-b4dd-87de33778d95")
	file := ClientInterface{Panels: []ClientInterfacePanel{{ID: unknown}, {ID: CurrentSectionFunctionsPanel}, {ID: SectionsPanel, PanelRepresentation: SectionsPanelText}}}
	file.DefineStandardPanels()
	var defined []string
	for _, panel := range file.Panels {
		defined = append(defined, panel.ID.String()+":"+string(panel.PanelRepresentation))
	}
	want := []string{"b553047f-c9aa-4157-978d-448ecad24248:text", "13322b22-3960-4d68-93a6-fe2dd7f28ca3:", "c933ac92-92cd-459d-81cc-e0c8a83ced99:",
		"cbab57f2-a0f3-4f0a-89ea-4cb19570ab75:", "b2735bd3-d822-4430-ba59-c9e869693b24:", "8e10648b-f52d-4ec2-b4dd-87de33778d95:"}
	if strings.Join(defined, " ") != strings.Join(want, " ") || len(DefaultClientInterface().Panels) != 5 {
		t.Errorf("the panels defined are %v, want %v", defined, want)
	}
}
