package syntax

import (
	"fmt"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/bsl/spec"
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

func TestParseRussianAndEnglishControlFlow(t *testing.T) {
	t.Parallel()

	sources := map[string]string{
		"russian": `Процедура Поток(Коллекция)
Итог = 0;
Если Истина Тогда
    Пока Ложь Цикл
        Прервать;
    КонецЦикла;
ИначеЕсли Не Ложь И 1 < 2 Или Ложь Тогда
    Для Индекс = 1 По 3 Цикл
        Продолжить;
    КонецЦикла;
Иначе
    Для Каждого Элемент Из Коллекция Цикл
        Прервать;
    КонецЦикла;
КонецЕсли;
КонецПроцедуры`,
		"english": `Procedure Flow(Collection)
Total = 0;
If True Then
    While False Do
        Break;
    EndDo;
ElsIf Not False And 1 < 2 Or False Then
    For Index = 1 To 3 Do
        Continue;
    EndDo;
Else
    For Each Item In Collection Do
        Break;
    EndDo;
EndIf;
EndProcedure`,
	}
	for name, source := range sources {
		t.Run(name, func(t *testing.T) {
			module, diagnostics := Parse(name+".bsl", source)
			if len(diagnostics) != 0 {
				t.Fatalf("diagnostics = %v", diagnostics)
			}
			if len(module.Routines) != 1 || len(module.Routines[0].Body) != 2 {
				t.Fatalf("module = %+v", module)
			}
			if _, ok := module.Routines[0].Body[0].(*AssignmentStatement); !ok {
				t.Fatalf("first statement = %T", module.Routines[0].Body[0])
			}
			conditional, ok := module.Routines[0].Body[1].(*IfStatement)
			if !ok || len(conditional.Branches) != 2 || len(conditional.ElseBody) != 1 {
				t.Fatalf("conditional = %#v", module.Routines[0].Body[1])
			}
			if _, ok := conditional.Branches[0].Body[0].(*WhileStatement); !ok {
				t.Fatalf("first branch = %T", conditional.Branches[0].Body[0])
			}
			if _, ok := conditional.Branches[1].Body[0].(*ForStatement); !ok {
				t.Fatalf("second branch = %T", conditional.Branches[1].Body[0])
			}
			if _, ok := conditional.ElseBody[0].(*ForEachStatement); !ok {
				t.Fatalf("else branch = %T", conditional.ElseBody[0])
			}
			condition := conditional.Branches[1].Condition.(*BinaryExpression)
			if condition.Operator != Or || condition.Left.(*BinaryExpression).Operator != And {
				t.Fatalf("condition precedence = %#v", condition)
			}
			if conditional.SourceSpan.Start.Line != 3 || conditional.SourceSpan.End.Line != 15 {
				t.Fatalf("conditional span = %+v", conditional.SourceSpan)
			}
		})
	}
}

func TestParsePrimitiveLiterals(t *testing.T) {
	t.Parallel()

	source := `Function DateValue() Return '20260904153045'; EndFunction
Function UndefinedValue() Return Undefined; EndFunction
Function NullValue() Return Null; EndFunction
Function BooleanValue() Return True; EndFunction`
	module, diagnostics := Parse("literals.bsl", source)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	if len(module.Routines) != 4 {
		t.Fatalf("routines = %d", len(module.Routines))
	}
	want := []Expression{&DateExpression{}, &UndefinedExpression{}, &NullExpression{}, &BooleanExpression{}}
	for index, expected := range want {
		returned := module.Routines[index].Body[0].(*ReturnStatement)
		if fmt.Sprintf("%T", returned.Value) != fmt.Sprintf("%T", expected) {
			t.Fatalf("routine %d literal = %T, want %T", index, returned.Value, expected)
		}
	}
}

func TestParseVariablesParametersAndCalls(t *testing.T) {
	t.Parallel()

	source := `Перем Состояние Экспорт, Счётчик;
Процедура Установить(Знач Новое, Дополнение = 1) Экспорт
    Перем Локальная;
    Состояние = Новое + Дополнение;
КонецПроцедуры
Функция Получить()
    Установить(Модуль.Значение(), );
    Установить(,);
    Возврат Состояние;
КонецФункции`
	module, diagnostics := Parse("module.bsl", source)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	if len(module.Variables) != 2 || !module.Variables[0].Export || module.Variables[1].Export || module.Variables[1].Name != "Счётчик" {
		t.Fatalf("module variables = %+v", module.Variables)
	}
	parameters := module.Routines[0].Parameters
	if len(parameters) != 2 || !parameters[0].ByValue || parameters[0].Default != nil || parameters[1].Default == nil {
		t.Fatalf("parameters = %+v", parameters)
	}
	if declaration, ok := module.Routines[0].Body[0].(*VariableStatement); !ok || declaration.Variables[0].Name != "Локальная" {
		t.Fatalf("local declaration = %#v", module.Routines[0].Body[0])
	}
	statement, ok := module.Routines[1].Body[0].(*CallStatement)
	if !ok || statement.Call.Name != "Установить" || len(statement.Call.Arguments) != 2 || statement.Call.Arguments[1].Value != nil {
		t.Fatalf("call statement = %#v", module.Routines[1].Body[0])
	}
	nested, ok := statement.Call.Arguments[0].Value.(*CallExpression)
	if !ok || nested.Qualifier != "Модуль" || nested.Name != "Значение" {
		t.Fatalf("nested call = %#v", statement.Call.Arguments[0].Value)
	}
	empty := module.Routines[1].Body[1].(*CallStatement)
	if len(empty.Call.Arguments) != 2 || empty.Call.Arguments[0].Value != nil || empty.Call.Arguments[1].Value != nil {
		t.Fatalf("omitted arguments = %#v", empty.Call.Arguments)
	}
}

func TestParseRejectsExportedLocalVariable(t *testing.T) {
	t.Parallel()

	_, diagnostics := Parse("local.bsl", "Procedure Test() Var Local Export; EndProcedure")
	if len(diagnostics) != 1 || diagnostics[0].Code != "BSL2003" {
		t.Fatalf("diagnostics = %v", diagnostics)
	}
}

func TestParseRussianAndEnglishExceptions(t *testing.T) {
	t.Parallel()

	tests := []string{
		`Процедура Проверить()
Попытка
    ВызватьИсключение "ошибка";
Исключение
    ВызватьИсключение;
КонецПопытки;
КонецПроцедуры`,
		`Procedure Check()
Try
    Raise "error";
Except
    Raise;
EndTry;
EndProcedure`,
	}
	for _, source := range tests {
		module, diagnostics := Parse("exceptions.bsl", source)
		if len(diagnostics) != 0 {
			t.Fatalf("unexpected diagnostics: %+v", diagnostics)
		}
		statement, ok := module.Routines[0].Body[0].(*TryStatement)
		if !ok || len(statement.Body) != 1 || len(statement.ExceptBody) != 1 {
			t.Fatalf("try statement = %#v", module.Routines[0].Body[0])
		}
		raise, ok := statement.Body[0].(*RaiseStatement)
		if !ok || raise.Value == nil {
			t.Fatalf("raise statement = %#v", statement.Body[0])
		}
		reraise, ok := statement.ExceptBody[0].(*RaiseStatement)
		if !ok || reraise.Value != nil {
			t.Fatalf("reraise statement = %#v", statement.ExceptBody[0])
		}
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

	tokens, diagnostics := Tokenize("broken.bsl", "Возврат \"текст\n")
	if len(diagnostics) != 1 || diagnostics[0].Code != "BSL1001" {
		t.Fatalf("diagnostics = %v", diagnostics)
	}
	if tokens[1].Kind != StringStart {
		t.Fatalf("tokens = %+v", tokens)
	}
}

func TestCatalogKeywordsAndOperatorsAreRecognized(t *testing.T) {
	t.Parallel()

	catalog, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	seenKinds := make(map[Kind]string, len(catalog.Keywords))
	for _, keyword := range catalog.Keywords {
		russian, russianOK := KeywordKind(strings.ToUpper(keyword.Russian))
		english, englishOK := KeywordKind(strings.ToUpper(keyword.English))
		if !russianOK || !englishOK || russian != english || !russian.IsKeyword() {
			t.Fatalf("keyword %+v maps to %v/%v (%v/%v)", keyword, russian, english, russianOK, englishOK)
		}
		if owner, exists := seenKinds[russian]; exists && owner != keyword.ID {
			t.Fatalf("keyword kind %v belongs to %q and %q", russian, owner, keyword.ID)
		}
		seenKinds[russian] = keyword.ID
		for _, alias := range []string{keyword.Russian, keyword.English} {
			tokens, diagnostics := Tokenize("property.bsl", "Object."+alias)
			if len(diagnostics) != 0 || len(tokens) != 4 || tokens[2].Kind != Identifier {
				t.Fatalf("property keyword %q tokens=%+v diagnostics=%v", alias, tokens, diagnostics)
			}
		}
	}
	if len(seenKinds) != len(catalog.Keywords) {
		t.Fatalf("recognized %d keyword kinds, want %d", len(seenKinds), len(catalog.Keywords))
	}
	for _, operator := range catalog.Operators {
		tokens, diagnostics := Tokenize("operator.bsl", operator.Lexeme)
		if len(diagnostics) != 0 || len(tokens) != 2 || tokens[0].Kind == Invalid || tokens[0].Lexeme != operator.Lexeme || tokens[1].Kind != EOF {
			t.Fatalf("operator %+v tokens=%+v diagnostics=%v", operator, tokens, diagnostics)
		}
	}
}

func TestConformanceCorpusIsLexicallyValid(t *testing.T) {
	t.Parallel()

	catalog, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	checked := make(map[string]bool)
	for _, feature := range catalog.Features {
		for _, name := range feature.Corpus {
			if checked[name] {
				continue
			}
			checked[name] = true
			source, err := spec.Corpus(name)
			if err != nil {
				t.Fatal(err)
			}
			tokens, diagnostics := Tokenize(name, string(source))
			if len(diagnostics) != 0 {
				t.Fatalf("%s diagnostics = %v", name, diagnostics)
			}
			assertLosslessTokens(t, string(source), tokens)
		}
	}
}

func TestEveryTokenKindHasAStableName(t *testing.T) {
	t.Parallel()

	for kind := Invalid; kind <= Bar; kind++ {
		if strings.HasPrefix(kind.String(), "token(") {
			t.Fatalf("token kind %d has no stable name", kind)
		}
	}
}

func TestTokenizeOperatorsNumbersDatesAndDelimiters(t *testing.T) {
	t.Parallel()

	source := "0 42. 42.50 '' '20260904' '20260904153045' + - * / % = <> < <= > >= ( ) [ ] . , ; : ? & # ~ |"
	tokens, diagnostics := Tokenize("all.bsl", source)
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %v", diagnostics)
	}
	want := []Kind{
		Number, Number, Number, Date, Date, Date,
		Plus, Minus, Star, Slash, Percent, Equal, NotEqual, Less, LessEqual, Greater, GreaterEqual,
		LeftParen, RightParen, LeftBracket, RightBracket, Dot, Comma, Semicolon, Colon, Question, Ampersand, Hash, Tilde, Bar, EOF,
	}
	if len(tokens) != len(want) {
		t.Fatalf("token count = %d, want %d: %+v", len(tokens), len(want), tokens)
	}
	for index, kind := range want {
		if tokens[index].Kind != kind {
			t.Fatalf("token %d = %v, want %v", index, tokens[index].Kind, kind)
		}
	}
	if tokens[2].Value != "42.50" || tokens[3].Value != "" || tokens[5].Value != "20260904153045" {
		t.Fatalf("literal values = %q, %q, %q", tokens[2].Value, tokens[3].Value, tokens[5].Value)
	}
}

func TestTokenizeMultilineStringAndStructuredTrivia(t *testing.T) {
	t.Parallel()

	source := "\ufeff// описание\r\n\tЗначение = \"Первая\r\n  |Вторая\n\t|Третья \"\"строка\"\"\"; // конец"
	tokens, diagnostics := Tokenize("multiline.bsl", source)
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %v", diagnostics)
	}
	if len(tokens) != 7 || tokens[0].Kind != Identifier || tokens[1].Kind != Equal || tokens[2].Kind != StringStart || tokens[3].Kind != StringPart || tokens[4].Kind != StringEnd || tokens[5].Kind != Semicolon || tokens[6].Kind != EOF {
		t.Fatalf("tokens = %+v", tokens)
	}
	if got := strings.Join([]string{tokens[2].Value, tokens[3].Value, tokens[4].Value}, "\n"); got != "Первая\nВторая\nТретья \"строка\"" {
		t.Fatalf("multiline value = %q", got)
	}
	if tokens[2].Span.Start.Line != 2 || tokens[4].Span.End.Line != 4 {
		t.Fatalf("multiline spans = %+v .. %+v", tokens[2].Span, tokens[4].Span)
	}
	trivia := tokens[0].LeadingTrivia
	if len(trivia) != 3 || trivia[0].Kind != ByteOrderMarkTrivia || trivia[1].Kind != LineCommentTrivia || trivia[2].Kind != WhitespaceTrivia {
		t.Fatalf("leading trivia = %+v", trivia)
	}
	if tokens[0].Span.Start.Offset != len("\ufeff// описание\r\n\t") || tokens[0].Span.Start.Line != 2 || tokens[0].Span.Start.Column != 2 {
		t.Fatalf("first token span = %+v", tokens[0].Span)
	}
	if len(tokens[6].LeadingTrivia) != 2 || tokens[6].LeadingTrivia[1].Kind != LineCommentTrivia {
		t.Fatalf("EOF trivia = %+v", tokens[6].LeadingTrivia)
	}
	assertLosslessTokens(t, source, tokens)
}

