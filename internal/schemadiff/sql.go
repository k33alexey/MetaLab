package schemadiff

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
)

func migrationStatements(plan Plan) ([]string, error) {
	var createSchemas, createTables, dropConstraints, dropIndexes, columns, dropTables, addConstraints, addIndexes []string
	constraintItems := make([]constraintTarget, 0)
	indexItems := make([]indexTarget, 0)
	for _, change := range plan.Changes {
		switch change.Kind {
		case CreateSchema:
			createSchemas = append(createSchemas, "CREATE SCHEMA "+quoteName(plan.Schema))
		case CreateTable:
			table, ok := change.After.(Table)
			if !ok {
				return nil, fmt.Errorf("invalid create-table migration plan")
			}
			statement, err := createTableSQL(plan.Schema, table)
			if err != nil {
				return nil, err
			}
			createTables = append(createTables, statement)
			for _, constraint := range table.Constraints {
				constraintItems = append(constraintItems, constraintTarget{table.Name, constraint})
			}
			for _, index := range table.Indexes {
				indexItems = append(indexItems, indexTarget{table.Name, index})
			}
		case DropTable:
			dropTables = append(dropTables, "DROP TABLE "+qualified(plan.Schema, change.Table))
		case AddColumn:
			column, ok := change.After.(Column)
			if !ok {
				return nil, fmt.Errorf("invalid add-column migration plan")
			}
			definition, err := columnSQL(column)
			if err != nil {
				return nil, err
			}
			columns = append(columns, "ALTER TABLE "+qualified(plan.Schema, change.Table)+" ADD COLUMN "+definition)
		case AlterColumn:
			before, beforeOK := change.Before.(Column)
			after, afterOK := change.After.(Column)
			if !beforeOK || !afterOK {
				return nil, fmt.Errorf("invalid alter-column migration plan")
			}
			statements, err := alterColumnSQL(plan.Schema, change.Table, before, after)
			if err != nil {
				return nil, err
			}
			columns = append(columns, statements...)
		case DropColumn:
			columns = append(columns, "ALTER TABLE "+qualified(plan.Schema, change.Table)+" DROP COLUMN "+quoteName(change.Object))
		case CreateIndex:
			index, ok := change.After.(Index)
			if !ok {
				return nil, fmt.Errorf("invalid create-index migration plan")
			}
			indexItems = append(indexItems, indexTarget{change.Table, index})
		case ReplaceIndex:
			index, ok := change.After.(Index)
			if !ok {
				return nil, fmt.Errorf("invalid replace-index migration plan")
			}
			dropIndexes = append(dropIndexes, "DROP INDEX "+qualified(plan.Schema, change.Object))
			indexItems = append(indexItems, indexTarget{change.Table, index})
		case DropIndex:
			dropIndexes = append(dropIndexes, "DROP INDEX "+qualified(plan.Schema, change.Object))
		case AddConstraint:
			constraint, ok := change.After.(Constraint)
			if !ok {
				return nil, fmt.Errorf("invalid add-constraint migration plan")
			}
			constraintItems = append(constraintItems, constraintTarget{change.Table, constraint})
		case ReplaceConstraint:
			constraint, ok := change.After.(Constraint)
			if !ok {
				return nil, fmt.Errorf("invalid replace-constraint migration plan")
			}
			dropConstraints = append(dropConstraints, "ALTER TABLE "+qualified(plan.Schema, change.Table)+" DROP CONSTRAINT "+quoteName(change.Object))
			constraintItems = append(constraintItems, constraintTarget{change.Table, constraint})
		case DropConstraint:
			dropConstraints = append(dropConstraints, "ALTER TABLE "+qualified(plan.Schema, change.Table)+" DROP CONSTRAINT "+quoteName(change.Object))
		default:
			return nil, fmt.Errorf("unsupported migration change %q", change.Kind)
		}
	}
	sort.SliceStable(constraintItems, func(i, j int) bool {
		return constraintPriority(constraintItems[i].constraint.Type) < constraintPriority(constraintItems[j].constraint.Type)
	})
	for _, target := range constraintItems {
		statement, err := addConstraintSQL(plan.Schema, target.table, target.constraint)
		if err != nil {
			return nil, err
		}
		addConstraints = append(addConstraints, statement)
	}
	for _, target := range indexItems {
		statement, err := createIndexSQL(plan.Schema, target.table, target.index)
		if err != nil {
			return nil, err
		}
		addIndexes = append(addIndexes, statement)
	}
	result := make([]string, 0)
	for _, phase := range [][]string{createSchemas, createTables, dropConstraints, dropIndexes, columns, dropTables, addConstraints, addIndexes} {
		result = append(result, phase...)
	}
	return result, nil
}

type constraintTarget struct {
	table      string
	constraint Constraint
}
type indexTarget struct {
	table string
	index Index
}

