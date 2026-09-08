package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/querylang"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

const (
	maxQueryTextBytes   = 1 << 20
	maxQueryParameters  = 1024
	maxQueryArguments   = 32_000
	maxQueryResultRows  = querylang.MaxTop
	maxQueryResultBytes = 64 << 20
)

type queryColumnKind uint8

const (
	queryStoredColumn queryColumnKind = iota
	queryRecorderColumn
	queryMovementKindColumn
)

type queryColumn struct {
	name       string
	sql        string
	kind       queryColumnKind
	storage    attributeStorage
	types      []Type
	metadataID uuid.UUID
}

type querySource struct {
	table       string
	logicalName string
	alias       string
	columns     []queryColumn
	byName      map[string]int
}

type queryOutput struct {
	name   string
	column *queryColumn
	fixed  *bytecode.Value
}

type queryCompiler struct {
	runtime    *Runtime
	source     querySource
	parameters map[string]bytecode.Value
	arguments  []any
	outputs    []queryOutput
	aliases    map[string]int
}

func (runtime *Runtime) executeQuery(ctx context.Context, text string, parameters map[string]bytecode.Value) (*queryResultObject, error) {
	if !utf8.ValidString(text) || strings.TrimSpace(text) == "" || len(text) > maxQueryTextBytes {
		return nil, fmt.Errorf("query text must contain 1..%d UTF-8 bytes", maxQueryTextBytes)
	}
	parsed, err := querylang.Parse(text)
	if err != nil {
		return nil, err
	}
	source, err := runtime.resolveQuerySource(parsed.Source)
	if err != nil {
		return nil, err
	}
	compiler := queryCompiler{
		runtime: runtime, source: source, parameters: parameters,
		aliases: make(map[string]int),
	}
	statement, err := compiler.compile(parsed)
	if err != nil {
		return nil, err
	}
	pool, err := runtime.databasePool()
	if err != nil {
		return nil, err
	}
	query, err := queryData(ctx, pool)
	if err != nil {
		return nil, err
	}
	rows, err := query.Query(ctx, statement, compiler.arguments...)
	if err != nil {
		return nil, recordDataError(ctx, pool, fmt.Errorf("execute query: %w", err))
	}
	defer rows.Close()
	result := &queryResultObject{runtime: runtime, columns: make([]string, len(compiler.outputs))}
	for index := range compiler.outputs {
		result.columns[index] = compiler.outputs[index].name
	}
	used := uint64(256)
	for rows.Next() {
		if len(result.rows) == maxQueryResultRows {
			return nil, fmt.Errorf("query result exceeds %d rows; use ПЕРВЫЕ", maxQueryResultRows)
		}
		raw := make([]json.RawMessage, len(compiler.outputs))
		targets := make([]any, len(raw))
		for index := range raw {
			targets[index] = &raw[index]
		}
		if err := rows.Scan(targets...); err != nil {
			return nil, fmt.Errorf("scan query result: %w", err)
		}
		values := make([]bytecode.Value, len(raw))
		for index := range raw {
			values[index], err = compiler.decodeOutput(compiler.outputs[index], raw[index])
			if err != nil {
				return nil, fmt.Errorf("decode query field %s: %w", compiler.outputs[index].name, err)
			}
			memory, ok := values[index].DynamicMemory(maxQueryResultBytes - min(used, maxQueryResultBytes))
			if !ok || memory > maxQueryResultBytes-used {
				return nil, fmt.Errorf("query result exceeds %d MiB", maxQueryResultBytes>>20)
			}
			used += memory + uint64(len(compiler.outputs[index].name)) + 32
			if used > maxQueryResultBytes {
				return nil, fmt.Errorf("query result exceeds %d MiB", maxQueryResultBytes>>20)
			}
		}
		result.rows = append(result.rows, values)
	}
	if err := rows.Err(); err != nil {
		return nil, recordDataError(ctx, pool, fmt.Errorf("read query result: %w", err))
	}
	return result, nil
}

