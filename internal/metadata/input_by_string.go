package metadata

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
)

// Ввод по строке - finding the object by typing into the field instead of
// opening a list and picking from it. Four settings, and the syntax assistant
// gives all four to the same eight kinds that carry the "Поле ввода" tab of the
// editing window: a catalog, a document, a chart of characteristic types, a
// chart of accounts, a chart of calculation types, a business process, a task
// and an exchange plan.
//
// That is why they live on ObjectInput, the part every kind that can be
// referenced carries, and not on ObjectChoice, which only the five reference
// kinds have: the two settings already on ObjectInput have exactly the same
// eight carriers. With these four the tab is complete - quick choice, create on
// input and the history of choice were the last iteration's.
//
// A ninth kind has three of the four: the table of an external data source
// carries everything but the full-text search. That kind is outlined and its
// properties are deliberately not unfolded yet - see outlined_kinds.go - so
// nothing here is written for it.

// ObjectField names one field of an object, either one the platform gave it or
// one the developer declared.
//
// Exactly one of the two names is given, because the platform's own lists say of
// every field which of the two it is: the demonstration configuration writes
// Catalog.X.StandardAttribute.Description beside Catalog.X.Attribute.ИНН. The
// role could be guessed from the name - a declared attribute may not shadow a
// standard field - but guessing it would make the file say less than the export
// it was read from.
//
// The prototype has one type here too, СписокПолей, and uses it for every list
// of fields an object carries: the fields input by string searches and the
// fields the object may be locked by. What may be in each list differs, and
// that is checked per list; the shape does not.
type ObjectField struct {
	// Standard names a field the platform gave the object; Attribute names one
	// the developer declared.
	Standard  string `yaml:"standard,omitempty" json:"standard,omitempty"`
	Attribute string `yaml:"attribute,omitempty" json:"attribute,omitempty"`
}

// SearchStringMode is where in the value the typed text is looked for.
//
// Only two values, and the difference is not cosmetic: searching from the
// beginning can be answered by an index over the field, searching any part of it
// cannot. The prototype leaves the choice to the developer and so do we.
type SearchStringMode string

const (
	SearchFromBeginning SearchStringMode = "begin"
	SearchAnyPart       SearchStringMode = "any-part"
)

func validSearchStringMode(mode SearchStringMode) bool {
	switch mode {
	case "", SearchFromBeginning, SearchAnyPart:
		return true
	default:
		return false
	}
}

// FullTextSearchOnInput is whether the typed text goes to the full-text index
// instead of a comparison over the field.
//
// Two values and no "auto", although the neighbouring settings of input have
// three: the prototype's enumeration is Использовать and НеИспользовать, and a
// third value here would let a configuration name a mode the platform does not
// read.
type FullTextSearchOnInput string

const (
	FullTextOnInputUse     FullTextSearchOnInput = "use"
	FullTextOnInputDontUse FullTextSearchOnInput = "dont-use"
)

func validFullTextSearchOnInput(mode FullTextSearchOnInput) bool {
	switch mode {
	case "", FullTextOnInputUse, FullTextOnInputDontUse:
		return true
	default:
		return false
	}
}

// ChoiceDataGetMode is whether the user waits for the search or the search runs
// behind the typing.
//
// Beware the wording here, because it names the wrong thing at first sight. This
// is not a choice between reading the table and running a query: the prototype's
// enumeration is Непосредственно and Фоновый, and both mean the same search, one
// awaited and one not. Overriding what the search returns is a different
// mechanism altogether - the ОбработкаПолученияДанныхВыбора event of the manager
// module, see MODULE-EVENTS.md - and it is available under either mode.
type ChoiceDataGetMode string

const (
	ChoiceDataDirectly   ChoiceDataGetMode = "directly"
	ChoiceDataBackground ChoiceDataGetMode = "background"
)

func validChoiceDataGetMode(mode ChoiceDataGetMode) bool {
	switch mode {
	case "", ChoiceDataDirectly, ChoiceDataBackground:
		return true
	default:
		return false
	}
}

// searchableStandardFieldNames is which of the platform's own fields the typed
// text may be matched against.
//
// The help gives the answer per kind - Код and Наименование for the catalog and
// the three charts and the exchange plan, Номер for the document, "Наименование
// and Номер for business processes and tasks" - and that last pair is two kinds
// rolled into one sentence. A business process has no Наименование at all: its
// standard fields are Ссылка, ПометкаУдаления, Номер, Дата, Стартован, Завершен
// and ВедущаяЗадача, and the demonstration configuration searches it by Номер
// alone. So the answer is not a list per kind but these three names intersected
// with the fields the kind actually has, which reproduces the help everywhere
// the help is right and does not promise a field that is not there.
var searchableStandardFieldNames = []string{"Код", "Наименование", "Номер"}

func searchableStandardFields(kind Kind) map[string]string {
	var fields []standardField
	for _, field := range standardFieldsOfKind(kind) {
		if slices.Contains(searchableStandardFieldNames, field.ru) {
			fields = append(fields, field)
		}
	}
	return standardNames(fields)
}

