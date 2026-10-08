package metadata

import (
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	bslnumber "github.com/k33alexey/MetaLab/internal/bsl/number"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// The settings of data composition a form carries: a filter, an
// appearance, and the values they compare and set. The conditional
// appearance of a form is made of them, and so are the settings of a dynamic
// list and the variants of a report (DATA-COMPOSITION.md); they are carried
// here and executed in blocks 8 and 12.
//
// The model follows the help (DataCompositionFilterItem,
// DataCompositionFilterItemGroup, DataCompositionComparisonType,
// DataCompositionAppearance, DataCompositionOrder, DataCompositionGroup) and
// carries what the exports write. What the help names and no export writes -
// the application of a filter item, a value list with values in it, the
// selection, filter and order of a group - is not carried yet; a writing the
// model does not know is refused, not dropped.

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
	// "Перечисление.СтавкиНДС.НДС0", "Справочник.Склады.ПустаяСсылка". Written
	// with two parts, "Документ.ЧекККМ", it names an object, and the value is
	// the type of its reference: the settings of dynamic lists compare a type
	// of a row with it (26 values, each compared with the field Тип or
	// ТипДокумента). It is resolved against the project by name
	// (hasPredefinedValue).
	CompositionPredefined CompositionValueKind = "predefined"
	// CompositionType is a type, Data its name as the prototype writes it in
	// the namespace of the types of the configuration. The exports write one
	// name only, Undefined (8 values in the settings of dynamic lists), and
	// what it stands for there is not known: it is carried as written and
	// noted (composition-type-unexplained).
	CompositionType CompositionValueKind = "type"
	// CompositionAccountType is a type of account (help, AccountType), Data
	// active, passive or active-passive.
	CompositionAccountType CompositionValueKind = "account-type"
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

// accountTypes are the values of AccountType of the help.
var accountTypes = []string{"active", "passive", "active-passive"}

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
		if _, _, ok := parsePredefinedValue(value.Data); !ok {
			issues = append(issues, path+".data must name the value as kind, object and item, as Перечисление.СтавкиНДС.НДС0, or an object as kind and object, as Документ.ЧекККМ; a document, a business process and a task have the empty reference alone")
		}
	case CompositionType:
		data = true
		if validateFormDataPath(path+".data", value.Data) != nil {
			issues = append(issues, path+".data must be the name of a type")
		}
	case CompositionAccountType:
		data = true
		issues = append(issues, oneOfList(path+".data", value.Data, accountTypes, true)...)
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

// predefinedValueKind is a kind of object a value by name is written of
// (help, PredefinedValue): its name in either language of the code, the
// collection of the catalog it is found in, and whether a value of it is an
// item - a value of an enumeration, a predefined item - or an empty
// reference only, which a document, a business process and a task have and
// nothing else.
type predefinedValueKind struct {
	russian, english, collection string
	items                        bool
}

var predefinedValueKinds = []predefinedValueKind{
	{"Перечисление", "Enum", "Enumerations", true},
	{"Справочник", "Catalog", "Catalogs", true},
	{"ПланВидовХарактеристик", "ChartOfCharacteristicTypes", "ChartsOfCharacteristicTypes", true},
	{"ПланСчетов", "ChartOfAccounts", "ChartsOfAccounts", true},
	{"ПланВидовРасчета", "ChartOfCalculationTypes", "ChartsOfCalculationTypes", true},
	{"Документ", "Document", "Documents", false},
	{"БизнесПроцесс", "BusinessProcess", "BusinessProcesses", false},
	{"Задача", "Task", "Tasks", false},
}

// isEmptyReferenceName says a value by name is the empty reference of its
// object.
func isEmptyReferenceName(name string) bool {
	return strings.EqualFold(name, "ПустаяСсылка") || strings.EqualFold(name, "EmptyRef")
}

// parsePredefinedValue reads a value by name: the kind of the object, the
// object and the item, each a name; or the kind and the object alone, the
// type of a reference. The kind is one the help names, and an object that
// has no items has the empty reference alone. A point of a route of a
// business process, which the help writes with four parts, no export writes,
// and it is not carried.
func parsePredefinedValue(value string) (predefinedValueKind, []string, bool) {
	parts := strings.Split(value, ".")
	if len(parts) != 2 && len(parts) != 3 {
		return predefinedValueKind{}, nil, false
	}
	for _, part := range parts {
		if !validIdentifier(part) || utf8.RuneCountInString(part) > maxNameLength {
			return predefinedValueKind{}, nil, false
		}
	}
	for _, kind := range predefinedValueKinds {
		if strings.EqualFold(parts[0], kind.russian) || strings.EqualFold(parts[0], kind.english) {
			if len(parts) == 3 && !kind.items && !isEmptyReferenceName(parts[2]) {
				return predefinedValueKind{}, nil, false
			}
			return kind, parts, true
		}
	}
	return predefinedValueKind{}, nil, false
}

// hasPredefinedValue answers whether the project has what a value by name
// names: the object, and its value or predefined item, or its empty
// reference. Names are compared as the platform compares them, whatever
// their case: erp writes Незапущен for the value НеЗапущен.
func (catalog *Catalog) hasPredefinedValue(value string) bool {
	kind, parts, ok := parsePredefinedValue(value)
	if !ok {
		return false
	}
	list := reflect.ValueOf(catalog).Elem().FieldByName(kind.collection)
	for index := range list.Len() {
		definition := list.Index(index)
		if !strings.EqualFold(definition.FieldByName("Name").String(), parts[1]) {
			continue
		}
		if len(parts) == 2 || isEmptyReferenceName(parts[2]) {
			return true
		}
		items := definition.FieldByName("Predefined")
		if kind.collection == "Enumerations" {
			items = definition.FieldByName("Values")
		}
		for position := range items.Len() {
			if strings.EqualFold(items.Index(position).FieldByName("Name").String(), parts[2]) {
				return true
			}
		}
		return false
	}
	return false
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
// items joined by Group. A comparison compares Left with Right. Right is
// one value, or several for a comparison with a list, which the prototype
// writes as values one after another (96 comparisons, two to seven values
// each). It is left out where the prototype writes none - always for filled
// and not filled, 11 times beside another comparison, which then compares
// with nothing set. A field with no path on the right is a field not chosen
// (3 times in the settings of lists), and compares with nothing set as well.
// Presentation is what the user sees for the item instead of the
// comparison.
type CompositionFilterItem struct {
	Group        CompositionFilterGroupType `yaml:"group,omitempty" json:"group,omitempty"`
	Items        []CompositionFilterItem    `yaml:"items,omitempty" json:"items,omitempty"`
	Left         *CompositionValue          `yaml:"left,omitempty" json:"left,omitempty"`
	Comparison   CompositionComparison      `yaml:"comparison,omitempty" json:"comparison,omitempty"`
	Right        []CompositionValue         `yaml:"right,omitempty" json:"right,omitempty"`
	Disabled     bool                       `yaml:"disabled,omitempty" json:"disabled,omitempty"`
	Presentation *CompositionValue          `yaml:"presentation,omitempty" json:"presentation,omitempty"`
	UserSetting  `yaml:",inline"`
}

// compositionOperands are the kinds a filter compares.
var compositionOperands = []CompositionValueKind{CompositionField, CompositionBoolean, CompositionNumber, CompositionString,
	CompositionDate, CompositionPredefined, CompositionType, CompositionAccountType, CompositionBeginningDate,
	CompositionValueList, CompositionNull, CompositionUndefined}

// compositionListComparisons are the comparisons with a list, the only ones
// that take several values.
var compositionListComparisons = []CompositionComparison{"in-list", "not-in-list", "in-list-by-hierarchy", "not-in-list-by-hierarchy"}

// CompositionViewMode is how a setting is shown to the user (help,
// DataCompositionSettingsItemViewMode).
type CompositionViewMode string

var compositionViewModes = []CompositionViewMode{"auto", "quick-access", "inaccessible", "normal"}

// UserSetting is what makes an item of the settings a user setting (help,
// the properties ViewMode, UserSettingID and UserSettingPresentation of an
// item of a filter, an order, a conditional appearance and their
// collections). An empty ViewMode is one the prototype does not write: the
// exports write normal and inaccessible only, and leave it out on 408 of
// 491 items of a filter at the top of the settings of a list; what an
// unwritten one means is a question of executing it (block 8). ID is the
// identifier the user setting is kept by, and it is not unique: the
// prototype gives the same one to settings of different forms.
type UserSetting struct {
	ViewMode                CompositionViewMode `yaml:"view_mode,omitempty" json:"viewMode,omitempty"`
	UserSettingID           *uuid.UUID          `yaml:"user_setting_id,omitempty" json:"userSettingId,omitempty"`
	UserSettingPresentation *CompositionValue   `yaml:"user_setting_presentation,omitempty" json:"userSettingPresentation,omitempty"`
}

func validateUserSetting(path string, value UserSetting) []string {
	issues := oneOfList(path+".view_mode", value.ViewMode, compositionViewModes, false)
	if value.UserSettingID != nil && value.UserSettingID.IsZero() {
		issues = append(issues, path+".user_setting_id must be a non-zero UUID")
	}
	if value.UserSettingPresentation != nil {
		issues = append(issues, validateCompositionValue(path+".user_setting_presentation", *value.UserSettingPresentation, compositionTexts)...)
	}
	return issues
}

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
			if len(item.Right) > 1 && !slices.Contains(compositionListComparisons, item.Comparison) {
				issues = append(issues, at+".right holds one value: only a comparison with a list takes several")
			}
			for position, right := range item.Right {
				if right.Kind == CompositionField && reflect.DeepEqual(right, CompositionValue{Kind: CompositionField}) {
					continue
				}
				issues = append(issues, validateCompositionValue(fmt.Sprintf("%s.right[%d]", at, position), right, compositionOperands)...)
			}
		}
		issues = append(issues, validateUserSetting(at, item.UserSetting)...)
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
	Parameter   AppearanceParameter `yaml:"parameter" json:"parameter"`
	Disabled    bool                `yaml:"disabled,omitempty" json:"disabled,omitempty"`
	Value       CompositionValue    `yaml:"value" json:"value"`
	UserSetting `yaml:",inline"`
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
		issues = append(issues, validateUserSetting(at, value.UserSetting)...)
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
// them (block 8). So is an item that sets nothing, empty all through (2 in
// the settings of lists of lombard1): it draws nothing. The help names nine
// more properties - where in a report
// the item applies; no form of the exports writes one.
type ConditionalAppearanceItem struct {
	Disabled     bool                         `yaml:"disabled,omitempty" json:"disabled,omitempty"`
	Fields       []CompositionAppearanceField `yaml:"fields,omitempty" json:"fields,omitempty"`
	Filter       []CompositionFilterItem      `yaml:"filter,omitempty" json:"filter,omitempty"`
	Appearance   []CompositionAppearanceValue `yaml:"appearance" json:"appearance"`
	Presentation *CompositionValue            `yaml:"presentation,omitempty" json:"presentation,omitempty"`
	UserSetting  `yaml:",inline"`
}

// validateConditionalAppearance checks a conditional appearance against
// itself. The fields of an item of the appearance of a form are names of
// its elements, one naming none a remnant found when the project is loaded
// (resolveConditionalAppearance); those of a dynamic list are fields of the
// list, by their path.
func validateConditionalAppearance(path string, items []ConditionalAppearanceItem, listFields bool) []string {
	var issues []string
	for index, item := range items {
		at := fmt.Sprintf("%s[%d]", path, index)
		for position, field := range item.Fields {
			place := fmt.Sprintf("%s.fields[%d].field", at, position)
			if listFields {
				if validateFormDataPath(place, field.Field) != nil {
					issues = append(issues, place+" must be the path of a field of the list")
				}
			} else if !validIdentifier(field.Field) || utf8.RuneCountInString(field.Field) > maxNameLength {
				issues = append(issues, place+" must be the name of an element of the form")
			}
		}
		issues = append(issues, validateCompositionFilter(at+".filter", item.Filter)...)
		issues = append(issues, validateCompositionAppearance(at+".appearance", item.Appearance)...)
		if item.Presentation != nil {
			issues = append(issues, validateCompositionValue(at+".presentation", *item.Presentation, compositionTexts)...)
		}
		issues = append(issues, validateUserSetting(at, item.UserSetting)...)
	}
	return issues
}

// cloneListComposition copies the settings of a dynamic list whole.
func cloneListComposition(list *DynamicListSettings) {
	list.Filter = deepCopy(reflect.ValueOf(list.Filter)).Interface().(*CompositionFilter)
	list.Order = deepCopy(reflect.ValueOf(list.Order)).Interface().(*CompositionOrder)
	list.ConditionalAppearance = deepCopy(reflect.ValueOf(list.ConditionalAppearance)).Interface().(*CompositionConditionalAppearance)
	list.Group = deepCopy(reflect.ValueOf(list.Group)).Interface().(*CompositionGroups)
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
	catalog.noteCompositionTypes(where+" conditional appearance", form.ConditionalAppearance)
	catalog.resolvePredefinedValues(where+" conditional appearance", form.ConditionalAppearance)
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

// CompositionFilter, CompositionOrder, CompositionConditionalAppearance and
// CompositionGroups are the collections of the settings of a dynamic list
// (help, DynamicList.Filter, Order, ConditionalAppearance and Group): their
// items, and the user setting of the collection itself. The prototype
// writes all four for nearly every list, most with no items and with the
// view mode and identifier of the collection alone; nil is a collection it
// does not write.
type CompositionFilter struct {
	Items       []CompositionFilterItem `yaml:"items,omitempty" json:"items,omitempty"`
	UserSetting `yaml:",inline"`
}

type CompositionOrder struct {
	Items       []CompositionOrderItem `yaml:"items,omitempty" json:"items,omitempty"`
	UserSetting `yaml:",inline"`
}

type CompositionConditionalAppearance struct {
	Items       []ConditionalAppearanceItem `yaml:"items,omitempty" json:"items,omitempty"`
	UserSetting `yaml:",inline"`
}

type CompositionGroups struct {
	Items       []CompositionGroup `yaml:"items,omitempty" json:"items,omitempty"`
	UserSetting `yaml:",inline"`
}

// CompositionOrderDirection is how an order sorts by a field (help,
// DataCompositionSortDirection).
type CompositionOrderDirection string

var compositionOrderDirections = []CompositionOrderDirection{"asc", "desc"}

// CompositionOrderItem is one item of an order: a field and the direction
// sorted in, or an automatic item (help, DataCompositionAutoOrderItem),
// which the platform turns into the fields of the grouping and has nothing
// but its use.
type CompositionOrderItem struct {
	Auto        bool                      `yaml:"auto,omitempty" json:"auto,omitempty"`
	Field       string                    `yaml:"field,omitempty" json:"field,omitempty"`
	Direction   CompositionOrderDirection `yaml:"direction,omitempty" json:"direction,omitempty"`
	Disabled    bool                      `yaml:"disabled,omitempty" json:"disabled,omitempty"`
	UserSetting `yaml:",inline"`
}

// CompositionGroupType is what a field groups by (help,
// DataCompositionGroupType).
type CompositionGroupType string

var compositionGroupTypes = []CompositionGroupType{"items", "hierarchy", "hierarchy-only"}

// compositionPeriodAdditions are the values of
// DataCompositionPeriodAdditionType of the help: how the periods with no
// data of a grouping by a date are added.
var compositionPeriodAdditions = []string{
	"none", "second", "minute", "hour", "day", "week", "ten-days", "month", "quarter", "half-year", "year",
	"minute-since-begin-of-period", "hour-since-begin-of-period", "day-since-begin-of-period", "week-since-begin-of-period",
	"month-since-begin-of-period", "month-since-begin-of-period-445", "quarter-since-begin-of-period",
	"quarter-since-begin-of-period-445", "half-year-since-begin-of-period", "half-year-since-begin-of-period-445",
	"year-since-begin-of-period", "year-since-begin-of-period-445",
}

// CompositionGroup is a grouping of a dynamic list (help,
// DataCompositionGroup): the fields it groups by, and the groupings nested
// in it. The help gives a grouping its own selection, filter, order,
// appearance and output parameters as well; the exports write none of them
// (32 groupings in 26 lists), and they are not carried yet.
type CompositionGroup struct {
	Fields []CompositionGroupField `yaml:"fields,omitempty" json:"fields,omitempty"`
	Groups []CompositionGroup      `yaml:"groups,omitempty" json:"groups,omitempty"`
}

// CompositionGroupField is a field a grouping groups by (help,
// DataCompositionGroupField). PeriodBegin and PeriodEnd bound the periods
// added; the prototype writes them always, the empty date where they are
// not set, and the model leaves an empty date out.
type CompositionGroupField struct {
	Field          string               `yaml:"field" json:"field"`
	Disabled       bool                 `yaml:"disabled,omitempty" json:"disabled,omitempty"`
	Type           CompositionGroupType `yaml:"type" json:"type"`
	PeriodAddition string               `yaml:"period_addition" json:"periodAddition"`
	PeriodBegin    string               `yaml:"period_begin,omitempty" json:"periodBegin,omitempty"`
	PeriodEnd      string               `yaml:"period_end,omitempty" json:"periodEnd,omitempty"`
}

// validateListComposition checks the settings of a dynamic list against
// themselves. The fields they name are fields of the list, resolved against
// its query when it is executed (block 7).
func validateListComposition(path string, settings DynamicListSettings) []string {
	var issues []string
	if filter := settings.Filter; filter != nil {
		issues = append(issues, validateUserSetting(path+".filter", filter.UserSetting)...)
		issues = append(issues, validateCompositionFilter(path+".filter.items", filter.Items)...)
	}
	if order := settings.Order; order != nil {
		issues = append(issues, validateUserSetting(path+".order", order.UserSetting)...)
		for index, item := range order.Items {
			at := fmt.Sprintf("%s.order.items[%d]", path, index)
			if item.Auto {
				if item.Field != "" || item.Direction != "" || item.UserSetting != (UserSetting{}) {
					issues = append(issues, at+" is automatic and has nothing but its use")
				}
				continue
			}
			issues = append(issues, validateFormDataPath(at+".field", item.Field)...)
			issues = append(issues, oneOfList(at+".direction", item.Direction, compositionOrderDirections, true)...)
			issues = append(issues, validateUserSetting(at, item.UserSetting)...)
		}
	}
	if appearance := settings.ConditionalAppearance; appearance != nil {
		issues = append(issues, validateUserSetting(path+".conditional_appearance", appearance.UserSetting)...)
		issues = append(issues, validateConditionalAppearance(path+".conditional_appearance.items", appearance.Items, true)...)
	}
	if groups := settings.Group; groups != nil {
		issues = append(issues, validateUserSetting(path+".group", groups.UserSetting)...)
		issues = append(issues, validateCompositionGroups(path+".group.items", groups.Items)...)
	}
	return issues
}

func validateCompositionGroups(path string, groups []CompositionGroup) []string {
	var issues []string
	for index, group := range groups {
		at := fmt.Sprintf("%s[%d]", path, index)
		for position, field := range group.Fields {
			place := fmt.Sprintf("%s.fields[%d]", at, position)
			issues = append(issues, validateFormDataPath(place+".field", field.Field)...)
			issues = append(issues, oneOfList(place+".type", field.Type, compositionGroupTypes, true)...)
			issues = append(issues, oneOfList(place+".period_addition", field.PeriodAddition, compositionPeriodAdditions, true)...)
			for _, bound := range []struct{ name, date string }{{"period_begin", field.PeriodBegin}, {"period_end", field.PeriodEnd}} {
				if _, err := time.Parse(compositionDateLayout, bound.date); bound.date != "" && err != nil {
					issues = append(issues, place+"."+bound.name+" must be a date written as 2006-01-02T15:04:05")
				}
			}
		}
		issues = append(issues, validateCompositionGroups(at+".groups", group.Groups)...)
	}
	return issues
}

// compositionValuesIn names the places of the values of a kind in the
// settings, each with its data.
func compositionValuesIn(root any, kind CompositionValueKind) map[string]string {
	found := map[string]string{}
	var walk func(path string, current reflect.Value)
	walk = func(path string, current reflect.Value) {
		switch current.Kind() {
		case reflect.Pointer:
			if !current.IsNil() {
				walk(path, current.Elem())
			}
		case reflect.Slice:
			for index := range current.Len() {
				walk(fmt.Sprintf("%s[%d]", path, index), current.Index(index))
			}
		case reflect.Struct:
			if value, ok := current.Interface().(CompositionValue); ok {
				if value.Kind == kind {
					found[path] = value.Data
				}
				return
			}
			for index := range current.NumField() {
				name, _, _ := strings.Cut(current.Type().Field(index).Tag.Get("yaml"), ",")
				next := path
				if name != "" {
					next = strings.TrimPrefix(path+"."+name, ".")
				}
				walk(next, current.Field(index))
			}
		}
	}
	walk("", reflect.ValueOf(root))
	return found
}

// resolveListComposition checks what the settings of a dynamic list refer to
// in the project: the style items of their appearance; and notes the values
// of kind type their filters compare with. The fields they name are fields
// of the list, resolved when it is executed; an order and a grouping hold
// nothing else.
func (catalog *Catalog) resolveListComposition(where string, list DynamicListSettings) error {
	if err := catalog.resolveStyleItems(where+" conditional_appearance", styleItemsIn(func(visit func(path string, part any)) {
		walkChartParts(reflect.ValueOf(list.ConditionalAppearance), visit)
	})); err != nil {
		return err
	}
	catalog.noteCompositionTypes(where+" filter", list.Filter)
	catalog.noteCompositionTypes(where+" conditional_appearance", list.ConditionalAppearance)
	catalog.resolvePredefinedValues(where+" filter", list.Filter)
	catalog.resolvePredefinedValues(where+" conditional_appearance", list.ConditionalAppearance)
	return nil
}

// resolvePredefinedValues checks each value by name in the settings against
// the project. One the project does not have - an enumeration, a value of
// it, a predefined item deleted with the settings left behind - is a
// remnant, as a reference to a deleted object is (2.203): 101 of the 1032
// values of the exports, 39 of them of an enumeration that is gone, 52 of
// one a configuration keeps with no values at all, 10 a value or item
// deleted from an object that has others.
func (catalog *Catalog) resolvePredefinedValues(where string, settings any) {
	found := compositionValuesIn(settings, CompositionPredefined)
	for _, path := range slices.Sorted(maps.Keys(found)) {
		if !catalog.hasPredefinedValue(found[path]) {
			catalog.unresolved = append(catalog.unresolved, UnresolvedReference{Where: where + " " + path, Written: found[path]})
		}
	}
}

// noteCompositionTypes notes each value of kind type in the settings: what
// the one name the exports write stands for is not known.
func (catalog *Catalog) noteCompositionTypes(where string, settings any) {
	found := compositionValuesIn(settings, CompositionType)
	paths := slices.Sorted(maps.Keys(found))
	for _, path := range paths {
		catalog.noteForm(NoteCompositionTypeUnexplained, where+" "+path, found[path])
	}
}
