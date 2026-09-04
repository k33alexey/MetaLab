package syntax

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

var keywords = map[string]Kind{
	"процедура": Procedure, "procedure": Procedure,
	"конецпроцедуры": EndProcedure, "endprocedure": EndProcedure,
	"функция": Function, "function": Function,
	"конецфункции": EndFunction, "endfunction": EndFunction,
	"перем": Var, "var": Var,
	"экспорт": Export, "export": Export,
	"знач": Val, "val": Val,
	"если": If, "if": If,
	"тогда": Then, "then": Then,
	"иначеесли": ElsIf, "elsif": ElsIf,
	"иначе": Else, "else": Else,
	"конецесли": EndIf, "endif": EndIf,
	"для": For, "for": For,
	"каждого": Each, "each": Each,
	"из": In, "in": In,
	"по": To, "to": To,
	"цикл": Do, "do": Do,
	"конеццикла": EndDo, "enddo": EndDo,
	"пока": While, "while": While,
	"попытка": Try, "try": Try,
	"исключение": Except, "except": Except,
	"конецпопытки": EndTry, "endtry": EndTry,
	"вызватьисключение": Raise, "raise": Raise,
	"возврат": Return, "return": Return,
	"продолжить": Continue, "continue": Continue,
	"прервать": Break, "break": Break,
	"перейти": Goto, "goto": Goto,
	"выполнить": Execute, "execute": Execute,
	"добавитьобработчик": AddHandler, "addhandler": AddHandler,
	"удалитьобработчик": RemoveHandler, "removehandler": RemoveHandler,
	"асинх": Async, "async": Async,
	"ждать": Await, "await": Await,
	"новый": New, "new": New,
	"не": Not, "not": Not,
	"и": And, "and": And,
	"или": Or, "or": Or,
	"истина": True, "true": True,
	"ложь": False, "false": False,
	"неопределено": Undefined, "undefined": Undefined,
	"null": Null,
}

// KeywordKind returns the canonical token kind of a case-insensitive BSL keyword.
func KeywordKind(value string) (Kind, bool) {
	kind, ok := keywords[strings.ToLower(value)]
	return kind, ok
}

// Tokenize converts one BSL source file into lossless tokens.
func Tokenize(filename, source string) ([]Token, []Diagnostic) {
	tokenCapacity := min(len(source)/8+1, 16_384)
	triviaCapacity := min(len(source)/16+1, 8_192)
	state := lexer{
		filename: filename, source: source, line: 1, column: 1,
		tokens: make([]Token, 0, tokenCapacity), trivia: make([]Trivia, 0, triviaCapacity),
		diagnostics: make([]Diagnostic, 0, 4),
	}
	return state.run()
}

type lexer struct {
	filename        string
	source          string
	offset          int
	line            int
	column          int
	tokens          []Token
	trivia          []Trivia
	diagnostics     []Diagnostic
	plainIdentifier bool
	multilineStart  *Position
	afterHash       bool
}

func (l *lexer) run() ([]Token, []Diagnostic) {
	for {
		triviaStart := l.offset
		leadingTrivia := l.scanTrivia()
		leading := l.source[triviaStart:l.offset]
		if l.atEnd() {
			position := l.position()
			if l.multilineStart != nil {
				l.diagnostics = append(l.diagnostics, Diagnostic{
					Filename: l.filename, Code: "BSL1001", Message: "unterminated multiline string literal",
					Span: Span{Start: *l.multilineStart, End: position},
				})
				l.multilineStart = nil
			}
			l.tokens = append(l.tokens, Token{
				Kind: EOF, Leading: leading, LeadingTrivia: leadingTrivia,
				Span: Span{Start: position, End: position},
			})
			return l.tokens, l.diagnostics
		}

		start := l.position()
		current, _ := l.peek()
		if l.plainIdentifier && !isIdentifierStart(current) {
			l.plainIdentifier = false
		}
		if l.afterHash && !isIdentifierStart(current) {
			l.afterHash = false
		}
		switch {
		case current == utf8.RuneError && l.currentRuneWidth() == 1:
			l.scanInvalidUTF8(start, leading, leadingTrivia)
		case isIdentifierStart(current):
			l.scanIdentifier(start, leading, leadingTrivia)
		case isASCIIDigit(current):
			l.scanNumber(start, leading, leadingTrivia)
		case current == '"':
			l.scanString(start, leading, leadingTrivia)
		case current == '\'':
			l.scanDate(start, leading, leadingTrivia)
		case current == '|':
			l.scanStringContinuation(start, leading, leadingTrivia)
		default:
			l.scanSymbol(start, leading, leadingTrivia, current)
		}
	}
}

