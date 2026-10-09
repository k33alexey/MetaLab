package metadata

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

const (
	formAttrForm      = "f0a00000-0000-4000-8000-000000000001"
	formAttrObject    = "f0a00000-0000-4000-8000-000000000002"
	formAttrList      = "f0a00000-0000-4000-8000-000000000003"
	formAttrColumn    = "f0a00000-0000-4000-8000-000000000004"
	formAttrExtra     = "f0a00000-0000-4000-8000-000000000005"
	formAttrOption    = "f0a00000-0000-4000-8000-000000000006"
	formAttrRole      = "f0a00000-0000-4000-8000-000000000007"
	formAttrCatalog   = "f0a00000-0000-4000-8000-000000000008"
	formAttrField     = "f0a00000-0000-4000-8000-000000000009"
	formAttrArbitrary = "f0a00000-0000-4000-8000-000000000010"
)

const formAttrHead = "format: 1\nid: " + formAttrForm + "\nname: ФормаЭлемента\ntitle: {ru: Форма элемента}\nkind: object\n"

// formAttrWhole is a form whose attributes carry every property an attribute
// and a column have.
const formAttrWhole = formAttrHead + `items:
  - {id: ` + formAttrField + `, name: Наименование, kind: input-field, data_path: Объект.Наименование}
attributes:
  - id: ` + formAttrObject + `
    name: Объект
    title: {ru: Объект}
    types: [{kind: catalog-object, reference: ` + formAttrCatalog + `}]
    main: true
    saved_data: true
    fill_checking: show-error
    functional_options: [` + formAttrOption + `]
    use_always: [Объект.Ref, ~Объект.RegisterRecords]
    save_in_settings: [Объект, "1/0:` + formAttrRole + `", "3/2"]
    view: {common: false, roles: [{role: ` + formAttrRole + `, value: true}]}
    edit: {common: false, roles: [{role: ` + formAttrRole + `, value: true}]}
    additional_columns:
      - table: Объект.Товары
        columns:
          - {id: ` + formAttrExtra + `, name: Остаток, title: {ru: Остаток}, types: [{kind: number, precision: 15, scale: 3}], functional_options: [` + formAttrOption + `]}
  - id: ` + formAttrList + `
    name: Варианты
    types: [{kind: value-table}]
    columns:
      - id: ` + formAttrColumn + `
        name: Вариант
        title: {ru: Вариант}
        types: [{kind: string, length: 0}]
        functional_options: [` + formAttrOption + `]
        view: {common: true}
        edit: {common: false, roles: [{role: ` + formAttrRole + `, value: true}]}
  - {id: ` + formAttrArbitrary + `, name: Любое}
`

