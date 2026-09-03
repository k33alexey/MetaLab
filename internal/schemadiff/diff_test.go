package schemadiff

import (
	"reflect"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestPhysicalNamesAreStableAndPostgreSQLSafe(t *testing.T) {
	id, err := uuid.Parse("550e8400-e29b-41d4-a716-446655440000")
	if err != nil {
		t.Fatal(err)
	}
	table, err := TableName(id)
	if err != nil || table != "t_550e8400e29b41d4a716446655440000" {
		t.Fatalf("table=%q error=%v", table, err)
	}
	column, err := ColumnName(id)
	if err != nil || column != "c_550e8400e29b41d4a716446655440000" {
		t.Fatalf("column=%q error=%v", column, err)
	}
	index, _ := IndexName(id)
	constraint, _ := ConstraintName(id)
	if index != "i_550e8400e29b41d4a716446655440000" || constraint != "k_550e8400e29b41d4a716446655440000" {
		t.Fatalf("index=%q constraint=%q", index, constraint)
	}
}

func TestCompareProducesStableCompleteAndRiskMarkedPlan(t *testing.T) {
	desired := Schema{Name: ApplicationSchema, Exists: true, Tables: []Table{
		{Name: "t_new", Columns: []Column{{Name: "id", Type: "uuid"}}},
		{Name: "t_shared", Columns: []Column{
			{Name: "added", Type: "text", Nullable: true},
			{Name: "changed", Type: "bigint", Nullable: false, Default: "0"},
			{Name: "same", Type: "text", Nullable: true},
		}, Indexes: []Index{
			{Name: "ix_added", Method: "btree", Keys: []string{"added"}},
			{Name: "ix_changed", Method: "btree", Keys: []string{"changed"}, Unique: true},
		}, Constraints: []Constraint{
			{Name: "ck_added", Type: "check", Definition: "CHECK (changed >= 0)"},
			{Name: "ck_changed", Type: "check", Definition: "CHECK (changed > 0)"},
		}},
	}}
	actual := Schema{Name: ApplicationSchema, Exists: true, Tables: []Table{
		{Name: "t_old", Columns: []Column{{Name: "id", Type: "uuid"}}},
		{Name: "t_shared", Columns: []Column{
			{Name: "changed", Type: "integer", Nullable: true},
			{Name: "removed", Type: "text", Nullable: true},
			{Name: "same", Type: "text", Nullable: true},
		}, Indexes: []Index{
			{Name: "ix_changed", Method: "btree", Keys: []string{"changed"}},
			{Name: "ix_removed", Method: "btree", Keys: []string{"removed"}},
		}, Constraints: []Constraint{
			{Name: "ck_changed", Type: "check", Definition: "CHECK (changed >= 0)"},
			{Name: "ck_removed", Type: "check", Definition: "CHECK (removed <> '')"},
		}},
	}}
	originalDesired := cloneSchema(desired)
	plan, err := Compare(desired, actual)
	if err != nil {
		t.Fatal(err)
	}
	wantKinds := []ChangeKind{CreateTable, DropTable, AddColumn, AlterColumn, DropColumn, CreateIndex, ReplaceIndex, DropIndex, AddConstraint, ReplaceConstraint, DropConstraint}
	if len(plan.Changes) != len(wantKinds) || plan.DestructiveCount != 7 {
		t.Fatalf("plan=%+v", plan)
	}
	for index, kind := range wantKinds {
		if plan.Changes[index].Kind != kind {
			t.Fatalf("change[%d]=%s, want %s; plan=%+v", index, plan.Changes[index].Kind, kind, plan)
		}
	}
	if !reflect.DeepEqual(desired, originalDesired) {
		t.Fatal("Compare mutated desired schema")
	}
}

func TestCompareMissingSchemaAndValidation(t *testing.T) {
	plan, err := Compare(
		Schema{Name: ApplicationSchema, Tables: []Table{{Name: "t_valid", Columns: []Column{{Name: "id", Type: "UUID"}}}}},
		Schema{Name: ApplicationSchema},
	)
	if err != nil || len(plan.Changes) != 2 || plan.Changes[0].Kind != CreateSchema || plan.Changes[1].Kind != CreateTable {
		t.Fatalf("plan=%+v error=%v", plan, err)
	}
	_, err = Compare(Schema{Name: "unsafe-name"}, Schema{Name: "unsafe-name"})
	if err == nil {
		t.Fatal("Compare accepted unsafe schema name")
	}
}

func TestCompareReportsManuallyCreatedPostgreSQLNames(t *testing.T) {
	plan, err := Compare(
		Schema{Name: ApplicationSchema, Exists: true},
		Schema{Name: ApplicationSchema, Exists: true, Tables: []Table{{Name: "Manual-Table", Columns: []Column{{Name: "Mixed Case", Type: "text"}}}}},
	)
	if err != nil || len(plan.Changes) != 1 || plan.Changes[0].Kind != DropTable || plan.Changes[0].Table != "Manual-Table" {
		t.Fatalf("plan=%+v error=%v", plan, err)
	}
}
