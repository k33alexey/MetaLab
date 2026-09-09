package studio

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestNavigateBSLFindsPublicDefinitionAndUsages(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	publicID, callerID := uuid.MustNew(), uuid.MustNew()
	publicPath, _ := project.ModulePath(publicID)
	callerPath, _ := project.ModulePath(callerID)
	publicSource := "Функция ПолучитьДанные(Ключ) Экспорт\n\tВозврат Ключ;\nКонецФункции\n"
	callerSource := "Процедура Запустить()\n\tПервый = Обмен.ПолучитьДанные(1);\n\tВторой = Обмен.ПолучитьДанные(2);\nКонецПроцедуры\n"
	writeBSLTestSource(t, root, publicPath, publicSource)
	writeBSLTestSource(t, root, callerPath, callerSource)
	commonPath, _ := project.MetadataPath("common-modules", uuid.MustNew())
	writeBSLTestSource(t, root, commonPath, "format: 1\nname: Обмен\nmodule: "+publicID.String()+"\n")
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}

	offset := strings.Index(callerSource, "ПолучитьДанные") + len("Получить")
	definition := navigateAt(t, workspace, callerPath, callerSource, offset, "definition")
	if definition.Symbol.Name != "ПолучитьДанные" || definition.Symbol.Kind != "function" || !definition.Symbol.CanRename ||
		len(definition.Locations) != 1 || definition.Locations[0].Path != publicPath || definition.Locations[0].Range.Start.Line != 1 {
		t.Fatalf("definition = %+v", definition)
	}
	usages := navigateAt(t, workspace, callerPath, callerSource, offset, "usages")
	if len(usages.Locations) != 2 || usages.Locations[0].Path != callerPath || usages.Locations[1].Path != callerPath {
		t.Fatalf("usages = %+v", usages)
	}

	parameterOffset := strings.LastIndex(publicSource, "Ключ") + len("Кл")
	parameter := navigateAt(t, workspace, publicPath, publicSource, parameterOffset, "definition")
	if parameter.Symbol.Kind != "parameter" || parameter.Symbol.Definition.Range.Start.Line != 1 {
		t.Fatalf("parameter definition = %+v", parameter)
	}
}

func TestNavigateBSLFindsMetadataDefinition(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	moduleID, catalogID := uuid.MustNew(), uuid.MustNew()
	modulePath, _ := project.ModulePath(moduleID)
	catalogPath, _ := project.MetadataPath("catalogs", catalogID)
	source := "Процедура Запустить()\n\tЭлемент = Справочники.Товары.СоздатьЭлемент();\nКонецПроцедуры\n"
	writeBSLTestSource(t, root, modulePath, source)
	writeBSLTestSource(t, root, catalogPath, "format: 1\nid: "+catalogID.String()+"\nname: Товары\ntitle: {ru: Товары}\ncode: {type: string, length: 9, auto: true, unique: true}\ndescription_length: 100\n")
	workspace, _ := Open(root)
	offset := strings.Index(source, "Товары") + len("Тов")
	result := navigateAt(t, workspace, modulePath, source, offset, "definition")
	if result.Symbol.Kind != "metadata" || result.Symbol.CanRename || result.Symbol.Definition.Path != catalogPath || result.Symbol.Definition.Range.Start.Line != 3 {
		t.Fatalf("metadata definition = %+v", result)
	}
}

func TestNavigateBSLDistinguishesRoutineCallFromSameNamedLocal(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	path, _ := project.ModulePath(uuid.MustNew())
	source := "Функция Рассчитать()\n\tВозврат 1;\nКонецФункции\n\nПроцедура Запустить()\n\tРассчитать = 2;\n\tРезультат = Рассчитать();\n\tРезультат = Рассчитать;\nКонецПроцедуры\n"
	writeBSLTestSource(t, root, path, source)
	workspace, _ := Open(root)
	callOffset := strings.Index(source, "Рассчитать();") + len("Рассч")
	call := navigateAt(t, workspace, path, source, callOffset, "definition")
	if call.Symbol.Kind != "function" || call.Symbol.Definition.Range.Start.Line != 1 {
		t.Fatalf("call definition = %+v", call)
	}
	valueOffset := strings.LastIndex(source, "Рассчитать;") + len("Рассч")
	value := navigateAt(t, workspace, path, source, valueOffset, "definition")
	if value.Symbol.Kind != "variable" || value.Symbol.Definition.Range.Start.Line != 6 {
		t.Fatalf("local definition = %+v", value)
	}
	callUsages := navigateAt(t, workspace, path, source, callOffset, "usages")
	if len(callUsages.Locations) != 1 || callUsages.Locations[0].Range.Start.Line != 7 {
		t.Fatalf("call usages = %+v", callUsages)
	}
	valueUsages := navigateAt(t, workspace, path, source, valueOffset, "usages")
	if len(valueUsages.Locations) != 1 || valueUsages.Locations[0].Range.Start.Line != 8 {
		t.Fatalf("value usages = %+v", valueUsages)
	}
}

