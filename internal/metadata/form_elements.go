package metadata

import (
	"fmt"
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
// tooltip shows, and whom it is shown to.
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

func validateFormField(path string, field FieldBehavior, class formElementClass, configuration project.Project) []string {
	if field.empty() {
		return nil
	}
	if class != formFieldClass {
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
// are: a number, which is no attribute of the form (5 times), and two paths
// joined by "~" (4 times); what they mean is not known.
func validateElementDataPath(path, value string) []string {
	if value == "" || strings.TrimSpace(value) != value {
		return []string{path + " must be a data path without surrounding spaces"}
	}
	if allDigits(value) {
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

func validateFieldLayout(path string, layout FieldLayout, class formElementClass) []string {
	if layout == (FieldLayout{}) {
		return nil
	}
	if class != formFieldClass {
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

func validateFieldLook(path string, look FieldLook, class formElementClass) []string {
	if look == (FieldLook{}) {
		return nil
	}
	if class != formFieldClass {
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
		if reference != nil && reference.Item != nil {
			uses = append(uses, styleItemUse{name: name, itemType: itemType, id: *reference.Item})
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

func validateFieldColumn(path string, column FieldColumn, class formElementClass, configuration project.Project) []string {
	if column.empty() {
		return nil
	}
	if class != formFieldClass {
		return []string{path + " has what a field has as a column of a table"}
	}
	var issues []string
	for _, picture := range []struct {
		name  string
		value *PictureReference
	}{{"header_picture", column.HeaderPicture}, {"footer_picture", column.FooterPicture}} {
		issues = append(issues, validatePictureReference(path+"."+picture.name, picture.value)...)
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
	if font := column.FooterFont; font != nil && font.Source == StyleFont && font.From != nil && font.From.Item != nil {
		uses = append(uses, styleItemUse{name: "footer_font", itemType: FontStyleItem, id: *font.From.Item})
	}
	for _, color := range column.colors() {
		if color.value != nil && color.value.Source == StyleColor && color.value.From != nil && color.value.From.Item != nil {
			uses = append(uses, styleItemUse{name: color.name, itemType: ColorStyleItem, id: *color.value.From.Item})
		}
	}
	return uses
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
	issues = append(issues, validatePictureReference(path+".choice_button_picture", buttons.ChoiceButtonPicture)...)
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
}

func (input FieldTextInput) empty() bool {
	return !input.NoWrap && !input.NoTextEdit && input.MultiLine == nil && input.ExtendedEdit == nil && input.PasswordMode == nil &&
		input.Mask == "" && len(input.InputHint) == 0 && input.EditTextUpdate == "" && input.SpecialTextInputMode == "" &&
		input.SpellChecking == "" && input.AutoCorrection == "" && input.HeightControlVariant == ""
}

func validateFieldTextInput(path string, input FieldTextInput, kind FormElementKind, configuration project.Project) []string {
	if input.empty() {
		return nil
	}
	if kind != FormElementInputField {
		rest := input
		if kind == FormElementLabelField {
			rest.PasswordMode = nil
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
	issues = append(issues, oneOf(path+".height_control_variant", input.HeightControlVariant,
		FormHeightControlAuto, FormHeightControlUseContentHeight, FormHeightControlUseHeightInFormRows)...)
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
	// value of its type. Each is yes, no or not said: the help gives each
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
	only("format", len(format.Format) != 0, "input and label fields", FormElementInputField, FormElementLabelField)
	only("edit_format", len(format.EditFormat) != 0, "input fields and check boxes", FormElementInputField, FormElementCheckBoxField)
	bounded := []FormElementKind{FormElementInputField, FormElementTrackBarField, FormElementProgressBarField}
	only("min_value", format.MinValue != "", "input fields, track bars and progress bars", bounded...)
	only("max_value", format.MaxValue != "", "input fields, track bars and progress bars", bounded...)
	only("mark_negatives", format.MarkNegatives != nil, "input and label fields", FormElementInputField, FormElementLabelField)
	only("auto_mark_incomplete", format.AutoMarkIncomplete != nil, "input fields", FormElementInputField)
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
// (13), "342:02023637-…/15" (2), "1/0:ba7dcb3b-…" (15). What the last two
// point at is an open question of the map of blocks; they are carried as
// written.
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
	only("hyperlink", view.Hyperlink, "label and picture fields", FormElementLabelField, FormElementPictureField)
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
}

func (picture FieldPicture) empty() bool {
	return picture.ValuesPicture == nil && picture.PictureSize == "" && len(picture.NonselectedPictureText) == 0 &&
		!picture.Zoomable && picture.FileDragMode == ""
}

func validateFieldPicture(path string, picture FieldPicture, kind FormElementKind, configuration project.Project) []string {
	if picture.empty() {
		return nil
	}
	if kind != FormElementPictureField {
		return []string{path + " has what only a picture field has"}
	}
	var issues []string
	issues = append(issues, validatePictureReference(path+".values_picture", picture.ValuesPicture)...)
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
// true is ScrollAlways (2). False is not among them and is read as never, the
// one use left; the import translates both, and that last is to be checked on
// the platform. Empty is not said: auto.
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
// document have (help, the extensions of a form field for each). Each
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
		{"vertical_scroll_bar", document.VerticalScrollBar != ""}, {"horizontal_scroll_bar", document.HorizontalScrollBar != ""},
		{"view_scaling_mode", document.ViewScalingMode != ""}, {"selection_show_mode", document.SelectionShowMode != ""},
		{"edit", document.Edit != nil}, {"protection", document.Protection != nil}, {"show_headers", document.ShowHeaders != nil},
		{"show_grid", document.ShowGrid != nil}, {"show_groups", document.ShowGroups != nil}, {"show_cell_names", document.ShowCellNames != nil},
		{"show_row_and_column_names", document.ShowRowAndColumnNames != nil},
		{"enable_drag", document.EnableDrag != nil}, {"enable_start_drag", document.EnableStartDrag != nil},
	} {
		only(property.name, property.set, "spreadsheet document fields", spreadsheet...)
	}
	only("output", document.Output != "", "spreadsheet, text, HTML and formatted document fields", FormElementSpreadsheetDocumentField,
		FormElementTextDocumentField, FormElementHTMLDocumentField, FormElementFormattedDocumentField)
	only("excluded_commands", len(document.ExcludedCommands) != 0, "spreadsheet and formatted document fields",
		FormElementSpreadsheetDocumentField, FormElementFormattedDocumentField)
	scrollBars := []FormScrollBarUse{FormScrollBarAutoUse, FormScrollBarUseAlways, FormScrollBarDontUse}
	issues = append(issues, oneOf(path+".vertical_scroll_bar", document.VerticalScrollBar, scrollBars...)...)
	issues = append(issues, oneOf(path+".horizontal_scroll_bar", document.HorizontalScrollBar, scrollBars...)...)
	issues = append(issues, oneOf(path+".view_scaling_mode", document.ViewScalingMode, FormViewScalingAuto, FormViewScalingNormal, FormViewScalingLarge)...)
	issues = append(issues, oneOf(path+".selection_show_mode", document.SelectionShowMode, FormSelectionAlways, FormSelectionDontShow,
		FormSelectionWhenActive, FormSelectionWhenMultipleCellsSelected, FormSelectionWhenMultipleCellsSelectedWhenActive)...)
	issues = append(issues, oneOf(path+".output", document.Output, FormUseOutputAuto, FormUseOutputEnable, FormUseOutputDisable)...)
	seen := map[string]bool{}
	for index, command := range document.ExcludedCommands {
		switch {
		case !validIdentifier(command):
			issues = append(issues, fmt.Sprintf("%s.excluded_commands[%d] must be the name of a command", path, index))
		case seen[command]:
			issues = append(issues, fmt.Sprintf("%s.excluded_commands[%d] names %s twice", path, index, command))
		}
		seen[command] = true
	}
	return issues
}
