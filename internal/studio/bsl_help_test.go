package studio

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/bsl/spec"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestSearchBSLHelpUsesLocalPlatformCatalog(t *testing.T) {
	t.Parallel()
	workspace, _ := Open(createProject(t))
	for _, query := range []string{"НачатьТранзакцию", "BeginTransaction", "временных таблиц", "movement kind"} {
		result, err := workspace.SearchBSLHelp(query)
		if err != nil {
			t.Fatalf("SearchBSLHelp(%q): %v", query, err)
		}
		if len(result.Items) == 0 {
			t.Fatalf("SearchBSLHelp(%q) returned no items", query)
		}
		if result.Items[0].Source != "Платформа ML" {
			t.Fatalf("SearchBSLHelp(%q) source = %q", query, result.Items[0].Source)
		}
	}
	if _, err := workspace.SearchBSLHelp(" "); err == nil {
		t.Fatal("SearchBSLHelp accepted an empty query")
	}
	if _, err := workspace.SearchBSLHelp(strings.Repeat("я", maxBSLHelpQueryRunes+1)); err == nil {
		t.Fatal("SearchBSLHelp accepted an oversized query")
	}
}

func TestPlatformHelpCoversCompletionCatalog(t *testing.T) {
	t.Parallel()
	workspace, _ := Open(createProject(t))
	workspace.mu.Lock()
	index, err := workspace.ensureBSLHelpIndexLocked()
	workspace.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	for alias := range standaloneBSLSignatures {
		if _, ok := index.firstAlias(alias); !ok {
			t.Errorf("standalone completion %q has no local help", alias)
		}
	}
	for alias := range builtinBSLSignatures {
		if _, ok := index.firstAlias(alias); !ok {
			t.Errorf("built-in completion %q has no local help", alias)
		}
	}
	catalog, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, keyword := range catalog.Keywords {
		for _, alias := range []string{keyword.Russian, keyword.English} {
			if _, ok := index.firstAlias(alias); !ok {
				t.Errorf("keyword %q has no local help", alias)
			}
		}
	}
	for _, alias := range []string{"AtClient", "НаСервере"} {
		if _, ok := index.firstAlias(alias); !ok {
			t.Errorf("directive %q has no local help", alias)
		}
	}
}

func TestResolveBSLHelpFindsKeywordGlobalConstructorAndSystemEnum(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	path, _ := project.ModulePath(uuid.MustNew())
	source := "Процедура Запустить()\n\tЕсли ТранзакцияАктивна() Тогда\n\t\tЗначения = Новый Массив;\n\t\tВид = ВидДвиженияНакопления.Приход;\n\tКонецЕсли;\nКонецПроцедуры\n"
	writeBSLTestSource(t, root, path, source)
	workspace, _ := Open(root)
	for _, test := range []struct {
		needle string
		name   string
		kind   string
	}{
		{"Если", "Если", "keyword"},
		{"ТранзакцияАктивна", "ТранзакцияАктивна", "function"},
		{"Массив", "Массив", "constructor"},
		{"Приход", "ВидДвиженияНакопления", "system-enum"},
	} {
		offset := strings.Index(source, test.needle) + len([]byte(test.needle))/2
		item, err := workspace.ResolveBSLHelp(path, source, bslPosition(source, offset))
		if err != nil {
			t.Fatalf("ResolveBSLHelp(%q): %v", test.needle, err)
		}
		if item.Name != test.name || item.Kind != test.kind || item.Summary == "" {
			t.Fatalf("ResolveBSLHelp(%q) = %+v", test.needle, item)
		}
	}
	commentOffset := strings.Index(source, "Процедура")
	commentSource := "// ТранзакцияАктивна\n" + source
	if _, err := workspace.ResolveBSLHelp(path, commentSource, bslPosition(commentSource, commentOffset+3)); !errors.Is(err, ErrBSLHelpNotFound) {
		t.Fatalf("help inside comment error = %v", err)
	}
}

