package metadata

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
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
  - {id: ` + formAttrField + `, name: Наименование, kind: field, data_path: Объект.Наименование}
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
		"заголовок колонки не на языке": {attribute(formAttrList, "Список", ", columns: [{id: "+formAttrColumn+", name: А, title: {\"d=e\": A}}]"),
			"attributes[0].columns[0].title"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			source := formAttrHead + "items:\n  - {id: " + formAttrField + ", name: Поле, kind: field}\nattributes:\n" + test.attributes
			_, err := DecodeManagedForm("form.yaml", strings.NewReader(source), configuration)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err = %v, want %q", err, test.want)
			}
		})
	}
}
