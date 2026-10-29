package metadata

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// What the application shows before anything is opened is described by the
// root in three files beside its description, as the prototype keeps three
// beside its own: the forms of the home page, the panels of the main window,
// the commands of the main section. The help types all three as Undefined -
// the language cannot read them - and the configurator keeps them all the
// same; the configurations being moved have each (erp, acc, sb; the home page
// in brayval too). They are carried, not yet acted upon: the interface of the
// application is built in block 18.

// HomePageTemplate is how the forms of the home page are laid out - the three
// templates the editor of the home page offers.
type HomePageTemplate string

const (
	OneColumnHomePage               HomePageTemplate = "one-column"
	TwoColumnsEqualWidthHomePage    HomePageTemplate = "two-columns-equal-width"
	TwoColumnsVariableWidthHomePage HomePageTemplate = "two-columns-variable-width"
)

// HomePage is the forms the application opens on its home page, column by
// column, in the order they stand. A page of one column keeps its forms in
// Left: the prototype writes that one column as a column of its own, and a
// right column it does not have cannot be written at all.
type HomePage struct {
	Template HomePageTemplate `yaml:"template" json:"template"`
	Left     []HomePageForm   `yaml:"left,omitempty" json:"left,omitempty"`
	Right    []HomePageForm   `yaml:"right,omitempty" json:"right,omitempty"`
	// MainSectionCommands is where the page shows the command interface of
	// the main section: «Сверху», «Снизу» or «Не использовать» in the
	// window of the work area (opis of 15.10.2026,
	// docs/block-00/konfiguraciya.md). The export writes it as
	// MACommandInterfaceDisplays (mdclasses: Bottom) and leaves it out at
	// «Не использовать», which empty is too.
	MainSectionCommands MainSectionCommandsPlace `yaml:"main_section_commands,omitempty" json:"mainSectionCommands,omitempty"`
}

// MainSectionCommandsPlace is where the home page shows the command
// interface of the main section.
type MainSectionCommandsPlace string

const (
	MainSectionCommandsTop      MainSectionCommandsPlace = "top"
	MainSectionCommandsBottom   MainSectionCommandsPlace = "bottom"
	MainSectionCommandsNotShown MainSectionCommandsPlace = "do-not-use"
)

// HomePageForm is one form of the home page: the form, its height in lines
// relative to the others, and who sees it. A form is named by its identifier,
// and one the configuration no longer has is carried and noted: the prototype
// keeps them (erp 1, sb 6), written by the identifier alone.
type HomePageForm struct {
	Form       uuid.UUID                   `yaml:"form" json:"form"`
	Height     int                         `yaml:"height,omitempty" json:"height,omitempty"`
	Visibility project.InterfaceVisibility `yaml:"visibility" json:"visibility"`
}

// ClientInterface is the panels of the main window and where they stand: at
// each side of the window, panels and the groups holding them, and the panels
// the configuration defines. Which panel a panel item shows is the identifier
// of a panel of the platform - the panel of sections, of open windows, of the
// functions of the current section.
type ClientInterface struct {
	Top    []ClientInterfaceItem  `yaml:"top,omitempty" json:"top,omitempty"`
	Left   []ClientInterfaceItem  `yaml:"left,omitempty" json:"left,omitempty"`
	Right  []ClientInterfaceItem  `yaml:"right,omitempty" json:"right,omitempty"`
	Bottom []ClientInterfaceItem  `yaml:"bottom,omitempty" json:"bottom,omitempty"`
	Panels []ClientInterfacePanel `yaml:"panels,omitempty" json:"panels,omitempty"`
}

// ClientInterfaceItem is a panel or a group of them. A panel names the panel
// it shows and may have a height; a group holds items. The prototype gives an
// outer group and every panel an identifier of its own and leaves the groups
// inside without one.
type ClientInterfaceItem struct {
	ID     *uuid.UUID            `yaml:"id,omitempty" json:"id,omitempty"`
	Panel  *PanelID              `yaml:"panel,omitempty" json:"panel,omitempty"`
	Height int                   `yaml:"height,omitempty" json:"height,omitempty"`
	Items  []ClientInterfaceItem `yaml:"items,omitempty" json:"items,omitempty"`
}