func TestResolveBSLHelpDoesNotReplaceLocalSymbolWithPlatformTopic(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	path, _ := project.ModulePath(uuid.MustNew())
	source := "Процедура Запустить()\n\tМассив = 1;\nКонецПроцедуры\n"
	writeBSLTestSource(t, root, path, source)
	workspace, _ := Open(root)
	offset := strings.Index(source, "Массив") + 2
	if _, err := workspace.ResolveBSLHelp(path, source, bslPosition(source, offset)); !errors.Is(err, ErrBSLHelpNotFound) {
		t.Fatalf("local variable resolved as platform help: %v", err)
	}
}

func TestBSLHelpDocumentsProjectRoutineAndStructuredComment(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	publicID, callerID := uuid.MustNew(), uuid.MustNew()
	publicPath, _ := project.ModulePath(publicID)
	callerPath, _ := project.ModulePath(callerID)
	publicSource := `// Возвращает цену товара по коду.
//
// Параметры:
// Код - Код искомого товара.
// Момент - Дата получения цены.
// Возвращаемое значение:
// Число или Неопределено.
&НаСервере
Функция ПолучитьЦену(Знач Код, Момент = Неопределено) Экспорт
	Возврат Неопределено;
КонецФункции
`
	callerSource := "Процедура Запустить()\n\tЦена = Цены.ПолучитьЦену(\"001\");\nКонецПроцедуры\n"
	writeBSLTestSource(t, root, publicPath, publicSource)
	writeBSLTestSource(t, root, callerPath, callerSource)
	commonPath, _ := project.MetadataPath("common-modules", uuid.MustNew())
	writeBSLTestSource(t, root, commonPath, "format: 1\nname: Цены\nmodule: "+publicID.String()+"\n")
	workspace, _ := Open(root)

	offset := strings.Index(callerSource, "ПолучитьЦену") + len("Получить")
	item, err := workspace.ResolveBSLHelp(callerPath, callerSource, bslPosition(callerSource, offset))
	if err != nil {
		t.Fatal(err)
	}
	if item.Name != "ПолучитьЦену" || item.Kind != "function" || item.Source != "ML Project · Цены" || item.Location == nil || item.Location.Path != publicPath {
		t.Fatalf("project help = %+v", item)
	}
	if item.Summary != "Возвращает цену товара по коду." || item.Returns != "Число или Неопределено." || len(item.Parameters) != 2 ||
		item.Parameters[0].Description != "Код искомого товара." || item.Parameters[1].Description != "Дата получения цены." {
		t.Fatalf("structured documentation = %+v", item)
	}
	result, err := workspace.SearchBSLHelp("искомого товара")
	if err != nil || !hasHelpItem(result.Items, "ПолучитьЦену", "ML Project · Цены") {
		t.Fatalf("project documentation search = %+v, %v", result, err)
	}
}

func TestResolveBSLHelpUsesUnsavedProjectDocumentation(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	path, _ := project.ModulePath(uuid.MustNew())
	saved := "// Сохранённое описание.\nФункция Рассчитать()\n\tВозврат 1;\nКонецФункции\n"
	writeBSLTestSource(t, root, path, saved)
	workspace, _ := Open(root)
	if _, err := workspace.SearchBSLHelp("Сохранённое"); err != nil {
		t.Fatal(err)
	}
	unsaved := "// Новое несохранённое описание.\nФункция Рассчитать()\n\tВозврат 2;\nКонецФункции\n"
	offset := strings.Index(unsaved, "Рассчитать") + 3
	item, err := workspace.ResolveBSLHelp(path, unsaved, bslPosition(unsaved, offset))
	if err != nil || item.Summary != "Новое несохранённое описание." {
		t.Fatalf("unsaved project help = %+v, %v", item, err)
	}
}

func TestBSLHelpParsesEnglishDocumentation(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	path, _ := project.ModulePath(uuid.MustNew())
	source := "// Calculates a value.\n// Parameters:\n// Value - Source value.\n// Returns:\n// Number.\nFunction Calculate(Val Value)\n\tReturn Value;\nEndFunction\n"
	writeBSLTestSource(t, root, path, source)
	workspace, _ := Open(root)
	offset := strings.LastIndex(source, "Calculate") + 2
	item, err := workspace.ResolveBSLHelp(path, source, bslPosition(source, offset))
	if err != nil || item.Summary != "Calculates a value." || item.Returns != "Number." || len(item.Parameters) != 1 || item.Parameters[0].Description != "Source value." {
		t.Fatalf("English project help = %+v, %v", item, err)
	}
}

