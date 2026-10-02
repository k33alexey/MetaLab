package metadata

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// A field is not only a type in a column. The prototype gives every attribute,
// every dimension and every resource the same set of settings for how its
// value is shown, entered and picked, and a configuration fills them in: the
// date that is entered as a day and shown as a day and a time, the number that
// turns red below zero, the field that is picked from a list already narrowed
// by what stands in the field above it.
//
// None of it changes what is stored, and all of it changes what the user sees
// and can do. Dropped on the way in, it is the second worst outcome of the
// conformance report: the object is there, the data is there, and the form
// behaves like somebody else's.

// UsageMode is the platform's three-valued switch: let the platform decide, or
// decide for it either way. It is not a boolean with a default, because "the
// platform decides" is an answer of its own and the platform's decision
// depends on the type of the field.
type UsageMode string

const (
	UsageAuto    UsageMode = "auto"
	UsageUse     UsageMode = "use"
	UsageDontUse UsageMode = "dont-use"
)

func validUsageMode(mode UsageMode) bool {
	switch mode {
	case "", UsageAuto, UsageUse, UsageDontUse:
		return true
	default:
		return false
	}
}

// ChoiceTarget says what a choice from a hierarchical object may land on: a
// folder, an item, or either. A field holding a folder where the application
// expects an item is a wrong value that passes every other check.
type ChoiceTarget string

const (
	ChoiceTargetAuto            ChoiceTarget = "auto"
	ChoiceTargetItems           ChoiceTarget = "items"
	ChoiceTargetFolders         ChoiceTarget = "folders"
	ChoiceTargetFoldersAndItems ChoiceTarget = "folders-and-items"
)

// Where a list of choice parameters stops being a list, and where a parameter
// name stops being a name.
const (
	maxChoiceParameterLen = 256
)

func validChoiceTarget(value ChoiceTarget) bool {
	switch value {
	case "", ChoiceTargetAuto, ChoiceTargetItems, ChoiceTargetFolders, ChoiceTargetFoldersAndItems:
		return true
	default:
		return false
	}
}

// ValueChange says what happens to this field when the field it takes a choice
// parameter from changes: the value picked under the old parameter is cleared,
// or it is left alone. Clearing is the platform's own default, and it is the
// answer that keeps the two fields agreeing with each other.
type ValueChange string

const (
	ValueChangeClear      ValueChange = "clear"
	ValueChangeDontChange ValueChange = "dont-change"
)

// FieldPath is a field of the same object: an attribute of it, or an attribute
// of one of its table parts. A link that works stays inside the object that
// draws it.
//
// A link the prototype saves may lead elsewhere all the same, and such a path
// is carried as it was written, in Unresolved, and does nothing. The
// configurations being moved have 1468 of them naming a field of another
// object by its identifier - "0:<table part>/0:<attribute>", a field copied
// from a document into a data processor with the link still pointing at the
// document - and 202 written as a number, "0", "-5", "0/0", that name no
// field at all (erp, acc, sb; owner, 01.10.2026: carry, do not execute). A
// running form resolves a path by its attribute or standard field, and an
// unresolved path has neither, so it filters nothing.
//
// The field is named one of two ways: an attribute by its identifier, or a
// standard field - the date of a document, its reference, the owner of a
// catalog item - by its name, since a standard field has no identifier of its
// own. The configurations being moved link to a standard field 366, 22 and 31
// times: the date 221 times, the reference 161, the owner 27, the deletion
// mark, the activity and the description the rest.
type FieldPath struct {
	TablePart *uuid.UUID `yaml:"table_part,omitempty" json:"tablePart,omitempty"`
	Attribute uuid.UUID  `yaml:"attribute,omitempty" json:"attribute,omitempty"`
	Standard  string     `yaml:"standard,omitempty" json:"standard,omitempty"`
	// Unresolved is a path the prototype wrote that leads to no field of
	// this object, kept word for word.
	Unresolved string `yaml:"unresolved,omitempty" json:"unresolved,omitempty"`
}

// validateFieldPathShape checks that a path names its field one way, not two
// and not none.
func validateFieldPathShape(path string, source FieldPath) []string {
	var issues []string
	if source.Unresolved != "" {
		if source.TablePart != nil || !source.Attribute.IsZero() || source.Standard != "" {
			issues = append(issues, path+".source.unresolved is a path kept as written, and names no field beside it")
		}
		if !validUnresolvedPath(source.Unresolved) {
			issues = append(issues, path+".source.unresolved must be a path the prototype writes: segments of <n>:<uuid> or numbers, joined by /")
		}
		return issues
	}
	switch {
	case source.Standard != "" && !source.Attribute.IsZero():
		issues = append(issues, path+".source names both an attribute and a standard field")
	case source.Standard == "" && source.Attribute.IsZero():
		issues = append(issues, path+".source.attribute must be a non-zero UUID, or source.standard must name a standard field")
	}
	if source.TablePart != nil && source.TablePart.IsZero() {
		issues = append(issues, path+".source.table_part must be a non-zero UUID")
	}
	return issues
}

