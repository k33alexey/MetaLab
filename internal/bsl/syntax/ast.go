package syntax

// Module is one parsed BSL source file.
type Module struct {
	Variables []Variable
	Routines  []*Routine
}

// Variable is one module-level or local declaration.
type Variable struct {
	Name       string
	Export     bool
	SourceSpan Span
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
	Name       string
	ByValue    bool
	Default    Expression
	SourceSpan Span
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

// AssignmentStatement assigns a value to a local identifier.
type AssignmentStatement struct {
	Qualifier  string
	Name       string
	Value      Expression
	SourceSpan Span
}

// VariableStatement declares local routine variables.
type VariableStatement struct {
	Variables  []Variable
	SourceSpan Span
}

func (*VariableStatement) statement()          {}
func (node *VariableStatement) NodeSpan() Span { return node.SourceSpan }

// CallStatement invokes a procedure or discards a function result.
type CallStatement struct {
	Call       *CallExpression
	SourceSpan Span
}

func (*CallStatement) statement()          {}
func (node *CallStatement) NodeSpan() Span { return node.SourceSpan }

// TryStatement executes ExceptBody when Body raises a runtime exception.
type TryStatement struct {
	Body       []Statement
	ExceptBody []Statement
	SourceSpan Span
}

func (*TryStatement) statement()          {}
func (node *TryStatement) NodeSpan() Span { return node.SourceSpan }

// RaiseStatement creates an exception or rethrows the current one when Value is nil.
type RaiseStatement struct {
	Value      Expression
	SourceSpan Span
}

func (*RaiseStatement) statement()          {}
func (node *RaiseStatement) NodeSpan() Span { return node.SourceSpan }

func (*AssignmentStatement) statement()          {}
func (node *AssignmentStatement) NodeSpan() Span { return node.SourceSpan }

// ConditionalBranch is one If or ElsIf branch.
type ConditionalBranch struct {
	Condition  Expression
	Body       []Statement
	SourceSpan Span
}

// IfStatement executes the first matching branch or its optional Else body.
type IfStatement struct {
	Branches   []ConditionalBranch
	ElseBody   []Statement
	SourceSpan Span
}

func (*IfStatement) statement()          {}
func (node *IfStatement) NodeSpan() Span { return node.SourceSpan }

// WhileStatement repeats its body while the condition is true.
type WhileStatement struct {
	Condition  Expression
	Body       []Statement
	SourceSpan Span
}

func (*WhileStatement) statement()          {}
func (node *WhileStatement) NodeSpan() Span { return node.SourceSpan }

// ForStatement iterates a numeric local variable over an inclusive range.
type ForStatement struct {
	Variable   string
	Initial    Expression
	Limit      Expression
	Body       []Statement
	SourceSpan Span
}

func (*ForStatement) statement()          {}
func (node *ForStatement) NodeSpan() Span { return node.SourceSpan }

// ForEachStatement iterates a local variable over a collection expression.
type ForEachStatement struct {
	Variable   string
	Collection Expression
	Body       []Statement
	SourceSpan Span
}

func (*ForEachStatement) statement()          {}
func (node *ForEachStatement) NodeSpan() Span { return node.SourceSpan }

// BreakStatement exits the nearest enclosing loop.
type BreakStatement struct{ SourceSpan Span }

func (*BreakStatement) statement()          {}
func (node *BreakStatement) NodeSpan() Span { return node.SourceSpan }

// ContinueStatement advances the nearest enclosing loop.
type ContinueStatement struct{ SourceSpan Span }

func (*ContinueStatement) statement()          {}
func (node *ContinueStatement) NodeSpan() Span { return node.SourceSpan }

// IdentifierExpression references a parameter or local value.
type IdentifierExpression struct {
	Qualifier  string
	Name       string
	SourceSpan Span
}

// CallArgument is one supplied or deliberately omitted positional argument.
type CallArgument struct {
	Value      Expression
	SourceSpan Span
}

// CallExpression invokes a function. The same node is used by CallStatement.
type CallExpression struct {
	Qualifier  string
	Name       string
	Arguments  []CallArgument
	SourceSpan Span
}

func (*CallExpression) expression()         {}
func (node *CallExpression) NodeSpan() Span { return node.SourceSpan }

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

// BooleanExpression is a True or False literal.
type BooleanExpression struct {
	Value      bool
	SourceSpan Span
}

func (*BooleanExpression) expression()         {}
func (node *BooleanExpression) NodeSpan() Span { return node.SourceSpan }

// DateExpression retains the compact BSL date literal payload.
type DateExpression struct {
	Text       string
	SourceSpan Span
}

func (*DateExpression) expression()         {}
func (node *DateExpression) NodeSpan() Span { return node.SourceSpan }

// UndefinedExpression is the Undefined literal.
type UndefinedExpression struct{ SourceSpan Span }

func (*UndefinedExpression) expression()         {}
func (node *UndefinedExpression) NodeSpan() Span { return node.SourceSpan }

// NullExpression is the database NULL literal.
type NullExpression struct{ SourceSpan Span }

func (*NullExpression) expression()         {}
func (node *NullExpression) NodeSpan() Span { return node.SourceSpan }

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