// An attribute and a column keep every property they were given, and come
// back the same after being written out and read again.
//
// Defect caught: a property of a form attribute read into the wrong field or
// not at all - the description refuses unknown properties, so a property left
// out of the model refuses the form, and one mapped wrong loses its value
// without a word; and the main attribute, the use on the client or the
// rights lost on the way through the Studio, which writes the form back.
func TestFormAttributeKeepsEveryPropertyItWasGiven(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	form, err := DecodeManagedForm("form.yaml", strings.NewReader(formAttrWhole), configuration)
	if err != nil {
		t.Fatal(err)
	}
	if len(form.Attributes) != 3 {
		t.Fatalf("attributes = %d", len(form.Attributes))
	}
	object, list, arbitrary := form.Attributes[0], form.Attributes[1], form.Attributes[2]
	switch {
	case object.Name != "Объект" || object.Title["ru"] != "Объект" || len(object.Types) != 1 || object.Types[0].Kind != CatalogObjectType:
		t.Fatalf("name, title or type lost: %+v", object)
	case !object.Main || !object.SavedData || object.FillChecking != ShowFillingError:
		t.Fatalf("main, saved data or the check of filling lost: %+v", object)
	case len(object.FunctionalOptions) != 1 || object.FunctionalOptions[0].String() != formAttrOption:
		t.Fatalf("functional options lost: %+v", object.FunctionalOptions)
	case !reflect.DeepEqual(object.UseAlways, []string{"Объект.Ref", "~Объект.RegisterRecords"}):
		t.Fatalf("use on the client lost: %+v", object.UseAlways)
	case !reflect.DeepEqual(object.SaveInSettings, []string{"Объект", "1/0:" + formAttrRole, "3/2"}):
		t.Fatalf("what is kept in the settings lost: %+v", object.SaveInSettings)
	case object.View == nil || object.View.Common || len(object.View.Roles) != 1 || object.View.Roles[0].Role.String() != formAttrRole || !object.View.Roles[0].Value:
		t.Fatalf("the right to view lost: %+v", object.View)
	case object.Edit == nil || object.Edit.Common || len(object.Edit.Roles) != 1:
		t.Fatalf("the right to edit lost: %+v", object.Edit)
	case len(object.AdditionalColumns) != 1 || object.AdditionalColumns[0].Table != "Объект.Товары" ||
		len(object.AdditionalColumns[0].Columns) != 1 || object.AdditionalColumns[0].Columns[0].Name != "Остаток" ||
		len(object.AdditionalColumns[0].Columns[0].FunctionalOptions) != 1:
		t.Fatalf("additional columns lost: %+v", object.AdditionalColumns)
	}
	column := list.Columns[0]
	switch {
	case len(list.Columns) != 1 || column.ID.String() != formAttrColumn || column.Name != "Вариант" || column.Title["ru"] != "Вариант":
		t.Fatalf("the column lost its identity: %+v", list.Columns)
	case len(column.Types) != 1 || column.Types[0].Kind != StringType || len(column.FunctionalOptions) != 1:
		t.Fatalf("the column lost its type or options: %+v", column)
	case column.View == nil || !column.View.Common || column.Edit == nil || column.Edit.Common || len(column.Edit.Roles) != 1:
		t.Fatalf("the column lost its rights: %+v %+v", column.View, column.Edit)
	}
	if len(arbitrary.Types) != 0 || arbitrary.Main {
		t.Fatalf("an attribute of arbitrary type: %+v", arbitrary)
	}
	written, err := yaml.Marshal(form)
	if err != nil {
		t.Fatal(err)
	}
	again, err := DecodeManagedForm("form.yaml", strings.NewReader(string(written)), configuration)
	if err != nil {
		t.Fatalf("a form written back is refused: %v\n%s", err, written)
	}
	if !reflect.DeepEqual(again.Attributes, form.Attributes) {
		t.Fatalf("written back:\n%+v\nread:\n%+v", again.Attributes, form.Attributes)
	}
	// The Studio hands the form over as JSON and saves what comes back.
	carried, err := json.Marshal(form)
	if err != nil {
		t.Fatal(err)
	}
	var received ManagedForm
	if err := json.Unmarshal(carried, &received); err != nil {
		t.Fatal(err)
	}
	if err := ValidateManagedForm("studio", received, configuration); err != nil {
		t.Fatalf("a form carried through the Studio is refused: %v", err)
	}
	if !reflect.DeepEqual(received.Attributes, form.Attributes) {
		t.Fatalf("carried through the Studio:\n%+v\nread:\n%+v", received.Attributes, form.Attributes)
	}
}

