package metadata

import (
	"fmt"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"

	bslnumber "github.com/k33alexey/MetaLab/internal/bsl/number"
	"github.com/k33alexey/MetaLab/internal/project"
)

// The settings of data composition a form carries: a filter, an
// appearance, and the values they compare and set. The conditional
// appearance of a form is made of them, and so are the settings of a dynamic
// list and the variants of a report (DATA-COMPOSITION.md); they are carried
// here and executed in blocks 8 and 12.
//
// The model follows the help (DataCompositionFilterItem,
// DataCompositionFilterItemGroup, DataCompositionComparisonType,
// DataCompositionAppearance) and carries what the exports write. What the
// help names and no export writes - the identifier and presentation of a
// user setting, the view mode, the application of a filter item, a value
// list with values in it - is not carried yet; a writing the model does not
// know is refused, not dropped.

// CompositionValueKind says what a value of the settings is.
type CompositionValueKind string

const (
	// CompositionField is a field, by its path: a value compared or shown is
	// taken from it.
	CompositionField CompositionValueKind = "field"
	// CompositionBoolean, CompositionNumber, CompositionString and
	// CompositionDate are values of the primitive types, Data written as the
	// prototype writes one: "true", a decimal, the text, a date without a
	// zone.
	CompositionBoolean CompositionValueKind = "boolean"
	CompositionNumber  CompositionValueKind = "number"
	CompositionString  CompositionValueKind = "string"
	CompositionDate    CompositionValueKind = "date"
	// CompositionLocalizedString is a string in the languages of the
	// configuration, apart from a plain string: the prototype writes both,
	// the plain one for every language.
	CompositionLocalizedString CompositionValueKind = "localized-string"
	// CompositionPredefined is a value written by its name at design time:
	// a value of an enumeration, a predefined item, an empty reference -
	// "Перечисление.СтавкиНДС.НДС0", "Справочник.Склады.ПустаяСсылка". It is
	// resolved against the project by name (a point of block 2ф).
	CompositionPredefined CompositionValueKind = "predefined"
	// CompositionBeginningDate is a standard beginning date: a variant, and
	// the date itself when the variant is custom.
	CompositionBeginningDate CompositionValueKind = "standard-beginning-date"
	// CompositionValueList is a list of values. Every one the exports write
	// is empty (14 here, 25 in all the settings of forms), its values put
	// there by the code of the form; values in it are not carried.
	CompositionValueList CompositionValueKind = "value-list"
	// CompositionNull and CompositionUndefined are Null and Неопределено.
	CompositionNull      CompositionValueKind = "null"
	CompositionUndefined CompositionValueKind = "undefined"
	// CompositionColor, CompositionFont and CompositionHorizontalAlign are
	// values of the appearance.
	CompositionColor           CompositionValueKind = "color"
	CompositionFont            CompositionValueKind = "font"
	CompositionHorizontalAlign CompositionValueKind = "horizontal-align"
)

// CompositionValue is one value of the settings. Which of its fields says
// what depends on the kind: Data for a field, a primitive value, a value by
// name, an alignment and the variant of a standard beginning date; Text for
// a localized string; Date for the date of a custom beginning date; Color
// and Font for theirs. The rest is empty.
type CompositionValue struct {
	Kind  CompositionValueKind `yaml:"kind" json:"kind"`
	Data  string               `yaml:"data,omitempty" json:"data,omitempty"`
	Text  LocalizedText        `yaml:"text,omitempty" json:"text,omitempty"`
	Date  string               `yaml:"date,omitempty" json:"date,omitempty"`
	Color *ColorValue          `yaml:"color,omitempty" json:"color,omitempty"`
	Font  *FontValue           `yaml:"font,omitempty" json:"font,omitempty"`
}

// compositionDateLayout is how the prototype writes a date in the settings:
// without a zone, the empty date 0001-01-01T00:00:00 among them.
const compositionDateLayout = "2006-01-02T15:04:05"

// standardBeginningDateVariants are the variants of a standard beginning
// date (help, StandardBeginningDateVariant), as the model writes names.
var standardBeginningDateVariants = []string{
	"custom",
	"beginning-of-last-day", "beginning-of-last-week", "beginning-of-last-ten-days", "beginning-of-last-month",
	"beginning-of-last-quarter", "beginning-of-last-half-year", "beginning-of-last-year",
	"beginning-of-this-day", "beginning-of-this-week", "beginning-of-this-ten-days", "beginning-of-this-month",
	"beginning-of-this-quarter", "beginning-of-this-half-year", "beginning-of-this-year",
	"beginning-of-next-day", "beginning-of-next-week", "beginning-of-next-ten-days", "beginning-of-next-month",
	"beginning-of-next-quarter", "beginning-of-next-half-year", "beginning-of-next-year",
}