func TestTokenizeReportsLexicalErrorsAndContinues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		source string
		code   string
		kind   Kind
	}{
		{name: "string", source: "\"строка\nСледующий", code: "BSL1001", kind: StringStart},
		{name: "date", source: "'date' Следующий", code: "BSL1004", kind: Invalid},
		{name: "unterminated date", source: "'20260904\nСледующий", code: "BSL1003", kind: Invalid},
		{name: "character", source: "! Следующий", code: "BSL1002", kind: Invalid},
		{name: "unsupported whitespace", source: "\u00a0Следующий", code: "BSL1002", kind: Invalid},
		{name: "utf8", source: string([]byte{'A', 0xff, 'B'}), code: "BSL1005", kind: Identifier},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			tokens, diagnostics := Tokenize(test.name+".bsl", test.source)
			if len(diagnostics) != 1 || diagnostics[0].Code != test.code {
				t.Fatalf("diagnostics = %v", diagnostics)
			}
			if tokens[0].Kind != test.kind || tokens[len(tokens)-1].Kind != EOF {
				t.Fatalf("tokens = %+v", tokens)
			}
			assertLosslessTokens(t, test.source, tokens)
		})
	}
}

func TestTokenizeContextSensitiveIdentifiersAndTrailingDots(t *testing.T) {
	t.Parallel()

	source := "Поле.Процедура Запрос.  Выполнить\nА. // комментарий\nЕсли\n~Если: &Если"
	tokens, diagnostics := Tokenize("contexts.bsl", source)
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %v", diagnostics)
	}
	want := []Kind{
		Identifier, Dot, Identifier,
		Identifier, Dot, Execute,
		Identifier, DotTrailing, If,
		Tilde, Identifier, Colon, Ampersand, Identifier, EOF,
	}
	if len(tokens) != len(want) {
		t.Fatalf("tokens = %+v", tokens)
	}
	for index, kind := range want {
		if tokens[index].Kind != kind {
			t.Fatalf("token %d (%q) = %v, want %v", index, tokens[index].Lexeme, tokens[index].Kind, kind)
		}
	}
}

