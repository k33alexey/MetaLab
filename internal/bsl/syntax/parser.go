package syntax

import (
	"fmt"
	"strings"
)

// Parse builds an AST and returns all frontend diagnostics.
func Parse(filename, source string) (*Module, []Diagnostic) {
	tokens, diagnostics := Tokenize(filename, source)
	state := parser{filename: filename, tokens: tokens, diagnostics: diagnostics}
	return state.parseModule(), state.diagnostics
}

type parser struct {
	filename    string
	tokens      []Token
	current     int
	diagnostics []Diagnostic
}

func (p *parser) parseModule() *Module {
	module := &Module{}
	for !p.atEnd() {
		if p.check(Invalid) {
			p.advance()
			continue
		}
		if p.check(Function) || p.check(Procedure) {
			if routine := p.parseRoutine(); routine != nil {
				module.Routines = append(module.Routines, routine)
			}
			continue
		}
		p.report(p.peek(), "BSL2001", fmt.Sprintf("unexpected %s, expected routine declaration", p.peek().Kind))
		p.advance()
	}
	return module
}

func (p *parser) parseRoutine() *Routine {
	start := p.advance()
	isFunction := start.Kind == Function
	name, ok := p.expect(Identifier, "expected routine name")
	if !ok {
		p.synchronizeRoutine(isFunction)
		return nil
	}
	if _, ok := p.expect(LeftParen, "expected '(' after routine name"); !ok {
		p.synchronizeRoutine(isFunction)
		return nil
	}

	parameters := p.parseParameters()
	if _, ok := p.expect(RightParen, "expected ')' after routine parameters"); !ok {
		p.synchronizeRoutine(isFunction)
		return nil
	}
	exported := p.match(Export)

	endKind := EndProcedure
	endName := "EndProcedure"
	if isFunction {
		endKind = EndFunction
		endName = "EndFunction"
	}
	body := make([]Statement, 0)
	var mismatchedEnd *Token
	for !p.check(endKind) && !p.atEnd() {
		if p.check(EndFunction) || p.check(EndProcedure) {
			unexpected := p.advance()
			p.report(unexpected, "BSL2002", "expected "+endName+", got "+unexpected.Kind.String())
			mismatchedEnd = &unexpected
			break
		}
		if statement := p.parseStatement(); statement != nil {
			body = append(body, statement)
		}
	}

	end := p.peek()
	if mismatchedEnd != nil {
		end = *mismatchedEnd
		p.match(Semicolon)
	} else if p.match(endKind) {
		end = p.previous()
		p.match(Semicolon)
	} else {
		p.report(p.peek(), "BSL2002", "expected "+endName+" before end of file")
	}

	return &Routine{
		Name: name.Value, Function: isFunction, Export: exported,
		Parameters: parameters, Body: body,
		SourceSpan: Span{Start: start.Span.Start, End: end.Span.End},
	}
}

func (p *parser) parseParameters() []Parameter {
	if p.check(RightParen) {
		return nil
	}
	var parameters []Parameter
	for {
		parameter, ok := p.expect(Identifier, "expected parameter name")
		if !ok {
			return parameters
		}
		parameters = append(parameters, Parameter{Name: parameter.Value, Span: parameter.Span})
		if !p.match(Comma) {
			return parameters
		}
	}
}

func (p *parser) parseStatement() Statement {
	switch p.peek().Kind {
	case Return:
		return p.parseReturnStatement()
	case If:
		return p.parseIfStatement()
	case While:
		return p.parseWhileStatement()
	case For:
		return p.parseForStatement()
	case Break:
		return p.parseLoopControlStatement(true)
	case Continue:
		return p.parseLoopControlStatement(false)
	case Identifier:
		if p.checkNext(Equal) {
			return p.parseAssignmentStatement()
		}
	}
	p.report(p.peek(), "BSL2001", fmt.Sprintf("unexpected %s, expected statement", p.peek().Kind))
	p.synchronizeStatement()
	return nil
}

