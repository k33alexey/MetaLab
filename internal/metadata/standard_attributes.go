package metadata

import (
	"fmt"
	"strings"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// Every kind of object has fields nobody declared: a reference, a code, a
// date, a recorder. The platform decides which ones a kind has, and the
// developer cannot add or remove one - but can override almost everything
// about each of them, from its synonym to the format its value is shown in.
//
// The composition below is read off the demonstration configuration, where
// every object of every kind writes its own list, and the list is the same for
// all objects of a kind. The properties are the syntax assistant's, which lists
// twenty-six for ОписаниеСтандартногоРеквизита.

// maxStandardAttributes bounds the descriptions one object may carry. The
// longest set the prototype gives anything is the accounting register's, and it
// grows with the number of ext dimensions.
const maxStandardAttributes = 128

// standardField is one field the platform gives, under both names it answers
// to. A description may name it either way; the Russian one is what it is
// stored and compared as.
type standardField struct {
	ru string
	en string
}

// The sets below are the prototype's, kind by kind. Where a set depends on the
// object rather than on its kind, the dependency is in standardAttributeFields
// below and not here.
var (
	referenceStandardFields = []standardField{
		{"Ссылка", "Ref"}, {"ПометкаУдаления", "DeletionMark"},
	}
	codedStandardFields = []standardField{
		{"Код", "Code"}, {"Наименование", "Description"},
		{"Предопределенный", "Predefined"}, {"ИмяПредопределенныхДанных", "PredefinedDataName"},
	}
	numberedStandardFields = []standardField{
		{"Номер", "Number"}, {"Дата", "Date"},
	}
	recordStandardFields = []standardField{
		{"Период", "Period"}, {"Регистратор", "Recorder"},
		{"НомерСтроки", "LineNumber"}, {"Активность", "Active"},
	}
)

func standardFieldsOf(groups ...[]standardField) []standardField {
	var result []standardField
	for _, group := range groups {
		result = append(result, group...)
	}
	return result
}

// standardFieldsOfKind is what the kind alone decides. Most kinds are
// decided by it entirely; the three registers whose set depends on a setting
// of the object have their own answers below, and they start from this one.
func standardFieldsOfKind(kind Kind) []standardField {
	switch kind {
	case CatalogKind:
		return standardFieldsOf(referenceStandardFields, codedStandardFields, []standardField{
			{"Родитель", "Parent"}, {"ЭтоГруппа", "IsFolder"}, {"Владелец", "Owner"},
		})
	case ChartOfCharacteristicTypesKind:
		return standardFieldsOf(referenceStandardFields, codedStandardFields, []standardField{
			{"Родитель", "Parent"}, {"ЭтоГруппа", "IsFolder"}, {"ТипЗначения", "ValueType"},
		})
	case ChartOfAccountsKind:
		return standardFieldsOf(referenceStandardFields, codedStandardFields, []standardField{
			{"Родитель", "Parent"}, {"Порядок", "Order"},
			{"Забалансовый", "OffBalance"}, {"Вид", "Type"},
		})
	case ChartOfCalculationTypesKind:
		return standardFieldsOf(referenceStandardFields, codedStandardFields, []standardField{
			{"ПериодДействияБазовый", "ActionPeriodIsBasic"},
		})
	case ExchangePlanKind:
		return standardFieldsOf(referenceStandardFields, []standardField{
			{"Код", "Code"}, {"Наименование", "Description"},
			{"ЭтотУзел", "ThisNode"}, {"НомерОтправленного", "SentNo"}, {"НомерПринятого", "ReceivedNo"},
		})
	case DocumentKind:
		return standardFieldsOf(referenceStandardFields, numberedStandardFields, []standardField{
			{"Проведен", "Posted"},
		})
	case BusinessProcessKind:
		return standardFieldsOf(referenceStandardFields, numberedStandardFields, []standardField{
			{"Стартован", "Started"}, {"Завершен", "Completed"}, {"ВедущаяЗадача", "HeadTask"},
		})
	case TaskKind:
		return standardFieldsOf(referenceStandardFields, numberedStandardFields, []standardField{
			{"Наименование", "Description"}, {"Выполнена", "Executed"},
			{"ТочкаМаршрута", "RoutePoint"}, {"БизнесПроцесс", "BusinessProcess"},
		})
	case DocumentJournalKind:
		return standardFieldsOf(referenceStandardFields, numberedStandardFields, []standardField{
			{"Тип", "Type"}, {"Проведен", "Posted"},
		})
	case EnumerationKind:
		return []standardField{{"Ссылка", "Ref"}, {"Порядок", "Order"}}
	case InformationRegisterKind:
		// All four whatever the write mode and the periodicity: the prototype
		// writes descriptions of the period and the recorder on independent
		// non-periodic registers too, and refusing them would lose a
		// description the configuration really carries.
		return recordStandardFields
	case AccumulationRegisterKind:
		return recordStandardFields
	case AccountingRegisterKind:
		return standardFieldsOf(recordStandardFields, []standardField{{"Счет", "Account"}})
	case CalculationRegisterKind:
		// A calculation is registered for a period, not made at an instant:
		// the register has no Период of its own, and says instead when the
		// calculation acts and which period it takes its base from.
		return standardFieldsOf(recordStandardFields[1:], []standardField{
			{"ВидРасчета", "CalculationType"}, {"ПериодДействия", "ActionPeriod"},
			{"ПериодДействияНачало", "BegOfActionPeriod"}, {"ПериодДействияКонец", "EndOfActionPeriod"},
			{"БазовыйПериодНачало", "BegOfBasePeriod"}, {"БазовыйПериодКонец", "EndOfBasePeriod"},
			{"ПериодРегистрации", "RegistrationPeriod"}, {"Сторно", "ReversingEntry"},
		})
	}
	return nil
}

// accumulationStandardFields adds what the kind of register decides. A register
// of balances tells an increase from a decrease; a register of turnovers has
// only the turnover, and nothing to tell apart.
func accumulationStandardFields(kind AccumulationRegisterKindValue) []standardField {
	fields := standardFieldsOfKind(AccumulationRegisterKind)
	if kind == AccumulationRegisterBalance {
		fields = append(fields, standardField{"ВидДвижения", "RecordType"})
	}
	return fields
}

// accountingStandardFields adds the entry's own: the kind of entry, where there
// are no two sides to tell it by, and a pair of fields per ext dimension the
// chart allows. The count comes from the chart of accounts, which lives in
// another file, so a decoder that has not seen it passes the platform's
// ceiling and load.go narrows it to the chart's own.
func accountingStandardFields(correspondence bool, extDimensions int) []standardField {
	fields := standardFieldsOfKind(AccountingRegisterKind)
	if !correspondence {
		fields = append(fields, standardField{"ВидДвижения", "RecordType"})
	}
	for position := 1; position <= extDimensions; position++ {
		fields = append(fields,
			standardField{fmt.Sprintf("Субконто%d", position), fmt.Sprintf("ExtDimension%d", position)},
			standardField{fmt.Sprintf("ВидСубконто%d", position), fmt.Sprintf("ExtDimensionType%d", position)},
		)
	}
	return fields
}

// standardAttributeFields is what one object of the loaded configuration is
// given, with every dependency on a setting of the object already resolved.
func (catalog *Catalog) standardAttributeFields(kind Kind, id uuid.UUID) []standardField {
	switch kind {
	case AccumulationRegisterKind:
		if index, ok := catalog.accumulationRegisterByID[id]; ok {
			return accumulationStandardFields(catalog.AccumulationRegisters[index].Kind)
		}
	case AccountingRegisterKind:
		index, ok := catalog.accountingRegisterByID[id]
		if !ok {
			break
		}
		register := catalog.AccountingRegisters[index]
		chart, ok := catalog.chartOfAccountsByID[register.ChartOfAccounts]
		if !ok {
			return accountingStandardFields(register.Correspondence, 0)
		}
		return accountingStandardFields(register.Correspondence, catalog.ChartsOfAccounts[chart].MaxExtDimensionCount)
	}
	return standardFieldsOfKind(kind)
}

// tablePartStandardFields is what a line of a table part is given. One field,
// and not the reference to the row that holds it: the prototype writes only
// the line number on all hundred and sixty table parts that describe any of
// this.
var tablePartStandardFields = []standardField{{"НомерСтроки", "LineNumber"}}

// foldStandardName is how a standard field is compared by name. Case does not
// matter, and neither does the letter ё: the prototype writes Проведен and
// Предопределенный with е, a developer types either, and the two spellings are
// one name.
func foldStandardName(name string) string {
	return strings.ReplaceAll(strings.ToLower(name), "ё", "е")
}

// standardNames is the folded names of one set of fields, under both languages,
// for a lookup by name.
func standardNames(fields []standardField) map[string]string {
	result := make(map[string]string, len(fields)*2)
	for _, field := range fields {
		result[foldStandardName(field.ru)] = field.ru
		result[foldStandardName(field.en)] = field.ru
	}
	return result
}

// reservedStandardName says the name is one the platform already gave this kind
// of object. It is the one answer to that question: a kind gaining a standard
// field gains the name for it here at the same time, so that an application
// field can never come to shadow one.
//
// The names a register may or may not have by a setting of the object - the kind
// of movement of an accumulation register, the kind of entry of an accounting
// one - are reserved for the kind whatever the setting says. A field that
// exists under one setting and not under another is still the platform's name.
func reservedStandardName(kind Kind, name string) bool {
	fields := standardFieldsOfKind(kind)
	switch kind {
	case AccumulationRegisterKind:
		fields = accumulationStandardFields(AccumulationRegisterBalance)
	case AccountingRegisterKind:
		// Without the ext dimensions: how many of those an entry has is the
		// chart's to say, and reserving eight names on a chart that allows
		// three would forbid a field the prototype allows.
		fields = accountingStandardFields(false, 0)
	}
	_, ok := standardNames(fields)[foldStandardName(name)]
	return ok
}

// tablePartStandardChoices are the links the standard fields of one object's
// own table parts draw, in the shape the link check reads.
func tablePartStandardChoices(parts []TablePart) []choiceHolder {
	var result []choiceHolder
	for index, part := range parts {
		result = append(result, standardAttributeChoices(fmt.Sprintf("table_parts[%d].standard_attributes", index), part.StandardAttributes)...)
	}
	return result
}

// standardTablePart is one table part the platform gives, under both names it
// answers to, together with the fields of its own line. Two kinds have any:
// the chart of accounts, whose account carries its kinds of analytics, and the
// chart of calculation types, whose type carries the three lists of competition.
type standardTablePart struct {
	ru     string
	en     string
	fields []standardField
}

var (
	extDimensionTypesPart = standardTablePart{"ВидыСубконто", "ExtDimensionTypes", []standardField{
		{"ТолькоОбороты", "TurnoversOnly"}, {"Предопределенный", "Predefined"},
		{"ВидСубконто", "ExtDimensionType"}, {"НомерСтроки", "LineNumber"},
	}}
	competitionPartFields = []standardField{
		{"Предопределенный", "Predefined"}, {"ВидРасчета", "CalculationType"},
		{"НомерСтроки", "LineNumber"},
	}
)

// standardTablePartsOfKind is what the kind gives. Unlike the standard fields
// of an object, this does not depend on a setting: the prototype writes all
// three lists of competition even on a chart where the setting that gives a
// list meaning is off, and the part exists whether the rule can apply or not.
func standardTablePartsOfKind(kind Kind) []standardTablePart {
	switch kind {
	case ChartOfAccountsKind:
		return []standardTablePart{extDimensionTypesPart}
	case ChartOfCalculationTypesKind:
		return []standardTablePart{
			{"ВедущиеВидыРасчета", "LeadingCalculationTypes", competitionPartFields},
			{"ВытесняющиеВидыРасчета", "DisplacingCalculationTypes", competitionPartFields},
			{"БазовыеВидыРасчета", "BaseCalculationTypes", competitionPartFields},
		}
	}
	return nil
}

// StandardTablePart is what a developer changed about one table part the
// platform gave. The help gives such a description five properties of its own
// and the descriptions of its line's fields; a standard part has no attributes
// to add, no usage and no line number length, because none of that is the
// developer's to set here.
type StandardTablePart struct {
	Name               string              `yaml:"name" json:"name"`
	Title              LocalizedText       `yaml:"title,omitempty" json:"title,omitempty"`
	Comment            string              `yaml:"comment,omitempty" json:"comment,omitempty"`
	ToolTip            LocalizedText       `yaml:"tooltip,omitempty" json:"tooltip,omitempty"`
	FillChecking       FillCheck           `yaml:"fill_checking,omitempty" json:"fillChecking,omitempty"`
	StandardAttributes []StandardAttribute `yaml:"standard_attributes,omitempty" json:"standardAttributes,omitempty"`
}

// validateStandardTableParts checks the descriptions of one object's standard
// table parts against the parts that kind of object actually has.
func validateStandardTableParts(path string, parts []StandardTablePart, known []standardTablePart, configuration project.Project) []string {
	if len(parts) > len(known) {
		return []string{fmt.Sprintf("%s must not contain more than %d items", path, len(known))}
	}
	byName := make(map[string]standardTablePart, len(known)*2)
	for _, part := range known {
		byName[foldStandardName(part.ru)] = part
		byName[foldStandardName(part.en)] = part
	}
	var issues []string
	seen := map[string]bool{}
	for index, part := range parts {
		prefix := fmt.Sprintf("%s[%d]", path, index)
		match, ok := byName[foldStandardName(part.Name)]
		if !ok {
			issues = append(issues, prefix+".name is not a standard table part this kind of object has")
			continue
		}
		if seen[match.ru] {
			issues = append(issues, prefix+".name describes "+match.ru+" a second time")
		}
		seen[match.ru] = true
		// Only what was written down is checked: a part carries a description
		// because one of its properties was set, and the rest stay unset.
		if len(part.Title) > 0 {
			issues = append(issues, validateTitle(prefix+".title", part.Title, configuration)...)
		}
		if len(part.ToolTip) > 0 {
			issues = append(issues, validateTitle(prefix+".tooltip", part.ToolTip, configuration)...)
		}
		if !validFillCheck(part.FillChecking) {
			issues = append(issues, prefix+".fill_checking must be dont-check or show-error")
		}
		issues = append(issues, validateStandardAttributes(prefix+".standard_attributes", part.StandardAttributes, match.fields, configuration)...)
	}
	return issues
}

func cloneStandardTableParts(parts []StandardTablePart) []StandardTablePart {
	if parts == nil {
		return nil
	}
	result := make([]StandardTablePart, len(parts))
	for index, part := range parts {
		part.Title = cloneTitle(part.Title)
		part.ToolTip = cloneTitle(part.ToolTip)
		part.StandardAttributes = cloneStandardAttributes(part.StandardAttributes)
		result[index] = part
	}
	return result
}

// standardTablePartChoices are the links the standard fields of one object's
// standard table parts draw, in the shape the link check reads.
func standardTablePartChoices(path string, parts []StandardTablePart) []choiceHolder {
	var result []choiceHolder
	for index, part := range parts {
		result = append(result, standardAttributeChoices(fmt.Sprintf("%s[%d].standard_attributes", path, index), part.StandardAttributes)...)
	}
	return result
}

// StandardAttributeNames are the fields one object is given, under the names a
// developer writes them by. It is the one place that knows the composition, so
// that a kind gaining a field gains it everywhere at once.
func (catalog *Catalog) StandardAttributeNames(kind Kind, id uuid.UUID) []string {
	fields := catalog.standardAttributeFields(kind, id)
	result := make([]string, len(fields))
	for index, field := range fields {
		result[index] = field.ru
	}
	return result
}

// TypeReduction is what happens to stored values when the type of a field
// narrows and a value no longer fits: the change is refused, the values are
// converted with whatever that costs, or the data is dropped. It is a setting
// about migration, and it is the only one of its kind on a field.
type TypeReduction string

const (
	TypeReductionDeny      TypeReduction = "deny"
	TypeReductionTransform TypeReduction = "transform-values"
	TypeReductionDelete    TypeReduction = "delete-data"
)

func validTypeReduction(mode TypeReduction) bool {
	switch mode {
	case "", TypeReductionDeny, TypeReductionTransform, TypeReductionDelete:
		return true
	default:
		return false
	}
}

// StandardAttribute is what a developer changed about one field the platform
// gave. Only the changes are written down: the field exists whether or not
// anything is said about it, and its type is the platform's to decide.
type StandardAttribute struct {
	Name    string        `yaml:"name" json:"name"`
	Title   LocalizedText `yaml:"title,omitempty" json:"title,omitempty"`
	Comment string        `yaml:"comment,omitempty" json:"comment,omitempty"`
	// The same settings an attribute has, minus the ones the platform keeps to
	// itself: a standard field is not indexed on demand, does not say whom it
	// belongs to, and its choice does not pick between folders and items.
	FillChecking   FillCheck         `yaml:"fill_checking,omitempty" json:"fillChecking,omitempty"`
	FullTextSearch UsageMode         `yaml:"full_text_search,omitempty" json:"fullTextSearch,omitempty"`
	DataHistory    UsageMode         `yaml:"data_history,omitempty" json:"dataHistory,omitempty"`
	TypeReduction  TypeReduction     `yaml:"type_reduction,omitempty" json:"typeReduction,omitempty"`
	Filling        FieldFilling      `yaml:"filling,omitempty" json:"filling,omitempty"`
	Presentation   FieldPresentation `yaml:"presentation,omitempty" json:"presentation,omitempty"`
	Choice         FieldChoice       `yaml:"choice,omitempty" json:"choice,omitempty"`
}

// asAttribute lends a standard attribute the shape the field checks expect. It
// carries no types on purpose: what a standard field may hold is the
// platform's answer, not a line of the description, so the checks that need
// types skip it.
func (attribute StandardAttribute) asAttribute() Attribute {
	return Attribute{
		Name: attribute.Name, Title: attribute.Title, Comment: attribute.Comment,
		FillChecking: attribute.FillChecking, FullTextSearch: attribute.FullTextSearch,
		DataHistory: attribute.DataHistory, Filling: attribute.Filling,
		Presentation: attribute.Presentation, Choice: attribute.Choice,
	}
}

// validateStandardAttributes checks the descriptions of one object's standard
// fields against the fields that object actually has.
func validateStandardAttributes(path string, attributes []StandardAttribute, fields []standardField, configuration project.Project) []string {
	if len(attributes) > maxStandardAttributes {
		return []string{fmt.Sprintf("%s must not contain more than %d items", path, maxStandardAttributes)}
	}
	known := standardNames(fields)
	var issues []string
	seen := map[string]bool{}
	for index, attribute := range attributes {
		prefix := fmt.Sprintf("%s[%d]", path, index)
		canonical, ok := known[foldStandardName(attribute.Name)]
		if !ok {
			issues = append(issues, prefix+".name is not a standard field this kind of object has")
			continue
		}
		if seen[canonical] {
			issues = append(issues, prefix+".name describes "+canonical+" a second time")
		}
		seen[canonical] = true
		if len(attribute.Title) > 0 {
			issues = append(issues, validateTitle(prefix+".title", attribute.Title, configuration)...)
		}
		if !validFillCheck(attribute.FillChecking) {
			issues = append(issues, prefix+".fill_checking must be dont-check or show-error")
		}
		for name, mode := range map[string]UsageMode{
			"full_text_search": attribute.FullTextSearch, "data_history": attribute.DataHistory,
		} {
			switch mode {
			case "", UsageUse, UsageDontUse:
			default:
				issues = append(issues, prefix+"."+name+" must be use or dont-use")
			}
		}
		if !validTypeReduction(attribute.TypeReduction) {
			issues = append(issues, prefix+".type_reduction must be deny, transform-values or delete-data")
		}
		// The prototype gives a standard field no choice between folders and
		// items: it is not among the twenty-six properties such a field has.
		if attribute.Choice.FoldersAndItems != "" {
			issues = append(issues, prefix+".choice.folders_and_items belongs to an attribute, not to a standard field")
		}
		issues = append(issues, validateFieldSettings(prefix, attribute.asAttribute(), configuration)...)
	}
	return issues
}

func cloneStandardAttributes(attributes []StandardAttribute) []StandardAttribute {
	if attributes == nil {
		return nil
	}
	result := make([]StandardAttribute, len(attributes))
	for index, attribute := range attributes {
		attribute.Title = cloneTitle(attribute.Title)
		lent := cloneFieldFilling(cloneFieldSettings(attribute.asAttribute()))
		attribute.Filling, attribute.Presentation, attribute.Choice = lent.Filling, lent.Presentation, lent.Choice
		result[index] = attribute
	}
	return result
}

// standardAttributeChoices are the links the standard fields of one object
// draw, in the shape the link check reads.
func standardAttributeChoices(path string, attributes []StandardAttribute) []choiceHolder {
	result := make([]choiceHolder, 0, len(attributes))
	for index, attribute := range attributes {
		result = append(result, choiceHolder{path: fmt.Sprintf("%s[%d]", path, index), choice: attribute.Choice})
	}
	return result
}
