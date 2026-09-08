package querylang

import (
	"fmt"
	"strconv"
	"unicode/utf8"
)

const (
	MaxTop               = 100_000
	MaxSourceBytes       = 1 << 20
	MaxTokens            = 100_000
	MaxResultFields      = 2048
	MaxOrderFields       = 1024
	MaxGroupFields       = 1024
	MaxJoins             = 64
	MaxFunctionArguments = 128
	MaxPackageStatements = 256
	MaxNesting           = 128
)

type parser struct {
	tokens  []token
	index   int
	nesting int
}

// Parse validates and parses one basic ML query.
func Parse(source string) (Query, error) {
	statements, err := ParsePackage(source)
	if err != nil {
		return Query{}, err
	}
	if len(statements) != 1 || statements[0].Query == nil {
		position := Position{Line: 1, Column: 1}
		if len(statements) > 1 {
			if statements[1].Query != nil {
				position = statements[1].Query.Source.Position
			} else {
				position = statements[1].Drop.Position
			}
		}
		return Query{}, queryError(position, "ожидался один запрос ВЫБРАТЬ")
	}
	return *statements[0].Query, nil
}

// ParsePackage validates and parses queries separated by semicolons.
func ParsePackage(source string) ([]Statement, error) {
	if !utf8.ValidString(source) || len(source) > MaxSourceBytes {
		return nil, fmt.Errorf("query source must be valid UTF-8 and not exceed %d bytes", MaxSourceBytes)
	}
	tokens, err := lex(source)
	if err != nil {
		return nil, err
	}
	if len(tokens) > MaxTokens {
		return nil, fmt.Errorf("query contains more than %d tokens", MaxTokens)
	}
	value := parser{tokens: tokens}
	if value.current().kind == tokenEOF {
		return nil, value.errorCurrent("ожидалось ВЫБРАТЬ или УНИЧТОЖИТЬ")
	}
	statements := make([]Statement, 0, 4)
	for value.current().kind != tokenEOF {
		var statement Statement
		if value.match(tokenDrop) {
			position := value.previous().position
			name, err := value.expect(tokenIdentifier, "после УНИЧТОЖИТЬ ожидалось имя временной таблицы")
			if err != nil {
				return nil, err
			}
			statement.Drop = &DropTemporaryTable{Name: name.text, Position: position}
		} else {
			query, err := value.parseQuery()
			if err != nil {
				return nil, err
			}
			statement.Query = &query
		}
		statements = append(statements, statement)
		if len(statements) > MaxPackageStatements {
			return nil, value.errorCurrent(fmt.Sprintf("пакет не может содержать более %d запросов", MaxPackageStatements))
		}
		if value.current().kind == tokenEOF {
			break
		}
		if !value.match(tokenSemicolon) {
			return nil, value.errorCurrent("между запросами пакета ожидался ;")
		}
		if value.current().kind == tokenEOF {
			break
		}
	}
	return statements, nil
}

