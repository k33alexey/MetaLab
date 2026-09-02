package syntax

import (
	"strings"
	"testing"
)

func TestTokenizeIsLosslessAndTracksUnicodePosition(t *testing.T) {
	t.Parallel()

	source := "// комментарий\r\nФункция Сумма(А, Б) Экспорт\r\n Возврат А + Б;\r\nКонецФункции\r\n"
	tokens, diagnostics := Tokenize("sum.bsl", source)
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %v", diagnostics)
	}
	var reconstructed strings.Builder
	for _, item := range tokens {
		reconstructed.WriteString(item.Leading)
		reconstructed.WriteString(item.Lexeme)
	}
	if reconstructed.String() != source {
		t.Fatalf("lossless source = %q, want %q", reconstructed.String(), source)
	}
	if tokens[0].Kind != Function || tokens[0].Span.Start.Line != 2 || tokens[0].Span.Start.Column != 1 {
		t.Fatalf("first token = %+v", tokens[0])
	}
}

func TestParseRussianAndEnglishHaveEquivalentShape(t *testing.T) {
	t.Parallel()

	russian := "Функция Сумма(А, Б) Экспорт\nВозврат (А + Б) * 2;\nКонецФункции"
	english := "Function Sum(A, B) Export\nReturn (A + B) * 2;\nEndFunction"
	for name, source := range map[string]string{"russian": russian, "english": english} {
		t.Run(name, func(t *testing.T) {
			module, diagnostics := Parse(name+".bsl", source)
			if len(diagnostics) != 0 {
				t.Fatalf("diagnostics = %v", diagnostics)
			}
			if len(module.Routines) != 1 {
				t.Fatalf("routines = %d", len(module.Routines))
			}
			routine := module.Routines[0]
			if !routine.Function || !routine.Export || len(routine.Parameters) != 2 || len(routine.Body) != 1 {
				t.Fatalf("routine = %+v", routine)
			}
		})
	}
}

func TestParseReportsFilenameLineAndColumn(t *testing.T) {
	t.Parallel()

	_, diagnostics := Parse("broken.bsl", "Функция Тест()\nВозврат 1 + ;\nКонецФункции")
	if len(diagnostics) == 0 {
		t.Fatal("expected a diagnostic")
	}
	if got := diagnostics[0].Error(); got != "broken.bsl:2:13: unexpected ;, expected expression [BSL2001]" {
		t.Fatalf("diagnostic = %q", got)
	}
}

func TestTokenizeRejectsUnterminatedString(t *testing.T) {
	t.Parallel()

	_, diagnostics := Tokenize("broken.bsl", "Возврат \"текст\n")
	if len(diagnostics) != 1 || diagnostics[0].Code != "BSL1001" {
		t.Fatalf("diagnostics = %v", diagnostics)
	}
}

func TestParseRecoversFromMismatchedRoutineEnd(t *testing.T) {
	t.Parallel()

	module, diagnostics := Parse("broken.bsl", "Procedure Test()\nReturn;\nEndFunction")
	if len(diagnostics) != 1 || diagnostics[0].Code != "BSL2002" {
		t.Fatalf("diagnostics = %v", diagnostics)
	}
	if len(module.Routines) != 1 {
		t.Fatalf("routines = %d, want 1", len(module.Routines))
	}
}
