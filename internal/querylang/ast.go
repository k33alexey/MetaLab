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
	Joins    []Join
	Where    Expression
	Group    []Expression
	Having   Expression
	Order    []OrderField
	Into     *TemporaryTable
	IndexBy  []Expression
}

// SelectField describes one result column. Wildcard is expanded against metadata later.
type SelectField struct {
	Expression     Expression
	Alias          string
	Wildcard       bool
	WildcardSource []string
	Position       Position
}

// Source is one logical metadata table and its optional alias.
type Source struct {
	Path     []string
	Alias    string
	Position Position
}

// JoinKind identifies one supported relational join.
type JoinKind uint8

const (
	JoinInner JoinKind = iota + 1
	JoinLeft
	JoinRight
	JoinFull
	JoinCross
)

// Join adds one source to a query. Cross joins have no condition.
type Join struct {
	Kind      JoinKind
	Source    Source
	Condition Expression
	Position  Position
}

// TemporaryTable describes a table created by a SELECT ... INTO statement.
type TemporaryTable struct {
	Name     string
	Position Position
}

// DropTemporaryTable removes one table from a temporary table manager.
type DropTemporaryTable struct {
	Name     string
	Position Position
}

// Statement is one query package item.
type Statement struct {
	Query *Query
	Drop  *DropTemporaryTable
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

// Function is a query function call. Aggregate functions may use Distinct or Wildcard.
type Function struct {
	Name      string
	Arguments []Expression
	Distinct  bool
	Wildcard  bool
	Position  Position
}

func (Function) queryExpression()                   {}
func (value Function) ExpressionPosition() Position { return value.Position }