func (compiler *queryCompiler) compile(query querylang.Query) (string, error) {
	selectSQL := make([]string, 0, len(query.Fields))
	for _, selected := range query.Fields {
		if selected.Wildcard {
			for index := range compiler.source.columns {
				column := compiler.source.columns[index]
				if err := compiler.appendOutput(&selectSQL, column.name, column.sql, &column, nil); err != nil {
					return "", err
				}
			}
			continue
		}
		expressionSQL, column, fixed, err := compiler.compileValue(selected.Expression, nil)
		if err != nil {
			return "", err
		}
		name := selected.Alias
		if name == "" {
			switch expression := selected.Expression.(type) {
			case querylang.Field:
				name = expression.Path[len(expression.Path)-1]
			case querylang.Parameter:
				name = expression.Name
			default:
				name = fmt.Sprintf("Поле%d", len(compiler.outputs)+1)
			}
		}
		if err := compiler.appendOutput(&selectSQL, name, expressionSQL, column, fixed); err != nil {
			return "", err
		}
	}
	if len(selectSQL) == 0 {
		return "", fmt.Errorf("query must select at least one field")
	}
	predicate := ""
	if query.Where != nil {
		var err error
		predicate, err = compiler.compilePredicate(query.Where)
		if err != nil {
			return "", err
		}
	}
	outerOrder := make([]string, 0, len(query.Order))
	order := make([]string, 0, len(query.Order))
	if len(query.Order) != 0 {
		for index, item := range query.Order {
			orderSQL, outerSQL, err := compiler.compileOrder(item.Expression, query.Distinct, &selectSQL, index)
			if err != nil {
				return "", err
			}
			if item.Descending {
				orderSQL += " DESC"
				outerSQL += " DESC"
			} else {
				orderSQL += " ASC"
				outerSQL += " ASC"
			}
			order = append(order, orderSQL)
			outerOrder = append(outerOrder, outerSQL)
		}
	}
	var statement strings.Builder
	statement.WriteString("SELECT ")
	if query.Distinct {
		statement.WriteString("DISTINCT ")
	}
	statement.WriteString(strings.Join(selectSQL, ", "))
	statement.WriteString(" FROM ")
	statement.WriteString(qualifiedCatalogTable(compiler.source.table))
	statement.WriteString(" AS src")
	if predicate != "" {
		statement.WriteString(" WHERE ")
		statement.WriteString(predicate)
	}
	if len(order) != 0 {
		statement.WriteString(" ORDER BY ")
		statement.WriteString(strings.Join(order, ", "))
	}
	limit := query.Top
	if limit == 0 {
		limit = maxQueryResultRows + 1
	}
	statement.WriteString(fmt.Sprintf(" LIMIT %d", limit))
	outer := make([]string, len(compiler.outputs))
	for index := range compiler.outputs {
		physicalAlias := pgx.Identifier{fmt.Sprintf("q%d", index)}.Sanitize()
		outer[index] = "COALESCE(to_jsonb(query_result." + physicalAlias + "), 'null'::jsonb)"
	}
	result := "SELECT " + strings.Join(outer, ", ") + " FROM (" + statement.String() + ") AS query_result"
	if len(outerOrder) != 0 {
		result += " ORDER BY " + strings.Join(outerOrder, ", ")
	}
	return result, nil
}

func (compiler *queryCompiler) appendOutput(target *[]string, name, expression string, column *queryColumn, fixed *bytecode.Value) error {
	folded := strings.ToLower(name)
	if folded == "" || compiler.aliases[folded] != 0 {
		return fmt.Errorf("query result field %q is duplicated or empty", name)
	}
	compiler.outputs = append(compiler.outputs, queryOutput{name: name, column: column, fixed: fixed})
	index := len(compiler.outputs)
	compiler.aliases[folded] = index
	physicalAlias := fmt.Sprintf("q%d", index-1)
	*target = append(*target, expression+" AS "+pgx.Identifier{physicalAlias}.Sanitize())
	return nil
}

func (compiler *queryCompiler) compileOrder(expression querylang.Expression, distinct bool, projection *[]string, orderIndex int) (string, string, error) {
	field, ok := expression.(querylang.Field)
	if !ok {
		return "", "", querySemanticError(expression.ExpressionPosition(), "сортировка допускается только по полю")
	}
	if len(field.Path) == 1 {
		if index := compiler.aliases[strings.ToLower(field.Path[0])]; index != 0 {
			alias := pgx.Identifier{fmt.Sprintf("q%d", index-1)}.Sanitize()
			return alias, "query_result." + alias, nil
		}
	}
	column, err := compiler.resolveField(field)
	if err != nil {
		return "", "", err
	}
	if column.kind == queryRecorderColumn {
		return "", "", querySemanticError(field.Position, "сортировка по составному полю Регистратор пока не поддерживается")
	}
	if index := compiler.outputColumnIndex(column); index >= 0 {
		alias := pgx.Identifier{fmt.Sprintf("q%d", index)}.Sanitize()
		return alias, "query_result." + alias, nil
	}
	if distinct {
		return "", "", querySemanticError(field.Position, "при РАЗЛИЧНЫЕ поле сортировки должно входить в результат")
	}
	alias := pgx.Identifier{fmt.Sprintf("o%d", orderIndex)}.Sanitize()
	*projection = append(*projection, column.sql+" AS "+alias)
	return alias, "query_result." + alias, nil
}

