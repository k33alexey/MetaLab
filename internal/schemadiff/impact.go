package schemadiff

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

// columnType is one PostgreSQL type out of the closed set an ML application
// schema can produce: text, character varying(n), numeric(p,s), boolean,
// timestamp with time zone, uuid, jsonb. Knowing the family and its parameters
// is what turns "the type changed" into "every price will be rounded".
type columnType struct {
	family    string
	precision int
	scale     int
	// bounded reports whether the parameters were given at all: plain "numeric"
	// keeps whatever scale a value has, so narrowing it rewrites values just as
	// numeric(10,2) -> numeric(10,0) does.
	bounded bool
}

func parseColumnType(value string) columnType {
	value = strings.ToLower(normalizeSQL(value))
	open := strings.IndexByte(value, '(')
	if open < 0 || !strings.HasSuffix(value, ")") {
		return columnType{family: value}
	}
	family := strings.TrimSpace(value[:open])
	parameters := strings.Split(value[open+1:len(value)-1], ",")
	result := columnType{family: family, bounded: true}
	first, err := strconv.Atoi(strings.TrimSpace(parameters[0]))
	if err != nil {
		return columnType{family: value}
	}
	result.precision = first
	if len(parameters) > 1 {
		second, err := strconv.Atoi(strings.TrimSpace(parameters[1]))
		if err != nil {
			return columnType{family: value}
		}
		result.scale = second
	}
	return result
}

// columnImpact classifies one ALTER COLUMN by what it does to rows that already
// exist. The distinction that matters most is between a change PostgreSQL
// refuses (the migration rolls back, data intact) and one it performs silently
// (values are rewritten and nothing says so afterwards).
func columnImpact(current, wanted Column) ChangeImpact {
	if current.Type != wanted.Type {
		if impact := typeChangeImpact(parseColumnType(current.Type), parseColumnType(wanted.Type)); impact != ImpactNone {
			return impact
		}
	}
	if !wanted.Nullable && current.Nullable {
		return ImpactMayFail
	}
	return ImpactNone
}

// textual reports the types PostgreSQL moves between freely as long as the
// target is not shorter: text, character varying(n) and character(n) are one
// family for this purpose even though their names differ.
func textual(family string) bool {
	return family == "text" || family == "character varying" || family == "character"
}

// integerDigits reports how many decimal digits an integer type can hold, and
// whether the type is an integer type at all. ML's own schema uses integer and
// bigint for line numbers and versions, so these are not hypothetical.
func integerDigits(family string) (int, bool) {
	switch family {
	case "smallint":
		return 5, true
	case "integer":
		return 10, true
	case "bigint":
		return 19, true
	default:
		return 0, false
	}
}

// numericLike reports the types PostgreSQL converts between with an assignment
// cast - no USING clause needed, but not always without changing the value.
func numericLike(family string) bool {
	_, ok := integerDigits(family)
	return ok || family == "numeric"
}

func typeChangeImpact(current, wanted columnType) ChangeImpact {
	if textual(current.family) && textual(wanted.family) {
		// Widening to text always succeeds; a shorter limit is refused by any
		// value that does not fit, which aborts the migration.
		if wanted.bounded && (!current.bounded || wanted.precision < current.precision) {
			return ImpactMayFail
		}
		return ImpactNone
	}
	if numericLike(current.family) && numericLike(wanted.family) {
		return numericChangeImpact(current, wanted)
	}
	if current.family != wanted.family {
		// Changing the family re-encodes every value in the column - a number
		// becomes its text, a text is parsed into a number, a single value
		// becomes a composite one. Some of those conversions are refused by the
		// data and some are not, so this is confirmed like any other rewrite.
		return ImpactValueRewrite
	}
	return ImpactNone
}

// numericChangeImpact covers the number types, where PostgreSQL always has a
// cast and the only question is what it does to the value: drop decimals
// silently, or refuse what does not fit.
func numericChangeImpact(current, wanted columnType) ChangeImpact {
	currentDigits, currentInteger := integerDigits(current.family)
	wantedDigits, wantedInteger := integerDigits(wanted.family)
	switch {
	case currentInteger && wantedInteger:
		if wantedDigits < currentDigits {
			return ImpactMayFail
		}
		return ImpactNone
	case currentInteger && !wantedInteger:
		// Into numeric: safe unless its integer part is too small to hold them.
		if wanted.bounded && wanted.precision-wanted.scale < currentDigits {
			return ImpactMayFail
		}
		return ImpactNone
	case !currentInteger && wantedInteger:
		// Into an integer type: PostgreSQL rounds, silently.
		return ImpactValueRewrite
	case wanted.bounded && (!current.bounded || wanted.scale < current.scale):
		return ImpactValueRewrite
	case wanted.bounded && current.bounded && wanted.precision-wanted.scale < current.precision-current.scale:
		return ImpactMayFail
	}
	return ImpactNone
}

