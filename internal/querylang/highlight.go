package querylang

import (
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

// Highlight is one stretch of a text in the query language an editor shows
// in a style of its own. Start and End count UTF-16 units, as a text field of
// the browser does.
type Highlight struct {
	Kind  string `json:"kind"`
	Start int    `json:"start"`
	End   int    `json:"end"`
}

// HighlightText marks out the keywords, strings, numbers, comments,
// parameters and instructions of a text in the query language. It never
// fails: the text of a restriction is highlighted while it is typed, with
// the preprocessor's lines, the role's templates and an unfinished string in
// it (help, «Редактирование роли»). A word after a dot is a field, not a
// keyword: Документ.Дата.
func HighlightText(source string) []Highlight {
	value := lexer{source: source, line: 1, column: 1}
	var result []Highlight
	units, unitsAt := 0, 0
	// utf16At counts the units up to a byte offset, going forward only.
	utf16At := func(offset int) int {
		for unitsAt < offset {
			r, size := utf8.DecodeRuneInString(source[unitsAt:])
			units += utf16.RuneLen(r)
			unitsAt += size
		}
		return units
	}
	mark := func(kind string, start int) {
		result = append(result, Highlight{Kind: kind, Start: utf16At(start), End: utf16At(value.offset)})
	}
	afterDot := false
	for value.offset < len(source) {
		start := value.offset
		r, _ := value.peek()
		switch {
		case unicode.IsSpace(r):
			value.take()
			continue
		case strings.HasPrefix(source[start:], "//"):
			for r, _ := value.peek(); value.offset < len(source) && r != '\n'; r, _ = value.peek() {
				value.take()
			}
			mark("comment", start)
		case identifierStart(r):
			word := value.readIdentifier()
			if !afterDot && isKeyword(word) {
				mark("keyword", start)
			}
		case unicode.IsDigit(r):
			for r, _ := value.peek(); unicode.IsDigit(r) || r == '.'; r, _ = value.peek() {
				value.take()
			}
			mark("number", start)
		case r == '"':
			value.take()
			for value.offset < len(source) {
				if value.take() == '"' {
					if next, _ := value.peek(); next != '"' {
						break
					}
					value.take()
				}
			}
			mark("string", start)
		case r == '&':
			value.take()
			value.readIdentifier()
			mark("constant", start)
		case r == '#':
			// An instruction of the preprocessor, a template, #ТекущаяТаблица,
			// #Параметр - or ##, the sign itself.
			value.take()
			if next, _ := value.peek(); next == '#' {
				value.take()
			} else {
				value.readIdentifier()
			}
			mark("directive", start)
		default:
			value.take()
			mark("operator", start)
		}
		afterDot = r == '.'
	}
	return result
}