func (p *parser) parseReturnStatement() Statement {
	p.advance()
	start := p.previous()
	var value Expression
	if canStartExpression(p.peek().Kind) {
		value = p.parseExpression()
	}
	end := start.Span.End
	if value != nil {
		end = value.NodeSpan().End
	}
	if semicolon, ok := p.expect(Semicolon, "expected ';' after Return"); ok {
		end = semicolon.Span.End
	}
	return &ReturnStatement{Value: value, SourceSpan: Span{Start: start.Span.Start, End: end}}
}

func (p *parser) parseAssignmentStatement() Statement {
	name := p.advance()
	p.advance() // Equal.
	value := p.parseExpression()
	end := name.Span.End
	if value != nil {
		end = value.NodeSpan().End
	}
	if semicolon, ok := p.expect(Semicolon, "expected ';' after assignment"); ok {
		end = semicolon.Span.End
	}
	return &AssignmentStatement{Name: name.Value, Value: value, SourceSpan: Span{Start: name.Span.Start, End: end}}
}

func (p *parser) parseIfStatement() Statement {
	start := p.advance()
	branches := make([]ConditionalBranch, 0, 2)
	condition := p.parseExpression()
	p.expect(Then, "expected Then after If condition")
	body := p.parseBlock(ElsIf, Else, EndIf)
	branches = append(branches, ConditionalBranch{
		Condition: condition, Body: body,
		SourceSpan: spanThroughBody(start.Span.Start, condition, body),
	})

	for p.match(ElsIf) {
		branchStart := p.previous()
		condition = p.parseExpression()
		p.expect(Then, "expected Then after ElsIf condition")
		body = p.parseBlock(ElsIf, Else, EndIf)
		branches = append(branches, ConditionalBranch{
			Condition: condition, Body: body,
			SourceSpan: spanThroughBody(branchStart.Span.Start, condition, body),
		})
	}

	var elseBody []Statement
	if p.match(Else) {
		elseBody = p.parseBlock(EndIf)
	}
	end := p.finishBlock(EndIf, "EndIf")
	return &IfStatement{Branches: branches, ElseBody: elseBody, SourceSpan: Span{Start: start.Span.Start, End: end}}
}

func (p *parser) parseWhileStatement() Statement {
	start := p.advance()
	condition := p.parseExpression()
	p.expect(Do, "expected Do after While condition")
	body := p.parseBlock(EndDo)
	end := p.finishBlock(EndDo, "EndDo")
	return &WhileStatement{Condition: condition, Body: body, SourceSpan: Span{Start: start.Span.Start, End: end}}
}

func (p *parser) parseForStatement() Statement {
	start := p.advance()
	if p.match(Each) {
		variable, _ := p.expect(Identifier, "expected iterator name after Each")
		p.expect(In, "expected In after iterator name")
		collection := p.parseExpression()
		p.expect(Do, "expected Do after collection")
		body := p.parseBlock(EndDo)
		end := p.finishBlock(EndDo, "EndDo")
		return &ForEachStatement{
			Variable: variable.Value, Collection: collection, Body: body,
			SourceSpan: Span{Start: start.Span.Start, End: end},
		}
	}

	variable, _ := p.expect(Identifier, "expected counter name after For")
	p.expect(Equal, "expected '=' after counter name")
	initial := p.parseExpression()
	p.expect(To, "expected To after initial counter value")
	limit := p.parseExpression()
	p.expect(Do, "expected Do after final counter value")
	body := p.parseBlock(EndDo)
	end := p.finishBlock(EndDo, "EndDo")
	return &ForStatement{
		Variable: variable.Value, Initial: initial, Limit: limit, Body: body,
		SourceSpan: Span{Start: start.Span.Start, End: end},
	}
}

