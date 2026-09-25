package metadata

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// FilterCriterionKind holds the named ways of going from one value to every
// place that value is used.
const FilterCriterionKind Kind = "filter-criteria"

// maxCriterionFields is where a list of searched fields stops being a list.
const maxCriterionFields = 1024

// CriterionField is one field the criterion searches: an attribute of an
// object, or an attribute of a table part of one.
type CriterionField struct {
	Kind   Kind      `yaml:"kind" json:"kind"`
	Object uuid.UUID `yaml:"object" json:"object"`
	// TablePart, when named, is the table part holding the attribute.
	TablePart *uuid.UUID `yaml:"table_part,omitempty" json:"tablePart,omitempty"`
	Attribute uuid.UUID  `yaml:"attribute" json:"attribute"`
}

// CriterionForms are the forms a criterion shows its result through. There is
// one role - the list of what was found - with an auxiliary form beside it.
type CriterionForms struct {
	List      string `yaml:"list,omitempty" json:"list,omitempty"`
	Auxiliary string `yaml:"auxiliary,omitempty" json:"auxiliary,omitempty"`
}

// FilterCriterionDefinition describes one filter criterion: a named set of
// places a value is used, gathered so that a user standing on that value can
// walk to all of them at once.
//
// Two halves. Types say what may be searched for; fields say where it is
// searched. A field that cannot hold any of those types is searched in vain -
// it never matches, and nobody is told why.
type FilterCriterionDefinition struct {
	Format      int           `yaml:"format" json:"format"`
	ID          uuid.UUID     `yaml:"id" json:"id"`
	Name        string        `yaml:"name" json:"name"`
	Title       LocalizedText `yaml:"title" json:"title"`
	Comment     string        `yaml:"comment,omitempty" json:"comment,omitempty"`
	Explanation LocalizedText `yaml:"explanation,omitempty" json:"explanation,omitempty"`
	// ListPresentation names the result of the criterion for the user.
	ListPresentation         LocalizedText `yaml:"list_presentation,omitempty" json:"listPresentation,omitempty"`
	ExtendedListPresentation LocalizedText `yaml:"extended_list_presentation,omitempty" json:"extendedListPresentation,omitempty"`
	// Types are what the criterion may be searched by.
	Types  []Type           `yaml:"types" json:"types"`
	Fields []CriterionField `yaml:"fields,omitempty" json:"fields,omitempty"`
	// UseStandardCommands decides whether the platform offers its own commands
	// for this criterion.
	UseStandardCommands bool           `yaml:"use_standard_commands,omitempty" json:"useStandardCommands,omitempty"`
	Forms               CriterionForms `yaml:"forms,omitempty" json:"forms,omitempty"`
	// A criterion has forms and commands and nothing else: it prints nothing,
	// so it keeps no templates.
	Commands []ObjectCommand `yaml:"commands,omitempty" json:"commands,omitempty"`
}

// DecodeFilterCriterion reads and validates one filter criterion.
func DecodeFilterCriterion(source string, reader io.Reader, configuration project.Project) (FilterCriterionDefinition, error) {
	var value FilterCriterionDefinition
	if err := decodeStrict(source, reader, &value); err != nil {
		return FilterCriterionDefinition{}, err
	}
	issues := validateBase(value.Format, value.ID, value.Name, value.Title, configuration)
	issues = append(issues, validateTypes("types", value.Types, value.ID)...)
	for name, text := range map[string]LocalizedText{
		"explanation": value.Explanation, "list_presentation": value.ListPresentation,
		"extended_list_presentation": value.ExtendedListPresentation,
	} {
		if len(text) > 0 {
			issues = append(issues, validateTitle(name, text, configuration)...)
		}
	}
	if len(value.Fields) > maxCriterionFields {
		issues = append(issues, fmt.Sprintf("fields must not contain more than %d items", maxCriterionFields))
	}
	seen := map[string]bool{}
	for index, field := range value.Fields {
		prefix := fmt.Sprintf("fields[%d]", index)
		if !knownMetadataKind(field.Kind) {
			issues = append(issues, prefix+".kind is not a kind of metadata object")
		}
		if field.Object.IsZero() {
			issues = append(issues, prefix+".object must be a non-zero UUID")
		}
		if field.Attribute.IsZero() {
			issues = append(issues, prefix+".attribute must be a non-zero UUID")
		}
		if field.TablePart != nil && field.TablePart.IsZero() {
			issues = append(issues, prefix+".table_part must be a non-zero UUID")
		}
		key := string(field.Kind) + ":" + field.Object.String() + ":" + field.Attribute.String()
		if seen[key] {
			issues = append(issues, prefix+" is already among the fields")
		}
		seen[key] = true
	}
	issues = append(issues, validateFormSlots(map[string]string{
		"forms.list": value.Forms.List, "forms.auxiliary": value.Forms.Auxiliary,
	})...)
	issues = append(issues, validateObjectCommands(value.Commands, value.ID, configuration)...)
	if err := issuesError(source, value.Format, issues); err != nil {
		return FilterCriterionDefinition{}, err
	}
	return value, nil
}