func (compiler *queryCompiler) outputColumnIndex(column *queryColumn) int {
	for index, output := range compiler.outputs {
		if output.column != nil && output.column.sql == column.sql {
			return index
		}
	}
	return -1
}

func (compiler *queryCompiler) compilePredicate(expression querylang.Expression) (string, error) {
	switch value := expression.(type) {
	case querylang.Unary:
		if value.Operator != "not" {
			return "", querySemanticError(value.Position, "неподдерживаемый унарный оператор")
		}
		operand, err := compiler.compilePredicate(value.Operand)
		if err != nil {
			return "", err
		}
		return "NOT (" + operand + ")", nil
	case querylang.Binary:
		if value.Operator == "and" || value.Operator == "or" {
			left, err := compiler.compilePredicate(value.Left)
			if err != nil {
				return "", err
			}
			right, err := compiler.compilePredicate(value.Right)
			if err != nil {
				return "", err
			}
			return "(" + left + ") " + strings.ToUpper(value.Operator) + " (" + right + ")", nil
		}
		return compiler.compileComparison(value)
	case querylang.Field:
		column, err := compiler.resolveField(value)
		if err != nil {
			return "", err
		}
		if column.kind != queryStoredColumn || column.storage.valueType != BooleanType || column.storage.composite {
			return "", querySemanticError(value.Position, "условие без сравнения требует булево поле")
		}
		return column.sql, nil
	case querylang.Parameter, querylang.Literal:
		sql, _, fixed, err := compiler.compileValue(value, nil)
		if err != nil {
			return "", err
		}
		if fixed == nil || fixed.Kind() != bytecode.BooleanKind {
			return "", querySemanticError(value.ExpressionPosition(), "условие требует булево значение")
		}
		return sql, nil
	default:
		return "", querySemanticError(expression.ExpressionPosition(), "ожидалось логическое условие")
	}
}

func (compiler *queryCompiler) compileComparison(value querylang.Binary) (string, error) {
	leftField, leftIsField := value.Left.(querylang.Field)
	rightField, rightIsField := value.Right.(querylang.Field)
	var leftColumn, rightColumn *queryColumn
	var err error
	if leftIsField {
		leftColumn, err = compiler.resolveField(leftField)
		if err != nil {
			return "", err
		}
	}
	if rightIsField {
		rightColumn, err = compiler.resolveField(rightField)
		if err != nil {
			return "", err
		}
	}
	if value.Operator == "is" || value.Operator == "is not" {
		if !leftIsField {
			return "", querySemanticError(value.Position, "слева от ЕСТЬ ожидается поле")
		}
		operator := "IS NULL"
		if value.Operator == "is not" {
			operator = "IS NOT NULL"
		}
		return leftColumn.sql + " " + operator, nil
	}
	if value.Operator == "in" || value.Operator == "not in" {
		if !leftIsField {
			return "", querySemanticError(value.Position, "слева от В ожидается поле")
		}
		list, ok := value.Right.(querylang.List)
		if !ok {
			return "", querySemanticError(value.Position, "справа от В ожидается список")
		}
		return compiler.compileIn(leftColumn, list, value.Operator == "not in")
	}
	if leftColumn != nil && leftColumn.kind == queryRecorderColumn || rightColumn != nil && rightColumn.kind == queryRecorderColumn {
		return compiler.compileRecorderComparison(value, leftColumn, rightColumn)
	}
	if value.Operator == "like" || value.Operator == "not like" {
		if !leftIsField || leftColumn.storage.valueType != StringType || leftColumn.storage.composite {
			return "", querySemanticError(value.Position, "слева от ПОДОБНО ожидается строковое поле")
		}
	}
	leftSQL, _, _, err := compiler.compileValue(value.Left, rightColumn)
	if err != nil {
		return "", err
	}
	rightSQL, _, _, err := compiler.compileValue(value.Right, leftColumn)
	if err != nil {
		return "", err
	}
	operator := strings.ToUpper(value.Operator)
	if operator == "<>" {
		operator = "<>"
	}
	return leftSQL + " " + operator + " " + rightSQL, nil
}

