package metadata

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// FormDisplayImportance is how important an element is when the form is
// fitted to a narrow screen (help, DisplayImportance).
type FormDisplayImportance string

const (
	FormDisplayImportanceAuto     FormDisplayImportance = "auto"
	FormDisplayImportanceVeryLow  FormDisplayImportance = "very-low"
	FormDisplayImportanceLow      FormDisplayImportance = "low"
	FormDisplayImportanceUsual    FormDisplayImportance = "usual"
	FormDisplayImportanceHigh     FormDisplayImportance = "high"
	FormDisplayImportanceVeryHigh FormDisplayImportance = "very-high"
)

// FormAttachedMenu is the context menu of an element and the automatic
// command bar of a table or of the form. Neither stands in the tree: each
// belongs to its element, which the prototype writes it inside of, and a
// form has one context menu for nearly every element (155317 in the exports)
// and one command bar for itself and each table. Code reaches both by name
// as elements, so the name is kept as written: the configurator does not
// rename them with their element (505 in the exports keep an older name, and
// 84 thousand are written in English), so it cannot be derived. Only the
// command bar of the form may have none (47 forms).
type FormAttachedMenu struct {
	ID   uuid.UUID `yaml:"id" json:"id"`
	Name string    `yaml:"name,omitempty" json:"name,omitempty"`
	// NoAutofill keeps the platform from filling the menu or the bar with
	// the standard commands; the prototype writes only that (1383 context
	// menus, 8116 command bars).
	NoAutofill bool `yaml:"no_autofill,omitempty" json:"noAutofill,omitempty"`
	// HorizontalAlign is where a command bar puts its buttons (help,
	// HorizontalAlign of a command bar - the prototype writes Auto too, which
	// the help forbids setting from code); DisplayImportance is how
	// important the command bar of the form is on a narrow screen (5 times).
	HorizontalAlign   ItemHorizontalAlign   `yaml:"horizontal_align,omitempty" json:"horizontalAlign,omitempty"`
	DisplayImportance FormDisplayImportance `yaml:"display_importance,omitempty" json:"displayImportance,omitempty"`
	// Children are the buttons, button groups and popups of the menu, and of
	// a command bar the search additions of its table too.
	Children []ManagedFormElement `yaml:"children,omitempty" json:"children,omitempty"`
}

// attachedMenus are the context menu and the command bar of an element, by
// the name they are written under.
func (element ManagedFormElement) attachedMenus() []struct {
	name string
	menu *FormAttachedMenu
} {
	return []struct {
		name string
		menu *FormAttachedMenu
	}{{"context_menu", element.ContextMenu}, {"auto_command_bar", element.AutoCommandBar}}
}

// Nested is every element an element holds: its children, the buttons of
// its context menu and command bar, the additions of a table, and its
// extended tooltip.
func (element ManagedFormElement) Nested() []ManagedFormElement {
	nested := element.Children
	for _, attached := range element.attachedMenus() {
		if attached.menu != nil && len(attached.menu.Children) != 0 {
			nested = append(append([]ManagedFormElement(nil), nested...), attached.menu.Children...)
		}
	}
	for _, held := range element.tableAdditions() {
		if held.addition != nil {
			nested = append(append([]ManagedFormElement(nil), nested...), *held.addition)
		}
	}
	if element.ExtendedTooltip != nil {
		nested = append(append([]ManagedFormElement(nil), nested...), *element.ExtendedTooltip)
	}
	return nested
}

// FormItems is every element of the form at its top level: what it holds,
// and the buttons of its command bar.
func (form ManagedForm) FormItems() []ManagedFormElement {
	if form.AutoCommandBar == nil || len(form.AutoCommandBar.Children) == 0 {
		return form.Items
	}
	return append(append([]ManagedFormElement(nil), form.Items...), form.AutoCommandBar.Children...)
}

// contextMenuHolder says an element of this class may have a context menu:
// the prototype gives one to every field, decoration, table and addition of a
// table, and to nothing else.
func contextMenuHolder(class formElementClass) bool {
	return class == formFieldClass || class == formDecorationClass || class == formTableClass || class == formAdditionClass
}