// Each rule an attribute is held to refuses the form, and says where.
//
// Defect caught: two main attributes, of which the form can follow one; a
// path used on the client that belongs to another attribute; additional
// columns of a table of another attribute, or twice of one; names and
// identifiers that collide, so that a field bound by name or a role granted
// by identifier finds two; and a check of filling or a reference left empty.
func TestFormAttributeRefusesWhatTheFormCannotHold(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	attribute := func(id, name, rest string) string {
		return "  - {id: " + id + ", name: " + name + rest + "}\n"
	}
	for name, test := range map[string]struct{ attributes, want string }{
		"два основных": {attribute(formAttrObject, "Объект", ", main: true") + attribute(formAttrList, "Список", ", main: true"),
			"attributes[1].main: attributes[0] is the main attribute already"},
		"имя занято": {attribute(formAttrObject, "Объект", "") + attribute(formAttrList, "объект", ""),
			"attributes[1].name must be unique within the attributes"},
		"идентификатор элемента": {attribute(formAttrField, "Объект", ""), "attributes[0].id must be unique within the form"},
		"нулевой идентификатор":  {attribute("00000000-0000-0000-0000-000000000000", "Объект", ""), "attributes[0].id must be a non-zero UUID"},
		"чужой путь на клиенте": {attribute(formAttrObject, "Объект", ", use_always: [Список.Ref]"),
			"attributes[0].use_always[0] must start with the attribute's own name"},
		"путь на клиенте не путь": {attribute(formAttrObject, "Объект", ", use_always: [\"Объект..Ref\"]"),
			"attributes[0].use_always[0] must contain only valid identifier segments"},
		"пустое в настройках": {attribute(formAttrObject, "Объект", ", save_in_settings: [\"\"]"),
			"attributes[0].save_in_settings[0] must be a non-empty path"},
		"проверка заполнения": {attribute(formAttrObject, "Объект", ", fill_checking: always"),
			"attributes[0].fill_checking must be dont-check or show-error"},
		"опция дважды": {attribute(formAttrObject, "Объект", ", functional_options: ["+formAttrOption+", "+formAttrOption+"]"),
			"attributes[0].functional_options[1] is already among the options"},
		"роль дважды": {attribute(formAttrObject, "Объект", ", edit: {common: false, roles: [{role: "+formAttrRole+", value: true}, {role: "+formAttrRole+", value: false}]}"),
			"attributes[0].edit.roles[1].role already has its answer"},
		"колонки чужой таблицы": {attribute(formAttrObject, "Объект", ", additional_columns: [{table: Список.Товары}]"),
			"attributes[0].additional_columns[0].table must start with the attribute's own name"},
		"колонки таблицы дважды": {attribute(formAttrObject, "Объект", ", additional_columns: [{table: Объект.Товары}, {table: объект.товары}]"),
			"attributes[0].additional_columns[1].table already has its additional columns"},
		"колонка дважды": {attribute(formAttrList, "Список", ", columns: [{id: "+formAttrColumn+", name: А}, {id: "+formAttrExtra+", name: а}]"),
			"attributes[0].columns[1].name must be unique within the columns"},
		"колонка с идентификатором реквизита": {attribute(formAttrList, "Список", ", columns: [{id: "+formAttrList+", name: А}]"),
			"attributes[0].columns[0].id must be unique within the form"},
		"тип без ссылки": {attribute(formAttrObject, "Объект", ", types: [{kind: catalog}]"),
			"attributes[0].types[0].reference is required"},
		"объект справочника без ссылки": {attribute(formAttrObject, "Объект", ", types: [{kind: catalog-object}]"),
			"attributes[0].types[0].reference is required"},
		"объект обработки без ссылки": {attribute(formAttrObject, "Объект", ", types: [{kind: data-processor-object}]"),
			"attributes[0].types[0].reference is required"},
		"отчёт с нулевой ссылкой": {attribute(formAttrObject, "Объект", ", types: [{kind: report-object, reference: 00000000-0000-0000-0000-000000000000}]"),
			"attributes[0].types[0].reference is required"},
		"заголовок колонки не на языке": {attribute(formAttrList, "Список", ", columns: [{id: "+formAttrColumn+", name: А, title: {\"d=e\": A}}]"),
			"attributes[0].columns[0].title"},
		"имя не идентификатор": {attribute(formAttrObject, "1Объект", ""),
			"attributes[0].name must be a valid identifier of at most 255 characters"},
		"имя длиннее 255": {attribute(formAttrObject, nameAt(maxNameLength+1), ""),
			"attributes[0].name must be a valid identifier of at most 255 characters"},
		"имя колонки длиннее 255": {attribute(formAttrList, "Список", ", columns: [{id: "+formAttrColumn+", name: "+nameAt(maxNameLength+1)+"}]"),
			"attributes[0].columns[0].name must be a valid identifier of at most 255 characters"},
		"пробелы в настройках": {attribute(formAttrObject, "Объект", ", save_in_settings: [\" Объект\"]"),
			"attributes[0].save_in_settings[0] must be a non-empty path without surrounding spaces"},
		"нулевая опция": {attribute(formAttrObject, "Объект", ", functional_options: [00000000-0000-0000-0000-000000000000]"),
			"attributes[0].functional_options[0] must be a non-zero UUID"},
		"роль просмотра дважды": {attribute(formAttrObject, "Объект", ", view: {common: false, roles: [{role: "+formAttrRole+", value: true}, {role: "+formAttrRole+", value: false}]}"),
			"attributes[0].view.roles[1].role already has its answer"},
		"роль без идентификатора": {attribute(formAttrObject, "Объект", ", edit: {common: true, roles: [{role: 00000000-0000-0000-0000-000000000000, value: false}]}"),
			"attributes[0].edit.roles[0].role must be a non-zero UUID"},
		"тип колонки без ссылки": {attribute(formAttrList, "Список", ", columns: [{id: "+formAttrColumn+", name: А, types: [{kind: catalog}]}]"),
			"attributes[0].columns[0].types[0].reference is required"},
		"добавленная колонка дважды": {attribute(formAttrObject, "Объект", ", additional_columns: [{table: Объект.Товары, columns: [{id: "+formAttrColumn+", name: А}, {id: "+formAttrExtra+", name: а}]}]"),
			"attributes[0].additional_columns[0].columns[1].name must be unique within the columns"},
		"тип добавленной колонки без ссылки": {attribute(formAttrObject, "Объект", ", additional_columns: [{table: Объект.Товары, columns: [{id: "+formAttrColumn+", name: А, types: [{kind: catalog}]}]}]"),
			"attributes[0].additional_columns[0].columns[0].types[0].reference is required"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			source := formAttrHead + "items:\n  - {id: " + formAttrField + ", name: Поле, kind: input-field}\nattributes:\n" + test.attributes
			_, err := DecodeManagedForm("form.yaml", strings.NewReader(source), configuration)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err = %v, want %q", err, test.want)
			}
		})
	}
}

