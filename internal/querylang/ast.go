package querylang

// Position identifies one rune in query source using one-based line and column numbers.
type Position struct {
	Offset int
	Line   int
	Column int
}

// Query is the parsed basic ML query representation.
type Query struct {
	Distinct bool
	Top      int
	Fields   []SelectField
	Source   Source
	Where    Expression
	Order    []OrderField
}

// SelectField describes one result column. Wildcard is expanded against metadata later.
type SelectField struct {
	Expression Expression
	Alias      string
	Wildcard   bool
	Position   Position
}

// Source is one logical metadata table and its optional alias.
type Source struct {
	Path     []string
	Alias    string
	Position Position
}

// OrderField describes one expression used for result ordering.
type OrderField struct {
	Expression Expression
	Descending bool
	Position   Position
}

// Expression is a safe query expression produced only by the parser.
type Expression interface {
	queryExpression()
	ExpressionPosition() Position
}

type Field struct {
	Path     []string
	Position Position
}

func (Field) queryExpression()                   {}
func (value Field) ExpressionPosition() Position { return value.Position }

type Parameter struct {
	Name     string
	Position Position
}

func (Parameter) queryExpression()                   {}
func (value Parameter) ExpressionPosition() Position { return value.Position }

type LiteralKind uint8

const (
	LiteralString LiteralKind = iota + 1
	LiteralNumber
	LiteralBoolean
	LiteralNull
	LiteralUndefined
)

type Literal struct {
	Kind     LiteralKind
	Text     string
	Position Position
}

func (Literal) queryExpression()                   {}
func (value Literal) ExpressionPosition() Position { return value.Position }

type Unary struct {
	Operator string
	Operand  Expression
	Position Position
}

func (Unary) queryExpression()                   {}
func (value Unary) ExpressionPosition() Position { return value.Position }

type Binary struct {
	Operator string
	Left     Expression
	Right    Expression
	Position Position
}

func (Binary) queryExpression()                   {}
func (value Binary) ExpressionPosition() Position { return value.Position }

type List struct {
	Items    []Expression
	Position Position
}

func (List) queryExpression()                   {}
func (value List) ExpressionPosition() Position { return value.Position }
