package querylang

import (
	"strings"
	"testing"
	"unicode/utf16"
)

// highlighted returns the stretches of the text by kind, cut from the text
// by the UTF-16 offsets the browser counts in.
func highlighted(source string) map[string][]string {
	units := utf16.Encode([]rune(source))
	result := map[string][]string{}
	for _, item := range HighlightText(source) {
		result[item.Kind] = append(result[item.Kind], string(utf16.Decode(units[item.Start:item.End])))
	}
	return result
}

// Every keyword of the help's table of two spellings («Двуязычное
// представление ключевых слов»), written out here apart from the list the
// highlighting reads. Defect caught: a keyword of the language left out, so
// a restriction shows it as a plain name.
func TestHighlightKnowsEveryKeywordOfTheHelp(t *testing.T) {
	const help = `АВТОУПОРЯДОЧИВАНИЕ AUTOORDER АВТОНОМЕРЗАПИСИ RECORDAUTONUMBER БУЛЕВО BOOLEAN В IN
		ВНЕШНЕЕ OUTER ВНУТРЕННЕЕ INNER ВОЗР ASC ВСЕ ALL ВЫБОР CASE ВЫБРАТЬ SELECT ВЫРАЗИТЬ CAST
		ГДЕ WHERE ГОД YEAR ГРУППИРУЮЩИМ GROUPING ДАТА DATE ДАТАВРЕМЯ DATETIME ДЕКАДА TENDAYS ДЕНЬ DAY
		ДЕНЬГОДА DAYOFYEAR ДЕНЬНЕДЕЛИ WEEKDAY ДЛЯ ИЗМЕНЕНИЯ FOR UPDATE OF ДОБАВИТЬ ADD
		ДОБАВИТЬКДАТЕ DATEADD ЕСТЬ IS ЕСТЬNULL ISNULL ЗНАЧЕНИЕ VALUE И AND ИЕРАРХИИ ИЕРАРХИЯ HIERARCHY
		ИЗ FROM ИЛИ OR ИМЕЮЩИЕ HAVING ИНАЧЕ ELSE ИНДЕКСИРОВАТЬ ПО INDEX BY ИСТИНА TRUE ИТОГИ TOTALS
		КАК AS КВАРТАЛ QUARTER КОГДА WHEN КОЛИЧЕСТВО COUNT КОНЕЦПЕРИОДА ENDOFPERIOD КОНЕЦ END
		ЛЕВОЕ LEFT ЛОЖЬ FALSE МАКСИМУМ MAX МЕЖДУ BETWEEN МЕСЯЦ MONTH МИНИМУМ MIN МИНУТА MINUTE
		НАБОРАМ SETS НАЧАЛОПЕРИОДА BEGINOFPERIOD НЕ NOT НЕДЕЛЯ WEEK НЕОПРЕДЕЛЕНО UNDEFINED
		ОБЩИЕ OVERALL ОБЪЕДИНИТЬ UNION ПЕРВЫЕ TOP ПЕРИОДАМИ PERIODS ПОДОБНО LIKE ПОДСТРОКА SUBSTRING
		ПОЛНОЕ FULL ПОЛУГОДИЕ HALFYEAR ПОМЕСТИТЬ INTO ПРАВОЕ RIGHT ПРЕДСТАВЛЕНИЕ PRESENTATION
		ПРЕДСТАВЛЕНИЕССЫЛКИ REFPRESENTATION ПУСТАЯТАБЛИЦА EMPTYTABLE РАЗЛИЧНЫЕ DISTINCT
		РАЗНОСТЬДАТ DATEDIFF РАЗРЕШЕННЫЕ ALLOWED СГРУППИРОВАТЬ GROUP СГРУППИРОВАНОПО GROUPEDBY
		СЕКУНДА SECOND СОЕДИНЕНИЕ JOIN ON СПЕЦСИМВОЛ ESCAPE СРЕДНЕЕ AVG ССЫЛКА REFS СТРОКА STRING
		СУММА SUM ТОГДА THEN ТИП TYPE ТИПЗНАЧЕНИЯ VALUETYPE ТОЛЬКО ONLY УБЫВ DESC УНИКАЛЬНО UNIQUE
		УПОРЯДОЧИТЬ ORDER ЧАС HOUR ЧИСЛО NUMBER УНИКАЛЬНЫЙИДЕНТИФИКАТОР UUID УНИЧТОЖИТЬ DROP NULL`
	words := strings.Fields(help)
	got := highlighted(help)["keyword"]
	if strings.Join(got, " ") != strings.Join(words, " ") {
		missing := []string{}
		for _, word := range words {
			if !isKeyword(word) {
				missing = append(missing, word)
			}
		}
		t.Fatalf("keywords highlighted %d of %d; not known: %v", len(got), len(words), missing)
	}
	// Names are told apart without regard to case, as in the language.
	if got := highlighted("где Где gdE where")["keyword"]; len(got) != 3 {
		t.Fatalf("keywords in any case = %v", got)
	}
}

// Defect caught: a field named as a keyword - Дата, Ссылка, Тип - painted as
// one after the dot, or a name merely starting with a keyword taken for it.
func TestHighlightTakesAWordAfterADotForAField(t *testing.T) {
	got := highlighted("ГДЕ Документ.Дата = Т . Ссылка И ГДЕНИБУДЬ")
	if strings.Join(got["keyword"], " ") != "ГДЕ И" {
		t.Fatalf("keywords = %v", got["keyword"])
	}
}

// The text of a restriction holds the preprocessor, the templates of the
// role and parameters of the session, and is highlighted while typed.
// Defect caught: an unfinished string or an instruction failing the whole
// text, a doubled quote ending a string, ## read as a template.
func TestHighlightTakesARestrictionAsTyped(t *testing.T) {
	source := `#Если &Флаг = "Да" #Тогда ГДЕ #Шаблон("""Склад""") ## 1.5 // заметка
#КонецЕсли "не закрыта`
	got := highlighted(source)
	want := map[string]string{
		"directive": `#Если|#Тогда|#Шаблон|##|#КонецЕсли`,
		"constant":  `&Флаг`,
		"string":    `"Да"|"""Склад"""|"не закрыта`,
		"keyword":   `ГДЕ`,
		"number":    `1.5`,
		"comment":   `// заметка`,
		"operator":  `=|(|)`,
	}
	for kind, expected := range want {
		if strings.Join(got[kind], "|") != expected {
			t.Errorf("%s = %q, want %q", kind, strings.Join(got[kind], "|"), expected)
		}
	}
}

// Offsets are counted as the browser counts them. Defect caught: offsets in
// bytes, or in characters, putting the colours beside the words after any
// Cyrillic letter or a character outside the basic plane.
func TestHighlightCountsInUTF16Units(t *testing.T) {
	source := "// 😀\nГДЕ"
	items := HighlightText(source)
	if len(items) != 2 || items[1] != (Highlight{Kind: "keyword", Start: 6, End: 9}) {
		t.Fatalf("highlights = %+v", items)
	}
	if items := HighlightText(""); len(items) != 0 {
		t.Fatalf("empty text = %+v", items)
	}
}
