// Package syntax implements the MetaLab BSL frontend.
package syntax

import "fmt"

// Kind identifies a lexical token.
type Kind uint8

const (
	Invalid Kind = iota
	EOF
	Identifier
	Number
	String
	StringStart
	StringPart
	StringEnd
	Date

	Procedure
	EndProcedure
	Function
	EndFunction
	Var
	Export
	Val
	If
	Then
	ElsIf
	Else
	EndIf
	For
	Each
	In
	To
	Do
	EndDo
	While
	Try
	Except
	EndTry
	Raise
	Return
	Continue
	Break
	Goto
	Execute
	AddHandler
	RemoveHandler
	Async
	Await
	New
	Not
	And
	Or
	True
	False
	Undefined
	Null

	Plus
	Minus
	Star
	Slash
	Percent
	Equal
	NotEqual
	Less
	LessEqual
	Greater
	GreaterEqual
	LeftParen
	RightParen
	LeftBracket
	RightBracket
	Dot
	DotTrailing
	Comma
	Semicolon
	Colon
	Question
	Ampersand
	Hash
	Tilde
	Bar
)

const (
	firstKeyword = Procedure
	lastKeyword  = Null
)

var kindNames = [...]string{
	Invalid:       "invalid token",
	EOF:           "end of file",
	Identifier:    "identifier",
	Number:        "number",
	String:        "string",
	StringStart:   "multiline string start",
	StringPart:    "multiline string part",
	StringEnd:     "multiline string end",
	Date:          "date",
	Procedure:     "Procedure",
	EndProcedure:  "EndProcedure",
	Function:      "Function",
	EndFunction:   "EndFunction",
	Var:           "Var",
	Export:        "Export",
	Val:           "Val",
	If:            "If",
	Then:          "Then",
	ElsIf:         "ElsIf",
	Else:          "Else",
	EndIf:         "EndIf",
	For:           "For",
	Each:          "Each",
	In:            "In",
	To:            "To",
	Do:            "Do",
	EndDo:         "EndDo",
	While:         "While",
	Try:           "Try",
	Except:        "Except",
	EndTry:        "EndTry",
	Raise:         "Raise",
	Return:        "Return",
	Continue:      "Continue",
	Break:         "Break",
	Goto:          "Goto",
	Execute:       "Execute",
	AddHandler:    "AddHandler",
	RemoveHandler: "RemoveHandler",
	Async:         "Async",
	Await:         "Await",
	New:           "New",
	Not:           "Not",
	And:           "And",
	Or:            "Or",
	True:          "True",
	False:         "False",
	Undefined:     "Undefined",
	Null:          "Null",
	Plus:          "+",
	Minus:         "-",
	Star:          "*",
	Slash:         "/",
	Percent:       "%",
	Equal:         "=",
	NotEqual:      "<>",
	Less:          "<",
	LessEqual:     "<=",
	Greater:       ">",
	GreaterEqual:  ">=",
	LeftParen:     "(",
	RightParen:    ")",
	LeftBracket:   "[",
	RightBracket:  "]",
	Dot:           ".",
	DotTrailing:   "trailing .",
	Comma:         ",",
	Semicolon:     ";",
	Colon:         ":",
	Question:      "?",
	Ampersand:     "&",
	Hash:          "#",
	Tilde:         "~",
	Bar:           "|",
}

func (kind Kind) String() string {
	if int(kind) >= len(kindNames) || kindNames[kind] == "" {
		return fmt.Sprintf("token(%d)", kind)
	}
	return kindNames[kind]
}

// IsKeyword reports whether the token is a Russian/English BSL keyword.
func (kind Kind) IsKeyword() bool { return kind >= firstKeyword && kind <= lastKeyword }

// Position identifies a UTF-8 byte offset and a human-readable location.
type Position struct {
	Offset int
	Line   int
	Column int
}

// Span identifies a half-open source range.
type Span struct {
	Start Position
	End   Position
}

// TriviaKind identifies lossless syntax outside significant tokens.
type TriviaKind uint8

const (
	WhitespaceTrivia TriviaKind = iota + 1
	LineCommentTrivia
	ByteOrderMarkTrivia
)

// Trivia retains whitespace and comments for formatting, highlighting and IDE operations.
type Trivia struct {
	Kind   TriviaKind
	Lexeme string
	Span   Span
}

// Token retains both syntax and leading trivia so source can be reconstructed.
type Token struct {
	Kind          Kind
	Leading       string
	LeadingTrivia []Trivia
	Lexeme        string
	Value         string
	Span          Span
}

// Diagnostic describes one source error.
type Diagnostic struct {
	Filename string
	Code     string
	Message  string
	Span     Span
}

func (diagnostic Diagnostic) Error() string {
	return fmt.Sprintf(
		"%s:%d:%d: %s [%s]",
		diagnostic.Filename,
		diagnostic.Span.Start.Line,
		diagnostic.Span.Start.Column,
		diagnostic.Message,
		diagnostic.Code,
	)
}
