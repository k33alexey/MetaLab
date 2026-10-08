package metadata

import (
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/k33alexey/MetaLab/internal/project"
)

// keepFormTexts gathers the texts of one form a rule on texts would note -
// a translation into a language the configuration does not declare, one
// with no language, a text of spaces - with the place each stands in. The
// forms are not part of the catalog, so the walk of texts does not see
// them; they are gathered when the forms are read.
//
// The whole form is walked, by reflection, so that a text added to the model
// of a form - of an element, a command, a chart, a setting - is noted the
// day it is added. Only the texts a rule would note are kept: erp writes
// hundreds of thousands of texts in its forms, nearly all of them sound.
func (catalog *Catalog) keepFormTexts(form string, value *ManagedForm) {
	catalog.formTitles = append(catalog.formTitles, formTextsToNote(form, value, catalog.Project)...)
}

// formTextsToNote lists the texts of one form a rule on texts would note. It
// reads nothing but the form and the languages of the configuration, so the
// forms are walked side by side as they are read.
func formTextsToNote(form string, value *ManagedForm, configuration project.Project) []formTitle {
	declared := map[string]bool{}
	for _, language := range configuration.Languages {
		declared[strings.ToLower(language.Code)] = true
	}
	var found []formTitle
	walk := formTextWalk{form: form, path: []string{form},
		keep: func(text LocalizedText) bool { return textToNote(text, declared) },
		visit: func(where string, text LocalizedText, chart bool) {
			found = append(found, formTitle{where: where, text: text, chart: chart})
		}}
	walk.value(reflect.ValueOf(value).Elem(), false)
	return found
}

// textToNote says a rule on texts may have something to say about the text:
// a language that is not declared - no language and "#" among them, which the
// rules tell apart - or a text of spaces.
func textToNote(text LocalizedText, declared map[string]bool) bool {
	for code, value := range text {
		if !declared[strings.ToLower(code)] {
			return true
		}
		if value != "" && strings.TrimSpace(value) == "" {
			return true
		}
	}
	return false
}

// formTextWalk walks the texts of one form and calls visit with every text
// that says something, and whether it stands in the content of a chart, a
// Gantt chart or a planner.
//
// A text is named as the other notes of a form name its place: an element by
// its name alone, whatever group it stands in ("element Поле"), an
// attribute, a column, a command and a parameter by a word and the name
// ("attribute Цены column Цена"), the columns added to a table by the table;
// below them by the keys of the fields, and an item of a list with no name by
// its place in the list.
type formTextWalk struct {
	form string
	// keep says whether a text is to be handed to visit; the place is named
	// only for those, and the walk names nothing for a sound text.
	keep  func(text LocalizedText) bool
	visit func(where string, text LocalizedText, chart bool)
	path  []string
}

// formListWords are the words the lists of a form are named by.
var formListWords = map[string]string{
	"attributes": "attribute", "columns": "column", "commands": "command", "parameters": "parameter",
}

var (
	chartContentType      = reflect.TypeFor[ChartContent]()
	ganttChartContentType = reflect.TypeFor[GanttChartContent]()
	plannerContentType    = reflect.TypeFor[PlannerContent]()
	formElementType       = reflect.TypeFor[ManagedFormElement]()
	addedColumnsType      = reflect.TypeFor[FormAdditionalColumns]()
	localizedTextType     = reflect.TypeFor[LocalizedText]()
)

