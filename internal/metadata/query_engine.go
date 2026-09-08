package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

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
	fromSQL     string
	logicalName string
	alias       string
	sqlAlias    string
	columns     []queryColumn
	byName      map[string]int
}

type queryOutput struct {
	name       string
	column     *queryColumn
	fixed      *bytecode.Value
	expression querylang.Expression
	aggregated bool
}

type queryCompiler struct {
	runtime           *Runtime
	sources           []querySource
	sourceNames       map[string]int
	parameters        map[string]bytecode.Value
	arguments         []any
	outputs           []queryOutput
	aliases           map[string]int
	allowAggregate    bool
	aggregateDepth    int
	containsAggregate bool
	visibleSources    int
	grouped           map[string]struct{}
}

type queryMaterializedTemporaryTable struct {
	fromSQL string
	table   queryTemporaryTable
}

func (runtime *Runtime) newQueryCompiler(query querylang.Query, parameters map[string]bytecode.Value, temporary map[string]queryMaterializedTemporaryTable) (*queryCompiler, error) {
	compiler := &queryCompiler{
		runtime: runtime, parameters: parameters, aliases: make(map[string]int), sourceNames: make(map[string]int),
	}
	definitions := make([]querylang.Source, 0, len(query.Joins)+1)
	definitions = append(definitions, query.Source)
	for _, join := range query.Joins {
		definitions = append(definitions, join.Source)
	}
	for index, definition := range definitions {
		sqlAlias := fmt.Sprintf("s%d", index)
		source, err := runtime.resolveQuerySource(definition, sqlAlias, temporary)
		if err != nil {
			return nil, err
		}
		foldedAlias := strings.ToLower(source.alias)
		if foldedAlias == "" {
			return nil, querySemanticError(definition.Position, "источник должен иметь псевдоним")
		}
		if existing := compiler.sourceNames[foldedAlias]; existing != 0 {
			return nil, querySemanticError(definition.Position, "псевдоним источника "+source.alias+" указан повторно")
		}
		compiler.sources = append(compiler.sources, source)
		compiler.sourceNames[foldedAlias] = len(compiler.sources)
		foldedLogical := strings.ToLower(source.logicalName)
		if existing := compiler.sourceNames[foldedLogical]; existing == 0 {
			compiler.sourceNames[foldedLogical] = len(compiler.sources)
		} else if existing != len(compiler.sources) {
			compiler.sourceNames[foldedLogical] = -1
		}
	}
	compiler.visibleSources = len(compiler.sources)
	return compiler, nil
}

func (runtime *Runtime) executeQuery(ctx context.Context, text string, parameters map[string]bytecode.Value) (*queryResultObject, error) {
	results, err := runtime.executeQueryPackage(ctx, text, parameters, nil)
	if err != nil {
		return nil, err
	}
	return results[len(results)-1], nil
}

