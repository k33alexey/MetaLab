package metadata

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/k33alexey/MetaLab/internal/kinds"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// FunctionalOptionKind holds the switches that turn parts of an application
// on and off without a developer.
const FunctionalOptionKind = kinds.FunctionalOption

// FunctionalOptionLocation is where the value of an option is kept. The value
// lives in the application's own data, not in the configuration: that is the
// whole point of a functional option - the user turns a part of the
// application off without a developer.
//
// Three places hold it, and only these three (help, FunctionalOption.Location;
// all 1133 options of the three configurations being moved keep to them): a
// constant, an attribute of a catalog, or a resource of an information
// register. The third is how an option comes out different for different
// companies or warehouses - the dimensions of the register say what it
// depends on.
type FunctionalOptionLocation struct {
	Kind   Kind      `yaml:"kind" json:"kind"`
	Object uuid.UUID `yaml:"object" json:"object"`
	// Element is the attribute of the catalog or the resource of the register
	// holding the value. A constant holds its value itself and names none.
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
	seen := map[string]bool{}
	for index, item := range value.Content {
		prefix := fmt.Sprintf("content[%d]", index)
		if !usesOf(item.Kind).option.accepted() {
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
	case InformationRegisterKind, CatalogKind:
		if location.Element == nil || location.Element.IsZero() {
			issues = append(issues, "location.element is required: the value lives in one field of the object")
		}
	case "":
		issues = append(issues, "location.kind is required")
	default:
		issues = append(issues, "location.kind cannot hold the value of a functional option: only a constant, a catalog or an information register can")
	}
	return issues
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
	// flags are the accounting flags of a chart of accounts, its own and those
	// of its kinds of subconto. A functional option switches them; nothing
	// else points at them, so they are kept apart from the attributes.
	flags map[uuid.UUID]bool
}

// has says whether the object holds this attribute at all.
func (elements objectElements) has(id uuid.UUID) bool {
	_, ok := elements.attributes[id]
	return ok
}

// objectElementsOf finds one object by kind and identifier and says what it
// holds, from the one walk of its composition (partsOf): an object of any
// kind, since a role or a functional option may name an object of every kind.
// The second result is false when the configuration has no such object.
func (catalog *Catalog) objectElementsOf(kind Kind, id uuid.UUID) (objectElements, bool) {
	parts, ok := catalog.partsOf(kind, id)
	if !ok {
		return objectElements{}, false
	}
	elements := objectElements{attributes: map[uuid.UUID][]Type{}, tableParts: map[uuid.UUID][]Attribute{},
		commands: map[uuid.UUID]bool{}}
	for _, part := range parts {
		id, err := uuid.Parse(part.key)
		if err != nil {
			continue
		}
		switch {
		case part.group == TablePartsPart:
			if _, ok := elements.tableParts[id]; !ok {
				elements.tableParts[id] = []Attribute{}
			}
		case part.group == AttributesPart && part.parent != "":
			owner, _ := uuid.Parse(part.parent)
			elements.tableParts[owner] = append(elements.tableParts[owner], *part.attribute)
		case part.group == CommandsPart:
			elements.commands[id] = true
		case part.group == AccountingFlagsPart || part.group == ExtDimensionAccountingFlagsPart:
			// The accounting flags a functional option switches; nothing else
			// points at them.
			if elements.flags == nil {
				elements.flags = map[uuid.UUID]bool{}
			}
			elements.flags[id] = true
		case part.attribute != nil:
			elements.attributes[id] = part.attribute.Types
		}
	}
	return elements, true
}

// validateFunctionalOptions resolves what every option points at. An option
// pointing at something that is not there switches nothing, and says so
// nowhere - it just quietly never turns anything off.
func (catalog *Catalog) validateFunctionalOptions() error {
	for _, option := range catalog.FunctionalOptions {
		owner := "functional option " + option.Name
		if err := catalog.checkOptionLocation(owner, option.Location); err != nil {
			return err
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

// checkOptionLocation resolves the place holding the value: the constant, the
// attribute of the catalog, the resource of the information register. A
// dimension of the register is not a place - it is what the value is looked up
// by - and neither is an attribute of the register.
func (catalog *Catalog) checkOptionLocation(owner string, location FunctionalOptionLocation) error {
	missing := fmt.Errorf("%s keeps its value in %s %s, which is not in the configuration", owner, location.Kind, location.Object)
	var fields []Attribute
	switch location.Kind {
	case ConstantKind:
		if _, ok := catalog.constantByID[location.Object]; !ok {
			return missing
		}
		return nil
	case CatalogKind:
		index, ok := catalog.catalogByID[location.Object]
		if !ok {
			return missing
		}
		fields = catalog.Catalogs[index].Attributes
	case InformationRegisterKind:
		index, ok := catalog.informationRegisterByID[location.Object]
		if !ok {
			return missing
		}
		fields = catalog.InformationRegisters[index].Resources
	default:
		return fmt.Errorf("%s cannot keep its value in %s", owner, location.Kind)
	}
	for _, field := range fields {
		if location.Element != nil && field.ID == *location.Element {
			return nil
		}
	}
	if location.Kind == InformationRegisterKind {
		return fmt.Errorf("%s keeps its value in field %s, which is not a resource of that information register", owner, location.Element)
	}
	return fmt.Errorf("%s keeps its value in field %s, which is not an attribute of that catalog", owner, location.Element)
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
	if elements.has(*item.Element) || elements.commands[*item.Element] || elements.flags[*item.Element] {
		return nil
	}
	if _, ok := elements.tableParts[*item.Element]; ok {
		return nil
	}
	return fmt.Errorf("%s switches %s, which that object does not have", where, item.Element)
}