func (value *parser) parseQuery() (Query, error) {
	if _, err := value.expect(tokenSelect, "ожидалось ВЫБРАТЬ"); err != nil {
		return Query{}, err
	}
	query := Query{Distinct: value.match(tokenDistinct)}
	var err error
	if value.match(tokenTop) {
		top, err := value.expect(tokenNumber, "после ПЕРВЫЕ ожидается целое число")
		if err != nil {
			return Query{}, err
		}
		parsed, parseErr := strconv.Atoi(top.text)
		if parseErr != nil || parsed < 1 || parsed > MaxTop {
			return Query{}, queryError(top.position, fmt.Sprintf("ПЕРВЫЕ должно быть целым числом от 1 до %d", MaxTop))
		}
		query.Top = parsed
	}
	for {
		field := SelectField{Position: value.current().position}
		if value.match(tokenStar) {
			field.Wildcard = true
		} else {
			path, wildcard, err := value.tryParseQualifiedWildcard()
			if err != nil {
				return Query{}, err
			}
			if wildcard {
				field.Wildcard = true
				field.WildcardSource = path
			} else {
				expression, err := value.parseValue()
				if err != nil {
					return Query{}, err
				}
				field.Expression = expression
				if value.match(tokenAs) {
					alias, err := value.expect(tokenIdentifier, "после КАК ожидается псевдоним поля")
					if err != nil {
						return Query{}, err
					}
					field.Alias = alias.text
				}
			}
		}
		query.Fields = append(query.Fields, field)
		if len(query.Fields) > MaxResultFields {
			return Query{}, queryError(field.Position, fmt.Sprintf("запрос не может содержать более %d полей результата", MaxResultFields))
		}
		if !value.match(tokenComma) {
			break
		}
	}
	if value.match(tokenInto) {
		name, err := value.expect(tokenIdentifier, "после ПОМЕСТИТЬ ожидалось имя временной таблицы")
		if err != nil {
			return Query{}, err
		}
		query.Into = &TemporaryTable{Name: name.text, Position: name.position}
	}
	if _, err := value.expect(tokenFrom, "ожидалось ИЗ и имя таблицы метаданных"); err != nil {
		return Query{}, err
	}
	query.Source, err = value.parseSource()
	if err != nil {
		return Query{}, err
	}
	for {
		kind, position, joined, err := value.parseJoinKind()
		if err != nil {
			return Query{}, err
		}
		if !joined {
			break
		}
		source, err := value.parseSource()
		if err != nil {
			return Query{}, err
		}
		join := Join{Kind: kind, Source: source, Position: position}
		if kind != JoinCross {
			if !value.match(tokenOn) && !value.match(tokenBy) {
				return Query{}, value.errorCurrent("после таблицы соединения ожидалось ПО")
			}
			join.Condition, err = value.parseOr()
			if err != nil {
				return Query{}, err
			}
		}
		query.Joins = append(query.Joins, join)
		if len(query.Joins) > MaxJoins {
			return Query{}, queryError(position, fmt.Sprintf("запрос не может содержать более %d соединений", MaxJoins))
		}
	}
	if value.match(tokenWhere) {
		query.Where, err = value.parseOr()
		if err != nil {
			return Query{}, err
		}
	}
	if value.match(tokenGroup) {
		if _, err := value.expect(tokenBy, "после СГРУППИРОВАТЬ ожидалось ПО"); err != nil {
			return Query{}, err
		}
		for {
			position := value.current().position
			expression, err := value.parseValue()
			if err != nil {
				return Query{}, err
			}
			query.Group = append(query.Group, expression)
			if len(query.Group) > MaxGroupFields {
				return Query{}, queryError(position, fmt.Sprintf("группировка не может содержать более %d полей", MaxGroupFields))
			}
			if !value.match(tokenComma) {
				break
			}
		}
	}
	if value.match(tokenHaving) {
		query.Having, err = value.parseOr()
		if err != nil {
			return Query{}, err
		}
	}
	if value.match(tokenOrder) {
		if _, err := value.expect(tokenBy, "после УПОРЯДОЧИТЬ ожидалось ПО"); err != nil {
			return Query{}, err
		}
		for {
			position := value.current().position
			expression, err := value.parseValue()
			if err != nil {
				return Query{}, err
			}
			item := OrderField{Expression: expression, Position: position}
			if value.match(tokenDescending) {
				item.Descending = true
			} else {
				value.match(tokenAscending)
			}
			query.Order = append(query.Order, item)
			if len(query.Order) > MaxOrderFields {
				return Query{}, queryError(position, fmt.Sprintf("сортировка не может содержать более %d полей", MaxOrderFields))
			}
			if !value.match(tokenComma) {
				break
			}
		}
	}
	if value.match(tokenIndex) {
		if _, err := value.expect(tokenBy, "после ИНДЕКСИРОВАТЬ ожидалось ПО"); err != nil {
			return Query{}, err
		}
		for {
			expression, err := value.parseValue()
			if err != nil {
				return Query{}, err
			}
			query.IndexBy = append(query.IndexBy, expression)
			if len(query.IndexBy) > MaxOrderFields {
				return Query{}, queryError(expression.ExpressionPosition(), fmt.Sprintf("индекс не может содержать более %d полей", MaxOrderFields))
			}
			if !value.match(tokenComma) {
				break
			}
		}
	}
	if len(query.IndexBy) != 0 && query.Into == nil {
		return Query{}, queryError(query.IndexBy[0].ExpressionPosition(), "ИНДЕКСИРОВАТЬ ПО допускается только вместе с ПОМЕСТИТЬ")
	}
	return query, nil
}

func (value *parser) parseSource() (Source, error) {
	path, position, err := value.parsePath()
	if err != nil {
		return Source{}, err
	}
	result := Source{Path: path, Position: position}
	if value.match(tokenAs) {
		alias, err := value.expect(tokenIdentifier, "после КАК ожидается псевдоним источника")
		if err != nil {
			return Source{}, err
		}
		result.Alias = alias.text
	} else if value.current().kind == tokenIdentifier {
		result.Alias = value.advance().text
	}
	return result, nil
}