func (compiler *queryCompiler) compileIn(column *queryColumn, list querylang.List, negate bool) (string, error) {
	if column.kind == queryRecorderColumn {
		parts := make([]string, 0, len(list.Items))
		for _, item := range list.Items {
			values, err := compiler.expandListItem(item)
			if err != nil {
				return "", err
			}
			for _, value := range values {
				condition, err := compiler.bindRecorder(column, value)
				if err != nil {
					return "", err
				}
				parts = append(parts, condition)
			}
		}
		if len(parts) == 0 {
			if negate {
				return "TRUE", nil
			}
			return "FALSE", nil
		}
		result := "(" + strings.Join(parts, " OR ") + ")"
		if negate {
			result = "NOT " + result
		}
		return result, nil
	}
	items := make([]string, 0, len(list.Items))
	for _, item := range list.Items {
		values, err := compiler.expandListItem(item)
		if err != nil {
			return "", err
		}
		for _, value := range values {
			bound, _, _, err := compiler.bindValue(value, column)
			if err != nil {
				return "", err
			}
			items = append(items, bound)
		}
	}
	if len(items) == 0 {
		if negate {
			return "TRUE", nil
		}
		return "FALSE", nil
	}
	operator := "IN"
	if negate {
		operator = "NOT IN"
	}
	return column.sql + " " + operator + " (" + strings.Join(items, ", ") + ")", nil
}

func (compiler *queryCompiler) expandListItem(expression querylang.Expression) ([]bytecode.Value, error) {
	if parameter, ok := expression.(querylang.Parameter); ok {
		value, err := compiler.parameter(parameter)
		if err != nil {
			return nil, err
		}
		if values, collection := bytecode.CollectionSnapshot(value); collection {
			return values, nil
		}
		return []bytecode.Value{value}, nil
	}
	value, err := literalValue(expression)
	if err != nil {
		return nil, err
	}
	return []bytecode.Value{value}, nil
}

func (compiler *queryCompiler) compileRecorderComparison(value querylang.Binary, left, right *queryColumn) (string, error) {
	if value.Operator != "=" && value.Operator != "<>" {
		return "", querySemanticError(value.Position, "Регистратор поддерживает только =, <>, В и НЕ В")
	}
	if left != nil && left.kind == queryRecorderColumn && right != nil && right.kind == queryRecorderColumn {
		return "", querySemanticError(value.Position, "сравнение двух полей Регистратор пока не поддерживается")
	}
	column, expression := left, value.Right
	if column == nil || column.kind != queryRecorderColumn {
		column, expression = right, value.Left
	}
	parameterValue, err := compiler.expressionValue(expression)
	if err != nil {
		return "", err
	}
	condition, err := compiler.bindRecorder(column, parameterValue)
	if err != nil {
		return "", err
	}
	if value.Operator == "<>" {
		condition = "NOT (" + condition + ")"
	}
	return condition, nil
}

func (compiler *queryCompiler) bindRecorder(column *queryColumn, value bytecode.Value) (string, error) {
	object, ok := value.AsRuntimeObject()
	reference, valid := object.(*documentReferenceObject)
	if !ok || !valid || reference.runtime != compiler.runtime || reference.reference.ObjectID.IsZero() {
		return "", fmt.Errorf("query field Регистратор requires a non-empty document reference")
	}
	if len(compiler.arguments) > maxQueryArguments-2 {
		return "", fmt.Errorf("query cannot contain more than %d SQL arguments", maxQueryArguments)
	}
	compiler.arguments = append(compiler.arguments, reference.reference.DocumentID.String(), reference.reference.ObjectID.String())
	first := len(compiler.arguments) - 1
	return fmt.Sprintf("(%s->>'type' = $%d AND %s->>'ref' = $%d)", column.sql, first, column.sql, first+1), nil
}

func (compiler *queryCompiler) compileValue(expression querylang.Expression, hint *queryColumn) (string, *queryColumn, *bytecode.Value, error) {
	switch value := expression.(type) {
	case querylang.Field:
		column, err := compiler.resolveField(value)
		if err != nil {
			return "", nil, nil, err
		}
		return column.sql, column, nil, nil
	case querylang.Parameter:
		parameter, err := compiler.parameter(value)
		if err != nil {
			return "", nil, nil, err
		}
		return compiler.bindValue(parameter, hint)
	case querylang.Literal:
		literal, err := literalValue(value)
		if err != nil {
			return "", nil, nil, err
		}
		return compiler.bindValue(literal, hint)
	default:
		return "", nil, nil, querySemanticError(expression.ExpressionPosition(), "выражение нельзя использовать как значение")
	}
}

func (compiler *queryCompiler) bindValue(value bytecode.Value, hint *queryColumn) (string, *queryColumn, *bytecode.Value, error) {
	if len(compiler.arguments) == maxQueryArguments {
		return "", nil, nil, fmt.Errorf("query cannot contain more than %d SQL arguments", maxQueryArguments)
	}
	if hint != nil {
		if hint.kind == queryRecorderColumn {
			return "", nil, nil, fmt.Errorf("Регистратор requires a comparison")
		}
		argument, cast, err := compiler.databaseValueForColumn(*hint, value)
		if err != nil {
			return "", nil, nil, err
		}
		compiler.arguments = append(compiler.arguments, argument)
		return fmt.Sprintf("$%d%s", len(compiler.arguments), cast), nil, nil, nil
	}
	argument, cast, err := compiler.genericDatabaseValue(value)
	if err != nil {
		return "", nil, nil, err
	}
	compiler.arguments = append(compiler.arguments, argument)
	copy := value
	return fmt.Sprintf("$%d%s", len(compiler.arguments), cast), nil, &copy, nil
}