func (walk *formTextWalk) value(value reflect.Value, chart bool) {
	if !holdsText(value.Type()) {
		return
	}
	// The path is a stack: every value sets it to its own place and adds to
	// it, and whoever walks a value next sets it again. An element starts a
	// path of its own, so the path it stood in is left as it was.
	switch value.Type() {
	case chartContentType, ganttChartContentType, plannerContentType:
		chart = true
	case formElementType:
		walk.path = append(make([]string, 0, 16), walk.form, "element", value.FieldByName("Name").String())
	case addedColumnsType:
		walk.path = append(walk.path, "table", value.FieldByName("Table").String())
	}
	base := walk.path
	switch value.Kind() {
	case reflect.Pointer:
		if !value.IsNil() {
			walk.value(value.Elem(), chart)
		}
	case reflect.Map:
		// The only map that holds a text is a text: holdsText says so of
		// every other.
		text := value.Interface().(LocalizedText)
		if len(text) != 0 && (walk.keep == nil || walk.keep(text)) {
			walk.visit(strings.Join(walk.path, " "), text, chart)
		}
	case reflect.Slice, reflect.Array:
		named := selfNamed(value.Type().Elem())
		for index := range value.Len() {
			element := value.Index(index)
			walk.path = base
			if !named {
				name := nameOf(element)
				if name == "" {
					name = strconv.Itoa(index)
				}
				walk.path = append(base, name)
			}
			walk.value(element, chart)
		}
	case reflect.Struct:
		for _, field := range textFieldsOf(value.Type()) {
			walk.path = base
			if field.segment != "" {
				walk.path = append(base, field.segment)
			}
			walk.value(value.Field(field.index), chart)
		}
	}
}

// textField is a field of a struct that may hold a text, with what it adds
// to the name of the place.
type textField struct {
	index   int
	segment string
}

// textFields remembers, by type, the fields of a struct that may hold a
// text: an element has some two hundred fields, a handful of them texts.
var textFields sync.Map

func textFieldsOf(of reflect.Type) []textField {
	if known, ok := textFields.Load(of); ok {
		return known.([]textField)
	}
	var fields []textField
	for index := range of.NumField() {
		field := of.Field(index)
		if !field.IsExported() || !holdsText(field.Type) {
			continue
		}
		segment := ""
		if !field.Anonymous && !selfNamed(field.Type) {
			key := yamlName(field)
			if word, ok := formListWords[key]; ok {
				key = word
			}
			segment = key
		}
		fields = append(fields, textField{index: index, segment: segment})
	}
	textFields.Store(of, fields)
	return fields
}

// textHolders remembers, by type, whether a value of the type may hold a
// text: the walk of a form passes over every field of some hundred thousand
// elements, and most fields - flags, names, colours - hold none.
var textHolders sync.Map

// holdsText says a value of the type may hold a text, at any depth.
func holdsText(of reflect.Type) bool {
	if known, ok := textHolders.Load(of); ok {
		return known.(bool)
	}
	return holdsTextBelow(of, map[reflect.Type]bool{})
}

func holdsTextBelow(of reflect.Type, visiting map[reflect.Type]bool) bool {
	if known, ok := textHolders.Load(of); ok {
		return known.(bool)
	}
	// A type met again below itself - an element inside an element - adds
	// nothing the outer one does not say.
	if visiting[of] {
		return false
	}
	visiting[of] = true
	defer delete(visiting, of)
	holds := false
	switch of.Kind() {
	case reflect.Map:
		// A map of anything but a text is not walked: the model of a form
		// has none that holds a text.
		holds = of == localizedTextType
	case reflect.Pointer, reflect.Slice, reflect.Array:
		holds = holdsTextBelow(of.Elem(), visiting)
	case reflect.Struct:
		for index := range of.NumField() {
			field := of.Field(index)
			if field.IsExported() && holdsTextBelow(field.Type, visiting) {
				holds = true
			}
		}
	}
	// What was decided while the type was being looked at from below itself
	// may be wrong; only the outermost answer is remembered.
	if len(visiting) == 1 || holds {
		textHolders.Store(of, holds)
	}
	return holds
}

// selfNamed says a value of the type, or the items of a list of them, name
// their own place: an element and the columns added to a table.
func selfNamed(of reflect.Type) bool {
	for of.Kind() == reflect.Pointer || of.Kind() == reflect.Slice {
		of = of.Elem()
	}
	return of == reflect.TypeFor[ManagedFormElement]() || of == reflect.TypeFor[FormAdditionalColumns]()
}
