package schemadiff

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestExecuteRequiresExplicitConfirmationBeforeDatabaseAccess(t *testing.T) {
	_, err := Execute(context.Background(), nil, MigrationRequest{})
	if !errors.Is(err, ErrConfirmationRequired) {
		t.Fatalf("Execute() error = %v", err)
	}
}

func TestPlanAndTargetDigestsAreStableAndDoNotMutateInput(t *testing.T) {
	schema := Schema{Name: ApplicationSchema, Tables: []Table{{Name: "t_demo", Columns: []Column{
		{Name: "z", Type: " TEXT ", Nullable: true}, {Name: "a", Type: "uuid"},
	}}}}
	original := cloneSchema(schema)
	first, err := SchemaSHA256(schema)
	if err != nil {
		t.Fatal(err)
	}
	second, err := SchemaSHA256(schema)
	if err != nil || first != second || len(first) != 64 {
		t.Fatalf("digests %q %q error=%v", first, second, err)
	}
	if !reflect.DeepEqual(schema, original) {
		t.Fatal("SchemaSHA256 mutated its input")
	}
	plan := Plan{Schema: ApplicationSchema, Changes: []Change{{Kind: CreateSchema, After: ApplicationSchema}}}
	if digest, err := PlanSHA256(plan); err != nil || len(digest) != 64 {
		t.Fatalf("plan digest=%q error=%v", digest, err)
	}
}

func TestMigrationStatementsQuoteNamesOrderDependenciesAndRejectInjection(t *testing.T) {
	table := Table{Name: "t_demo", Columns: []Column{{Name: "id", Type: "uuid", Nullable: false}},
		Indexes:     []Index{{Name: "i_title", Method: "btree", Keys: []string{"title"}}},
		Constraints: []Constraint{{Name: "pk_demo", Type: "primary_key", Definition: "PRIMARY KEY (id)"}}}
	plan := Plan{Schema: ApplicationSchema, Changes: []Change{
		{Kind: CreateSchema, After: ApplicationSchema}, {Kind: CreateTable, Table: table.Name, After: table},
		{Kind: AddColumn, Table: table.Name, Object: "title", After: Column{Name: "title", Type: "text", Nullable: true}},
	}}
	statements, err := migrationStatements(plan)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(statements, "\n")
	for _, fragment := range []string{`CREATE SCHEMA "ml_data"`, `CREATE TABLE "ml_data"."t_demo"`, `ADD COLUMN "title" text`, `ADD CONSTRAINT "pk_demo" PRIMARY KEY`, `CREATE INDEX "i_title"`} {
		if !strings.Contains(joined, fragment) {
			t.Fatalf("statements do not contain %q:\n%s", fragment, joined)
		}
	}
	constraintPosition, indexPosition := strings.Index(joined, "ADD CONSTRAINT"), strings.Index(joined, "CREATE INDEX")
	if constraintPosition < 0 || indexPosition < constraintPosition {
		t.Fatalf("dependency order is unsafe:\n%s", joined)
	}
	_, err = migrationStatements(Plan{Schema: ApplicationSchema, Changes: []Change{{
		Kind: AddColumn, Table: "t_demo", After: Column{Name: "bad", Type: "text; DROP TABLE users", Nullable: true},
	}}})
	if err == nil {
		t.Fatal("migration SQL accepted statement injection")
	}
}

func TestMigrationStatementsReplaceDependenciesBeforeColumnChanges(t *testing.T) {
	plan := Plan{Schema: ApplicationSchema, Changes: []Change{
		{Kind: AlterColumn, Table: "t_demo", Object: "value", Before: Column{Name: "value", Type: "integer", Nullable: true}, After: Column{Name: "value", Type: "bigint", Nullable: false, Default: "0"}},
		{Kind: ReplaceIndex, Table: "t_demo", Object: "i_value", Before: Index{Name: "i_value", Method: "btree", Keys: []string{"value"}}, After: Index{Name: "i_value", Method: "btree", Keys: []string{"value"}, Unique: true}},
		{Kind: ReplaceConstraint, Table: "t_demo", Object: "k_value", Before: Constraint{Name: "k_value", Type: "check", Definition: "CHECK (value >= 0)"}, After: Constraint{Name: "k_value", Type: "check", Definition: "CHECK (value > 0)"}},
		{Kind: DropColumn, Table: "t_demo", Object: "obsolete", Before: Column{Name: "obsolete", Type: "text"}},
		{Kind: DropTable, Table: "t_old", Before: Table{Name: "t_old"}},
	}}
	statements, err := migrationStatements(plan)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(statements, "\n")
	positions := []int{
		strings.Index(joined, "DROP CONSTRAINT"), strings.Index(joined, "DROP INDEX"),
		strings.Index(joined, "TYPE bigint"), strings.Index(joined, "DROP COLUMN"), strings.Index(joined, "DROP TABLE"),
		strings.LastIndex(joined, "ADD CONSTRAINT"), strings.LastIndex(joined, "CREATE UNIQUE INDEX"),
	}
	for index := 1; index < len(positions); index++ {
		if positions[index-1] < 0 || positions[index] <= positions[index-1] {
			t.Fatalf("unsafe migration order at %d: %v\n%s", index, positions, joined)
		}
	}
	_, err = migrationStatements(Plan{Schema: ApplicationSchema, Changes: []Change{{
		Kind: AddConstraint, Table: "t_demo", After: Constraint{Name: "k_bad", Type: "check", Definition: "UNIQUE (value)"},
	}}})
	if err == nil {
		t.Fatal("migration accepted a constraint definition of another type")
	}
}
