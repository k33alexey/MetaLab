package querylang

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

type tokenKind uint8

const (
	tokenEOF tokenKind = iota
	tokenIdentifier
	tokenParameter
	tokenString
	tokenNumber
	tokenSelect
	tokenDistinct
	tokenTop
	tokenFrom
	tokenAs
	tokenInto
	tokenWhere
	tokenGroup
	tokenHaving
	tokenOrder
	tokenBy
	tokenIndex
	tokenDrop
	tokenInner
	tokenLeft
	tokenRight
	tokenFull
	tokenOuter
	tokenJoin
	tokenOn
	tokenAscending
	tokenDescending
	tokenAnd
	tokenOr
	tokenNot
	tokenIn
	tokenLike
	tokenIs
	tokenNull
	tokenTrue
	tokenFalse
	tokenUndefined
	tokenComma
	tokenDot
	tokenLeftParen
	tokenRightParen
	tokenStar
	tokenEqual
	tokenNotEqual
	tokenLess
	tokenLessEqual
	tokenGreater
	tokenGreaterEqual
	tokenMinus
	tokenSemicolon
)

type token struct {
	kind     tokenKind
	text     string
	position Position
}

type lexer struct {
	source string
	offset int
	line   int
	column int
}

func lex(source string) ([]token, error) {
	value := lexer{source: source, line: 1, column: 1}
	result := make([]token, 0, len(source)/4)
	for {
		value.skipSpaceAndComments()
		position := value.position()
		r, _ := value.peek()
		if r == 0 {
			return append(result, token{kind: tokenEOF, position: position}), nil
		}
		if identifierStart(r) {
			text := value.readIdentifier()
			result = append(result, token{kind: keywordKind(text), text: text, position: position})
			continue
		}
		if unicode.IsDigit(r) {
			text, err := value.readNumber()
			if err != nil {
				return nil, queryError(position, err.Error())
			}
			result = append(result, token{kind: tokenNumber, text: text, position: position})
			continue
		}
		switch r {
		case '&':
			value.take()
			start, _ := value.peek()
			if !identifierStart(start) {
				return nil, queryError(position, "после & ожидается имя параметра")
			}
			result = append(result, token{kind: tokenParameter, text: value.readIdentifier(), position: position})
		case '"':
			text, err := value.readString()
			if err != nil {
				return nil, queryError(position, err.Error())
			}
			result = append(result, token{kind: tokenString, text: text, position: position})
		case ',':
			value.take()
			result = append(result, token{kind: tokenComma, text: ",", position: position})
		case '.':
			value.take()
			result = append(result, token{kind: tokenDot, text: ".", position: position})
		case '(':
			value.take()
			result = append(result, token{kind: tokenLeftParen, text: "(", position: position})
		case ')':
			value.take()
			result = append(result, token{kind: tokenRightParen, text: ")", position: position})
		case '*':
			value.take()
			result = append(result, token{kind: tokenStar, text: "*", position: position})
		case '=':
			value.take()
			result = append(result, token{kind: tokenEqual, text: "=", position: position})
		case '<':
			value.take()
			next, _ := value.peek()
			switch next {
			case '=':
				value.take()
				result = append(result, token{kind: tokenLessEqual, text: "<=", position: position})
			case '>':
				value.take()
				result = append(result, token{kind: tokenNotEqual, text: "<>", position: position})
			default:
				result = append(result, token{kind: tokenLess, text: "<", position: position})
			}
		case '>':
			value.take()
			if next, _ := value.peek(); next == '=' {
				value.take()
				result = append(result, token{kind: tokenGreaterEqual, text: ">=", position: position})
			} else {
				result = append(result, token{kind: tokenGreater, text: ">", position: position})
			}
		case '!':
			value.take()
			if next, _ := value.peek(); next != '=' {
				return nil, queryError(position, "ожидался оператор !=")
			}
			value.take()
			result = append(result, token{kind: tokenNotEqual, text: "!=", position: position})
		case '-':
			value.take()
			result = append(result, token{kind: tokenMinus, text: "-", position: position})
		case ';':
			value.take()
			result = append(result, token{kind: tokenSemicolon, text: ";", position: position})
		default:
			return nil, queryError(position, fmt.Sprintf("недопустимый символ %q", r))
		}
	}
}

func (value *lexer) position() Position {
	return Position{Offset: value.offset, Line: value.line, Column: value.column}
}

func (value *lexer) peek() (rune, int) {
	if value.offset >= len(value.source) {
		return 0, 0
	}
	r, size := utf8.DecodeRuneInString(value.source[value.offset:])
	return r, size
}

func (value *lexer) take() rune {
	r, size := value.peek()
	if size == 0 {
		return 0
	}
	value.offset += size
	if r == '\n' {
		value.line++
		value.column = 1
	} else {
		value.column++
	}
	return r
}