// horizontalAligns are the values of HorizontalAlign of the help.
var horizontalAligns = []string{"auto", "left", "right", "center", "justify"}

// validateCompositionValue checks a value against its kind, and the kind
// against those the place takes.
func validateCompositionValue(path string, value CompositionValue, allowed []CompositionValueKind) []string {
	if !containsKind(allowed, value.Kind) {
		names := make([]string, len(allowed))
		for index, kind := range allowed {
			names[index] = string(kind)
		}
		return []string{path + ".kind must be one of " + strings.Join(names, ", ")}
	}
	var issues []string
	data, text, date, color, font := false, false, false, false, false
	switch value.Kind {
	case CompositionField:
		data = true
		issues = append(issues, validateFormDataPath(path+".data", value.Data)...)
	case CompositionBoolean:
		data = true
		if value.Data != "true" && value.Data != "false" {
			issues = append(issues, path+".data must be true or false")
		}
	case CompositionNumber:
		data = true
		if _, err := bslnumber.Parse(value.Data); err != nil || strings.TrimSpace(value.Data) != value.Data {
			issues = append(issues, path+".data must be a decimal")
		}
	case CompositionString:
		data = true
		if !utf8.ValidString(value.Data) {
			issues = append(issues, path+".data must be valid UTF-8")
		}
	case CompositionDate:
		data = true
		if _, err := time.Parse(compositionDateLayout, value.Data); err != nil {
			issues = append(issues, path+".data must be a date written as 2006-01-02T15:04:05")
		}
	case CompositionLocalizedString:
		text = true
		issues = append(issues, validateTitle(path+".text", value.Text, project.Project{})...)
	case CompositionPredefined:
		data = true
		if !predefinedValueName(value.Data) {
			issues = append(issues, path+".data must name the value as kind, object and item, as Перечисление.СтавкиНДС.НДС0")
		}
	case CompositionBeginningDate:
		data = true
		issues = append(issues, oneOfList(path+".data", value.Data, standardBeginningDateVariants, true)...)
		// The prototype writes the date of a custom one only, and always.
		date = value.Data == "custom"
		if _, err := time.Parse(compositionDateLayout, value.Date); date && err != nil {
			issues = append(issues, path+".date must be a date written as 2006-01-02T15:04:05")
		}
	case CompositionHorizontalAlign:
		data = true
		issues = append(issues, oneOfList(path+".data", value.Data, horizontalAligns, true)...)
	case CompositionColor:
		color = true
		if value.Color == nil {
			issues = append(issues, path+".color must say what the colour is")
		} else {
			issues = append(issues, validateColorValue(path+".color", *value.Color)...)
		}
	case CompositionFont:
		font = true
		if value.Font == nil {
			issues = append(issues, path+".font must say what the font is")
		} else {
			issues = append(issues, validateFontValue(path+".font", *value.Font)...)
		}
	}
	for _, field := range []struct {
		name  string
		used  bool
		empty bool
	}{{"data", data, value.Data == ""}, {"text", text, len(value.Text) == 0}, {"date", date, value.Date == ""},
		{"color", color, value.Color == nil}, {"font", font, value.Font == nil}} {
		if !field.used && !field.empty {
			issues = append(issues, path+"."+field.name+" does not belong to a value of kind "+string(value.Kind))
		}
	}
	return issues
}

func containsKind(list []CompositionValueKind, kind CompositionValueKind) bool {
	for _, item := range list {
		if item == kind {
			return true
		}
	}
	return false
}

// predefinedValueName is the shape of a value by name: the kind of the
// object, the object and the item, each a name.
func predefinedValueName(value string) bool {
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return false
	}
	for _, part := range parts {
		if !validIdentifier(part) || utf8.RuneCountInString(part) > maxNameLength {
			return false
		}
	}
	return true
}

// CompositionComparison is how a filter item compares (help,
// DataCompositionComparisonType).
type CompositionComparison string

var compositionComparisons = []CompositionComparison{
	"equal", "not-equal", "less", "less-or-equal", "greater", "greater-or-equal",
	"in-list", "not-in-list", "in-list-by-hierarchy", "not-in-list-by-hierarchy", "in-hierarchy", "not-in-hierarchy",
	"contains", "not-contains", "begins-with", "not-begins-with", "like", "not-like", "filled", "not-filled",
}

// CompositionFilterGroupType is how a group of filter items joins them
// (help, DataCompositionFilterItemsGroupType).
type CompositionFilterGroupType string

var compositionFilterGroupTypes = []CompositionFilterGroupType{"and", "or", "not"}

