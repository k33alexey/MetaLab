package metadata

import (
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// FormElementKind is what an element of a form is. Each kind is one tag of
// the prototype's description of a form, so that a form is moved element by
// element (owner, 06.10.2026); the kinds are the ones the help gives a form
// field, group, decoration and button (FormFieldType, FormGroupType,
// FormDecorationType, FormButtonType), a table, and the additions of a table
// that stand where the form puts them.
//
// The four managed forms of the exports being moved - 8726 forms - write
// their elements with these tags and no other. The period, PDF document,
// geographical schema and dendrogram fields the help gives are in none of
// them; they are kinds all the same.
//
// The context menu, the extended tooltip and the automatic command bar are
// not here: each belongs to the element it stands with, not to the tree.
type FormElementKind string

const (
	FormElementInputField               FormElementKind = "input-field"
	FormElementLabelField               FormElementKind = "label-field"
	FormElementCheckBoxField            FormElementKind = "check-box-field"
	FormElementRadioButtonField         FormElementKind = "radio-button-field"
	FormElementPictureField             FormElementKind = "picture-field"
	FormElementSpreadsheetDocumentField FormElementKind = "spreadsheet-document-field"
	FormElementTextDocumentField        FormElementKind = "text-document-field"
	FormElementHTMLDocumentField        FormElementKind = "html-document-field"
	FormElementFormattedDocumentField   FormElementKind = "formatted-document-field"
	FormElementChartField               FormElementKind = "chart-field"
	FormElementCalendarField            FormElementKind = "calendar-field"
	FormElementGanttChartField          FormElementKind = "gantt-chart-field"
	FormElementPlannerField             FormElementKind = "planner-field"
	FormElementGraphicalSchemaField     FormElementKind = "graphical-schema-field"
	FormElementProgressBarField         FormElementKind = "progress-bar-field"
	FormElementTrackBarField            FormElementKind = "track-bar-field"
	FormElementPeriodField              FormElementKind = "period-field"
	FormElementPDFDocumentField         FormElementKind = "pdf-document-field"
	FormElementGeographicalSchemaField  FormElementKind = "geographical-schema-field"
	FormElementDendrogramField          FormElementKind = "dendrogram-field"

	FormElementUsualGroup  FormElementKind = "usual-group"
	FormElementPages       FormElementKind = "pages"
	FormElementPage        FormElementKind = "page"
	FormElementColumnGroup FormElementKind = "column-group"
	FormElementButtonGroup FormElementKind = "button-group"
	FormElementPopup       FormElementKind = "popup"
	FormElementCommandBar  FormElementKind = "command-bar"

	FormElementLabelDecoration   FormElementKind = "label-decoration"
	FormElementPictureDecoration FormElementKind = "picture-decoration"

	FormElementButton FormElementKind = "button"
	FormElementTable  FormElementKind = "table"

	FormElementSearchStringAddition  FormElementKind = "search-string-addition"
	FormElementSearchControlAddition FormElementKind = "search-control-addition"
	FormElementViewStatusAddition    FormElementKind = "view-status-addition"
)

// formElementClass is what an element is beside its kind: what it may hold
// and where it may stand follow from it.
type formElementClass int

const (
	formFieldClass formElementClass = iota + 1
	// formAreaClass lays out what a form holds: a usual group and a page.
	formAreaClass
	formPagesClass
	formColumnsClass
	// formButtonsClass holds buttons: a command bar, a button group, a popup.
	formButtonsClass
	formDecorationClass
	formButtonClass
	formTableClass
	formAdditionClass
)

var formElementClasses = map[FormElementKind]formElementClass{
	FormElementInputField: formFieldClass, FormElementLabelField: formFieldClass, FormElementCheckBoxField: formFieldClass,
	FormElementRadioButtonField: formFieldClass, FormElementPictureField: formFieldClass, FormElementSpreadsheetDocumentField: formFieldClass,
	FormElementTextDocumentField: formFieldClass, FormElementHTMLDocumentField: formFieldClass, FormElementFormattedDocumentField: formFieldClass,
	FormElementChartField: formFieldClass, FormElementCalendarField: formFieldClass, FormElementGanttChartField: formFieldClass,
	FormElementPlannerField: formFieldClass, FormElementGraphicalSchemaField: formFieldClass, FormElementProgressBarField: formFieldClass,
	FormElementTrackBarField: formFieldClass, FormElementPeriodField: formFieldClass, FormElementPDFDocumentField: formFieldClass,
	FormElementGeographicalSchemaField: formFieldClass, FormElementDendrogramField: formFieldClass,
	FormElementUsualGroup: formAreaClass, FormElementPage: formAreaClass, FormElementPages: formPagesClass,
	FormElementColumnGroup: formColumnsClass,
	FormElementButtonGroup: formButtonsClass, FormElementPopup: formButtonsClass, FormElementCommandBar: formButtonsClass,
	FormElementLabelDecoration: formDecorationClass, FormElementPictureDecoration: formDecorationClass,
	FormElementButton: formButtonClass, FormElementTable: formTableClass,
	FormElementSearchStringAddition: formAdditionClass, FormElementSearchControlAddition: formAdditionClass,
	FormElementViewStatusAddition: formAdditionClass,
}

// IsField says the element shows data: one of the twenty fields.
func (kind FormElementKind) IsField() bool { return formElementClasses[kind] == formFieldClass }

// Holds says what an element of this kind may hold, as the forms being moved
// hold it: a usual group and a page - and the form itself - what a form lays
// out; pages only pages; a table and a group of columns fields and groups of
// columns; a command bar, a button group and a popup buttons, button groups,
// popups and the search additions of a table. A field, a decoration, a button
// and an addition hold nothing.
func (kind FormElementKind) Holds(child FormElementKind) bool {
	return formClassHolds(formElementClasses[kind], child)
}

func formClassHolds(class formElementClass, child FormElementKind) bool {
	held := formElementClasses[child]
	switch class {
	case formAreaClass:
		switch held {
		case formFieldClass, formAreaClass, formPagesClass, formButtonsClass, formDecorationClass, formButtonClass, formTableClass, formAdditionClass:
			// A page stands only in pages.
			return child != FormElementPage
		}
	case formPagesClass:
		return child == FormElementPage
	case formColumnsClass, formTableClass:
		return held == formFieldClass || child == FormElementColumnGroup
	case formButtonsClass:
		return held == formButtonClass || held == formButtonsClass && child != FormElementCommandBar ||
			child == FormElementSearchStringAddition || child == FormElementSearchControlAddition
	}
	return false
}

// FormButtonType is what a button is (help, FormButtonType). The prototype
// writes it on nearly every button; empty is not written.
type FormButtonType string

const (
	FormButtonUsual               FormButtonType = "usual-button"
	FormButtonHyperlink           FormButtonType = "hyperlink"
	FormButtonCommandBarButton    FormButtonType = "command-bar-button"
	FormButtonCommandBarHyperlink FormButtonType = "command-bar-hyperlink"
)

// FormToolTipRepresentation is how an element shows its tooltip (help,
// ToolTipRepresentation). Empty is Auto, which the prototype never writes.
type FormToolTipRepresentation string

const (
	FormToolTipAuto       FormToolTipRepresentation = "auto"
	FormToolTipNone       FormToolTipRepresentation = "none"
	FormToolTipButton     FormToolTipRepresentation = "button"
	FormToolTipBalloon    FormToolTipRepresentation = "balloon"
	FormToolTipShowAuto   FormToolTipRepresentation = "show-auto"
	FormToolTipShowTop    FormToolTipRepresentation = "show-top"
	FormToolTipShowLeft   FormToolTipRepresentation = "show-left"
	FormToolTipShowBottom FormToolTipRepresentation = "show-bottom"
	FormToolTipShowRight  FormToolTipRepresentation = "show-right"
)

// validateElementCommon checks what every element has: its tooltip, how the
// tooltip shows, whom it is shown to and how important it is on a narrow
// screen.
func validateElementCommon(path string, item ManagedFormElement, class formElementClass, configuration project.Project) []string {
	var issues []string
	if len(item.ToolTip) != 0 {
		// A button shows the tooltip of its command (help: a form button has
		// no tooltip of its own).
		if class == formButtonClass {
			issues = append(issues, path+".tool_tip is the tooltip of the command for a button")
		}
		issues = append(issues, validateTitle(path+".tool_tip", item.ToolTip, configuration)...)
	}
	issues = append(issues, oneOf(path+".tool_tip_representation", item.ToolTipRepresentation, FormToolTipAuto, FormToolTipNone, FormToolTipButton,
		FormToolTipBalloon, FormToolTipShowAuto, FormToolTipShowTop, FormToolTipShowLeft, FormToolTipShowBottom, FormToolTipShowRight)...)
	issues = append(issues, validateFormRight(path+".user_visible", item.UserVisible)...)
	issues = append(issues, oneOf(path+".display_importance", item.DisplayImportance, FormDisplayImportanceAuto, FormDisplayImportanceVeryLow,
		FormDisplayImportanceLow, FormDisplayImportanceUsual, FormDisplayImportanceHigh, FormDisplayImportanceVeryHigh)...)
	return issues
}

// FormTitleLocation is where the title of a field stands (help,
// FormItemTitleLocation). Empty is Auto, which the prototype never writes;
// None turns the title off, while an empty title is chosen by the platform.
type FormTitleLocation string

const (
	FormTitleAuto   FormTitleLocation = "auto"
	FormTitleNone   FormTitleLocation = "none"
	FormTitleLeft   FormTitleLocation = "left"
	FormTitleRight  FormTitleLocation = "right"
	FormTitleTop    FormTitleLocation = "top"
	FormTitleBottom FormTitleLocation = "bottom"
)

// FormEditMode is how a field that is a column of a table is edited (help,
// ColumnEditMode). Empty is Enter, which the prototype never writes.
type FormEditMode string

const (
	FormEditEnter        FormEditMode = "enter"
	FormEditEnterOnInput FormEditMode = "enter-on-input"
	FormEditDirectly     FormEditMode = "directly"
)

// FormWarningOnEdit is whether a field warns before it is edited (help,
// WarningOnEditRepresentation). Empty is Auto.
type FormWarningOnEdit string

const (
	FormWarningOnEditAuto     FormWarningOnEdit = "auto"
	FormWarningOnEditShow     FormWarningOnEdit = "show"
	FormWarningOnEditDontShow FormWarningOnEdit = "dont-show"
)

// FieldBehavior is what a field has besides what every element has: how it is
// titled, entered and edited (help, FormField).
type FieldBehavior struct {
	TitleLocation FormTitleLocation `yaml:"title_location,omitempty" json:"titleLocation,omitempty"`
	// SkipOnInput is yes, no or not said: the help gives it Undefined as the
	// third, which then follows the warning on edit, and the prototype writes
	// both true (1298) and false (32).
	SkipOnInput *bool `yaml:"skip_on_input,omitempty" json:"skipOnInput,omitempty"`
	// DefaultItem makes the field the one the form, page or table activates
	// first.
	DefaultItem bool         `yaml:"default_item,omitempty" json:"defaultItem,omitempty"`
	EditMode    FormEditMode `yaml:"edit_mode,omitempty" json:"editMode,omitempty"`
	// WarningOnEdit is the warning shown before editing when its
	// representation says show; the prototype writes it in languages.
	WarningOnEdit               LocalizedText     `yaml:"warning_on_edit,omitempty" json:"warningOnEdit,omitempty"`
	WarningOnEditRepresentation FormWarningOnEdit `yaml:"warning_on_edit_representation,omitempty" json:"warningOnEditRepresentation,omitempty"`
	// Shortcut is the key that puts the focus on the field, carried as the
	// prototype writes it ("Cmd+Shift+F", "Num +"), as the shortcut of a
	// command is.
	Shortcut string `yaml:"shortcut,omitempty" json:"shortcut,omitempty"`
}

func (field FieldBehavior) empty() bool {
	return field.TitleLocation == "" && field.SkipOnInput == nil && !field.DefaultItem && field.EditMode == "" &&
		len(field.WarningOnEdit) == 0 && field.WarningOnEditRepresentation == "" && field.Shortcut == ""
}

func validateFormField(path string, field FieldBehavior, class formElementClass, kind FormElementKind, configuration project.Project) []string {
	if field.empty() {
		return nil
	}
	if class != formFieldClass && !outsideFields(field, kind).empty() {
		return []string{path + " has what only a field has: title_location, skip_on_input, default_item, edit_mode, warning_on_edit, warning_on_edit_representation, shortcut"}
	}
	var issues []string
	issues = append(issues, oneOf(path+".title_location", field.TitleLocation, FormTitleAuto, FormTitleNone, FormTitleLeft, FormTitleRight, FormTitleTop, FormTitleBottom)...)
	issues = append(issues, oneOf(path+".edit_mode", field.EditMode, FormEditEnter, FormEditEnterOnInput, FormEditDirectly)...)
	issues = append(issues, oneOf(path+".warning_on_edit_representation", field.WarningOnEditRepresentation, FormWarningOnEditAuto, FormWarningOnEditShow, FormWarningOnEditDontShow)...)
	issues = append(issues, validateTitle(path+".warning_on_edit", field.WarningOnEdit, configuration)...)
	if strings.TrimSpace(field.Shortcut) != field.Shortcut {
		issues = append(issues, path+".shortcut must be written without surrounding spaces")
	}
	return issues
}

// validateElementDataPath checks the data path of an element as the
// prototype writes one: names separated by dots, a name followed by an index
// where it is an item of a collection ("ОбъектПрототип[0].Владелец", 290
// times), and a leading "~" (372 times) carried as written, as on the data an
// attribute passes to the client. Two other writings are carried as they
// are: a number, which is no attribute of the form (5 times, and 6 of a
// footer), and two paths joined by "~" (4 times); what they mean is not
// known. A code of several segments (formPathCode, 139 times) is a path that
// leads nowhere and is a remnant (notePathCode).
func validateElementDataPath(path, value string) []string {
	if value == "" || strings.TrimSpace(value) != value {
		return []string{path + " must be a data path without surrounding spaces"}
	}
	if allDigits(value) || formPathCode.MatchString(value) {
		return nil
	}
	trimmed := strings.TrimPrefix(value, "~")
	parts := []string{trimmed}
	if strings.HasPrefix(value, "~") && strings.Contains(trimmed, "~") {
		parts = strings.Split(trimmed, "~")
	}
	for _, part := range parts {
		if !elementDataPath(part) {
			return []string{path + " must be names separated by dots, each with an index if any"}
		}
	}
	return nil
}

func elementDataPath(value string) bool {
	for _, segment := range strings.Split(value, ".") {
		name, index, indexed := strings.Cut(segment, "[")
		for indexed {
			var number string
			var closed bool
			number, index, closed = strings.Cut(index, "]")
			if !closed || !allDigits(number) {
				return false
			}
			if index == "" {
				break
			}
			if !strings.HasPrefix(index, "[") {
				return false
			}
			index = index[1:]
		}
		if !validIdentifier(name) || utf8.RuneCountInString(name) > maxNameLength {
			return false
		}
	}
	return true
}

func allDigits(value string) bool {
	for _, symbol := range value {
		if symbol < '0' || symbol > '9' {
			return false
		}
	}
	return value != ""
}

// formGroupKinds are the groups of a form: a command bar is one too (help,
// FormGroupType), and has what every group has.
var formGroupKinds = []FormElementKind{FormElementUsualGroup, FormElementPages, FormElementPage, FormElementColumnGroup,
	FormElementPopup, FormElementButtonGroup, FormElementCommandBar}

var (
	button               = []FormElementKind{FormElementButton}
	tableKind            = []FormElementKind{FormElementTable}
	buttonAndTable       = []FormElementKind{FormElementButton, FormElementTable}
	groupsAndTable       = append([]FormElementKind{FormElementTable}, formGroupKinds...)
	groupsButtonAndTable = append([]FormElementKind{FormElementButton, FormElementTable}, formGroupKinds...)
)

// fieldPropertyElsewhere lists, by the name a property of a field is written
// under, the kinds of element other than fields that hold it too. The help
// gives every group its size, its stretching, its place in its group, its
// shortcut and the font and colour of its title (FormGroup); the extension of
// each group gives the rest. The prototype writes each on these kinds, and
// writes no other property of a field on a group. A button has its size and
// its limits, its place, its colours, font and border colour, the height of
// its title, its shortcut, whether it is skipped on input and activated
// first (help, FormButton; the prototype writes all but the shortcut). A
// table has all a button has, and where its title stands (help, FormTable).
//
// Two of them mean something else on a group than on a field, under the same
// tag: horizontal_align is where a usual group or a page puts what it holds
// (help, ChildItemsHorizontalAlign) and a command bar its buttons (help,
// HorizontalAlign of a command bar - the prototype writes HorizontalLocation,
// Auto included), not where text stands in a column, and vertical_align where
// a usual group or a page puts what it holds up and down.
var fieldPropertyElsewhere = map[string][]FormElementKind{
	"width": withDecorations(withAdditions(groupsButtonAndTable)), "height": withDecorations(groupsButtonAndTable),
	"horizontal_stretch": withDecorations(withAdditions(groupsButtonAndTable)), "vertical_stretch": withDecorations(groupsButtonAndTable),
	"group_horizontal_align": withDecorations(withAdditions(groupsButtonAndTable)), "group_vertical_align": withDecorations(groupsButtonAndTable),
	"shortcut":   withDecorations(groupsButtonAndTable),
	"title_font": groupsAndTable, "title_text_color": groupsAndTable,
	"horizontal_align":  {FormElementUsualGroup, FormElementPage, FormElementCommandBar, FormElementViewStatusAddition, FormElementLabelDecoration},
	"vertical_align":    {FormElementUsualGroup, FormElementPage, FormElementLabelDecoration},
	"back_color":        {FormElementUsualGroup, FormElementPage, FormElementPopup, FormElementButton, FormElementTable, FormElementLabelDecoration},
	"border_color":      withDecorations([]FormElementKind{FormElementPopup, FormElementButton, FormElementTable}),
	"border":            formDecorationKinds,
	"no_auto_max_width": withDecorations(withAdditions(buttonAndTable)), "max_width": withDecorations(withAdditions(buttonAndTable)),
	"no_auto_max_height": withDecorations(buttonAndTable), "max_height": withDecorations(buttonAndTable),
	"text_color": withDecorations(buttonAndTable), "font": withDecorations(buttonAndTable),
	"title_height":  append(slices.Clone(buttonAndTable), FormElementLabelDecoration),
	"skip_on_input": withDecorations(buttonAndTable),
	"default_item":  buttonAndTable, "title_location": tableKind,
	"title_back_color": {FormElementColumnGroup}, "header_picture": {FormElementColumnGroup},
	"header_horizontal_align": {FormElementColumnGroup}, "fixing_in_table": {FormElementColumnGroup},
}

// formAdditionKinds are the additions of a table. Each has its width, its
// limit and stretching across and its place in its group (help, the
// extension of each addition; the prototype writes the limit of the width on
// a search string too, which the help does not give it), and the view
// status where its text stands - HorizontalAlign, which the prototype writes
// HorizontalLocation, as on a command bar. The help gives them a font,
// colours, a border and the place up and down in the group besides, which
// the prototype writes on no addition of the exports; they are not carried.
var formAdditionKinds = []FormElementKind{FormElementSearchStringAddition, FormElementViewStatusAddition, FormElementSearchControlAddition}

func withAdditions(kinds []FormElementKind) []FormElementKind {
	return append(slices.Clone(kinds), formAdditionKinds...)
}

// formDecorationKinds are the label and the picture. Each has its size and
// limits, stretching, place in its group, font and colour of text, border
// and its colour, shortcut and skipping on input (help, FormDecoration and
// the extension of each); a label besides its background, where its text
// stands across and up and down, and the height of its title.
var formDecorationKinds = []FormElementKind{FormElementLabelDecoration, FormElementPictureDecoration}

func withDecorations(kinds []FormElementKind) []FormElementKind {
	return append(slices.Clone(kinds), formDecorationKinds...)
}

// outsideFields is what of a group of properties of a field an element of
// another kind does not hold: the properties it does hold are taken out, by
// the name they are written under.
func outsideFields[T any](value T, kind FormElementKind) T {
	fields := reflect.ValueOf(&value).Elem()
	for index := range fields.NumField() {
		name, _, _ := strings.Cut(fields.Type().Field(index).Tag.Get("yaml"), ",")
		if slices.Contains(fieldPropertyElsewhere[name], kind) {
			fields.Field(index).SetZero()
		}
	}
	return value
}

// FieldLayout is the size of a field and where it stands (help, FormField and
// the extension of each field). Sizes are in characters and take no limit,
// as the prototype sets none; 0 is chosen by the platform, and a maximum of 0
// is no maximum.
type FieldLayout struct {
	Width  int `yaml:"width,omitempty" json:"width,omitempty"`
	Height int `yaml:"height,omitempty" json:"height,omitempty"`
	// NoAutoMaxWidth and NoAutoMaxHeight turn off the platform's own limit,
	// so that MaxWidth and MaxHeight apply; the prototype writes only the
	// "off" (8463 and 305 times).
	NoAutoMaxWidth  bool `yaml:"no_auto_max_width,omitempty" json:"noAutoMaxWidth,omitempty"`
	MaxWidth        int  `yaml:"max_width,omitempty" json:"maxWidth,omitempty"`
	NoAutoMaxHeight bool `yaml:"no_auto_max_height,omitempty" json:"noAutoMaxHeight,omitempty"`
	MaxHeight       int  `yaml:"max_height,omitempty" json:"maxHeight,omitempty"`
	// HorizontalStretch and VerticalStretch are yes, no or not said: what a
	// field does when not told depends on its kind - the help gives an input
	// and a label field Undefined beside yes and no - and the prototype
	// writes both values (11612 and 1602 across, 1430 and 138 up and down).
	HorizontalStretch *bool `yaml:"horizontal_stretch,omitempty" json:"horizontalStretch,omitempty"`
	VerticalStretch   *bool `yaml:"vertical_stretch,omitempty" json:"verticalStretch,omitempty"`
	// GroupHorizontalAlign and GroupVerticalAlign are where the field stands in
	// its group; Auto takes the group's own (help, HorizontalAlignInGroup,
	// VerticalAlignInGroup - the prototype's description names them
	// GroupHorizontalAlign and GroupVerticalAlign).
	GroupHorizontalAlign ItemHorizontalAlign `yaml:"group_horizontal_align,omitempty" json:"groupHorizontalAlign,omitempty"`
	GroupVerticalAlign   ItemVerticalAlign   `yaml:"group_vertical_align,omitempty" json:"groupVerticalAlign,omitempty"`
	// HorizontalAlign is where the text stands in a column of a table;
	// VerticalAlign where the field stands up and down.
	HorizontalAlign ItemHorizontalAlign `yaml:"horizontal_align,omitempty" json:"horizontalAlign,omitempty"`
	VerticalAlign   ItemVerticalAlign   `yaml:"vertical_align,omitempty" json:"verticalAlign,omitempty"`
}

func validateFieldLayout(path string, layout FieldLayout, class formElementClass, kind FormElementKind) []string {
	if layout == (FieldLayout{}) {
		return nil
	}
	if class != formFieldClass && outsideFields(layout, kind) != (FieldLayout{}) {
		return []string{path + " has the size and alignment of a field"}
	}
	var issues []string
	for _, size := range []struct {
		name  string
		value int
	}{{"width", layout.Width}, {"height", layout.Height}, {"max_width", layout.MaxWidth}, {"max_height", layout.MaxHeight}} {
		if size.value < 0 {
			issues = append(issues, path+"."+size.name+" must not be negative")
		}
	}
	horizontal := []ItemHorizontalAlign{ItemHorizontalAuto, ItemHorizontalLeft, ItemHorizontalCenter, ItemHorizontalRight}
	vertical := []ItemVerticalAlign{ItemVerticalAuto, ItemVerticalTop, ItemVerticalCenter, ItemVerticalBottom}
	issues = append(issues, oneOf(path+".group_horizontal_align", layout.GroupHorizontalAlign, horizontal...)...)
	issues = append(issues, oneOf(path+".group_vertical_align", layout.GroupVerticalAlign, vertical...)...)
	issues = append(issues, oneOf(path+".horizontal_align", layout.HorizontalAlign, horizontal...)...)
	issues = append(issues, oneOf(path+".vertical_align", layout.VerticalAlign, vertical...)...)
	return issues
}

// FieldLook is how a field and its title are drawn: the colours, fonts and
// border are the values a style item has (styles.go), written as the
// prototype writes them on a field - a style item, a colour of the web or the
// system palette, an absolute colour; a font of a style item, of the system
// or absolute, with what it changes. Nil is the field's own.
type FieldLook struct {
	Font        *FontValue   `yaml:"font,omitempty" json:"font,omitempty"`
	TextColor   *ColorValue  `yaml:"text_color,omitempty" json:"textColor,omitempty"`
	BackColor   *ColorValue  `yaml:"back_color,omitempty" json:"backColor,omitempty"`
	BorderColor *ColorValue  `yaml:"border_color,omitempty" json:"borderColor,omitempty"`
	Border      *BorderValue `yaml:"border,omitempty" json:"border,omitempty"`
	// The title of the field: its font, its colours, and its height in lines
	// (1 to 16 in the exports; 0 is chosen by the platform).
	TitleFont      *FontValue  `yaml:"title_font,omitempty" json:"titleFont,omitempty"`
	TitleTextColor *ColorValue `yaml:"title_text_color,omitempty" json:"titleTextColor,omitempty"`
	TitleBackColor *ColorValue `yaml:"title_back_color,omitempty" json:"titleBackColor,omitempty"`
	TitleHeight    int         `yaml:"title_height,omitempty" json:"titleHeight,omitempty"`
}

func validateFieldLook(path string, look FieldLook, class formElementClass, kind FormElementKind) []string {
	if look == (FieldLook{}) {
		return nil
	}
	if class != formFieldClass && outsideFields(look, kind) != (FieldLook{}) {
		return []string{path + " has the look of a field"}
	}
	var issues []string
	for _, font := range []struct {
		name  string
		value *FontValue
	}{{"font", look.Font}, {"title_font", look.TitleFont}} {
		if font.value != nil {
			issues = append(issues, validateFontValue(path+"."+font.name, *font.value)...)
		}
	}
	for _, color := range look.colors() {
		if color.value != nil {
			issues = append(issues, validateColorValue(path+"."+color.name, *color.value)...)
		}
	}
	if look.Border != nil {
		issues = append(issues, validateBorderValue(path+".border", *look.Border)...)
	}
	if look.TitleHeight < 0 {
		issues = append(issues, path+".title_height must not be negative")
	}
	return issues
}

type namedColor struct {
	name  string
	value *ColorValue
}

func (look FieldLook) colors() []namedColor {
	return []namedColor{{"text_color", look.TextColor}, {"back_color", look.BackColor}, {"border_color", look.BorderColor},
		{"title_text_color", look.TitleTextColor}, {"title_back_color", look.TitleBackColor}}
}

// styleItems lists the style items of the configuration the look takes its
// values from, each with the type it must be.
func (look FieldLook) styleItems() []styleItemUse {
	var uses []styleItemUse
	add := func(name string, itemType StyleItemType, reference *StyleItemReference) {
		if use, ok := styleItemUseOf(name, itemType, reference); ok {
			uses = append(uses, use)
		}
	}
	for _, font := range []struct {
		name  string
		value *FontValue
	}{{"font", look.Font}, {"title_font", look.TitleFont}} {
		if font.value != nil && font.value.Source == StyleFont {
			add(font.name, FontStyleItem, font.value.From)
		}
	}
	for _, color := range look.colors() {
		if color.value != nil && color.value.Source == StyleColor {
			add(color.name, ColorStyleItem, color.value.From)
		}
	}
	if look.Border != nil && look.Border.Source == StyleBorder {
		add("border", BorderStyleItem, look.Border.From)
	}
	return uses
}

type styleItemUse struct {
	name     string
	itemType StyleItemType
	id       uuid.UUID
	// written is a reference written as a number, which is noted and not
	// resolved.
	written string
}

// FormFixingInTable is whether a column of a table stays in place as the
// table scrolls across, and at which edge (help, FixingInTable). Empty is
// None, which the prototype never writes.
type FormFixingInTable string

const (
	FormFixingNone  FormFixingInTable = "none"
	FormFixingLeft  FormFixingInTable = "left"
	FormFixingRight FormFixingInTable = "right"
)

// FieldColumn is what a field has as a column of a table: its header and
// footer, whether it stays in place, and how its cells are shown (help,
// FormField). The help says each has a meaning only in a table; the
// prototype writes some of them on fields that stand elsewhere (the
// alignment in the footer 479 times, whether it shows in the header 18, the
// height of a cell 18), and they are carried there as written.
type FieldColumn struct {
	// HiddenInHeader and HiddenInFooter take the column out of the header and
	// the footer of the table; shown is the default, and the prototype writes
	// only the "not shown" (3129 and 1007 times).
	HiddenInHeader bool `yaml:"hidden_in_header,omitempty" json:"hiddenInHeader,omitempty"`
	HiddenInFooter bool `yaml:"hidden_in_footer,omitempty" json:"hiddenInFooter,omitempty"`
	// HeaderPicture and FooterPicture are drawn in the header and the footer
	// of the column.
	HeaderPicture *PictureReference `yaml:"header_picture,omitempty" json:"headerPicture,omitempty"`
	FooterPicture *PictureReference `yaml:"footer_picture,omitempty" json:"footerPicture,omitempty"`
	// HeaderHorizontalAlign is where the title stands in the header. The help
	// forbids Auto there, and the prototype writes it once all the same; it is
	// carried as written.
	HeaderHorizontalAlign ItemHorizontalAlign `yaml:"header_horizontal_align,omitempty" json:"headerHorizontalAlign,omitempty"`
	// The footer shows FooterText, or the attribute FooterDataPath names -
	// most often a total of a column of a tabular section.
	FooterText            LocalizedText       `yaml:"footer_text,omitempty" json:"footerText,omitempty"`
	FooterDataPath        string              `yaml:"footer_data_path,omitempty" json:"footerDataPath,omitempty"`
	FooterHorizontalAlign ItemHorizontalAlign `yaml:"footer_horizontal_align,omitempty" json:"footerHorizontalAlign,omitempty"`
	// The font and colours of the footer are values of a style item, as the
	// look of the field is.
	FooterFont      *FontValue        `yaml:"footer_font,omitempty" json:"footerFont,omitempty"`
	FooterTextColor *ColorValue       `yaml:"footer_text_color,omitempty" json:"footerTextColor,omitempty"`
	FooterBackColor *ColorValue       `yaml:"footer_back_color,omitempty" json:"footerBackColor,omitempty"`
	FixingInTable   FormFixingInTable `yaml:"fixing_in_table,omitempty" json:"fixingInTable,omitempty"`
	// CellHyperlink shows the text of the cells as a link, and choosing a
	// cell is pressing Enter; AutoCellHeight fits the height of a cell to
	// its text.
	CellHyperlink  bool `yaml:"cell_hyperlink,omitempty" json:"cellHyperlink,omitempty"`
	AutoCellHeight bool `yaml:"auto_cell_height,omitempty" json:"autoCellHeight,omitempty"`
}

func (column FieldColumn) empty() bool {
	return !column.HiddenInHeader && !column.HiddenInFooter && column.HeaderPicture == nil && column.FooterPicture == nil &&
		column.HeaderHorizontalAlign == "" && len(column.FooterText) == 0 && column.FooterDataPath == "" &&
		column.FooterHorizontalAlign == "" && column.FooterFont == nil && column.FooterTextColor == nil &&
		column.FooterBackColor == nil && column.FixingInTable == "" && !column.CellHyperlink && !column.AutoCellHeight
}

func validateFieldColumn(path string, column FieldColumn, class formElementClass, kind FormElementKind, configuration project.Project) []string {
	if column.empty() {
		return nil
	}
	if class != formFieldClass && !outsideFields(column, kind).empty() {
		return []string{path + " has what a field has as a column of a table"}
	}
	var issues []string
	for _, picture := range []struct {
		name  string
		value *PictureReference
	}{{"header_picture", column.HeaderPicture}, {"footer_picture", column.FooterPicture}} {
		issues = append(issues, validateElementPictureReference(path+"."+picture.name, picture.value)...)
	}
	horizontal := []ItemHorizontalAlign{ItemHorizontalAuto, ItemHorizontalLeft, ItemHorizontalCenter, ItemHorizontalRight}
	issues = append(issues, oneOf(path+".header_horizontal_align", column.HeaderHorizontalAlign, horizontal...)...)
	issues = append(issues, oneOf(path+".footer_horizontal_align", column.FooterHorizontalAlign, horizontal...)...)
	issues = append(issues, validateTitle(path+".footer_text", column.FooterText, configuration)...)
	if column.FooterDataPath != "" {
		issues = append(issues, validateElementDataPath(path+".footer_data_path", column.FooterDataPath)...)
	}
	if column.FooterFont != nil {
		issues = append(issues, validateFontValue(path+".footer_font", *column.FooterFont)...)
	}
	for _, color := range column.colors() {
		if color.value != nil {
			issues = append(issues, validateColorValue(path+"."+color.name, *color.value)...)
		}
	}
	issues = append(issues, oneOf(path+".fixing_in_table", column.FixingInTable, FormFixingNone, FormFixingLeft, FormFixingRight)...)
	return issues
}

func (column FieldColumn) colors() []namedColor {
	return []namedColor{{"footer_text_color", column.FooterTextColor}, {"footer_back_color", column.FooterBackColor}}
}

// styleItems lists the style items of the configuration the footer takes its
// font and colours from.
func (column FieldColumn) styleItems() []styleItemUse {
	var uses []styleItemUse
	if font := column.FooterFont; font != nil && font.Source == StyleFont {
		if use, ok := styleItemUseOf("footer_font", FontStyleItem, font.From); ok {
			uses = append(uses, use)
		}
	}
	for _, color := range column.colors() {
		if color.value != nil && color.value.Source == StyleColor {
			if use, ok := styleItemUseOf(color.name, ColorStyleItem, color.value.From); ok {
				uses = append(uses, use)
			}
		}
	}
	return uses
}

// styleItemUseOf is the use of a style item of the configuration, or of one
// written as a number, that a reference makes; a standard style item is no
// use of one.
func styleItemUseOf(name string, itemType StyleItemType, reference *StyleItemReference) (styleItemUse, bool) {
	switch {
	case reference == nil:
		return styleItemUse{}, false
	case reference.Item != nil:
		return styleItemUse{name: name, itemType: itemType, id: *reference.Item}, true
	case reference.Written != "":
		return styleItemUse{name: name, itemType: itemType, written: reference.Written}, true
	}
	return styleItemUse{}, false
}

type namedPicture struct {
	name  string
	value *PictureReference
}

// FormChoiceButtonRepresentation is where the choice button of an input field
// stands (help, ChoiceButtonRepresentation). Empty is Auto: in the drop list
// for a reference, in the field for any other type. The prototype never
// writes Auto.
type FormChoiceButtonRepresentation string

const (
	FormChoiceButtonAuto                          FormChoiceButtonRepresentation = "auto"
	FormChoiceButtonShowInInputField              FormChoiceButtonRepresentation = "show-in-input-field"
	FormChoiceButtonShowInDropList                FormChoiceButtonRepresentation = "show-in-drop-list"
	FormChoiceButtonShowInDropListAndInInputField FormChoiceButtonRepresentation = "show-in-drop-list-and-in-input-field"
)

// FormAutoShowButton is when the clear or the open button of an input field
// shows (help, AutoShowClearButtonMode and AutoShowOpenButtonMode). Empty is
// Auto, which the help reads as filled only.
type FormAutoShowButton string

const (
	FormAutoShowButtonAuto       FormAutoShowButton = "auto"
	FormAutoShowButtonAlways     FormAutoShowButton = "always"
	FormAutoShowButtonFilledOnly FormAutoShowButton = "filled-only"
)

// FieldButtons are the buttons of an input field (help, the extension of a
// form field for an input field). Each button is yes, no or not said: the
// help gives each Undefined as the third, chosen by the type edited, and the
// prototype writes both true and false (the choice button 3680 and 1317
// times). The prototype writes them on input fields only.
type FieldButtons struct {
	ChoiceButton     *bool `yaml:"choice_button,omitempty" json:"choiceButton,omitempty"`
	OpenButton       *bool `yaml:"open_button,omitempty" json:"openButton,omitempty"`
	ClearButton      *bool `yaml:"clear_button,omitempty" json:"clearButton,omitempty"`
	CreateButton     *bool `yaml:"create_button,omitempty" json:"createButton,omitempty"`
	DropListButton   *bool `yaml:"drop_list_button,omitempty" json:"dropListButton,omitempty"`
	SpinButton       *bool `yaml:"spin_button,omitempty" json:"spinButton,omitempty"`
	ChoiceListButton *bool `yaml:"choice_list_button,omitempty" json:"choiceListButton,omitempty"`
	// ChoiceButtonRepresentation is where the choice button stands, and
	// ChoiceButtonPicture is drawn on it - most often a standard picture,
	// a common one or a file of the element's own (once in the exports).
	ChoiceButtonRepresentation FormChoiceButtonRepresentation `yaml:"choice_button_representation,omitempty" json:"choiceButtonRepresentation,omitempty"`
	ChoiceButtonPicture        *PictureReference              `yaml:"choice_button_picture,omitempty" json:"choiceButtonPicture,omitempty"`
	// AutoShowClearButton and AutoShowOpenButton are when the clear and the
	// open button show; the prototype writes them as AutoShow…Mode.
	AutoShowClearButton FormAutoShowButton `yaml:"auto_show_clear_button,omitempty" json:"autoShowClearButton,omitempty"`
	AutoShowOpenButton  FormAutoShowButton `yaml:"auto_show_open_button,omitempty" json:"autoShowOpenButton,omitempty"`
}

func (buttons FieldButtons) empty() bool {
	return buttons.ChoiceButton == nil && buttons.OpenButton == nil && buttons.ClearButton == nil && buttons.CreateButton == nil &&
		buttons.DropListButton == nil && buttons.SpinButton == nil && buttons.ChoiceListButton == nil &&
		buttons.ChoiceButtonRepresentation == "" && buttons.ChoiceButtonPicture == nil &&
		buttons.AutoShowClearButton == "" && buttons.AutoShowOpenButton == ""
}

func validateFieldButtons(path string, buttons FieldButtons, kind FormElementKind) []string {
	if buttons.empty() {
		return nil
	}
	if kind != FormElementInputField {
		return []string{path + " has the buttons of an input field"}
	}
	var issues []string
	issues = append(issues, oneOf(path+".choice_button_representation", buttons.ChoiceButtonRepresentation,
		FormChoiceButtonAuto, FormChoiceButtonShowInInputField, FormChoiceButtonShowInDropList, FormChoiceButtonShowInDropListAndInInputField)...)
	issues = append(issues, validateElementPictureReference(path+".choice_button_picture", buttons.ChoiceButtonPicture)...)
	modes := []FormAutoShowButton{FormAutoShowButtonAuto, FormAutoShowButtonAlways, FormAutoShowButtonFilledOnly}
	issues = append(issues, oneOf(path+".auto_show_clear_button", buttons.AutoShowClearButton, modes...)...)
	issues = append(issues, oneOf(path+".auto_show_open_button", buttons.AutoShowOpenButton, modes...)...)
	return issues
}

// commonPictures lists the common pictures the element is drawn with.
func (element ManagedFormElement) commonPictures() []namedPicture {
	var pictures []namedPicture
	for _, picture := range element.pictures() {
		if picture.value != nil && picture.value.Common != nil {
			pictures = append(pictures, namedPicture(picture))
		}
	}
	return pictures
}

// FormEditTextUpdate is when the text being edited in an input field is
// updated (help, EditTextUpdate). Empty is Auto.
type FormEditTextUpdate string

const (
	FormEditTextUpdateAuto          FormEditTextUpdate = "auto"
	FormEditTextUpdateAlways        FormEditTextUpdate = "always"
	FormEditTextUpdateOnValueChange FormEditTextUpdate = "on-value-change"
	FormEditTextUpdateDontUse       FormEditTextUpdate = "dont-use"
)

// FormSpecialTextInputMode is the keyboard a mobile client offers for an
// input field (help, SpecialTextInputMode). Empty is Auto.
type FormSpecialTextInputMode string

const (
	FormSpecialTextInputAuto                 FormSpecialTextInputMode = "auto"
	FormSpecialTextInputNone                 FormSpecialTextInputMode = "none"
	FormSpecialTextInputDigits               FormSpecialTextInputMode = "digits"
	FormSpecialTextInputDigitsAndPunctuation FormSpecialTextInputMode = "digits-and-punctuation"
	FormSpecialTextInputEmail                FormSpecialTextInputMode = "email"
	FormSpecialTextInputPhoneNumber          FormSpecialTextInputMode = "phone-number"
	FormSpecialTextInputURL                  FormSpecialTextInputMode = "url"
)

// FormTextInputUse is whether spelling is checked or errors corrected as text
// is typed into an input field (help, SpellCheckingOnTextInput and
// AutoCorrectionOnTextInput). Empty is Auto.
type FormTextInputUse string

const (
	FormTextInputUseAuto    FormTextInputUse = "auto"
	FormTextInputUseUse     FormTextInputUse = "use"
	FormTextInputUseDontUse FormTextInputUse = "dont-use"
)

// FormHeightControlVariant is how the height of a multiline input field is
// governed (help, ItemHeightControlVariant). Empty is Auto.
type FormHeightControlVariant string

const (
	FormHeightControlAuto                FormHeightControlVariant = "auto"
	FormHeightControlUseContentHeight    FormHeightControlVariant = "use-content-height"
	FormHeightControlUseHeightInFormRows FormHeightControlVariant = "use-height-in-form-rows"
	// FormHeightControlUseHeightInTableRows is of a table only (help,
	// TableHeightControlVariant), whose height is governed so too.
	FormHeightControlUseHeightInTableRows FormHeightControlVariant = "use-height-in-table-rows"
)

// FieldTextInput is how text is typed into an input field (help, the
// extension of a form field for an input field). The prototype writes these
// on input fields only, the password mode on a label field too (4 times), as
// the help gives it there.
type FieldTextInput struct {
	// NoWrap turns off wrapping the lines, and NoTextEdit editing the text
	// (the buttons still work): both are on by default, and the prototype
	// writes only the "off" (6007 and 1600 times).
	NoWrap     bool `yaml:"no_wrap,omitempty" json:"noWrap,omitempty"`
	NoTextEdit bool `yaml:"no_text_edit,omitempty" json:"noTextEdit,omitempty"`
	// MultiLine and ExtendedEdit - Tab typed into the text, search in it -
	// are yes, no or not said: the help gives them Boolean, and the
	// prototype writes both true and false (1461 and 53, 296 and 30), so
	// what it leaves out is not one of the two.
	MultiLine    *bool `yaml:"multi_line,omitempty" json:"multiLine,omitempty"`
	ExtendedEdit *bool `yaml:"extended_edit,omitempty" json:"extendedEdit,omitempty"`
	// PasswordMode shows every character as a star; not said follows the
	// configuration, as the help gives Undefined.
	PasswordMode *bool `yaml:"password_mode,omitempty" json:"passwordMode,omitempty"`
	// Mask is the mask of the text typed, carried as written: its spaces and
	// special characters are the mask.
	Mask string `yaml:"mask,omitempty" json:"mask,omitempty"`
	// InputHint is shown in an empty field without the focus.
	InputHint            LocalizedText            `yaml:"input_hint,omitempty" json:"inputHint,omitempty"`
	EditTextUpdate       FormEditTextUpdate       `yaml:"edit_text_update,omitempty" json:"editTextUpdate,omitempty"`
	SpecialTextInputMode FormSpecialTextInputMode `yaml:"special_text_input_mode,omitempty" json:"specialTextInputMode,omitempty"`
	SpellChecking        FormTextInputUse         `yaml:"spell_checking,omitempty" json:"spellChecking,omitempty"`
	AutoCorrection       FormTextInputUse         `yaml:"auto_correction,omitempty" json:"autoCorrection,omitempty"`
	HeightControlVariant FormHeightControlVariant `yaml:"height_control_variant,omitempty" json:"heightControlVariant,omitempty"`
	// MultipleValuesExtendedEdit types several values straight into the field
	// rather than in the dialog of a list of values (help, since 8.3.23). The
	// help gives it true by default, but the prototype writes it - as
	// ExtendedEditMultipleValues - only as true and only on some fields: 600
	// input fields of 15 450 in lombard1, side by side with fields without it
	// in 54 forms, and neither the designer nor EDT ever writes false. What
	// it leaves out is therefore off, the behaviour of a field made before
	// 8.3.23.
	MultipleValuesExtendedEdit bool `yaml:"multiple_values_extended_edit,omitempty" json:"multipleValuesExtendedEdit,omitempty"`
}

func (input FieldTextInput) empty() bool {
	return !input.NoWrap && !input.NoTextEdit && input.MultiLine == nil && input.ExtendedEdit == nil && input.PasswordMode == nil &&
		input.Mask == "" && len(input.InputHint) == 0 && input.EditTextUpdate == "" && input.SpecialTextInputMode == "" &&
		input.SpellChecking == "" && input.AutoCorrection == "" && input.HeightControlVariant == "" && !input.MultipleValuesExtendedEdit
}

func validateFieldTextInput(path string, input FieldTextInput, kind FormElementKind, configuration project.Project) []string {
	if input.empty() {
		return nil
	}
	if kind != FormElementInputField {
		rest := input
		switch kind {
		case FormElementLabelField:
			rest.PasswordMode = nil
		case FormElementTable:
			rest.HeightControlVariant = ""
		}
		if !rest.empty() {
			return []string{path + " has the text input of an input field"}
		}
	}
	var issues []string
	issues = append(issues, validateTitle(path+".input_hint", input.InputHint, configuration)...)
	issues = append(issues, oneOf(path+".edit_text_update", input.EditTextUpdate,
		FormEditTextUpdateAuto, FormEditTextUpdateAlways, FormEditTextUpdateOnValueChange, FormEditTextUpdateDontUse)...)
	issues = append(issues, oneOf(path+".special_text_input_mode", input.SpecialTextInputMode,
		FormSpecialTextInputAuto, FormSpecialTextInputNone, FormSpecialTextInputDigits, FormSpecialTextInputDigitsAndPunctuation,
		FormSpecialTextInputEmail, FormSpecialTextInputPhoneNumber, FormSpecialTextInputURL)...)
	uses := []FormTextInputUse{FormTextInputUseAuto, FormTextInputUseUse, FormTextInputUseDontUse}
	issues = append(issues, oneOf(path+".spell_checking", input.SpellChecking, uses...)...)
	issues = append(issues, oneOf(path+".auto_correction", input.AutoCorrection, uses...)...)
	variants := []FormHeightControlVariant{FormHeightControlAuto, FormHeightControlUseContentHeight, FormHeightControlUseHeightInFormRows}
	if kind == FormElementTable {
		variants = append(variants, FormHeightControlUseHeightInTableRows)
	}
	issues = append(issues, oneOf(path+".height_control_variant", input.HeightControlVariant, variants...)...)
	return issues
}

// FormNumber is a number an element of a form is given, as the prototype
// writes it there - always a decimal (xs:decimal), never a string. It is kept
// as its digits, so that no decimal is rounded on the way, and YAML and JSON
// carry it as a number: a number in quotes is refused, as is anything that is
// not decimal digits with an optional sign and fraction.
type FormNumber string

// MarshalYAML writes the number as a number.
func (number FormNumber) MarshalYAML() (any, error) {
	tag := "!!int"
	if strings.Contains(string(number), ".") {
		tag = "!!float"
	}
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: string(number)}, nil
}

