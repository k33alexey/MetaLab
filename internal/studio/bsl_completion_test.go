package studio

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
	commonPath, _ := project.MetadataPath("common-modules", uuid.MustNew())
	writeBSLTestSource(t, root, commonPath, "format: 1\nname: Обмен\nmodule: "+publicID.String()+"\n")
	catalogPath, _ := project.MetadataPath("catalogs", catalogID)
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

	source := "Процедура Тест()\nРезультат = РегистрыНакопления.Остатки.Обороты(Начало, Конец, );\nКонецПроцедуры"
	completion := completeAt(t, workspace, path, source, strings.Index(source, ", );")+len(", "))
	if completion.Signature == nil || completion.Signature.ActiveParameter != 2 || !strings.Contains(completion.Signature.Label, "Отбор") {
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
	path := "modules/10000000-0000-4000-8000-000000000401.bsl"
	for _, test := range []struct {
		source string
		label  string
		kind   string
	}{
		{"Процедура Тест()\nДвижения.Ост\nКонецПроцедуры", "ОстаткиТоваров", "metadata-object"},
		{"Процедура Тест()\nДвижения.ОстаткиТоваров.Доб\nКонецПроцедуры", "Добавить", "method"},
		{"Процедура Тест()\nЭтотОбъект.Тов\nКонецПроцедуры", "Товары", "property"},
		{"Процедура Тест()\nЭтотОбъект.Пол\nКонецПроцедуры", "ПолучитьСсылку", "method"},
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
	commonPath, _ := project.MetadataPath("common-modules", uuid.MustNew())
	writeBSLTestSource(t, root, commonPath, "format: 1\nname: Обмен\nmodule: "+publicID.String()+"\n")
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
