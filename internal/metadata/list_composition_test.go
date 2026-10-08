package metadata

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
	"go.yaml.in/yaml/v3"
)

// listCompositionWhole is a dynamic list whose settings set every property
// of a collection, an order item, a grouping and a field of it somewhere,
// written as the prototype writes them in the exports; the filter and the
// appearance are those of a form, checked by TestAFormKeepsItsConditionalAppearance.
const listCompositionWhole = `attributes:
  - id: c0de0000-0000-4000-8000-000000990020
    name: Список
    types: [{kind: dynamic-list}]
    dynamic_list:
      filter:
        items:
          - {left: {kind: field, data: ПараметрыДанных.ДокументОснование}, comparison: equal, right: [{kind: field}],
             user_setting_id: c4822df3-439f-463c-9363-bc96314469fd}
          - {left: {kind: field, data: Тип}, comparison: in-list, right: [{kind: predefined, data: Документ.РасходныйОрдерНаТовары},
             {kind: predefined, data: Документ.ОрдерНаПеремещениеТоваров}]}
        view_mode: normal
        user_setting_id: dfcece9d-5077-440b-b6b3-45a5cb4538eb
        user_setting_presentation: {kind: localized-string, text: {ru: Отбор}}
      order:
        items:
          - {field: Ссылка.Дата, direction: desc, view_mode: inaccessible}
          - {field: Организация, direction: asc, disabled: true}
          - {auto: true, disabled: true}
        view_mode: normal
        user_setting_id: 88619765-ccb3-46c6-ac52-38e9c992ebd4
      conditional_appearance:
        items:
          - fields: [{field: Ссылка.Номер}]
            filter: [{left: {kind: field, data: Проведен}, comparison: equal, right: [{kind: boolean, data: "false"}]}]
            appearance: [{parameter: text-color, value: {kind: color, color: {source: style, from: {standard: InaccessibleDataColor}}}}]
          - appearance: []
        view_mode: normal
        user_setting_id: b75fecce-942b-4aed-abc9-e6a02e460fb3
      group:
        items:
          - fields:
              - {field: Статус, disabled: true, type: items, period_addition: none}
            groups:
              - fields:
                  - {field: Ссылка.Дата, type: hierarchy-only, period_addition: month-since-begin-of-period-445,
                     period_begin: "2020-01-01T00:00:00", period_end: "2020-12-31T23:59:59"}
        view_mode: normal
        user_setting_id: 911b6018-f537-43e8-a417-da56b22f9aec
`