// CompositionFilterItem is one item of a filter: a comparison, or a group of
// items joined by Group. A comparison compares Left with Right; Right is
// left out where the prototype writes none - always for filled and not
// filled, 11 times beside another comparison, which then compares with
// nothing set. Presentation is what the user sees for the item instead of
// the comparison.
type CompositionFilterItem struct {
	Group        CompositionFilterGroupType `yaml:"group,omitempty" json:"group,omitempty"`
	Items        []CompositionFilterItem    `yaml:"items,omitempty" json:"items,omitempty"`
	Left         *CompositionValue          `yaml:"left,omitempty" json:"left,omitempty"`
	Comparison   CompositionComparison      `yaml:"comparison,omitempty" json:"comparison,omitempty"`
	Right        *CompositionValue          `yaml:"right,omitempty" json:"right,omitempty"`
	Disabled     bool                       `yaml:"disabled,omitempty" json:"disabled,omitempty"`
	Presentation *CompositionValue          `yaml:"presentation,omitempty" json:"presentation,omitempty"`
}

// compositionOperands are the kinds a filter compares.
var compositionOperands = []CompositionValueKind{CompositionField, CompositionBoolean, CompositionNumber, CompositionString,
	CompositionDate, CompositionPredefined, CompositionBeginningDate, CompositionValueList, CompositionNull, CompositionUndefined}

// compositionTexts are the kinds a presentation is written in.
var compositionTexts = []CompositionValueKind{CompositionString, CompositionLocalizedString}

func validateCompositionFilter(path string, items []CompositionFilterItem) []string {
	var issues []string
	for index, item := range items {
		at := fmt.Sprintf("%s[%d]", path, index)
		if item.Group != "" {
			issues = append(issues, oneOfList(at+".group", item.Group, compositionFilterGroupTypes, true)...)
			if item.Left != nil || item.Comparison != "" || item.Right != nil {
				issues = append(issues, at+" is a group and compares nothing itself")
			}
			issues = append(issues, validateCompositionFilter(at+".items", item.Items)...)
		} else {
			if len(item.Items) != 0 {
				issues = append(issues, at+".items belong to a group")
			}
			issues = append(issues, oneOfList(at+".comparison", item.Comparison, compositionComparisons, true)...)
			if item.Left == nil {
				issues = append(issues, at+".left must say what is compared")
			} else {
				issues = append(issues, validateCompositionValue(at+".left", *item.Left, compositionOperands)...)
			}
			if item.Right != nil {
				issues = append(issues, validateCompositionValue(at+".right", *item.Right, compositionOperands)...)
			}
		}
		if item.Presentation != nil {
			issues = append(issues, validateCompositionValue(at+".presentation", *item.Presentation, compositionTexts)...)
		}
	}
	return issues
}

// AppearanceParameter is a parameter of the appearance (help,
// DataCompositionAppearance), as the model writes names; the prototype
// writes it in the language of the configuration (ЦветТекста).
type AppearanceParameter string

// appearanceParameters are the parameters the conditional appearance of the
// forms of the exports sets, each with the kinds of its value. The help
// names 45; the rest come with the settings that set them.
var appearanceParameters = map[AppearanceParameter][]CompositionValueKind{
	"text-color":       {CompositionColor},
	"back-color":       {CompositionColor},
	"font":             {CompositionFont},
	"horizontal-align": {CompositionHorizontalAlign},
	"visible":          {CompositionBoolean},
	"enabled":          {CompositionBoolean},
	"read-only":        {CompositionBoolean},
	"show":             {CompositionBoolean},
	"mark-incomplete":  {CompositionBoolean},
	"mark-negatives":   {CompositionBoolean},
	// A text is a string, in languages or for all of them, or the value of
	// a field shown instead; left as Неопределено (22 times, each turned
	// off) it says nothing.
	"text":   {CompositionLocalizedString, CompositionString, CompositionField, CompositionUndefined},
	"format": {CompositionLocalizedString, CompositionString},
}

// CompositionAppearanceValue is one parameter of an appearance set to a
// value. Disabled keeps the value without setting it: the prototype writes
// such a parameter as it was before it was turned off (591 times).
type CompositionAppearanceValue struct {
	Parameter AppearanceParameter `yaml:"parameter" json:"parameter"`
	Disabled  bool                `yaml:"disabled,omitempty" json:"disabled,omitempty"`
	Value     CompositionValue    `yaml:"value" json:"value"`
}

func validateCompositionAppearance(path string, values []CompositionAppearanceValue) []string {
	var issues []string
	set := map[AppearanceParameter]bool{}
	for index, value := range values {
		at := fmt.Sprintf("%s[%d]", path, index)
		kinds, known := appearanceParameters[value.Parameter]
		if !known {
			issues = append(issues, at+".parameter is not a parameter of the appearance the model carries")
			continue
		}
		if set[value.Parameter] {
			issues = append(issues, at+".parameter "+string(value.Parameter)+" is set twice")
		}
		set[value.Parameter] = true
		issues = append(issues, validateCompositionValue(at+".value", value.Value, kinds)...)
	}
	return issues
}

