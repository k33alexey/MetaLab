package querylang

import "strings"

// keyword is one keyword of the prototype's query language in both its
// spellings, with the token the parser knows it by. A keyword the parser does
// not take yet is an identifier to it, as it was before the keyword was
// listed: the list is the language's, the tokens are this parser's.
type keyword struct {
	russian, english string
	kind             tokenKind
}

// keywords are every keyword of the query language: the help's table of the
// two spellings («Двуязычное представление ключевых слов»), its phrases taken
// word by word (ДЛЯ ИЗМЕНЕНИЯ, СОЕДИНЕНИЕ ... ПО), and NULL, which has one
// spelling and a page of its own. ПО is BY and ON both.
var keywords = []keyword{
	{"АВТОУПОРЯДОЧИВАНИЕ", "AUTOORDER", tokenIdentifier},
	{"АВТОНОМЕРЗАПИСИ", "RECORDAUTONUMBER", tokenIdentifier},
	{"БУЛЕВО", "BOOLEAN", tokenIdentifier},
	{"В", "IN", tokenIn},
	{"ВНЕШНЕЕ", "OUTER", tokenOuter},
	{"ВНУТРЕННЕЕ", "INNER", tokenInner},
	{"ВОЗР", "ASC", tokenAscending},
	{"ВСЕ", "ALL", tokenIdentifier},
	{"ВЫБОР", "CASE", tokenIdentifier},
	{"ВЫБРАТЬ", "SELECT", tokenSelect},
	{"ВЫРАЗИТЬ", "CAST", tokenIdentifier},
	{"ГДЕ", "WHERE", tokenWhere},
	{"ГОД", "YEAR", tokenIdentifier},
	{"ГРУППИРУЮЩИМ", "GROUPING", tokenIdentifier},
	{"ДАТА", "DATE", tokenIdentifier},
	{"ДАТАВРЕМЯ", "DATETIME", tokenIdentifier},
	{"ДЕКАДА", "TENDAYS", tokenIdentifier},
	{"ДЕНЬ", "DAY", tokenIdentifier},
	{"ДЕНЬГОДА", "DAYOFYEAR", tokenIdentifier},
	{"ДЕНЬНЕДЕЛИ", "WEEKDAY", tokenIdentifier},
	{"ДЛЯ", "FOR", tokenIdentifier},
	{"ИЗМЕНЕНИЯ", "UPDATE", tokenIdentifier},
	{"", "OF", tokenIdentifier},
	{"ДОБАВИТЬ", "ADD", tokenIdentifier},
	{"ДОБАВИТЬКДАТЕ", "DATEADD", tokenIdentifier},
	{"ЕСТЬ", "IS", tokenIs},
	{"ЕСТЬNULL", "ISNULL", tokenIdentifier},
	{"ЗНАЧЕНИЕ", "VALUE", tokenIdentifier},
	{"И", "AND", tokenAnd},
	{"ИЕРАРХИИ", "HIERARCHY", tokenIdentifier},
	{"ИЕРАРХИЯ", "HIERARCHY", tokenIdentifier},
	{"ИЗ", "FROM", tokenFrom},
	{"ИЛИ", "OR", tokenOr},
	{"ИМЕЮЩИЕ", "HAVING", tokenHaving},
	{"ИНАЧЕ", "ELSE", tokenIdentifier},
	{"ИНДЕКСИРОВАТЬ", "INDEX", tokenIndex},
	{"ИСТИНА", "TRUE", tokenTrue},
	{"ИТОГИ", "TOTALS", tokenIdentifier},
	{"КАК", "AS", tokenAs},
	{"КВАРТАЛ", "QUARTER", tokenIdentifier},
	{"КОГДА", "WHEN", tokenIdentifier},
	{"КОЛИЧЕСТВО", "COUNT", tokenIdentifier},
	{"КОНЕЦПЕРИОДА", "ENDOFPERIOD", tokenIdentifier},
	{"КОНЕЦ", "END", tokenIdentifier},
	{"ЛЕВОЕ", "LEFT", tokenLeft},
	{"ЛОЖЬ", "FALSE", tokenFalse},
	{"МАКСИМУМ", "MAX", tokenIdentifier},
	{"МЕЖДУ", "BETWEEN", tokenIdentifier},
	{"МЕСЯЦ", "MONTH", tokenIdentifier},
	{"МИНИМУМ", "MIN", tokenIdentifier},
	{"МИНУТА", "MINUTE", tokenIdentifier},
	{"НАБОРАМ", "SETS", tokenIdentifier},
	{"НАЧАЛОПЕРИОДА", "BEGINOFPERIOD", tokenIdentifier},
	{"НЕ", "NOT", tokenNot},
	{"НЕДЕЛЯ", "WEEK", tokenIdentifier},
	{"НЕОПРЕДЕЛЕНО", "UNDEFINED", tokenUndefined},
	{"", "NULL", tokenNull},
	{"ОБЩИЕ", "OVERALL", tokenIdentifier},
	{"ОБЪЕДИНИТЬ", "UNION", tokenIdentifier},
	{"ПЕРВЫЕ", "TOP", tokenTop},
	{"ПЕРИОДАМИ", "PERIODS", tokenIdentifier},
	{"ПО", "BY", tokenBy},
	{"", "ON", tokenOn},
	{"ПОДОБНО", "LIKE", tokenLike},
	{"ПОДСТРОКА", "SUBSTRING", tokenIdentifier},
	{"ПОЛНОЕ", "FULL", tokenFull},
	{"ПОЛУГОДИЕ", "HALFYEAR", tokenIdentifier},
	{"ПОМЕСТИТЬ", "INTO", tokenInto},
	{"ПРАВОЕ", "RIGHT", tokenRight},
	{"ПРЕДСТАВЛЕНИЕ", "PRESENTATION", tokenIdentifier},
	{"ПРЕДСТАВЛЕНИЕССЫЛКИ", "REFPRESENTATION", tokenIdentifier},
	{"ПУСТАЯТАБЛИЦА", "EMPTYTABLE", tokenIdentifier},
	{"РАЗЛИЧНЫЕ", "DISTINCT", tokenDistinct},
	{"РАЗНОСТЬДАТ", "DATEDIFF", tokenIdentifier},
	{"РАЗРЕШЕННЫЕ", "ALLOWED", tokenIdentifier},
	{"СГРУППИРОВАТЬ", "GROUP", tokenGroup},
	{"СГРУППИРОВАНОПО", "GROUPEDBY", tokenIdentifier},
	{"СЕКУНДА", "SECOND", tokenIdentifier},
	{"СОЕДИНЕНИЕ", "JOIN", tokenJoin},
	{"СПЕЦСИМВОЛ", "ESCAPE", tokenIdentifier},
	{"СРЕДНЕЕ", "AVG", tokenIdentifier},
	{"ССЫЛКА", "REFS", tokenIdentifier},
	{"СТРОКА", "STRING", tokenIdentifier},
	{"СУММА", "SUM", tokenIdentifier},
	{"ТОГДА", "THEN", tokenIdentifier},
	{"ТИП", "TYPE", tokenIdentifier},
	{"ТИПЗНАЧЕНИЯ", "VALUETYPE", tokenIdentifier},
	{"ТОЛЬКО", "ONLY", tokenIdentifier},
	{"УБЫВ", "DESC", tokenDescending},
	{"УНИКАЛЬНО", "UNIQUE", tokenIdentifier},
	{"УПОРЯДОЧИТЬ", "ORDER", tokenOrder},
	{"ЧАС", "HOUR", tokenIdentifier},
	{"ЧИСЛО", "NUMBER", tokenIdentifier},
	{"УНИКАЛЬНЫЙИДЕНТИФИКАТОР", "UUID", tokenIdentifier},
	{"УНИЧТОЖИТЬ", "DROP", tokenDrop},
}

// keywordKinds are the keywords by their spelling in upper case.
var keywordKinds = func() map[string]tokenKind {
	result := make(map[string]tokenKind, 2*len(keywords))
	for _, word := range keywords {
		for _, spelling := range []string{word.russian, word.english} {
			if spelling != "" {
				result[spelling] = word.kind
			}
		}
	}
	return result
}()

func keywordKind(value string) tokenKind {
	if kind, ok := keywordKinds[strings.ToUpper(value)]; ok {
		return kind
	}
	return tokenIdentifier
}

// isKeyword says the word is a keyword of the query language, in either
// spelling and any case.
func isKeyword(value string) bool {
	_, ok := keywordKinds[strings.ToUpper(value)]
	return ok
}
