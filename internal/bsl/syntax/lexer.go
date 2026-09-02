package syntax

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

var keywords = map[string]Kind{
	"функция":        Function,
	"function":       Function,
	"конецфункции":   EndFunction,
	"endfunction":    EndFunction,
	"процедура":      Procedure,
	"procedure":      Procedure,
	"конецпроцедуры": EndProcedure,
	"endprocedure":   EndProcedure,
	"экспорт":        Export,
	"export":         Export,
	"возврат":        Return,
	"return":         Return,
}

// Tokenize converts one BSL source file into lossless tokens.
func Tokenize(filename, source string) ([]Token, []Diagnostic) {
	state := lexer{filename: filename, source: source, line: 1, column: 1}
	return state.run()
}

type lexer struct {
	filename    string
	source      string
	offset      int
	line        int
	column      int
	tokens      []Token
	diagnostics []Diagnostic
}

func (l *lexer) run() ([]Token, []Diagnostic) {
	for !l.atEnd() {
		triviaStart := l.offset
		l.skipTrivia()
		leading := l.source[triviaStart:l.offset]
		if l.atEnd() {
			position := l.position()
			l.tokens = append(l.tokens, Token{Kind: EOF, Leading: leading, Span: Span{Start: position, End: position}})
			return l.tokens, l.diagnostics
		}

		start := l.position()
		current, _ := l.peek()
		switch {
		case isIdentifierStart(current):
			l.scanIdentifier(start, leading)
		case unicode.IsDigit(current):
			l.scanNumber(start, leading)
		case current == '"':
			l.scanString(start, leading)
		default:
			l.scanSymbol(start, leading, current)
		}
	}

	position := l.position()
	l.tokens = append(l.tokens, Token{Kind: EOF, Span: Span{Start: position, End: position}})
	return l.tokens, l.diagnostics
}

func (l *lexer) skipTrivia() {
	for !l.atEnd() {
		current, _ := l.peek()
		if unicode.IsSpace(current) {
			l.advance()
			continue
		}
		if current == '/' {
			next, ok := l.peekNext()
			if ok && next == '/' {
				for !l.atEnd() {
					commentRune, _ := l.peek()
					if commentRune == '\n' || commentRune == '\r' {
						break
					}
					l.advance()
				}
				continue
			}
		}
		return
	}
}

func (l *lexer) scanIdentifier(start Position, leading string) {
	l.advance()
	for !l.atEnd() {
		current, _ := l.peek()
		if !isIdentifierPart(current) {
			break
		}
		l.advance()
	}
	lexeme := l.source[start.Offset:l.offset]
	kind := Identifier
	if keyword, ok := keywords[strings.ToLower(lexeme)]; ok {
		kind = keyword
	}
	l.emit(kind, start, leading, lexeme, lexeme)
}

func (l *lexer) scanNumber(start Position, leading string) {
	for !l.atEnd() {
		current, _ := l.peek()
		if !unicode.IsDigit(current) {
			break
		}
		l.advance()
	}
	if current, ok := l.peek(); ok && current == '.' {
		if next, hasNext := l.peekNext(); hasNext && unicode.IsDigit(next) {
			l.advance()
			for !l.atEnd() {
				digit, _ := l.peek()
				if !unicode.IsDigit(digit) {
					break
				}
				l.advance()
			}
		}
	}
	lexeme := l.source[start.Offset:l.offset]
	l.emit(Number, start, leading, lexeme, lexeme)
}

func (l *lexer) scanString(start Position, leading string) {
	l.advance()
	var value strings.Builder
	for !l.atEnd() {
		current, _ := l.peek()
		if current == '"' {
			l.advance()
			if escaped, ok := l.peek(); ok && escaped == '"' {
				value.WriteRune('"')
				l.advance()
				continue
			}
			lexeme := l.source[start.Offset:l.offset]
			l.emit(String, start, leading, lexeme, value.String())
			return
		}
		if current == '\n' || current == '\r' {
			break
		}
		value.WriteRune(current)
		l.advance()
	}

	l.report(start, "BSL1001", "unterminated string literal")
	lexeme := l.source[start.Offset:l.offset]
	l.emit(Invalid, start, leading, lexeme, value.String())
}

func (l *lexer) scanSymbol(start Position, leading string, current rune) {
	l.advance()
	kind := Invalid
	switch current {
	case '+':
		kind = Plus
	case '-':
		kind = Minus
	case '*':
		kind = Star
	case '/':
		kind = Slash
	case '(':
		kind = LeftParen
	case ')':
		kind = RightParen
	case ',':
		kind = Comma
	case ';':
		kind = Semicolon
	}
	lexeme := l.source[start.Offset:l.offset]
	l.emit(kind, start, leading, lexeme, lexeme)
	if kind == Invalid {
		l.report(start, "BSL1002", "unexpected character "+lexeme)
	}
}

func (l *lexer) emit(kind Kind, start Position, leading, lexeme, value string) {
	l.tokens = append(l.tokens, Token{
		Kind: kind, Leading: leading, Lexeme: lexeme, Value: value,
		Span: Span{Start: start, End: l.position()},
	})
}

func (l *lexer) report(start Position, code, message string) {
	l.diagnostics = append(l.diagnostics, Diagnostic{
		Filename: l.filename, Code: code, Message: message,
		Span: Span{Start: start, End: l.position()},
	})
}

func (l *lexer) atEnd() bool { return l.offset >= len(l.source) }

func (l *lexer) position() Position {
	return Position{Offset: l.offset, Line: l.line, Column: l.column}
}

func (l *lexer) peek() (rune, bool) {
	if l.atEnd() {
		return 0, false
	}
	current, _ := utf8.DecodeRuneInString(l.source[l.offset:])
	return current, true
}

func (l *lexer) peekNext() (rune, bool) {
	if l.atEnd() {
		return 0, false
	}
	_, width := utf8.DecodeRuneInString(l.source[l.offset:])
	if l.offset+width >= len(l.source) {
		return 0, false
	}
	next, _ := utf8.DecodeRuneInString(l.source[l.offset+width:])
	return next, true
}

func (l *lexer) advance() rune {
	current, width := utf8.DecodeRuneInString(l.source[l.offset:])
	l.offset += width
	if current == '\r' {
		if next, ok := l.peek(); ok && next == '\n' {
			_, nextWidth := utf8.DecodeRuneInString(l.source[l.offset:])
			l.offset += nextWidth
		}
		l.line++
		l.column = 1
		return '\n'
	}
	if current == '\n' {
		l.line++
		l.column = 1
	} else {
		l.column++
	}
	return current
}

func isIdentifierStart(current rune) bool {
	return current == '_' || unicode.IsLetter(current)
}

func isIdentifierPart(current rune) bool {
	return isIdentifierStart(current) || unicode.IsDigit(current)
}