// A name of an attribute and of a column as long as a name may be - 255
// characters, the limit of every name of the configuration - is accepted.
//
// Defect caught: a limit counted one short, so that the longest name the
// designer saves refuses the form.
func TestAFormAttributeTakesTheLongestName(t *testing.T) {
	t.Parallel()
	source := formAttrHead + "attributes:\n  - {id: " + formAttrList + ", name: " + nameAt(maxNameLength) +
		", types: [{kind: value-table}], columns: [{id: " + formAttrColumn + ", name: " + nameAt(maxNameLength) + "}]}\n"
	if _, err := DecodeManagedForm("form.yaml", strings.NewReader(source), managedFormConfiguration()); err != nil {
		t.Fatal(err)
	}
}

// Every type only a form holds is accepted in an attribute of a form and in a
// column of one, and refused everywhere else - a field the database stores, a
// defined type, an attribute of a data processor - by a message that says
// where it may stand.
//
// Defect caught: a form type left out of the model, so that the 3451 dynamic
// lists and the rest of the forms being moved are refused; and a form type
// accepted in a stored field, which the schema would then have to build a
// column for.
func TestFormOnlyTypesStandInAFormAndNowhereElse(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	// Listed here and not taken from the model, so that a kind dropped from
	// the model is still tried.
	kinds := []TypeKind{"dynamic-list", "formatted-string", "color", "font", "picture", "formatted-document",
		"text-document", "gantt-chart", "planner", "dendrogram", "graphical-schema", "geographical-schema", "null-type"}
	if len(formOnlyKinds) != len(kinds) {
		t.Errorf("the model has %d form types, the test tries %d", len(formOnlyKinds), len(kinds))
	}
	for _, kind := range kinds {
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()
			source := formAttrHead + "attributes:\n  - {id: " + formAttrObject + ", name: Значение, types: [{kind: " + string(kind) + "}]," +
				" columns: [{id: " + formAttrColumn + ", name: Колонка, types: [{kind: " + string(kind) + "}]}]}\n"
			if _, err := DecodeManagedForm("form.yaml", strings.NewReader(source), configuration); err != nil {
				t.Fatalf("a form refused its own type: %v", err)
			}
			for place, where := range map[typePlace]string{
				placeStored: "in a field the database stores", placeDefinedType: "in a defined type",
				placeRunningObject: "in an attribute of a data processor or a report", placeCommandParameter: "in the parameter of a command",
			} {
				issues := validateTypesIn("types", []Type{{Kind: kind}}, place)
				if len(issues) != 1 || !strings.Contains(issues[0], "is a type of an attribute of a form and cannot stand "+where) {
					t.Errorf("%s: %v", where, issues)
				}
			}
			if issues := validateTypesIn("types", []Type{{Kind: kind, Length: 10}}, placeFormAttribute); len(issues) != 1 {
				t.Errorf("a form type took a qualifier: %v", issues)
			}
		})
	}
}

