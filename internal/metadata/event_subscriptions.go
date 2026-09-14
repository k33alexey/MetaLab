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

// EventSubscriptionDefinition attaches one exported common module procedure
// to a lifecycle event of one or more catalogs/documents/registers, without
// requiring those objects' own modules to implement the handler (1C style).
// Unlike CommonAttributeDefinition.Objects, which propagates the exact same
// field into every target, Objects here may span different metadata kinds:
// each target only needs to support the declared Event. The handler
// procedure receives the source object as its first argument, followed by
// the same arguments the object's own module would receive for that event
// (for example Cancel, then any event-specific values) — matching real 1C
// subscription handler signatures, which take Source explicitly rather than
// through an implicit predefined variable.
type EventSubscriptionDefinition struct {
	Format    int           `yaml:"format"`
	ID        uuid.UUID     `yaml:"id"`
	Name      string        `yaml:"name"`
	Title     LocalizedText `yaml:"title"`
	Objects   []uuid.UUID   `yaml:"objects"`
	Event     string        `yaml:"event"`
	Module    uuid.UUID     `yaml:"module"`
	Procedure string        `yaml:"procedure"`
}

var validEventSubscriptionEvents = map[string]bool{
	"fill": true, "fill-check": true, "before-write": true, "on-write": true,
	"after-write": true, "before-delete": true, "posting": true, "undo-posting": true,
}

// eventsByObjectKind lists which of the events above apply to each kind of
// subscribable object: registers write as record sets (no fill/delete), and
// only documents can be posted.
var eventSubscriptionEventsByKind = map[string]map[string]bool{
	"catalog": {
		"fill": true, "fill-check": true, "before-write": true, "on-write": true, "after-write": true, "before-delete": true,
	},
	"document": {
		"fill": true, "fill-check": true, "before-write": true, "on-write": true, "after-write": true, "before-delete": true,
		"posting": true, "undo-posting": true,
	},
	"information register":  {"before-write": true, "on-write": true, "after-write": true},
	"accumulation register": {"before-write": true, "on-write": true, "after-write": true},
}

func DecodeEventSubscription(source string, reader io.Reader, manifest project.Project) (EventSubscriptionDefinition, error) {
	var value EventSubscriptionDefinition
	if err := decodeStrict(source, reader, &value); err != nil {
		return EventSubscriptionDefinition{}, err
	}
	if err := ValidateEventSubscription(source, value, manifest); err != nil {
		return EventSubscriptionDefinition{}, err
	}
	return value, nil
}

// ValidateEventSubscription checks structure only; object/module existence,
// object kind and event compatibility are checked catalog-wide by
// validateEventSubscriptionReferences.
func ValidateEventSubscription(source string, value EventSubscriptionDefinition, manifest project.Project) error {
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, manifest)
	if len(value.Objects) == 0 {
		issues = append(issues, "objects must list at least one target object")
	}
	seen := make(map[uuid.UUID]bool, len(value.Objects))
	for index, target := range value.Objects {
		prefix := fmt.Sprintf("objects[%d]", index)
		if target.IsZero() {
			issues = append(issues, prefix+" must be a non-zero UUID")
		}
		if seen[target] {
			issues = append(issues, prefix+" must be unique")
		}
		seen[target] = true
	}
	if !validEventSubscriptionEvents[value.Event] {
		issues = append(issues, "event must be one of: fill, fill-check, before-write, on-write, after-write, before-delete, posting, undo-posting")
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

// eventSubscriptionsForObject returns every subscription that targets id, in
// their stable catalog order.
func (catalog *Catalog) eventSubscriptionsForObject(id uuid.UUID) []EventSubscriptionDefinition {
	var result []EventSubscriptionDefinition
	for _, item := range catalog.EventSubscriptions {
		if slices.Contains(item.Objects, id) {
			result = append(result, cloneEventSubscriptionDefinition(item))
		}
	}
	return result
}

func cloneEventSubscriptionDefinition(value EventSubscriptionDefinition) EventSubscriptionDefinition {
	value.Title = cloneTitle(value.Title)
	value.Objects = slices.Clone(value.Objects)
	return value
}

// eventSubscriptionObjectKind reports which subscribable kind id belongs to.
func (catalog *Catalog) eventSubscriptionObjectKind(id uuid.UUID) (string, bool) {
	if _, ok := catalog.catalogByID[id]; ok {
		return "catalog", true
	}
	if _, ok := catalog.documentByID[id]; ok {
		return "document", true
	}
	if _, ok := catalog.informationRegisterByID[id]; ok {
		return "information register", true
	}
	if _, ok := catalog.accumulationRegisterByID[id]; ok {
		return "accumulation register", true
	}
	return "", false
}

// validateEventSubscriptionReferences checks that every subscription's
// handler module exists and can run on the server (object lifecycle events
// always execute server-side), that every target object exists, and that
// the declared event applies to that object's kind.
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
		for _, objectID := range item.Objects {
			kind, ok := catalog.eventSubscriptionObjectKind(objectID)
			if !ok {
				return fmt.Errorf("event subscription %s references unknown object %s", item.Name, objectID)
			}
			if !eventSubscriptionEventsByKind[kind][item.Event] {
				return fmt.Errorf("event subscription %s event %q is not valid for %s objects", item.Name, item.Event, kind)
			}
		}
	}
	return nil
}