// ImpactMeasurement answers "how many rows does this actually touch". A plan
// says a column will be rounded; only the data says whether that is 0 rows or
// every row in the table, and that is the difference between a routine change
// and one an operator must not confirm without looking.
type ImpactMeasurement struct {
	Table  string       `json:"table"`
	Column string       `json:"column"`
	Impact ChangeImpact `json:"impact"`
	Rows   int64        `json:"rows"`
	Reason string       `json:"reason"`
}

// rowCounter is the subset of a pgx pool, connection or transaction this needs.
type rowCounter interface {
	QueryRow(ctx context.Context, sql string, arguments ...any) pgx.Row
}

// Measure counts the rows each data-touching change would affect. It reads only
// tables that already exist, so a plan that merely creates things measures as
// nothing. Counting is deliberately done BEFORE the migration: afterwards the
// rounded values are indistinguishable from values that were always round.
func Measure(ctx context.Context, counter rowCounter, plan Plan, actual Schema) ([]ImpactMeasurement, error) {
	existing := tableMap(actual.Tables)
	result := make([]ImpactMeasurement, 0)
	for _, change := range plan.Changes {
		table, ok := existing[change.Table]
		if !ok || change.Impact == ImpactNone || change.Impact == ImpactObjectLoss {
			continue
		}
		condition, reason := "", ""
		switch change.Kind {
		case AddColumn:
			// Every existing row blocks a NOT NULL column without a default.
			condition, reason = "TRUE", "в таблице есть строки — NOT NULL без значения по умолчанию не применится"
		case AlterColumn:
			before, after, ok := alterColumnPair(change)
			if !ok {
				continue
			}
			condition, reason = alterColumnCondition(before, after)
		default:
			continue
		}
		if condition == "" {
			continue
		}
		count, err := countRows(ctx, counter, actual.Name, table.Name, condition)
		if err != nil {
			return nil, err
		}
		result = append(result, ImpactMeasurement{Table: change.Table, Column: change.Object, Impact: change.Impact, Rows: count, Reason: reason})
	}
	return result, nil
}

func alterColumnPair(change Change) (Column, Column, bool) {
	before, beforeOK := change.Before.(Column)
	after, afterOK := change.After.(Column)
	return before, after, beforeOK && afterOK
}

// alterColumnCondition builds the WHERE that selects exactly the rows the change
// would touch, and says in words what those rows are.
func alterColumnCondition(before, after Column) (string, string) {
	column := quoteName(before.Name)
	current, wanted := parseColumnType(before.Type), parseColumnType(after.Type)
	if before.Type != after.Type && current.family == wanted.family {
		switch current.family {
		case "numeric":
			if wanted.bounded && (!current.bounded || wanted.scale < current.scale) {
				return fmt.Sprintf("%s IS NOT NULL AND %s <> round(%s, %d)", column, column, column, wanted.scale),
					fmt.Sprintf("значения будут округлены до %d знаков после запятой", wanted.scale)
			}
			if wanted.bounded && current.bounded && wanted.precision-wanted.scale < current.precision-current.scale {
				return fmt.Sprintf("%s IS NOT NULL AND abs(%s) >= 10::numeric ^ %d", column, column, wanted.precision-wanted.scale),
					"значения не помещаются в целую часть нового типа — миграция будет отклонена"
			}
		}
	}
	if before.Type != after.Type && textual(current.family) && textual(wanted.family) && wanted.bounded {
		return fmt.Sprintf("%s IS NOT NULL AND length(%s) > %d", column, column, wanted.precision),
			fmt.Sprintf("значения длиннее %d символов — миграция будет отклонена", wanted.precision)
	}
	if !after.Nullable && before.Nullable {
		return column + " IS NULL", "значения NULL — миграция будет отклонена"
	}
	return "", ""
}

func countRows(ctx context.Context, counter rowCounter, schema, table, condition string) (int64, error) {
	statement := "SELECT count(*) FROM " + qualified(schema, table) + " WHERE " + condition
	var count int64
	if err := counter.QueryRow(ctx, statement).Scan(&count); err != nil {
		return 0, fmt.Errorf("measure migration impact on %s: %w", table, err)
	}
	return count, nil
}