func (l *lexer) scanTrivia() []Trivia {
	first := len(l.trivia)
	for !l.atEnd() {
		start := l.position()
		if l.offset == 0 && strings.HasPrefix(l.source, "\ufeff") {
			l.offset += len("\ufeff")
			l.trivia = append(l.trivia, Trivia{
				Kind: ByteOrderMarkTrivia, Lexeme: "\ufeff", Span: Span{Start: start, End: l.position()},
			})
			continue
		}
		current, _ := l.peek()
		if isWhitespace(current) {
			for !l.atEnd() {
				current, _ = l.peek()
				if !isWhitespace(current) {
					break
				}
				l.advance()
			}
			l.trivia = append(l.trivia, Trivia{
				Kind: WhitespaceTrivia, Lexeme: l.source[start.Offset:l.offset], Span: Span{Start: start, End: l.position()},
			})
			continue
		}
		if current == '/' {
			next, ok := l.peekNext()
			if ok && next == '/' {
				l.advance()
				l.advance()
				for !l.atEnd() {
					commentRune, _ := l.peek()
					if commentRune == '\n' || commentRune == '\r' {
						break
					}
					l.advance()
				}
				l.trivia = append(l.trivia, Trivia{
					Kind: LineCommentTrivia, Lexeme: l.source[start.Offset:l.offset], Span: Span{Start: start, End: l.position()},
				})
				continue
			}
		}
		if first == len(l.trivia) {
			return nil
		}
		return l.trivia[first:]
	}
	if first == len(l.trivia) {
		return nil
	}
	return l.trivia[first:]
}

func (l *lexer) scanIdentifier(start Position, leading string, trivia []Trivia) {
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
	forced := l.plainIdentifier
	if !forced {
		if keyword, ok := KeywordKind(lexeme); ok {
			kind = keyword
		}
	}
	l.plainIdentifier = false
	if l.afterHash {
		l.afterHash = false
		folded := strings.ToLower(lexeme)
		if folded == "область" || folded == "region" {
			l.plainIdentifier = true
		}
	}
	l.emit(kind, start, leading, trivia, lexeme, lexeme)
}

func (l *lexer) scanNumber(start Position, leading string, trivia []Trivia) {
	for !l.atEnd() {
		current, _ := l.peek()
		if !isASCIIDigit(current) {
			break
		}
		l.advance()
	}
	if current, ok := l.peek(); ok && current == '.' {
		l.advance()
		for !l.atEnd() {
			digit, _ := l.peek()
			if !isASCIIDigit(digit) {
				break
			}
			l.advance()
		}
	}
	lexeme := l.source[start.Offset:l.offset]
	l.emit(Number, start, leading, trivia, lexeme, lexeme)
}

func (l *lexer) scanString(start Position, leading string, trivia []Trivia) {
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
			l.emit(String, start, leading, trivia, lexeme, value.String())
			return
		}
		if current == '\n' || current == '\r' {
			lexeme := l.source[start.Offset:l.offset]
			l.emit(StringStart, start, leading, trivia, lexeme, value.String())
			if l.multilineStart == nil {
				copy := start
				l.multilineStart = &copy
			}
			return
		}
		value.WriteRune(current)
		l.advance()
	}

	lexeme := l.source[start.Offset:l.offset]
	l.emit(StringStart, start, leading, trivia, lexeme, value.String())
	if l.multilineStart == nil {
		copy := start
		l.multilineStart = &copy
	}
}

func (l *lexer) scanStringContinuation(start Position, leading string, trivia []Trivia) {
	l.advance()
	if l.multilineStart == nil {
		l.emit(Bar, start, leading, trivia, l.source[start.Offset:l.offset], "|")
		return
	}
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
			l.emit(StringEnd, start, leading, trivia, l.source[start.Offset:l.offset], value.String())
			l.multilineStart = nil
			return
		}
		if current == '\n' || current == '\r' {
			l.emit(StringPart, start, leading, trivia, l.source[start.Offset:l.offset], value.String())
			return
		}
		value.WriteRune(current)
		l.advance()
	}
	l.emit(StringPart, start, leading, trivia, l.source[start.Offset:l.offset], value.String())
}