func (compiler *queryCompiler) databaseValueForColumn(column queryColumn, value bytecode.Value) (any, string, error) {
	if value.Kind() == bytecode.NullKind || value.Kind() == bytecode.UndefinedKind {
		return nil, queryStorageCast(column.storage), nil
	}
	if column.kind == queryMovementKindColumn {
		text, ok := value.AsString()
		if !ok {
			return nil, "", fmt.Errorf("query field ВидДвижения requires ВидДвиженияНакопления")
		}
		switch strings.ToLower(text) {
		case "receipt", "приход":
			return int16(AccumulationMovementReceipt), "::smallint", nil
		case "expense", "расход":
			return int16(AccumulationMovementExpense), "::smallint", nil
		default:
			return nil, "", fmt.Errorf("unknown accumulation movement kind %q", text)
		}
	}
	if len(column.types) == 1 && (column.types[0].Kind == CatalogType || column.types[0].Kind == DocumentType) {
		object, ok := value.AsRuntimeObject()
		if !ok {
			return nil, "", fmt.Errorf("query field %s requires an object reference", column.name)
		}
		switch reference := object.(type) {
		case *catalogReferenceObject:
			if reference.runtime != compiler.runtime || column.types[0].Kind != CatalogType || column.types[0].Reference == nil || reference.reference.CatalogID != *column.types[0].Reference {
				return nil, "", fmt.Errorf("query field %s received a reference of another type", column.name)
			}
			return reference.reference.ObjectID.String(), "::uuid", nil
		case *documentReferenceObject:
			if reference.runtime != compiler.runtime || column.types[0].Kind != DocumentType || column.types[0].Reference == nil || reference.reference.DocumentID != *column.types[0].Reference {
				return nil, "", fmt.Errorf("query field %s received a reference of another type", column.name)
			}
			return reference.reference.ObjectID.String(), "::uuid", nil
		default:
			return nil, "", fmt.Errorf("query field %s requires an object reference", column.name)
		}
	}
	if !column.storage.composite {
		switch column.storage.valueType {
		case StringType:
			text, ok := value.AsString()
			if !ok {
				return nil, "", fmt.Errorf("query field %s requires a string", column.name)
			}
			return text, "::text", nil
		case NumberType:
			number, ok := value.NumberText()
			if !ok {
				return nil, "", fmt.Errorf("query field %s requires a number", column.name)
			}
			return number, "::numeric", nil
		case BooleanType:
			boolean, ok := value.AsBoolean()
			if !ok {
				return nil, "", fmt.Errorf("query field %s requires a boolean", column.name)
			}
			return boolean, "::boolean", nil
		case DateType:
			date, ok := value.AsDate()
			if !ok {
				return nil, "", fmt.Errorf("query field %s requires a date", column.name)
			}
			return date, "::timestamptz", nil
		case UUIDType:
			text, ok := value.AsString()
			if !ok {
				return nil, "", fmt.Errorf("query field %s requires a UUID", column.name)
			}
			if _, err := uuid.Parse(text); err != nil {
				return nil, "", fmt.Errorf("query field %s requires a UUID: %w", column.name, err)
			}
			return text, "::uuid", nil
		}
	}
	stored, err := compiler.runtime.applicationValueFromBSL(column.types, value, "query field "+column.name)
	if err != nil {
		return nil, "", err
	}
	argument, err := databaseAttributeValue(column.storage, stored)
	return argument, queryStorageCast(column.storage), err
}

func (compiler *queryCompiler) genericDatabaseValue(value bytecode.Value) (any, string, error) {
	switch value.Kind() {
	case bytecode.UndefinedKind, bytecode.NullKind:
		return nil, "::text", nil
	case bytecode.StringKind:
		text, _ := value.AsString()
		return text, "::text", nil
	case bytecode.NumberKind:
		text, _ := value.NumberText()
		return text, "::numeric", nil
	case bytecode.BooleanKind:
		boolean, _ := value.AsBoolean()
		return boolean, "::boolean", nil
	case bytecode.DateKind:
		date, _ := value.AsDate()
		return date, "::timestamptz", nil
	case bytecode.RuntimeObjectKind:
		object, _ := value.AsRuntimeObject()
		switch reference := object.(type) {
		case *catalogReferenceObject:
			if reference.runtime != compiler.runtime {
				return nil, "", fmt.Errorf("query parameter belongs to another metadata runtime")
			}
			return reference.reference.ObjectID.String(), "::uuid", nil
		case *documentReferenceObject:
			if reference.runtime != compiler.runtime {
				return nil, "", fmt.Errorf("query parameter belongs to another metadata runtime")
			}
			return reference.reference.ObjectID.String(), "::uuid", nil
		}
	}
	return nil, "", fmt.Errorf("query parameter kind %s is not supported", value.Kind())
}

