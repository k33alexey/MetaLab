package schemadiff

import (
	"fmt"
	"reflect"
	"sort"
)

type ChangeKind string

const (
	CreateSchema      ChangeKind = "create_schema"
	CreateTable       ChangeKind = "create_table"
	DropTable         ChangeKind = "drop_table"
	AddColumn         ChangeKind = "add_column"
	AlterColumn       ChangeKind = "alter_column"
	DropColumn        ChangeKind = "drop_column"
	CreateIndex       ChangeKind = "create_index"
	ReplaceIndex      ChangeKind = "replace_index"
	DropIndex         ChangeKind = "drop_index"
	AddConstraint     ChangeKind = "add_constraint"
	ReplaceConstraint ChangeKind = "replace_constraint"
	DropConstraint    ChangeKind = "drop_constraint"
)

type Change struct {
	Kind        ChangeKind `json:"kind"`
	Table       string     `json:"table,omitempty"`
	Object      string     `json:"object,omitempty"`
	Destructive bool       `json:"destructive"`
	Before      any        `json:"before,omitempty"`
	After       any        `json:"after,omitempty"`
}

type Plan struct {
	Schema           string   `json:"schema"`
	Changes          []Change `json:"changes"`
	DestructiveCount int      `json:"destructiveCount"`
}

func Compare(desired, actual Schema) (Plan, error) {
	desired, actual = cloneSchema(desired), cloneSchema(actual)
	if err := desired.NormalizeAndValidate(); err != nil {
		return Plan{}, fmt.Errorf("validate desired schema: %w", err)
	}
	if err := actual.normalizeActual(); err != nil {
		return Plan{}, fmt.Errorf("validate actual schema: %w", err)
	}
	if desired.Name != actual.Name {
		return Plan{}, fmt.Errorf("cannot compare different PostgreSQL schemas")
	}
	plan := Plan{Schema: desired.Name, Changes: make([]Change, 0)}
	if !actual.Exists {
		plan.Changes = append(plan.Changes, Change{Kind: CreateSchema, After: desired.Name})
	}
	desiredTables, actualTables := tableMap(desired.Tables), tableMap(actual.Tables)
	for _, name := range unionKeys(desiredTables, actualTables) {
		desiredTable, wanted := desiredTables[name]
		actualTable, exists := actualTables[name]
		switch {
		case wanted && !exists:
			plan.Changes = append(plan.Changes, Change{Kind: CreateTable, Table: name, After: desiredTable})
		case !wanted && exists:
			plan.add(Change{Kind: DropTable, Table: name, Destructive: true, Before: actualTable})
		default:
			compareTable(&plan, desiredTable, actualTable)
		}
	}
	return plan, nil
}

func compareTable(plan *Plan, desired, actual Table) {
	desiredColumns, actualColumns := columnMap(desired.Columns), columnMap(actual.Columns)
	for _, name := range unionKeys(desiredColumns, actualColumns) {
		wanted, wantedOK := desiredColumns[name]
		current, currentOK := actualColumns[name]
		switch {
		case wantedOK && !currentOK:
			plan.add(Change{Kind: AddColumn, Table: desired.Name, Object: name, Destructive: !wanted.Nullable && wanted.Default == "", After: wanted})
		case !wantedOK && currentOK:
			plan.add(Change{Kind: DropColumn, Table: desired.Name, Object: name, Destructive: true, Before: current})
		case !reflect.DeepEqual(wanted, current):
			destructive := wanted.Type != current.Type || !wanted.Nullable && current.Nullable
			plan.add(Change{Kind: AlterColumn, Table: desired.Name, Object: name, Destructive: destructive, Before: current, After: wanted})
		}
	}
	compareNamed(plan, desired.Name, desired.Indexes, actual.Indexes, CreateIndex, ReplaceIndex, DropIndex)
	compareNamed(plan, desired.Name, desired.Constraints, actual.Constraints, AddConstraint, ReplaceConstraint, DropConstraint)
}

func compareNamed[T any](plan *Plan, table string, desired, actual []T, create, replace, drop ChangeKind) {
	desiredItems, actualItems := namedMap(desired), namedMap(actual)
	for _, name := range unionKeys(desiredItems, actualItems) {
		wanted, wantedOK := desiredItems[name]
		current, currentOK := actualItems[name]
		switch {
		case wantedOK && !currentOK:
			plan.add(Change{Kind: create, Table: table, Object: name, After: wanted})
		case !wantedOK && currentOK:
			plan.add(Change{Kind: drop, Table: table, Object: name, Destructive: true, Before: current})
		case !reflect.DeepEqual(wanted, current):
			plan.add(Change{Kind: replace, Table: table, Object: name, Destructive: true, Before: current, After: wanted})
		}
	}
}

func (plan *Plan) add(change Change) {
	plan.Changes = append(plan.Changes, change)
	if change.Destructive {
		plan.DestructiveCount++
	}
}

func tableMap(items []Table) map[string]Table {
	return keyed(items, func(item Table) string { return item.Name })
}
func columnMap(items []Column) map[string]Column {
	return keyed(items, func(item Column) string { return item.Name })
}

func namedMap[T any](items []T) map[string]T {
	return keyed(items, func(item T) string {
		switch value := any(item).(type) {
		case Index:
			return value.Name
		case Constraint:
			return value.Name
		default:
			panic("unsupported named schema object")
		}
	})
}

func keyed[T any](items []T, key func(T) string) map[string]T {
	result := make(map[string]T, len(items))
	for _, item := range items {
		result[key(item)] = item
	}
	return result
}

func unionKeys[T any](first, second map[string]T) []string {
	set := make(map[string]struct{}, len(first)+len(second))
	for key := range first {
		set[key] = struct{}{}
	}
	for key := range second {
		set[key] = struct{}{}
	}
	result := make([]string, 0, len(set))
	for key := range set {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}
