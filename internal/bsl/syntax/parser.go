package syntax

import "fmt"

// Parse builds an AST and returns all diagnostics found by the prototype frontend.
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
	if !p.match(Return) {
		p.report(p.peek(), "BSL2001", fmt.Sprintf("unexpected %s, expected Return", p.peek().Kind))
		p.synchronizeStatement()
		return nil
	}
	start := p.previous()
	var value Expression
	if !p.check(Semicolon) && !p.check(EndFunction) && !p.check(EndProcedure) && !p.atEnd() {
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

func (p *parser) parseExpression() Expression { return p.parseAdditive() }

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
	for p.check(Star) || p.check(Slash) {
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
	if p.check(Plus) || p.check(Minus) {
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

func (p *parser) synchronizeStatement() {
	for !p.atEnd() {
		if p.match(Semicolon) || p.check(EndFunction) || p.check(EndProcedure) {
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