// ChoiceParameter fixes one parameter of the list a value is picked from - a
// filter by a field of that list, most often.
type ChoiceParameter struct {
	Name string `yaml:"name" json:"name"`
	// Values is what the parameter is set to. List tells one value from a list
	// of one: the platform filters by equality against a value and by
	// membership against a list, and the two are not the same filter.
	Values []Value `yaml:"values" json:"values"`
	List   bool    `yaml:"list,omitempty" json:"list,omitempty"`
}

// ChoiceParameterLink takes a choice parameter from another field of the same
// object instead of fixing it: the list of contracts narrows to the partner
// standing in the field above, and follows it when it changes.
type ChoiceParameterLink struct {
	Name   string      `yaml:"name" json:"name"`
	Source FieldPath   `yaml:"source" json:"source"`
	Change ValueChange `yaml:"change,omitempty" json:"change,omitempty"`
}

// TypeLink is the prototype's link by type: this field takes its type from the
// value of another field. Item is which element of that field's type
// description is taken, counted from zero, because the field it reads may be
// composite.
type TypeLink struct {
	Source FieldPath `yaml:"source" json:"source"`
	Item   int       `yaml:"item,omitempty" json:"item,omitempty"`
}

// ChoiceFormReference is the form opened to pick a value of this field. It is
// a form of the object being picked from, not of the object holding the field,
// so it names that object; named without one, it is a common form of the
// configuration.
//
// The object is one of the kinds that have forms of their own (formOwnerKind);
// the help puts no other bound on it. The configurations being moved fill it
// in on fields 98, 6 and 11 times, with forms of catalogs, documents, charts of
// accounts and an exchange plan.
//
// Nothing resolves the form itself yet: the place that would open it is the
// form engine, which is not built, and a form that is not there is carried
// unresolved rather than refused, as every reference by name is.
type ChoiceFormReference struct {
	Kind   Kind       `yaml:"kind,omitempty" json:"kind,omitempty"`
	Object *uuid.UUID `yaml:"object,omitempty" json:"object,omitempty"`
	Name   string     `yaml:"name" json:"name"`
}

// FieldPresentation is how the value of a field is shown and entered.
type FieldPresentation struct {
	// Format and EditFormat are the same picture in two roles - how the value
	// is shown, and how it is shown while it is being edited. Both are
	// localized: a format is read by a person, and the picture of a date is
	// not the same picture in two languages.
	Format     LocalizedText `yaml:"format,omitempty" json:"format,omitempty"`
	EditFormat LocalizedText `yaml:"edit_format,omitempty" json:"editFormat,omitempty"`
	// Mask is the shape the entered text must take.
	Mask string `yaml:"mask,omitempty" json:"mask,omitempty"`
	// ToolTip is the sentence shown beside the field.
	ToolTip LocalizedText `yaml:"tooltip,omitempty" json:"tooltip,omitempty"`
	// MultiLine, Password and ExtendedEdit are what the input itself is:
	// several lines instead of one, dots instead of what is typed, and a
	// field opened in a window of its own for a value too long for a line.
	MultiLine    bool `yaml:"multi_line,omitempty" json:"multiLine,omitempty"`
	Password     bool `yaml:"password,omitempty" json:"password,omitempty"`
	ExtendedEdit bool `yaml:"extended_edit,omitempty" json:"extendedEdit,omitempty"`
	// MarkNegatives shows a number below zero apart from the rest.
	MarkNegatives bool `yaml:"mark_negatives,omitempty" json:"markNegatives,omitempty"`
	// MinValue and MaxValue bound what may be entered, kept as the text they
	// were written with, on a field of any type. The designer offers them on a
	// number only (owner, 01.10.2026), yet the configurations being moved keep
	// them on booleans, strings, dates and references - left there when the
	// type was changed - and write them as a string or a decimal, with a point
	// or a comma: "0", "0,5". They bound a number only - see NumberBound.
	MinValue *string `yaml:"min_value,omitempty" json:"minValue,omitempty"`
	MaxValue *string `yaml:"max_value,omitempty" json:"maxValue,omitempty"`
}