func TestTokenizeBarOutsideMultilineString(t *testing.T) {
	t.Parallel()

	tokens, diagnostics := Tokenize("bar.bsl", "|Если")
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %v", diagnostics)
	}
	want := []Kind{Bar, If, EOF}
	if len(tokens) != len(want) {
		t.Fatalf("tokens = %+v", tokens)
	}
	for index, kind := range want {
		if tokens[index].Kind != kind {
			t.Fatalf("token %d (%q) = %v, want %v", index, tokens[index].Lexeme, tokens[index].Kind, kind)
		}
	}
}

func TestTokenizeMultilineStringWithPreprocessorLines(t *testing.T) {
	t.Parallel()

	source := "\"Начало\n#Если Сервер Тогда\n|Сервер\n#Иначе\n|Клиент\""
	tokens, diagnostics := Tokenize("conditional-string.bsl", source)
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %v", diagnostics)
	}
	want := []Kind{StringStart, Hash, If, Identifier, Then, StringPart, Hash, Else, StringEnd, EOF}
	if len(tokens) != len(want) {
		t.Fatalf("tokens = %+v", tokens)
	}
	for index, kind := range want {
		if tokens[index].Kind != kind {
			t.Fatalf("token %d (%q) = %v, want %v", index, tokens[index].Lexeme, tokens[index].Kind, kind)
		}
	}
	assertLosslessTokens(t, source, tokens)
}

