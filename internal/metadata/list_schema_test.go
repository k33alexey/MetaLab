package metadata

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
	"go.yaml.in/yaml/v3"
)

// listSchemaWhole is a dynamic list with fields, a nested data set, a
// calculated field and parameters, written as demo-base writes them, with
// every property of the model set somewhere.
const listSchemaWhole = `attributes:
  - id: c0de0000-0000-4000-8000-000000990020
    name: Список
    types: [{kind: dynamic-list}]
    dynamic_list:
      fields:
        - {kind: field, data_path: Ссылка, field: Ссылка}
        - {kind: field, data_path: Наименование, field: Наименование, title: {ru: Название},
           value_type: [{kind: string, length: 0}],
           use_restriction: {field: true, condition: true}, attribute_use_restriction: {group: true, order: true}}
        - {kind: nested-data-set, data_path: КонтактнаяИнформация, field: КонтактнаяИнформация, title: {ru: Контакты}}
        - {kind: field, data_path: КонтактнаяИнформация.Вид, field: КонтактнаяИнформация.Вид}
      calculated_fields:
        - {data_path: СтандартнаяКартинка, expression: "4", title: {ru: Стандартная картинка}, value_type: [{kind: number, precision: 2}],
           use_restriction: {condition: true, group: true, order: true}}
      data_parameters:
        - {name: Доверитель, title: {ru: Доверитель}, value_type: [{kind: vanished-type, reference: c14a0491-75b5-4af9-b0d0-bb698a5467a4}],
           value: {kind: undefined}}
        - {name: ДоверительИНН, title: {ru: Доверитель ИНН}, value_type: [{kind: string, length: 0}], value: {kind: string}}
        - {name: ТекущаяДата, value_type: [{kind: date, date_parts: date-time}], value: {kind: date, data: "0001-01-01T00:00:00"}, use_restriction: true}
        - {name: Наборы, use_restriction: true, value_list_allowed: true}
`

// A dynamic list keeps its fields, calculated fields and data parameters
// whole through YAML and through the Studio, and the runtime copy of them
// shares nothing with the snapshot.
//
// Defect caught: a field, a nested data set, a calculated field or a
// parameter demo-base writes refused or dropped (the model carried none of
// the three before 2.271, and the list lost them on the way); a field of a
// nested set refused because its query field is written as a path; a
// parameter with no value taken for one whose value is Неопределено; a
// property of the model that nothing writes or reads; the runtime copy
// sharing the schema with the snapshot.
func TestADynamicListKeepsItsSchema(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	form, err := DecodeManagedForm("form.yaml", strings.NewReader(formAttrHead+listSchemaWhole), configuration)
	if err != nil {
		t.Fatal(err)
	}
	list := form.Attributes[0].DynamicList
	fields, calculated, parameters := list.Fields, list.CalculatedFields, list.DataParameters
	switch {
	case len(fields) != 4 || fields[1].Title["ru"] != "Название" || fields[1].ValueType[0].Kind != StringType ||
		*fields[1].UseRestriction != (CompositionUseRestriction{Field: true, Condition: true}) ||
		*fields[1].AttributeUseRestriction != (CompositionUseRestriction{Group: true, Order: true}) ||
		fields[2].Kind != CompositionNestedDataSet || fields[3].Field != "КонтактнаяИнформация.Вид" || fields[0].UseRestriction != nil:
		t.Fatalf("fields: %+v", fields)
	case len(calculated) != 1 || calculated[0].Expression != "4" || calculated[0].DataPath != "СтандартнаяКартинка" ||
		*calculated[0].UseRestriction != (CompositionUseRestriction{Condition: true, Group: true, Order: true}):
		t.Fatalf("calculated fields: %+v", calculated)
	case len(parameters) != 4 || parameters[0].Value.Kind != CompositionUndefined || parameters[0].ValueType[0].Kind != VanishedType ||
		parameters[1].Value.Kind != CompositionString || parameters[1].Value.Data != "" ||
		parameters[2].Value.Data != "0001-01-01T00:00:00" || !parameters[2].UseRestriction || parameters[0].UseRestriction ||
		parameters[3].Value != nil || !parameters[3].ValueListAllowed || len(parameters[3].ValueType) != 0:
		t.Fatalf("parameters: %+v", parameters)
	}
	if unset := fieldsNeverSet(reflect.ValueOf([]any{fields, calculated, parameters})); len(unset) != 0 {
		t.Fatalf("set by no case: %v", unset)
	}
	written, err := yaml.Marshal(form)
	if err != nil {
		t.Fatal(err)
	}
	again, err := DecodeManagedForm("form.yaml", strings.NewReader(string(written)), configuration)
	if err != nil || !reflect.DeepEqual(again.Attributes[0].DynamicList, list) {
		t.Fatalf("written back: %v", err)
	}
	carried, err := json.Marshal(form)
	if err != nil {
		t.Fatal(err)
	}
	var received ManagedForm
	if err := json.Unmarshal(carried, &received); err != nil {
		t.Fatal(err)
	}
	if err := ValidateManagedForm("studio", received, configuration); err != nil || !reflect.DeepEqual(received.Attributes[0].DynamicList, list) {
		t.Fatalf("carried through the Studio: %v", err)
	}
	copied := cloneRuntimeForm(form).Attributes[0].DynamicList
	copied.Fields[1].Title["ru"] = "changed"
	copied.Fields[1].UseRestriction.Order = true
	copied.Fields[1].ValueType[0].Kind = NumberType
	copied.CalculatedFields[0].UseRestriction.Field = true
	copied.DataParameters[0].Value.Kind = CompositionNull
	copied.DataParameters[1].Title["ru"] = "changed"
	if fields[1].Title["ru"] != "Название" || fields[1].UseRestriction.Order || fields[1].ValueType[0].Kind != StringType ||
		calculated[0].UseRestriction.Field || parameters[0].Value.Kind != CompositionUndefined || parameters[1].Title["ru"] != "Доверитель ИНН" {
		t.Fatal("the runtime copy shares the schema of the list with the snapshot")
	}
}

