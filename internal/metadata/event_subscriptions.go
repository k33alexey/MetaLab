package metadata

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

const EventSubscriptionKind Kind = "event-subscriptions"

// EventSubscriptionDefinition attaches one exported procedure of a common
// module to an event of the objects its source names, without those objects'
// own modules implementing the handler. The procedure receives the source
// object first, then the arguments the object's own module would receive for
// that event - the signature a subscription handler has in the prototype.
//
// The source is a type description, as the prototype's is, not a list of
// objects: the object of a catalog or a document, the record set of a
// register, the manager of a constant's value or of any kind, a kind as a
// whole (a type without a reference: every document, every exchange plan),
// and defined types of such. The configurations being moved give their
// subscriptions some 16 800 such types and 293 sets; a list of catalogs,
// documents and two kinds of register, as it was, lost most of them on import.
type EventSubscriptionDefinition struct {
	Format    int           `yaml:"format"`
	ID        uuid.UUID     `yaml:"id"`
	Name      string        `yaml:"name"`
	Title     LocalizedText `yaml:"title"`
	Source    []Type        `yaml:"source"`
	Event     string        `yaml:"event"`
	Module    uuid.UUID     `yaml:"module"`
	Procedure string        `yaml:"procedure"`
	Comment   string        `yaml:"comment,omitempty"`
}

// subscriptionEvents are the events of every kind a subscription can name, as
// the syntax assistant 8.3.27 lists them on the object, record set and manager
// of each kind, in the prototype's own names. A subscription names one of them
// in the kebab spelling of eventSubscriptionName; the event has to be one the
// kind has, for every kind of the source.
var subscriptionEvents = map[TypeKind][]string{
	CatalogObjectType:                 {"BeforeDelete", "BeforeWrite", "FillCheckProcessing", "Filling", "GenerateFromDataHistoryVersionProcessing", "OnCopy", "OnSetNewCode", "OnWrite"},
	DocumentObjectType:                {"BeforeDelete", "BeforeWrite", "FillCheckProcessing", "Filling", "GenerateFromDataHistoryVersionProcessing", "OnCopy", "OnSetNewNumber", "OnWrite", "Posting", "UndoPosting"},
	CharacteristicTypesObjectType:     {"BeforeDelete", "BeforeWrite", "FillCheckProcessing", "Filling", "GenerateFromDataHistoryVersionProcessing", "OnCopy", "OnSetNewCode", "OnWrite"},
	AccountsObjectType:                {"BeforeDelete", "BeforeWrite", "FillCheckProcessing", "Filling", "GenerateFromDataHistoryVersionProcessing", "OnCopy", "OnWrite"},
	CalculationTypesObjectType:        {"BeforeDelete", "BeforeWrite", "FillCheckProcessing", "Filling", "GenerateFromDataHistoryVersionProcessing", "OnCopy", "OnWrite"},
	ExchangePlanObjectType:            {"BeforeBeginSendDataToMaster", "BeforeBeginSendDataToSlave", "BeforeCreateInitialImage", "BeforeDelete", "BeforeWrite", "FillCheckProcessing", "Filling", "GenerateFromDataHistoryVersionProcessing", "OnAutoCreateNewNode", "OnCopy", "OnReceiveDataFromMaster", "OnReceiveDataFromSlave", "OnReceiveNodeDataFromMaster", "OnSendDataToMaster", "OnSendDataToSlave", "OnSendNodeDataToSlave", "OnSetNewCode", "OnWrite"},
	BusinessProcessObjectType:         {"BeforeDelete", "BeforeWrite", "FillCheckProcessing", "Filling", "GenerateFromDataHistoryVersionProcessing", "InteractiveActivationProcessing", "OnCopy", "OnSetNewNumber", "OnWrite"},
	TaskObjectType:                    {"BeforeDelete", "BeforeExecute", "BeforeExecuteInteractively", "BeforeWrite", "FillCheckProcessing", "Filling", "GenerateFromDataHistoryVersionProcessing", "InteractiveActivationProcessing", "OnCheckExecutionProcessing", "OnCopy", "OnExecute", "OnSetNewNumber", "OnWrite"},
	InformationRegisterRecordSetType:  {"BeforeWrite", "FillCheckProcessing", "Filling", "GenerateFromDataHistoryVersionProcessing", "OnWrite"},
	AccumulationRegisterRecordSetType: {"BeforeWrite", "FillCheckProcessing", "OnWrite"},
	AccountingRegisterRecordSetType:   {"BeforeWrite", "FillCheckProcessing", "OnWrite"},
	CalculationRegisterRecordSetType:  {"BeforeWrite", "FillCheckProcessing", "OnWrite"},
	SequenceRecordSetType:             {"BeforeWrite", "FillCheckProcessing", "OnWrite"},
	RecalculationRecordSetType:        {"BeforeWrite", "FillCheckProcessing", "OnWrite"},
	ConstantValueManagerType:          {"BeforeWrite", "FillCheckProcessing", "GenerateFromDataHistoryVersionProcessing", "OnWrite"},
	CatalogManagerType:                {"AfterWriteDataHistoryVersionsProcessing", "ChoiceDataGetProcessing", "FormGetProcessing", "PresentationFieldsGetProcessing", "PresentationGetProcessing"},
	DocumentManagerType:               {"AfterWriteDataHistoryVersionsProcessing", "ChoiceDataGetProcessing", "FormGetProcessing", "PresentationFieldsGetProcessing", "PresentationGetProcessing"},
	EnumerationManagerType:            {"ChoiceDataGetProcessing", "FormGetProcessing"},
	CharacteristicTypesManagerType:    {"AfterWriteDataHistoryVersionsProcessing", "ChoiceDataGetProcessing", "FormGetProcessing", "PresentationFieldsGetProcessing", "PresentationGetProcessing"},
	AccountsManagerType:               {"AfterWriteDataHistoryVersionsProcessing", "ChoiceDataGetProcessing", "FormGetProcessing", "PresentationFieldsGetProcessing", "PresentationGetProcessing"},
	CalculationTypesManagerType:       {"AfterWriteDataHistoryVersionsProcessing", "ChoiceDataGetProcessing", "FormGetProcessing", "PresentationFieldsGetProcessing", "PresentationGetProcessing"},
	ExchangePlanManagerType:           {"AfterWriteDataHistoryVersionsProcessing", "ChoiceDataGetProcessing", "FormGetProcessing", "PresentationFieldsGetProcessing", "PresentationGetProcessing"},
	BusinessProcessManagerType:        {"AfterWriteDataHistoryVersionsProcessing", "ChoiceDataGetProcessing", "FormGetProcessing", "PresentationFieldsGetProcessing", "PresentationGetProcessing"},
	TaskManagerType:                   {"AfterWriteDataHistoryVersionsProcessing", "ChoiceDataGetProcessing", "FormGetProcessing", "PresentationFieldsGetProcessing", "PresentationGetProcessing"},
	InformationRegisterManagerType:    {"AfterWriteDataHistoryVersionsProcessing", "FormGetProcessing"},
	AccumulationRegisterManagerType:   {"FormGetProcessing"},
	AccountingRegisterManagerType:     {"FormGetProcessing"},
	CalculationRegisterManagerType:    {"FormGetProcessing"},
	DocumentJournalManagerType:        {"FormGetProcessing"},
}

