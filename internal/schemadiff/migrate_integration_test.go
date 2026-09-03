package schemadiff

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestMigrationConfirmationExecutionRecheckAndJournalIntegration(t *testing.T) {
	databaseURL := os.Getenv("ML_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("ML_TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	projectID := uuid.MustNew()
	schemaName := fmt.Sprintf("ml_migrate_%x", time.Now().UnixNano())
	quotedSchema := pgx.Identifier{schemaName}.Sanitize()
	t.Cleanup(func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupContext, "DROP SCHEMA IF EXISTS "+quotedSchema+" CASCADE")
		_, _ = pool.Exec(cleanupContext, "DELETE FROM ml_core.migration_journal WHERE project_id = $1", projectID.String())
		pool.Close()
	})
	desired := Schema{Name: schemaName, Exists: true, Tables: []Table{{
		Name: "t_demo",
		Columns: []Column{
			{Name: "id", Type: "uuid", Nullable: false},
			{Name: "title", Type: "text", Nullable: false, Default: "''::text"},
		},
		Indexes:     []Index{{Name: "i_demo_title", Method: "btree", Keys: []string{"title"}, Include: []string{"id"}}},
		Constraints: []Constraint{{Name: "k_demo_primary", Type: "primary_key", Definition: "PRIMARY KEY (id)"}},
	}}}
	prepared, err := Prepare(ctx, pool, desired)
	if err != nil || prepared.SHA256 == "" || prepared.TargetSHA256 == "" || len(prepared.Plan.Changes) != 2 {
		t.Fatalf("prepared=%+v error=%v", prepared, err)
	}
	request := MigrationRequest{
		ProjectID: projectID, PackageSHA256: repeatedHex('a', 64), GitCommit: repeatedHex('b', 40),
		Desired: desired, ExpectedPlanSHA256: prepared.SHA256,
	}
	if _, err := Execute(ctx, pool, request); !errors.Is(err, ErrConfirmationRequired) {
		t.Fatalf("unconfirmed Execute() error = %v", err)
	}
	request.Confirmed = true
	type executionResult struct {
		record MigrationRecord
		err    error
	}
	start := make(chan struct{})
	results := make(chan executionResult, 2)
	for range 2 {
		go func() {
			<-start
			executed, executeErr := Execute(ctx, pool, request)
			results <- executionResult{record: executed, err: executeErr}
		}()
	}
	close(start)
	var record MigrationRecord
	successes, stalePlans := 0, 0
	for range 2 {
		result := <-results
		switch {
		case result.err == nil:
			successes++
			record = result.record
		case errors.Is(result.err, ErrPlanChanged):
			stalePlans++
		default:
			t.Fatalf("concurrent Execute() error = %v", result.err)
		}
	}
	if successes != 1 || stalePlans != 1 {
		t.Fatalf("concurrent migrations succeeded=%d stale=%d", successes, stalePlans)
	}
	if record.Status != "succeeded" || record.SchemaSHA256 != prepared.TargetSHA256 || record.CompletedAt.IsZero() {
		t.Fatalf("record=%+v", record)
	}
	actual, err := Inspect(ctx, pool, schemaName)
	if err != nil {
		t.Fatal(err)
	}
	if plan, err := Compare(desired, actual); err != nil || len(plan.Changes) != 0 {
		t.Fatalf("post-migration plan=%+v error=%v actual=%+v", plan, err, actual)
	}

	destructive := cloneSchema(desired)
	destructive.Tables[0].Columns = destructive.Tables[0].Columns[:1]
	destructive.Tables[0].Indexes = nil
	destructivePlan, err := Prepare(ctx, pool, destructive)
	if err != nil || destructivePlan.Plan.DestructiveCount == 0 {
		t.Fatalf("destructive plan=%+v error=%v", destructivePlan, err)
	}
	request.Desired, request.ExpectedPlanSHA256 = destructive, destructivePlan.SHA256
	if _, err := Execute(ctx, pool, request); !errors.Is(err, ErrDestructiveDenied) {
		t.Fatalf("destructive Execute() error = %v", err)
	}
	request.AllowDestructive = true
	if record, err = Execute(ctx, pool, request); err != nil || record.Status != "succeeded" {
		t.Fatalf("destructive record=%+v error=%v", record, err)
	}

	changed := cloneSchema(destructive)
	changed.Tables[0].Columns = append(changed.Tables[0].Columns, Column{Name: "external", Type: "text", Nullable: true})
	changedPlan, err := Prepare(ctx, pool, changed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "ALTER TABLE "+quotedSchema+`.t_demo ADD COLUMN external text`); err != nil {
		t.Fatal(err)
	}
	request.Desired, request.ExpectedPlanSHA256 = changed, changedPlan.SHA256
	if _, err := Execute(ctx, pool, request); !errors.Is(err, ErrPlanChanged) {
		t.Fatalf("changed-plan Execute() error = %v", err)
	}

	failing := cloneSchema(changed)
	failing.Tables[0].Columns = append(failing.Tables[0].Columns,
		Column{Name: "agood", Type: "text", Nullable: true},
		Column{Name: "zbroken", Type: "no_such_postgresql_type", Nullable: true},
	)
	failingPlan, err := Prepare(ctx, pool, failing)
	if err != nil {
		t.Fatal(err)
	}
	request.Desired, request.ExpectedPlanSHA256 = failing, failingPlan.SHA256
	failed, err := Execute(ctx, pool, request)
	if err == nil || failed.Status != "failed" || failed.Error == "" {
		t.Fatalf("failed record=%+v error=%v", failed, err)
	}
	afterFailure, inspectErr := Inspect(ctx, pool, schemaName)
	if inspectErr != nil {
		t.Fatal(inspectErr)
	}
	for _, column := range afterFailure.Tables[0].Columns {
		if column.Name == "agood" || column.Name == "zbroken" {
			t.Fatalf("failed migration was not rolled back: %+v", afterFailure.Tables[0].Columns)
		}
	}
	migrations, err := ListMigrations(ctx, pool, 20)
	if err != nil {
		t.Fatal(err)
	}
	succeeded, failures := 0, 0
	for _, migration := range migrations {
		if migration.ProjectID != projectID {
			continue
		}
		if migration.Status == "succeeded" {
			succeeded++
		}
		if migration.Status == "failed" {
			failures++
		}
	}
	if succeeded != 2 || failures != 1 {
		t.Fatalf("migration journal succeeded=%d failed=%d entries=%+v", succeeded, failures, migrations)
	}
}

func repeatedHex(symbol byte, length int) string {
	value := make([]byte, length)
	for index := range value {
		value[index] = symbol
	}
	return string(value)
}