// UnmarshalYAML reads a number written as one.
func (number *FormNumber) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.ScalarNode || (node.ShortTag() != "!!int" && node.ShortTag() != "!!float") || !decimalText.MatchString(node.Value) {
		return fmt.Errorf("line %d: %q is not a number written as decimal digits", node.Line, node.Value)
	}
	*number = FormNumber(node.Value)
	return nil
}

// MarshalJSON writes the number as a number.
func (number FormNumber) MarshalJSON() ([]byte, error) {
	if !decimalText.MatchString(string(number)) {
		return nil, fmt.Errorf("%q is not a number written as decimal digits", string(number))
	}
	return []byte(number), nil
}

// UnmarshalJSON reads a number written as one.
func (number *FormNumber) UnmarshalJSON(data []byte) error {
	if !decimalText.Match(data) {
		return fmt.Errorf("%s is not a number written as decimal digits", data)
	}
	*number = FormNumber(data)
	return nil
}

// FieldFormat is how a field shows a value and bounds a number (help, the
// extensions of a form field). Each property stands on the fields the help
// gives it, which are the fields the prototype writes it on.
type FieldFormat struct {
	// Format is how the value is shown, on an input and a label field;
	// EditFormat how it is shown while edited, on an input field, and on a
	// check box the text of true and false shown as a tumbler. Both are
	// written in languages, as the format of an attribute is.
	Format     LocalizedText `yaml:"format,omitempty" json:"format,omitempty"`
	EditFormat LocalizedText `yaml:"edit_format,omitempty" json:"editFormat,omitempty"`
	// MinValue and MaxValue bound the number entered in an input field, and
	// the scale of a track bar and a progress bar. Neither is checked against
	// the other: the exports never set the minimum above the maximum, and
	// nothing says the designer refuses to keep it.
	MinValue FormNumber `yaml:"min_value,omitempty" json:"minValue,omitempty"`
	MaxValue FormNumber `yaml:"max_value,omitempty" json:"maxValue,omitempty"`
	// MarkNegatives shows a number below zero in red, on an input and a label
	// field, and AutoMarkIncomplete marks an input field holding the empty
	// value of its type, and a table holding no row (29 and 42 times). Each
	// is yes, no or not said: the help gives each
	// Undefined, chosen by the attribute, and the prototype writes both true
	// and false (235 and 17, 1410 and 302 times).
	MarkNegatives      *bool `yaml:"mark_negatives,omitempty" json:"markNegatives,omitempty"`
	AutoMarkIncomplete *bool `yaml:"auto_mark_incomplete,omitempty" json:"autoMarkIncomplete,omitempty"`
}