// PanelID is the panel of the platform an element of the client interface
// shows. The zero one is a panel added and not yet chosen («Стандартная
// панель: Не выбрана» in the window of the interface): the export writes it
// as the zero identifier (opis of 29.10.2026, docs/block-00/konfiguraciya.md),
// the project as not-chosen - a zero uuid.UUID is written by nothing, and
// one that says IsZero is left out of the file. Read, the zero identifier is
// taken as not-chosen too.
type PanelID uuid.UUID

const notChosenPanel = "not-chosen"

// Chosen answers whether the panel names a panel of the platform.
func (id PanelID) Chosen() bool { return !uuid.UUID(id).IsZero() }

func (id PanelID) String() string {
	if !id.Chosen() {
		return notChosenPanel
	}
	return uuid.UUID(id).String()
}

// MarshalText implements encoding.TextMarshaler.
func (id PanelID) MarshalText() ([]byte, error) { return []byte(id.String()), nil }

// UnmarshalText implements encoding.TextUnmarshaler.
func (id *PanelID) UnmarshalText(data []byte) error {
	if string(data) == notChosenPanel {
		*id = PanelID{}
		return nil
	}
	parsed, err := uuid.Parse(string(data))
	if err != nil {
		return err
	}
	*id = PanelID(parsed)
	return nil
}

// ClientInterfacePanel is a panel the configuration defines, with how it is
// shown: «Описание панели» in the window of the interface gives the panel of
// sections its «Отображение», and no other panel (opis of 27.10.2026). The
// export writes it on the panel's definition (panelDef, spr) and leaves it
// out at «Картинка и текст», which empty is too.
type ClientInterfacePanel struct {
	ID                  uuid.UUID                   `yaml:"id" json:"id"`
	PanelRepresentation SectionsPanelRepresentation `yaml:"representation,omitempty" json:"representation,omitempty"`
}

// SectionsPanelRepresentation is how the panel of sections shows a section:
// the five values of ОтображениеПанелиРазделов in the help, the five the
// window of the interface offers.
type SectionsPanelRepresentation string

const (
	SectionsPanelPicture              SectionsPanelRepresentation = "picture"
	SectionsPanelPictureAndText       SectionsPanelRepresentation = "picture-and-text"
	SectionsPanelPictureOnTopAndText  SectionsPanelRepresentation = "picture-on-top-and-text"
	SectionsPanelPictureOnLeftAndText SectionsPanelRepresentation = "picture-on-left-and-text"
	SectionsPanelText                 SectionsPanelRepresentation = "text"
)

// SectionsPanel is the identifier of the platform's panel of sections, the one
// panel with a representation (export after the edit of 29.10.2026, the
// panel's name in «Описание панели» - ПанельРазделов).
var SectionsPanel = uuid.MustParse("b553047f-c9aa-4157-978d-448ecad24248")

// StandardPanel is a panel of the platform an element of the client
// interface shows: its identifier, its name («Описание панели», «Имя») and
// its caption in the window of the interface.
type StandardPanel struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Title string    `json:"title"`
}