// FieldChoice is how a value of a field is picked.
type FieldChoice struct {
	// QuickChoice offers the values in a drop-down list instead of opening a
	// form; CreateOnInput lets the user make a new one out of what they typed;
	// HistoryOnInput offers what this user picked before.
	QuickChoice    UsageMode `yaml:"quick_choice,omitempty" json:"quickChoice,omitempty"`
	CreateOnInput  UsageMode `yaml:"create_on_input,omitempty" json:"createOnInput,omitempty"`
	HistoryOnInput UsageMode `yaml:"history_on_input,omitempty" json:"historyOnInput,omitempty"`
	// FoldersAndItems is what a choice from a hierarchical object may land on.
	FoldersAndItems ChoiceTarget          `yaml:"folders_and_items,omitempty" json:"foldersAndItems,omitempty"`
	Form            *ChoiceFormReference  `yaml:"form,omitempty" json:"form,omitempty"`
	Parameters      []ChoiceParameter     `yaml:"parameters,omitempty" json:"parameters,omitempty"`
	ParameterLinks  []ChoiceParameterLink `yaml:"parameter_links,omitempty" json:"parameterLinks,omitempty"`
	LinkByType      *TypeLink             `yaml:"link_by_type,omitempty" json:"linkByType,omitempty"`
}

// validateFieldSettings checks the presentation and the choice of one field as
// far as the field alone allows. What the links point at is answered by the
// object, in validateFieldLinks.
func validateFieldSettings(prefix string, attribute Attribute, configuration project.Project) []string {
	var issues []string
	presentation, choice := attribute.Presentation, attribute.Choice
	for name, text := range map[string]LocalizedText{
		"presentation.format": presentation.Format, "presentation.edit_format": presentation.EditFormat,
		"presentation.tooltip": presentation.ToolTip,
	} {
		if len(text) > 0 {
			issues = append(issues, validateTitle(prefix+"."+name, text, configuration)...)
		}
	}
	// A bound is carried whatever it says and whatever the field holds: see
	// MinValue. Whether it bounds anything is NumberBound's to answer.
	for name, mode := range map[string]UsageMode{
		"choice.quick_choice": choice.QuickChoice, "choice.create_on_input": choice.CreateOnInput,
		"choice.history_on_input": choice.HistoryOnInput,
	} {
		if !validUsageMode(mode) {
			issues = append(issues, prefix+"."+name+" must be auto, use or dont-use")
		}
	}
	if !validChoiceTarget(choice.FoldersAndItems) {
		issues = append(issues, prefix+".choice.folders_and_items must be auto, items, folders or folders-and-items")
	}
	if form := choice.Form; form != nil {
		issues = append(issues, validateChoiceFormReference(prefix+".choice.form", *form)...)
	}
	issues = append(issues, validateChoiceParameters(prefix, choice)...)
	return issues
}

// validateValueSettings checks the settings of a value that is the whole object
// rather than a field of one - a constant. The paths it reports have no field
// prefix, because there is no field to name: the object is the value.
func validateValueSettings(field Attribute, configuration project.Project) []string {
	issues := validateFieldSettings("", field, configuration)
	for index, issue := range issues {
		issues[index] = strings.TrimPrefix(issue, ".")
	}
	return issues
}

// NumberBound is the number a bound sets, and false when it sets none. A bound
// bounds a field that holds one number and nothing else, and its text is read
// with a point or a comma as the decimal separator - the configurations being
// moved write both. On any other field, or with text that is not a number,
// the bound is carried and bounds nothing. Everything that checks an entered
// value against a bound reads it here, not from MinValue or MaxValue.
func NumberBound(bound *string, types []Type) (string, bool) {
	single, ok := SingleType(types)
	if bound == nil || !ok || single.Kind != NumberType {
		return "", false
	}
	text := strings.Replace(strings.TrimSpace(*bound), ",", ".", 1)
	if !decimalText.MatchString(text) {
		return "", false
	}
	return text, true
}

// decimalText is a number as a bound writes it once its comma is a point.
var decimalText = regexp.MustCompile(`^-?[0-9]+(\.[0-9]+)?$`)

// fieldHolds says whether a value is of a type the field may hold. A field
// whose description is open to the configuration - a set or a defined type -
// or carries no types at all is answered yes: what it holds is known only with
// the whole configuration in hand, or is the platform's.
func fieldHolds(value Value, types []Type) bool {
	if len(types) == 0 {
		return true
	}
	for _, item := range types {
		if IsTypeSet(item.Kind) || item.Kind == value.Kind {
			return true
		}
	}
	return false
}