// eventSubscriptionName spells an event of the prototype the way a project
// file names it: Filling is fill and FillCheckProcessing fill-check, as they
// always were; the rest is the name in kebab case, OnSetNewCode on-set-new-code.
func eventSubscriptionName(event string) string {
	switch event {
	case "Filling":
		return "fill"
	case "FillCheckProcessing":
		return "fill-check"
	}
	var builder strings.Builder
	for index, character := range event {
		if character >= 'A' && character <= 'Z' {
			if index > 0 {
				builder.WriteByte('-')
			}
			character += 'a' - 'A'
		}
		builder.WriteRune(character)
	}
	return builder.String()
}

// kindHasSubscriptionEvent says the kind has the event, in a project's
// spelling.
func kindHasSubscriptionEvent(kind TypeKind, event string) bool {
	for _, name := range subscriptionEvents[kind] {
		if eventSubscriptionName(name) == event {
			return true
		}
	}
	return false
}

// knownSubscriptionEvent says some kind has the event.
func knownSubscriptionEvent(event string) bool {
	for kind := range subscriptionEvents {
		if kindHasSubscriptionEvent(kind, event) {
			return true
		}
	}
	return false
}

func DecodeEventSubscription(source string, reader io.Reader, configuration project.Project) (EventSubscriptionDefinition, error) {
	var value EventSubscriptionDefinition
	if err := decodeStrict(source, reader, &value); err != nil {
		return EventSubscriptionDefinition{}, err
	}
	if err := ValidateEventSubscription(source, value, configuration); err != nil {
		return EventSubscriptionDefinition{}, err
	}
	return value, nil
}