func (compiler *queryCompiler) expressionValue(expression querylang.Expression) (bytecode.Value, error) {
	if parameter, ok := expression.(querylang.Parameter); ok {
		return compiler.parameter(parameter)
	}
	return literalValue(expression)
}

func (compiler *queryCompiler) parameter(value querylang.Parameter) (bytecode.Value, error) {
	result, ok := compiler.parameters[strings.ToLower(value.Name)]
	if !ok {
		return bytecode.Undefined(), querySemanticError(value.Position, "не установлен параметр &"+value.Name)
	}
	return result, nil
}

func literalValue(expression querylang.Expression) (bytecode.Value, error) {
	literal, ok := expression.(querylang.Literal)
	if !ok {
		return bytecode.Undefined(), querySemanticError(expression.ExpressionPosition(), "ожидался параметр или литерал")
	}
	switch literal.Kind {
	case querylang.LiteralString:
		return bytecode.String(literal.Text), nil
	case querylang.LiteralNumber:
		value, err := bytecode.ParseNumber(literal.Text)
		if err != nil {
			return bytecode.Undefined(), querySemanticError(literal.Position, "некорректное число")
		}
		return value, nil
	case querylang.LiteralBoolean:
		return bytecode.Boolean(literal.Text == "true"), nil
	case querylang.LiteralNull:
		return bytecode.Null(), nil
	case querylang.LiteralUndefined:
		return bytecode.Undefined(), nil
	default:
		return bytecode.Undefined(), querySemanticError(literal.Position, "неподдерживаемый литерал")
	}
}

func (compiler *queryCompiler) resolveField(field querylang.Field) (*queryColumn, error) {
	path := field.Path
	if len(path) > 1 {
		prefix := strings.Join(path[:len(path)-1], ".")
		valid := strings.EqualFold(prefix, compiler.source.alias) || strings.EqualFold(prefix, compiler.source.logicalName)
		if !valid && len(path) == 2 {
			valid = strings.EqualFold(path[0], compiler.source.alias) || strings.EqualFold(path[0], compiler.source.logicalName)
		}
		if !valid {
			return nil, querySemanticError(field.Position, "поле относится к неизвестному источнику "+prefix)
		}
	}
	name := path[len(path)-1]
	index, ok := compiler.source.byName[strings.ToLower(name)]
	if !ok {
		return nil, querySemanticError(field.Position, "неизвестное поле "+name)
	}
	return &compiler.source.columns[index], nil
}

func (compiler *queryCompiler) decodeOutput(output queryOutput, raw json.RawMessage) (bytecode.Value, error) {
	if output.fixed != nil {
		return *output.fixed, nil
	}
	if output.column == nil {
		return bytecode.Undefined(), fmt.Errorf("query output has no type")
	}
	if len(raw) == 0 || string(raw) == "null" {
		return bytecode.Null(), nil
	}
	column := *output.column
	switch column.kind {
	case queryRecorderColumn:
		var stored struct {
			Type string `json:"type"`
			Ref  string `json:"ref"`
		}
		if err := json.Unmarshal(raw, &stored); err != nil {
			return bytecode.Undefined(), err
		}
		typeID, err := uuid.Parse(stored.Type)
		if err != nil {
			return bytecode.Undefined(), err
		}
		objectID, err := uuid.Parse(stored.Ref)
		if err != nil {
			return bytecode.Undefined(), err
		}
		definition, ok := compiler.runtime.catalog.DocumentByID(typeID)
		if !ok {
			return bytecode.Undefined(), fmt.Errorf("unknown recorder document %s", typeID)
		}
		return compiler.runtime.wrapDocumentReference(definition, DocumentReference{DocumentID: typeID, ObjectID: objectID})
	case queryMovementKindColumn:
		var movement int
		if err := json.Unmarshal(raw, &movement); err != nil {
			return bytecode.Undefined(), err
		}
		if AccumulationMovementKind(movement) == AccumulationMovementReceipt {
			return bytecode.String("receipt"), nil
		}
		if AccumulationMovementKind(movement) == AccumulationMovementExpense {
			return bytecode.String("expense"), nil
		}
		return bytecode.Undefined(), fmt.Errorf("invalid movement kind %d", movement)
	default:
		stored, err := decodeDatabaseAttribute(column.storage, raw)
		if err != nil {
			return bytecode.Undefined(), err
		}
		return compiler.runtime.applicationValueToBSL(column.types, stored)
	}
}

