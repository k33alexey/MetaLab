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
	Function
	EndFunction
	Procedure
	EndProcedure
	Export
	Return
	Plus
	Minus
	Star
	Slash
	LeftParen
	RightParen
	Comma
	Semicolon
)

var kindNames = [...]string{
	Invalid:      "invalid token",
	EOF:          "end of file",
	Identifier:   "identifier",
	Number:       "number",
	String:       "string",
	Function:     "Function",
	EndFunction:  "EndFunction",
	Procedure:    "Procedure",
	EndProcedure: "EndProcedure",
	Export:       "Export",
	Return:       "Return",
	Plus:         "+",
	Minus:        "-",
	Star:         "*",
	Slash:        "/",
	LeftParen:    "(",
	RightParen:   ")",
	Comma:        ",",
	Semicolon:    ";",
}

func (kind Kind) String() string {
	if int(kind) >= len(kindNames) || kindNames[kind] == "" {
		return fmt.Sprintf("token(%d)", kind)
	}
	return kindNames[kind]
}

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

// Token retains both syntax and leading trivia so source can be reconstructed.
type Token struct {
	Kind    Kind
	Leading string
	Lexeme  string
	Value   string
	Span    Span
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
