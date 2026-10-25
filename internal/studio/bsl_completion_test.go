package studio

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestCompleteBSLProvidesLocalSymbolsAndSignature(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	id := uuid.MustNew()
	path, _ := project.ModulePath(id)
	source := `Функция Рассчитать(Количество, Знач Точность = 2)
	Локальная = Количество;
	Возврат Локальная;
КонецФункции

Процедура Запустить()
	Результат = Рассчитать(1, );
КонецПроцедуры
`
	writeBSLTestSource(t, root, path, source)
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}

	completion := completeAt(t, workspace, path, source, strings.Index(source, "Колич")+len("Кол"))
	if !hasCompletion(completion.Items, "Количество", "parameter") {
		t.Fatalf("parameter completion = %+v", completion.Items)
	}
	completion = completeAt(t, workspace, path, source, strings.Index(source, "Локальная;")+len("Лок"))
	if !hasCompletion(completion.Items, "Локальная", "variable") {
		t.Fatalf("local completion = %+v", completion.Items)
	}
	callOffset := strings.Index(source, "Рассчитать(1, )") + len("Рассчитать(1, ")
	completion = completeAt(t, workspace, path, source, callOffset)
	if completion.Signature == nil || completion.Signature.ActiveParameter != 1 ||
		!strings.Contains(completion.Signature.Label, "Знач Точность = 2") {
		t.Fatalf("signature = %+v", completion.Signature)
	}
}

func TestCompleteBSLIndexesPublicModulesAndMetadata(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	publicID, callerID, catalogID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	publicPath, _ := project.ModulePath(publicID)
	callerPath, _ := project.ModulePath(callerID)
	writeBSLTestSource(t, root, publicPath, "Функция ПолучитьДанные(Ключ) Экспорт\nВозврат Ключ;\nКонецФункции\n")
	caller := "Процедура Запуск()\n\tОбмен.Пол\nКонецПроцедуры\n"
	writeBSLTestSource(t, root, callerPath, caller)
	commonID := uuid.MustNew()
	commonPath, _ := project.MetadataPath("common-modules", commonID)
	writeBSLTestSource(t, root, commonPath, "format: 1\nid: "+commonID.String()+"\nname: Обмен\ntitle: {ru: Обмен}\nserver: true\nmodule: "+publicID.String()+"\n")
	catalogPath, _ := project.ObjectMetadataPath("catalogs", "Товары")
	writeBSLTestSource(t, root, catalogPath, `format: 1
id: `+catalogID.String()+`
name: Товары
title: {ru: Товары}
code: {type: string, length: 9, auto: true, unique: true}
description_length: 100
`)

	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	completion := completeAt(t, workspace, callerPath, caller, strings.Index(caller, "Обмен.Пол")+len("Обмен.Пол"))
	if !hasCompletion(completion.Items, "ПолучитьДанные", "function") {
		t.Fatalf("public module completion = %+v", completion.Items)
	}
	publicCall := "Обмен.ПолучитьДанные("
	completion = completeAt(t, workspace, callerPath, publicCall, len(publicCall))
	if completion.Signature == nil || completion.Signature.Label != "Функция ПолучитьДанные(Ключ)" {
		t.Fatalf("public module signature = %+v", completion.Signature)
	}
	metadataSource := "Процедура Запуск()\n\tСправочники.Тов\nКонецПроцедуры\n"
	completion = completeAt(t, workspace, callerPath, metadataSource, strings.Index(metadataSource, "Тов")+len("Тов"))
	if !hasCompletion(completion.Items, "Товары", "metadata-object") {
		t.Fatalf("metadata completion = %+v", completion.Items)
	}
	methodSource := "Процедура Запуск()\n\tСправочники.Товары.Соз\nКонецПроцедуры\n"
	completion = completeAt(t, workspace, callerPath, methodSource, strings.Index(methodSource, ".Соз")+len(".Соз"))
	if !hasCompletion(completion.Items, "СоздатьЭлемент", "method") {
		t.Fatalf("manager completion = %+v", completion.Items)
	}
}

