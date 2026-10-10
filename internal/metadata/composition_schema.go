package metadata

import (
	"fmt"
	"reflect"
	"strings"
	"unicode/utf8"

	"github.com/k33alexey/MetaLab/internal/project"
)

// The parts of a data composition schema a dynamic list keeps: the fields of
// its data set, its calculated fields and its data parameters (help,
// DynamicList.Fields, DynamicList.CalculatedFields, DynamicList.DataParameters,
// since 8.3.19). They are the types of the schema itself -
// DataCompositionSchemaDataSetField, DataCompositionSchemaNestedDataSet,
// DataCompositionSchemaCalculatedField, DataCompositionSchemaParameter - and
// the schema of a report (block 12) is to take them as they are, as it takes
// the settings of composition.
//
// Only one export writes them, demo-base, in 19 lists of 16 forms: 368 fields,
// 6 nested data sets, 1 calculated field, 28 parameters. The model carries
// what it writes and what the help names beside it in the same simple form -
// the type of a field, the restriction of its attributes. What the help names
// and needs a model of its own - a role, an appearance, the parameters of
// editing, the available values, the expressions of presentation and of
// order, a folder of fields, the expression of a parameter - is not carried
// yet; a writing the model does not know is refused, not dropped. The list
// does not support a role, an expression of presentation, the check of a
// hierarchy and the parameter of functional options anyway (help).

// CompositionDataSetFieldKind says what a field of a data set is.
type CompositionDataSetFieldKind string

const (
	// CompositionDataSetFieldField is a field (DataCompositionSchemaDataSetField).
	CompositionDataSetFieldField CompositionDataSetFieldKind = "field"
	// CompositionNestedDataSet is a table part read as a data set of its own
	// (DataCompositionSchemaNestedDataSet): its fields follow it as fields
	// whose path starts with its own.
	CompositionNestedDataSet CompositionDataSetFieldKind = "nested-data-set"
)

// CompositionDataSetField is a field of a data set. DataPath is how the
// settings and the form name it; Field is the field of the query it is
// taken from, a path as well: a field of a nested data set is written with
// the name of the set before its own - КонтактнаяИнформация.Вид, 34 of the
// 368 - though a designer may write the name alone. A nested data set has a
// path, a field and a title only.
type CompositionDataSetField struct {
	Kind     CompositionDataSetFieldKind `yaml:"kind" json:"kind"`
	DataPath string                      `yaml:"data_path" json:"dataPath"`
	Field    string                      `yaml:"field" json:"field"`
	Title    LocalizedText               `yaml:"title,omitempty" json:"title,omitempty"`
	// ValueType is the type of the field, empty when it is taken from the
	// query.
	ValueType []Type `yaml:"value_type,omitempty" json:"valueType,omitempty"`
	// UseRestriction and AttributeUseRestriction say where the field and
	// the attributes of its value may not be used.
	UseRestriction          *CompositionUseRestriction `yaml:"use_restriction,omitempty" json:"useRestriction,omitempty"`
	AttributeUseRestriction *CompositionUseRestriction `yaml:"attribute_use_restriction,omitempty" json:"attributeUseRestriction,omitempty"`
}

// CompositionUseRestriction says where a field may not be used (help,
// DataCompositionSchemaFieldUseRestriction): among the selected fields, in
// a filter, in a grouping, in an order. Nil restricts nothing.
type CompositionUseRestriction struct {
	Field     bool `yaml:"field,omitempty" json:"field,omitempty"`
	Condition bool `yaml:"condition,omitempty" json:"condition,omitempty"`
	Group     bool `yaml:"group,omitempty" json:"group,omitempty"`
	Order     bool `yaml:"order,omitempty" json:"order,omitempty"`
}

// CompositionCalculatedField is a field whose value is an expression of the
// language of composition over the fields of the data sets. The expression
// is kept as written and parsed where it is executed (block 12).
type CompositionCalculatedField struct {
	DataPath       string                     `yaml:"data_path" json:"dataPath"`
	Expression     string                     `yaml:"expression" json:"expression"`
	Title          LocalizedText              `yaml:"title,omitempty" json:"title,omitempty"`
	ValueType      []Type                     `yaml:"value_type,omitempty" json:"valueType,omitempty"`
	UseRestriction *CompositionUseRestriction `yaml:"use_restriction,omitempty" json:"useRestriction,omitempty"`
}

// CompositionParameter is a parameter of the data. ValueType empty takes a
// value of any type (8 of the 28). Value nil is a value not written, which
// the prototype leaves out only on a parameter that takes a list (3 of the
// 4); Неопределено is written as a value of its own (11). UseRestriction
// hides the parameter from the user.
type CompositionParameter struct {
	Name             string            `yaml:"name" json:"name"`
	Title            LocalizedText     `yaml:"title,omitempty" json:"title,omitempty"`
	ValueType        []Type            `yaml:"value_type,omitempty" json:"valueType,omitempty"`
	Value            *CompositionValue `yaml:"value,omitempty" json:"value,omitempty"`
	UseRestriction   bool              `yaml:"use_restriction,omitempty" json:"useRestriction,omitempty"`
	ValueListAllowed bool              `yaml:"value_list_allowed,omitempty" json:"valueListAllowed,omitempty"`
}

// compositionParameterValues are the kinds a parameter holds: a value, never
// a field or an appearance.
var compositionParameterValues = []CompositionValueKind{CompositionBoolean, CompositionNumber, CompositionString,
	CompositionDate, CompositionPredefined, CompositionType, CompositionAccountType, CompositionBeginningDate,
	CompositionValueList, CompositionNull, CompositionUndefined}