// Fields, calculated fields and parameters of a dynamic list that are wrong
// are refused, naming the place.
//
// Defect caught: two fields - calculated or not, in any case of letters -
// under one path, or two parameters under one name, so that the settings
// naming the path cannot say which one they mean; a path or a query field
// that is no path; a kind of field not in the model - a folder, which the
// help names and the model does not carry yet - loading as if it were
// known; a nested data set with a type or a restriction of its own; a
// restriction that restricts nothing, a second writing of no restriction;
// a field taken for the value of a parameter; the type of a parameter, a
// field or a calculated field never checked.
func TestTheSchemaOfADynamicListRefusesWhatIsWrong(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	list := func(settings string) string {
		return formAttrHead + "attributes:\n  - {id: c0de0000-0000-4000-8000-000000990020, name: Список, types: [{kind: dynamic-list}], dynamic_list: {" + settings + "}}\n"
	}
	const at = "attributes[0].dynamic_list."
	field := func(rest string) string { return "{kind: field, data_path: Сумма, field: Сумма" + rest + "}" }
	for name, test := range map[string]struct{ body, want string }{
		"путь дважды": {list("fields: [" + field("") + ", {kind: field, data_path: СУММА, field: Итог}]"),
			at + "fields[1].data_path is the path of " + at + "fields[0] already"},
		"путь поля и вычисляемого": {list("fields: [" + field("") + "], calculated_fields: [{data_path: сумма, expression: \"1\"}]"),
			at + "calculated_fields[0].data_path is the path of " + at + "fields[0] already"},
		"путь не путь":          {list("fields: [{kind: field, data_path: \"Сумма..Итог\", field: Сумма}]"), at + "fields[0].data_path must contain only valid identifier segments"},
		"путь пустой":           {list("calculated_fields: [{data_path: \"\", expression: \"1\"}]"), at + "calculated_fields[0].data_path must contain only valid identifier segments"},
		"поле запроса не путь":  {list("fields: [{kind: field, data_path: Сумма, field: \"1Сумма\"}]"), at + "fields[0].field must contain only valid identifier segments"},
		"папка":                 {list("fields: [{kind: folder, data_path: Сумма, field: Сумма}]"), at + "fields[0].kind must be field or nested-data-set"},
		"без вида":              {list("fields: [{data_path: Сумма, field: Сумма}]"), at + "fields[0].kind must be field or nested-data-set"},
		"набор с типом":         {list("fields: [{kind: nested-data-set, data_path: Товары, field: Товары, value_type: [{kind: boolean}]}]"), at + "fields[0] is a nested data set and has a path, a field and a title only"},
		"набор с ограничением":  {list("fields: [{kind: nested-data-set, data_path: Товары, field: Товары, use_restriction: {field: true}}]"), at + "fields[0] is a nested data set and has a path, a field and a title only"},
		"пустое ограничение":    {list("fields: [" + field(", use_restriction: {}") + "]"), at + "fields[0].use_restriction restricts nothing"},
		"пустое реквизитов":     {list("fields: [" + field(", attribute_use_restriction: {}") + "]"), at + "fields[0].attribute_use_restriction restricts nothing"},
		"пустое вычисляемого":   {list("calculated_fields: [{data_path: Итог, expression: \"1\", use_restriction: {}}]"), at + "calculated_fields[0].use_restriction restricts nothing"},
		"тип поля":              {list("fields: [" + field(", value_type: [{kind: string, length: -1}]") + "]"), at + "fields[0].value_type[0]"},
		"тип вычисляемого":      {list("calculated_fields: [{data_path: Итог, expression: \"1\", value_type: [{kind: boolean}, {kind: boolean}]}]"), at + "calculated_fields[0].value_type[1] duplicates an allowed type"},
		"имя дважды":            {list("data_parameters: [{name: Дата}, {name: ДАТА}]"), at + "data_parameters[1].name is the name of another parameter already"},
		"имя не имя":            {list("data_parameters: [{name: \"Дата.Начало\"}]"), at + "data_parameters[0].name must be the name of a parameter"},
		"имя пустое":            {list("data_parameters: [{name: \"\"}]"), at + "data_parameters[0].name must be the name of a parameter"},
		"значение поле":         {list("data_parameters: [{name: Дата, value: {kind: field, data: Дата}}]"), at + "data_parameters[0].value.kind must be one of"},
		"значение проверяется":  {list("data_parameters: [{name: Дата, value: {kind: date, data: вчера}}]"), at + "data_parameters[0].value.data must be a date"},
		"тип параметра":         {list("data_parameters: [{name: Дата, value_type: [{kind: date, date_parts: hours}]}]"), at + "data_parameters[0].value_type[0]"},
		"заголовок проверяется": {list("data_parameters: [{name: Дата, title: {ru: \"\\u0001\"}}]"), at + "data_parameters[0].title"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeManagedForm("form.yaml", strings.NewReader(test.body), configuration)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err = %v, want %q", err, test.want)
			}
		})
	}
}