func (value *parser) parseJoinKind() (JoinKind, Position, bool, error) {
	position := value.current().position
	if value.match(tokenComma) {
		return JoinCross, position, true, nil
	}
	kind := JoinInner
	switch {
	case value.match(tokenJoin):
		return kind, position, true, nil
	case value.match(tokenInner):
		kind = JoinInner
	case value.match(tokenLeft):
		kind = JoinLeft
	case value.match(tokenRight):
		kind = JoinRight
	case value.match(tokenFull):
		kind = JoinFull
	default:
		return 0, position, false, nil
	}
	if kind != JoinInner {
		value.match(tokenOuter)
	}
	if _, err := value.expect(tokenJoin, "ожидалось СОЕДИНЕНИЕ"); err != nil {
		return 0, position, false, err
	}
	return kind, position, true, nil
}

func (value *parser) parseOr() (Expression, error) {
	left, err := value.parseAnd()
	if err != nil {
		return nil, err
	}
	for value.match(tokenOr) {
		position := value.previous().position
		right, err := value.parseAnd()
		if err != nil {
			return nil, err
		}
		left = Binary{Operator: "or", Left: left, Right: right, Position: position}
	}
	return left, nil
}

func (value *parser) parseAnd() (Expression, error) {
	left, err := value.parseNot()
	if err != nil {
		return nil, err
	}
	for value.match(tokenAnd) {
		position := value.previous().position
		right, err := value.parseNot()
		if err != nil {
			return nil, err
		}
		left = Binary{Operator: "and", Left: left, Right: right, Position: position}
	}
	return left, nil
}

func (value *parser) parseNot() (Expression, error) {
	if value.match(tokenNot) {
		position := value.previous().position
		if err := value.enter(position); err != nil {
			return nil, err
		}
		defer value.leave()
		operand, err := value.parseNot()
		if err != nil {
			return nil, err
		}
		return Unary{Operator: "not", Operand: operand, Position: position}, nil
	}
	return value.parseComparison()
}

func (value *parser) parseComparison() (Expression, error) {
	left, err := value.parsePrimary()
	if err != nil {
		return nil, err
	}
	operator := ""
	operatorConsumed := false
	position := value.current().position
	switch value.current().kind {
	case tokenEqual:
		operator = "="
	case tokenNotEqual:
		operator = "<>"
	case tokenLess:
		operator = "<"
	case tokenLessEqual:
		operator = "<="
	case tokenGreater:
		operator = ">"
	case tokenGreaterEqual:
		operator = ">="
	case tokenIn:
		operator = "in"
	case tokenLike:
		operator = "like"
	case tokenIs:
		operator = "is"
	case tokenNot:
		value.advance()
		operatorConsumed = true
		if value.match(tokenIn) {
			operator = "not in"
		} else if value.match(tokenLike) {
			operator = "not like"
		} else {
			return nil, queryError(position, "после НЕ ожидалось В или ПОДОБНО")
		}
	}
	if operator == "" {
		return left, nil
	}
	if !operatorConsumed {
		value.advance()
	}
	if operator == "is" && value.match(tokenNot) {
		operator = "is not"
	}
	var right Expression
	if operator == "in" || operator == "not in" {
		if _, err := value.expect(tokenLeftParen, "после В ожидалась ("); err != nil {
			return nil, err
		}
		items := make([]Expression, 0, 4)
		for {
			item, err := value.parseValue()
			if err != nil {
				return nil, err
			}
			items = append(items, item)
			if !value.match(tokenComma) {
				break
			}
		}
		if _, err := value.expect(tokenRightParen, "ожидалась ) после списка В"); err != nil {
			return nil, err
		}
		right = List{Items: items, Position: position}
	} else {
		right, err = value.parsePrimary()
		if err != nil {
			return nil, err
		}
		if operator == "is" || operator == "is not" {
			literal, ok := right.(Literal)
			if !ok || literal.Kind != LiteralNull {
				return nil, queryError(right.ExpressionPosition(), "после ЕСТЬ допускается только NULL")
			}
		}
	}
	return Binary{Operator: operator, Left: left, Right: right, Position: position}, nil
}

func (value *parser) parsePrimary() (Expression, error) {
	if value.match(tokenLeftParen) {
		position := value.previous().position
		if err := value.enter(position); err != nil {
			return nil, err
		}
		defer value.leave()
		expression, err := value.parseOr()
		if err != nil {
			return nil, err
		}
		if _, err := value.expect(tokenRightParen, "ожидалась )"); err != nil {
			return nil, err
		}
		return expression, nil
	}
	return value.parseValue()
}

