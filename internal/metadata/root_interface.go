package metadata

import (
	"fmt"
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
}

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
	Top    []ClientInterfaceItem `yaml:"top,omitempty" json:"top,omitempty"`
	Left   []ClientInterfaceItem `yaml:"left,omitempty" json:"left,omitempty"`
	Right  []ClientInterfaceItem `yaml:"right,omitempty" json:"right,omitempty"`
	Bottom []ClientInterfaceItem `yaml:"bottom,omitempty" json:"bottom,omitempty"`
	Panels []uuid.UUID           `yaml:"panels,omitempty" json:"panels,omitempty"`
}

// ClientInterfaceItem is a panel or a group of them. A panel names the panel
// it shows and may have a height; a group holds items. The prototype gives an
// outer group and every panel an identifier of its own and leaves the groups
// inside without one.
type ClientInterfaceItem struct {
	ID     *uuid.UUID            `yaml:"id,omitempty" json:"id,omitempty"`
	Panel  *uuid.UUID            `yaml:"panel,omitempty" json:"panel,omitempty"`
	Height int                   `yaml:"height,omitempty" json:"height,omitempty"`
	Items  []ClientInterfaceItem `yaml:"items,omitempty" json:"items,omitempty"`
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
				if item.Panel.IsZero() {
					issues = append(issues, prefix+".panel must be a non-zero UUID")
				}
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
		if panel.IsZero() {
			issues = append(issues, fmt.Sprintf("panels[%d] must be a non-zero UUID", index))
		}
	}
	return issues
}

// loadRootInterface reads the three descriptions of the root's interface and
// checks that the support settings, when there, are a file. None of them is
// required: a configuration without a home page opens on an empty one.
func (catalog *Catalog) loadRootInterface(root string) error {
	read := func(file string, target any, validate func() []string) (bool, error) {
		handle, err := openRootFile(root, file)
		if handle == nil || err != nil {
			return false, err
		}
		defer handle.Close()
		if err := decodeStrict(file, handle, target); err != nil {
			return false, err
		}
		if issues := validate(); len(issues) > 0 {
			return false, fmt.Errorf("%s: %s", file, strings.Join(issues, "; "))
		}
		return true, nil
	}
	var page HomePage
	if found, err := read(project.HomePageFile, &page, func() []string { return validateHomePage(page) }); err != nil {
		return err
	} else if found {
		catalog.HomePage = &page
	}
	var panels ClientInterface
	if found, err := read(project.ClientInterfaceFile, &panels, func() []string { return validateClientInterface(panels) }); err != nil {
		return err
	} else if found {
		catalog.ClientInterface = &panels
	}
	var commands CommandInterface
	if _, err := read(project.MainSectionCommandInterfaceFile, &commands, func() []string {
		issues := validateCommandInterface("command_interface", commands)
		// The order of the sections is the root's own, on its description:
		// the main section is one of them, not their parent.
		if len(commands.SubsystemsOrder) > 0 {
			issues = append(issues, "subsystems_order lies on the configuration, not on its main section")
		}
		return issues
	}); err != nil {
		return err
	}
	catalog.MainSectionCommandInterface = commands
	// The file is carried opaque; only that it is a file is checked.
	_, err := rootFilePath(root, project.ParentConfigurationsFile)
	return err
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
		return "", fmt.Errorf("the configuration keeps %q, which must be a file", file)
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
	for id, kind := range catalog.objectKindByID {
		if kind == commonFormObjectKind {
			forms[id] = true
		}
	}
	for _, column := range catalog.HomePage.columns() {
		where := "the configuration's home page " + column.name
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