// validateInputByString checks the list against the object that carries it,
// which is why it cannot live with the rest of validateObjectInput: a name is
// only answerable beside the kind that gave the standard fields and the
// attributes the developer declared.
//
// A field that is not there, or one the platform would never search, describes a
// search that silently never matches - the object is simply not found by what
// the user typed, and nobody is told why.
//
// switchedOff are the standard fields a length of 0 takes away - the code, the
// description, the number. The designer refuses to save a configuration whose
// input by string still names one of them, «Указано неверное поле для ввода по
// строке», checked by the owner on 01.10.2026; so does the load.
func validateInputByString(fields []ObjectField, kind Kind, attributes []Attribute, switchedOff ...string) []string {
	standard := searchableStandardFields(kind)
	declared := make(map[string]Attribute, len(attributes))
	for _, attribute := range attributes {
		declared[strings.ToLower(attribute.Name)] = attribute
	}
	var issues []string
	seen := map[string]bool{}
	for index, field := range fields {
		prefix := fmt.Sprintf("input_by_string[%d]", index)
		var key string
		switch {
		case field.Standard != "" && field.Attribute != "":
			issues = append(issues, prefix+" names both a standard field and an attribute")
			continue
		case field.Standard != "":
			canonical, ok := standard[foldStandardName(field.Standard)]
			if !ok {
				issues = append(issues, prefix+".standard is not a standard field this kind of object is searched by")
				continue
			}
			if slices.Contains(switchedOff, canonical) {
				issues = append(issues, prefix+".standard "+canonical+" is switched off by a length of 0, and a field that is not there cannot be searched by")
				continue
			}
			key = "standard:" + canonical
		case field.Attribute != "":
			attribute, ok := declared[strings.ToLower(field.Attribute)]
			if !ok {
				issues = append(issues, prefix+".attribute is not an attribute of this object")
				continue
			}
			issues = append(issues, validateInputByStringAttribute(prefix, attribute)...)
			key = "attribute:" + strings.ToLower(attribute.Name)
		default:
			issues = append(issues, prefix+" names neither a standard field nor an attribute")
			continue
		}
		if seen[key] {
			issues = append(issues, prefix+" is already among the searched fields")
		}
		seen[key] = true
	}
	return issues
}

// validateInputByStringAttribute is the help's rule about which of the
// developer's own attributes may be listed: one of type Число or Строка whose
// Индексировать asks for an index. The demonstration configuration lists exactly
// one such attribute - ИНН on two catalogs, a string of twelve with an index -
// and the rule is why: the search compares the typed text against the field on
// every keystroke, and a field with no index is compared against row by row.
//
// A composite type is refused for the same reason the platform refuses it: the
// text has to be compared as something, and a field that may hold a reference
// today and a date tomorrow has no such something.
//
// A defined type stands for its type here: erp searches the items of
// ОбъектыЭксплуатации by an inventory number whose type is a defined string
// of fifteen. What it stands for is known only beside the defined types, and
// validateInputByStringTypes checks it there.
func validateInputByStringAttribute(prefix string, attribute Attribute) []string {
	var issues []string
	if single, ok := SingleType(attribute.Types); !ok || single.Kind != DefinedType && !isOneStringOrNumber(attribute.Types) {
		issues = append(issues, prefix+".attribute must be of one type, string or number, to be searched by")
	}
	if !attribute.Indexing.indexes() {
		issues = append(issues, prefix+".attribute must be indexed to be searched by")
	}
	return issues
}

// cloneObjectInput copies the one part of the group that is not a value: the
// list of searched fields, whose order is the search order and therefore data.
func cloneObjectInput(input ObjectInput) ObjectInput {
	input.InputByString = slices.Clone(input.InputByString)
	return input
}

// validateInputByStringTypes resolves the searched attributes whose type is a
// defined type, and holds them to the rule validateInputByStringAttribute
// holds the others to. Every kind that is searched by string embeds its list
// and its attributes the same way, so the walk finds them by name and a kind
// added later is covered without being listed.
func (catalog *Catalog) validateInputByStringTypes() error {
	top := reflect.ValueOf(catalog).Elem()
	for index := range top.NumField() {
		list := top.Field(index)
		if list.Kind() != reflect.Slice || list.Type().Elem().Kind() != reflect.Struct {
			continue
		}
		for item := range list.Len() {
			object := list.Index(item)
			searchedField, attributesField := object.FieldByName("InputByString"), object.FieldByName("Attributes")
			if !searchedField.IsValid() || !attributesField.IsValid() {
				break
			}
			searched, _ := searchedField.Interface().([]ObjectField)
			attributes, _ := attributesField.Interface().([]Attribute)
			for _, field := range searched {
				if field.Attribute == "" {
					continue
				}
				for _, attribute := range attributes {
					if !strings.EqualFold(attribute.Name, field.Attribute) {
						continue
					}
					if single, ok := SingleType(attribute.Types); !ok || single.Kind != DefinedType {
						break
					}
					resolved, err := catalog.expandTypes(attribute.Types, nil)
					if err != nil {
						return fmt.Errorf("%s %s input by string attribute %s: %w", kebab(top.Type().Field(index).Name), nameOf(object), attribute.Name, err)
					}
					if !isOneStringOrNumber(resolved) {
						return fmt.Errorf("%s %s input by string attribute %s must be of one type, string or number, to be searched by, and its defined type stands for another",
							kebab(top.Type().Field(index).Name), nameOf(object), attribute.Name)
					}
				}
			}
		}
	}
	return nil
}
