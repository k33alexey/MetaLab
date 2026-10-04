package metadata

import (
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
// of the main window, the commands of the main section, and the vendor's
// support settings as written. Defect caught: none of the four had a place,
// and the home page of every configuration moved arrived empty.
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
panels: [b553047f-c9aa-4157-978d-448ecad24248, cbab57f2-a0f3-4f0a-89ea-4cb19570ab75]
`)
	writeText(t, root, "", project.MainSectionCommandInterfaceFile, `groups_order:
  - {standard: navigation-panel-important}
commands_visibility:
  - {written: "0", visibility: {common: true}}
`)
	writeText(t, root, "", project.ParentConfigurationsFile, "\x00\x01поддержка")
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

// A form, a role, a command or a group the configuration no longer has is
// carried and noted, never refused: the home pages of erp and sb name 1 and 6
// forms that are gone, the one of acc a role that is gone. Defect caught: the
// root's interface named nothing a note would ever find.
func TestRootInterfaceNotesWhatPointsAtNothing(t *testing.T) {
	t.Parallel()
	root := rootInterfaceProject(t)
	goneForm, goneRole, goneCommand := uuid.MustNew().String(), uuid.MustNew().String(), uuid.MustNew().String()
	writeText(t, root, "", project.HomePageFile, "template: one-column\nleft:\n"+
		"  - {form: "+goneForm+", visibility: {common: true, roles: [{role: "+goneRole+", visible: false}]}}\n")
	writeText(t, root, "", project.MainSectionCommandInterfaceFile, "commands_order:\n"+
		"  - {command: "+goneCommand+", group: {group: "+rootInterfaceGroup+"}}\n")
	catalog, err := Load(root)
	if err != nil {
		t.Fatalf("a home page naming what is gone was refused: %v", err)
	}
	noted := map[string]string{}
	for _, note := range catalog.Notes() {
		if note.Kind == NoteUnresolvedReference {
			noted[note.Written] = note.Where
		}
	}
	for id, where := range map[string]string{
		goneForm:           "the configuration's home page left form",
		goneRole:           "the configuration's home page left role",
		goneCommand:        "the configuration's main section command interface command",
		rootInterfaceGroup: "the configuration's main section command interface group",
	} {
		if noted[id] != where {
			t.Errorf("%s is not noted at %q: %v", id, where, noted)
		}
	}
}

// Each description is checked for what its format can hold. Defects caught:
// a template nobody offers, a right column on a page of one, a panel holding
// items, a height on a group, the order of the sections on the main section,
// a form with no identifier, a description or the support settings that are
// a folder.
func TestRootInterfaceIsCheckedForShape(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		file, content, refusal string
	}{
		"шаблон не из трёх": {project.HomePageFile, "template: three-columns\n",
			"template must be one-column, two-columns-equal-width or two-columns-variable-width"},
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
			"subsystems_order lies on the configuration, not on its main section"},
		"неизвестное поле": {project.ClientInterfaceFile, "middle: []\n", "field middle not found"},
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