func (l *lexer) scanDate(start Position, leading string, trivia []Trivia) {
	l.advance()
	valueStart := l.offset
	for !l.atEnd() {
		current, _ := l.peek()
		if current == '\'' {
			value := l.source[valueStart:l.offset]
			l.advance()
			lexeme := l.source[start.Offset:l.offset]
			if !validDateLiteral(value) {
				l.report(start, "BSL1004", "invalid date literal")
				l.emit(Invalid, start, leading, trivia, lexeme, value)
				return
			}
			l.emit(Date, start, leading, trivia, lexeme, value)
			return
		}
		if current == '\n' || current == '\r' {
			break
		}
		l.advance()
	}
	value := l.source[valueStart:l.offset]
	l.report(start, "BSL1003", "unterminated date literal")
	l.emit(Invalid, start, leading, trivia, l.source[start.Offset:l.offset], value)
}

func validDateLiteral(value string) bool {
	for _, current := range value {
		if !isASCIIDigit(current) {
			return false
		}
	}
	return true
}

func (l *lexer) scanSymbol(start Position, leading string, trivia []Trivia, current rune) {
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
	case '%':
		kind = Percent
	case '=':
		kind = Equal
	case '<':
		kind = Less
		if next, ok := l.peek(); ok && next == '=' {
			l.advance()
			kind = LessEqual
		} else if ok && next == '>' {
			l.advance()
			kind = NotEqual
		}
	case '>':
		kind = Greater
		if next, ok := l.peek(); ok && next == '=' {
			l.advance()
			kind = GreaterEqual
		}
	case '(':
		kind = LeftParen
	case ')':
		kind = RightParen
	case '[':
		kind = LeftBracket
	case ']':
		kind = RightBracket
	case '.':
		if l.dotIsTrailing() {
			kind = DotTrailing
		} else {
			kind = Dot
			if next, ok := l.peek(); ok && isIdentifierStart(next) {
				l.plainIdentifier = true
			}
		}
	case ',':
		kind = Comma
	case ';':
		kind = Semicolon
	case ':':
		kind = Colon
	case '?':
		kind = Question
	case '&':
		kind = Ampersand
		l.plainIdentifier = true
	case '#':
		kind = Hash
		l.afterHash = true
	case '~':
		kind = Tilde
		l.plainIdentifier = true
	}
	lexeme := l.source[start.Offset:l.offset]
	l.emit(kind, start, leading, trivia, lexeme, lexeme)
	if kind == Invalid {
		l.report(start, "BSL1002", "unexpected character "+lexeme)
	}
}

func (l *lexer) dotIsTrailing() bool {
	position := l.offset
	for position < len(l.source) {
		switch l.source[position] {
		case ' ', '\t', '\f':
			position++
		case '\n', '\r':
			return true
		case '/':
			return position+1 < len(l.source) && l.source[position+1] == '/'
		default:
			return false
		}
	}
	return false
}

func (l *lexer) scanInvalidUTF8(start Position, leading string, trivia []Trivia) {
	l.advance()
	lexeme := l.source[start.Offset:l.offset]
	l.emit(Invalid, start, leading, trivia, lexeme, lexeme)
}

func (l *lexer) emit(kind Kind, start Position, leading string, trivia []Trivia, lexeme, value string) {
	l.tokens = append(l.tokens, Token{
		Kind: kind, Leading: leading, LeadingTrivia: trivia, Lexeme: lexeme, Value: value,
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

func (l *lexer) currentRuneWidth() int {
	if l.atEnd() {
		return 0
	}
	_, width := utf8.DecodeRuneInString(l.source[l.offset:])
	return width
}

func (l *lexer) advance() rune {
	start := l.position()
	current, width := utf8.DecodeRuneInString(l.source[l.offset:])
	l.offset += width
	if current == utf8.RuneError && width == 1 {
		l.column++
		l.diagnostics = append(l.diagnostics, Diagnostic{
			Filename: l.filename, Code: "BSL1005", Message: "invalid UTF-8 encoding",
			Span: Span{Start: start, End: l.position()},
		})
		return current
	}
	if current == '\r' {
		if next, ok := l.peek(); ok && next == '\n' {
			_, nextWidth := utf8.DecodeRuneInString(l.source[l.offset:])
			l.offset += nextWidth
		}
		l.line++
		l.column = 1
		l.plainIdentifier = false
		l.afterHash = false
		return '\n'
	}
	if current == '\n' {
		l.line++
		l.column = 1
		l.plainIdentifier = false
		l.afterHash = false
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

func isASCIIDigit(current rune) bool { return current >= '0' && current <= '9' }

func isWhitespace(current rune) bool {
	return current == ' ' || current == '\t' || current == '\f' || current == '\r' || current == '\n'
}
