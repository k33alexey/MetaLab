package metadata

import (
	"strings"
	"unicode/utf8"

	"github.com/k33alexey/MetaLab/internal/project"
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
