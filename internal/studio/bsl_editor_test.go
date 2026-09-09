package studio

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestAnalyzeBSLProvidesEditorModel(t *testing.T) {
	t.Parallel()
	source := `#Область Расчеты
&НаСервере
Процедура Рассчитать(Количество) Экспорт
	Если Количество > 0 Тогда
		Значение = (Количество + 1); // пояснение
		Значение = Массив[0];
	КонецЕсли;
КонецПроцедуры
#КонецОбласти
`
	analysis, err := AnalyzeBSL("modules/test.bsl", source)
	if err != nil {
		t.Fatal(err)
	}
	if len(analysis.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v", analysis.Diagnostics)
	}
	if len(analysis.Symbols) != 1 || analysis.Symbols[0].Kind != "procedure" || analysis.Symbols[0].Name != "Рассчитать" || analysis.Symbols[0].Range.Start.Line != 2 {
		t.Fatalf("symbols = %+v", analysis.Symbols)
	}
	for _, kind := range []string{"comment", "keyword", "number", "operator", "directive", "indent"} {
		if !hasHighlight(analysis.Highlights, kind) {
			t.Fatalf("highlights do not contain %q: %+v", kind, analysis.Highlights)
		}
	}
	for _, kind := range []string{"region", "procedure", "if", "parentheses", "brackets"} {
		if !hasPair(analysis.Pairs, kind) {
			t.Fatalf("pairs do not contain %q: %+v", kind, analysis.Pairs)
		}
	}
	for _, kind := range []string{"region", "procedure", "if"} {
		if !hasFold(analysis.Folds, kind) {
			t.Fatalf("folds do not contain %q: %+v", kind, analysis.Folds)
		}
	}
	if analysis.Truncated {
		t.Fatal("small analysis was truncated")
	}
}

func TestAnalyzeBSLReturnsSyntaxDiagnostics(t *testing.T) {
	t.Parallel()
	analysis, err := AnalyzeBSL("modules/test.bsl", "Процедура Ошибка(\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(analysis.Diagnostics) == 0 || analysis.Diagnostics[0].Code == "" || analysis.Diagnostics[0].Range.Start.Line < 1 {
		t.Fatalf("diagnostics = %+v", analysis.Diagnostics)
	}
	if _, err := AnalyzeBSL("forms/test.yaml", ""); err == nil {
		t.Fatal("AnalyzeBSL accepted a non-BSL path")
	}
}

func TestStudioHandlerAnalyzesUnsavedBSL(t *testing.T) {
	t.Parallel()
	root := createProject(t)
	id := uuid.MustNew()
	modulePath, err := project.ModulePath(id)
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(map[string]string{
		"path": modulePath, "content": "Функция Ответ()\nВозврат 42;\nКонецФункции",
	})
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(workspace)
	request := httptest.NewRequest(http.MethodPost, "/api/bsl/analyze", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-ML-CSRF", "1")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var analysis BSLAnalysis
	if err := json.Unmarshal(response.Body.Bytes(), &analysis); err != nil {
		t.Fatal(err)
	}
	if len(analysis.Symbols) != 1 || analysis.Symbols[0].Name != "Ответ" {
		t.Fatalf("analysis = %+v", analysis)
	}

	request = httptest.NewRequest(http.MethodPost, "/api/bsl/analyze", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("request without CSRF status = %d", response.Code)
	}
}

func TestAnalyzeBSLRejectsOversizedSource(t *testing.T) {
	t.Parallel()
	path := filepath.ToSlash(filepath.Join("modules", "test.bsl"))
	source := string(make([]byte, MaxEditableFileBytes+1))
	if _, err := AnalyzeBSL(path, source); err == nil {
		t.Fatal("AnalyzeBSL accepted oversized source")
	}
}

func BenchmarkAnalyzeBSL(b *testing.B) {
	source := strings.Repeat("Функция Рассчитать(Значение)\nЕсли Значение > 0 Тогда\nВозврат Значение + 1;\nКонецЕсли;\nКонецФункции\n", 300)
	b.ReportAllocs()
	b.SetBytes(int64(len(source)))
	for range b.N {
		analysis, err := AnalyzeBSL("modules/benchmark.bsl", source)
		if err != nil || len(analysis.Diagnostics) != 0 {
			b.Fatalf("analysis error=%v diagnostics=%d", err, len(analysis.Diagnostics))
		}
	}
}

func hasHighlight(items []BSLHighlight, kind string) bool {
	for _, item := range items {
		if item.Kind == kind {
			return true
		}
	}
	return false
}

func hasPair(items []BSLPair, kind string) bool {
	for _, item := range items {
		if item.Kind == kind {
			return true
		}
	}
	return false
}

func hasFold(items []BSLFold, kind string) bool {
	for _, item := range items {
		if item.Kind == kind {
			return true
		}
	}
	return false
}