// EffectiveFillingValue is the value a new object's field is filled with, and
// nil when it is filled with nothing. A filling value of a type the field
// cannot hold is carried - the configurations being moved keep an empty
// reference on a string field, left when the type was changed - and fills
// nothing. Everything that fills a field reads it here.
func EffectiveFillingValue(filling *Value, types []Type) *Value {
	if filling == nil || !fieldHolds(*filling, types) {
		return nil
	}
	copied := *filling
	return &copied
}

func validateChoiceFormReference(path string, form ChoiceFormReference) []string {
	var issues []string
	if !validIdentifier(form.Name) {
		issues = append(issues, path+".name must be a valid identifier")
	}
	switch {
	case form.Object == nil && form.Kind != "":
		issues = append(issues, path+".object must be set beside the kind")
	case form.Object != nil && form.Kind == "":
		issues = append(issues, path+".kind must be set beside the object")
	case form.Object != nil && form.Object.IsZero():
		issues = append(issues, path+".object must be a non-zero UUID")
	case form.Object != nil && !formOwnerKind(form.Kind):
		issues = append(issues, path+".kind must be a kind of object that has forms")
	}
	return issues
}

func validateChoiceParameters(prefix string, choice FieldChoice) []string {
	var issues []string
	// One parameter cannot be both fixed and taken from another field: the
	// two answers would be applied one after the other, and which of them
	// wins is not a thing a description should leave to be found out.
	names := map[string]bool{}
	for index, parameter := range choice.Parameters {
		path := fmt.Sprintf("%s.choice.parameters[%d]", prefix, index)
		issues = append(issues, validateChoiceParameterName(path, parameter.Name, names)...)
		if len(parameter.Values) == 0 {
			issues = append(issues, path+".values must contain at least one value")
		}
		for position, value := range parameter.Values {
			issues = append(issues, validateDesignTimeValue(fmt.Sprintf("%s.values[%d]", path, position), value)...)
		}
		if !parameter.List && len(parameter.Values) > 1 {
			issues = append(issues, path+" sets several values, so it must say it is a list")
		}
	}
	for index, link := range choice.ParameterLinks {
		path := fmt.Sprintf("%s.choice.parameter_links[%d]", prefix, index)
		issues = append(issues, validateChoiceParameterName(path, link.Name, names)...)
		issues = append(issues, validateFieldPathShape(path, link.Source)...)
		switch link.Change {
		case "", ValueChangeClear, ValueChangeDontChange:
		default:
			issues = append(issues, path+".change must be clear or dont-change")
		}
	}
	if link := choice.LinkByType; link != nil {
		path := prefix + ".choice.link_by_type"
		issues = append(issues, validateFieldPathShape(path, link.Source)...)
		if link.Item < 0 {
			issues = append(issues, path+".item must not be negative")
		}
	}
	return issues
}

func validateChoiceParameterName(path, name string, seen map[string]bool) []string {
	var issues []string
	switch {
	case name == "":
		issues = append(issues, path+".name must not be empty")
	case len(name) > maxChoiceParameterLen:
		issues = append(issues, fmt.Sprintf("%s.name must not be longer than %d characters", path, maxChoiceParameterLen))
	}
	folded := strings.ToLower(name)
	if seen[folded] {
		issues = append(issues, path+".name is already set or linked")
	}
	seen[folded] = true
	return issues
}

// fieldGroup is one named list of fields of an object: its attributes, or a
// register's dimensions or resources. The groups are passed together because a
// link may lead from any field of an object to any other, and the name of the
// group is what a message about a bad link has to say.
type fieldGroup struct {
	path   string
	fields []Attribute
}

// choiceHolder is anything that draws a link without being a field the link
// may point at: a standard field, whose identity is the platform's.
type choiceHolder struct {
	path   string
	choice FieldChoice
}

// validateFieldLinks resolves the links one object's fields draw between each
// other. A link to a field that is not there takes its parameter from nowhere:
// the list is never narrowed, and the user picks out of everything with no
// sign that anything was meant to narrow it.
//
// standard is the set of standard fields of the object the links stand in: a
// link to one of them is resolved by name. A row of a table part has its line
// number and nothing else.
// unresolvedSegment is one step of a path the prototype writes when the path
// leads out of the object: a field by its identifier with a number before
// it, or a bare number.
var unresolvedSegment = regexp.MustCompile(`^(-?[0-9]+|[0-9]+:[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})$`)

func validUnresolvedPath(path string) bool {
	for _, segment := range strings.Split(path, "/") {
		if !unresolvedSegment.MatchString(segment) {
			return false
		}
	}
	return true
}