func TestCompleteBSLSuppressesCommentsAndValidatesPosition(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	id := uuid.MustNew()
	path, _ := project.ModulePath(id)
	source := "Процедура Запуск()\n\t// Спра\nКонецПроцедуры\n"
	writeBSLTestSource(t, root, path, source)
	workspace, _ := Open(root)
	completion := completeAt(t, workspace, path, source, strings.Index(source, "Спра")+len("Спра"))
	if len(completion.Items) != 0 || completion.Signature != nil {
		t.Fatalf("comment completion = %+v", completion)
	}
	if _, err := workspace.CompleteBSL(path, source, BSLPosition{Line: 99, Column: 1}); err == nil {
		t.Fatal("CompleteBSL accepted an out-of-range cursor")
	}
}

func TestCompleteBSLProvidesDirectivesConstructorsEnumsAndBuiltinSignatures(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	id := uuid.MustNew()
	path, _ := project.ModulePath(id)
	writeBSLTestSource(t, root, path, "")
	workspace, _ := Open(root)
	for _, test := range []struct {
		source string
		label  string
		kind   string
	}{
		{"&НаС", "НаСервере", "directive"},
		{"Процедура Тест()\nЗначение = Новый Таб\nКонецПроцедуры", "ТаблицаЗначений", "constructor"},
		{"Процедура Тест()\nЗначение = ВидДвиженияНакопления.При\nКонецПроцедуры", "Приход", "enum-value"},
	} {
		completion := completeAt(t, workspace, path, test.source, len(test.source)-len("\nКонецПроцедуры"))
		if test.source[0] == '&' {
			completion = completeAt(t, workspace, path, test.source, len(test.source))
		}
		if !hasCompletion(completion.Items, test.label, test.kind) {
			t.Fatalf("completion for %q = %+v", test.source, completion.Items)
		}
	}

	source := "Процедура Тест()\nРезультат = Запрос(\nКонецПроцедуры"
	completion := completeAt(t, workspace, path, source, strings.Index(source, "(\n")+1)
	if completion.Signature == nil || completion.Signature.Label != "Запрос(Текст = \"\")" {
		t.Fatalf("builtin signature = %+v", completion.Signature)
	}
}

func TestBSLPositionRoundTripSupportsUnicodeAndCRLF(t *testing.T) {
	t.Parallel()
	source := "Строка😀\r\nДругая"
	for _, offset := range []int{0, len("Строка"), len("Строка😀"), len("Строка😀\r\n"), len(source)} {
		position := bslPosition(source, offset)
		actual, err := bslOffset(source, position)
		if err != nil || actual != offset {
			t.Fatalf("offset %d -> %+v -> %d, %v", offset, position, actual, err)
		}
	}
}

func TestCompleteBSLUsesDocumentContextFromDemoProject(t *testing.T) {
	t.Parallel()
	root, err := filepath.Abs(filepath.Join("..", "..", "examples", "sales-and-warehouse"))
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	path := "metadata/documents/ПоступлениеТоваров/" + project.ObjectModuleFile
	for _, test := range []struct {
		source string
		label  string
		kind   string
	}{
		{"Процедура Тест()\nДвижения.Ост\nКонецПроцедуры", "ОстаткиТоваров", "metadata-object"},
		{"Процедура Тест()\nДвижения.ОстаткиТоваров.Доб\nКонецПроцедуры", "Добавить", "method"},
		{"Процедура Тест()\nЭтотОбъект.Тов\nКонецПроцедуры", "Товары", "property"},
		{"Процедура Тест()\nЭтотОбъект.Пол\nКонецПроцедуры", "ПолучитьСсылкуНового", "method"},
		// The English word is the help's: ДокументОбъект.Движения is
		// RegisterRecords.
		{"Процедура Тест()\nRegisterRecords.Ост\nКонецПроцедуры", "ОстаткиТоваров", "metadata-object"},
		{"Процедура Тест()\nThisObject.Regi\nКонецПроцедуры", "RegisterRecords", "property"},
	} {
		offset := len(test.source) - len("\nКонецПроцедуры")
		completion := completeAt(t, workspace, path, test.source, offset)
		if !hasCompletion(completion.Items, test.label, test.kind) {
			t.Fatalf("document completion for %q = %+v", test.source, completion.Items)
		}
	}
	source := "Процедура Тест()\nДвижения.ОстаткиТоваров.Добавить(\nКонецПроцедуры"
	completion := completeAt(t, workspace, path, source, strings.Index(source, "Добавить(")+len("Добавить("))
	if completion.Signature == nil || completion.Signature.Label != "Движения.ОстаткиТоваров.Добавить()" {
		t.Fatalf("movement signature = %+v", completion.Signature)
	}
	source = "Процедура Тест()\nЭтотОбъект.Записать(\nКонецПроцедуры"
	completion = completeAt(t, workspace, path, source, strings.Index(source, "Записать(")+len("Записать("))
	if completion.Signature == nil || !strings.Contains(completion.Signature.Label, "РежимЗаписиДокумента.Запись") {
		t.Fatalf("document object signature = %+v", completion.Signature)
	}
}

