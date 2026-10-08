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
	"github.com/k33alexey/MetaLab/internal/uuid"
	"go.yaml.in/yaml/v3"
)

// conditionalAppearanceWhole is a conditional appearance with every
// property of an item, a filter item, an appearance value and a value, and
// every kind of value, set somewhere: written as the prototype writes them
// in the exports.
const conditionalAppearanceWhole = `conditional_appearance:
  - disabled: true
    fields: [{field: Сумма}, {field: Цена, disabled: true}]
    filter:
      - {left: {kind: field, data: Объект.Сумма}, comparison: greater, right: [{kind: number, data: "-1.5"}], disabled: true,
         presentation: {kind: localized-string, text: {ru: Больше, uk: Більше}}}
      - group: or
        disabled: true
        presentation: {kind: string, data: Группа}
        items:
          - {left: {kind: field, data: Объект.Вид}, comparison: in-list, right: [{kind: value-list}]}
          - {left: {kind: field, data: Объект.Ставка}, comparison: not-equal, right: [{kind: predefined, data: Перечисление.СтавкиНДС.НДС0}]}
          - group: not
            items:
              - {left: {kind: field, data: Объект.Дата}, comparison: less, right: [{kind: standard-beginning-date, data: custom, date: "2015-01-01T00:00:00"}]}
              - {left: {kind: field, data: Объект.Дата}, comparison: greater-or-equal, right: [{kind: standard-beginning-date, data: beginning-of-this-day}]}
          - {left: {kind: boolean, data: "true"}, comparison: equal, right: [{kind: boolean, data: "true"}]}
          - {left: {kind: field, data: Объект.Время}, comparison: equal, right: [{kind: date, data: "0001-01-01T00:00:00"}]}
          - {left: {kind: field, data: Подразделение}, comparison: not-like, right: [{kind: "null"}]}
          - {left: {kind: field, data: Комментарий}, comparison: contains, right: [{kind: string, data: заг}]}
          - {left: {kind: field, data: ДатаНачала}, comparison: less-or-equal, right: [{kind: field, data: ДатаОкончания}]}
          - {left: {kind: field, data: Комментарий}, comparison: equal, right: [{kind: undefined}]}
          - {left: {kind: field, data: Комментарий}, comparison: not-filled}
          - {left: {kind: field, data: Тип}, comparison: not-in-list, right: [{kind: predefined, data: Документ.ЧекККМ}, {kind: undefined},
             {kind: type, data: Undefined}], view_mode: inaccessible, user_setting_id: 23577b33-7493-4682-a79c-5e9334cba679,
             user_setting_presentation: {kind: string, data: Тип}}
          - {left: {kind: field, data: Счет.Вид}, comparison: equal, right: [{kind: account-type, data: active-passive}]}
    appearance:
      - {parameter: text-color, value: {kind: color, color: {source: style, from: {standard: SpecialTextColor}}}}
      - {parameter: back-color, value: {kind: color, color: {source: absolute, rgb: "#C0C0C0"}}}
      - {parameter: font, value: {kind: font, font: {source: system, system: DefaultGUIFont, face: Roboto, size: 10}}}
      - {parameter: horizontal-align, value: {kind: horizontal-align, data: justify}}
      - {parameter: visible, disabled: true, value: {kind: boolean, data: "false"}}
      - {parameter: enabled, value: {kind: boolean, data: "false"}}
      - {parameter: read-only, value: {kind: boolean, data: "true"}}
      - {parameter: show, value: {kind: boolean, data: "false"}}
      - {parameter: mark-incomplete, value: {kind: boolean, data: "true"}}
      - {parameter: text, value: {kind: localized-string, text: {ru: "<...>", uk: "<...>"}}}
      - {parameter: format, value: {kind: string, data: ЧДЦ=2}}
      - {parameter: mark-negatives, value: {kind: boolean, data: "true"}, view_mode: normal, user_setting_id: 8abd341d-0b30-411a-8df6-90600f7b2565,
         user_setting_presentation: {kind: localized-string, text: {ru: Отрицательные}}}
    presentation: {kind: localized-string, text: {ru: Выделение подобранных}}
    view_mode: quick-access
    user_setting_id: b75fecce-942b-4aed-abc9-e6a02e460fb3
    user_setting_presentation: {kind: string, data: Подобранные}
  - appearance: [{parameter: text, value: {kind: field, data: Объект.Представление}}]
  - appearance: [{parameter: text, disabled: true, value: {kind: undefined}}]
  - appearance: [{parameter: text, value: {kind: localized-string}}]
`