func validateFieldLinks(standard []standardField, groups []fieldGroup, parts []TablePart, extra ...choiceHolder) []string {
	objectStandard, partStandard := standardNames(standard), standardNames(tablePartStandardFields)
	within := map[uuid.UUID]bool{}
	partFields := map[uuid.UUID]map[uuid.UUID]bool{}
	for _, group := range groups {
		for _, attribute := range group.fields {
			within[attribute.ID] = true
		}
	}
	for _, part := range parts {
		fields := make(map[uuid.UUID]bool, len(part.Attributes))
		for _, attribute := range part.Attributes {
			fields[attribute.ID] = true
		}
		partFields[part.ID] = fields
	}
	resolve := func(source FieldPath) bool {
		// A path kept as written is not resolved, and is not meant to be.
		if source.Unresolved != "" {
			return true
		}
		if source.TablePart == nil {
			if source.Standard != "" {
				_, ok := objectStandard[foldStandardName(source.Standard)]
				return ok
			}
			return within[source.Attribute]
		}
		fields, ok := partFields[*source.TablePart]
		if ok && source.Standard != "" {
			_, ok = partStandard[foldStandardName(source.Standard)]
			return ok
		}
		return ok && fields[source.Attribute]
	}
	var issues []string
	check := func(prefix string, field Attribute) {
		for index, link := range field.Choice.ParameterLinks {
			if !resolve(link.Source) {
				issues = append(issues, fmt.Sprintf("%s.choice.parameter_links[%d].source is not a field of this object", prefix, index))
			}
		}
		if link := field.Choice.LinkByType; link != nil && !resolve(link.Source) {
			issues = append(issues, prefix+".choice.link_by_type.source is not a field of this object")
		}
	}
	for _, group := range groups {
		for index, attribute := range group.fields {
			check(fmt.Sprintf("%s[%d]", group.path, index), attribute)
		}
	}
	for partIndex, part := range parts {
		for index, attribute := range part.Attributes {
			check(fmt.Sprintf("table_parts[%d].attributes[%d]", partIndex, index), attribute)
		}
	}
	for _, holder := range extra {
		check(holder.path, Attribute{Choice: holder.choice})
	}
	return issues
}

func cloneFieldSettings(attribute Attribute) Attribute {
	attribute.Presentation.Format = cloneTitle(attribute.Presentation.Format)
	attribute.Presentation.EditFormat = cloneTitle(attribute.Presentation.EditFormat)
	attribute.Presentation.ToolTip = cloneTitle(attribute.Presentation.ToolTip)
	attribute.Presentation.MinValue = cloneStringPointer(attribute.Presentation.MinValue)
	attribute.Presentation.MaxValue = cloneStringPointer(attribute.Presentation.MaxValue)
	if form := attribute.Choice.Form; form != nil {
		copied := *form
		if form.Object != nil {
			object := *form.Object
			copied.Object = &object
		}
		attribute.Choice.Form = &copied
	}
	attribute.Choice.Parameters = slices.Clone(attribute.Choice.Parameters)
	for index := range attribute.Choice.Parameters {
		attribute.Choice.Parameters[index].Values = slices.Clone(attribute.Choice.Parameters[index].Values)
	}
	attribute.Choice.ParameterLinks = slices.Clone(attribute.Choice.ParameterLinks)
	for index := range attribute.Choice.ParameterLinks {
		attribute.Choice.ParameterLinks[index].Source = cloneFieldPath(attribute.Choice.ParameterLinks[index].Source)
	}
	if link := attribute.Choice.LinkByType; link != nil {
		copied := *link
		copied.Source = cloneFieldPath(link.Source)
		attribute.Choice.LinkByType = &copied
	}
	return attribute
}

func cloneFieldPath(path FieldPath) FieldPath {
	if path.TablePart != nil {
		tablePart := *path.TablePart
		path.TablePart = &tablePart
	}
	return path
}

func cloneStringPointer(value *string) *string {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}

func cloneValuePointer(value *Value) *Value {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}

// formOwnerKind says whether objects of this kind have forms of their own -
// which is what a choice form of a field has to be one of, unless it is a
// common form.
func formOwnerKind(kind Kind) bool {
	switch kind {
	case CatalogKind, DocumentKind, EnumerationKind, DocumentJournalKind,
		ChartOfCharacteristicTypesKind, ChartOfAccountsKind, ChartOfCalculationTypesKind,
		ExchangePlanKind, BusinessProcessKind, TaskKind,
		InformationRegisterKind, AccumulationRegisterKind, AccountingRegisterKind, CalculationRegisterKind,
		ReportKind, DataProcessorKind, FilterCriterionKind, SettingsStorageKind,
		ExternalDataSourceTableKind, ExternalCubeKind, ExternalDimensionTableKind:
		return true
	}
	return false
}
