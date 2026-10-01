package metadata

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// ExternalFunction is one function of an external data source: an expression
// in the source's own query language, called from code as a method of the
// source's manager.
//
// It lies inside the description of its source and not in a folder of its
// own, unlike a table or a cube: an export of the platform writes a function
// among the child objects of the source, the way an attribute is written
// inside a catalog. It has no modules, forms or templates to keep a folder for.
//
// Its parameters are not objects of the configuration. They are positional and
// live in the expression itself - see validateExpressionParameters.
type ExternalFunction struct {
	ID      uuid.UUID     `yaml:"id" json:"id"`
	Name    string        `yaml:"name" json:"name"`
	Title   LocalizedText `yaml:"title" json:"title"`
	Comment string        `yaml:"comment,omitempty" json:"comment,omitempty"`
	// ReturnValue says the function returns a value, and Types is the type of
	// that value.
	ReturnValue bool   `yaml:"return_value,omitempty" json:"returnValue,omitempty"`
	Types       []Type `yaml:"types,omitempty" json:"types,omitempty"`
	// ExpressionInDataSource is not required: the configurator creates a
	// function by hand with none and lets it be written later.
	ExpressionInDataSource string `yaml:"expression_in_data_source,omitempty" json:"expressionInDataSource,omitempty"`
}

// validateExternalFunctions checks the functions of one source.
func validateExternalFunctions(functions []ExternalFunction, configuration project.Project) []string {
	var issues []string
	names, ids := map[string]bool{}, map[uuid.UUID]bool{}
	for index, function := range functions {
		prefix := fmt.Sprintf("functions[%d]", index)
		if function.ID.IsZero() {
			issues = append(issues, prefix+".id must be a non-zero UUID")
		} else if ids[function.ID] {
			issues = append(issues, prefix+".id is used twice")
		}
		ids[function.ID] = true
		if !validIdentifier(function.Name) || utf8.RuneCountInString(function.Name) > 128 {
			issues = append(issues, prefix+".name must start with a letter, contain only letters or digits and not exceed 128 characters")
		} else if names[strings.ToLower(function.Name)] {
			issues = append(issues, prefix+".name is used twice")
		}
		names[strings.ToLower(function.Name)] = true
		issues = append(issues, validateTitle(prefix+".title", function.Title, configuration)...)
		if len(function.Types) > 0 {
			issues = append(issues, validateExternalFieldTypes(prefix+".types", function.Types)...)
		}
		if len(function.ExpressionInDataSource) > maxExpressionInDataSource {
			issues = append(issues, fmt.Sprintf("%s.expression_in_data_source must not exceed %d bytes", prefix, maxExpressionInDataSource))
		} else {
			issues = append(issues, validateExpressionParameters(prefix+".expression_in_data_source", function.ExpressionInDataSource)...)
		}
	}
	return issues
}

// validateExpressionParameters checks the parameters of an expression in the
// source's language: &n is the n-th actual parameter, counted from one, and
// &n[] takes a variable number of values, joined by commas, and may only be
// the last parameter.
//
// Only what the notation says unambiguously is checked. A block in braces
// marks optional parameters, and braces are not checked at all: the query
// languages of other databases have braces of their own - the escapes of ODBC,
// {fn NOW()}, among them - and a check of balance would refuse an expression
// that works. An ampersand followed by anything but a digit is not a parameter
// either: it is an operator of the other language.
func validateExpressionParameters(path, expression string) []string {
	var issues []string
	highest, variadic := 0, 0
	for index := 0; index < len(expression); index++ {
		if expression[index] != '&' {
			continue
		}
		end := index + 1
		for end < len(expression) && expression[end] >= '0' && expression[end] <= '9' {
			end++
		}
		if end == index+1 {
			continue
		}
		number, err := strconv.Atoi(expression[index+1 : end])
		if err != nil || number < 1 {
			issues = append(issues, fmt.Sprintf("%s: &%s is not a parameter - parameters are counted from 1", path, expression[index+1:end]))
			index = end - 1
			continue
		}
		if strings.HasPrefix(expression[end:], "[]") {
			if variadic != 0 && variadic != number {
				issues = append(issues, fmt.Sprintf("%s: &%d[] and &%d[] both take a variable number of values, and only the last parameter may", path, variadic, number))
			}
			variadic = number
			end += 2
		}
		if number > highest {
			highest = number
		}
		index = end - 1
	}
	if variadic != 0 && variadic != highest {
		issues = append(issues, fmt.Sprintf("%s: &%d[] takes a variable number of values, and only the last parameter may - the last here is &%d", path, variadic, highest))
	}
	return issues
}

func cloneExternalFunctions(functions []ExternalFunction) []ExternalFunction {
	if functions == nil {
		return nil
	}
	result := make([]ExternalFunction, len(functions))
	for index, function := range functions {
		function.Title = cloneTitle(function.Title)
		function.Types = cloneTypes(function.Types)
		result[index] = function
	}
	return result
}
