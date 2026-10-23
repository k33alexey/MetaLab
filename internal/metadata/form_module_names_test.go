package metadata

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// formModuleNamesProject is managedOnlyProject whose report form has the
// attributes Объект, the main one, Список with a column Цена, and Заголовок,
// named as a property of the form is; its common form has Пароль. Each
// form's module is the source given.
func formModuleNamesProject(t *testing.T, objectModule, commonModule string) string {
	t.Helper()
	attribute := func(name, extra string) string {
		return "  - {id: " + uuid.MustNew().String() + ", name: " + name + extra + "}\n"
	}
	root, object, common := managedOnlyProject(t, "")
	writeText(t, object, "", project.FormMetadataFile, "format: 1\nid: "+uuid.MustNew().String()+
		"\nname: Форма\ntitle: {ru: Форма}\nkind: object\nattributes:\n"+attribute("Объект", ", main: true")+
		"  - id: "+uuid.MustNew().String()+"\n    name: Список\n    columns:\n      - {id: "+uuid.MustNew().String()+", name: Цена}\n"+
		attribute("Заголовок", ""))
	writeText(t, object, "", project.FormModuleFile, objectModule)
	writeCommonForm(t, root, "Настройки", "format: 1\nid: "+uuid.MustNew().String()+
		"\nname: Настройки\ntitle: {ru: Настройки}\nkind: common\nattributes:\n"+attribute("Пароль", ""))
	writeText(t, common, "", project.FormModuleFile, commonModule)
	return root
}

// compileFormModules compiles the project's modules as ML Service does: read
// for "Сохранить данные", carried in the snapshot through its JSON, compiled
// from it.
func compileFormModules(t *testing.T, root string) error {
	t.Helper()
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	modules, err := LoadProjectModules(root, catalog)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := json.Marshal(modules)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot RuntimeSnapshot
	if err := json.Unmarshal(stored, &snapshot.Modules); err != nil {
		t.Fatal(err)
	}
	_, _, err = snapshot.CompileModules()
	return err
}

// TestFormModuleNamesItsForm catches the module of a form that does not
// compile because its code names its form's attributes, the form's properties
// or calls the form's methods without the form (help,
// ФормаКлиентскогоПриложения: <Имя реквизита>, its properties and methods),
// as the module of nearly every form moved from the prototype does - Объект
// is used in 719 of the 2125 forms of acc. And a form's module that names the
// attributes of another form, or a column without its table: that is a name
// the module has not; read in a routine that does not assign it, because a
// name assigned to anywhere in a routine is a variable of the routine.
func TestFormModuleNamesItsForm(t *testing.T) {
	t.Parallel()
	objectModule := `&НаКлиенте
Процедура Обновить()
	Если Объект = Неопределено И ЗначениеЗаполнено(Список) Тогда
		Заголовок = "Пусто";
	КонецЕсли;
	ЭтаФорма.Заголовок = Заголовок;
	Модифицированность = Ложь;
	ThisObject.Modified = ReadOnly;
	Если Открыта() Тогда
		Закрыть();
	КонецЕсли;
КонецПроцедуры

Функция ЗначениеЗаполнено(Значение)
	Возврат Значение <> Неопределено;
КонецФункции
`
	commonModule := `&НаКлиенте
Процедура Обновить()
	Пароль = Параметры.Пароль;
	Если Not IsOpen() Тогда
		Close();
	КонецЕсли;
КонецПроцедуры

Функция Пусто()
	Возврат Пароль = "";
КонецФункции
`
	if err := compileFormModules(t, formModuleNamesProject(t, objectModule, commonModule)); err != nil {
		t.Fatalf("the modules of the forms name their forms: %v", err)
	}

	for name, test := range map[string]struct{ objectModule, commonModule, unknown string }{
		"реквизит общей формы в форме отчёта": {"Процедура Обновить()\n\tЗначение = Пароль;\nКонецПроцедуры\n", "", "unknown identifier Пароль"},
		"реквизит формы отчёта в общей форме": {"", "Процедура Обновить()\n\tЗначение = Список;\nКонецПроцедуры\n", "unknown identifier Список"},
		"колонка без таблицы":                 {"Процедура Обновить()\n\tЗначение = Цена;\nКонецПроцедуры\n", "", "unknown identifier Цена"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := compileFormModules(t, formModuleNamesProject(t, test.objectModule, test.commonModule))
			if err == nil || !strings.Contains(err.Error(), test.unknown) {
				t.Fatalf("a name the module has not: compiled or refused for another reason: %v", err)
			}
		})
	}
}
