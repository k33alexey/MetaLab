// Package schemadiff models and compares a desired ML application schema with PostgreSQL.
package schemadiff

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

const ApplicationSchema = "ml_data"

var sqlIdentifier = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)

type Schema struct {
	Name   string  `json:"name"`
	Exists bool    `json:"exists"`
	Tables []Table `json:"tables"`
}

type Table struct {
	Name        string       `json:"name"`
	Columns     []Column     `json:"columns"`
	Indexes     []Index      `json:"indexes"`
	Constraints []Constraint `json:"constraints"`
}

type Column struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
	Default  string `json:"default,omitempty"`
}

type Index struct {
	Name      string   `json:"name"`
	Unique    bool     `json:"unique"`
	Method    string   `json:"method"`
	Keys      []string `json:"keys"`
	Include   []string `json:"include,omitempty"`
	Predicate string   `json:"predicate,omitempty"`
}

type Constraint struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Definition string `json:"definition"`
}

func TableName(id uuid.UUID) (string, error)      { return physicalName("t", id) }
func ColumnName(id uuid.UUID) (string, error)     { return physicalName("c", id) }
func IndexName(id uuid.UUID) (string, error)      { return physicalName("i", id) }
func ConstraintName(id uuid.UUID) (string, error) { return physicalName("k", id) }

func physicalName(prefix string, id uuid.UUID) (string, error) {
	if id.IsZero() {
		return "", fmt.Errorf("physical PostgreSQL name requires a non-zero UUID")
	}
	return prefix + "_" + strings.ReplaceAll(id.String(), "-", ""), nil
}

func (schema *Schema) NormalizeAndValidate() error {
	return schema.normalizeAndValidate(true)
}

func (schema *Schema) normalizeActual() error {
	return schema.normalizeAndValidate(false)
}

func (schema *Schema) normalizeAndValidate(strictNames bool) error {
	if !sqlIdentifier.MatchString(schema.Name) {
		return fmt.Errorf("invalid PostgreSQL schema name %q", schema.Name)
	}
	sort.Slice(schema.Tables, func(i, j int) bool { return schema.Tables[i].Name < schema.Tables[j].Name })
	if duplicateTable := duplicateName(schema.Tables, func(table Table) string { return table.Name }); duplicateTable != "" {
		return fmt.Errorf("duplicate PostgreSQL table %q", duplicateTable)
	}
	for tableIndex := range schema.Tables {
		table := &schema.Tables[tableIndex]
		if !validObjectName(table.Name, strictNames) {
			return fmt.Errorf("invalid PostgreSQL table name %q", table.Name)
		}
		sort.Slice(table.Columns, func(i, j int) bool { return table.Columns[i].Name < table.Columns[j].Name })
		sort.Slice(table.Indexes, func(i, j int) bool { return table.Indexes[i].Name < table.Indexes[j].Name })
		sort.Slice(table.Constraints, func(i, j int) bool { return table.Constraints[i].Name < table.Constraints[j].Name })
		if name := duplicateName(table.Columns, func(column Column) string { return column.Name }); name != "" {
			return fmt.Errorf("duplicate PostgreSQL column %q.%q", table.Name, name)
		}
		if name := duplicateName(table.Indexes, func(index Index) string { return index.Name }); name != "" {
			return fmt.Errorf("duplicate PostgreSQL index %q", name)
		}
		if name := duplicateName(table.Constraints, func(constraint Constraint) string { return constraint.Name }); name != "" {
			return fmt.Errorf("duplicate PostgreSQL constraint %q.%q", table.Name, name)
		}
		for columnIndex := range table.Columns {
			column := &table.Columns[columnIndex]
			column.Type, column.Default = normalizeSQL(column.Type), normalizeSQL(column.Default)
			if !validObjectName(column.Name, strictNames) || column.Type == "" {
				return fmt.Errorf("invalid PostgreSQL column %q.%q", table.Name, column.Name)
			}
		}
		for index := range table.Indexes {
			item := &table.Indexes[index]
			item.Method, item.Predicate = strings.ToLower(strings.TrimSpace(item.Method)), normalizeSQL(item.Predicate)
			if item.Method == "" {
				item.Method = "btree"
			}
			if !validObjectName(item.Name, strictNames) || len(item.Keys) == 0 {
				return fmt.Errorf("invalid PostgreSQL index %q", item.Name)
			}
			for key := range item.Keys {
				item.Keys[key] = normalizeSQL(item.Keys[key])
			}
			for included := range item.Include {
				item.Include[included] = normalizeSQL(item.Include[included])
			}
			if len(item.Include) == 0 {
				item.Include = nil
			}
		}
		for constraint := range table.Constraints {
			item := &table.Constraints[constraint]
			item.Type, item.Definition = strings.ToLower(strings.TrimSpace(item.Type)), normalizeSQL(item.Definition)
			if !validObjectName(item.Name, strictNames) || !validConstraintType(item.Type) || item.Definition == "" {
				return fmt.Errorf("invalid PostgreSQL constraint %q.%q", table.Name, item.Name)
			}
		}
	}
	return nil
}

func validObjectName(value string, strict bool) bool {
	if strict {
		return sqlIdentifier.MatchString(value)
	}
	return value != "" && len(value) <= 63 && !strings.ContainsRune(value, 0)
}

func cloneSchema(source Schema) Schema {
	result := source
	result.Tables = slices.Clone(source.Tables)
	for tableIndex := range result.Tables {
		result.Tables[tableIndex].Columns = slices.Clone(source.Tables[tableIndex].Columns)
		result.Tables[tableIndex].Indexes = slices.Clone(source.Tables[tableIndex].Indexes)
		result.Tables[tableIndex].Constraints = slices.Clone(source.Tables[tableIndex].Constraints)
		for index := range result.Tables[tableIndex].Indexes {
			result.Tables[tableIndex].Indexes[index].Keys = slices.Clone(source.Tables[tableIndex].Indexes[index].Keys)
			result.Tables[tableIndex].Indexes[index].Include = slices.Clone(source.Tables[tableIndex].Indexes[index].Include)
		}
	}
	return result
}

func validConstraintType(value string) bool {
	switch value {
	case "primary_key", "unique", "foreign_key", "check", "exclusion":
		return true
	default:
		return false
	}
}

func normalizeSQL(value string) string { return strings.Join(strings.Fields(value), " ") }

func duplicateName[T any](items []T, name func(T) string) string {
	for index := 1; index < len(items); index++ {
		if name(items[index-1]) == name(items[index]) {
			return name(items[index])
		}
	}
	return ""
}