// A dynamic list keeps its settings whole - filter, order, appearance,
// grouping and the user setting of each - through YAML and through the
// Studio, and the runtime copy of them shares nothing with the snapshot.
//
// Defect caught: a collection, an item of an order or a grouping the
// exports write refused or read into the wrong property; a grouping nested
// in another lost; an automatic order item taken for one by a field; a
// field of the appearance that is a path of the list refused, as a name of
// an element of the form would be; a field not chosen on the right of a
// comparison, or an item of the appearance that sets nothing, refused,
// though the prototype writes both; a property of the model that nothing
// writes or reads, set by no case here.
func TestADynamicListKeepsItsSettings(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	form, err := DecodeManagedForm("form.yaml", strings.NewReader(formAttrHead+listCompositionWhole), configuration)
	if err != nil {
		t.Fatal(err)
	}
	list := form.Attributes[0].DynamicList
	filter, order, appearance, groups := list.Filter, list.Order, list.ConditionalAppearance, list.Group
	switch {
	case len(filter.Items) != 2 || filter.Items[0].Right[0].Kind != CompositionField || filter.Items[0].Right[0].Data != "" ||
		len(filter.Items[1].Right) != 2 || filter.Items[1].Right[1].Data != "Документ.ОрдерНаПеремещениеТоваров" ||
		filter.ViewMode != "normal" || filter.UserSettingID.String() != "dfcece9d-5077-440b-b6b3-45a5cb4538eb" || filter.UserSettingPresentation.Text["ru"] != "Отбор":
		t.Fatalf("filter: %+v", filter)
	case len(order.Items) != 3 || order.Items[0].Field != "Ссылка.Дата" || order.Items[0].Direction != "desc" || order.Items[0].ViewMode != "inaccessible" ||
		order.Items[0].Disabled || !order.Items[1].Disabled || order.Items[1].Direction != "asc" || !order.Items[2].Auto || order.Items[2].Field != "" ||
		order.UserSettingID.String() != "88619765-ccb3-46c6-ac52-38e9c992ebd4":
		t.Fatalf("order: %+v", order)
	case len(appearance.Items) != 2 || appearance.Items[0].Fields[0].Field != "Ссылка.Номер" || len(appearance.Items[1].Appearance) != 0 ||
		appearance.Items[0].Appearance[0].Value.Color.From.Standard != "InaccessibleDataColor":
		t.Fatalf("appearance: %+v", appearance)
	case len(groups.Items) != 1 || groups.Items[0].Fields[0].Field != "Статус" || !groups.Items[0].Fields[0].Disabled || groups.Items[0].Fields[0].Type != "items" ||
		len(groups.Items[0].Groups) != 1 || groups.Items[0].Groups[0].Fields[0].Type != "hierarchy-only" ||
		groups.Items[0].Groups[0].Fields[0].PeriodAddition != "month-since-begin-of-period-445" || groups.Items[0].Groups[0].Fields[0].PeriodEnd != "2020-12-31T23:59:59" ||
		groups.UserSettingID.String() != "911b6018-f537-43e8-a417-da56b22f9aec":
		t.Fatalf("group: %+v", groups)
	}
	if unset := fieldsNeverSet(reflect.ValueOf([]any{order, groups, filter.UserSetting})); len(unset) != 0 {
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
	copied.Filter.Items[1].Right[0].Data = "changed"
	copied.Order.Items[0].Field = "changed"
	copied.ConditionalAppearance.Items[0].Fields[0].Field = "changed"
	copied.Group.Items[0].Groups[0].Fields[0].Field = "changed"
	copied.Filter.UserSettingPresentation.Text["ru"] = "changed"
	if list.Filter.Items[1].Right[0].Data != "Документ.РасходныйОрдерНаТовары" || list.Order.Items[0].Field != "Ссылка.Дата" ||
		list.ConditionalAppearance.Items[0].Fields[0].Field != "Ссылка.Номер" || list.Group.Items[0].Groups[0].Fields[0].Field != "Ссылка.Дата" ||
		list.Filter.UserSettingPresentation.Text["ru"] != "Отбор" {
		t.Fatal("the runtime copy shares the settings of the list with the snapshot")
	}
}

// Settings of a dynamic list that are wrong are refused, naming the place.
//
// Defect caught: an order item with no field or no direction, or one not in
// the help; an automatic order item carrying a field, a direction or a user
// setting; a grouping by a field that is no path, of a type or with an
// addition of periods not in the help, or bounded by what is no date; a
// wrong field in a grouping nested in another; a field of the appearance
// that is no path; a filter, an appearance or a user setting of a
// collection never checked, so that what is wrong in them loads.
func TestTheSettingsOfADynamicListRefuseWhatIsWrong(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	list := func(settings string) string {
		return formAttrHead + "attributes:\n  - {id: c0de0000-0000-4000-8000-000000990020, name: Список, types: [{kind: dynamic-list}], dynamic_list: {" + settings + "}}\n"
	}
	const at = "attributes[0].dynamic_list."
	field := func(rest string) string {
		return list("group: {items: [{fields: [{field: Статус, " + rest + "}]}]}")
	}
	for name, test := range map[string]struct{ body, want string }{
		"порядок без поля":          {list("order: {items: [{direction: asc}]}"), at + "order.items[0].field must contain only valid identifier segments"},
		"порядок без направления":   {list("order: {items: [{field: Дата}]}"), at + "order.items[0].direction must be one of"},
		"порядок незнакомый":        {list("order: {items: [{field: Дата, direction: up}]}"), at + "order.items[0].direction must be one of"},
		"авто с полем":              {list("order: {items: [{auto: true, field: Дата}]}"), at + "order.items[0] is automatic and has nothing but its use"},
		"авто с режимом":            {list("order: {items: [{auto: true, view_mode: normal}]}"), at + "order.items[0] is automatic and has nothing but its use"},
		"режим элемента порядка":    {list("order: {items: [{field: Дата, direction: asc, view_mode: hidden}]}"), at + "order.items[0].view_mode must be one of"},
		"режим порядка":             {list("order: {view_mode: hidden}"), at + "order.view_mode must be one of"},
		"группировка не путь":       {list("group: {items: [{fields: [{field: \"Статус..Код\", type: items, period_addition: none}]}]}"), at + "group.items[0].fields[0].field must contain only valid identifier segments"},
		"группировка без типа":      {field("period_addition: none"), at + "group.items[0].fields[0].type must be one of"},
		"группировка тип":           {field("type: folders, period_addition: none"), at + "group.items[0].fields[0].type must be one of"},
		"без дополнения":            {field("type: items"), at + "group.items[0].fields[0].period_addition must be one of"},
		"дополнение незнакомое":     {field("type: items, period_addition: fortnight"), at + "group.items[0].fields[0].period_addition must be one of"},
		"начало не дата":            {field("type: items, period_addition: day, period_begin: \"2020-01-01\""), at + "group.items[0].fields[0].period_begin must be a date"},
		"конец не дата":             {field("type: items, period_addition: day, period_end: завтра"), at + "group.items[0].fields[0].period_end must be a date"},
		"вложенная группировка":     {list("group: {items: [{groups: [{fields: [{field: Статус, type: items}]}]}]}"), at + "group.items[0].groups[0].fields[0].period_addition must be one of"},
		"идентификатор группировок": {list("group: {user_setting_id: 00000000-0000-0000-0000-000000000000}"), at + "group.user_setting_id must be a non-zero UUID"},
		"поле оформления не путь": {list("conditional_appearance: {items: [{fields: [{field: \"Ссылка..Номер\"}], appearance: []}]}"),
			at + "conditional_appearance.items[0].fields[0].field must be the path of a field of the list"},
		"оформление проверяется": {list("conditional_appearance: {items: [{appearance: [{parameter: visible, value: {kind: color, color: {source: auto}}}]}]}"),
			at + "conditional_appearance.items[0].appearance[0].value.kind must be one of boolean"},
		"режим оформления": {list("conditional_appearance: {view_mode: hidden}"), at + "conditional_appearance.view_mode must be one of"},
		"отбор проверяется": {list("filter: {items: [{left: {kind: field, data: Сумма}, comparison: equal, right: [{kind: number, data: x}]}]}"),
			at + "filter.items[0].right[0].data must be a decimal"},
		"поле слева пустое": {list("filter: {items: [{left: {kind: field}, comparison: filled}]}"), at + "filter.items[0].left.data must contain only valid identifier segments"},
		"представление отбора": {list("filter: {user_setting_presentation: {kind: field, data: Сумма}}"),
			at + "filter.user_setting_presentation.kind must be one of string, localized-string"},
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

// What the settings of a dynamic list name in the project is resolved at
// load: a colour of their appearance taken from a style item that is gone
// is a reference to nothing, one taken from a style item of another type is
// refused; a value of kind type in their filter, and in the appearance of a
// form, is noted where it stands.
//
// Defect caught: the settings of a list never resolved at load, so that a
// style item deleted from under them loads clean; a value the meaning of
// which is not known carried with no note in the import report.
func TestWhatTheSettingsOfADynamicListNameIsResolved(t *testing.T) {
	t.Parallel()
	const font = "c0de0000-0000-4000-8000-000000990031"
	listAttribute := func(settings string) string {
		return "  - {id: c0de0000-0000-4000-8000-000000990020, name: Документы, types: [{kind: dynamic-list}], dynamic_list: {" + settings + "}}\n"
	}
	typed := "{left: {kind: field, data: Тип}, comparison: in-list, right: [{kind: predefined, data: Документ.Поступление}, {kind: type, data: Undefined}]}"
	for name, test := range map[string]struct {
		settings, appearance string
		gone, refused        string
		notes                []string
	}{
		"элемента стиля нет": {settings: "conditional_appearance: {items: [{appearance: [{parameter: back-color, value: {kind: color, color: {source: style, from: {item: " + refGone + "}}}}]}]}",
			gone: "catalog Номенклатура form ФормаЭлемента attribute Документы dynamic_list conditional_appearance items[0].appearance[0].value.color"},
		"шрифт из шрифта, цвет из шрифта": {settings: "conditional_appearance: {items: [{appearance: [{parameter: font, value: {kind: font, font: {source: style, from: {item: " + font + "}}}}," +
			" {parameter: text-color, value: {kind: color, color: {source: style, from: {item: " + font + "}}}}]}]}",
			refused: "dynamic_list conditional_appearance items[0].appearance[1].value.color takes its value from style item Мелкий, which is a font and not a color"},
		"тип в отборе": {settings: "filter: {items: [{group: or, items: [" + typed + "]}]}",
			appearance: "conditional_appearance:\n  - {filter: [" + typed + "], appearance: [{parameter: visible, value: {kind: boolean, data: \"false\"}}]}\n",
			notes: []string{"catalog Номенклатура form ФормаЭлемента attribute Документы dynamic_list filter items[0].items[0].right[1] = Undefined",
				"catalog Номенклатура form ФормаЭлемента conditional appearance [0].filter[0].right[1] = Undefined"}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := formReferencesProject(t)
			writeMetadata(t, root, StyleItemKind, font, "format: 1\nid: "+font+"\nname: Мелкий\ntitle: {ru: Мелкий}\ntype: font\nvalue: {font: {source: auto}}\n")
			path := filepath.Join(root, "metadata", string(CatalogKind), "Номенклатура", "forms", "ФормаЭлемента", project.FormMetadataFile)
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, path, string(content)+listAttribute(test.settings)+test.appearance)
			if test.gone != "" {
				found := unresolvedOf(t, root)
				if len(found) != 1 || found[0].Where != test.gone {
					t.Fatalf("unresolved = %+v", found)
				}
				return
			}
			catalog, err := Load(root)
			if test.refused != "" {
				if err == nil || !strings.Contains(err.Error(), test.refused) {
					t.Fatalf("err = %v, want %q", err, test.refused)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var found []string
			for _, note := range catalog.Notes() {
				if note.Kind == NoteCompositionTypeUnexplained {
					found = append(found, note.Where+" = "+note.Written)
				}
			}
			sort.Strings(found)
			if !reflect.DeepEqual(found, test.notes) {
				t.Fatalf("notes = %q, want %q", found, test.notes)
			}
		})
	}
}

// A value by name is looked for in the project as the platform looks for
// it: the object by kind and name, then the value of an enumeration or the
// predefined item of a catalog or a chart, or the empty reference of any of
// them; two parts name the object alone. Names are compared whatever their
// case, the kind is written in either language of the code.
//
// Defect caught: a value looked for among the items of another kind of
// object, or of another object of the same kind; a value found only when
// written in the case of its definition (erp writes Незапущен for
// НеЗапущен); the empty reference taken for the name of an item; a value
// of an object the project does not have, or one it keeps with no values
// at all, taken for found.
func TestAValueByNameIsFoundInTheProject(t *testing.T) {
	t.Parallel()
	catalog := &Catalog{
		Enumerations: []Enumeration{{Name: "СтатусыБюджетов", Values: []EnumerationValue{{Name: "НеЗапущен"}, {Name: "Завершен"}}},
			{Name: "СтатусыСделок"}},
		Catalogs:                    []CatalogDefinition{{Name: "Организации", Predefined: []PredefinedCatalogItem{{Name: "Управленческая"}}}, {Name: "Склады"}},
		ChartsOfCharacteristicTypes: []ChartOfCharacteristicTypesDefinition{{Name: "Настройки", Predefined: []PredefinedCharacteristic{{PredefinedCatalogItem: PredefinedCatalogItem{Name: "Подбор"}}}}},
		ChartsOfAccounts:            []ChartOfAccountsDefinition{{Name: "Хозрасчетный", Predefined: []PredefinedAccount{{Name: "Депоненты"}}}},
		ChartsOfCalculationTypes:    []ChartOfCalculationTypesDefinition{{Name: "Начисления", Predefined: []PredefinedCalculationType{{Name: "Оклад"}}}},
		Documents:                   []DocumentDefinition{{Name: "ЧекККМ"}},
		BusinessProcesses:           []BusinessProcessDefinition{{Name: "Задание"}},
		Tasks:                       []TaskDefinition{{Name: "ЗадачаИсполнителя"}},
	}
	for value, want := range map[string]bool{
		"Перечисление.СтатусыБюджетов.НеЗапущен":           true,
		"Перечисление.СтатусыБюджетов.Незапущен":           true,
		"перечисление.статусыбюджетов.завершен":            true,
		"Enum.СтатусыБюджетов.Завершен":                    true,
		"Перечисление.СтатусыБюджетов.ПустаяСсылка":        true,
		"Перечисление.СтатусыБюджетов":                     true,
		"Справочник.Организации.Управленческая":            true,
		"Catalog.Организации.EmptyRef":                     true,
		"Справочник.Склады.ПустаяСсылка":                   true,
		"ПланВидовХарактеристик.Настройки.Подбор":          true,
		"ПланСчетов.Хозрасчетный.Депоненты":                true,
		"ПланВидовРасчета.Начисления.Оклад":                true,
		"Документ.ЧекККМ":                                  true,
		"Документ.ЧекККМ.ПустаяСсылка":                     true,
		"БизнесПроцесс.Задание.ПустаяСсылка":               true,
		"Задача.ЗадачаИсполнителя.ПустаяСсылка":            true,
		"Перечисление.СтатусыБюджетов.Отменен":             false,
		"Перечисление.СтатусыСделок.ВРаботе":               false,
		"Перечисление.Удалено.Значение":                    false,
		"Перечисление.Удалено":                             false,
		"Перечисление.Организации.Управленческая":          false,
		"Справочник.СтатусыБюджетов.Завершен":              false,
		"Справочник.Склады.Управленческая":                 false,
		"ПланСчетов.Хозрасчетный.Оклад":                    false,
		"ПланВидовРасчета.Начисления.Депоненты":            false,
		"ПланВидовХарактеристик.Настройки.Управленческая":  false,
		"Документ.Удалено":                                 false,
		"Документ.Удалено.ПустаяСсылка":                    false,
		"Задача.ЗадачаИсполнителя.Первая":                  false,
		"Константа.ОсновнойСклад.ПустаяСсылка":             false,
		"ПланОбмена.Филиалы.ПустаяСсылка":                  false,
		"БизнесПроцесс.Задание.ТочкаМаршрута.Утверждение":  false,
		"Справочник.Организации.Управленческая.Лишнее.Ещё": false,
	} {
		if got := catalog.hasPredefinedValue(value); got != want {
			t.Errorf("%s: found = %t, want %t", value, got, want)
		}
	}
}

// A value by name the project does not have, in the conditional appearance
// of a form and in the filter and appearance of a dynamic list, is a
// remnant of what was deleted (2.203), named where it stands; one the
// project has loads clean.
//
// Defect caught: a value of a deleted enumeration, or a deleted value of
// one, loading clean though it compares with nothing; the values of a
// filter or of the appearance of a list left unresolved, or those nested in
// a group of a filter.
func TestAValueByNameTheProjectDoesNotHaveIsARemnant(t *testing.T) {
	t.Parallel()
	visible := "appearance: [{parameter: visible, value: {kind: boolean, data: \"false\"}}]"
	compare := func(value string) string {
		return "{left: {kind: field, data: Вид}, comparison: equal, right: [{kind: predefined, data: " + value + "}]}"
	}
	list := func(settings string) string {
		return "  - {id: c0de0000-0000-4000-8000-000000990020, name: Документы, types: [{kind: dynamic-list}], dynamic_list: {" + settings + "}}\n"
	}
	const at = "catalog Номенклатура form ФормаЭлемента "
	for name, test := range map[string]struct{ attributes, appearance, gone, written string }{
		"всё на месте": {attributes: list("filter: {items: [" + compare("Перечисление.ВидыТоваров.Товар") + ", " + compare("Документ.Поступление") + "]}, " +
			"conditional_appearance: {items: [{filter: [" + compare("Справочник.Склады.ПустаяСсылка") + "], " + visible + "}]}"),
			appearance: "  - {filter: [" + compare("Задача.Задача.ПустаяСсылка") + "], " + visible + "}\n"},
		"значения нет в оформлении формы": {appearance: "  - {filter: [" + compare("Перечисление.ВидыТоваров.Товар") + "], " + visible + "}\n  - {filter: [" + compare("Перечисление.ВидыТоваров.Услуга") + "], " + visible + "}\n",
			gone: at + "conditional appearance [1].filter[0].right[0]", written: "Перечисление.ВидыТоваров.Услуга"},
		"перечисления нет в отборе списка": {attributes: list("filter: {items: [{group: and, items: [" + compare("Перечисление.Удалено.Товар") + "]}]}"),
			gone: at + "attribute Документы dynamic_list filter items[0].items[0].right[0]", written: "Перечисление.Удалено.Товар"},
		"документа нет в оформлении списка": {attributes: list("conditional_appearance: {items: [{filter: [" + compare("Документ.Удалено") + "], " + visible + "}]}"),
			gone: at + "attribute Документы dynamic_list conditional_appearance items[0].filter[0].right[0]", written: "Документ.Удалено"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := formReferencesProject(t)
			path := filepath.Join(root, "metadata", string(CatalogKind), "Номенклатура", "forms", "ФормаЭлемента", project.FormMetadataFile)
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			body := string(content) + test.attributes
			if test.appearance != "" {
				body += "conditional_appearance:\n" + test.appearance
			}
			writeFile(t, path, body)
			if test.gone == "" {
				if _, err := Load(root); err != nil {
					t.Fatal(err)
				}
				return
			}
			found := unresolvedOf(t, root)
			if len(found) != 1 || found[0].Where != test.gone || found[0].Written != test.written {
				t.Fatalf("unresolved = %+v", found)
			}
		})
	}
}