// validateAttachedMenu checks a context menu or a command bar apart from what
// it holds; commandBar says it is a command bar, and ofForm the one of the
// form.
func validateAttachedMenu(path string, menu FormAttachedMenu, commandBar, ofForm bool) []string {
	var issues []string
	if menu.ID.IsZero() {
		issues = append(issues, path+".id must be a non-zero UUID")
	}
	if menu.Name == "" && !ofForm || menu.Name != "" && (!validIdentifier(menu.Name) || utf8.RuneCountInString(menu.Name) > maxNameLength) {
		issues = append(issues, path+".name must be a valid identifier of at most 255 characters")
	}
	if !commandBar && (menu.HorizontalAlign != "" || menu.DisplayImportance != "") {
		issues = append(issues, path+" is a context menu: horizontal_align and display_importance are of a command bar")
	}
	issues = append(issues, oneOf(path+".horizontal_align", menu.HorizontalAlign, ItemHorizontalAuto, ItemHorizontalLeft, ItemHorizontalCenter, ItemHorizontalRight)...)
	issues = append(issues, oneOf(path+".display_importance", menu.DisplayImportance, FormDisplayImportanceAuto, FormDisplayImportanceVeryLow,
		FormDisplayImportanceLow, FormDisplayImportanceUsual, FormDisplayImportanceHigh, FormDisplayImportanceVeryHigh)...)
	return issues
}

// menuHolds says what a context menu or a command bar may hold: buttons,
// button groups and popups, and in a command bar the search additions of its
// table.
func menuHolds(commandBar bool, child FormElementKind) bool {
	if !commandBar && (child == FormElementSearchStringAddition || child == FormElementSearchControlAddition) {
		return false
	}
	return formClassHolds(formButtonsClass, child)
}

func (menu *FormAttachedMenu) clone() *FormAttachedMenu {
	if menu == nil {
		return nil
	}
	copied := *menu
	copied.Children = cloneRuntimeFormElements(menu.Children)
	return &copied
}

// attachedMenuPlace names where an element of a menu stands.
func attachedMenuPlace(path, name string, index int) string {
	return fmt.Sprintf("%s.%s.children[%d]", path, name, index)
}

// foldedName is how names of a form are compared.
func foldedName(name string) string { return strings.ToLower(name) }

// noteRepeatedNames notes every name the elements and context menus of a
// form share - see the shared names of ValidateManagedForm.
func (catalog *Catalog) noteRepeatedNames(where string, form ManagedForm) {
	counts, order := map[string]int{}, []string(nil)
	count := func(name string) {
		if name == "" {
			return
		}
		folded := foldedName(name)
		if counts[folded] == 0 {
			order = append(order, name)
		}
		counts[folded]++
	}
	var walk func(items []ManagedFormElement)
	walk = func(items []ManagedFormElement) {
		for _, item := range items {
			count(item.Name)
			for _, attached := range item.attachedMenus() {
				if attached.menu != nil {
					count(attached.menu.Name)
				}
			}
			walk(item.Nested())
		}
	}
	if form.AutoCommandBar != nil {
		count(form.AutoCommandBar.Name)
	}
	walk(form.FormItems())
	for _, name := range order {
		if repeated := counts[foldedName(name)]; repeated > 1 {
			catalog.noteForm(NoteRepeatedElementName, where+" element "+name, fmt.Sprintf("%d times", repeated))
		}
	}
}

// noteMobileCommandBar carries what the command bar of a form holds on a
// mobile device beyond a group or a button by name: an empty value
// (NoteMobileCommandBarEmpty), an element of another kind - an extended
// tooltip once in erp (NoteMobileCommandBarNotGroup) - and the code of an
// element (noteElementCode).
func (catalog *Catalog) noteMobileCommandBar(where string, form ManagedForm) {
	if len(form.MobileCommandBar) == 0 {
		return
	}
	kinds := map[string]FormElementKind{}
	name := func(name string, kind FormElementKind) {
		if _, seen := kinds[foldedName(name)]; name != "" && !seen {
			kinds[foldedName(name)] = kind
		}
	}
	var walk func(items []ManagedFormElement)
	walk = func(items []ManagedFormElement) {
		for _, item := range items {
			name(item.Name, item.Kind)
			if item.ContextMenu != nil {
				name(item.ContextMenu.Name, formContextMenu)
			}
			if item.AutoCommandBar != nil {
				name(item.AutoCommandBar.Name, FormElementCommandBar)
			}
			walk(item.Nested())
		}
	}
	if form.AutoCommandBar != nil {
		name(form.AutoCommandBar.Name, FormElementCommandBar)
	}
	walk(form.FormItems())
	for index, item := range form.MobileCommandBar {
		at := fmt.Sprintf("%s mobile_command_bar[%d]", where, index)
		switch kind := kinds[foldedName(item)]; {
		case item == "":
			catalog.noteForm(NoteMobileCommandBarEmpty, at, `""`)
		case formElementCode.MatchString(item):
			catalog.noteElementCode(at, item)
		case kind != FormElementButton && !slices.Contains(formGroupKinds, kind):
			catalog.noteForm(NoteMobileCommandBarNotGroup, at, item+" ("+string(kind)+")")
		}
	}
}

// formContextMenu is how a context menu is named where an element is told by
// its kind: it stands in no tree, and has no kind of its own.
const formContextMenu FormElementKind = "context-menu"
