package metadata

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// FormEvent is an event of a form or of an element of a form a procedure of
// the module of the form handles, spelled as the model spells an event
// (before-write); the prototype writes it in English (BeforeWrite).
//
// The prototype writes an event in another way too: by an identifier of its
// own (64 times in the exports), on a form whose main attribute does not
// raise it - a data processor with a handler of writing on the server, a
// form with no main attribute that reads its object - and once on a field of
// a spreadsheet document. By the name of its handler it is an event a form
// of another kind has: what is left after the main attribute changed. It is
// carried as written and noted (NoteEventByIdentifier).
type FormEvent string

// formElementEvents are the events each kind of element raises (help, the
// events of FormField, FormTable, the decorations, pages and the extension
// of each field), which are the events the exports write on them. A button,
// a group other than pages, an addition and a command bar raise none.
var formElementEvents = map[FormElementKind][]FormEvent{
	FormElementInputField: {
		"auto-complete", "choice-processing", "clearing", "command-generate-processing", "creating", "edit-text-change",
		"multiple-value-opening", "multiple-value-url-processing", "multiple-values-add", "multiple-values-delete", "on-change",
		"opening", "start-choice", "start-list-choice", "text-edit-end", "tuning",
	},
	FormElementLabelField: {
		"click", "on-change", "url-processing",
	},
	FormElementPictureField: {
		"click", "drag", "drag-check", "drag-end", "drag-start", "on-change",
	},
	FormElementSpreadsheetDocumentField: {
		"additional-detail-processing", "after-write", "before-print", "before-write", "detail-processing", "drag", "drag-check",
		"drag-end", "drag-start", "on-activate", "on-change", "on-change-area-content", "selection", "url-processing",
	},
	FormElementTextDocumentField: {
		"after-write", "before-print", "before-write", "on-change",
	},
	FormElementHTMLDocumentField: {
		"after-write", "before-print", "before-write", "document-complete", "on-change", "on-click",
	},
	FormElementFormattedDocumentField: {
		"after-write", "before-print", "before-write", "on-change",
	},
	FormElementChartField: {
		"detail-processing", "on-activate", "on-change", "selection",
	},
	FormElementCalendarField: {
		"drag", "drag-check", "drag-end", "drag-start", "on-activate-date", "on-change", "on-period-output", "selection",
	},
	FormElementGanttChartField: {
		"before-collapse", "before-expand", "detail-processing", "on-activate-interval", "on-activate-value", "on-change",
		"on-interval-edit-end", "selection",
	},
	FormElementPlannerField: {
		"before-collapse-dimension-item", "before-create", "before-delete", "before-expand-dimension-item", "before-print",
		"before-start-edit", "before-start-quick-edit", "command-generate-processing", "dimension-item-click", "drag", "drag-check",
		"drag-end", "drag-start", "inside-drag-check", "on-activate", "on-change", "on-current-representation-period-change",
		"on-edit-end", "planner-action-click", "selection", "time-scale-item-click", "url-click", "wrapped-time-scale-header-click",
	},
	FormElementGraphicalSchemaField: {
		"after-write", "before-print", "before-write", "on-activate", "on-change", "selection",
	},
	FormElementPeriodField: {
		"on-change", "selection",
	},
	FormElementPDFDocumentField: {
		"on-change", "url-click",
	},
	FormElementGeographicalSchemaField: {
		"after-write", "before-print", "before-write", "detail-processing", "on-change",
	},
	FormElementDendrogramField: {
		"detail-processing", "on-change", "selection",
	},
	FormElementCheckBoxField: {
		"on-change",
	},
	FormElementRadioButtonField: {
		"on-change",
	},
	FormElementProgressBarField: {
		"on-change",
	},
	FormElementTrackBarField: {
		"on-change",
	},
	FormElementLabelDecoration: {
		"click", "url-processing",
	},
	FormElementPictureDecoration: {
		"click", "drag", "drag-check", "drag-end", "drag-start",
	},
	FormElementTable: {
		"after-delete-row", "before-add-row", "before-collapse", "before-delete-row", "before-edit-end", "before-expand",
		"before-load-user-settings-at-server", "before-row-change", "choice-processing", "drag", "drag-check", "drag-end",
		"drag-start", "new-write-processing", "on-activate-cell", "on-activate-field", "on-activate-row", "on-change",
		"on-current-parent-change", "on-edit-end", "on-get-data-at-server", "on-load-user-settings-at-server",
		"on-save-user-settings-at-server", "on-start-edit", "on-update-user-setting-set-at-server", "refresh-request-processing",
		"selection", "url-get-processing", "url-list-get-processing", "value-choice",
	},
	FormElementPages: {
		"on-current-page-change",
	},
}