func createTableSQL(schema string, table Table) (string, error) {
	columns := make([]string, len(table.Columns))
	for index, column := range table.Columns {
		definition, err := columnSQL(column)
		if err != nil {
			return "", err
		}
		columns[index] = definition
	}
	return "CREATE TABLE " + qualified(schema, table.Name) + " (" + strings.Join(columns, ", ") + ")", nil
}

func columnSQL(column Column) (string, error) {
	if err := safeFragment(column.Type); err != nil {
		return "", fmt.Errorf("unsafe type for column %s: %w", column.Name, err)
	}
	result := quoteName(column.Name) + " " + column.Type
	if column.Default != "" {
		if err := safeFragment(column.Default); err != nil {
			return "", fmt.Errorf("unsafe default for column %s: %w", column.Name, err)
		}
		result += " DEFAULT " + column.Default
	}
	if !column.Nullable {
		result += " NOT NULL"
	}
	return result, nil
}

func alterColumnSQL(schema, table string, before, after Column) ([]string, error) {
	prefix := "ALTER TABLE " + qualified(schema, table) + " ALTER COLUMN " + quoteName(after.Name)
	result := make([]string, 0, 3)
	if before.Type != after.Type {
		if err := safeFragment(after.Type); err != nil {
			return nil, err
		}
		result = append(result, prefix+" TYPE "+after.Type)
	}
	if before.Default != after.Default {
		if after.Default == "" {
			result = append(result, prefix+" DROP DEFAULT")
		} else {
			if err := safeFragment(after.Default); err != nil {
				return nil, err
			}
			result = append(result, prefix+" SET DEFAULT "+after.Default)
		}
	}
	if before.Nullable != after.Nullable {
		if after.Nullable {
			result = append(result, prefix+" DROP NOT NULL")
		} else {
			result = append(result, prefix+" SET NOT NULL")
		}
	}
	return result, nil
}

func createIndexSQL(schema, table string, index Index) (string, error) {
	method := strings.ToLower(index.Method)
	switch method {
	case "btree", "hash", "gist", "spgist", "gin", "brin":
	default:
		return "", fmt.Errorf("unsupported PostgreSQL index method %q", method)
	}
	keys := make([]string, len(index.Keys))
	for position, key := range index.Keys {
		var err error
		keys[position], err = indexTerm(key)
		if err != nil {
			return "", err
		}
	}
	statement := "CREATE "
	if index.Unique {
		statement += "UNIQUE "
	}
	statement += "INDEX " + quoteName(index.Name) + " ON " + qualified(schema, table) + " USING " + method + " (" + strings.Join(keys, ", ") + ")"
	if len(index.Include) > 0 {
		included := make([]string, len(index.Include))
		for position, value := range index.Include {
			if !sqlIdentifier.MatchString(value) {
				return "", fmt.Errorf("unsupported included index column %q", value)
			}
			included[position] = quoteName(value)
		}
		statement += " INCLUDE (" + strings.Join(included, ", ") + ")"
	}
	if index.Predicate != "" {
		if err := safeFragment(index.Predicate); err != nil {
			return "", err
		}
		statement += " WHERE " + index.Predicate
	}
	return statement, nil
}

func indexTerm(value string) (string, error) {
	if sqlIdentifier.MatchString(value) {
		return quoteName(value), nil
	}
	if err := safeFragment(value); err != nil {
		return "", err
	}
	return value, nil
}

func addConstraintSQL(schema, table string, constraint Constraint) (string, error) {
	if err := safeFragment(constraint.Definition); err != nil {
		return "", err
	}
	prefixes := map[string]string{"primary_key": "PRIMARY KEY", "unique": "UNIQUE", "foreign_key": "FOREIGN KEY", "check": "CHECK", "exclusion": "EXCLUDE"}
	wanted, ok := prefixes[constraint.Type]
	if !ok || !strings.HasPrefix(strings.ToUpper(constraint.Definition), wanted) {
		return "", fmt.Errorf("constraint %q definition does not match its type", constraint.Name)
	}
	return "ALTER TABLE " + qualified(schema, table) + " ADD CONSTRAINT " + quoteName(constraint.Name) + " " + constraint.Definition, nil
}

func safeFragment(value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || strings.ContainsRune(trimmed, 0) || strings.Contains(trimmed, ";") || strings.Contains(trimmed, "--") || strings.Contains(trimmed, "/*") || strings.Contains(trimmed, "*/") {
		return fmt.Errorf("unsafe PostgreSQL expression")
	}
	return nil
}

func quoteName(value string) string          { return pgx.Identifier{value}.Sanitize() }
func qualified(schema, object string) string { return pgx.Identifier{schema, object}.Sanitize() }

func constraintPriority(value string) int {
	if value == "foreign_key" {
		return 1
	}
	return 0
}