// StandardPanels are the panels of the platform in the order of the list of
// the window and of panelDef in every export (snapshot konfiguraciya/23;
// names by «Описание панели» and the export after the edit of 29.10.2026,
// docs/block-00/konfiguraciya.md). The panel 8e10648b… the exports place
// and define is not one of them: 8.3.27 lists it nowhere and shows its row
// empty.
var StandardPanels = []StandardPanel{
	{SectionsPanel, "ПанельРазделов", "Панель разделов"},
	{uuid.MustParse("13322b22-3960-4d68-93a6-fe2dd7f28ca3"), "ПанельИзбранного", "Панель избранного"},
	{uuid.MustParse("c933ac92-92cd-459d-81cc-e0c8a83ced99"), "ПанельИстории", "Панель истории"},
	{uuid.MustParse("cbab57f2-a0f3-4f0a-89ea-4cb19570ab75"), "ПанельОткрытых", "Панель открытых"},
	{uuid.MustParse("b2735bd3-d822-4430-ba59-c9e869693b24"), "ПанельФункцийТекущегоРаздела", "Панель функций текущего раздела"},
}

type homePageColumn struct {
	name  string
	forms []HomePageForm
}

// columns is the columns of the page in the order they stand.
func (value HomePage) columns() []homePageColumn {
	return []homePageColumn{{"left", value.Left}, {"right", value.Right}}
}

func validateHomePage(value HomePage) []string {
	var issues []string
	switch value.Template {
	case OneColumnHomePage:
		if len(value.Right) > 0 {
			issues = append(issues, "right belongs to a page of two columns")
		}
	case TwoColumnsEqualWidthHomePage, TwoColumnsVariableWidthHomePage:
	default:
		issues = append(issues, "template must be one-column, two-columns-equal-width or two-columns-variable-width")
	}
	switch value.MainSectionCommands {
	case "", MainSectionCommandsTop, MainSectionCommandsBottom, MainSectionCommandsNotShown:
	default:
		issues = append(issues, "main_section_commands must be top, bottom or do-not-use")
	}
	for _, column := range value.columns() {
		for index, form := range column.forms {
			prefix := fmt.Sprintf("%s[%d]", column.name, index)
			if form.Form.IsZero() {
				issues = append(issues, prefix+".form must be a non-zero UUID")
			}
			issues = append(issues, project.ValidateInterfaceVisibility(prefix+".visibility", form.Visibility)...)
		}
	}
	return issues
}

func validateClientInterface(value ClientInterface) []string {
	var issues []string
	var walk func(path string, items []ClientInterfaceItem)
	walk = func(path string, items []ClientInterfaceItem) {
		for index, item := range items {
			prefix := fmt.Sprintf("%s[%d]", path, index)
			if item.ID != nil && item.ID.IsZero() {
				issues = append(issues, prefix+".id must be a non-zero UUID")
			}
			if item.Panel != nil {
				if len(item.Items) > 0 {
					issues = append(issues, prefix+" is a panel and holds no items")
				}
				continue
			}
			if item.Height != 0 {
				issues = append(issues, prefix+".height belongs to a panel")
			}
			walk(prefix+".items", item.Items)
		}
	}
	walk("top", value.Top)
	walk("left", value.Left)
	walk("right", value.Right)
	walk("bottom", value.Bottom)
	for index, panel := range value.Panels {
		prefix := fmt.Sprintf("panels[%d]", index)
		if panel.ID.IsZero() {
			issues = append(issues, prefix+".id must be a non-zero UUID")
		}
		switch panel.PanelRepresentation {
		case "":
		case SectionsPanelPicture, SectionsPanelPictureAndText, SectionsPanelPictureOnTopAndText,
			SectionsPanelPictureOnLeftAndText, SectionsPanelText:
			if panel.ID != SectionsPanel {
				issues = append(issues, prefix+".representation belongs to the panel of sections")
			}
		default:
			issues = append(issues, prefix+".representation must be picture, picture-and-text, "+
				"picture-on-top-and-text, picture-on-left-and-text or text")
		}
	}
	return issues
}

