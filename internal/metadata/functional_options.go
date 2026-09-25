package metadata

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// FunctionalOptionKind holds the switches that turn parts of an application
// on and off without a developer.
const FunctionalOptionKind Kind = "functional-options"

// maxFunctionalOptionContent is where a list of switched things stops being a
// list. The reference configuration's largest option switches a few dozen.
const maxFunctionalOptionContent = 4096

// FunctionalOptionLocation is where the value of an option is kept. The value
// lives in the application's own data, not in the configuration: that is the
// whole point of a functional option - the user turns a part of the
// application off without a developer.
//
// Three places hold it: a constant, an attribute of an object, or a resource
// of an information register. The third is how an option comes out different
// for different companies or warehouses - the dimensions of the register say
// what it depends on.
type FunctionalOptionLocation struct {
	Kind   Kind      `yaml:"kind" json:"kind"`
	Object uuid.UUID `yaml:"object" json:"object"`
	// Element is the attribute or the resource holding the value. A constant
	// holds its value itself and names none.
	Element *uuid.UUID `yaml:"element,omitempty" json:"element,omitempty"`
}

// FunctionalOptionItem is one thing the option switches on and off. It is
// either a whole object, or a part of one: an attribute, a table part, an
// attribute of a table part, a resource, a dimension, a command.
//
// The object is always named, even when a part of it is switched, for the same
// reason a reference value names its object: an identifier of a part says
// nothing about whom it belongs to, and looking for the one object that
// happens to have a part with this identifier answers by coincidence.
type FunctionalOptionItem struct {
	Kind   Kind      `yaml:"kind" json:"kind"`
	Object uuid.UUID `yaml:"object" json:"object"`
	// TablePart, when named, is the table part inside the object. Named alone
	// it is the table part itself that is switched; named together with an
	// element, the element is an attribute of that table part.
	TablePart *uuid.UUID `yaml:"table_part,omitempty" json:"tablePart,omitempty"`
	Element   *uuid.UUID `yaml:"element,omitempty" json:"element,omitempty"`
}

// FunctionalOptionDefinition describes one functional option: what part of the
// application it switches and where the user's answer is kept.
type FunctionalOptionDefinition struct {
	Format  int           `yaml:"format" json:"format"`
	ID      uuid.UUID     `yaml:"id" json:"id"`
	Name    string        `yaml:"name" json:"name"`
	Title   LocalizedText `yaml:"title" json:"title"`
	Comment string        `yaml:"comment,omitempty" json:"comment,omitempty"`
	// Location is where the value is read from.
	Location FunctionalOptionLocation `yaml:"location" json:"location"`
	// PrivilegedGetMode reads the value past the user's rights. Without it a
	// user who may not read the constant behind the option sees the
	// application as if the option were off, which is not the same answer as
	// "you may not look" and is far harder to explain.
	PrivilegedGetMode bool                   `yaml:"privileged_get_mode,omitempty" json:"privilegedGetMode,omitempty"`
	Content           []FunctionalOptionItem `yaml:"content,omitempty" json:"content,omitempty"`
}