func validateFieldFormat(path string, format FieldFormat, kind FormElementKind, configuration project.Project) []string {
	var issues []string
	only := func(name string, set bool, what string, kinds ...FormElementKind) {
		if set && !slices.Contains(kinds, kind) {
			issues = append(issues, path+"."+name+" is allowed only for "+what)
		}
	}
	only("format", len(format.Format) != 0, "input and label fields, usual groups and pages", FormElementInputField, FormElementLabelField,
		FormElementUsualGroup, FormElementPage)
	only("edit_format", len(format.EditFormat) != 0, "input fields and check boxes", FormElementInputField, FormElementCheckBoxField)
	bounded := []FormElementKind{FormElementInputField, FormElementTrackBarField, FormElementProgressBarField}
	only("min_value", format.MinValue != "", "input fields, track bars and progress bars", bounded...)
	only("max_value", format.MaxValue != "", "input fields, track bars and progress bars", bounded...)
	only("mark_negatives", format.MarkNegatives != nil, "input and label fields", FormElementInputField, FormElementLabelField)
	only("auto_mark_incomplete", format.AutoMarkIncomplete != nil, "input fields and tables", FormElementInputField, FormElementTable)
	issues = append(issues, validateTitle(path+".format", format.Format, configuration)...)
	issues = append(issues, validateTitle(path+".edit_format", format.EditFormat, configuration)...)
	for _, bound := range []struct {
		name  string
		value FormNumber
	}{{"min_value", format.MinValue}, {"max_value", format.MaxValue}} {
		if bound.value != "" && !decimalText.MatchString(string(bound.value)) {
			issues = append(issues, path+"."+bound.name+" must be a number written as decimal digits")
		}
	}
	return issues
}