// ValidateEventSubscription checks structure only; object/module existence,
// object kind and event compatibility are checked catalog-wide by
// validateEventSubscriptionReferences.
func ValidateEventSubscription(source string, value EventSubscriptionDefinition, configuration project.Project) error {
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, configuration)
	if len(value.Source) == 0 {
		issues = append(issues, "source must name at least one type")
	}
	seen := make(map[string]bool, len(value.Source))
	for index, item := range value.Source {
		prefix := fmt.Sprintf("source[%d]", index)
		key := string(item.Kind)
		if item.Reference != nil {
			key += ":" + item.Reference.String()
		}
		if seen[key] {
			issues = append(issues, prefix+" repeats a type of the source")
		}
		seen[key] = true
		switch {
		case item.Kind == DefinedType:
			if item.Reference == nil || item.Reference.IsZero() {
				issues = append(issues, prefix+".reference is required")
			}
		case subscriptionEvents[item.Kind] != nil:
			// No reference is the kind as a whole.
			if item.Reference != nil && item.Reference.IsZero() {
				issues = append(issues, prefix+".reference must be a non-zero UUID or left out for the kind as a whole")
			}
		default:
			issues = append(issues, prefix+".kind "+string(item.Kind)+" is not an object, a record set or a manager a subscription can listen to")
			continue
		}
		if item.Length != 0 || item.Precision != 0 || item.Scale != 0 || item.FixedLength || item.NonNegative || item.DateParts != "" {
			issues = append(issues, prefix+" has unsupported qualifiers")
		}
	}
	if !knownSubscriptionEvent(value.Event) {
		issues = append(issues, fmt.Sprintf("event %q is an event of no object, record set or manager", value.Event))
	}
	if value.Module.IsZero() {
		issues = append(issues, "module must be a non-zero UUID")
	}
	if !validIdentifier(value.Procedure) {
		issues = append(issues, "procedure must start with a letter and contain only letters or digits")
	}
	return issuesError(source, value.Format, issues)
}

func (catalog *Catalog) EventSubscription(name string) (EventSubscriptionDefinition, bool) {
	index, ok := catalog.eventSubscriptionByName[strings.ToLower(name)]
	if !ok {
		return EventSubscriptionDefinition{}, false
	}
	return cloneEventSubscriptionDefinition(catalog.EventSubscriptions[index]), true
}

func (catalog *Catalog) EventSubscriptionByID(id uuid.UUID) (EventSubscriptionDefinition, bool) {
	index, ok := catalog.eventSubscriptionByID[id]
	if !ok {
		return EventSubscriptionDefinition{}, false
	}
	return cloneEventSubscriptionDefinition(catalog.EventSubscriptions[index]), true
}

// eventSubscriptionsFor returns every subscription whose source reaches the
// object of metadata id as the kind - by name, as the kind as a whole, or
// through a defined type - in their stable catalog order.
func (catalog *Catalog) eventSubscriptionsFor(kind TypeKind, id uuid.UUID) []EventSubscriptionDefinition {
	var result []EventSubscriptionDefinition
	for _, item := range catalog.EventSubscriptions {
		if slices.ContainsFunc(catalog.subscriptionSourceTypes(item), func(source Type) bool {
			return source.Kind == kind && (source.Reference == nil || *source.Reference == id)
		}) {
			result = append(result, cloneEventSubscriptionDefinition(item))
		}
	}
	return result
}

// subscriptionSourceTypes is the source with its defined types opened up. A
// defined type of a source holds objects, record sets and managers; anything
// else it may hold says nothing about the source and is passed over.
func (catalog *Catalog) subscriptionSourceTypes(item EventSubscriptionDefinition) []Type {
	var result []Type
	for _, source := range item.Source {
		if source.Kind != DefinedType || source.Reference == nil {
			result = append(result, source)
			continue
		}
		if defined, ok := catalog.DefinedTypeByID(*source.Reference); ok {
			for _, inner := range defined.Types {
				if subscriptionEvents[inner.Kind] != nil {
					result = append(result, inner)
				}
			}
		}
	}
	return result
}

func cloneEventSubscriptionDefinition(value EventSubscriptionDefinition) EventSubscriptionDefinition {
	value.Title = cloneTitle(value.Title)
	value.Source = cloneTypes(value.Source)
	return value
}

// validateEventSubscriptionReferences checks that every subscription's
// handler module exists and can run on the server - object events always run
// there - that what the source names exists, and that the event is one every
// kind of the source has.
func (catalog *Catalog) validateEventSubscriptionReferences() error {
	for _, item := range catalog.EventSubscriptions {
		index, ok := catalog.commonModuleByID[item.Module]
		if !ok {
			return fmt.Errorf("event subscription %s references unknown common module %s", item.Name, item.Module)
		}
		module := catalog.CommonModules[index]
		if !module.Server {
			return fmt.Errorf("event subscription %s handler module %s must be a server module", item.Name, module.Name)
		}
		for _, source := range item.Source {
			if source.Reference == nil {
				continue
			}
			if source.Kind == DefinedType {
				if _, ok := catalog.DefinedTypeByID(*source.Reference); !ok {
					return fmt.Errorf("event subscription %s source references unknown defined type %s", item.Name, source.Reference)
				}
				continue
			}
			if exists := objectTypeOwners[source.Kind]; exists == nil || !exists(catalog, *source.Reference) {
				return fmt.Errorf("event subscription %s source is %s of unknown object %s", item.Name, source.Kind, source.Reference)
			}
		}
		for _, source := range catalog.subscriptionSourceTypes(item) {
			if !kindHasSubscriptionEvent(source.Kind, item.Event) {
				return fmt.Errorf("event subscription %s event %q is not an event of %s", item.Name, item.Event, source.Kind)
			}
		}
	}
	return nil
}