func TestNavigateBSLLocalUsagesStayInsideRoutine(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	path, _ := project.ModulePath(uuid.MustNew())
	source := "Процедура Первая()\n\tЛокальная = 1;\n\tСообщить(Локальная);\nКонецПроцедуры\n\nПроцедура Вторая()\n\tЛокальная = 2;\n\tСообщить(Локальная);\nКонецПроцедуры\n"
	writeBSLTestSource(t, root, path, source)
	workspace, _ := Open(root)
	offset := strings.Index(source, "Сообщить(Локальная)") + len("Сообщить(Лок")
	result := navigateAt(t, workspace, path, source, offset, "usages")
	if len(result.Locations) != 1 || result.Locations[0].Range.Start.Line != 3 {
		t.Fatalf("local usages = %+v", result)
	}
}

func TestRenameBSLUpdatesOnlySemanticUsages(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	publicID, callerID := uuid.MustNew(), uuid.MustNew()
	publicPath, _ := project.ModulePath(publicID)
	callerPath, _ := project.ModulePath(callerID)
	publicSource := "Функция ПолучитьДанные(Ключ) Экспорт\n\tВозврат Ключ;\nКонецФункции\n"
	callerSource := "Процедура Запустить()\n\t// Обмен.ПолучитьДанные не изменяется\n\tТекст = \"Обмен.ПолучитьДанные\";\n\tРезультат = Обмен.ПолучитьДанные(1);\nКонецПроцедуры\n"
	writeBSLTestSource(t, root, publicPath, publicSource)
	writeBSLTestSource(t, root, callerPath, callerSource)
	commonPath, _ := project.MetadataPath("common-modules", uuid.MustNew())
	writeBSLTestSource(t, root, commonPath, "format: 1\nname: Обмен\nmodule: "+publicID.String()+"\n")
	workspace, _ := Open(root)
	callerFile, _ := workspace.ReadSource(callerPath)
	position := bslPosition(callerSource, strings.LastIndex(callerSource, "ПолучитьДанные")+len("Получить"))
	result, err := workspace.RenameBSL(callerPath, position, "ЗагрузитьДанные", callerFile.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Changed) != 2 || result.Current.Path != callerPath {
		t.Fatalf("rename result = %+v", result)
	}
	updatedPublic, _ := workspace.ReadSource(publicPath)
	updatedCaller, _ := workspace.ReadSource(callerPath)
	if !strings.Contains(updatedPublic.Content, "Функция ЗагрузитьДанные") || !strings.Contains(updatedCaller.Content, "Обмен.ЗагрузитьДанные(1)") {
		t.Fatalf("public=%q caller=%q", updatedPublic.Content, updatedCaller.Content)
	}
	if !strings.Contains(updatedCaller.Content, "// Обмен.ПолучитьДанные") || !strings.Contains(updatedCaller.Content, "\"Обмен.ПолучитьДанные\"") {
		t.Fatalf("rename changed comments or strings: %q", updatedCaller.Content)
	}
	newOffset := strings.LastIndex(updatedCaller.Content, "ЗагрузитьДанные") + len("Загрузить")
	navigation := navigateAt(t, workspace, callerPath, updatedCaller.Content, newOffset, "definition")
	if navigation.Symbol.Name != "ЗагрузитьДанные" {
		t.Fatalf("updated navigation = %+v", navigation)
	}
}