// ЭтотОбъект has what the help gives its type and what the model gives the
// object, for every kind and not only the four the completion once listed by
// hand. The expectations are the help's (8.3.27), written here rather than
// read from kinds/objects.yaml. Defects caught: a catalog's standard
// attribute or own field missing; a method the prototype's object does not
// have offered (ПолучитьСсылку, which ДокументОбъект and СправочникОбъект
// lack - only ПолучитьСсылкуНового); Количество of a record set offered as a
// property, as the hand-written list had it, while the help makes it a
// method; Записать of a record set signed with a parameter the help does not
// name (Замещение instead of Замещать).
func TestCompleteBSLKnowsThisObjectByTheHelp(t *testing.T) {
	t.Parallel()
	root := copyDemoProjectForStudioTest(t)
	catalogModule := "metadata/catalogs/Товары/" + project.ObjectModuleFile
	registerModule := "metadata/accumulation-registers/ОстаткиТоваров/МодульНабораЗаписей.bsl"
	writeBSLTestSource(t, root, catalogModule, "")
	writeBSLTestSource(t, root, registerModule, "")
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	members := func(path, source string) []BSLCompletionItem {
		return completeAt(t, workspace, path, source, len(source)).Items
	}
	for _, test := range []struct {
		path, source, label, kind string
		present                   bool
	}{
		{catalogModule, "ЭтотОбъект.Код", "Код", "property", true},
		{catalogModule, "ЭтотОбъект.Арт", "Артикул", "property", true},
		{catalogModule, "ЭтотОбъект.ВерсияД", "ВерсияДанных", "property", true},
		{catalogModule, "ЭтотОбъект.Доп", "ДополнительныеСвойства", "property", true},
		{catalogModule, "ЭтотОбъект.Уров", "Уровень", "method", true},
		{catalogModule, "ЭтотОбъект.ПолучитьСс", "ПолучитьСсылкуНового", "method", true},
		{catalogModule, "ЭтотОбъект.ПолучитьСс", "ПолучитьСсылку", "method", false},
		{catalogModule, "ThisObject.Cod", "Code", "property", true},
		{catalogModule, "ThisObject.Lev", "Level", "method", true},
		{registerModule, "ЭтотОбъект.Кол", "Количество", "method", true},
		{registerModule, "ЭтотОбъект.Кол", "Количество", "property", false},
		{registerModule, "ЭтотОбъект.Отб", "Отбор", "property", true},
		{registerModule, "ЭтотОбъект.Доб", "ДобавитьПриход", "method", true},
		{registerModule, "ЭтотОбъект.Бло", "БлокироватьДляИзменения", "property", true},
		// A record set has the fields of its records, not of its own.
		{registerModule, "ЭтотОбъект.Пери", "Период", "property", false},
	} {
		if got := hasCompletion(members(test.path, test.source), test.label, test.kind); got != test.present {
			t.Errorf("%s: %q offers %s %s = %v, want %v", test.path, test.source, test.kind, test.label, got, test.present)
		}
	}
	document := "metadata/documents/ПоступлениеТоваров/" + project.ObjectModuleFile
	source := "Процедура Тест()\nДвижения.ОстаткиТоваров.Записать(\nКонецПроцедуры"
	completion := completeAt(t, workspace, document, source, strings.Index(source, "Записать(")+len("Записать("))
	if completion.Signature == nil || completion.Signature.Label != "Движения.ОстаткиТоваров.Записать(Замещать = Истина)" {
		t.Fatalf("record set signature = %+v", completion.Signature)
	}
}