func (runtime *Runtime) resolveQuerySource(source querylang.Source) (querySource, error) {
	if len(source.Path) != 2 {
		return querySource{}, querySemanticError(source.Position, "базовый запрос поддерживает источник вида ТипМетаданных.Имя")
	}
	kind, name := source.Path[0], source.Path[1]
	var result querySource
	result.logicalName = strings.Join(source.Path, ".")
	result.alias = source.Alias
	if result.alias == "" {
		result.alias = name
	}
	result.byName = make(map[string]int)
	switch {
	case queryName(kind, "Справочник", "Catalog"):
		definition, ok := runtime.catalog.CatalogDefinition(name)
		if !ok {
			return querySource{}, querySemanticError(source.Position, "неизвестный справочник "+name)
		}
		result.table, _ = PhysicalCatalogTable(definition.ID)
		result.addStored("Ссылка", "src.ref", []Type{referenceType(CatalogType, definition.ID)}, "Ref")
		result.addStored("Версия", "src.version", []Type{{Kind: NumberType, Precision: 19}}, "Version")
		result.addStored("Код", "src.code", []Type{catalogCodeType(definition.Code)}, "Code")
		result.addStored("Наименование", "src.description", []Type{{Kind: StringType, Length: definition.DescriptionLength}}, "Description")
		result.addStored("ПометкаУдаления", "src.deletion_mark", []Type{{Kind: BooleanType}}, "DeletionMark")
		result.addStored("ИмяПредопределенныхДанных", "src.predefined_name", []Type{{Kind: StringType, Length: 128}}, "PredefinedDataName")
		if err := runtime.addQueryAttributes(&result, definition.Attributes); err != nil {
			return querySource{}, err
		}
	case queryName(kind, "Документ", "Document"):
		definition, ok := runtime.catalog.DocumentDefinition(name)
		if !ok {
			return querySource{}, querySemanticError(source.Position, "неизвестный документ "+name)
		}
		result.table, _ = PhysicalDocumentTable(definition.ID)
		result.addStored("Ссылка", "src.ref", []Type{referenceType(DocumentType, definition.ID)}, "Ref")
		result.addStored("Версия", "src.version", []Type{{Kind: NumberType, Precision: 19}}, "Version")
		result.addStored("Номер", "src.number", []Type{documentNumberType(definition.Number)}, "Number")
		result.addStored("Дата", "src.date", []Type{{Kind: DateType}}, "Date")
		result.addStored("Проведен", "src.posted", []Type{{Kind: BooleanType}}, "Posted")
		result.addStored("ПометкаУдаления", "src.deletion_mark", []Type{{Kind: BooleanType}}, "DeletionMark")
		if err := runtime.addQueryAttributes(&result, definition.Attributes); err != nil {
			return querySource{}, err
		}
	case queryName(kind, "РегистрСведений", "InformationRegister"):
		definition, ok := runtime.catalog.InformationRegisterDefinition(name)
		if !ok {
			return querySource{}, querySemanticError(source.Position, "неизвестный регистр сведений "+name)
		}
		result.table, _ = PhysicalInformationRegisterTable(definition.ID)
		result.addStored("ИдентификаторЗаписи", "src.record_id", []Type{{Kind: UUIDType}}, "RecordID")
		if definition.Periodicity != InformationRegisterPeriodNone {
			result.addStored("Период", "src.period", []Type{{Kind: DateType}}, "Period")
		}
		if definition.WriteMode == InformationRegisterRecorder {
			result.addRecorder()
			result.addStored("НомерСтроки", "src.line_no", []Type{{Kind: NumberType, Precision: 10}}, "LineNumber")
			result.addStored("Активность", "src.active", []Type{{Kind: BooleanType}}, "Active")
		}
		if err := runtime.addQueryAttributes(&result, appendQueryAttributes(definition.Dimensions, definition.Resources, definition.Attributes)); err != nil {
			return querySource{}, err
		}
	case queryName(kind, "РегистрНакопления", "AccumulationRegister"):
		definition, ok := runtime.catalog.AccumulationRegisterDefinition(name)
		if !ok {
			return querySource{}, querySemanticError(source.Position, "неизвестный регистр накопления "+name)
		}
		result.table, _ = PhysicalAccumulationRegisterTable(definition.ID)
		result.addStored("ИдентификаторЗаписи", "src.record_id", []Type{{Kind: UUIDType}}, "RecordID")
		result.addStored("Период", "src.period", []Type{{Kind: DateType}}, "Period")
		result.addRecorder()
		result.addStored("НомерСтроки", "src.line_no", []Type{{Kind: NumberType, Precision: 10}}, "LineNumber")
		result.addStored("Активность", "src.active", []Type{{Kind: BooleanType}}, "Active")
		if definition.Kind == AccumulationRegisterBalance {
			result.add(queryColumn{name: "ВидДвижения", sql: "src.movement_kind", kind: queryMovementKindColumn, storage: attributeStorage{sqlType: "smallint", valueType: NumberType}}, "MovementKind")
		}
		if err := runtime.addQueryAttributes(&result, appendQueryAttributes(definition.Dimensions, definition.Resources, definition.Attributes)); err != nil {
			return querySource{}, err
		}
	default:
		return querySource{}, querySemanticError(source.Position, "неподдерживаемый тип таблицы "+kind)
	}
	return result, nil
}