// FormIncompleteChoiceMode is when an input field offers to pick a value while
// it is empty (help, IncompleteChoiceMode). Empty is not said; the prototype
// writes only OnActivate (97 times).
type FormIncompleteChoiceMode string

const (
	FormIncompleteChoiceOnActivate     FormIncompleteChoiceMode = "on-activate"
	FormIncompleteChoiceOnEnterPressed FormIncompleteChoiceMode = "on-enter-pressed"
)

// InputFieldChoice is how a value of an input field is picked (help, the
// extension of a form field for an input field). The prototype writes these
// on input fields only. The list the value is picked from and whether a folder
// or an item may be picked are apart.
type InputFieldChoice struct {
	// ListChoiceMode makes the field pick one of its choice list, of any
	// type; the prototype writes only the "on" (2448 times).
	ListChoiceMode bool `yaml:"list_choice_mode,omitempty" json:"listChoiceMode,omitempty"`
	// QuickChoice picks from a drop list instead of a form; yes, no or not
	// said, as the help gives Undefined and the prototype writes both (180
	// and 210 times).
	QuickChoice *bool `yaml:"quick_choice,omitempty" json:"quickChoice,omitempty"`
	// ChoiceForm is the form opened to pick a value, named as on an attribute.
	// ChoiceFormGone is the identifier the prototype writes in its place for
	// a form the configuration no longer has (9 times in erp, naming nothing
	// in any export): a remnant of what was deleted, carried and resolved to
	// nothing, as a vanished type is.
	ChoiceForm     *ChoiceFormReference `yaml:"choice_form,omitempty" json:"choiceForm,omitempty"`
	ChoiceFormGone *uuid.UUID           `yaml:"choice_form_gone,omitempty" json:"choiceFormGone,omitempty"`
	// ChoiceListHeight is the height of the drop list in lines, and
	// DropListWidth its width in characters; zero is chosen by the platform.
	ChoiceListHeight int `yaml:"choice_list_height,omitempty" json:"choiceListHeight,omitempty"`
	DropListWidth    int `yaml:"drop_list_width,omitempty" json:"dropListWidth,omitempty"`
	// ChoiceHistoryOnInput is whether what was picked before is offered, as
	// on an attribute.
	ChoiceHistoryOnInput ChoiceHistory `yaml:"choice_history_on_input,omitempty" json:"choiceHistoryOnInput,omitempty"`
	// AutoChoiceIncomplete offers to pick a value while the field is empty;
	// yes, no or not said, as the help gives Undefined (276 and 42 times).
	AutoChoiceIncomplete *bool                    `yaml:"auto_choice_incomplete,omitempty" json:"autoChoiceIncomplete,omitempty"`
	IncompleteChoiceMode FormIncompleteChoiceMode `yaml:"incomplete_choice_mode,omitempty" json:"incompleteChoiceMode,omitempty"`
}