func TestParseSimpleMultilineString(t *testing.T) {
	t.Parallel()

	module, diagnostics := Parse("string.bsl", "Функция Текст()\nВозврат \"Первая\n |Вторая\n |Третья\";\nКонецФункции")
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %v", diagnostics)
	}
	returned, ok := module.Routines[0].Body[0].(*ReturnStatement)
	if !ok {
		t.Fatalf("statement = %#v", module.Routines[0].Body[0])
	}
	value, ok := returned.Value.(*StringExpression)
	if !ok || value.Value != "Первая\nВторая\nТретья" || value.SourceSpan.Start.Line != 2 || value.SourceSpan.End.Line != 4 {
		t.Fatalf("value = %#v", returned.Value)
	}
}

func TestTokenizeRegionNameAsIdentifier(t *testing.T) {
	t.Parallel()

	for _, source := range []string{"#Область Если", "# Region If"} {
		tokens, diagnostics := Tokenize("region.bsl", source)
		if len(diagnostics) != 0 || len(tokens) != 4 || tokens[0].Kind != Hash || tokens[1].Kind != Identifier || tokens[2].Kind != Identifier || tokens[3].Kind != EOF {
			t.Fatalf("source=%q tokens=%+v diagnostics=%v", source, tokens, diagnostics)
		}
	}
}