// formEventsOfEveryForm are the events every managed form raises (help,
// ClientApplicationForm).
var formEventsOfEveryForm = []FormEvent{
	"activation-processing", "add-in-detachment-on-error", "before-close", "before-load-data-from-settings-at-server",
	"before-reopen-from-other-server", "choice-processing", "collaboration-system-users-auto-complete",
	"collaboration-system-users-choice-form-get-processing", "external-event", "fill-check-processing-at-server",
	"navigation-processing", "new-write-processing", "notification-processing", "on-change-display-settings", "on-close",
	"on-create-at-server", "on-load-data-from-settings-at-server", "on-main-server-availability-change", "on-open",
	"on-paste-from-clipboard", "on-reopen", "on-reopen-from-other-server", "on-save-data-in-settings-at-server",
	"url-get-processing", "url-list-get-processing", "url-processing",
}

// formEventsByMainAttribute are the events a form raises besides, by the type
// of its main attribute (help, the extension of a managed form for each).
// A form of a data processor or of a dynamic list, or one with no main
// attribute, raises none besides.
var formEventsByMainAttribute = map[TypeKind][]FormEvent{
	CatalogObjectType: {
		"after-write", "after-write-at-server", "before-write", "before-write-at-server", "on-read-at-server", "on-write-at-server",
		"value-choice",
	},
	DocumentObjectType: {
		"after-write", "after-write-at-server", "before-write", "before-write-at-server", "on-read-at-server", "on-write-at-server",
		"value-choice",
	},
	CharacteristicTypesObjectType: {
		"after-write", "after-write-at-server", "before-write", "before-write-at-server", "on-read-at-server", "on-write-at-server",
		"value-choice",
	},
	BusinessProcessObjectType: {
		"activation-processing", "after-write", "after-write-at-server", "before-start", "before-write", "before-write-at-server",
		"on-read-at-server", "on-write-at-server", "value-choice",
	},
	TaskObjectType: {
		"activation-processing", "after-write", "after-write-at-server", "before-execute", "before-write", "before-write-at-server",
		"on-read-at-server", "on-write-at-server", "value-choice",
	},
	InformationRegisterRecordManagerType: {
		"after-write", "after-write-at-server", "before-write", "before-write-at-server", "on-read-at-server", "on-write-at-server",
	},
	ConstantsSetType: {
		"after-write", "after-write-at-server", "before-write", "before-write-at-server", "on-read-at-server", "on-write-at-server",
	},
	ReportObjectType: {
		"before-load-user-settings-at-server", "before-load-variant-at-server", "on-load-user-settings-at-server",
		"on-load-variant-at-server", "on-save-user-settings-at-server", "on-save-variant-at-server",
		"on-update-user-setting-set-at-server",
	},
	SettingsComposerType: {
		"on-update-user-setting-set-at-server",
	},
	ExchangePlanObjectType: {
		"after-write", "after-write-at-server", "before-write", "before-write-at-server", "on-read-at-server", "on-write-at-server",
		"value-choice",
	},
	AccountsObjectType: {
		"after-write", "after-write-at-server", "before-write", "before-write-at-server", "on-read-at-server", "on-write-at-server",
		"value-choice",
	},
	CalculationTypesObjectType: {
		"after-write", "after-write-at-server", "before-write", "before-write-at-server", "on-read-at-server", "on-write-at-server",
		"value-choice",
	},
	InformationRegisterRecordSetType: {
		"after-write", "after-write-at-server", "before-write", "before-write-at-server", "on-read-at-server", "on-write-at-server",
	},
	AccumulationRegisterRecordSetType: {
		"after-write", "after-write-at-server", "before-write", "before-write-at-server", "on-read-at-server", "on-write-at-server",
	},
	AccountingRegisterRecordSetType: {
		"after-write", "after-write-at-server", "before-write", "before-write-at-server", "on-read-at-server", "on-write-at-server",
	},
	CalculationRegisterRecordSetType: {
		"after-write", "after-write-at-server", "before-write", "before-write-at-server", "on-read-at-server", "on-write-at-server",
	},
}