func (choice InputFieldChoice) empty() bool {
	return !choice.ListChoiceMode && choice.QuickChoice == nil && choice.ChoiceForm == nil && choice.ChoiceFormGone == nil &&
		choice.ChoiceListHeight == 0 && choice.DropListWidth == 0 && choice.ChoiceHistoryOnInput == "" &&
		choice.AutoChoiceIncomplete == nil && choice.IncompleteChoiceMode == ""
}

func validateInputFieldChoice(path string, choice InputFieldChoice, kind FormElementKind) []string {
	if choice.empty() {
		return nil
	}
	if kind != FormElementInputField {
		return []string{path + " has the choice of an input field"}
	}
	var issues []string
	if choice.ChoiceForm != nil {
		issues = append(issues, validateChoiceFormReference(path+".choice_form", *choice.ChoiceForm)...)
		if choice.ChoiceFormGone != nil {
			issues = append(issues, path+".choice_form_gone stands in place of a choice form, not beside one")
		}
	}
	if choice.ChoiceFormGone != nil && choice.ChoiceFormGone.IsZero() {
		issues = append(issues, path+".choice_form_gone must be a non-zero UUID")
	}
	if choice.ChoiceListHeight < 0 {
		issues = append(issues, path+".choice_list_height must not be negative")
	}
	if choice.DropListWidth < 0 {
		issues = append(issues, path+".drop_list_width must not be negative")
	}
	if !validChoiceHistory(choice.ChoiceHistoryOnInput) {
		issues = append(issues, path+".choice_history_on_input must be auto or dont-use")
	}
	issues = append(issues, oneOf(path+".incomplete_choice_mode", choice.IncompleteChoiceMode,
		FormIncompleteChoiceOnActivate, FormIncompleteChoiceOnEnterPressed)...)
	return issues
}

// FormChoiceListItem is one value of the choice list of an input field or a
// radio button field (help, ChoiceList, a value list): the value as written at
// design time, its presentation in languages, and whether it is checked. The
// prototype writes all three for each of 8247 items in the exports, never
// checked; the presentation of the list item beside that of the value is
// always empty there and is not carried.
type FormChoiceListItem struct {
	Value        Value         `yaml:"value" json:"value"`
	Presentation LocalizedText `yaml:"presentation,omitempty" json:"presentation,omitempty"`
	Check        bool          `yaml:"check,omitempty" json:"check,omitempty"`
}

func validateChoiceList(path string, list []FormChoiceListItem, kind FormElementKind, configuration project.Project) []string {
	if len(list) == 0 {
		return nil
	}
	if kind != FormElementInputField && kind != FormElementRadioButtonField {
		return []string{path + ".choice_list is allowed only for input and radio button fields"}
	}
	var issues []string
	for index, item := range list {
		where := fmt.Sprintf("%s.choice_list[%d]", path, index)
		issues = append(issues, validateDesignTimeValue(where+".value", item.Value)...)
		issues = append(issues, validateTitle(where+".presentation", item.Presentation, configuration)...)
	}
	return issues
}

// FormChoiceParameterLink takes a choice parameter of an input field from the
// data of the form (help, ChoiceParameterLink). DataPath is a path in the
// data of the form, written as the prototype writes it - see
// validateFormLinkPath.
type FormChoiceParameterLink struct {
	Name        string      `yaml:"name" json:"name"`
	DataPath    string      `yaml:"data_path" json:"dataPath"`
	ValueChange ValueChange `yaml:"value_change,omitempty" json:"valueChange,omitempty"`
}

// FormTypeLink takes the type of the value of an input field from the data of
// the form (help, TypeLink): DataPath is written as in a link of a choice
// parameter, LinkItem is the element of the type description taken, counted
// from zero (0 to 3 in the exports).
type FormTypeLink struct {
	DataPath string `yaml:"data_path" json:"dataPath"`
	LinkItem int    `yaml:"link_item,omitempty" json:"linkItem,omitempty"`
}

// InputFieldChoiceParameters narrow what an input field offers to pick and of
// which type (help, the extension of a form field for an input field). The
// prototype writes them on input fields only.
type InputFieldChoiceParameters struct {
	// ChoiceParameters are fixed, as on an attribute: a value written at
	// design time, or a fixed array of them, which is a list.
	ChoiceParameters     []ChoiceParameter         `yaml:"choice_parameters,omitempty" json:"choiceParameters,omitempty"`
	ChoiceParameterLinks []FormChoiceParameterLink `yaml:"choice_parameter_links,omitempty" json:"choiceParameterLinks,omitempty"`
	TypeLink             *FormTypeLink             `yaml:"type_link,omitempty" json:"typeLink,omitempty"`
	// NoChooseType stops asking for the type before a value of a composite
	// type is picked, and NoTypeDomain allows a type description of one type
	// only: both are on by default, and the prototype writes only the "off"
	// (1342 and 46 times).
	NoChooseType bool `yaml:"no_choose_type,omitempty" json:"noChooseType,omitempty"`
	NoTypeDomain bool `yaml:"no_type_domain,omitempty" json:"noTypeDomain,omitempty"`
	// AvailableTypes are the types offered when the field edits a type
	// description, with the bounds of their qualifiers.
	AvailableTypes []Type `yaml:"available_types,omitempty" json:"availableTypes,omitempty"`
	// ChoiceFoldersAndItems is what a choice from a hierarchical object may
	// land on, as on an attribute.
	ChoiceFoldersAndItems ChoiceTarget `yaml:"choice_folders_and_items,omitempty" json:"choiceFoldersAndItems,omitempty"`
}

func (parameters InputFieldChoiceParameters) empty() bool {
	return len(parameters.ChoiceParameters) == 0 && len(parameters.ChoiceParameterLinks) == 0 && parameters.TypeLink == nil &&
		!parameters.NoChooseType && !parameters.NoTypeDomain && len(parameters.AvailableTypes) == 0 && parameters.ChoiceFoldersAndItems == ""
}

func validateInputFieldChoiceParameters(path string, parameters InputFieldChoiceParameters, kind FormElementKind) []string {
	if parameters.empty() {
		return nil
	}
	if kind != FormElementInputField {
		return []string{path + " has the choice parameters of an input field"}
	}
	var issues []string
	names := map[string]bool{}
	for index, parameter := range parameters.ChoiceParameters {
		where := fmt.Sprintf("%s.choice_parameters[%d]", path, index)
		issues = append(issues, validateChoiceParameterName(where, parameter.Name, names)...)
		if len(parameter.Values) == 0 {
			issues = append(issues, where+".values must contain at least one value")
		}
		for position, value := range parameter.Values {
			issues = append(issues, validateDesignTimeValue(fmt.Sprintf("%s.values[%d]", where, position), value)...)
		}
		if !parameter.List && len(parameter.Values) > 1 {
			issues = append(issues, where+" sets several values, so it must say it is a list")
		}
	}
	linked := map[string]bool{}
	for index, link := range parameters.ChoiceParameterLinks {
		where := fmt.Sprintf("%s.choice_parameter_links[%d]", path, index)
		issues = append(issues, validateChoiceParameterName(where, link.Name, linked)...)
		issues = append(issues, validateFormLinkPath(where+".data_path", link.DataPath)...)
		issues = append(issues, oneOf(where+".value_change", link.ValueChange, ValueChangeClear, ValueChangeDontChange)...)
	}
	if link := parameters.TypeLink; link != nil {
		issues = append(issues, validateFormLinkPath(path+".type_link.data_path", link.DataPath)...)
		if link.LinkItem < 0 {
			issues = append(issues, path+".type_link.link_item must not be negative")
		}
	}
	issues = append(issues, validateTypesIn(path+".available_types", parameters.AvailableTypes, placeFormAttribute)...)
	if !validChoiceTarget(parameters.ChoiceFoldersAndItems) {
		issues = append(issues, path+".choice_folders_and_items must be items, folders or folders-and-items")
	}
	return issues
}

// validateFormLinkPath checks the path a link of an input field takes its
// value or type from, as the prototype writes one: a data path of the form
// ("Объект.Партнер", "Items.Список.CurrentData.Вид"), a number (22 and 30
// times), or the code of an element of a form: segments joined by a slash,
// each a number or a number with an identifier - "48:02023637-…/0:3c1e…"
// (13), "342:02023637-…/15" (3), "1/0:ba7dcb3b-…" (20). A code of several
// segments leads to nothing and is a remnant (notePathCode).
func validateFormLinkPath(path, value string) []string {
	if value == "" || strings.TrimSpace(value) != value {
		return []string{path + " must be a data path without surrounding spaces"}
	}
	if allDigits(value) || elementDataPath(value) || formElementCode.MatchString(value) {
		return nil
	}
	return []string{path + " must be a data path, a number or the code of an element of a form"}
}

// formElementCode is the code the prototype writes for an element of a form:
// segments joined by a slash, each a number, or a number, a colon and an
// identifier - the one the platform names an element of a form by, or
// another.
var formElementCode = regexp.MustCompile(`^[0-9]+(:[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})?(/[0-9]+(:[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})?)*$`)

// formPathCode is a data path the prototype writes as a code of several
// segments: an attribute of the form by its number, or an element by its
// code, then a field of what it holds - by its identifier ("1/0:3c1e…", a
// column "1/0:<table part>/0:<column>", the footer of a column
// "…/101000000:<column>"), a standard attribute by a negative number
// ("1/-2", "1/-5") or a column of a value table by its number ("1/0",
// "5/10000000").
var formPathCode = regexp.MustCompile(`^[0-9]+(:[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})?(/-?[0-9]+(:[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})?)+$`)

// deletedFormElement is the code the configurator leaves where a property
// named an element of the form that has since been deleted: the number the
// element had and the identifier the platform names an element of a form by.
// While the element is there, the reference is written by its name: the
// exports of the configurator and of EDT side by side (mdclasses) name the
// group of the user settings of a new list form, which is element 1, by its
// name, and in the exports no element of the form has the number of such a
// code - 126 of 126; a list form of erp whose table is element 3 points its
// user settings at 1. It is a remnant of what was deleted (2.203). A code of
// several segments is a path (notePathCode).
var deletedFormElement = regexp.MustCompile(`^[0-9]+:02023637-7868-4a5f-8576-835a76e0c9ba$`)

// FormCheckBoxType is how a check box is drawn (help, CheckBoxType). The
// prototype writes Auto as well (9511 times), Switch as "Switcher" (34), and
// nothing on a check box of three states (all 137 of them).
type FormCheckBoxType string

const (
	FormCheckBoxAuto     FormCheckBoxType = "auto"
	FormCheckBoxCheckBox FormCheckBoxType = "check-box"
	FormCheckBoxTumbler  FormCheckBoxType = "tumbler"
	FormCheckBoxSwitch   FormCheckBoxType = "switch"
)

// FormRadioButtonType is how a radio button field is drawn (help,
// RadioButtonType); the prototype writes RadioButton as "RadioButtons".
type FormRadioButtonType string

const (
	FormRadioButtonAuto        FormRadioButtonType = "auto"
	FormRadioButtonRadioButton FormRadioButtonType = "radio-button"
	FormRadioButtonTumbler     FormRadioButtonType = "tumbler"
)

// FieldValueView is how a label field, a picture field, a check box and a
// radio button field show their value (help, the extensions of a form field for each). Each
// property stands on the fields the help gives it, which are the fields the
// prototype writes it on.
type FieldValueView struct {
	// Hyperlink shows the text of a label field as a link, and makes a
	// picture field a link clicked; the prototype writes only the "on", on a
	// label field as "Hiperlink" (1103 times), on a picture field as
	// "Hyperlink" (54).
	Hyperlink bool `yaml:"hyperlink,omitempty" json:"hyperlink,omitempty"`
	// CheckBoxType and ThreeState are of a check box: a check box of three
	// states edits a number. The prototype writes only the "on" of the
	// second.
	CheckBoxType FormCheckBoxType `yaml:"check_box_type,omitempty" json:"checkBoxType,omitempty"`
	ThreeState   bool             `yaml:"three_state,omitempty" json:"threeState,omitempty"`
	// EqualItemsWidth gives the items of a check box drawn as a tumbler one
	// width; yes, no or not said, which the help reads as yes.
	EqualItemsWidth *bool `yaml:"equal_items_width,omitempty" json:"equalItemsWidth,omitempty"`
	// RadioButtonType, ColumnsCount and EqualColumnsWidth are of a radio
	// button field; the last is yes, no or not said, which the help reads as
	// no. Zero columns is chosen by the platform.
	RadioButtonType   FormRadioButtonType `yaml:"radio_button_type,omitempty" json:"radioButtonType,omitempty"`
	ColumnsCount      int                 `yaml:"columns_count,omitempty" json:"columnsCount,omitempty"`
	EqualColumnsWidth *bool               `yaml:"equal_columns_width,omitempty" json:"equalColumnsWidth,omitempty"`
	// The width, height and height of the title of an item of a check box or
	// a radio button field; zero is chosen by the platform.
	ItemWidth       int `yaml:"item_width,omitempty" json:"itemWidth,omitempty"`
	ItemHeight      int `yaml:"item_height,omitempty" json:"itemHeight,omitempty"`
	ItemTitleHeight int `yaml:"item_title_height,omitempty" json:"itemTitleHeight,omitempty"`
}