func TestTokenizeReportsInvalidUTF8InsideCommentAndString(t *testing.T) {
	t.Parallel()

	source := string([]byte{'/', '/', ' ', 0xff, '\n', '"', 0xfe, '"'})
	tokens, diagnostics := Tokenize("encoding.bsl", source)
	if len(diagnostics) != 2 || diagnostics[0].Code != "BSL1005" || diagnostics[1].Code != "BSL1005" {
		t.Fatalf("diagnostics = %v", diagnostics)
	}
	if len(tokens) != 2 || tokens[0].Kind != String || tokens[1].Kind != EOF {
		t.Fatalf("tokens = %+v", tokens)
	}
	assertLosslessTokens(t, source, tokens)
}

func FuzzTokenizeLossless(f *testing.F) {
	for _, source := range []string{
		"", "Функция Тест()\nКонецФункции", "// комментарий\r\nВозврат 1;",
		"\"Первая\n|Вторая\"", "'20260904153045'", string([]byte{0xff, '\r', '\n'}),
	} {
		f.Add(source)
	}
	f.Fuzz(func(t *testing.T, source string) {
		tokens, _ := Tokenize("fuzz.bsl", source)
		assertLosslessTokens(t, source, tokens)
		previousEnd := 0
		for index, token := range tokens {
			if token.Span.Start.Offset < previousEnd || token.Span.Start.Offset > token.Span.End.Offset || token.Span.End.Offset > len(source) {
				t.Fatalf("invalid token %d span %+v after offset %d", index, token.Span, previousEnd)
			}
			previousEnd = token.Span.End.Offset
		}
		if len(tokens) == 0 || tokens[len(tokens)-1].Kind != EOF {
			t.Fatalf("missing EOF token: %+v", tokens)
		}
	})
}

func FuzzParseControlFlowNeverPanics(f *testing.F) {
	for _, source := range []string{
		"Procedure Test() EndProcedure",
		"Function Test(Value) If Value > 0 Then Return Value; Else Return 0; EndIf; EndFunction",
		"Procedure Test() While True Do Break; EndDo; EndProcedure",
		"Procedure Test(Items) For Each Item In Items Do Continue; EndDo; EndProcedure",
		"Procedure Test() Try Raise \"boom\"; Except Raise; EndTry; EndProcedure",
	} {
		f.Add(source)
	}
	f.Fuzz(func(t *testing.T, source string) {
		module, diagnostics := Parse("fuzz.bsl", source)
		if module == nil {
			t.Fatal("Parse() returned a nil module")
		}
		for _, diagnostic := range diagnostics {
			if diagnostic.Span.Start.Offset < 0 || diagnostic.Span.End.Offset < diagnostic.Span.Start.Offset || diagnostic.Span.End.Offset > len(source) {
				t.Fatalf("diagnostic span = %+v, source bytes = %d", diagnostic.Span, len(source))
			}
		}
	})
}

func BenchmarkTokenize(b *testing.B) {
	source := strings.Repeat("// Строка\nЕсли Значение >= 42 Тогда\nРезультат = \"Текст\";\nКонецЕсли;\n", 100)
	b.ReportAllocs()
	b.SetBytes(int64(len(source)))
	for range b.N {
		Tokenize("benchmark.bsl", source)
	}
}

func assertLosslessTokens(t testing.TB, source string, tokens []Token) {
	t.Helper()
	var reconstructed strings.Builder
	for _, token := range tokens {
		reconstructed.WriteString(token.Leading)
		reconstructed.WriteString(token.Lexeme)
	}
	if reconstructed.String() != source {
		t.Fatalf("lossless source = %q, want %q", reconstructed.String(), source)
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
