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

// ChangeImpact says what a change does to data that already exists. One flag
// for "destructive" is not enough to decide anything: dropping an attribute and
// rounding every price in the table are both destructive, and an operator who
// meant the first must not be silently granting the second.
type ChangeImpact string

const (
	// ImpactNone - existing rows are untouched.
	ImpactNone ChangeImpact = ""
	// ImpactObjectLoss - something is removed: a table, a column, an index, a
	// constraint. What it held is gone and cannot be recovered from the database.
	ImpactObjectLoss ChangeImpact = "object_loss"
	// ImpactValueRewrite - rows keep existing but their values change, silently:
	// PostgreSQL rounds numeric(10,2) to numeric(10,0) without a word. This is
	// the dangerous one, because nothing reports it afterwards.
	ImpactValueRewrite ChangeImpact = "value_rewrite"
	// ImpactMayFail - the change can be rejected by the data itself: a value too
	// long for the narrowed type, a NULL where NOT NULL is now required, a
	// duplicate under a new unique index. The whole migration then rolls back,
	// so data is safe - but the operator deserves to know before starting.
	ImpactMayFail ChangeImpact = "may_fail"
)

type Change struct {
	Kind ChangeKind `json:"kind"`
	// Destructive stays as the coarse answer to "does this touch existing
	// data", with Impact saying in which way.
	Destructive bool         `json:"destructive"`
	Impact      ChangeImpact `json:"impact,omitempty"`
	Table       string       `json:"table,omitempty"`
	Object      string       `json:"object,omitempty"`
	Before      any          `json:"before,omitempty"`
	After       any          `json:"after,omitempty"`
}

type Plan struct {
	Schema           string   `json:"schema"`
	Changes          []Change `json:"changes"`
	DestructiveCount int      `json:"destructiveCount"`
	// ObjectLossCount and ValueRewriteCount are what the two separate
	// confirmations are about; MayFailCount needs no permission - it cannot
	// damage anything, it can only abort the migration.
	ObjectLossCount   int `json:"objectLossCount"`
	ValueRewriteCount int `json:"valueRewriteCount"`
	MayFailCount      int `json:"mayFailCount"`
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
			plan.add(Change{Kind: DropTable, Table: name, Impact: ImpactObjectLoss, Before: actualTable})
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
			// A new column that is NOT NULL without a default cannot be added to
			// a table that has rows - the migration aborts, nothing is lost.
			impact := ImpactNone
			if !wanted.Nullable && wanted.Default == "" {
				impact = ImpactMayFail
			}
			plan.add(Change{Kind: AddColumn, Table: desired.Name, Object: name, Impact: impact, After: wanted})
		case !wantedOK && currentOK:
			plan.add(Change{Kind: DropColumn, Table: desired.Name, Object: name, Impact: ImpactObjectLoss, Before: current})
		case !reflect.DeepEqual(wanted, current):
			plan.add(Change{Kind: AlterColumn, Table: desired.Name, Object: name, Impact: columnImpact(current, wanted), Before: current, After: wanted})
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
			plan.add(Change{Kind: drop, Table: table, Object: name, Impact: ImpactObjectLoss, Before: current})
		case !reflect.DeepEqual(wanted, current):
			// Replacing an index or a constraint drops the old one first, and the
			// new one can be rejected by data the old one allowed.
			plan.add(Change{Kind: replace, Table: table, Object: name, Impact: ImpactObjectLoss, Before: current, After: wanted})
		}
	}
}

func (plan *Plan) add(change Change) {
	change.Destructive = change.Impact != ImpactNone
	plan.Changes = append(plan.Changes, change)
	if !change.Destructive {
		return
	}
	plan.DestructiveCount++
	switch change.Impact {
	case ImpactObjectLoss:
		plan.ObjectLossCount++
	case ImpactValueRewrite:
		plan.ValueRewriteCount++
	case ImpactMayFail:
		plan.MayFailCount++
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