// DecodeFunctionalOption reads and validates one functional option. What it
// points at is resolved against the configuration later, when every kind of
// object is loaded.
func DecodeFunctionalOption(source string, reader io.Reader, configuration project.Project) (FunctionalOptionDefinition, error) {
	var value FunctionalOptionDefinition
	if err := decodeStrict(source, reader, &value); err != nil {
		return FunctionalOptionDefinition{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, configuration)
	issues = append(issues, validateOptionLocation(value.Location)...)
	if len(value.Content) > maxFunctionalOptionContent {
		issues = append(issues, fmt.Sprintf("content must not contain more than %d items", maxFunctionalOptionContent))
	}
	seen := map[string]bool{}
	for index, item := range value.Content {
		prefix := fmt.Sprintf("content[%d]", index)
		if !knownMetadataKind(item.Kind) {
			issues = append(issues, prefix+".kind is not a kind of metadata object")
		}
		if item.Object.IsZero() {
			issues = append(issues, prefix+".object must be a non-zero UUID")
		}
		for name, id := range map[string]*uuid.UUID{".table_part": item.TablePart, ".element": item.Element} {
			if id != nil && id.IsZero() {
				issues = append(issues, prefix+name+" must be a non-zero UUID")
			}
		}
		key := string(item.Kind) + ":" + item.Object.String()
		if item.TablePart != nil {
			key += ":" + item.TablePart.String()
		}
		if item.Element != nil {
			key += ":" + item.Element.String()
		}
		if seen[key] {
			issues = append(issues, prefix+" is already in the content")
		}
		seen[key] = true
	}
	if err := issuesError(source, value.Format, issues); err != nil {
		return FunctionalOptionDefinition{}, err
	}
	return value, nil
}

// validateOptionLocation checks the shape of the place holding the value. A
// constant is the place itself; everything else names the field inside it.
func validateOptionLocation(location FunctionalOptionLocation) []string {
	var issues []string
	if location.Object.IsZero() {
		issues = append(issues, "location.object must be a non-zero UUID")
	}
	switch location.Kind {
	case ConstantKind:
		if location.Element != nil {
			issues = append(issues, "location.element is not allowed: a constant holds the value itself")
		}
	case InformationRegisterKind, CatalogKind, DocumentKind, ChartOfCharacteristicTypesKind,
		ChartOfAccountsKind, ChartOfCalculationTypesKind, ExchangePlanKind, BusinessProcessKind, TaskKind:
		if location.Element == nil || location.Element.IsZero() {
			issues = append(issues, "location.element is required: the value lives in one field of the object")
		}
	case "":
		issues = append(issues, "location.kind is required")
	default:
		issues = append(issues, "location.kind cannot hold the value of a functional option")
	}
	return issues
}

// knownMetadataKind says whether the model has this kind at all. A kind it
// does not have cannot be written down here, and the conformance report is
// where that shows up until the kind is modelled.
func knownMetadataKind(kind Kind) bool {
	switch kind {
	case ConstantKind, EnumerationKind, CatalogKind, DocumentKind, DocumentJournalKind,
		InformationRegisterKind, AccumulationRegisterKind, AccountingRegisterKind, CalculationRegisterKind,
		ChartOfCharacteristicTypesKind, ChartOfAccountsKind, ChartOfCalculationTypesKind,
		BusinessProcessKind, TaskKind, ExchangePlanKind, ReportKind, DataProcessorKind,
		SubsystemKind, CommonAttributeKind, CommonModuleKind, SessionParameterKind, DefinedTypeKind,
		NumeratorKind, SequenceKind, RoleKind, EventSubscriptionKind:
		return true
	default:
		return false
	}
}

func cloneFunctionalOption(value FunctionalOptionDefinition) FunctionalOptionDefinition {
	value.Title = cloneTitle(value.Title)
	if value.Location.Element != nil {
		copied := *value.Location.Element
		value.Location.Element = &copied
	}
	value.Content = slices.Clone(value.Content)
	for index := range value.Content {
		item := &value.Content[index]
		for _, id := range []**uuid.UUID{&item.TablePart, &item.Element} {
			if *id != nil {
				copied := **id
				*id = &copied
			}
		}
	}
	return value
}

// FunctionalOption returns one functional option by name, folded case.
func (catalog *Catalog) FunctionalOption(name string) (FunctionalOptionDefinition, bool) {
	index, ok := catalog.functionalOptionByName[strings.ToLower(name)]
	if !ok {
		return FunctionalOptionDefinition{}, false
	}
	return cloneFunctionalOption(catalog.FunctionalOptions[index]), true
}

// objectElements is what one object holds that something else may point at: a
// functional option switching it, a parameter standing for it, a filter
// criterion searching it.
//
// Attributes keep their types, because pointing at a field is not always
// enough - a criterion has to know what the field can hold.
type objectElements struct {
	attributes map[uuid.UUID][]Type
	tableParts map[uuid.UUID][]Attribute
	commands   map[uuid.UUID]bool
}

// has says whether the object holds this attribute at all.
func (elements objectElements) has(id uuid.UUID) bool {
	_, ok := elements.attributes[id]
	return ok
}

func elementsOf(attributes []Attribute, parts []TablePart, commands []ObjectCommand, extra ...[]Attribute) objectElements {
	elements := objectElements{
		attributes: make(map[uuid.UUID][]Type, len(attributes)),
		tableParts: make(map[uuid.UUID][]Attribute, len(parts)),
		commands:   make(map[uuid.UUID]bool, len(commands)),
	}
	for _, group := range append([][]Attribute{attributes}, extra...) {
		for _, attribute := range group {
			elements.attributes[attribute.ID] = attribute.Types
		}
	}
	for _, part := range parts {
		elements.tableParts[part.ID] = part.Attributes
	}
	for _, command := range commands {
		elements.commands[command.ID] = true
	}
	return elements
}

// objectElementsOf finds one object by kind and identifier and says what it
// holds. The second result is false when the configuration has no such object.
func (catalog *Catalog) objectElementsOf(kind Kind, id uuid.UUID) (objectElements, bool) {
	switch kind {
	case ConstantKind:
		_, ok := catalog.constantByID[id]
		return objectElements{}, ok
	case SubsystemKind:
		_, ok := catalog.subsystemByID[id]
		return objectElements{}, ok
	case CommonAttributeKind:
		_, ok := catalog.commonAttributeByID[id]
		return objectElements{}, ok
	case CommonModuleKind:
		_, ok := catalog.commonModuleByID[id]
		return objectElements{}, ok
	case SessionParameterKind:
		_, ok := catalog.sessionParameterByID[id]
		return objectElements{}, ok
	case DefinedTypeKind:
		_, ok := catalog.definedTypeByID[id]
		return objectElements{}, ok
	case RoleKind:
		_, ok := catalog.roleByID[id]
		return objectElements{}, ok
	case EventSubscriptionKind:
		_, ok := catalog.eventSubscriptionByID[id]
		return objectElements{}, ok
	case NumeratorKind:
		_, ok := catalog.numeratorByID[id]
		return objectElements{}, ok
	case SequenceKind:
		_, ok := catalog.sequenceByID[id]
		return objectElements{}, ok
	case EnumerationKind:
		index, ok := catalog.enumerationByID[id]
		if !ok {
			return objectElements{}, false
		}
		item := catalog.Enumerations[index]
		return elementsOf(nil, nil, item.Commands), true
	case CatalogKind:
		index, ok := catalog.catalogByID[id]
		if !ok {
			return objectElements{}, false
		}
		item := catalog.Catalogs[index]
		return elementsOf(item.Attributes, item.TableParts, item.Commands), true
	case DocumentKind:
		index, ok := catalog.documentByID[id]
		if !ok {
			return objectElements{}, false
		}
		item := catalog.Documents[index]
		return elementsOf(item.Attributes, item.TableParts, item.Commands), true
	case DocumentJournalKind:
		index, ok := catalog.documentJournalByID[id]
		if !ok {
			return objectElements{}, false
		}
		return elementsOf(nil, nil, catalog.DocumentJournals[index].Commands), true
	case ChartOfCharacteristicTypesKind:
		index, ok := catalog.chartOfCharacteristicTypesByID[id]
		if !ok {
			return objectElements{}, false
		}
		item := catalog.ChartsOfCharacteristicTypes[index]
		return elementsOf(item.Attributes, item.TableParts, item.Commands), true
	case ChartOfAccountsKind:
		index, ok := catalog.chartOfAccountsByID[id]
		if !ok {
			return objectElements{}, false
		}
		item := catalog.ChartsOfAccounts[index]
		return elementsOf(item.Attributes, item.TableParts, item.Commands), true
	case ChartOfCalculationTypesKind:
		index, ok := catalog.chartOfCalculationTypesByID[id]
		if !ok {
			return objectElements{}, false
		}
		item := catalog.ChartsOfCalculationTypes[index]
		return elementsOf(item.Attributes, item.TableParts, item.Commands), true
	case BusinessProcessKind:
		index, ok := catalog.businessProcessByID[id]
		if !ok {
			return objectElements{}, false
		}
		item := catalog.BusinessProcesses[index]
		return elementsOf(item.Attributes, item.TableParts, item.Commands), true
	case TaskKind:
		index, ok := catalog.taskByID[id]
		if !ok {
			return objectElements{}, false
		}
		item := catalog.Tasks[index]
		elements := elementsOf(item.Attributes, item.TableParts, item.Commands)
		// The attributes a task is addressed by are attributes like any other
		// as far as switching them off goes.
		for _, attribute := range item.AddressingAttributes {
			elements.attributes[attribute.ID] = attribute.Types
		}
		return elements, true
	case ExchangePlanKind:
		index, ok := catalog.exchangePlanByID[id]
		if !ok {
			return objectElements{}, false
		}
		item := catalog.ExchangePlans[index]
		return elementsOf(item.Attributes, item.TableParts, item.Commands), true
	case InformationRegisterKind:
		index, ok := catalog.informationRegisterByID[id]
		if !ok {
			return objectElements{}, false
		}
		item := catalog.InformationRegisters[index]
		return elementsOf(item.Attributes, nil, item.Commands, item.Dimensions, item.Resources), true
	case AccumulationRegisterKind:
		index, ok := catalog.accumulationRegisterByID[id]
		if !ok {
			return objectElements{}, false
		}
		item := catalog.AccumulationRegisters[index]
		return elementsOf(item.Attributes, nil, item.Commands, item.Dimensions, item.Resources), true
	case ReportKind:
		index, ok := catalog.reportByID[id]
		if !ok {
			return objectElements{}, false
		}
		item := catalog.Reports[index]
		return elementsOf(item.Attributes, item.TableParts, item.Commands), true
	case DataProcessorKind:
		index, ok := catalog.dataProcessorByID[id]
		if !ok {
			return objectElements{}, false
		}
		item := catalog.DataProcessors[index]
		return elementsOf(item.Attributes, item.TableParts, item.Commands), true
	}
	return objectElements{}, false
}

// validateFunctionalOptions resolves what every option points at. An option
// pointing at something that is not there switches nothing, and says so
// nowhere - it just quietly never turns anything off.
func (catalog *Catalog) validateFunctionalOptions() error {
	for _, option := range catalog.FunctionalOptions {
		owner := "functional option " + option.Name
		elements, ok := catalog.objectElementsOf(option.Location.Kind, option.Location.Object)
		if !ok {
			return fmt.Errorf("%s keeps its value in %s %s, which is not in the configuration",
				owner, option.Location.Kind, option.Location.Object)
		}
		if element := option.Location.Element; element != nil && !elements.has(*element) {
			return fmt.Errorf("%s keeps its value in field %s, which that %s does not have",
				owner, element, option.Location.Kind)
		}
		for index, item := range option.Content {
			where := fmt.Sprintf("%s content[%d]", owner, index)
			elements, ok := catalog.objectElementsOf(item.Kind, item.Object)
			if !ok {
				return fmt.Errorf("%s switches %s %s, which is not in the configuration", where, item.Kind, item.Object)
			}
			if err := checkOptionElement(where, item, elements); err != nil {
				return err
			}
		}
	}
	return nil
}

// checkOptionElement resolves the part of an object an option switches.
func checkOptionElement(where string, item FunctionalOptionItem, elements objectElements) error {
	if item.TablePart != nil {
		attributes, ok := elements.tableParts[*item.TablePart]
		if !ok {
			return fmt.Errorf("%s switches table part %s, which that object does not have", where, item.TablePart)
		}
		if item.Element == nil {
			return nil
		}
		for _, attribute := range attributes {
			if attribute.ID == *item.Element {
				return nil
			}
		}
		return fmt.Errorf("%s switches attribute %s, which that table part does not have", where, item.Element)
	}
	if item.Element == nil {
		return nil
	}
	if elements.has(*item.Element) || elements.commands[*item.Element] {
		return nil
	}
	if _, ok := elements.tableParts[*item.Element]; ok {
		return nil
	}
	return fmt.Errorf("%s switches %s, which that object does not have", where, item.Element)
}