// A number of no length stands in a form and nowhere else; binary data stands
// nowhere in a form; what the designer marks «not available in form data» -
// value storage - and a reference to a table of an external source are kept.
//
// Defect caught: the 188 numbers of no length of the forms being moved
// refused, or a number of no length let into a stored field, where the column
// needs a precision; binary data accepted in a form against the designer;
// value storage refused, though the designer saves it and only marks it.
func TestAFormHoldsWhatTheDesignerSavesInIt(t *testing.T) {
	t.Parallel()
	unlimited := []Type{{Kind: NumberType}}
	if issues := validateTypesIn("types", unlimited, placeFormAttribute); len(issues) != 0 {
		t.Fatalf("a number of no length refused in a form: %v", issues)
	}
	for _, place := range []typePlace{placeStored, placeRunningObject, placeDefinedType} {
		if issues := validateTypesIn("types", unlimited, place); len(issues) != 1 || !strings.Contains(issues[0], "precision must be 1..") {
			t.Errorf("a number of no length %s: %v", placeName(place), issues)
		}
	}
	if issues := validateTypesIn("types", []Type{{Kind: NumberType, Scale: 2}}, placeFormAttribute); len(issues) == 0 {
		t.Error("a number of no length with a fraction accepted")
	}
	if issues := validateTypesIn("types", []Type{{Kind: BinaryDataType}}, placeFormAttribute); len(issues) != 1 || !strings.Contains(issues[0], "the designer does not offer it") {
		t.Errorf("binary data in a form: %v", issues)
	}
	if issues := validateTypesIn("types", []Type{{Kind: BinaryDataType}}, placeRunningObject); len(issues) != 0 {
		t.Errorf("binary data refused where it stood before: %v", issues)
	}
	for _, kept := range []Type{{Kind: ValueStorageType}, {Kind: ExternalTableType, Reference: refID(formAttrCatalog)}, {Kind: ValueTableType}, {Kind: PlatformType, Name: "Отбор"}} {
		if issues := validateTypesIn("types", []Type{kept}, placeFormAttribute); len(issues) != 0 {
			t.Errorf("%s refused in a form: %v", kept.Kind, issues)
		}
	}
}

func refID(value string) *uuid.UUID {
	id, err := uuid.Parse(value)
	if err != nil {
		panic(err)
	}
	return &id
}