func (value *parser) parseValue() (Expression, error) {
	current := value.current()
	switch current.kind {
	case tokenIdentifier:
		if value.peekToken(1).kind == tokenLeftParen {
			return value.parseFunction()
		}
		path, position, err := value.parsePath()
		if err != nil {
			return nil, err
		}
		return Field{Path: path, Position: position}, nil
	case tokenParameter:
		value.advance()
		return Parameter{Name: current.text, Position: current.position}, nil
	case tokenString:
		value.advance()
		return Literal{Kind: LiteralString, Text: current.text, Position: current.position}, nil
	case tokenNumber:
		value.advance()
		return Literal{Kind: LiteralNumber, Text: current.text, Position: current.position}, nil
	case tokenMinus:
		value.advance()
		number, err := value.expect(tokenNumber, "после - ожидалось число")
		if err != nil {
			return nil, err
		}
		return Literal{Kind: LiteralNumber, Text: "-" + number.text, Position: current.position}, nil
	case tokenTrue:
		value.advance()
		return Literal{Kind: LiteralBoolean, Text: "true", Position: current.position}, nil
	case tokenFalse:
		value.advance()
		return Literal{Kind: LiteralBoolean, Text: "false", Position: current.position}, nil
	case tokenNull:
		value.advance()
		return Literal{Kind: LiteralNull, Position: current.position}, nil
	case tokenUndefined:
		value.advance()
		return Literal{Kind: LiteralUndefined, Position: current.position}, nil
	default:
		return nil, value.errorCurrent("ожидалось поле, параметр или литерал")
	}
}

func (value *parser) parseFunction() (Expression, error) {
	name := value.advance()
	if _, err := value.expect(tokenLeftParen, "после имени функции ожидалась ("); err != nil {
		return nil, err
	}
	if err := value.enter(name.position); err != nil {
		return nil, err
	}
	defer value.leave()
	result := Function{Name: name.text, Position: name.position, Distinct: value.match(tokenDistinct)}
	if value.match(tokenStar) {
		result.Wildcard = true
	} else if value.current().kind != tokenRightParen {
		for {
			argument, err := value.parseValue()
			if err != nil {
				return nil, err
			}
			result.Arguments = append(result.Arguments, argument)
			if len(result.Arguments) > MaxFunctionArguments {
				return nil, queryError(name.position, fmt.Sprintf("функция не может иметь более %d аргументов", MaxFunctionArguments))
			}
			if !value.match(tokenComma) {
				break
			}
		}
	}
	if _, err := value.expect(tokenRightParen, "ожидалась ) после аргументов функции"); err != nil {
		return nil, err
	}
	return result, nil
}

func (value *parser) tryParseQualifiedWildcard() ([]string, bool, error) {
	if value.current().kind != tokenIdentifier {
		return nil, false, nil
	}
	saved := value.index
	path := []string{value.advance().text}
	for value.match(tokenDot) {
		if value.match(tokenStar) {
			return path, true, nil
		}
		part, err := value.expect(tokenIdentifier, "после точки ожидалось имя или *")
		if err != nil {
			return nil, false, err
		}
		path = append(path, part.text)
	}
	value.index = saved
	return nil, false, nil
}

func (value *parser) parsePath() ([]string, Position, error) {
	first, err := value.expect(tokenIdentifier, "ожидался идентификатор")
	if err != nil {
		return nil, Position{}, err
	}
	path := []string{first.text}
	for value.match(tokenDot) {
		part, err := value.expect(tokenIdentifier, "после точки ожидался идентификатор")
		if err != nil {
			return nil, Position{}, err
		}
		path = append(path, part.text)
	}
	return path, first.position, nil
}

func (value *parser) current() token { return value.tokens[value.index] }
func (value *parser) peekToken(distance int) token {
	index := value.index + distance
	if index >= len(value.tokens) {
		return value.tokens[len(value.tokens)-1]
	}
	return value.tokens[index]
}
func (value *parser) previous() token {
	if value.index == 0 {
		return value.tokens[0]
	}
	return value.tokens[value.index-1]
}
func (value *parser) advance() token {
	current := value.current()
	if current.kind != tokenEOF {
		value.index++
	}
	return current
}
func (value *parser) match(kind tokenKind) bool {
	if value.current().kind != kind {
		return false
	}
	value.advance()
	return true
}
func (value *parser) expect(kind tokenKind, message string) (token, error) {
	if value.current().kind != kind {
		return token{}, value.errorCurrent(message)
	}
	return value.advance(), nil
}
func (value *parser) errorCurrent(message string) error {
	return queryError(value.current().position, message)
}

func (value *parser) enter(position Position) error {
	if value.nesting == MaxNesting {
		return queryError(position, fmt.Sprintf("глубина выражения превышает %d", MaxNesting))
	}
	value.nesting++
	return nil
}

func (value *parser) leave() { value.nesting-- }