func TestRenameBSLRejectsConflictKeywordAndStaleRevision(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	id := uuid.MustNew()
	path, _ := project.ModulePath(id)
	source := "Процедура Запустить()\n\tПервый = 1;\n\tВторой = Первый;\nКонецПроцедуры\n"
	writeBSLTestSource(t, root, path, source)
	workspace, _ := Open(root)
	file, _ := workspace.ReadSource(path)
	position := bslPosition(source, strings.LastIndex(source, "Первый")+2)
	if _, err := workspace.RenameBSL(path, position, "Второй", file.Revision); !errors.Is(err, ErrBSLRenameConflict) {
		t.Fatalf("conflict error = %v", err)
	}
	if _, err := workspace.RenameBSL(path, position, "Если", file.Revision); err == nil {
		t.Fatal("rename accepted a keyword")
	}
	if _, err := workspace.RenameBSL(path, position, "НачатьТранзакцию", file.Revision); !errors.Is(err, ErrBSLRenameConflict) {
		t.Fatalf("platform name conflict error = %v", err)
	}
	if _, err := workspace.RenameBSL(path, position, "НовоеИмя", strings.Repeat("0", 64)); !errors.Is(err, ErrSourceChanged) {
		t.Fatalf("stale revision error = %v", err)
	}
	unchanged, _ := workspace.ReadSource(path)
	if unchanged.Content != source {
		t.Fatalf("rejected rename changed source: %q", unchanged.Content)
	}
}

func TestRefactoringWritesRollbackEarlierFilesOnRevisionChange(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	paths := make([]string, 2)
	for index := range paths {
		paths[index], _ = project.ModulePath(uuid.MustNew())
	}
	sort.Strings(paths)
	original := map[string][]byte{
		paths[0]: []byte("Процедура Первая()\nКонецПроцедуры\n"),
		paths[1]: []byte("Процедура Вторая()\nКонецПроцедуры\n"),
	}
	for path, source := range original {
		writeBSLTestSource(t, root, path, string(source))
	}
	workspace, _ := Open(root)
	changes := map[string][]byte{
		paths[0]: []byte("Процедура ИзмененаПервая()\nКонецПроцедуры\n"),
		paths[1]: []byte("Процедура ИзмененаВторая()\nКонецПроцедуры\n"),
	}
	snapshots := map[string][]byte{paths[0]: original[paths[0]], paths[1]: []byte("устаревшая версия")}
	workspace.mu.Lock()
	err := workspace.replaceSourcesLocked(changes, snapshots)
	workspace.mu.Unlock()
	if !errors.Is(err, ErrSourceChanged) {
		t.Fatalf("replaceSourcesLocked() error = %v", err)
	}
	for _, path := range paths {
		content, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if readErr != nil || string(content) != string(original[path]) {
			t.Fatalf("source %s after rollback = %q, %v", path, content, readErr)
		}
	}
}

func TestSearchProjectFindsBSLFormsAndKeepsLineNumbers(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	modulePath, _ := project.ModulePath(uuid.MustNew())
	formID := uuid.MustNew()
	formPath, _ := project.FormPath(formID)
	writeBSLTestSource(t, root, modulePath, "Процедура Запустить()\n\n\tКонтрагент = Неопределено;\nКонецПроцедуры\n")
	formSource := "format: 1\nid: " + formID.String() + "\nname: Форма\ntitle: {ru: Карточка контрагента}\nkind: object\n"
	writeBSLTestSource(t, root, formPath, formSource)
	workspace, _ := Open(root)
	result, err := workspace.SearchProject("контрагент")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Locations) != 2 {
		t.Fatalf("search = %+v", result)
	}
	foundModuleLine := false
	foundForm := false
	for _, location := range result.Locations {
		foundModuleLine = foundModuleLine || location.Path == modulePath && location.Range.Start.Line == 3
		foundForm = foundForm || location.Path == formPath
	}
	if !foundModuleLine || !foundForm {
		t.Fatalf("search locations = %+v", result.Locations)
	}

	form, _ := workspace.ReadSource(formPath)
	if _, err := workspace.SaveSource(formPath, strings.Replace(formSource, "Карточка контрагента", "Карточка поставщика", 1), form.Revision); err != nil {
		t.Fatal(err)
	}
	updated, err := workspace.SearchProject("поставщика")
	if err != nil || len(updated.Locations) != 1 || updated.Locations[0].Path != formPath {
		t.Fatalf("updated search = %+v, %v", updated, err)
	}
}