// A dynamic list keeps what it reads and how, and a value list the type of
// its items, through YAML and through the Studio's JSON.
//
// Defect caught: a property of a dynamic list lost or read into the wrong
// field - the main table, the query, the key, the two «off» switches whose
// default is on; the type of a value list's items lost.
func TestADynamicListAndAValueListKeepTheirSettings(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	source := formAttrHead + `attributes:
  - id: ` + formAttrList + `
    name: Список
    types: [{kind: dynamic-list}]
    dynamic_list:
      main_table: {object: ` + formAttrCatalog + `, virtual: Balance}
      manual_query: true
      query_text: "ВЫБРАТЬ Ссылка ИЗ Справочник.Товары"
      dynamic_data_read: true
      key_type: field-value
      key_fields: [Ссылка, Дата]
      no_auto_fill_available_fields: true
      no_auto_save_user_settings: true
  - id: ` + formAttrObject + `
    name: Варианты
    types: [{kind: value-list}]
    value_type: [{kind: catalog, reference: ` + formAttrCatalog + `}, {kind: string, length: 10}]
`
	form, err := DecodeManagedForm("form.yaml", strings.NewReader(source), configuration)
	if err != nil {
		t.Fatal(err)
	}
	list := form.Attributes[0].DynamicList
	switch {
	case list == nil || list.MainTable == nil || list.MainTable.Object.String() != formAttrCatalog || list.MainTable.Virtual != "Balance":
		t.Fatalf("the main table lost: %+v", list)
	case !list.ManualQuery || list.QueryText != "ВЫБРАТЬ Ссылка ИЗ Справочник.Товары" || !list.DynamicDataRead:
		t.Fatalf("the query lost: %+v", list)
	case list.KeyType != DynamicListKeyFieldValue || len(list.KeyFields) != 2 || list.KeyFields[1] != "Дата":
		t.Fatalf("the key lost: %+v", list)
	case !list.NoAutoFillAvailableFields || !list.NoAutoSaveUserSettings:
		t.Fatalf("a switch turned off was lost: %+v", list)
	}
	if types := form.Attributes[1].ValueType; len(types) != 2 || types[0].Kind != CatalogType || types[1].Length != 10 {
		t.Fatalf("the type of the items lost: %+v", types)
	}
	written, err := yaml.Marshal(form)
	if err != nil {
		t.Fatal(err)
	}
	again, err := DecodeManagedForm("form.yaml", strings.NewReader(string(written)), configuration)
	if err != nil || !reflect.DeepEqual(again.Attributes, form.Attributes) {
		t.Fatalf("written back: %v\n%s", err, written)
	}
	carried, err := json.Marshal(form)
	if err != nil {
		t.Fatal(err)
	}
	var received ManagedForm
	if err := json.Unmarshal(carried, &received); err != nil || !reflect.DeepEqual(received.Attributes, form.Attributes) {
		t.Fatalf("carried through the Studio: %v %+v", err, received.Attributes)
	}
}

// Settings of a dynamic list and the type of a value list's items stand only
// on such attributes, and hold only what the platform knows.
//
// Defect caught: settings of a dynamic list on an attribute of another type,
// which nothing would ever read; a key type the platform does not have; a key
// field twice; an empty main table.
func TestDynamicListSettingsStandWhereTheyBelong(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	for name, test := range map[string]struct{ attribute, want string }{
		"настройки не у списка":              {"types: [{kind: value-table}], dynamic_list: {dynamic_data_read: true}", "dynamic_list belongs to an attribute that is a dynamic list"},
		"настройки у составного типа":        {"types: [{kind: dynamic-list}, {kind: boolean}], dynamic_list: {dynamic_data_read: true}", "dynamic_list belongs to an attribute that is a dynamic list"},
		"тип элементов не у списка значений": {"types: [{kind: value-table}], value_type: [{kind: boolean}]", "value_type belongs to a value list"},
		"тип элементов без ссылки":           {"types: [{kind: value-list}], value_type: [{kind: catalog}]", "value_type[0].reference is required"},
		"вид ключа":                  {"types: [{kind: dynamic-list}], dynamic_list: {key_type: primary}", "dynamic_list.key_type must be auto, field-value, row-key or row-number"},
		"поле ключа дважды":          {"types: [{kind: dynamic-list}], dynamic_list: {key_fields: [Ссылка, ссылка]}", "dynamic_list.key_fields[1] is already in the key"},
		"поле ключа не имя":          {"types: [{kind: dynamic-list}], dynamic_list: {key_fields: [\"Ссылка.Код\"]}", "dynamic_list.key_fields[0] must be the name of a field"},
		"основная таблица пустая":    {"types: [{kind: dynamic-list}], dynamic_list: {main_table: {object: 00000000-0000-0000-0000-000000000000}}", "dynamic_list.main_table.object must be a non-zero UUID"},
		"виртуальная таблица не имя": {"types: [{kind: dynamic-list}], dynamic_list: {main_table: {object: " + formAttrCatalog + ", virtual: \"Остатки(&Период)\"}}", "dynamic_list.main_table.virtual must be the name of a virtual table"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			source := formAttrHead + "attributes:\n  - {id: " + formAttrList + ", name: Список, " + test.attribute + "}\n"
			_, err := DecodeManagedForm("form.yaml", strings.NewReader(source), configuration)
			if err == nil || !strings.Contains(err.Error(), "attributes[0]."+test.want) {
				t.Fatalf("err = %v, want %q", err, test.want)
			}
		})
	}
}