func (runtime *Runtime) addQueryAttributes(source *querySource, attributes []Attribute) error {
	for _, attribute := range attributes {
		column, err := PhysicalAttributeColumn(attribute.ID)
		if err != nil {
			return err
		}
		storage, err := runtime.catalog.attributeStorage(attribute.Types)
		if err != nil {
			return err
		}
		source.add(queryColumn{name: attribute.Name, sql: "src." + pgx.Identifier{column}.Sanitize(), storage: storage, types: attribute.Types})
	}
	return nil
}

func (source *querySource) addStored(name, sql string, types []Type, aliases ...string) {
	storage, _ := (&Catalog{}).attributeStorage(types)
	// Built-in reference types need catalog expansion, which attributeStorage does not use.
	if len(types) == 1 && (types[0].Kind == CatalogType || types[0].Kind == DocumentType) {
		storage = attributeStorage{sqlType: "uuid", valueType: types[0].Kind, referenceObject: types[0].Reference}
	}
	source.add(queryColumn{name: name, sql: sql, storage: storage, types: types}, aliases...)
}

func (source *querySource) addRecorder() {
	source.add(queryColumn{
		name: "Регистратор", kind: queryRecorderColumn,
		sql: "jsonb_build_object('type', src.recorder_type::text, 'ref', src.recorder_ref::text)",
	}, "Recorder")
}

func (source *querySource) add(column queryColumn, aliases ...string) {
	index := len(source.columns)
	source.columns = append(source.columns, column)
	source.byName[strings.ToLower(column.name)] = index
	for _, alias := range aliases {
		source.byName[strings.ToLower(alias)] = index
	}
}

func queryStorageCast(storage attributeStorage) string {
	if storage.composite {
		return "::jsonb"
	}
	switch storage.valueType {
	case NumberType:
		return "::numeric"
	case BooleanType:
		return "::boolean"
	case DateType:
		return "::timestamptz"
	case UUIDType, EnumerationType, CatalogType, DocumentType:
		return "::uuid"
	default:
		return "::text"
	}
}

func queryName(value string, names ...string) bool {
	for _, name := range names {
		if strings.EqualFold(value, name) {
			return true
		}
	}
	return false
}

func referenceType(kind TypeKind, id uuid.UUID) Type {
	copy := id
	return Type{Kind: kind, Reference: &copy}
}

func catalogCodeType(code CatalogCode) Type {
	if code.Type == NumberType {
		return Type{Kind: NumberType, Precision: code.Length}
	}
	return Type{Kind: StringType, Length: code.Length}
}

func documentNumberType(number DocumentNumber) Type {
	if number.Type == NumberType {
		return Type{Kind: NumberType, Precision: number.Length}
	}
	return Type{Kind: StringType, Length: number.Length}
}

func appendQueryAttributes(groups ...[]Attribute) []Attribute {
	count := 0
	for _, group := range groups {
		count += len(group)
	}
	result := make([]Attribute, 0, count)
	for _, group := range groups {
		result = append(result, group...)
	}
	return result
}

func querySemanticError(position querylang.Position, message string) error {
	return fmt.Errorf("query line %d, column %d: %s", position.Line, position.Column, message)
}

func queryParameterName(name string) bool {
	if name == "" || len(name) > 128 || !utf8.ValidString(name) {
		return false
	}
	for index, r := range name {
		if r != '_' && !isQueryLetter(r) && (index == 0 || r < '0' || r > '9') {
			return false
		}
	}
	return true
}

func isQueryLetter(r rune) bool {
	return unicode.IsLetter(r)
}
