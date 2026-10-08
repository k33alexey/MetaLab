package metadata

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// FormParameter is a parameter of a form: a value the form is opened with,
// which code reads by its name (help, the parameters of a managed form). The
// exports give 13943 parameters to 3063 forms. A parameter has no identifier
// of its own - the prototype writes a name, a type and whether it is a key -
// so it is known by its name alone.
type FormParameter struct {
	Name string `yaml:"name" json:"name"`
	// Types is the type of the value, as the type of an attribute of a form
	// is: left empty it is arbitrary (5950 parameters of the exports), and it
	// may name a set (any reference, 430) or a type the configuration no
	// longer has.
	Types []Type `yaml:"types,omitempty" json:"types,omitempty"`
	// Key makes the parameter one of those by which a form already open is
	// told apart from a new one: opening the form again with the same key
	// parameters activates the open window (help, the key parameter of a
	// form; 1127 in the exports).
	Key bool `yaml:"key,omitempty" json:"key,omitempty"`
}

func validateFormParameters(parameters []FormParameter) []string {
	var issues []string
	names := map[string]bool{}
	for index, parameter := range parameters {
		path := fmt.Sprintf("parameters[%d]", index)
		if !validIdentifier(parameter.Name) || utf8.RuneCountInString(parameter.Name) > maxNameLength {
			issues = append(issues, path+".name must be a valid identifier of at most 255 characters")
		}
		folded := strings.ToLower(parameter.Name)
		if names[folded] {
			issues = append(issues, path+".name must be unique within the parameters")
		}
		names[folded] = true
		issues = append(issues, validateTypesIn(path+".types", parameter.Types, placeFormAttribute)...)
	}
	return issues
}

func cloneFormParameters(parameters []FormParameter) []FormParameter {
	if parameters == nil {
		return nil
	}
	result := make([]FormParameter, len(parameters))
	for index, parameter := range parameters {
		parameter.Types = cloneTypes(parameter.Types)
		result[index] = parameter
	}
	return result
}

// resolveFormParameters checks the types of the parameters of one form
// against the project, as the types of its attributes are checked.
func (catalog *Catalog) resolveFormParameters(form string, parameters []FormParameter) error {
	for _, parameter := range parameters {
		if err := catalog.resolveFormData(form+" parameter "+parameter.Name, parameter.Types, nil); err != nil {
			return err
		}
	}
	return nil
}
