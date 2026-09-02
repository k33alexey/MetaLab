package syntax

// Module is one parsed BSL source file.
type Module struct {
	Routines []*Routine
}

// Routine is a procedure or function declaration.
type Routine struct {
	Name       string
	Function   bool
	Export     bool
	Parameters []Parameter
	Body       []Statement
	SourceSpan Span
}

// Parameter is one routine argument.
type Parameter struct {
	Name string
	Span Span
}

// Statement is a BSL executable statement.
type Statement interface {
	Node
	statement()
}

// Expression is a BSL value expression.
type Expression interface {
	Node
	expression()
}

// Node retains its source range.
type Node interface {
	NodeSpan() Span
}

// ReturnStatement exits a routine, optionally with a value.
type ReturnStatement struct {
	Value      Expression
	SourceSpan Span
}

func (*ReturnStatement) statement()          {}
func (node *ReturnStatement) NodeSpan() Span { return node.SourceSpan }

// IdentifierExpression references a parameter or local value.
type IdentifierExpression struct {
	Name       string
	SourceSpan Span
}

func (*IdentifierExpression) expression()         {}
func (node *IdentifierExpression) NodeSpan() Span { return node.SourceSpan }

// NumberExpression retains a decimal source representation.
type NumberExpression struct {
	Text       string
	SourceSpan Span
}

func (*NumberExpression) expression()         {}
func (node *NumberExpression) NodeSpan() Span { return node.SourceSpan }

// StringExpression is a decoded string literal.
type StringExpression struct {
	Value      string
	SourceSpan Span
}

func (*StringExpression) expression()         {}
func (node *StringExpression) NodeSpan() Span { return node.SourceSpan }

// GroupExpression preserves a parenthesized expression source range.
type GroupExpression struct {
	Expression Expression
	SourceSpan Span
}

func (*GroupExpression) expression()         {}
func (node *GroupExpression) NodeSpan() Span { return node.SourceSpan }

// UnaryExpression applies an operator to one value.
type UnaryExpression struct {
	Operator   Kind
	Operand    Expression
	SourceSpan Span
}

func (*UnaryExpression) expression()         {}
func (node *UnaryExpression) NodeSpan() Span { return node.SourceSpan }

// BinaryExpression applies an operator to two values.
type BinaryExpression struct {
	Left       Expression
	Operator   Kind
	Right      Expression
	SourceSpan Span
}

func (*BinaryExpression) expression()         {}
func (node *BinaryExpression) NodeSpan() Span { return node.SourceSpan }