// What the fields and parameters of a dynamic list name in the project is
// resolved at load: an object in the type of a field, a calculated field or
// a parameter is refused when it is gone, as in the type of an attribute;
// a value by name the project does not have is a remnant; a value of kind
// type is noted.
//
// Defect caught: the schema of a list never resolved, so that a type
// pointing at a deleted object, or a value of an enumeration that is gone,
// loads clean and silent.
func TestWhatTheSchemaOfADynamicListNamesIsResolved(t *testing.T) {
	t.Parallel()
	schema := "fields: [{kind: field, data_path: Склад, field: Склад, value_type: [{kind: catalog, reference: " + cmpWarehouses + "}]}], " +
		"calculated_fields: [{data_path: Товар, expression: Склад, value_type: [{kind: catalog, reference: " + cmpGoods + "}]}], " +
		"data_parameters: [{name: Пользователь, value_type: [{kind: catalog, reference: " + cmpUsers + "}]}, " +
		"{name: Ставка, value: {kind: predefined, data: Перечисление.СтавкиНДСИсчезли.НДС0}}, {name: Тип, value: {kind: type, data: Undefined}}]"
	const where = "catalog Номенклатура form ФормаЭлемента attribute Документы dynamic_list "
	write := func(t *testing.T, target string) string {
		root := formReferencesProject(t)
		path := filepath.Join(root, "metadata", string(CatalogKind), "Номенклатура", "forms", "ФормаЭлемента", project.FormMetadataFile)
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		attribute := "  - {id: c0de0000-0000-4000-8000-000000990020, name: Документы, types: [{kind: dynamic-list}], dynamic_list: {" + schema + "}}\n"
		if strings.HasPrefix(target, "{") {
			attribute = strings.Replace(attribute, target, "", 1)
		} else if target != "" {
			attribute = strings.ReplaceAll(attribute, target, refGone)
		}
		writeFile(t, path, string(content)+attribute)
		return root
	}
	for name, test := range map[string]struct{ target, place string }{
		"тип поля":         {cmpWarehouses, "field Склад value type"},
		"тип вычисляемого": {cmpGoods, "calculated field Товар value type"},
		"тип параметра":    {cmpUsers, "parameter Пользователь value type"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := Load(write(t, test.target))
			if err == nil || !strings.HasPrefix(err.Error(), where+test.place+" ") || !strings.Contains(err.Error(), refGone) {
				t.Fatalf("err = %v", err)
			}
		})
	}
	t.Run("значение по имени", func(t *testing.T) {
		t.Parallel()
		found := unresolvedOf(t, write(t, ""))
		if len(found) != 1 || found[0].Where != where+"data_parameters [1].value" || found[0].Written != "Перечисление.СтавкиНДСИсчезли.НДС0" {
			t.Fatalf("unresolved = %+v", found)
		}
	})
	t.Run("тип", func(t *testing.T) {
		t.Parallel()
		catalog, err := Load(write(t, "{name: Ставка, value: {kind: predefined, data: Перечисление.СтавкиНДСИсчезли.НДС0}}, "))
		if err != nil {
			t.Fatal(err)
		}
		var notes []string
		for _, note := range catalog.Notes() {
			if note.Kind == NoteCompositionTypeUnexplained {
				notes = append(notes, note.Where+" = "+note.Written)
			}
		}
		if !reflect.DeepEqual(notes, []string{where + "data_parameters [1].value = Undefined"}) {
			t.Fatalf("notes = %q", notes)
		}
	})
}