// validateListSchema checks the fields, calculated fields and parameters of
// a dynamic list against themselves. A data path names one field, calculated
// or not, and a name one parameter, in any case of letters, as the language
// of composition reads them.
func validateListSchema(path string, settings DynamicListSettings) []string {
	var issues []string
	paths := map[string]string{}
	usePath := func(at, value string) {
		if validateFormDataPath(at+".data_path", value) != nil {
			issues = append(issues, at+".data_path must contain only valid identifier segments separated by dots")
			return
		}
		folded := strings.ToLower(value)
		if first, ok := paths[folded]; ok {
			issues = append(issues, at+".data_path is the path of "+first+" already")
			return
		}
		paths[folded] = at
	}
	for index, field := range settings.Fields {
		at := fmt.Sprintf("%s.fields[%d]", path, index)
		usePath(at, field.DataPath)
		issues = append(issues, validateFormDataPath(at+".field", field.Field)...)
		issues = append(issues, validateCompositionTitle(at+".title", field.Title)...)
		switch field.Kind {
		case CompositionDataSetFieldField:
			issues = append(issues, validateTypesIn(at+".value_type", field.ValueType, placeFormAttribute)...)
			issues = append(issues, validateUseRestriction(at+".use_restriction", field.UseRestriction)...)
			issues = append(issues, validateUseRestriction(at+".attribute_use_restriction", field.AttributeUseRestriction)...)
		case CompositionNestedDataSet:
			if len(field.ValueType) != 0 || field.UseRestriction != nil || field.AttributeUseRestriction != nil {
				issues = append(issues, at+" is a nested data set and has a path, a field and a title only")
			}
		default:
			issues = append(issues, at+".kind must be field or nested-data-set")
		}
	}
	for index, field := range settings.CalculatedFields {
		at := fmt.Sprintf("%s.calculated_fields[%d]", path, index)
		usePath(at, field.DataPath)
		if !utf8.ValidString(field.Expression) {
			issues = append(issues, at+".expression must be valid UTF-8")
		}
		issues = append(issues, validateCompositionTitle(at+".title", field.Title)...)
		issues = append(issues, validateTypesIn(at+".value_type", field.ValueType, placeFormAttribute)...)
		issues = append(issues, validateUseRestriction(at+".use_restriction", field.UseRestriction)...)
	}
	names := map[string]bool{}
	for index, parameter := range settings.DataParameters {
		at := fmt.Sprintf("%s.data_parameters[%d]", path, index)
		if !validIdentifier(parameter.Name) || utf8.RuneCountInString(parameter.Name) > maxNameLength {
			issues = append(issues, at+".name must be the name of a parameter")
		} else if folded := strings.ToLower(parameter.Name); names[folded] {
			issues = append(issues, at+".name is the name of another parameter already")
		} else {
			names[folded] = true
		}
		issues = append(issues, validateCompositionTitle(at+".title", parameter.Title)...)
		issues = append(issues, validateTypesIn(at+".value_type", parameter.ValueType, placeFormAttribute)...)
		if parameter.Value != nil {
			issues = append(issues, validateCompositionValue(at+".value", *parameter.Value, compositionParameterValues)...)
		}
	}
	return issues
}

// validateCompositionTitle checks a title of the schema as a localized
// string of the settings is checked, with no configuration at hand.
func validateCompositionTitle(path string, title LocalizedText) []string {
	return validateTitle(path, title, project.Project{})
}

// validateUseRestriction refuses a restriction that restricts nothing: the
// model writes none for it, so that one meaning has one writing.
func validateUseRestriction(path string, restriction *CompositionUseRestriction) []string {
	if restriction != nil && *restriction == (CompositionUseRestriction{}) {
		return []string{path + " restricts nothing and is left out"}
	}
	return nil
}

// cloneListSchema copies the fields, calculated fields and parameters of a
// dynamic list whole.
func cloneListSchema(list *DynamicListSettings) {
	list.Fields = deepCopy(reflect.ValueOf(list.Fields)).Interface().([]CompositionDataSetField)
	list.CalculatedFields = deepCopy(reflect.ValueOf(list.CalculatedFields)).Interface().([]CompositionCalculatedField)
	list.DataParameters = deepCopy(reflect.ValueOf(list.DataParameters)).Interface().([]CompositionParameter)
}

// resolveListSchema checks what the fields and parameters of a dynamic list
// refer to in the project: the objects in their types, and the values by
// name of the parameters.
func (catalog *Catalog) resolveListSchema(where string, list DynamicListSettings) error {
	for _, field := range list.Fields {
		if len(field.ValueType) != 0 {
			if err := catalog.resolveFormData(where+" field "+field.DataPath+" value type", field.ValueType, nil); err != nil {
				return err
			}
		}
	}
	for _, field := range list.CalculatedFields {
		if len(field.ValueType) != 0 {
			if err := catalog.resolveFormData(where+" calculated field "+field.DataPath+" value type", field.ValueType, nil); err != nil {
				return err
			}
		}
	}
	for _, parameter := range list.DataParameters {
		if len(parameter.ValueType) != 0 {
			if err := catalog.resolveFormData(where+" parameter "+parameter.Name+" value type", parameter.ValueType, nil); err != nil {
				return err
			}
		}
	}
	catalog.noteCompositionTypes(where+" data_parameters", list.DataParameters)
	catalog.resolvePredefinedValues(where+" data_parameters", list.DataParameters)
	return nil
}