func (p *parser) parseLoopControlStatement(breakStatement bool) Statement {
	start := p.advance()
	end := start.Span.End
	if semicolon, ok := p.expect(Semicolon, "expected ';' after "+start.Kind.String()); ok {
		end = semicolon.Span.End
	}
	span := Span{Start: start.Span.Start, End: end}
	if breakStatement {
		return &BreakStatement{SourceSpan: span}
	}
	return &ContinueStatement{SourceSpan: span}
}

func (p *parser) parseBlock(stops ...Kind) []Statement {
	var body []Statement
	for !p.atEnd() && !p.checkAny(stops...) && !p.check(EndFunction) && !p.check(EndProcedure) {
		if statement := p.parseStatement(); statement != nil {
			body = append(body, statement)
		}
	}
	return body
}

func (p *parser) finishBlock(kind Kind, name string) Position {
	end := p.peek().Span.End
	if token, ok := p.expect(kind, "expected "+name); ok {
		end = token.Span.End
		if semicolon, found := p.expect(Semicolon, "expected ';' after "+name); found {
			end = semicolon.Span.End
		}
	}
	return end
}

func spanThroughBody(start Position, expression Expression, body []Statement) Span {
	end := start
	if expression != nil {
		end = expression.NodeSpan().End
	}
	if len(body) != 0 {
		end = body[len(body)-1].NodeSpan().End
	}
	return Span{Start: start, End: end}
}

func (p *parser) parseExpression() Expression { return p.parseOr() }

func (p *parser) parseOr() Expression {
	return p.parseBinary(p.parseAnd, Or)
}

func (p *parser) parseAnd() Expression {
	return p.parseBinary(p.parseComparison, And)
}

func (p *parser) parseComparison() Expression {
	return p.parseBinary(p.parseAdditive, Equal, NotEqual, Less, LessEqual, Greater, GreaterEqual)
}

func (p *parser) parseAdditive() Expression {
	expression := p.parseMultiplicative()
	for p.check(Plus) || p.check(Minus) {
		operator := p.advance()
		right := p.parseMultiplicative()
		if expression == nil || right == nil {
			return expression
		}
		expression = &BinaryExpression{
			Left: expression, Operator: operator.Kind, Right: right,
			SourceSpan: Span{Start: expression.NodeSpan().Start, End: right.NodeSpan().End},
		}
	}
	return expression
}

func (p *parser) parseMultiplicative() Expression {
	expression := p.parseUnary()
	for p.check(Star) || p.check(Slash) || p.check(Percent) {
		operator := p.advance()
		right := p.parseUnary()
		if expression == nil || right == nil {
			return expression
		}
		expression = &BinaryExpression{
			Left: expression, Operator: operator.Kind, Right: right,
			SourceSpan: Span{Start: expression.NodeSpan().Start, End: right.NodeSpan().End},
		}
	}
	return expression
}

func (p *parser) parseUnary() Expression {
	if p.check(Plus) || p.check(Minus) || p.check(Not) {
		operator := p.advance()
		operand := p.parseUnary()
		if operand == nil {
			return nil
		}
		return &UnaryExpression{
			Operator: operator.Kind, Operand: operand,
			SourceSpan: Span{Start: operator.Span.Start, End: operand.NodeSpan().End},
		}
	}
	return p.parsePrimary()
}

func (p *parser) parsePrimary() Expression {
	current := p.peek()
	switch current.Kind {
	case Number:
		p.advance()
		return &NumberExpression{Text: current.Value, SourceSpan: current.Span}
	case String:
		p.advance()
		return &StringExpression{Value: current.Value, SourceSpan: current.Span}
	case StringStart:
		return p.parseMultilineString()
	case True, False:
		p.advance()
		return &BooleanExpression{Value: current.Kind == True, SourceSpan: current.Span}
	case Date:
		p.advance()
		return &DateExpression{Text: current.Value, SourceSpan: current.Span}
	case Undefined:
		p.advance()
		return &UndefinedExpression{SourceSpan: current.Span}
	case Null:
		p.advance()
		return &NullExpression{SourceSpan: current.Span}
	case Identifier:
		p.advance()
		return &IdentifierExpression{Name: current.Value, SourceSpan: current.Span}
	case LeftParen:
		left := p.advance()
		expression := p.parseExpression()
		right, ok := p.expect(RightParen, "expected ')' after expression")
		if !ok || expression == nil {
			return expression
		}
		return &GroupExpression{Expression: expression, SourceSpan: Span{Start: left.Span.Start, End: right.Span.End}}
	default:
		p.report(current, "BSL2001", fmt.Sprintf("unexpected %s, expected expression", current.Kind))
		if !p.atEnd() {
			p.advance()
		}
		return nil
	}
}