// CompositionAppearanceField is a field an item of a conditional appearance
// draws; Disabled keeps it in the item without drawing it (19 times).
type CompositionAppearanceField struct {
	Field    string `yaml:"field" json:"field"`
	Disabled bool   `yaml:"disabled,omitempty" json:"disabled,omitempty"`
}

// ConditionalAppearanceItem is one item of a conditional appearance (help,
// DataCompositionConditionalAppearanceItem): the appearance it sets on its
// fields where its filter holds. An item with no fields (17 of the forms of
// the exports) and one with no filter (7) are written so by the prototype
// and are carried as they are; what they draw is a question of executing
// them (block 8). The help names twelve more properties - where in a report
// the item applies, the user setting and the view mode; no form of the
// exports writes one.
type ConditionalAppearanceItem struct {
	Disabled     bool                         `yaml:"disabled,omitempty" json:"disabled,omitempty"`
	Fields       []CompositionAppearanceField `yaml:"fields,omitempty" json:"fields,omitempty"`
	Filter       []CompositionFilterItem      `yaml:"filter,omitempty" json:"filter,omitempty"`
	Appearance   []CompositionAppearanceValue `yaml:"appearance" json:"appearance"`
	Presentation *CompositionValue            `yaml:"presentation,omitempty" json:"presentation,omitempty"`
}

// validateConditionalAppearance checks a conditional appearance against
// itself. The fields of an item are names of elements of the form; one
// naming none is a remnant, found when the project is loaded
// (resolveConditionalAppearance).
func validateConditionalAppearance(path string, items []ConditionalAppearanceItem) []string {
	var issues []string
	for index, item := range items {
		at := fmt.Sprintf("%s[%d]", path, index)
		for position, field := range item.Fields {
			if !validIdentifier(field.Field) || utf8.RuneCountInString(field.Field) > maxNameLength {
				issues = append(issues, fmt.Sprintf("%s.fields[%d].field must be the name of an element of the form", at, position))
			}
		}
		issues = append(issues, validateCompositionFilter(at+".filter", item.Filter)...)
		if len(item.Appearance) == 0 {
			issues = append(issues, at+".appearance must set at least one parameter")
		}
		issues = append(issues, validateCompositionAppearance(at+".appearance", item.Appearance)...)
		if item.Presentation != nil {
			issues = append(issues, validateCompositionValue(at+".presentation", *item.Presentation, compositionTexts)...)
		}
	}
	return issues
}

// cloneConditionalAppearance copies a conditional appearance whole: nothing
// the copy holds is shared.
func cloneConditionalAppearance(items []ConditionalAppearanceItem) []ConditionalAppearanceItem {
	return deepCopy(reflect.ValueOf(items)).Interface().([]ConditionalAppearanceItem)
}

// conditionalAppearanceParts walks a conditional appearance for what its
// values take from style items.
func conditionalAppearanceParts(items []ConditionalAppearanceItem) func(visit func(path string, part any)) {
	return func(visit func(path string, part any)) {
		walkChartParts(reflect.ValueOf(items), visit)
	}
}

// resolveConditionalAppearance checks what the conditional appearance of a
// form refers to: the style items its colours and fonts are taken from, as
// anywhere in a form, and the elements it draws. A field naming no element
// of the form is what is left of an element deleted with its appearance
// left behind - 567 of 5319 fields in the exports, none of them made by
// the code of its form - and is a remnant, as a reference to a deleted
// object is (2.203). The import is to drop it and, when it was the last
// field of its item, the item with it: an item with no fields draws
// something else.
func (catalog *Catalog) resolveConditionalAppearance(where string, form ManagedForm) error {
	if len(form.ConditionalAppearance) == 0 {
		return nil
	}
	if err := catalog.resolveStyleItems(where+" conditional appearance", styleItemsIn(conditionalAppearanceParts(form.ConditionalAppearance))); err != nil {
		return err
	}
	names := map[string]bool{}
	var walk func(items []ManagedFormElement)
	walk = func(items []ManagedFormElement) {
		for _, item := range items {
			names[foldedName(item.Name)] = true
			walk(item.Nested())
		}
	}
	walk(form.FormItems())
	for index, item := range form.ConditionalAppearance {
		for _, field := range item.Fields {
			if !names[foldedName(field.Field)] {
				catalog.unresolved = append(catalog.unresolved, UnresolvedReference{
					Where: fmt.Sprintf("%s conditional appearance[%d] field", where, index), Written: field.Field})
			}
		}
	}
	return nil
}