func validateFieldValueView(path string, view FieldValueView, kind FormElementKind) []string {
	var issues []string
	only := func(name string, set bool, what string, kinds ...FormElementKind) {
		if set && !slices.Contains(kinds, kind) {
			issues = append(issues, path+"."+name+" is allowed only for "+what)
		}
	}
	only("hyperlink", view.Hyperlink, "label and picture fields and decorations", FormElementLabelField, FormElementPictureField,
		FormElementLabelDecoration, FormElementPictureDecoration)
	only("check_box_type", view.CheckBoxType != "", "check boxes", FormElementCheckBoxField)
	only("three_state", view.ThreeState, "check boxes", FormElementCheckBoxField)
	only("equal_items_width", view.EqualItemsWidth != nil, "check boxes", FormElementCheckBoxField)
	only("radio_button_type", view.RadioButtonType != "", "radio button fields", FormElementRadioButtonField)
	only("columns_count", view.ColumnsCount != 0, "radio button fields", FormElementRadioButtonField)
	only("equal_columns_width", view.EqualColumnsWidth != nil, "radio button fields", FormElementRadioButtonField)
	items := []FormElementKind{FormElementCheckBoxField, FormElementRadioButtonField}
	only("item_width", view.ItemWidth != 0, "check boxes and radio button fields", items...)
	only("item_height", view.ItemHeight != 0, "check boxes and radio button fields", items...)
	only("item_title_height", view.ItemTitleHeight != 0, "check boxes and radio button fields", items...)
	issues = append(issues, oneOf(path+".check_box_type", view.CheckBoxType, FormCheckBoxAuto, FormCheckBoxCheckBox, FormCheckBoxTumbler, FormCheckBoxSwitch)...)
	issues = append(issues, oneOf(path+".radio_button_type", view.RadioButtonType, FormRadioButtonAuto, FormRadioButtonRadioButton, FormRadioButtonTumbler)...)
	for _, size := range []struct {
		name  string
		value int
	}{{"columns_count", view.ColumnsCount}, {"item_width", view.ItemWidth}, {"item_height", view.ItemHeight}, {"item_title_height", view.ItemTitleHeight}} {
		if size.value < 0 {
			issues = append(issues, path+"."+size.name+" must not be negative")
		}
	}
	return issues
}

// FormPictureSize is how a picture field fits a picture to its size (help,
// PictureSize). Empty is not said.
type FormPictureSize string

const (
	FormPictureAutoSize            FormPictureSize = "auto-size"
	FormPictureAutoSizeIgnoreScale FormPictureSize = "auto-size-ignore-scale"
	FormPictureByFontSize          FormPictureSize = "by-font-size"
	FormPictureProportionally      FormPictureSize = "proportionally"
	FormPictureRealSize            FormPictureSize = "real-size"
	FormPictureRealSizeIgnoreScale FormPictureSize = "real-size-ignore-scale"
	FormPictureStretch             FormPictureSize = "stretch"
	FormPictureTile                FormPictureSize = "tile"
)

// FormFileDragMode is what a picture field is handed when files are dragged
// onto it (help, FileDragMode): a file, or a reference to one. Empty is not
// said; the prototype writes only AsFile (1406 times).
type FormFileDragMode string

const (
	FormFileDragAsFile    FormFileDragMode = "as-file"
	FormFileDragAsFileRef FormFileDragMode = "as-file-ref"
)

// FieldPicture is what a picture field has besides what every field has
// (help, the extension of a form field for a picture field). Whether it is a
// link is in FieldValueView, with the link of a label field.
type FieldPicture struct {
	// ValuesPicture is the set of pictures a number or a boolean the field
	// shows picks from by index - a common picture (1397 times), a standard
	// one (76) or a file of the element's own (22).
	ValuesPicture *PictureReference `yaml:"values_picture,omitempty" json:"valuesPicture,omitempty"`
	PictureSize   FormPictureSize   `yaml:"picture_size,omitempty" json:"pictureSize,omitempty"`
	// NonselectedPictureText is shown while no picture is chosen.
	NonselectedPictureText LocalizedText `yaml:"nonselected_picture_text,omitempty" json:"nonselectedPictureText,omitempty"`
	// Zoomable scrolls a picture larger than the field and lets it be
	// zoomed; the prototype writes only the "on" (9 times).
	Zoomable     bool             `yaml:"zoomable,omitempty" json:"zoomable,omitempty"`
	FileDragMode FormFileDragMode `yaml:"file_drag_mode,omitempty" json:"fileDragMode,omitempty"`
	// ImageScale is the scale of the picture of a picture decoration in
	// percent, besides the scale of the form (help, Scale - the prototype
	// writes ImageScale, 6 times); 0 is the default, 100.
	ImageScale int `yaml:"image_scale,omitempty" json:"imageScale,omitempty"`
}

func (picture FieldPicture) empty() bool {
	return picture.ValuesPicture == nil && picture.PictureSize == "" && len(picture.NonselectedPictureText) == 0 &&
		!picture.Zoomable && picture.FileDragMode == "" && picture.ImageScale == 0
}

func validateFieldPicture(path string, picture FieldPicture, kind FormElementKind, configuration project.Project) []string {
	if picture.empty() {
		return nil
	}
	var issues []string
	only := func(name string, set bool, what string, kinds ...FormElementKind) {
		if set && !slices.Contains(kinds, kind) {
			issues = append(issues, path+"."+name+" is allowed only for "+what)
		}
	}
	// A picture decoration shows its picture as a picture field shows a
	// value (help, the extension of a decoration for a picture), and a table
	// takes files dragged onto it as a picture field does (help, FormTable;
	// the prototype writes AsFile on 8502 tables).
	pictures := []FormElementKind{FormElementPictureField, FormElementPictureDecoration}
	only("values_picture", picture.ValuesPicture != nil, "picture fields", FormElementPictureField)
	only("picture_size", picture.PictureSize != "", "picture fields and pictures", pictures...)
	only("nonselected_picture_text", len(picture.NonselectedPictureText) != 0, "picture fields and pictures", pictures...)
	only("zoomable", picture.Zoomable, "picture fields and pictures", pictures...)
	only("file_drag_mode", picture.FileDragMode != "", "picture fields, pictures and tables", FormElementPictureField, FormElementPictureDecoration, FormElementTable)
	only("image_scale", picture.ImageScale != 0, "pictures", FormElementPictureDecoration)
	if picture.ImageScale < 0 {
		issues = append(issues, path+".image_scale must not be negative")
	}
	issues = append(issues, validateElementPictureReference(path+".values_picture", picture.ValuesPicture)...)
	issues = append(issues, oneOf(path+".picture_size", picture.PictureSize, FormPictureAutoSize, FormPictureAutoSizeIgnoreScale,
		FormPictureByFontSize, FormPictureProportionally, FormPictureRealSize, FormPictureRealSizeIgnoreScale, FormPictureStretch, FormPictureTile)...)
	issues = append(issues, validateTitle(path+".nonselected_picture_text", picture.NonselectedPictureText, configuration)...)
	issues = append(issues, oneOf(path+".file_drag_mode", picture.FileDragMode, FormFileDragAsFile, FormFileDragAsFileRef)...)
	return issues
}

// FormScrollBarUse is when a scroll bar is shown (help, ScrollBarUse). A
// table writes it so. A spreadsheet document field writes it as a boolean,
// and the forms the mdclasses project keeps in both formats of export tell
// which: one written nowhere is ScrollAuto in the other format (16 pairs),
// true is ScrollAlways (2). False is never: where the subsystems library,
// kept in both formats, writes false on five fields, the other format writes
// nothing - its default, the one use left. Empty is not said: auto.
type FormScrollBarUse string

const (
	FormScrollBarAutoUse   FormScrollBarUse = "auto-use"
	FormScrollBarUseAlways FormScrollBarUse = "use-always"
	FormScrollBarDontUse   FormScrollBarUse = "dont-use"
)

// FormViewScalingMode is how a spreadsheet document is scaled for viewing
// (help, ViewScalingMode).
type FormViewScalingMode string

const (
	FormViewScalingAuto   FormViewScalingMode = "auto"
	FormViewScalingNormal FormViewScalingMode = "normal"
	FormViewScalingLarge  FormViewScalingMode = "large"
)

// FormSelectionShowMode is when the selection of a spreadsheet document is
// drawn (help, SelectionShowMode).
type FormSelectionShowMode string

const (
	FormSelectionAlways                              FormSelectionShowMode = "always"
	FormSelectionDontShow                            FormSelectionShowMode = "dont-show"
	FormSelectionWhenActive                          FormSelectionShowMode = "when-active"
	FormSelectionWhenMultipleCellsSelected           FormSelectionShowMode = "when-multiple-cells-selected"
	FormSelectionWhenMultipleCellsSelectedWhenActive FormSelectionShowMode = "when-multiple-cells-selected-when-active"
)

// FormUseOutput is whether a document may be printed, saved and copied
// (help, UseOutput).
type FormUseOutput string

const (
	FormUseOutputAuto    FormUseOutput = "auto"
	FormUseOutputEnable  FormUseOutput = "enable"
	FormUseOutputDisable FormUseOutput = "disable"
)

// FieldDocument is what the fields of a spreadsheet, text, HTML and formatted
// document have (help, the extensions of a form field for each), with what
// the other fields share of it: a graphical schema is edited and printed, a
// calendar and a planner are dragged. Each
// property stands on the kinds of element it is written on; a table, a
// decoration and the other fields add theirs where they are described.
//
// The prototype writes the switches of a spreadsheet document field in sets,
// some on and some off, so what it leaves out says nothing of the default:
// each is yes, no or not said.
type FieldDocument struct {
	// VerticalScrollBar and HorizontalScrollBar are when the scroll bars are
	// shown - see FormScrollBarUse for how a spreadsheet document field
	// writes them (true 658 and 650 times, false 28 and 33).
	VerticalScrollBar   FormScrollBarUse      `yaml:"vertical_scroll_bar,omitempty" json:"verticalScrollBar,omitempty"`
	HorizontalScrollBar FormScrollBarUse      `yaml:"horizontal_scroll_bar,omitempty" json:"horizontalScrollBar,omitempty"`
	ViewScalingMode     FormViewScalingMode   `yaml:"view_scaling_mode,omitempty" json:"viewScalingMode,omitempty"`
	SelectionShowMode   FormSelectionShowMode `yaml:"selection_show_mode,omitempty" json:"selectionShowMode,omitempty"`
	// Edit lets the document be changed; Protection protects the cells.
	Edit       *bool `yaml:"edit,omitempty" json:"edit,omitempty"`
	Protection *bool `yaml:"protection,omitempty" json:"protection,omitempty"`
	// What a spreadsheet document shows: the headers of its rows and
	// columns, the grid, the groupings, and the names of its areas of cells
	// and of rows and columns.
	ShowHeaders           *bool `yaml:"show_headers,omitempty" json:"showHeaders,omitempty"`
	ShowGrid              *bool `yaml:"show_grid,omitempty" json:"showGrid,omitempty"`
	ShowGroups            *bool `yaml:"show_groups,omitempty" json:"showGroups,omitempty"`
	ShowCellNames         *bool `yaml:"show_cell_names,omitempty" json:"showCellNames,omitempty"`
	ShowRowAndColumnNames *bool `yaml:"show_row_and_column_names,omitempty" json:"showRowAndColumnNames,omitempty"`
	// EnableDrag and EnableStartDrag let the element take what is dragged
	// onto it and start dragging from it.
	EnableDrag      *bool         `yaml:"enable_drag,omitempty" json:"enableDrag,omitempty"`
	EnableStartDrag *bool         `yaml:"enable_start_drag,omitempty" json:"enableStartDrag,omitempty"`
	Output          FormUseOutput `yaml:"output,omitempty" json:"output,omitempty"`
	// ExcludedCommands are the standard commands of the element taken out of
	// its command bar and context menu, by name as the prototype writes them
	// (CommandSet, which the help does not name).
	ExcludedCommands []string `yaml:"excluded_commands,omitempty" json:"excludedCommands,omitempty"`
}