// A form keeps its conditional appearance whole, through YAML and through
// the Studio, and the runtime copy of it shares nothing with the snapshot.
//
// Defect caught: a kind of value, a comparison, a group or a parameter the
// exports write refused or read into the wrong property; a filter of a group
// lost at the second level and below; a parameter turned off read as set,
// or a field turned off read as drawn; a localized text and a plain one
// taken for one another; a property of the model that nothing writes or
// reads, set by no case here.
func TestAFormKeepsItsConditionalAppearance(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	form, err := DecodeManagedForm("form.yaml", strings.NewReader(formAttrHead+conditionalAppearanceWhole), configuration)
	if err != nil {
		t.Fatal(err)
	}
	items := form.ConditionalAppearance
	if len(items) != 4 {
		t.Fatalf("items: %d", len(items))
	}
	first := items[0]
	group := first.Filter[1]
	switch {
	case !first.Disabled || len(first.Fields) != 2 || first.Fields[0].Field != "Сумма" || first.Fields[0].Disabled || !first.Fields[1].Disabled:
		t.Fatalf("fields: %+v", first.Fields)
	case first.Filter[0].Comparison != "greater" || first.Filter[0].Left.Data != "Объект.Сумма" || len(first.Filter[0].Right) != 1 ||
		first.Filter[0].Right[0].Kind != CompositionNumber || first.Filter[0].Right[0].Data != "-1.5" || !first.Filter[0].Disabled || first.Filter[0].Presentation.Text["uk"] != "Більше":
		t.Fatalf("comparison: %+v", first.Filter[0])
	case group.Group != "or" || !group.Disabled || group.Presentation.Kind != CompositionString || len(group.Items) != 12 ||
		group.Items[2].Group != "not" || group.Items[2].Items[0].Right[0].Date != "2015-01-01T00:00:00" || group.Items[2].Items[1].Right[0].Data != "beginning-of-this-day":
		t.Fatalf("group: %+v", group)
	case group.Items[1].Right[0].Kind != CompositionPredefined || group.Items[1].Right[0].Data != "Перечисление.СтавкиНДС.НДС0" || group.Items[9].Right != nil:
		t.Fatalf("operands: %+v %+v", group.Items[1], group.Items[9])
	case len(group.Items[10].Right) != 3 || group.Items[10].Right[0].Data != "Документ.ЧекККМ" || group.Items[10].Right[1].Kind != CompositionUndefined ||
		group.Items[10].Right[2].Kind != CompositionType || group.Items[10].ViewMode != "inaccessible" || group.Items[10].UserSettingPresentation.Data != "Тип":
		t.Fatalf("a list of values: %+v", group.Items[10])
	case first.ViewMode != "quick-access" || first.UserSettingID.String() != "b75fecce-942b-4aed-abc9-e6a02e460fb3" || first.UserSettingPresentation.Data != "Подобранные":
		t.Fatalf("user setting: %+v", first.UserSetting)
	case first.Appearance[2].Value.Font.System != "DefaultGUIFont" || first.Appearance[2].Value.Font.Face != "Roboto" ||
		!first.Appearance[4].Disabled || first.Appearance[5].Disabled || first.Appearance[9].Value.Text["ru"] != "<...>" ||
		first.Appearance[11].ViewMode != "normal" || first.Appearance[11].UserSettingID.String() != "8abd341d-0b30-411a-8df6-90600f7b2565":
		t.Fatalf("appearance: %+v", first.Appearance)
	case items[1].Appearance[0].Value.Kind != CompositionField || items[2].Appearance[0].Value.Kind != CompositionUndefined ||
		items[3].Appearance[0].Value.Kind != CompositionLocalizedString || len(items[3].Appearance[0].Value.Text) != 0:
		t.Fatalf("texts: %+v %+v %+v", items[1], items[2], items[3])
	}
	if unset := fieldsNeverSet(reflect.ValueOf(items)); len(unset) != 0 {
		t.Fatalf("set by no case: %v", unset)
	}
	written, err := yaml.Marshal(form)
	if err != nil {
		t.Fatal(err)
	}
	again, err := DecodeManagedForm("form.yaml", strings.NewReader(string(written)), configuration)
	if err != nil || !reflect.DeepEqual(again.ConditionalAppearance, items) {
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
	if err := ValidateManagedForm("studio", received, configuration); err != nil || !reflect.DeepEqual(received.ConditionalAppearance, items) {
		t.Fatalf("carried through the Studio: %v", err)
	}
	copied := cloneRuntimeForm(form)
	copied.ConditionalAppearance[0].Filter[1].Items[2].Items[0].Right[0].Date = "changed"
	copied.ConditionalAppearance[0].Appearance[9].Value.Text["ru"] = "changed"
	copied.ConditionalAppearance[0].Appearance[2].Value.Font.Face = "changed"
	*copied.ConditionalAppearance[0].UserSettingID = uuid.UUID{}
	if form.ConditionalAppearance[0].Filter[1].Items[2].Items[0].Right[0].Date != "2015-01-01T00:00:00" || form.ConditionalAppearance[0].UserSettingID.IsZero() ||
		form.ConditionalAppearance[0].Appearance[9].Value.Text["ru"] != "<...>" || form.ConditionalAppearance[0].Appearance[2].Value.Font.Face != "Roboto" {
		t.Fatal("the runtime copy shares the conditional appearance with the snapshot")
	}
}

// fieldsNeverSet names the exported fields of the structs a value holds
// that hold their zero in every place they stand: a field of a union is
// set only in some of its places, and one place is enough.
func fieldsNeverSet(root reflect.Value) []string {
	seen, set := map[string]bool{}, map[string]bool{}
	var walk func(value reflect.Value)
	walk = func(value reflect.Value) {
		switch value.Kind() {
		case reflect.Pointer:
			if !value.IsNil() {
				switch value.Interface().(type) {
				case *ColorValue, *FontValue:
					return
				}
				walk(value.Elem())
			}
		case reflect.Slice:
			for index := range value.Len() {
				walk(value.Index(index))
			}
		case reflect.Struct:
			for index := range value.NumField() {
				field := value.Type().Field(index)
				if !field.IsExported() {
					continue
				}
				name := value.Type().Name() + "." + field.Name
				seen[name] = true
				if !value.Field(index).IsZero() {
					set[name] = true
				}
				walk(value.Field(index))
			}
		}
	}
	walk(root)
	var unset []string
	for name := range seen {
		if !set[name] {
			unset = append(unset, name)
		}
	}
	sort.Strings(unset)
	return unset
}

// A conditional appearance that is wrong is refused, naming the place.
//
// Defect caught: a value of a kind its place does not take accepted - a
// colour where the text colour takes a boolean, a colour as an operand of a
// filter; a value carrying what belongs to another kind; a malformed number,
// date, name of a predefined value or variant of a beginning date; a custom
// beginning date with no date; a parameter set twice or unknown; a group
// that compares, a comparison that groups, a comparison with nothing on the
// left or no kind of comparison; a presentation of an item or a filter item
// that is no text; a field that is no name; a value by name of one part or
// four; a wrong value on the right taken, for having no path or no data,
// for a field not chosen; several values compared by a comparison that is not with a list, or
// a wrong one past the first; a type that is no name, a type of account or
// a view mode not in the help; a user setting with a zero identifier or a
// presentation that is no text.
func TestAConditionalAppearanceRefusesWhatIsWrong(t *testing.T) {
	t.Parallel()
	configuration := managedFormConfiguration()
	item := func(rest string) string {
		return "conditional_appearance:\n  - " + rest + "\n"
	}
	set := "appearance: [{parameter: visible, value: {kind: boolean, data: \"false\"}}]"
	compare := func(comparison string) string { return item("{filter: [" + comparison + "], " + set + "}") }
	for name, test := range map[string]struct{ body, want string }{
		"цвет текста булевым": {item("{appearance: [{parameter: text-color, value: {kind: boolean, data: \"true\"}}]}"),
			"conditional_appearance[0].appearance[0].value.kind must be one of color"},
		"параметр дважды": {item("{appearance: [{parameter: show, value: {kind: boolean, data: \"true\"}}, {parameter: show, value: {kind: boolean, data: \"false\"}}]}"),
			"conditional_appearance[0].appearance[1].parameter show is set twice"},
		"параметр незнакомый": {item("{appearance: [{parameter: indent, value: {kind: number, data: \"1\"}}]}"),
			"conditional_appearance[0].appearance[0].parameter is not a parameter of the appearance the model carries"},
		"поле не имя":       {item("{fields: [{field: Объект.Сумма}], " + set + "}"), "conditional_appearance[0].fields[0].field must be the name of an element of the form"},
		"булево не булево":  {item("{appearance: [{parameter: visible, value: {kind: boolean, data: \"да\"}}]}"), "appearance[0].value.data must be true or false"},
		"булево без данных": {item("{appearance: [{parameter: visible, value: {kind: boolean}}]}"), "appearance[0].value.data must be true or false"},
		"цвет без цвета":    {item("{appearance: [{parameter: back-color, value: {kind: color}}]}"), "appearance[0].value.color must say what the colour is"},
		"цвет неверный": {item("{appearance: [{parameter: back-color, value: {kind: color, color: {source: absolute, rgb: red}}}]}"),
			"appearance[0].value.color.rgb must be a colour written as #RRGGBB"},
		"шрифт без шрифта": {item("{appearance: [{parameter: font, value: {kind: font}}]}"), "appearance[0].value.font must say what the font is"},
		"шрифт неверный": {item("{appearance: [{parameter: font, value: {kind: font, font: {source: system}}}]}"),
			"appearance[0].value.font.system must name the font of the system"},
		"выравнивание":       {item("{appearance: [{parameter: horizontal-align, value: {kind: horizontal-align, data: middle}}]}"), "appearance[0].value.data must be one of"},
		"чужое у значения":   {item("{appearance: [{parameter: visible, value: {kind: boolean, data: \"true\", text: {ru: да}}}]}"), "appearance[0].value.text does not belong to a value of kind boolean"},
		"цвет с данными":     {item("{appearance: [{parameter: back-color, value: {kind: color, data: red, color: {source: auto}}}]}"), "appearance[0].value.data does not belong to a value of kind color"},
		"текст не язык":      {item("{appearance: [{parameter: text, value: {kind: localized-string, text: {\"r u\": да}}}]}"), "appearance[0].value.text.r u is not a language code"},
		"поле без имени":     {item("{appearance: [{parameter: text, value: {kind: field}}]}"), "appearance[0].value.data must contain only valid identifier segments"},
		"поле не путь":       {item("{appearance: [{parameter: text, value: {kind: field, data: \"Объект..Сумма\"}}]}"), "appearance[0].value.data must contain only valid identifier segments"},
		"представление поле": {item("{" + set + ", presentation: {kind: field, data: Сумма}}"), "conditional_appearance[0].presentation.kind must be one of string, localized-string"},
		"представление отбора поле": {compare("{left: {kind: field, data: Сумма}, comparison: filled, presentation: {kind: field, data: Сумма}}"),
			"filter[0].presentation.kind must be one of string, localized-string"},
		"число не число": {compare("{left: {kind: field, data: Сумма}, comparison: equal, right: [{kind: number, data: \"1,5\"}]}"), "filter[0].right[0].data must be a decimal"},
		"дата не дата":   {compare("{left: {kind: field, data: Дата}, comparison: equal, right: [{kind: date, data: \"2015-01-01\"}]}"), "filter[0].right[0].data must be a date"},
		"предопределённое из одного": {compare("{left: {kind: field, data: Вид}, comparison: equal, right: [{kind: predefined, data: Перечисление}]}"),
			"filter[0].right[0].data must name the value as kind, object and item"},
		"предопределённое из четырёх": {compare("{left: {kind: field, data: Вид}, comparison: equal, right: [{kind: predefined, data: Перечисление.Виды.Опт.Лишнее}]}"),
			"filter[0].right[0].data must name the value as kind, object and item"},
		"два значения не списку": {compare("{left: {kind: field, data: Вид}, comparison: equal, right: [{kind: boolean, data: \"true\"}, {kind: boolean, data: \"false\"}]}"),
			"filter[0].right holds one value: only a comparison with a list takes several"},
		"второе значение списка": {compare("{left: {kind: field, data: Вид}, comparison: not-in-list-by-hierarchy, right: [{kind: boolean, data: \"true\"}, {kind: number, data: x}]}"),
			"filter[0].right[1].data must be a decimal"},
		"правое поле не путь": {compare("{left: {kind: field, data: Сумма}, comparison: equal, right: [{kind: field, data: \"Объект..Сумма\"}]}"),
			"filter[0].right[0].data must contain only valid identifier segments"},
		"правое булево пустое": {compare("{left: {kind: field, data: Сумма}, comparison: equal, right: [{kind: boolean}]}"),
			"filter[0].right[0].data must be true or false"},
		"правое поле с текстом": {compare("{left: {kind: field, data: Сумма}, comparison: equal, right: [{kind: field, text: {ru: Сумма}}]}"),
			"filter[0].right[0].data must contain only valid identifier segments"},
		"тип не имя": {compare("{left: {kind: field, data: Тип}, comparison: equal, right: [{kind: type, data: \"d8p1:Undefined\"}]}"),
			"filter[0].right[0].data must be the name of a type"},
		"вид счёта": {compare("{left: {kind: field, data: Вид}, comparison: equal, right: [{kind: account-type, data: neutral}]}"),
			"filter[0].right[0].data must be one of"},
		"режим отображения": {compare("{left: {kind: field, data: Вид}, comparison: filled, view_mode: hidden}"), "filter[0].view_mode must be one of"},
		"нулевой идентификатор настройки": {item("{" + set + ", user_setting_id: 00000000-0000-0000-0000-000000000000}"),
			"conditional_appearance[0].user_setting_id must be a non-zero UUID"},
		"представление настройки поле": {item("{appearance: [{parameter: visible, value: {kind: boolean, data: \"false\"}, user_setting_presentation: {kind: field, data: Сумма}}]}"),
			"conditional_appearance[0].appearance[0].user_setting_presentation.kind must be one of string, localized-string"},
		"вариант даты": {compare("{left: {kind: field, data: Дата}, comparison: less, right: [{kind: standard-beginning-date, data: yesterday}]}"),
			"filter[0].right[0].data must be one of"},
		"произвольная без даты": {compare("{left: {kind: field, data: Дата}, comparison: less, right: [{kind: standard-beginning-date, data: custom}]}"),
			"filter[0].right[0].date must be a date"},
		"дата у стандартной": {compare("{left: {kind: field, data: Дата}, comparison: less, right: [{kind: standard-beginning-date, data: beginning-of-this-day, date: \"2015-01-01T00:00:00\"}]}"),
			"filter[0].right[0].date does not belong to a value of kind standard-beginning-date"},
		"список с данными": {compare("{left: {kind: field, data: Вид}, comparison: in-list, right: [{kind: value-list, data: \"1\"}]}"),
			"filter[0].right[0].data does not belong to a value of kind value-list"},
		"цвет в отборе": {compare("{left: {kind: field, data: Цвет}, comparison: equal, right: [{kind: color, color: {source: auto}}]}"),
			"filter[0].right[0].kind must be one of field, boolean"},
		"сравнение без вида": {compare("{left: {kind: field, data: Сумма}, right: [{kind: number, data: \"1\"}]}"), "filter[0].comparison must be one of"},
		"сравнение незнакомое": {compare("{left: {kind: field, data: Сумма}, comparison: between, right: [{kind: number, data: \"1\"}]}"),
			"filter[0].comparison must be one of"},
		"сравнение без левого": {compare("{comparison: filled}"), "filter[0].left must say what is compared"},
		"сравнение с элементами": {compare("{left: {kind: field, data: Сумма}, comparison: filled, items: [{left: {kind: field, data: Сумма}, comparison: filled}]}"),
			"filter[0].items belong to a group"},
		"группа сравнивает": {compare("{group: and, left: {kind: field, data: Сумма}, items: [{left: {kind: field, data: Сумма}, comparison: filled}]}"),
			"filter[0] is a group and compares nothing itself"},
		"группа незнакомая": {compare("{group: xor, items: [{left: {kind: field, data: Сумма}, comparison: filled}]}"), "filter[0].group must be one of"},
		"глубоко в группе": {compare("{group: and, items: [{group: or, items: [{left: {kind: field, data: Сумма}, comparison: filled, right: [{kind: number, data: x}]}]}]}"),
			"filter[0].items[0].items[0].right[0].data must be a decimal"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeManagedForm("form.yaml", strings.NewReader(formAttrHead+test.body), configuration)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err = %v, want %q", err, test.want)
			}
		})
	}
}