// formEvents are the events a managed form of any kind raises: those of
// every form and those of every main attribute. A form is checked against
// all of them, not against its own kind: the prototype writes by name an
// event its main attribute does not raise as well (14 times in the
// exports), and such an event is noted at load (NoteFormEventNotRaised).
var formEvents = []FormEvent{
	"activation-processing", "add-in-detachment-on-error", "after-write", "after-write-at-server", "before-close", "before-execute",
	"before-load-data-from-settings-at-server", "before-load-user-settings-at-server", "before-load-variant-at-server",
	"before-reopen-from-other-server", "before-start", "before-write", "before-write-at-server", "choice-processing",
	"collaboration-system-users-auto-complete", "collaboration-system-users-choice-form-get-processing", "external-event",
	"fill-check-processing-at-server", "navigation-processing", "new-write-processing", "notification-processing",
	"on-change-display-settings", "on-close", "on-create-at-server", "on-load-data-from-settings-at-server",
	"on-load-user-settings-at-server", "on-load-variant-at-server", "on-main-server-availability-change", "on-open",
	"on-paste-from-clipboard", "on-read-at-server", "on-reopen", "on-reopen-from-other-server",
	"on-save-data-in-settings-at-server", "on-save-user-settings-at-server", "on-save-variant-at-server",
	"on-update-user-setting-set-at-server", "on-write-at-server", "url-get-processing", "url-list-get-processing", "url-processing",
	"value-choice",
}

// validateFormEvents checks the events of a form or of an element: each is
// an event the holder raises, or the identifier of one written in its stead,
// and is handled by a procedure named as a procedure is.
func validateFormEvents(path string, events map[FormEvent]string, allowed []FormEvent, holder string) []string {
	var issues []string
	keys := make([]FormEvent, 0, len(events))
	for event := range events {
		keys = append(keys, event)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	for _, event := range keys {
		switch {
		case eventByIdentifier(event):
		case !slices.Contains(allowed, event):
			issues = append(issues, fmt.Sprintf("%s.events.%s is not an event of %s", path, event, holder))
		}
		if handler := events[event]; !validIdentifier(handler) || utf8.RuneCountInString(handler) > maxNameLength {
			issues = append(issues, fmt.Sprintf("%s.events.%s must name a procedure of the module of the form", path, event))
		}
	}
	return issues
}

// eventByIdentifier says the event is written as the prototype writes one
// it does not name: by its identifier, in lower case.
func eventByIdentifier(event FormEvent) bool {
	id, err := uuid.Parse(string(event))
	return err == nil && !id.IsZero() && id.String() == string(event)
}

// noteFormEvents notes the events of a form written by an identifier, and
// those its main attribute does not raise: their handlers are never called.
func (catalog *Catalog) noteFormEvents(where string, form ManagedForm) {
	raised := slices.Clone(formEventsOfEveryForm)
	for _, attribute := range form.Attributes {
		if single, ok := SingleType(attribute.Types); attribute.Main && ok {
			raised = append(raised, formEventsByMainAttribute[single.Kind]...)
		}
	}
	for _, event := range sortedEvents(form.Events) {
		switch {
		case eventByIdentifier(event):
			catalog.noteForm(NoteEventByIdentifier, where+" event "+string(event), form.Events[event])
		case !slices.Contains(raised, event):
			catalog.noteForm(NoteFormEventNotRaised, where+" event "+string(event), form.Events[event])
		}
	}
	var walk func(items []ManagedFormElement)
	walk = func(items []ManagedFormElement) {
		for _, item := range items {
			for _, event := range sortedEvents(item.Events) {
				if eventByIdentifier(event) {
					catalog.noteForm(NoteEventByIdentifier, where+" element "+item.Name+" event "+string(event), item.Events[event])
				}
			}
			walk(item.Nested())
		}
	}
	walk(form.FormItems())
}

func sortedEvents(events map[FormEvent]string) []FormEvent {
	keys := make([]FormEvent, 0, len(events))
	for event := range events {
		keys = append(keys, event)
	}
	sort.Slice(keys, func(i, j int) bool { return strings.Compare(string(keys[i]), string(keys[j])) < 0 })
	return keys
}