func (runtime *Runtime) executeQueryPackage(ctx context.Context, text string, parameters map[string]bytecode.Value, manager *temporaryTableManagerObject) ([]*queryResultObject, error) {
	if !utf8.ValidString(text) || strings.TrimSpace(text) == "" || len(text) > maxQueryTextBytes {
		return nil, fmt.Errorf("query text must contain 1..%d UTF-8 bytes", maxQueryTextBytes)
	}
	statements, err := querylang.ParsePackage(text)
	if err != nil {
		return nil, err
	}
	if manager == nil {
		for _, statement := range statements {
			if statement.Drop != nil || statement.Query.Into != nil || queryUsesTemporarySource(*statement.Query) {
				return nil, fmt.Errorf("temporary table operations require МенеджерВременныхТаблиц")
			}
		}
	}
	pool, err := runtime.databasePool()
	if err != nil {
		return nil, err
	}
	if manager == nil && len(statements) == 1 {
		query, err := queryData(ctx, pool)
		if err != nil {
			return nil, err
		}
		result, _, err := runtime.executeParsedQuery(ctx, query, pool, *statements[0].Query, parameters, nil, false)
		if err != nil {
			return nil, err
		}
		return []*queryResultObject{result}, nil
	}
	work := make(map[string]queryTemporaryTable)
	originalVersion := uint64(0)
	if manager != nil {
		if manager.runtime != runtime {
			return nil, fmt.Errorf("temporary table manager belongs to another metadata runtime")
		}
		work, originalVersion, err = manager.snapshot()
		if err != nil {
			return nil, err
		}
	}
	original := cloneTemporaryTables(work)
	appliedVersion := uint64(0)
	rollback := func() {
		if manager != nil && appliedVersion != 0 {
			manager.restore(appliedVersion, original, originalVersion)
		}
	}
	var results []*queryResultObject
	err = runDataTransaction(ctx, pool, rollback, func(transactionContext context.Context, transaction pgx.Tx) error {
		results = make([]*queryResultObject, 0, len(statements))
		changed := false
		for _, statement := range statements {
			if statement.Drop != nil {
				key := temporaryTableKey(statement.Drop.Name)
				if _, ok := work[key]; !ok {
					return querySemanticError(statement.Drop.Position, "неизвестная временная таблица "+statement.Drop.Name)
				}
				delete(work, key)
				changed = true
				results = append(results, &queryResultObject{runtime: runtime})
				continue
			}
			materialized, cleanup, err := runtime.materializeTemporarySources(transactionContext, transaction, *statement.Query, work)
			if err != nil {
				return err
			}
			result, raw, executeErr := runtime.executeParsedQuery(transactionContext, transaction, pool, *statement.Query, parameters, materialized, statement.Query.Into != nil)
			cleanupErr := cleanup()
			if executeErr != nil {
				return executeErr
			}
			if cleanupErr != nil {
				return cleanupErr
			}
			if statement.Query.Into != nil {
				table, err := buildTemporaryTable(statement.Query.Into.Name, statement.Query.IndexBy, result, raw)
				if err != nil {
					return err
				}
				key := temporaryTableKey(table.name)
				if _, exists := work[key]; exists {
					return querySemanticError(statement.Query.Into.Position, "временная таблица "+table.name+" уже существует")
				}
				work[key] = table
				changed = true
				count, _ := bytecode.ParseNumber(strconv.Itoa(table.rowCount))
				result = &queryResultObject{runtime: runtime, columns: []string{"Количество"}, rows: [][]bytecode.Value{{count}}}
			}
			results = append(results, result)
		}
		if manager != nil && changed {
			appliedVersion, err = manager.apply(originalVersion, cloneTemporaryTables(work))
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}

func (runtime *Runtime) executeParsedQuery(ctx context.Context, query dataQueryer, pool *pgxpool.Pool, parsed querylang.Query, parameters map[string]bytecode.Value, temporary map[string]queryMaterializedTemporaryTable, retainRaw bool) (*queryResultObject, [][]json.RawMessage, error) {
	compiler, err := runtime.newQueryCompiler(parsed, parameters, temporary)
	if err != nil {
		return nil, nil, err
	}
	statement, err := compiler.compile(parsed)
	if err != nil {
		return nil, nil, err
	}
	rows, err := query.Query(ctx, statement, compiler.arguments...)
	if err != nil {
		return nil, nil, recordDataError(ctx, pool, fmt.Errorf("execute query: %w", err))
	}
	defer rows.Close()
	result := &queryResultObject{runtime: runtime, columns: make([]string, len(compiler.outputs)), descriptors: make([]queryColumn, len(compiler.outputs))}
	for index := range compiler.outputs {
		result.columns[index] = compiler.outputs[index].name
		descriptor, descriptorErr := compiler.outputDescriptor(compiler.outputs[index])
		if descriptorErr != nil && retainRaw {
			return nil, nil, descriptorErr
		}
		result.descriptors[index] = descriptor
	}
	used := uint64(256)
	var retained [][]json.RawMessage
	for rows.Next() {
		if len(result.rows) == maxQueryResultRows {
			return nil, nil, fmt.Errorf("query result exceeds %d rows; use ПЕРВЫЕ", maxQueryResultRows)
		}
		raw := make([]json.RawMessage, len(compiler.outputs))
		targets := make([]any, len(raw))
		for index := range raw {
			targets[index] = &raw[index]
		}
		if err := rows.Scan(targets...); err != nil {
			return nil, nil, fmt.Errorf("scan query result: %w", err)
		}
		if retainRaw {
			copy := make([]json.RawMessage, len(raw))
			for index := range raw {
				copy[index] = append(json.RawMessage(nil), raw[index]...)
			}
			retained = append(retained, copy)
		}
		values := make([]bytecode.Value, len(raw))
		for index := range raw {
			values[index], err = compiler.decodeOutput(compiler.outputs[index], raw[index])
			if err != nil {
				return nil, nil, fmt.Errorf("decode query field %s: %w", compiler.outputs[index].name, err)
			}
			memory, ok := values[index].DynamicMemory(maxQueryResultBytes - min(used, maxQueryResultBytes))
			if !ok || memory > maxQueryResultBytes-used {
				return nil, nil, fmt.Errorf("query result exceeds %d MiB", maxQueryResultBytes>>20)
			}
			used += memory + uint64(len(compiler.outputs[index].name)) + 32
			if used > maxQueryResultBytes {
				return nil, nil, fmt.Errorf("query result exceeds %d MiB", maxQueryResultBytes>>20)
			}
		}
		result.rows = append(result.rows, values)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, recordDataError(ctx, pool, fmt.Errorf("read query result: %w", err))
	}
	return result, retained, nil
}

func (compiler *queryCompiler) outputDescriptor(output queryOutput) (queryColumn, error) {
	if output.column != nil {
		return *output.column, nil
	}
	if output.fixed == nil {
		return queryColumn{}, fmt.Errorf("query field %s has no type", output.name)
	}
	column := queryColumn{name: output.name, kind: queryStoredColumn}
	switch output.fixed.Kind() {
	case bytecode.StringKind:
		column.storage = attributeStorage{sqlType: "text", valueType: StringType}
		column.types = []Type{{Kind: StringType}}
	case bytecode.NumberKind:
		column.storage = attributeStorage{sqlType: "numeric", valueType: NumberType}
		column.types = []Type{{Kind: NumberType, Precision: 38}}
	case bytecode.BooleanKind:
		column.storage = attributeStorage{sqlType: "boolean", valueType: BooleanType}
		column.types = []Type{{Kind: BooleanType}}
	case bytecode.DateKind:
		column.storage = attributeStorage{sqlType: "timestamp with time zone", valueType: DateType}
		column.types = []Type{{Kind: DateType}}
	case bytecode.RuntimeObjectKind:
		object, _ := output.fixed.AsRuntimeObject()
		switch reference := object.(type) {
		case *catalogReferenceObject:
			column.storage = attributeStorage{sqlType: "uuid", valueType: CatalogType, referenceObject: &reference.reference.CatalogID}
			column.types = []Type{referenceType(CatalogType, reference.reference.CatalogID)}
		case *documentReferenceObject:
			column.storage = attributeStorage{sqlType: "uuid", valueType: DocumentType, referenceObject: &reference.reference.DocumentID}
			column.types = []Type{referenceType(DocumentType, reference.reference.DocumentID)}
		default:
			return queryColumn{}, fmt.Errorf("query field %s cannot be stored in a temporary table", output.name)
		}
	default:
		return queryColumn{}, fmt.Errorf("query field %s has no storable type", output.name)
	}
	return column, nil
}

func (compiler *queryCompiler) compile(query querylang.Query) (string, error) {
	selectSQL := make([]string, 0, len(query.Fields))
	for _, selected := range query.Fields {
		if selected.Wildcard {
			sources := compiler.sources
			if len(selected.WildcardSource) != 0 {
				source, err := compiler.resolveSourceName(selected.WildcardSource, selected.Position)
				if err != nil {
					return "", err
				}
				sources = []querySource{*source}
			}
			for sourceIndex := range sources {
				for columnIndex := range sources[sourceIndex].columns {
					column := sources[sourceIndex].columns[columnIndex]
					field := querylang.Field{Path: []string{sources[sourceIndex].alias, column.name}, Position: selected.Position}
					if err := compiler.appendOutput(&selectSQL, column.name, column.sql, &column, nil, field, false); err != nil {
						return "", err
					}
				}
			}
			continue
		}
		compiler.allowAggregate = true
		expressionSQL, column, fixed, err := compiler.compileValue(selected.Expression, nil)
		compiler.allowAggregate = false
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
		aggregated := expressionContainsAggregate(selected.Expression)
		if err := compiler.appendOutput(&selectSQL, name, expressionSQL, column, fixed, selected.Expression, aggregated); err != nil {
			return "", err
		}
	}
	if len(selectSQL) == 0 {
		return "", fmt.Errorf("query must select at least one field")
	}
	joinConditions := make([]string, len(query.Joins))
	for index, join := range query.Joins {
		if join.Kind == querylang.JoinCross {
			continue
		}
		compiler.visibleSources = index + 2
		condition, err := compiler.compilePredicate(join.Condition)
		compiler.visibleSources = len(compiler.sources)
		if err != nil {
			return "", err
		}
		joinConditions[index] = condition
	}
	predicate := ""
	if query.Where != nil {
		var err error
		predicate, err = compiler.compilePredicate(query.Where)
		if err != nil {
			return "", err
		}
	}
	group := make([]string, 0, len(query.Group))
	grouped := make(map[string]struct{}, len(query.Group))
	compiler.grouped = grouped
	for _, expression := range query.Group {
		field, ok := expression.(querylang.Field)
		if !ok {
			return "", querySemanticError(expression.ExpressionPosition(), "группировка допускает только поля")
		}
		column, err := compiler.resolveGroupedField(field)
		if err != nil {
			return "", err
		}
		if column.kind == queryRecorderColumn {
			return "", querySemanticError(field.Position, "группировка по составному полю Регистратор пока не поддерживается")
		}
		if _, duplicate := grouped[column.sql]; duplicate {
			return "", querySemanticError(field.Position, "поле группировки указано повторно")
		}
		grouped[column.sql] = struct{}{}
		group = append(group, column.sql)
	}
	aggregateQuery := compiler.containsAggregate || expressionContainsAggregate(query.Having)
	if len(group) != 0 || aggregateQuery {
		for _, output := range compiler.outputs {
			if output.aggregated || output.column == nil {
				continue
			}
			if _, ok := grouped[output.column.sql]; !ok {
				return "", querySemanticError(output.expression.ExpressionPosition(), "поле "+output.name+" должно входить в СГРУППИРОВАТЬ ПО")
			}
		}
	}
	having := ""
	if query.Having != nil {
		if len(group) == 0 && !compiler.containsAggregate && !expressionContainsAggregate(query.Having) {
			return "", querySemanticError(query.Having.ExpressionPosition(), "ИМЕЮЩИЕ требует группировку или агрегат")
		}
		if err := compiler.validateGroupedExpression(query.Having, grouped, false); err != nil {
			return "", err
		}
		compiler.allowAggregate = true
		var err error
		having, err = compiler.compilePredicate(query.Having)
		compiler.allowAggregate = false
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
	statement.WriteString(compiler.sources[0].fromSQL)
	statement.WriteString(" AS ")
	statement.WriteString(pgx.Identifier{compiler.sources[0].sqlAlias}.Sanitize())
	for index, join := range query.Joins {
		statement.WriteByte(' ')
		statement.WriteString(queryJoinSQL(join.Kind))
		statement.WriteByte(' ')
		statement.WriteString(compiler.sources[index+1].fromSQL)
		statement.WriteString(" AS ")
		statement.WriteString(pgx.Identifier{compiler.sources[index+1].sqlAlias}.Sanitize())
		if join.Kind != querylang.JoinCross {
			statement.WriteString(" ON ")
			statement.WriteString(joinConditions[index])
		}
	}
	if predicate != "" {
		statement.WriteString(" WHERE ")
		statement.WriteString(predicate)
	}
	if len(group) != 0 {
		statement.WriteString(" GROUP BY ")
		statement.WriteString(strings.Join(group, ", "))
	}
	if having != "" {
		statement.WriteString(" HAVING ")
		statement.WriteString(having)
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

func (compiler *queryCompiler) appendOutput(target *[]string, name, expression string, column *queryColumn, fixed *bytecode.Value, source querylang.Expression, aggregated bool) error {
	folded := strings.ToLower(name)
	if folded == "" || compiler.aliases[folded] != 0 {
		return fmt.Errorf("query result field %q is duplicated or empty", name)
	}
	compiler.outputs = append(compiler.outputs, queryOutput{name: name, column: column, fixed: fixed, expression: source, aggregated: aggregated})
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
	if (compiler.containsAggregate || len(compiler.grouped) != 0) && compiler.grouped != nil {
		if _, grouped := compiler.grouped[column.sql]; !grouped {
			return "", "", querySemanticError(field.Position, "поле сортировки должно входить в СГРУППИРОВАТЬ ПО или результат агрегата")
		}
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
	case querylang.Function:
		return compiler.compileAggregate(value)
	default:
		return "", nil, nil, querySemanticError(expression.ExpressionPosition(), "выражение нельзя использовать как значение")
	}
}

func (compiler *queryCompiler) compileAggregate(function querylang.Function) (string, *queryColumn, *bytecode.Value, error) {
	if !compiler.allowAggregate {
		return "", nil, nil, querySemanticError(function.Position, "агрегатную функцию нельзя использовать в этом месте")
	}
	if compiler.aggregateDepth != 0 {
		return "", nil, nil, querySemanticError(function.Position, "вложенные агрегатные функции не поддерживаются")
	}
	name := strings.ToUpper(function.Name)
	switch {
	case queryName(name, "КОЛИЧЕСТВО", "COUNT"):
		name = "COUNT"
	case queryName(name, "СУММА", "SUM"):
		name = "SUM"
	case queryName(name, "МИНИМУМ", "MIN"):
		name = "MIN"
	case queryName(name, "МАКСИМУМ", "MAX"):
		name = "MAX"
	case queryName(name, "СРЕДНЕЕ", "AVG"):
		name = "AVG"
	default:
		return "", nil, nil, querySemanticError(function.Position, "неизвестная агрегатная функция "+function.Name)
	}
	if function.Wildcard {
		if name != "COUNT" || function.Distinct || len(function.Arguments) != 0 {
			return "", nil, nil, querySemanticError(function.Position, "* допускается только в КОЛИЧЕСТВО(*)")
		}
		compiler.containsAggregate = true
		column := &queryColumn{name: function.Name, kind: queryStoredColumn, storage: attributeStorage{sqlType: "numeric", valueType: NumberType}, types: []Type{{Kind: NumberType, Precision: 38}}}
		return "COUNT(*)", column, nil, nil
	}
	if len(function.Arguments) != 1 {
		return "", nil, nil, querySemanticError(function.Position, "агрегатная функция ожидает один аргумент")
	}
	compiler.aggregateDepth++
	argumentSQL, argumentColumn, _, err := compiler.compileValue(function.Arguments[0], nil)
	compiler.aggregateDepth--
	if err != nil {
		return "", nil, nil, err
	}
	if argumentColumn == nil {
		return "", nil, nil, querySemanticError(function.Arguments[0].ExpressionPosition(), "аргументом агрегатной функции должно быть поле")
	}
	if name != "COUNT" && (argumentColumn.kind == queryRecorderColumn || argumentColumn.storage.composite) {
		return "", nil, nil, querySemanticError(function.Arguments[0].ExpressionPosition(), "агрегатная функция не поддерживает составное поле")
	}
	if (name == "SUM" || name == "AVG") && argumentColumn.storage.valueType != NumberType {
		return "", nil, nil, querySemanticError(function.Arguments[0].ExpressionPosition(), function.Name+" требует числовое поле")
	}
	if (name == "MIN" || name == "MAX") && argumentColumn.storage.valueType != NumberType && argumentColumn.storage.valueType != StringType && argumentColumn.storage.valueType != DateType {
		return "", nil, nil, querySemanticError(function.Arguments[0].ExpressionPosition(), function.Name+" требует числовое, строковое поле или дату")
	}
	distinct := ""
	if function.Distinct {
		distinct = "DISTINCT "
	}
	resultColumn := *argumentColumn
	resultColumn.name = function.Name
	resultColumn.sql = ""
	if name == "COUNT" || name == "SUM" || name == "AVG" {
		resultColumn.kind = queryStoredColumn
		resultColumn.storage = attributeStorage{sqlType: "numeric", valueType: NumberType}
		resultColumn.types = []Type{{Kind: NumberType, Precision: 38}}
	}
	compiler.containsAggregate = true
	return name + "(" + distinct + argumentSQL + ")", &resultColumn, nil, nil
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
	if len(path) == 0 {
		return nil, querySemanticError(field.Position, "имя поля пусто")
	}
	name := path[len(path)-1]
	if len(path) > 1 {
		source, err := compiler.resolveSourceName(path[:len(path)-1], field.Position)
		if err != nil {
			return nil, err
		}
		index, ok := source.byName[strings.ToLower(name)]
		if !ok {
			return nil, querySemanticError(field.Position, "неизвестное поле "+name+" источника "+source.alias)
		}
		return &source.columns[index], nil
	}
	matchSource, matchColumn := -1, -1
	for sourceIndex := 0; sourceIndex < compiler.visibleSources; sourceIndex++ {
		if columnIndex, ok := compiler.sources[sourceIndex].byName[strings.ToLower(name)]; ok {
			if matchSource >= 0 {
				return nil, querySemanticError(field.Position, "неоднозначное поле "+name+"; укажите псевдоним источника")
			}
			matchSource, matchColumn = sourceIndex, columnIndex
		}
	}
	if matchSource < 0 {
		return nil, querySemanticError(field.Position, "неизвестное поле "+name)
	}
	return &compiler.sources[matchSource].columns[matchColumn], nil
}

func (compiler *queryCompiler) resolveSourceName(path []string, position querylang.Position) (*querySource, error) {
	name := strings.ToLower(strings.Join(path, "."))
	index := compiler.sourceNames[name]
	if index == 0 {
		return nil, querySemanticError(position, "неизвестный источник "+strings.Join(path, "."))
	}
	if index < 0 {
		return nil, querySemanticError(position, "неоднозначный источник "+strings.Join(path, ".")+"; укажите псевдоним")
	}
	if index > compiler.visibleSources {
		return nil, querySemanticError(position, "источник "+strings.Join(path, ".")+" ещё недоступен в этом соединении")
	}
	return &compiler.sources[index-1], nil
}

func (compiler *queryCompiler) resolveGroupedField(field querylang.Field) (*queryColumn, error) {
	if len(field.Path) == 1 {
		if index := compiler.aliases[strings.ToLower(field.Path[0])]; index != 0 {
			output := compiler.outputs[index-1]
			if output.aggregated {
				return nil, querySemanticError(field.Position, "агрегатное поле нельзя использовать в СГРУППИРОВАТЬ ПО")
			}
			source, ok := output.expression.(querylang.Field)
			if !ok {
				return nil, querySemanticError(field.Position, "псевдоним группировки должен ссылаться на поле")
			}
			return compiler.resolveField(source)
		}
	}
	return compiler.resolveField(field)
}

func (compiler *queryCompiler) validateGroupedExpression(expression querylang.Expression, grouped map[string]struct{}, insideAggregate bool) error {
	switch value := expression.(type) {
	case querylang.Field:
		if insideAggregate {
			return nil
		}
		column, err := compiler.resolveField(value)
		if err != nil {
			return err
		}
		if _, ok := grouped[column.sql]; !ok {
			return querySemanticError(value.Position, "поле "+value.Path[len(value.Path)-1]+" должно входить в СГРУППИРОВАТЬ ПО")
		}
	case querylang.Function:
		for _, argument := range value.Arguments {
			if err := compiler.validateGroupedExpression(argument, grouped, true); err != nil {
				return err
			}
		}
	case querylang.Unary:
		return compiler.validateGroupedExpression(value.Operand, grouped, insideAggregate)
	case querylang.Binary:
		if err := compiler.validateGroupedExpression(value.Left, grouped, insideAggregate); err != nil {
			return err
		}
		return compiler.validateGroupedExpression(value.Right, grouped, insideAggregate)
	case querylang.List:
		for _, item := range value.Items {
			if err := compiler.validateGroupedExpression(item, grouped, insideAggregate); err != nil {
				return err
			}
		}
	}
	return nil
}

func expressionContainsAggregate(expression querylang.Expression) bool {
	switch value := expression.(type) {
	case nil:
		return false
	case querylang.Function:
		return true
	case querylang.Unary:
		return expressionContainsAggregate(value.Operand)
	case querylang.Binary:
		return expressionContainsAggregate(value.Left) || expressionContainsAggregate(value.Right)
	case querylang.List:
		for _, item := range value.Items {
			if expressionContainsAggregate(item) {
				return true
			}
		}
	}
	return false
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

func (runtime *Runtime) resolveQuerySource(source querylang.Source, sqlAlias string, temporary map[string]queryMaterializedTemporaryTable) (querySource, error) {
	if len(source.Path) != 1 && len(source.Path) != 2 {
		return querySource{}, querySemanticError(source.Position, "ожидалось имя временной таблицы или ТипМетаданных.Имя")
	}
	var result querySource
	result.logicalName = strings.Join(source.Path, ".")
	result.sqlAlias = sqlAlias
	result.alias = source.Alias
	if result.alias == "" {
		result.alias = source.Path[len(source.Path)-1]
	}
	result.byName = make(map[string]int)
	if len(source.Path) == 1 {
		materialized, ok := temporary[strings.ToLower(source.Path[0])]
		if !ok {
			return querySource{}, querySemanticError(source.Position, "неизвестная временная таблица "+source.Path[0])
		}
		result.fromSQL = materialized.fromSQL
		for index, stored := range materialized.table.columns {
			column := stored
			column.sql = querySourceColumnSQL(sqlAlias, fmt.Sprintf("c%d", index))
			result.add(column)
		}
		return result, nil
	}
	kind, name := source.Path[0], source.Path[1]
	switch {
	case queryName(kind, "Справочник", "Catalog"):
		definition, ok := runtime.catalog.CatalogDefinition(name)
		if !ok {
			return querySource{}, querySemanticError(source.Position, "неизвестный справочник "+name)
		}
		table, _ := PhysicalCatalogTable(definition.ID)
		result.fromSQL = qualifiedCatalogTable(table)
		result.addStored("Ссылка", querySourceColumnSQL(sqlAlias, "ref"), []Type{referenceType(CatalogType, definition.ID)}, "Ref")
		result.addStored("Версия", querySourceColumnSQL(sqlAlias, "version"), []Type{{Kind: NumberType, Precision: 19}}, "Version")
		result.addStored("Код", querySourceColumnSQL(sqlAlias, "code"), []Type{catalogCodeType(definition.Code)}, "Code")
		result.addStored("Наименование", querySourceColumnSQL(sqlAlias, "description"), []Type{{Kind: StringType, Length: definition.DescriptionLength}}, "Description")
		result.addStored("ПометкаУдаления", querySourceColumnSQL(sqlAlias, "deletion_mark"), []Type{{Kind: BooleanType}}, "DeletionMark")
		result.addStored("ИмяПредопределенныхДанных", querySourceColumnSQL(sqlAlias, "predefined_name"), []Type{{Kind: StringType, Length: 128}}, "PredefinedDataName")
		if err := runtime.addQueryAttributes(&result, definition.Attributes); err != nil {
			return querySource{}, err
		}
	case queryName(kind, "Документ", "Document"):
		definition, ok := runtime.catalog.DocumentDefinition(name)
		if !ok {
			return querySource{}, querySemanticError(source.Position, "неизвестный документ "+name)
		}
		table, _ := PhysicalDocumentTable(definition.ID)
		result.fromSQL = qualifiedCatalogTable(table)
		result.addStored("Ссылка", querySourceColumnSQL(sqlAlias, "ref"), []Type{referenceType(DocumentType, definition.ID)}, "Ref")
		result.addStored("Версия", querySourceColumnSQL(sqlAlias, "version"), []Type{{Kind: NumberType, Precision: 19}}, "Version")
		result.addStored("Номер", querySourceColumnSQL(sqlAlias, "number"), []Type{documentNumberType(definition.Number)}, "Number")
		result.addStored("Дата", querySourceColumnSQL(sqlAlias, "date"), []Type{{Kind: DateType}}, "Date")
		result.addStored("Проведен", querySourceColumnSQL(sqlAlias, "posted"), []Type{{Kind: BooleanType}}, "Posted")
		result.addStored("ПометкаУдаления", querySourceColumnSQL(sqlAlias, "deletion_mark"), []Type{{Kind: BooleanType}}, "DeletionMark")
		if err := runtime.addQueryAttributes(&result, definition.Attributes); err != nil {
			return querySource{}, err
		}
	case queryName(kind, "РегистрСведений", "InformationRegister"):
		definition, ok := runtime.catalog.InformationRegisterDefinition(name)
		if !ok {
			return querySource{}, querySemanticError(source.Position, "неизвестный регистр сведений "+name)
		}
		table, _ := PhysicalInformationRegisterTable(definition.ID)
		result.fromSQL = qualifiedCatalogTable(table)
		result.addStored("ИдентификаторЗаписи", querySourceColumnSQL(sqlAlias, "record_id"), []Type{{Kind: UUIDType}}, "RecordID")
		if definition.Periodicity != InformationRegisterPeriodNone {
			result.addStored("Период", querySourceColumnSQL(sqlAlias, "period"), []Type{{Kind: DateType}}, "Period")
		}
		if definition.WriteMode == InformationRegisterRecorder {
			result.addRecorder()
			result.addStored("НомерСтроки", querySourceColumnSQL(sqlAlias, "line_no"), []Type{{Kind: NumberType, Precision: 10}}, "LineNumber")
			result.addStored("Активность", querySourceColumnSQL(sqlAlias, "active"), []Type{{Kind: BooleanType}}, "Active")
		}
		if err := runtime.addQueryAttributes(&result, appendQueryAttributes(definition.Dimensions, definition.Resources, definition.Attributes)); err != nil {
			return querySource{}, err
		}
	case queryName(kind, "РегистрНакопления", "AccumulationRegister"):
		definition, ok := runtime.catalog.AccumulationRegisterDefinition(name)
		if !ok {
			return querySource{}, querySemanticError(source.Position, "неизвестный регистр накопления "+name)
		}
		table, _ := PhysicalAccumulationRegisterTable(definition.ID)
		result.fromSQL = qualifiedCatalogTable(table)
		result.addStored("ИдентификаторЗаписи", querySourceColumnSQL(sqlAlias, "record_id"), []Type{{Kind: UUIDType}}, "RecordID")
		result.addStored("Период", querySourceColumnSQL(sqlAlias, "period"), []Type{{Kind: DateType}}, "Period")
		result.addRecorder()
		result.addStored("НомерСтроки", querySourceColumnSQL(sqlAlias, "line_no"), []Type{{Kind: NumberType, Precision: 10}}, "LineNumber")
		result.addStored("Активность", querySourceColumnSQL(sqlAlias, "active"), []Type{{Kind: BooleanType}}, "Active")
		if definition.Kind == AccumulationRegisterBalance {
			result.add(queryColumn{name: "ВидДвижения", sql: querySourceColumnSQL(sqlAlias, "movement_kind"), kind: queryMovementKindColumn, storage: attributeStorage{sqlType: "smallint", valueType: NumberType}}, "MovementKind")
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
		source.add(queryColumn{name: attribute.Name, sql: querySourceColumnSQL(source.sqlAlias, column), storage: storage, types: attribute.Types})
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
	table := pgx.Identifier{source.sqlAlias}.Sanitize()
	source.add(queryColumn{
		name: "Регистратор", kind: queryRecorderColumn,
		sql: "jsonb_build_object('type', " + table + ".recorder_type::text, 'ref', " + table + ".recorder_ref::text)",
	}, "Recorder")
}

func querySourceColumnSQL(alias, column string) string {
	return pgx.Identifier{alias}.Sanitize() + "." + pgx.Identifier{column}.Sanitize()
}

func queryJoinSQL(kind querylang.JoinKind) string {
	switch kind {
	case querylang.JoinInner:
		return "INNER JOIN"
	case querylang.JoinLeft:
		return "LEFT JOIN"
	case querylang.JoinRight:
		return "RIGHT JOIN"
	case querylang.JoinFull:
		return "FULL JOIN"
	case querylang.JoinCross:
		return "CROSS JOIN"
	default:
		return "JOIN"
	}
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