func (value *lexer) skipSpaceAndComments() {
	for {
		for r, _ := value.peek(); unicode.IsSpace(r); r, _ = value.peek() {
			value.take()
		}
		if !strings.HasPrefix(value.source[value.offset:], "//") && !strings.HasPrefix(value.source[value.offset:], "--") {
			return
		}
		for r, _ := value.peek(); r != 0 && r != '\n'; r, _ = value.peek() {
			value.take()
		}
	}
}

func (value *lexer) readIdentifier() string {
	start := value.offset
	for r, _ := value.peek(); identifierPart(r); r, _ = value.peek() {
		value.take()
	}
	return value.source[start:value.offset]
}

func (value *lexer) readNumber() (string, error) {
	start := value.offset
	for r, _ := value.peek(); unicode.IsDigit(r); r, _ = value.peek() {
		value.take()
	}
	if r, _ := value.peek(); r == '.' {
		value.take()
		if digit, _ := value.peek(); !unicode.IsDigit(digit) {
			return "", fmt.Errorf("после десятичной точки ожидается цифра")
		}
		for digit, _ := value.peek(); unicode.IsDigit(digit); digit, _ = value.peek() {
			value.take()
		}
	}
	return value.source[start:value.offset], nil
}

func (value *lexer) readString() (string, error) {
	value.take()
	var result strings.Builder
	for {
		r, _ := value.peek()
		if r == 0 {
			return "", fmt.Errorf("незавершённая строка")
		}
		value.take()
		if r != '"' {
			result.WriteRune(r)
			continue
		}
		if next, _ := value.peek(); next == '"' {
			value.take()
			result.WriteRune('"')
			continue
		}
		return result.String(), nil
	}
}

func identifierStart(r rune) bool { return r == '_' || unicode.IsLetter(r) }
func identifierPart(r rune) bool  { return identifierStart(r) || unicode.IsDigit(r) }

func keywordKind(value string) tokenKind {
	if kind, ok := keywordKinds[strings.ToUpper(value)]; ok {
		return kind
	}
	return tokenIdentifier
}

var keywordKinds = map[string]tokenKind{
	"ВЫБРАТЬ": tokenSelect, "SELECT": tokenSelect,
	"РАЗЛИЧНЫЕ": tokenDistinct, "DISTINCT": tokenDistinct,
	"ПЕРВЫЕ": tokenTop, "TOP": tokenTop,
	"ИЗ": tokenFrom, "FROM": tokenFrom,
	"КАК": tokenAs, "AS": tokenAs,
	"ПОМЕСТИТЬ": tokenInto, "INTO": tokenInto,
	"ГДЕ": tokenWhere, "WHERE": tokenWhere,
	"СГРУППИРОВАТЬ": tokenGroup, "GROUP": tokenGroup,
	"ИМЕЮЩИЕ": tokenHaving, "HAVING": tokenHaving,
	"УПОРЯДОЧИТЬ": tokenOrder, "ORDER": tokenOrder,
	"ПО": tokenBy, "BY": tokenBy,
	"ИНДЕКСИРОВАТЬ": tokenIndex, "INDEX": tokenIndex,
	"УНИЧТОЖИТЬ": tokenDrop, "DROP": tokenDrop,
	"ВНУТРЕННЕЕ": tokenInner, "INNER": tokenInner,
	"ЛЕВОЕ": tokenLeft, "LEFT": tokenLeft,
	"ПРАВОЕ": tokenRight, "RIGHT": tokenRight,
	"ПОЛНОЕ": tokenFull, "FULL": tokenFull,
	"ВНЕШНЕЕ": tokenOuter, "OUTER": tokenOuter,
	"СОЕДИНЕНИЕ": tokenJoin, "JOIN": tokenJoin,
	"ON":   tokenOn,
	"ВОЗР": tokenAscending, "ASC": tokenAscending,
	"УБЫВ": tokenDescending, "DESC": tokenDescending,
	"И": tokenAnd, "AND": tokenAnd,
	"ИЛИ": tokenOr, "OR": tokenOr,
	"НЕ": tokenNot, "NOT": tokenNot,
	"В": tokenIn, "IN": tokenIn,
	"ПОДОБНО": tokenLike, "LIKE": tokenLike,
	"ЕСТЬ": tokenIs, "IS": tokenIs,
	"NULL":   tokenNull,
	"ИСТИНА": tokenTrue, "TRUE": tokenTrue,
	"ЛОЖЬ": tokenFalse, "FALSE": tokenFalse,
	"НЕОПРЕДЕЛЕНО": tokenUndefined, "UNDEFINED": tokenUndefined,
}

func queryError(position Position, message string) error {
	return fmt.Errorf("query line %d, column %d: %s", position.Line, position.Column, message)
}