func cloneFilterCriterion(value FilterCriterionDefinition) FilterCriterionDefinition {
	value.Title = cloneTitle(value.Title)
	value.Explanation = cloneTitle(value.Explanation)
	value.ListPresentation = cloneTitle(value.ListPresentation)
	value.ExtendedListPresentation = cloneTitle(value.ExtendedListPresentation)
	value.Types = cloneTypes(value.Types)
	value.Fields = slices.Clone(value.Fields)
	for index := range value.Fields {
		if value.Fields[index].TablePart != nil {
			copied := *value.Fields[index].TablePart
			value.Fields[index].TablePart = &copied
		}
	}
	value.Commands = cloneObjectCommands(value.Commands)
	return value
}

// FilterCriterion returns one criterion by name, folded case.
func (catalog *Catalog) FilterCriterion(name string) (FilterCriterionDefinition, bool) {
	index, ok := catalog.filterCriterionByName[strings.ToLower(name)]
	if !ok {
		return FilterCriterionDefinition{}, false
	}
	return cloneFilterCriterion(catalog.FilterCriteria[index]), true
}

// validateFilterCriteria resolves every field a criterion searches, and checks
// that it can hold what the criterion is searched by.
func (catalog *Catalog) validateFilterCriteria() error {
	for _, criterion := range catalog.FilterCriteria {
		owner := "filter criterion " + criterion.Name
		if err := catalog.validateReferences(owner+" types", criterion.Types); err != nil {
			return err
		}
		searched, err := catalog.expandTypes(criterion.Types, nil)
		if err != nil {
			return fmt.Errorf("%s types: %w", owner, err)
		}
		for index, field := range criterion.Fields {
			where := fmt.Sprintf("%s fields[%d]", owner, index)
			types, err := catalog.criterionFieldTypes(where, field)
			if err != nil {
				return err
			}
			held, err := catalog.expandTypes(types, nil)
			if err != nil {
				return fmt.Errorf("%s: %w", where, err)
			}
			if !typesIntersect(searched, held) {
				return fmt.Errorf("%s holds nothing the criterion searches for, so it can never match", where)
			}
		}
	}
	return nil
}

// criterionFieldTypes finds the field a criterion searches and returns what it
// may hold.
func (catalog *Catalog) criterionFieldTypes(where string, field CriterionField) ([]Type, error) {
	elements, ok := catalog.objectElementsOf(field.Kind, field.Object)
	if !ok {
		return nil, fmt.Errorf("%s searches %s %s, which is not in the configuration", where, field.Kind, field.Object)
	}
	if field.TablePart == nil {
		types, ok := elements.attributes[field.Attribute]
		if !ok {
			return nil, fmt.Errorf("%s searches attribute %s, which that object does not have", where, field.Attribute)
		}
		return types, nil
	}
	attributes, ok := elements.tableParts[*field.TablePart]
	if !ok {
		return nil, fmt.Errorf("%s searches table part %s, which that object does not have", where, field.TablePart)
	}
	for _, attribute := range attributes {
		if attribute.ID == field.Attribute {
			return attribute.Types, nil
		}
	}
	return nil, fmt.Errorf("%s searches attribute %s, which that table part does not have", where, field.Attribute)
}

// typesIntersect says whether two expanded type descriptions have a type in
// common. Both sides are already expanded, so a set has become its members and
// comparing by kind and reference is comparing like with like.
func typesIntersect(left, right []Type) bool {
	keys := make(map[string]bool, len(left))
	for _, item := range left {
		key := string(item.Kind)
		if item.Reference != nil {
			key += ":" + item.Reference.String()
		}
		keys[key] = true
	}
	for _, item := range right {
		key := string(item.Kind)
		if item.Reference != nil {
			key += ":" + item.Reference.String()
		}
		if keys[key] {
			return true
		}
	}
	return false
}
