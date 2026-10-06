package metadata

import "github.com/k33alexey/MetaLab/internal/project"

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