func validateFieldDocument(path string, document FieldDocument, kind FormElementKind) []string {
	var issues []string
	only := func(name string, set bool, what string, kinds ...FormElementKind) {
		if set && !slices.Contains(kinds, kind) {
			issues = append(issues, path+"."+name+" is allowed only for "+what)
		}
	}
	spreadsheet := []FormElementKind{FormElementSpreadsheetDocumentField}
	for _, property := range []struct {
		name string
		set  bool
	}{
		{"view_scaling_mode", document.ViewScalingMode != ""}, {"selection_show_mode", document.SelectionShowMode != ""},
		{"protection", document.Protection != nil}, {"show_headers", document.ShowHeaders != nil},
		{"show_grid", document.ShowGrid != nil}, {"show_groups", document.ShowGroups != nil}, {"show_cell_names", document.ShowCellNames != nil},
		{"show_row_and_column_names", document.ShowRowAndColumnNames != nil},
	} {
		only(property.name, property.set, "spreadsheet document fields", spreadsheet...)
	}
	only("edit", document.Edit != nil, "spreadsheet document and graphical schema fields", FormElementSpreadsheetDocumentField, FormElementGraphicalSchemaField)
	only("vertical_scroll_bar", document.VerticalScrollBar != "", "spreadsheet document fields and tables", FormElementSpreadsheetDocumentField, FormElementTable)
	only("horizontal_scroll_bar", document.HorizontalScrollBar != "", "spreadsheet document fields and tables", FormElementSpreadsheetDocumentField, FormElementTable)
	dragging := []FormElementKind{FormElementSpreadsheetDocumentField, FormElementCalendarField, FormElementPlannerField, FormElementTable,
		FormElementPictureDecoration}
	only("enable_drag", document.EnableDrag != nil, "spreadsheet document, calendar and planner fields, tables and pictures", dragging...)
	only("enable_start_drag", document.EnableStartDrag != nil, "spreadsheet document, calendar and planner fields, tables and pictures", dragging...)
	only("output", document.Output != "", "spreadsheet, text, HTML and formatted document and graphical schema fields and tables",
		FormElementSpreadsheetDocumentField, FormElementTextDocumentField, FormElementHTMLDocumentField, FormElementFormattedDocumentField,
		FormElementGraphicalSchemaField, FormElementTable)
	only("excluded_commands", len(document.ExcludedCommands) != 0, "spreadsheet and formatted document fields and tables",
		FormElementSpreadsheetDocumentField, FormElementFormattedDocumentField, FormElementTable)
	scrollBars := []FormScrollBarUse{FormScrollBarAutoUse, FormScrollBarUseAlways, FormScrollBarDontUse}
	issues = append(issues, oneOf(path+".vertical_scroll_bar", document.VerticalScrollBar, scrollBars...)...)
	issues = append(issues, oneOf(path+".horizontal_scroll_bar", document.HorizontalScrollBar, scrollBars...)...)
	issues = append(issues, oneOf(path+".view_scaling_mode", document.ViewScalingMode, FormViewScalingAuto, FormViewScalingNormal, FormViewScalingLarge)...)
	issues = append(issues, oneOf(path+".selection_show_mode", document.SelectionShowMode, FormSelectionAlways, FormSelectionDontShow,
		FormSelectionWhenActive, FormSelectionWhenMultipleCellsSelected, FormSelectionWhenMultipleCellsSelectedWhenActive)...)
	issues = append(issues, oneOf(path+".output", document.Output, FormUseOutputAuto, FormUseOutputEnable, FormUseOutputDisable)...)
	issues = append(issues, validateExcludedCommands(path+".excluded_commands", document.ExcludedCommands)...)
	return issues
}

// validateExcludedCommands checks the standard commands taken out of a form or
// an element: each by name, once.
func validateExcludedCommands(path string, commands []string) []string {
	var issues []string
	seen := map[string]bool{}
	for index, command := range commands {
		switch {
		case !validIdentifier(command):
			issues = append(issues, fmt.Sprintf("%s[%d] must be the name of a command", path, index))
		case seen[command]:
			issues = append(issues, fmt.Sprintf("%s[%d] names %s twice", path, index, command))
		}
		seen[command] = true
	}
	return issues
}

// FormSelectionMode is how an element selects: the dates of a calendar field
// (help, DateSelectionMode), and the rows of a table where a table is
// described. The values allowed depend on the kind of element.
type FormSelectionMode string

// FormElementRepresentation is how an element is drawn: a progress bar
// (help, ProgressBarSmoothingMode), how a usual group is set apart (help,
// UsualGroupRepresentation), the tabs of pages (help,
// FormPagesRepresentation - the prototype writes it as PagesRepresentation),
// a button and a popup (help, ButtonRepresentation), a button group (help,
// ButtonGroupRepresentation), and tables where they are described. The values allowed depend on the kind of element.
type FormElementRepresentation string

var (
	formSelectionModes = map[FormElementKind][]FormSelectionMode{
		FormElementCalendarField: {"single", "interval", "multiple"},
		FormElementTable:         {"single-row", "multi-row"},
	}
	formRepresentations = map[FormElementKind][]FormElementRepresentation{
		FormElementProgressBarField: {"smooth", "broken", "broken-tilt"},
		FormElementUsualGroup:       {"none", "weak-separation", "normal-separation", "strong-separation"},
		FormElementPages: {"auto", "none", "swipe", "tabs-on-top", "tabs-on-bottom", "tabs-on-left-horizontal",
			"tabs-on-right-horizontal"},
		FormElementPopup:       {"auto", "picture", "picture-and-text", "text"},
		FormElementButton:      {"auto", "picture", "picture-and-text", "text"},
		FormElementButtonGroup: {"auto", "compact", "usual"},
		FormElementTable:       {"list", "hierarchical-list", "tree"},
	}
)

// FieldOther is what a calendar, a progress bar and a track bar field have
// (help, the extensions of a form field for each). A chart, a Gantt chart, a
// planner and a graphical schema write nothing of their own in the exports
// beyond what they share with the fields of documents (FieldDocument).
type FieldOther struct {
	// ShowCurrentDate shows the line of the current date in a calendar; yes,
	// no or not said, as the help names no default and the prototype writes
	// only false (12 times).
	ShowCurrentDate *bool `yaml:"show_current_date,omitempty" json:"showCurrentDate,omitempty"`
	// WidthInMonths and HeightInMonths size a calendar in months: one by
	// default, and zero takes the width or height of the field instead - the
	// prototype writes zero 12 times, so zero is said and nil is not.
	WidthInMonths  *int `yaml:"width_in_months,omitempty" json:"widthInMonths,omitempty"`
	HeightInMonths *int `yaml:"height_in_months,omitempty" json:"heightInMonths,omitempty"`
	// ShowMonthsPanel shows the panel of months of a calendar; off by
	// default.
	ShowMonthsPanel bool              `yaml:"show_months_panel,omitempty" json:"showMonthsPanel,omitempty"`
	SelectionMode   FormSelectionMode `yaml:"selection_mode,omitempty" json:"selectionMode,omitempty"`
	// ShowPercent shows the percent in a progress bar; the prototype writes
	// only the "on" (65 times).
	ShowPercent    bool                      `yaml:"show_percent,omitempty" json:"showPercent,omitempty"`
	Representation FormElementRepresentation `yaml:"representation,omitempty" json:"representation,omitempty"`
	// Step, LargeStep and MarkingStep are how far a track bar moves on an
	// arrow key and on a page key, and how often it is marked: numbers, as
	// its bounds are.
	Step        FormNumber `yaml:"step,omitempty" json:"step,omitempty"`
	LargeStep   FormNumber `yaml:"large_step,omitempty" json:"largeStep,omitempty"`
	MarkingStep FormNumber `yaml:"marking_step,omitempty" json:"markingStep,omitempty"`
	// MarkingAppearance is on which side of a track bar its marks are drawn
	// (help, TrackBarMarkingAppearance). The help names no default, and the
	// prototype leaves it out on 8 track bars of 17, so empty is not said.
	MarkingAppearance FormMarkingAppearance `yaml:"marking_appearance,omitempty" json:"markingAppearance,omitempty"`
}

// FormMarkingAppearance is on which side of a track bar its marks are drawn
// (help, TrackBarMarkingAppearance).
type FormMarkingAppearance string

const (
	FormMarkingDontShow    FormMarkingAppearance = "dont-show"
	FormMarkingTopLeft     FormMarkingAppearance = "top-left"
	FormMarkingBottomRight FormMarkingAppearance = "bottom-right"
	FormMarkingBothSides   FormMarkingAppearance = "both-sides"
)

func validateFieldOther(path string, other FieldOther, kind FormElementKind) []string {
	var issues []string
	only := func(name string, set bool, what string, kinds ...FormElementKind) {
		if set && !slices.Contains(kinds, kind) {
			issues = append(issues, path+"."+name+" is allowed only for "+what)
		}
	}
	only("show_current_date", other.ShowCurrentDate != nil, "calendar fields", FormElementCalendarField)
	only("width_in_months", other.WidthInMonths != nil, "calendar fields", FormElementCalendarField)
	only("height_in_months", other.HeightInMonths != nil, "calendar fields", FormElementCalendarField)
	only("show_months_panel", other.ShowMonthsPanel, "calendar fields", FormElementCalendarField)
	only("show_percent", other.ShowPercent, "progress bar fields", FormElementProgressBarField)
	only("step", other.Step != "", "track bar fields", FormElementTrackBarField)
	only("large_step", other.LargeStep != "", "track bar fields", FormElementTrackBarField)
	only("marking_step", other.MarkingStep != "", "track bar fields", FormElementTrackBarField)
	only("marking_appearance", other.MarkingAppearance != "", "track bar fields", FormElementTrackBarField)
	for _, months := range []struct {
		name  string
		value *int
	}{{"width_in_months", other.WidthInMonths}, {"height_in_months", other.HeightInMonths}} {
		if months.value != nil && *months.value < 0 {
			issues = append(issues, path+"."+months.name+" must not be negative")
		}
	}
	issues = append(issues, oneOfKind(path+".selection_mode", other.SelectionMode, kind, formSelectionModes)...)
	issues = append(issues, oneOfKind(path+".representation", other.Representation, kind, formRepresentations)...)
	issues = append(issues, oneOf(path+".marking_appearance", other.MarkingAppearance,
		FormMarkingDontShow, FormMarkingTopLeft, FormMarkingBottomRight, FormMarkingBothSides)...)
	for _, step := range []struct {
		name  string
		value FormNumber
	}{{"step", other.Step}, {"large_step", other.LargeStep}, {"marking_step", other.MarkingStep}} {
		if step.value != "" && !decimalText.MatchString(string(step.value)) {
			issues = append(issues, path+"."+step.name+" must be a number written as decimal digits")
		}
	}
	return issues
}

// oneOfKind checks a value whose allowed values depend on the kind of the
// element it stands on.
func oneOfKind[T ~string](name string, value T, kind FormElementKind, allowed map[FormElementKind][]T) []string {
	if value == "" {
		return nil
	}
	values, ok := allowed[kind]
	if !ok {
		return []string{name + " is not a property of a " + string(kind)}
	}
	return oneOf(name, value, values...)
}

// FormGroupBehavior is how a usual group behaves (help, UsualGroupBehavior).
type FormGroupBehavior string

const (
	FormGroupBehaviorAuto        FormGroupBehavior = "auto"
	FormGroupBehaviorUsual       FormGroupBehavior = "usual"
	FormGroupBehaviorCollapsible FormGroupBehavior = "collapsible"
	FormGroupBehaviorPopUp       FormGroupBehavior = "popup"
)

// FormGroupControlRepresentation is how a collapsible group shows the control
// that collapses it (help, UsualGroupControlRepresentation).
type FormGroupControlRepresentation string

const (
	FormGroupControlPicture        FormGroupControlRepresentation = "picture"
	FormGroupControlTitleHyperlink FormGroupControlRepresentation = "title-hyperlink"
)

// FormUse is auto, use or do not use: the through alignment of the titles of
// a usual group (help, ThroughAlign) and the use of the current row of a
// table (help, CurrentRowUse).
type FormUse string

const (
	FormUseAuto    FormUse = "auto"
	FormUseYes     FormUse = "use"
	FormUseDontUse FormUse = "dont-use"
	// What the current row of a table does in the mobile client beside auto
	// (help, TableCurrentRowUse): offers a choice, shows the row selected,
	// or both.
	FormCurrentRowChoice                         FormUse = "choice"
	FormCurrentRowSelectionPresentation          FormUse = "selection-presentation"
	FormCurrentRowSelectionPresentationAndChoice FormUse = "selection-presentation-and-choice"
)

// FormShapeRepresentation is when the shape of a button or a popup is drawn
// (help, ButtonShapeRepresentation).
type FormShapeRepresentation string

const (
	FormShapeAuto       FormShapeRepresentation = "auto"
	FormShapeNone       FormShapeRepresentation = "none"
	FormShapeAlways     FormShapeRepresentation = "always"
	FormShapeWhenActive FormShapeRepresentation = "when-active"
)

// The sources of commands a command bar, a button group and a popup fill
// themselves from besides an element of the form.
const (
	FormCommandSourceForm           = "form"
	FormCommandSourceGlobalCommands = "global-commands"
)