// What the conditional appearance of a form names is resolved at load, in
// a form of an object and in a common form alike: a field naming no element
// of the form - looked for among the elements an element holds, its extended
// tooltip among them, and whatever the case of the name - is a remnant; a
// style item that is gone is a reference to nothing, and one of another type
// is refused.
//
// Defect caught: a field of a deleted element loading clean, though it draws
// nothing; a field naming an extended tooltip, or written in another case,
// taken for a remnant; the appearance of a common form never resolved; a
// colour taken from a style item that is gone loading clean, or one taken
// from a font accepted.
func TestWhatAConditionalAppearanceNamesIsResolved(t *testing.T) {
	t.Parallel()
	const font, color = "c0de0000-0000-4000-8000-000000990031", "c0de0000-0000-4000-8000-000000990032"
	items := "items:\n  - {id: c0de0000-0000-4000-8000-000000990001, name: Наименование, kind: input-field, data_path: Объект.Наименование," +
		" extended_tooltip: {id: c0de0000-0000-4000-8000-000000990002, name: НаименованиеРасширеннаяПодсказка, kind: label-decoration}}\n"
	visible := "{parameter: visible, value: {kind: boolean, data: \"false\"}}"
	for name, test := range map[string]struct {
		appearance string
		common     bool
		gone       string
		written    string
		refused    string
	}{
		"всё на месте": {appearance: "  - {fields: [{field: Наименование}, {field: наименованиерасширеннаяподсказка}], appearance: [" + visible +
			", {parameter: text-color, value: {kind: color, color: {source: style, from: {item: " + color + "}}}}, {parameter: font, value: {kind: font, font: {source: style, from: {item: " + font + "}}}}]}\n"},
		"поля нет": {appearance: "  - {fields: [{field: Наименование}], appearance: [" + visible + "]}\n  - {fields: [{field: Удалено}], appearance: [" + visible + "]}\n",
			gone: "catalog Номенклатура form ФормаЭлемента conditional appearance[1] field", written: "Удалено"},
		"поля нет в общей форме": {common: true, appearance: "  - {fields: [{field: Удалено}], appearance: [" + visible + "]}\n",
			gone: "common form АдреснаяКнига conditional appearance[0] field", written: "Удалено"},
		"элемента стиля нет": {appearance: "  - {appearance: [{parameter: back-color, value: {kind: color, color: {source: style, from: {item: " + refGone + "}}}}]}\n",
			gone: "catalog Номенклатура form ФормаЭлемента conditional appearance [0].appearance[0].value.color"},
		"шрифт из цвета": {appearance: "  - {appearance: [{parameter: font, value: {kind: font, font: {source: style, from: {item: " + color + "}}}}]}\n",
			refused: "conditional appearance [0].appearance[0].value.font takes its value from style item Фон, which is a color and not a font"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := formReferencesProject(t)
			writeMetadata(t, root, StyleItemKind, font, "format: 1\nid: "+font+"\nname: Мелкий\ntitle: {ru: Мелкий}\ntype: font\nvalue: {font: {source: auto}}\n")
			writeMetadata(t, root, StyleItemKind, color, "format: 1\nid: "+color+"\nname: Фон\ntitle: {ru: Фон}\ntype: color\nvalue: {color: {source: auto}}\n")
			path := filepath.Join(root, "metadata", string(CatalogKind), "Номенклатура", "forms", "ФормаЭлемента", project.FormMetadataFile)
			if test.common {
				path = filepath.Join(root, "metadata", "common-forms", "АдреснаяКнига", project.FormMetadataFile)
			}
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, path, string(content)+items+"conditional_appearance:\n"+test.appearance)
			if test.gone != "" {
				found := unresolvedOf(t, root)
				if len(found) != 1 || found[0].Where != test.gone || found[0].Written != test.written {
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
			if unresolved := catalog.UnresolvedReferences(); len(unresolved) != 0 {
				t.Fatalf("unresolved = %+v", unresolved)
			}
		})
	}
}