func TestBSLHelpIndexInvalidatesAfterSave(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	path, _ := project.ModulePath(uuid.MustNew())
	oldSource := "// Старый термин.\nПроцедура Обработать()\nКонецПроцедуры\n"
	newSource := "// Новый термин.\nПроцедура Обработать()\nКонецПроцедуры\n"
	writeBSLTestSource(t, root, path, oldSource)
	workspace, _ := Open(root)
	if result, err := workspace.SearchBSLHelp("Старый термин"); err != nil || len(result.Items) != 1 {
		t.Fatalf("initial search = %+v, %v", result, err)
	}
	file, _ := workspace.ReadSource(path)
	if _, err := workspace.SaveSource(path, newSource, file.Revision); err != nil {
		t.Fatal(err)
	}
	if result, err := workspace.SearchBSLHelp("Новый термин"); err != nil || len(result.Items) != 1 || result.Items[0].Name != "Обработать" {
		t.Fatalf("updated search = %+v, %v", result, err)
	}
	if result, err := workspace.SearchBSLHelp("Старый термин"); err != nil || len(result.Items) != 0 {
		t.Fatalf("stale search = %+v, %v", result, err)
	}
}

func TestBSLHelpHTTPAPIAndCSRF(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	path, _ := project.ModulePath(uuid.MustNew())
	source := "Процедура Запустить()\n\tНачатьТранзакцию();\nКонецПроцедуры\n"
	writeBSLTestSource(t, root, path, source)
	workspace, _ := Open(root)
	handler := NewHandler(workspace)

	search := httptest.NewRecorder()
	handler.ServeHTTP(search, httptest.NewRequest(http.MethodGet, "/api/bsl/help?query="+"НачатьТранзакцию", nil))
	if search.Code != http.StatusOK || !strings.Contains(search.Body.String(), "Платформа ML") {
		t.Fatalf("help search response = %d %q", search.Code, search.Body.String())
	}
	body, _ := json.Marshal(map[string]any{"path": path, "content": source, "position": bslPosition(source, strings.Index(source, "НачатьТранзакцию")+2)})
	withoutCSRF := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/bsl/help/resolve", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(withoutCSRF, request)
	if withoutCSRF.Code != http.StatusForbidden {
		t.Fatalf("help without CSRF = %d", withoutCSRF.Code)
	}
	resolved := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/bsl/help/resolve", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-ML-CSRF", "1")
	handler.ServeHTTP(resolved, request)
	if resolved.Code != http.StatusOK || !strings.Contains(resolved.Body.String(), "НачатьТранзакцию") {
		t.Fatalf("help resolve response = %d %q", resolved.Code, resolved.Body.String())
	}
}

func TestBSLHelpSearchIsBounded(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	path, _ := project.ModulePath(uuid.MustNew())
	var source strings.Builder
	for index := 0; index < maxBSLHelpResults+20; index++ {
		source.WriteString("// Общий маркер справки.\nПроцедура Проверка")
		source.WriteString(strconv.Itoa(index))
		source.WriteString("()\nКонецПроцедуры\n")
	}
	writeBSLTestSource(t, root, path, source.String())
	workspace, _ := Open(root)
	result, err := workspace.SearchBSLHelp("Общий маркер справки")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != maxBSLHelpResults || !result.Truncated {
		t.Fatalf("bounded result = %d, truncated=%v", len(result.Items), result.Truncated)
	}
}

func BenchmarkSearchBSLHelp(b *testing.B) {
	root := createProject(b)
	path, _ := project.ModulePath(uuid.MustNew())
	writeBSLTestSource(b, root, path, "// Рассчитывает значение.\nФункция Рассчитать(Значение)\nВозврат Значение;\nКонецФункции\n")
	workspace, _ := Open(root)
	if _, err := workspace.SearchBSLHelp("рассчитать"); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for b.Loop() {
		if _, err := workspace.SearchBSLHelp("рассчитать"); err != nil {
			b.Fatal(err)
		}
	}
}

func hasHelpItem(items []BSLHelpItem, name, source string) bool {
	for _, item := range items {
		if item.Name == name && item.Source == source {
			return true
		}
	}
	return false
}