// DecodeRootDescription reads one of the three descriptions of the root's
// interface (project.RootDescriptionFiles) by its file and checks it by its
// shape: a HomePage, a ClientInterface or the CommandInterface of the main
// section. It is how the loader reads them and how Studio checks one it
// writes.
func DecodeRootDescription(file string, reader io.Reader) (any, error) {
	switch file {
	case project.HomePageFile:
		return decodeChecked(file, reader, validateHomePage)
	case project.ClientInterfaceFile:
		return decodeChecked(file, reader, validateClientInterface)
	case project.MainSectionCommandInterfaceFile:
		return decodeChecked(file, reader, func(commands CommandInterface) []string {
			issues := validateCommandInterface("command_interface", commands)
			// The order of the sections is the root's own, on its
			// description: the main section is one of them, not their
			// parent.
			if len(commands.SubsystemsOrder) > 0 {
				issues = append(issues, "subsystems_order lies on the project, not on its main section")
			}
			return issues
		})
	}
	return nil, fmt.Errorf("%s is no description of the project's interface", file)
}

// decodeChecked reads a value of file strictly and refuses it with what
// issues finds in it.
func decodeChecked[T any](file string, reader io.Reader, issues func(T) []string) (any, error) {
	var value T
	if err := decodeStrict(file, reader, &value); err != nil {
		return nil, err
	}
	if found := issues(value); len(found) > 0 {
		return nil, fmt.Errorf("%s: %s", file, strings.Join(found, "; "))
	}
	return value, nil
}

// loadRootInterface reads the three descriptions of the root's interface. None
// of them is required: a configuration without a home page opens on an empty
// one.
func (catalog *Catalog) loadRootInterface(root string) error {
	read := func(file string) (any, error) {
		handle, err := openRootFile(root, file)
		if handle == nil || err != nil {
			return nil, err
		}
		defer handle.Close()
		return DecodeRootDescription(file, handle)
	}
	if value, err := read(project.HomePageFile); err != nil {
		return err
	} else if page, ok := value.(HomePage); ok {
		catalog.HomePage = &page
	}
	if value, err := read(project.ClientInterfaceFile); err != nil {
		return err
	} else if panels, ok := value.(ClientInterface); ok {
		catalog.ClientInterface = &panels
	}
	value, err := read(project.MainSectionCommandInterfaceFile)
	if err != nil {
		return err
	}
	commands, _ := value.(CommandInterface)
	catalog.MainSectionCommandInterface = commands
	return nil
}

// openRootFile opens a file of the root that may be missing, and refuses one
// that is a folder or a link. A missing file answers nil and no error.
func openRootFile(root, file string) (*os.File, error) {
	path, err := rootFilePath(root, file)
	if path == "" || err != nil {
		return nil, err
	}
	return os.Open(path)
}

// rootFilePath answers where a file of the root lies, "" when it is not there,
// and an error when what lies there is not a plain file.
func rootFilePath(root, file string) (string, error) {
	if root == "" {
		return "", nil
	}
	path := filepath.Join(root, file)
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Mode()&fs.ModeSymlink != 0 {
		return "", fmt.Errorf("the project keeps %q, which must be a file", file)
	}
	return path, nil
}

// resolveRootInterface notes every form and role of the home page that points
// at nothing, the way the command interface notes its own: nothing is
// refused, because the prototype keeps such lines and draws nothing from them.
func (catalog *Catalog) resolveRootInterface() {
	if catalog.HomePage == nil {
		return
	}
	forms := map[uuid.UUID]bool{}
	for _, objects := range catalog.objectForms {
		for _, object := range objects {
			for _, form := range object.forms {
				forms[form.id] = true
			}
		}
	}
	for id, object := range catalog.objects {
		if object.kind == CommonFormKind {
			forms[id] = true
		}
	}
	for _, column := range catalog.HomePage.columns() {
		where := "the project's home page " + column.name
		for _, item := range column.forms {
			if !forms[item.Form] {
				catalog.noteUnresolved(where+" form", item.Form)
			}
			if catalog.rolesLoaded {
				for _, role := range item.Visibility.Roles {
					if _, ok := catalog.roleByID[role.Role]; !ok {
						catalog.noteUnresolved(where+" role", role.Role)
					}
				}
			}
		}
	}
}