// A font taken from a style item the prototype wrote as a number (ref="0")
// is carried in a form - on a field and in its conditional appearance - and
// noted, pointing at nothing; an absolute font with no face, which the
// prototype writes on fields, loads clean and keeps its size.
//
// Defect caught: the 20 fonts written as "0" refused, so that their four
// forms are not moved, or carried with no note, or taken for a reference
// to a style item that is gone; the 10 absolute fonts with an empty face
// refused, though the help leaves the face of a font made of its
// description to the style.
func TestAFontWrittenAsANumberIsNotedInAForm(t *testing.T) {
	t.Parallel()
	root := formReferencesProject(t)
	path := filepath.Join(root, "metadata", string(CatalogKind), "Номенклатура", "forms", "ФормаЭлемента", project.FormMetadataFile)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, string(content)+"items:\n"+
		"  - {id: c0de0000-0000-4000-8000-000000990001, name: Код, kind: input-field, data_path: Объект.Код, font: {source: style, from: {written: \"0\"}}}\n"+
		"  - {id: c0de0000-0000-4000-8000-000000990002, name: Наименование, kind: input-field, data_path: Объект.Наименование, font: {source: absolute, size: 12, scale: 100}}\n"+
		"conditional_appearance:\n"+
		"  - {fields: [{field: Код}], appearance: [{parameter: font, value: {kind: font, font: {source: style, from: {written: \"0\"}}}}]}\n")
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if unresolved := catalog.UnresolvedReferences(); len(unresolved) != 0 {
		t.Fatalf("unresolved = %+v", unresolved)
	}
	var found []string
	for _, note := range catalog.Notes() {
		if note.Kind == NoteFormReferenceAsWritten {
			found = append(found, note.Where+" = "+note.Written)
		}
	}
	want := []string{"catalog Номенклатура form ФормаЭлемента conditional appearance [0].appearance[0].value.font = 0",
		"catalog Номенклатура form ФормаЭлемента element Код font = 0"}
	sort.Strings(found)
	if !reflect.DeepEqual(found, want) {
		t.Fatalf("notes = %q, want %q", found, want)
	}
	form, err := DecodeManagedForm("form.yaml", strings.NewReader(formAttrHead+"items:\n"+
		"  - {id: c0de0000-0000-4000-8000-000000990002, name: Наименование, kind: input-field, font: {source: absolute, size: 12, scale: 100}}\n"), managedFormConfiguration())
	if err != nil || form.Items[0].Font.Face != "" || form.Items[0].Font.Size != 12 {
		t.Fatalf("absolute font with no face: %v %+v", err, form.Items)
	}
}