func navigateAt(t *testing.T, workspace *Workspace, path, source string, offset int, mode string) BSLNavigation {
	t.Helper()
	result, err := workspace.NavigateBSL(path, source, bslPosition(source, offset), mode)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestProjectSearchDoesNotFollowSymlinks(t *testing.T) {
	root := createProject(t)
	target := filepath.Join(t.TempDir(), "outside.bsl")
	if err := os.WriteFile(target, []byte("СекретнаяСтрока"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, "modules", uuid.MustNew().String()+".bsl")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	workspace, _ := Open(root)
	result, err := workspace.SearchProject("СекретнаяСтрока")
	if err != nil || len(result.Locations) != 0 {
		t.Fatalf("symlink search = %+v, %v", result, err)
	}
}

func TestStudioNavigationRenameAndSearchAPI(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	path, _ := project.ModulePath(uuid.MustNew())
	source := "Процедура Запустить()\n\tЗначение = 1;\n\tЗначение = Значение + 1;\nКонецПроцедуры\n"
	writeBSLTestSource(t, root, path, source)
	workspace, _ := Open(root)
	handler := NewHandler(workspace)
	position := bslPosition(source, strings.LastIndex(source, "Значение")+3)

	navigationBody, _ := json.Marshal(map[string]any{"path": path, "content": source, "position": position, "mode": "usages"})
	navigationRequest := httptest.NewRequest(http.MethodPost, "/api/bsl/navigate", bytes.NewReader(navigationBody))
	navigationRequest.Header.Set("Content-Type", "application/json")
	navigationRequest.Header.Set("X-ML-CSRF", "1")
	navigationResponse := httptest.NewRecorder()
	handler.ServeHTTP(navigationResponse, navigationRequest)
	if navigationResponse.Code != http.StatusOK || !strings.Contains(navigationResponse.Body.String(), "Значение") {
		t.Fatalf("navigation status=%d body=%s", navigationResponse.Code, navigationResponse.Body.String())
	}

	file, _ := workspace.ReadSource(path)
	renameBody, _ := json.Marshal(map[string]any{"path": path, "position": position, "newName": "Результат", "expectedRevision": file.Revision})
	renameRequest := httptest.NewRequest(http.MethodPost, "/api/bsl/rename", bytes.NewReader(renameBody))
	renameRequest.Header.Set("Content-Type", "application/json")
	renameRequest.Header.Set("X-ML-CSRF", "1")
	renameResponse := httptest.NewRecorder()
	handler.ServeHTTP(renameResponse, renameRequest)
	if renameResponse.Code != http.StatusOK || !strings.Contains(renameResponse.Body.String(), "Результат") {
		t.Fatalf("rename status=%d body=%s", renameResponse.Code, renameResponse.Body.String())
	}

	searchResponse := httptest.NewRecorder()
	handler.ServeHTTP(searchResponse, httptest.NewRequest(http.MethodGet, "/api/search?query="+url.QueryEscape("Результат"), nil))
	if searchResponse.Code != http.StatusOK || !strings.Contains(searchResponse.Body.String(), path) {
		t.Fatalf("search status=%d body=%s", searchResponse.Code, searchResponse.Body.String())
	}

	denied := httptest.NewRecorder()
	requestWithoutCSRF := httptest.NewRequest(http.MethodPost, "/api/bsl/rename", bytes.NewReader(renameBody))
	requestWithoutCSRF.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(denied, requestWithoutCSRF)
	if denied.Code != http.StatusForbidden {
		t.Fatalf("rename without CSRF status=%d", denied.Code)
	}
}

func BenchmarkNavigateBSL(b *testing.B) {
	root, err := filepath.Abs(filepath.Join("..", "..", "examples", "sales-and-warehouse"))
	if err != nil {
		b.Fatal(err)
	}
	workspace, err := Open(root)
	if err != nil {
		b.Fatal(err)
	}
	path := "modules/10000000-0000-4000-8000-000000000401.bsl"
	file, err := workspace.ReadSource(path)
	if err != nil {
		b.Fatal(err)
	}
	position := bslPosition(file.Content, strings.Index(file.Content, "ОбработкаПроведения")+4)
	if _, err := workspace.NavigateBSL(path, file.Content, position, "definition"); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := workspace.NavigateBSL(path, file.Content, position, "definition"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSearchProject(b *testing.B) {
	root, err := filepath.Abs(filepath.Join("..", "..", "examples", "sales-and-warehouse"))
	if err != nil {
		b.Fatal(err)
	}
	workspace, err := Open(root)
	if err != nil {
		b.Fatal(err)
	}
	if _, err := workspace.SearchProject("Остатки"); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := workspace.SearchProject("Остатки"); err != nil {
			b.Fatal(err)
		}
	}
}
