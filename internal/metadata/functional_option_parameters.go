package metadata

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// FunctionalOptionParameterKind holds the axes along which the value of a
// functional option differs.
const FunctionalOptionParameterKind Kind = "functional-options-parameters"

// FunctionalOptionParameterUse is one place where the axis appears: the object
// the parameter stands for, or a dimension of an information register that
// holds the option's value.
type FunctionalOptionParameterUse struct {
	Kind   Kind      `yaml:"kind" json:"kind"`
	Object uuid.UUID `yaml:"object" json:"object"`
	// Element is the dimension of the register. Named alone, the object is the
	// whole of what the parameter stands for.
	Element *uuid.UUID `yaml:"element,omitempty" json:"element,omitempty"`
}

// FunctionalOptionParameterDefinition describes one axis along which the value
// of a functional option differs - an option turned on for one company and off
// for another.
//
// The parameter exists because the value has to be looked up by something. An
// option kept in a resource of an information register has one value per
// combination of that register's dimensions, and without a parameter standing
// for a dimension nothing says which combination to read.
type FunctionalOptionParameterDefinition struct {
	Format  int                            `yaml:"format" json:"format"`
	ID      uuid.UUID                      `yaml:"id" json:"id"`
	Name    string                         `yaml:"name" json:"name"`
	Title   LocalizedText                  `yaml:"title" json:"title"`
	Comment string                         `yaml:"comment,omitempty" json:"comment,omitempty"`
	Use     []FunctionalOptionParameterUse `yaml:"use,omitempty" json:"use,omitempty"`
}

// DecodeFunctionalOptionParameter reads and validates one parameter.
func DecodeFunctionalOptionParameter(source string, reader io.Reader, configuration project.Project) (FunctionalOptionParameterDefinition, error) {
	var value FunctionalOptionParameterDefinition
	if err := decodeStrict(source, reader, &value); err != nil {
		return FunctionalOptionParameterDefinition{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, configuration)
	seen := map[string]bool{}
	for index, item := range value.Use {
		prefix := fmt.Sprintf("use[%d]", index)
		if !knownMetadataKind(item.Kind) {
			issues = append(issues, prefix+".kind is not a kind of metadata object")
		}
		if item.Object.IsZero() {
			issues = append(issues, prefix+".object must be a non-zero UUID")
		}
		if item.Element != nil && item.Element.IsZero() {
			issues = append(issues, prefix+".element must be a non-zero UUID")
		}
		key := string(item.Kind) + ":" + item.Object.String()
		if item.Element != nil {
			key += ":" + item.Element.String()
		}
		if seen[key] {
			issues = append(issues, prefix+" is already in the use")
		}
		seen[key] = true
	}
	if err := issuesError(source, value.Format, issues); err != nil {
		return FunctionalOptionParameterDefinition{}, err
	}
	return value, nil
}

func cloneFunctionalOptionParameter(value FunctionalOptionParameterDefinition) FunctionalOptionParameterDefinition {
	value.Title = cloneTitle(value.Title)
	value.Use = slices.Clone(value.Use)
	for index := range value.Use {
		if value.Use[index].Element != nil {
			copied := *value.Use[index].Element
			value.Use[index].Element = &copied
		}
	}
	return value
}

// FunctionalOptionParameter returns one parameter by name, folded case.
func (catalog *Catalog) FunctionalOptionParameter(name string) (FunctionalOptionParameterDefinition, bool) {
	index, ok := catalog.functionalOptionParameterByName[strings.ToLower(name)]
	if !ok {
		return FunctionalOptionParameterDefinition{}, false
	}
	return cloneFunctionalOptionParameter(catalog.FunctionalOptionParameters[index]), true
}

// validateFunctionalOptionParameters resolves what every parameter points at.
//
// It does not ask the other direction - that every dimension of the register
// holding an option has a parameter standing for it. The prototype saves an
// option without one (7, 20 and 2 such options in the three configurations
// being moved), and the help says what reading it gives: parameters passed
// only in part make a Boolean option true if any of the values selected by
// them is true, and raise for an option of any other type (help,
// ПолучитьФункциональнуюОпцию). An uncovered dimension is parameters passed in
// part every time, which is defined behaviour, not a broken option.
func (catalog *Catalog) validateFunctionalOptionParameters() error {
	for _, parameter := range catalog.FunctionalOptionParameters {
		owner := "functional option parameter " + parameter.Name
		for index, item := range parameter.Use {
			where := fmt.Sprintf("%s use[%d]", owner, index)
			elements, ok := catalog.objectElementsOf(item.Kind, item.Object)
			if !ok {
				return fmt.Errorf("%s stands for %s %s, which is not in the configuration", where, item.Kind, item.Object)
			}
			if item.Element != nil && !elements.has(*item.Element) {
				return fmt.Errorf("%s stands for field %s, which that object does not have", where, item.Element)
			}
		}
	}
	return nil
}