func TestStudioHandlerCompletesUnsavedBSL(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	id := uuid.MustNew()
	path, _ := project.ModulePath(id)
	source := "Процедура Запуск()\n\tНачатьТ\nКонецПроцедуры\n"
	writeBSLTestSource(t, root, path, source)
	position := bslPosition(source, strings.Index(source, "НачатьТ")+len("НачатьТ"))
	body, _ := json.Marshal(map[string]any{"path": path, "content": source, "position": position})
	workspace, _ := Open(root)
	request := httptest.NewRequest(http.MethodPost, "/api/bsl/complete", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-ML-CSRF", "1")
	response := httptest.NewRecorder()
	NewHandler(workspace).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var completion BSLCompletion
	if err := json.Unmarshal(response.Body.Bytes(), &completion); err != nil || !hasCompletion(completion.Items, "НачатьТранзакцию", "procedure") {
		t.Fatalf("completion=%+v error=%v", completion, err)
	}
}

func TestCompleteBSLRebuildsIndexAfterStudioSave(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	publicID, callerID := uuid.MustNew(), uuid.MustNew()
	publicPath, _ := project.ModulePath(publicID)
	callerPath, _ := project.ModulePath(callerID)
	oldSource := "Функция СтароеИмя() Экспорт\nКонецФункции\n"
	writeBSLTestSource(t, root, publicPath, oldSource)
	writeBSLTestSource(t, root, callerPath, "")
	commonID := uuid.MustNew()
	commonPath, _ := project.MetadataPath("common-modules", commonID)
	writeBSLTestSource(t, root, commonPath, "format: 1\nid: "+commonID.String()+"\nname: Обмен\ntitle: {ru: Обмен}\nserver: true\nmodule: "+publicID.String()+"\n")
	workspace, _ := Open(root)
	first := "Обмен.Стар"
	if completion := completeAt(t, workspace, callerPath, first, len(first)); !hasCompletion(completion.Items, "СтароеИмя", "function") {
		t.Fatalf("initial completion = %+v", completion.Items)
	}
	file, err := workspace.ReadSource(publicPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.SaveSource(publicPath, "Функция НовоеИмя() Экспорт\nКонецФункции\n", file.Revision); err != nil {
		t.Fatal(err)
	}
	second := "Обмен.Нов"
	completion := completeAt(t, workspace, callerPath, second, len(second))
	if !hasCompletion(completion.Items, "НовоеИмя", "function") || hasCompletion(completion.Items, "СтароеИмя", "function") {
		t.Fatalf("completion after save = %+v", completion.Items)
	}
}

func completeAt(t *testing.T, workspace *Workspace, path, source string, offset int) BSLCompletion {
	t.Helper()
	completion, err := workspace.CompleteBSL(path, source, bslPosition(source, offset))
	if err != nil {
		t.Fatal(err)
	}
	return completion
}

func hasCompletion(items []BSLCompletionItem, label, kind string) bool {
	for _, item := range items {
		if item.Label == label && item.Kind == kind {
			return true
		}
	}
	return false
}

func writeBSLTestSource(t testing.TB, root, relative, content string) {
	t.Helper()
	absolute := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(absolute, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeManagedProject writes a project with an object of each sort a path
// through the managers passes: a catalog with a predefined item, a source of
// data with a function, a table and a cube holding a dimension table, a
// second source with a table of its own, a session parameter.
func writeManagedProject(t *testing.T) string {
	t.Helper()
	root := createProject(t)
	writeProjectFile(t, root, "metadata/catalogs/Товары/object.yaml", "format: 1\nid: f9100000-0000-4000-8000-000000000001\n"+
		"name: Товары\ntitle: {ru: Товары}\ncode: {type: string, length: 9, auto: true, unique: true}\ndescription_length: 100\n"+
		"predefined:\n  - {id: f9100000-0000-4000-8000-000000000002, name: Услуга, code: \"001\"}\n")
	writeProjectFile(t, root, "metadata/catalogs/Склады/object.yaml", "format: 1\nid: f9100000-0000-4000-8000-000000000003\n"+
		"name: Склады\ntitle: {ru: Склады}\ncode: {type: string, length: 9, auto: true, unique: true}\ndescription_length: 100\n")
	const source = "metadata/external-data-sources/Склад"
	writeProjectFile(t, root, source+"/object.yaml", "format: 1\nid: f9100000-0000-4000-8000-000000000011\nname: Склад\ntitle: {ru: Склад}\n"+
		"functions:\n  - {id: f9100000-0000-4000-8000-000000000012, name: Остаток, title: {ru: Остаток}, types: [{kind: string, length: 10}]}\n")
	writeProjectFile(t, root, source+"/tables/Остатки/object.yaml", "format: 1\nid: f9100000-0000-4000-8000-000000000013\n"+
		"name: Остатки\ntitle: {ru: Остатки}\nname_in_data_source: dbo.Remains\ndata_type: non-object\n")
	writeProjectFile(t, root, source+"/cubes/Продажи/object.yaml", "format: 1\nid: f9100000-0000-4000-8000-000000000014\n"+
		"name: Продажи\ntitle: {ru: Продажи}\nname_in_data_source: Sales\n")
	writeProjectFile(t, root, source+"/cubes/Продажи/dimension-tables/Номенклатура/object.yaml", "format: 1\n"+
		"id: f9100000-0000-4000-8000-000000000015\nname: Номенклатура\ntitle: {ru: Номенклатура}\nname_in_data_source: Goods\n")
	writeProjectFile(t, root, "metadata/external-data-sources/Архив/object.yaml", "format: 1\nid: f9100000-0000-4000-8000-000000000016\n"+
		"name: Архив\ntitle: {ru: Архив}\n")
	writeProjectFile(t, root, "metadata/external-data-sources/Архив/tables/Старые/object.yaml", "format: 1\n"+
		"id: f9100000-0000-4000-8000-000000000017\nname: Старые\ntitle: {ru: Старые}\nname_in_data_source: dbo.Old\ndata_type: non-object\n")
	writeProjectFile(t, root, "metadata/session-parameters/f9100000-0000-4000-8000-000000000021.yaml", "format: 1\n"+
		"id: f9100000-0000-4000-8000-000000000021\nname: Валюта\ntitle: {ru: Валюта}\ntypes: [{kind: boolean}]\n")
	return root
}

// A path through the managers of the global context completes at every
// point by what the help gives there and the project has: the collections,
// the objects of one, the members of an object's manager with its predefined
// items and the functions of a source, the objects held by a collection of a
// manager, and the signature of a method of a manager. Catches a kind whose
// manager completes with nothing (all but five did), methods of a manager the
// help does not give (ПолучитьОбъект of a catalog), an object of another
// owner or kind offered in a collection (the dimension table among the
// tables), a path through an object the project lacks completed anyway, a
// collection the global context lacks offered (ОпределяемыеТипы), and the
// signature of a method of a register the project lacks.
func TestCompleteBSLWalksTheManagersOfEveryKind(t *testing.T) {
	t.Parallel()
	root := writeManagedProject(t)
	path, _ := project.ModulePath(uuid.MustNew())
	writeBSLTestSource(t, root, path, "")
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	labels := func(code string) []string {
		source := "Процедура Тест()\n" + code + "\nКонецПроцедуры"
		completion := completeAt(t, workspace, path, source, strings.Index(source, "\nКонецПроцедуры"))
		result := make([]string, 0, len(completion.Items))
		for _, item := range completion.Items {
			result = append(result, item.Label)
		}
		return result
	}
	for _, test := range []struct {
		code         string
		want, absent []string
	}{
		{"ВнешниеИс", []string{"ВнешниеИсточникиДанных"}, nil},
		{"ПараметрыС", []string{"ПараметрыСеанса"}, nil},
		{"Определяемые", nil, []string{"ОпределяемыеТипы"}},
		{"Справочники.", []string{"Склады", "Товары"}, nil},
		{"Справочники.Товары.", []string{"Услуга", "НайтиПоКоду", "СоздатьГруппу", "ПустаяСсылка"}, []string{"ПолучитьОбъект"}},
		{"Справочники.Склады.", []string{"НайтиПоКоду"}, []string{"Услуга"}},
		{"Справочники.Нет.", nil, []string{"НайтиПоКоду"}},
		{"Справочники.Товары.Таблицы.", nil, []string{"Остатки"}},
		{"ВнешниеИсточникиДанных.", []string{"Склад"}, nil},
		{"ВнешниеИсточникиДанных.Склад.", []string{"Таблицы", "Кубы", "Остаток", "НачатьТранзакцию"}, nil},
		{"ВнешниеИсточникиДанных.Склад.Таблицы.", []string{"Остатки"}, []string{"Продажи", "Номенклатура", "Старые"}},
		{"ВнешниеИсточникиДанных.Склад.Таблицы.Остатки.", []string{"СоздатьНаборЗаписей", "НайтиПоПолю"}, []string{"Остаток"}},
		{"ВнешниеИсточникиДанных.Склад.Кубы.Продажи.ТаблицыИзмерений.", []string{"Номенклатура"}, []string{"Остатки"}},
		{"ВнешниеИсточникиДанных.Склад.Кубы.Продажи.ТаблицыИзмерений.Номенклатура.", []string{"ПолучитьСсылку"}, nil},
		{"ВнешниеИсточникиДанных.Склад.Кубы.Нет.ТаблицыИзмерений.", nil, []string{"Номенклатура"}},
		{"ПараметрыСеанса.", []string{"Валюта"}, nil},
		// Таблицы is a collection of a source's manager, not of the global
		// context.
		{"Таблицы.", nil, []string{"Остатки", "Склад.Остатки", "Архив.Старые"}},
		{"Catalogs.Товары.Find", []string{"FindByCode", "FindByDescription"}, []string{"НайтиПоКоду"}},
		{"ExternalDataSources.Склад.Tables.", []string{"Остатки"}, nil},
	} {
		got := labels(test.code)
		for _, label := range test.want {
			if !slices.Contains(got, label) {
				t.Errorf("%s does not offer %s: %v", test.code, label, got)
			}
		}
		for _, label := range test.absent {
			if slices.Contains(got, label) {
				t.Errorf("%s offers %s: %v", test.code, label, got)
			}
		}
	}
	for _, test := range []struct{ code, label string }{
		{"Справочники.Товары.НайтиПоКоду(", "Справочники.Товары.НайтиПоКоду(Код, ПоискПоПолномуКоду = Ложь, Родитель, Владелец)"},
		{"ВнешниеИсточникиДанных.Склад.Таблицы.Остатки.НайтиПоПолю(", "ВнешниеИсточникиДанных.Склад.Таблицы.Остатки.НайтиПоПолю(ИмяПоля, ЗначениеПоля)"},
		{"РегистрыНакопления.Остатки.Обороты(", ""},
	} {
		source := "Процедура Тест()\n" + test.code + "\nКонецПроцедуры"
		completion := completeAt(t, workspace, path, source, strings.Index(source, "\nКонецПроцедуры"))
		got := ""
		if completion.Signature != nil {
			got = completion.Signature.Label
		}
		if got != test.label {
			t.Errorf("%s is signed %q, want %q", test.code, got, test.label)
		}
	}
}

// TestCompleteBSLNamesTheFormInItsModule catches the module of a form whose
// completion does not offer what its code names without the form: the form's
// own attributes and the properties of the form (help,
// ФормаКлиентскогоПриложения) - the editor taken from the one ModuleNamer the
// compiler names them by, and so offering what compiles. A form's module
// offers no attribute of another form.
func TestCompleteBSLNamesTheFormInItsModule(t *testing.T) {
	t.Parallel()
	root := copyDemoProjectForStudioTest(t)
	description := filepath.Join(root, "metadata", "common-forms", "ДемоФормаТовара", project.FormMetadataFile)
	content, err := os.ReadFile(description)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(description, append(content, "attributes:\n  - {id: 10000000-0000-4000-8000-0000000019a1, name: ВыбранныйТовар}\n"...), 0o644); err != nil {
		t.Fatal(err)
	}
	module := "metadata/common-forms/ДемоФормаТовара/" + project.FormModuleFile
	writeBSLTestSource(t, root, module, "")
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ source, label string }{
		{"Процедура Тест()\nВыбранный", "ВыбранныйТовар"},
		{"Процедура Тест()\nМодифиц", "Модифицированность"},
		{"Процедура Тест()\nReadO", "ReadOnly"},
	} {
		if completion := completeAt(t, workspace, module, test.source, len(test.source)); !hasCompletion(completion.Items, test.label, "variable") {
			t.Errorf("%q does not offer %s: %+v", test.source, test.label, completion.Items)
		}
	}
	other := "metadata/catalogs/Товары/" + project.ObjectModuleFile
	writeBSLTestSource(t, root, other, "")
	if completion := completeAt(t, workspace, other, "Процедура Тест()\nВыбранный", len("Процедура Тест()\nВыбранный")); hasCompletion(completion.Items, "ВыбранныйТовар", "variable") {
		t.Errorf("the object module of a catalog offers the attribute of a form")
	}
}