// GroupProperties is what the groups have of their own (help, FormGroup and
// the extension of each group): how a usual group is set apart, collapses
// and lays out what it holds, what a page shows, what a group of columns
// shows in the header, how a popup is drawn, where the commands of a group
// of buttons come from, and what every group lets the user change.
type GroupProperties struct {
	// HideTitle hides the title of a usual group, a page and a group of
	// columns; the prototype writes only the "off" (36478, 1043 and 85
	// times).
	HideTitle bool              `yaml:"hide_title,omitempty" json:"hideTitle,omitempty"`
	Behavior  FormGroupBehavior `yaml:"behavior,omitempty" json:"behavior,omitempty"`
	// NotUnited lays the items of a usual group out in the group it stands
	// in, which then ignores all but its background and orientation (help,
	// United); the prototype writes only that (1848 times).
	NotUnited bool `yaml:"not_united,omitempty" json:"notUnited,omitempty"`
	// Collapsed is a collapsible group shown collapsed, CollapsedTitle the
	// title it shows so, ControlRepresentation the control that collapses it.
	// The prototype writes only collapsed (753 times).
	Collapsed             bool                           `yaml:"collapsed,omitempty" json:"collapsed,omitempty"`
	CollapsedTitle        LocalizedText                  `yaml:"collapsed_title,omitempty" json:"collapsedTitle,omitempty"`
	ControlRepresentation FormGroupControlRepresentation `yaml:"control_representation,omitempty" json:"controlRepresentation,omitempty"`
	// NoLeftMargin shows what a collapsible group holds without the margin on
	// the left; the prototype writes only that (270 times).
	NoLeftMargin bool `yaml:"no_left_margin,omitempty" json:"noLeftMargin,omitempty"`
	// ChildrenWidth, ItemsAndTitlesAlign and the spacings lay out what a usual
	// group and a page hold as the form lays out what it holds (FormLayout).
	ChildrenWidth       ChildrenWidth       `yaml:"children_width,omitempty" json:"childrenWidth,omitempty"`
	ItemsAndTitlesAlign ItemsAndTitlesAlign `yaml:"items_and_titles_align,omitempty" json:"itemsAndTitlesAlign,omitempty"`
	HorizontalSpacing   ItemSpacing         `yaml:"horizontal_spacing,omitempty" json:"horizontalSpacing,omitempty"`
	VerticalSpacing     ItemSpacing         `yaml:"vertical_spacing,omitempty" json:"verticalSpacing,omitempty"`
	// ThroughAlign lines up the titles of a usual group with those around it.
	ThroughAlign FormUse `yaml:"through_align,omitempty" json:"throughAlign,omitempty"`
	// TitleDataPath is the attribute shown in the title of a usual group or
	// a page, written as the data path of a field is.
	TitleDataPath string `yaml:"title_data_path,omitempty" json:"titleDataPath,omitempty"`
	// Picture is drawn on the tab of a page, on a popup and on a button, and
	// is what a picture decoration shows.
	Picture *PictureReference `yaml:"picture,omitempty" json:"picture,omitempty"`
	// ScrollOnCompress scrolls a page whose content is higher than the page;
	// yes, no or not said, as the help gives Undefined beside the two and the
	// prototype writes true (45 times).
	ScrollOnCompress *bool `yaml:"scroll_on_compress,omitempty" json:"scrollOnCompress,omitempty"`
	// EnableContentChange lets the user change what a group holds (help,
	// FormGroup - a command bar is a group too); the prototype writes only
	// the "on" (1240 times, on every group and on command bars).
	EnableContentChange bool `yaml:"enable_content_change,omitempty" json:"enableContentChange,omitempty"`
	// CurrentRowUse hides a usual group or pages in the mobile client and
	// shows them from the context menu of a row of AssociatedTable, a table of
	// the same form named by its element. On a table it is what the current
	// row does in the mobile client (help, TableCurrentRowUse), with values
	// of its own.
	CurrentRowUse   FormUse `yaml:"current_row_use,omitempty" json:"currentRowUse,omitempty"`
	AssociatedTable string  `yaml:"associated_table,omitempty" json:"associatedTable,omitempty"`
	// ShowInHeader shows a group of columns in the header of its table; off
	// by default, and the prototype writes only the "on" (1046 times) - the
	// other way round from a field, whose header is shown unless hidden.
	ShowInHeader bool `yaml:"show_in_header,omitempty" json:"showInHeader,omitempty"`
	// ShapeRepresentation is when the shape of a popup or a button is drawn.
	ShapeRepresentation FormShapeRepresentation `yaml:"shape_representation,omitempty" json:"shapeRepresentation,omitempty"`
	// CommandSource is where a command bar, a button group or a popup takes
	// the commands it fills itself with: the form, the global commands of
	// the form's command bar, or a field or a table of the same form,
	// written "Items.<name>". The help does not name it; the prototype writes
	// Form, FormCommandPanelGlobalCommands and Item.<name> (1740, 794 and 847
	// times), and the code of an element (8, see validateFormLinkPath), which
	// is carried as written.
	CommandSource string `yaml:"command_source,omitempty" json:"commandSource,omitempty"`
}

// commandSourceItem is the name of the element a source of commands names,
// if it names one.
func (group GroupProperties) commandSourceItem() (string, bool) {
	return strings.CutPrefix(group.CommandSource, "Items.")
}

func validateGroupProperties(path string, group GroupProperties, kind FormElementKind, configuration project.Project) []string {
	var issues []string
	only := func(name string, set bool, what string, kinds ...FormElementKind) {
		if set && !slices.Contains(kinds, kind) {
			issues = append(issues, path+"."+name+" is allowed only for "+what)
		}
	}
	usual := []FormElementKind{FormElementUsualGroup}
	areas := []FormElementKind{FormElementUsualGroup, FormElementPage}
	for _, property := range []struct {
		name string
		set  bool
	}{
		{"behavior", group.Behavior != ""}, {"not_united", group.NotUnited}, {"collapsed", group.Collapsed},
		{"collapsed_title", len(group.CollapsedTitle) != 0}, {"control_representation", group.ControlRepresentation != ""},
		{"no_left_margin", group.NoLeftMargin}, {"through_align", group.ThroughAlign != ""},
	} {
		only(property.name, property.set, "usual groups", usual...)
	}
	for _, property := range []struct {
		name string
		set  bool
	}{
		{"children_width", group.ChildrenWidth != ""}, {"items_and_titles_align", group.ItemsAndTitlesAlign != ""},
		{"horizontal_spacing", group.HorizontalSpacing != ""}, {"vertical_spacing", group.VerticalSpacing != ""},
		{"title_data_path", group.TitleDataPath != ""},
	} {
		only(property.name, property.set, "usual groups and pages", areas...)
	}
	only("hide_title", group.HideTitle, "usual groups, pages and groups of columns", FormElementUsualGroup, FormElementPage, FormElementColumnGroup)
	only("picture", group.Picture != nil, "pages, popups, buttons and pictures", FormElementPage, FormElementPopup, FormElementButton, FormElementPictureDecoration)
	only("scroll_on_compress", group.ScrollOnCompress != nil, "pages", FormElementPage)
	only("enable_content_change", group.EnableContentChange, "groups", formGroupKinds...)
	only("current_row_use", group.CurrentRowUse != "", "usual groups, pages and tables", FormElementUsualGroup, FormElementPages, FormElementTable)
	only("associated_table", group.AssociatedTable != "", "usual groups and pages", FormElementUsualGroup, FormElementPages)
	only("show_in_header", group.ShowInHeader, "groups of columns", FormElementColumnGroup)
	only("shape_representation", group.ShapeRepresentation != "", "popups and buttons", FormElementPopup, FormElementButton)
	only("command_source", group.CommandSource != "", "command bars, button groups and popups", FormElementCommandBar,
		FormElementButtonGroup, FormElementPopup)
	issues = append(issues, oneOf(path+".behavior", group.Behavior, FormGroupBehaviorAuto, FormGroupBehaviorUsual,
		FormGroupBehaviorCollapsible, FormGroupBehaviorPopUp)...)
	issues = append(issues, validateTitle(path+".collapsed_title", group.CollapsedTitle, configuration)...)
	issues = append(issues, oneOf(path+".control_representation", group.ControlRepresentation, FormGroupControlPicture, FormGroupControlTitleHyperlink)...)
	issues = append(issues, oneOf(path+".children_width", group.ChildrenWidth, ChildrenWidthAuto, ChildrenWidthEqual,
		ChildrenWidthLeftNarrowest, ChildrenWidthLeftNarrow, ChildrenWidthLeftWide, ChildrenWidthLeftWidest)...)
	issues = append(issues, oneOf(path+".items_and_titles_align", group.ItemsAndTitlesAlign, ItemsAndTitlesAuto, ItemsAndTitlesNone,
		ItemsLeftTitlesLeft, ItemsLeftTitlesRight, ItemsRightTitlesLeft, ItemsRightTitlesRight, ItemsAndTitlesTitlesLeftDataAuto)...)
	spacings := []ItemSpacing{ItemSpacingAuto, ItemSpacingNone, ItemSpacingHalf, ItemSpacingSingle, ItemSpacingOneAndHalf, ItemSpacingDouble}
	issues = append(issues, oneOf(path+".horizontal_spacing", group.HorizontalSpacing, spacings...)...)
	issues = append(issues, oneOf(path+".vertical_spacing", group.VerticalSpacing, spacings...)...)
	issues = append(issues, oneOf(path+".through_align", group.ThroughAlign, FormUseAuto, FormUseYes, FormUseDontUse)...)
	if group.TitleDataPath != "" {
		issues = append(issues, validateElementDataPath(path+".title_data_path", group.TitleDataPath)...)
	}
	issues = append(issues, validateElementPictureReference(path+".picture", group.Picture)...)
	if kind == FormElementTable {
		issues = append(issues, oneOf(path+".current_row_use", group.CurrentRowUse, FormUseAuto, FormCurrentRowChoice,
			FormCurrentRowSelectionPresentation, FormCurrentRowSelectionPresentationAndChoice)...)
	} else {
		issues = append(issues, oneOf(path+".current_row_use", group.CurrentRowUse, FormUseAuto, FormUseYes, FormUseDontUse)...)
	}
	if group.AssociatedTable != "" && (!validIdentifier(group.AssociatedTable) || utf8.RuneCountInString(group.AssociatedTable) > maxNameLength) {
		issues = append(issues, path+".associated_table must be the name of a table of the form")
	}
	issues = append(issues, oneOf(path+".shape_representation", group.ShapeRepresentation, FormShapeAuto, FormShapeNone, FormShapeAlways, FormShapeWhenActive)...)
	if name, item := group.commandSourceItem(); item {
		if !validIdentifier(name) || utf8.RuneCountInString(name) > maxNameLength {
			issues = append(issues, path+".command_source must name an element of the form after Items.")
		}
	} else if group.CommandSource != "" && group.CommandSource != FormCommandSourceForm && group.CommandSource != FormCommandSourceGlobalCommands &&
		!formElementCode.MatchString(group.CommandSource) {
		issues = append(issues, path+".command_source must be form, global-commands, Items.<name> or the code of an element of a form")
	}
	return issues
}

// FormLocationInCommandBar is where a button of a command bar stands (help,
// ButtonLocationInCommandBar).
type FormLocationInCommandBar string

const (
	FormLocationInCommandBarAuto                   FormLocationInCommandBar = "auto"
	FormLocationInCommandBarInCommandBar           FormLocationInCommandBar = "in-command-bar"
	FormLocationInAdditionalSubmenu                FormLocationInCommandBar = "in-additional-submenu"
	FormLocationInCommandBarAndInAdditionalSubmenu FormLocationInCommandBar = "in-command-bar-and-in-additional-submenu"
)

// FormRepresentationInContextMenu is whether a button of a command bar is
// shown in the context menu too. The help does not name it; the prototype
// writes these three (405, 55 and 28 times), and leaves it out otherwise.
type FormRepresentationInContextMenu string

const (
	FormInContextMenuNone       FormRepresentationInContextMenu = "none"
	FormInContextMenuOnly       FormRepresentationInContextMenu = "only-in-context-menu"
	FormInContextMenuAdditional FormRepresentationInContextMenu = "additional-in-context-menu"
)

// FormButtonShape is the shape of a button (help, ButtonShape).
type FormButtonShape string

const (
	FormButtonShapeAuto  FormButtonShape = "auto"
	FormButtonShapeUsual FormButtonShape = "usual"
	FormButtonShapeOval  FormButtonShape = "oval"
)

// FormPictureLocation is where the picture of a button stands against its
// text (help, FormButtonPictureLocation).
type FormPictureLocation string

const (
	FormPictureLocationAuto  FormPictureLocation = "auto"
	FormPictureLocationLeft  FormPictureLocation = "left"
	FormPictureLocationRight FormPictureLocation = "right"
)

// ButtonProperties is what a button has of its own (help, FormButton),
// besides its type, its picture, how it is drawn and its shape, which it
// shares with a popup, and what it shares with a field.
type ButtonProperties struct {
	// Check shows a button pressed; the prototype writes only the "on" (57
	// times).
	Check bool `yaml:"check,omitempty" json:"check,omitempty"`
	// DefaultButton is pressed by the Enter key of the form; the prototype
	// writes only the "on" (4038 times).
	DefaultButton               bool                            `yaml:"default_button,omitempty" json:"defaultButton,omitempty"`
	LocationInCommandBar        FormLocationInCommandBar        `yaml:"location_in_command_bar,omitempty" json:"locationInCommandBar,omitempty"`
	RepresentationInContextMenu FormRepresentationInContextMenu `yaml:"representation_in_context_menu,omitempty" json:"representationInContextMenu,omitempty"`
	Shape                       FormButtonShape                 `yaml:"shape,omitempty" json:"shape,omitempty"`
	PictureLocation             FormPictureLocation             `yaml:"picture_location,omitempty" json:"pictureLocation,omitempty"`
}

func validateButtonProperties(path string, properties ButtonProperties, kind FormElementKind) []string {
	if properties == (ButtonProperties{}) {
		return nil
	}
	if kind != FormElementButton {
		return []string{path + " has what only a button has: check, default_button, location_in_command_bar, representation_in_context_menu, shape, picture_location"}
	}
	var issues []string
	issues = append(issues, oneOf(path+".location_in_command_bar", properties.LocationInCommandBar, FormLocationInCommandBarAuto,
		FormLocationInCommandBarInCommandBar, FormLocationInAdditionalSubmenu, FormLocationInCommandBarAndInAdditionalSubmenu)...)
	issues = append(issues, oneOf(path+".representation_in_context_menu", properties.RepresentationInContextMenu, FormInContextMenuNone,
		FormInContextMenuOnly, FormInContextMenuAdditional)...)
	issues = append(issues, oneOf(path+".shape", properties.Shape, FormButtonShapeAuto, FormButtonShapeUsual, FormButtonShapeOval)...)
	issues = append(issues, oneOf(path+".picture_location", properties.PictureLocation, FormPictureLocationAuto, FormPictureLocationLeft, FormPictureLocationRight)...)
	return issues
}
