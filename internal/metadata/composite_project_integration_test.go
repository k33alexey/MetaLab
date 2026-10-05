package metadata

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// The schema of the composite project is built on PostgreSQL, holds a table
// for every object that keeps data, and a second preparation over the built
// schema plans nothing.
//
// Defect caught: two kinds whose tables, indexes or constraints collide or
// that PostgreSQL refuses only together; a table that is planned and never
// built, or built differently from what is planned, so that every opening of
// the project plans the same migration again.
func TestCompositeProjectSchemaIntegration(t *testing.T) {
	databaseURL := os.Getenv("ML_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("ML_TEST_DATABASE_URL is not set")
	}
	catalog, err := Load(compositeProject(t))
	if err != nil {
		t.Fatal(err)
	}
	desired, err := catalog.ApplicationSchema()
	if err != nil {
		t.Fatal(err)
	}
	tables := map[string]bool{}
	for _, table := range desired.Tables {
		tables[table.Name] = true
	}
	for name, id := range map[string]string{
		"справочник Пользователи": cmpUsers, "справочник Склады": cmpWarehouses, "справочник Номенклатура": cmpGoods,
		"документ": cmpDocument, "план видов характеристик": cmpCharacteristics, "план счетов": cmpAccounts,
		"план видов расчёта": cmpCalculationTypes, "план обмена": cmpExchangePlan, "задача": cmpTask,
		"бизнес-процесс": cmpProcess, "регистр сведений": cmpPrices, "регистр накопления": cmpBalances,
		"регистр бухгалтерии": cmpEntries, "регистр расчёта": cmpPayroll,
	} {
		parsed, err := uuid.Parse(id)
		if err != nil {
			t.Fatal(err)
		}
		table, err := schemadiff.TableName(parsed)
		if err != nil {
			t.Fatal(err)
		}
		if !tables[table] {
			t.Errorf("%s has no table %s in the schema", name, table)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	projectID := uuid.MustNew()
	dropSchema := "DROP SCHEMA IF EXISTS " + pgx.Identifier{schemadiff.ApplicationSchema}.Sanitize() + " CASCADE"
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		_, _ = pool.Exec(cleanup, dropSchema)
		_, _ = pool.Exec(cleanup, "DELETE FROM ml_core.migration_journal WHERE project_id = $1", projectID.String())
		pool.Close()
	})
	if _, err := pool.Exec(ctx, dropSchema); err != nil {
		t.Fatal(err)
	}
	prepared, err := schemadiff.Prepare(ctx, pool, desired)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := schemadiff.Execute(ctx, pool, schemadiff.MigrationRequest{
		ProjectID: projectID, PackageSHA256: repeatCatalogHex('c', 64), GitCommit: repeatCatalogHex('d', 40), Desired: desired,
		ExpectedPlanSHA256: prepared.SHA256, ExpectedSchemaSHA256: prepared.ActualSHA256, Confirmed: true,
	}); err != nil {
		t.Fatal(err)
	}
	again, err := schemadiff.Prepare(ctx, pool, desired)
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Plan.Changes) != 0 {
		t.Fatalf("the built schema plans %d changes again: %+v", len(again.Plan.Changes), again.Plan.Changes)
	}
	refusesWhatTheModelForbids(ctx, t, pool, desired)
}

// refusesWhatTheModelForbids writes rows the model forbids straight into the
// built tables, and the database must refuse each: the numbers a register
// reads come out right with or without these constraints, and only the rows
// that should never have been written tell them apart.
//
// Defect caught: an entry of an accounting register unique by its document
// alone, so that the second line of any entry is refused - or not unique at
// all, so that a line is written twice and every balance counts it twice; a
// calculation record or a calculation type whose flags may be left empty, so
// that a record calculated by nothing is kept.
func refusesWhatTheModelForbids(ctx context.Context, t *testing.T, pool *pgxpool.Pool, desired schemadiff.Schema) {
	t.Helper()
	tableOf := func(id string) schemadiff.Table {
		parsed, err := uuid.Parse(id)
		if err != nil {
			t.Fatal(err)
		}
		name, err := schemadiff.TableName(parsed)
		if err != nil {
			t.Fatal(err)
		}
		for _, table := range desired.Tables {
			if table.Name == name {
				return table
			}
		}
		t.Fatalf("no table %s", name)
		return schemadiff.Table{}
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// insert writes one row in a savepoint of its own, so that a refusal
	// leaves the rest of the transaction usable, and returns the code of the
	// refusal, empty when the row was written.
	insert := func(table schemadiff.Table, given map[string]any) string {
		var names, places []string
		var values []any
		for _, column := range table.Columns {
			value, ok := given[column.Name]
			if !ok {
				if column.Nullable || column.Default != "" {
					continue
				}
				value = valueOfType(t, column.Type)
			}
			names = append(names, pgx.Identifier{column.Name}.Sanitize())
			values = append(values, value)
			places = append(places, "$"+strconv.Itoa(len(values)))
		}
		savepoint, err := tx.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		_, err = savepoint.Exec(ctx, "INSERT INTO "+pgx.Identifier{schemadiff.ApplicationSchema, table.Name}.Sanitize()+
			" ("+strings.Join(names, ", ")+") VALUES ("+strings.Join(places, ", ")+")", values...)
		if err == nil {
			if err := savepoint.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			return ""
		}
		_ = savepoint.Rollback(ctx)
		var refusal *pgconn.PgError
		if !errors.As(err, &refusal) {
			t.Fatalf("insert into %s: %v", table.Name, err)
		}
		return refusal.Code
	}
	const uniqueViolation, notNullViolation = "23505", "23502"

	entries := tableOf(cmpEntries)
	recorderType, recorderRef := uuid.MustNew(), uuid.MustNew()
	line := func(number int) map[string]any {
		return map[string]any{"record_id": uuid.MustNew(), "recorder_type": recorderType, "recorder_ref": recorderRef, "line_no": number}
	}
	if code := insert(entries, line(1)); code != "" {
		t.Fatalf("the first line of an entry was refused: %s", code)
	}
	if code := insert(entries, line(2)); code != "" {
		t.Fatalf("the second line of an entry was refused: %s", code)
	}
	if code := insert(entries, line(1)); code != uniqueViolation {
		t.Fatalf("a line of an entry written twice: %q", code)
	}

	payroll := tableOf(cmpPayroll)
	if code := insert(payroll, map[string]any{"calculation_type": nil}); code != notNullViolation {
		t.Fatalf("a calculation record calculated by nothing: %q", code)
	}
	calculationTypes := tableOf(cmpCalculationTypes)
	if code := insert(calculationTypes, map[string]any{"action_period_is_base": nil}); code != notNullViolation {
		t.Fatalf("a calculation type neither is nor is not based on its action period: %q", code)
	}
}

// valueOfType is a value a column of the type accepts, for the columns a test
// does not care about.
func valueOfType(t *testing.T, sqlType string) any {
	t.Helper()
	switch {
	case sqlType == "uuid":
		return uuid.MustNew()
	case sqlType == "timestamp with time zone":
		return time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	case sqlType == "integer" || sqlType == "bigint" || sqlType == "smallint" || strings.HasPrefix(sqlType, "numeric"):
		return 0
	case sqlType == "boolean":
		return false
	case strings.HasPrefix(sqlType, "character varying"):
		return ""
	case sqlType == "jsonb":
		return []any{}
	}
	t.Fatalf("no value for a column of type %s", sqlType)
	return nil
}