func (p *parser) parseBinary(operand func() Expression, kinds ...Kind) Expression {
	expression := operand()
	for p.checkAny(kinds...) {
		operator := p.advance()
		right := operand()
		if expression == nil || right == nil {
			return expression
		}
		expression = &BinaryExpression{
			Left: expression, Operator: operator.Kind, Right: right,
			SourceSpan: Span{Start: expression.NodeSpan().Start, End: right.NodeSpan().End},
		}
	}
	return expression
}

func (p *parser) parseMultilineString() Expression {
	start := p.advance()
	var value strings.Builder
	value.WriteString(start.Value)
	for p.check(StringPart) {
		value.WriteByte('\n')
		value.WriteString(p.advance().Value)
	}
	if !p.check(StringEnd) {
		p.report(p.peek(), "BSL2002", "expected multiline string end")
		return &StringExpression{Value: value.String(), SourceSpan: start.Span}
	}
	end := p.advance()
	value.WriteByte('\n')
	value.WriteString(end.Value)
	return &StringExpression{
		Value: value.String(), SourceSpan: Span{Start: start.Span.Start, End: end.Span.End},
	}
}

func (p *parser) synchronizeStatement() {
	for !p.atEnd() {
		if p.match(Semicolon) {
			return
		}
		if p.checkAny(ElsIf, Else, EndIf, EndDo) {
			p.advance()
			p.match(Semicolon)
			return
		}
		if p.checkAny(EndFunction, EndProcedure, Return, If, While, For, Break, Continue) {
			return
		}
		p.advance()
	}
}

func (p *parser) synchronizeRoutine(function bool) {
	end := EndProcedure
	if function {
		end = EndFunction
	}
	for !p.atEnd() {
		if p.match(end) {
			p.match(Semicolon)
			return
		}
		p.advance()
	}
}

func (p *parser) expect(kind Kind, message string) (Token, bool) {
	if p.check(kind) {
		return p.advance(), true
	}
	p.report(p.peek(), "BSL2002", message)
	return p.peek(), false
}

func (p *parser) match(kind Kind) bool {
	if !p.check(kind) {
		return false
	}
	p.advance()
	return true
}

func (p *parser) check(kind Kind) bool { return p.peek().Kind == kind }

func (p *parser) checkAny(kinds ...Kind) bool {
	for _, kind := range kinds {
		if p.check(kind) {
			return true
		}
	}
	return false
}

func (p *parser) checkNext(kind Kind) bool {
	return p.current+1 < len(p.tokens) && p.tokens[p.current+1].Kind == kind
}

func (p *parser) atEnd() bool { return p.peek().Kind == EOF }

func (p *parser) peek() Token { return p.tokens[p.current] }

func (p *parser) previous() Token {
	if p.current == 0 {
		return p.tokens[0]
	}
	return p.tokens[p.current-1]
}

func (p *parser) advance() Token {
	if !p.atEnd() {
		p.current++
	}
	return p.previous()
}

func (p *parser) report(current Token, code, message string) {
	p.diagnostics = append(p.diagnostics, Diagnostic{
		Filename: p.filename, Code: code, Message: message, Span: current.Span,
	})
}

func canStartExpression(kind Kind) bool {
	switch kind {
	case Number, String, StringStart, Identifier, True, False, Date, Undefined, Null, LeftParen, Plus, Minus, Not:
		return true
	default:
		return false
	}
}
