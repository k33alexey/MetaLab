package schemadiff

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// The defect this sub-point exists for: reducing the scale of a numeric column
// is performed silently by PostgreSQL, and one "destructive" flag let an
// operator who meant to drop an attribute authorize it without knowing.
func TestValueRewriteIsConfirmedSeparatelyAndMeasuredIntegration(t *testing.T) {
	databaseURL := os.Getenv("ML_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("ML_TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	projectID := uuid.MustNew()
	schemaName := fmt.Sprintf("ml_impact_%x", time.Now().UnixNano())
	quotedSchema := pgx.Identifier{schemaName}.Sanitize()
	t.Cleanup(func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupContext, "DROP SCHEMA IF EXISTS "+quotedSchema+" CASCADE")
		_, _ = pool.Exec(cleanupContext, "DELETE FROM ml_core.migration_journal WHERE project_id = $1", projectID.String())
		pool.Close()
	})
	before := Schema{Name: schemaName, Exists: true, Tables: []Table{{
		Name: "t_goods",
		Columns: []Column{
			{Name: "id", Type: "uuid", Nullable: false},
			{Name: "price", Type: "numeric(10,2)", Nullable: false, Default: "0"},
			{Name: "note", Type: "character varying(50)", Nullable: true},
		},
		Constraints: []Constraint{{Name: "k_goods_primary", Type: "primary_key", Definition: "PRIMARY KEY (id)"}},
	}}}
	apply := func(desired Schema, consent MigrationConsent) (MigrationRecord, error) {
		prepared, err := Prepare(ctx, pool, desired)
		if err != nil {
			t.Fatal(err)
		}
		return Execute(ctx, pool, MigrationRequest{
			ProjectID: projectID, PackageSHA256: repeatedHex('a', 64), GitCommit: repeatedHex('b', 40),
			Desired: desired, ExpectedPlanSHA256: prepared.SHA256, ExpectedSchemaSHA256: prepared.ActualSHA256,
			Confirmed: true, Consent: consent,
		})
	}
	if _, err := apply(before, MigrationConsent{}); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	for _, price := range []string{"10.56", "11.00", "12.49"} {
		if _, err := pool.Exec(ctx, "INSERT INTO "+quotedSchema+".t_goods (id, price, note) VALUES ($1, $2, 'x')", uuid.MustNew().String(), price); err != nil {
			t.Fatal(err)
		}
	}

	// Rounding prices AND dropping a column in one plan: agreeing to the drop
	// must not be agreement to the rounding.
	rounded := cloneSchema(before)
	rounded.Tables[0].Columns = []Column{
		{Name: "id", Type: "uuid", Nullable: false},
		{Name: "price", Type: "numeric(10,0)", Nullable: false, Default: "0"},
	}
	_, err = apply(rounded, MigrationConsent{ObjectLoss: true})
	var needed *ConfirmationNeeded
	if !errors.As(err, &needed) || !errors.Is(err, ErrValueRewriteDenied) {
		t.Fatalf("allowing object loss also allowed rewriting values: %v", err)
	}
	measured := map[string]int64{}
	for _, impact := range needed.Impacts {
		measured[impact.Column] = impact.Rows
	}
	// 10.56 and 12.49 round, 11.00 does not: the count is the answer to "how
	// much data does this actually change", which no flag could give.
	if measured["price"] != 2 {
		t.Fatalf("measured rows = %+v, want 2 rounded prices", needed.Impacts)
	}
	var remaining int64
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+quotedSchema+".t_goods WHERE price = 10.56").Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 1 {
		t.Fatalf("a refused migration changed data: %d rows still hold 10.56", remaining)
	}
	if _, err := apply(rounded, MigrationConsent{ObjectLoss: true, ValueRewrite: true}); err != nil {
		t.Fatalf("confirmed rewrite: %v", err)
	}
	var rows int64
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+quotedSchema+".t_goods WHERE price = 11").Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 2 {
		t.Fatalf("rounded prices = %d, want 10.56 and 11.00 to meet at 11", rows)
	}
}

// A change that can only fail needs no permission - but the operator still
// deserves the number of rows that will reject it, before starting.
func TestMayFailChangesAreMeasuredWithoutRequiringConsentIntegration(t *testing.T) {
	databaseURL := os.Getenv("ML_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("ML_TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	projectID := uuid.MustNew()
	schemaName := fmt.Sprintf("ml_mayfail_%x", time.Now().UnixNano())
	quotedSchema := pgx.Identifier{schemaName}.Sanitize()
	t.Cleanup(func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupContext, "DROP SCHEMA IF EXISTS "+quotedSchema+" CASCADE")
		_, _ = pool.Exec(cleanupContext, "DELETE FROM ml_core.migration_journal WHERE project_id = $1", projectID.String())
		pool.Close()
	})
	before := Schema{Name: schemaName, Exists: true, Tables: []Table{{
		Name:        "t_partners",
		Columns:     []Column{{Name: "id", Type: "uuid", Nullable: false}, {Name: "title", Type: "character varying(50)", Nullable: true}},
		Constraints: []Constraint{{Name: "k_partners_primary", Type: "primary_key", Definition: "PRIMARY KEY (id)"}},
	}}}
	prepared, err := Prepare(ctx, pool, before)
	if err != nil {
		t.Fatal(err)
	}
	request := MigrationRequest{
		ProjectID: projectID, PackageSHA256: repeatedHex('a', 64), GitCommit: repeatedHex('b', 40),
		Desired: before, ExpectedPlanSHA256: prepared.SHA256, ExpectedSchemaSHA256: prepared.ActualSHA256, Confirmed: true,
	}
	if _, err := Execute(ctx, pool, request); err != nil {
		t.Fatal(err)
	}
	for _, title := range []string{"короткое", "это значение заведомо длиннее десяти символов"} {
		if _, err := pool.Exec(ctx, "INSERT INTO "+quotedSchema+".t_partners (id, title) VALUES ($1, $2)", uuid.MustNew().String(), title); err != nil {
			t.Fatal(err)
		}
	}
	narrowed := cloneSchema(before)
	narrowed.Tables[0].Columns[1].Type = "character varying(10)"
	prepared, err = Prepare(ctx, pool, narrowed)
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Plan.MayFailCount != 1 || prepared.Plan.ValueRewriteCount != 0 || prepared.Plan.ObjectLossCount != 0 {
		t.Fatalf("plan counts: %+v", prepared.Plan)
	}
	if len(prepared.Impacts) != 1 || prepared.Impacts[0].Rows != 1 || prepared.Impacts[0].Impact != ImpactMayFail {
		t.Fatalf("impacts = %+v, want one row that does not fit", prepared.Impacts)
	}
	// No consent needed: the migration is attempted and PostgreSQL rejects it,
	// leaving the data exactly as it was.
	request.Desired, request.ExpectedPlanSHA256, request.ExpectedSchemaSHA256 = narrowed, prepared.SHA256, prepared.ActualSHA256
	if _, err := Execute(ctx, pool, request); err == nil {
		t.Fatal("narrowing a column below its data succeeded")
	}
	var rows int64
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+quotedSchema+".t_partners").Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 2 {
		t.Fatalf("rows after a rejected migration = %d, want 2", rows)
	}
}

// Changing an attribute's type between families used to fail with a raw
// PostgreSQL error ("cannot be cast automatically"), which is the same as
// telling a developer that a routine 1C operation is impossible.
func TestCrossFamilyTypeChangesConvertOrExplainThemselvesIntegration(t *testing.T) {
	databaseURL := os.Getenv("ML_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("ML_TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	projectID := uuid.MustNew()
	schemaName := fmt.Sprintf("ml_convert_%x", time.Now().UnixNano())
	quotedSchema := pgx.Identifier{schemaName}.Sanitize()
	t.Cleanup(func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupContext, "DROP SCHEMA IF EXISTS "+quotedSchema+" CASCADE")
		_, _ = pool.Exec(cleanupContext, "DELETE FROM ml_core.migration_journal WHERE project_id = $1", projectID.String())
		pool.Close()
	})
	base := Schema{Name: schemaName, Exists: true, Tables: []Table{{
		Name: "t_convert",
		Columns: []Column{
			{Name: "id", Type: "uuid", Nullable: false},
			{Name: "amount", Type: "character varying(50)", Nullable: true},
			{Name: "moment", Type: "timestamp with time zone", Nullable: true},
			{Name: "owner", Type: "uuid", Nullable: true},
		},
		Constraints: []Constraint{{Name: "k_convert_primary", Type: "primary_key", Definition: "PRIMARY KEY (id)"}},
	}}}
	apply := func(desired Schema, consent MigrationConsent) error {
		prepared, err := Prepare(ctx, pool, desired)
		if err != nil {
			return err
		}
		_, err = Execute(ctx, pool, MigrationRequest{
			ProjectID: projectID, PackageSHA256: repeatedHex('a', 64), GitCommit: repeatedHex('b', 40),
			Desired: desired, ExpectedPlanSHA256: prepared.SHA256, ExpectedSchemaSHA256: prepared.ActualSHA256,
			Confirmed: true, Consent: consent,
		})
		return err
	}
	if err := apply(base, MigrationConsent{}); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO "+quotedSchema+".t_convert (id, amount, moment, owner) VALUES ($1, '42.50', '2026-09-16T12:00:00Z', $2)",
		uuid.MustNew().String(), uuid.MustNew().String()); err != nil {
		t.Fatal(err)
	}

	// Строка → число: раньше падало сырой ошибкой PostgreSQL.
	toNumber := cloneSchema(base)
	toNumber.Tables[0].Columns[1].Type = "numeric(10,2)"
	if err := apply(toNumber, MigrationConsent{ValueRewrite: true}); err != nil {
		t.Fatalf("character varying -> numeric: %v", err)
	}
	var amount string
	if err := pool.QueryRow(ctx, "SELECT amount::text FROM "+quotedSchema+".t_convert").Scan(&amount); err != nil {
		t.Fatal(err)
	}
	if amount != "42.50" {
		t.Fatalf("converted amount = %q", amount)
	}

	// Одиночный тип → составной: добавление второго типа реквизиту.
	toComposite := cloneSchema(toNumber)
	toComposite.Tables[0].Columns[1].Type = "jsonb"
	toComposite.Tables[0].Columns[2].Type = "jsonb"
	if err := apply(toComposite, MigrationConsent{ValueRewrite: true}); err != nil {
		t.Fatalf("scalar -> jsonb: %v", err)
	}
	var composite, moment string
	if err := pool.QueryRow(ctx, "SELECT amount::text, moment::text FROM "+quotedSchema+".t_convert").Scan(&composite, &moment); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(composite, `"kind": "number"`) || !strings.Contains(composite, `"data": "42.50"`) {
		t.Fatalf("composite number = %s", composite)
	}
	// Дата хранится в RFC 3339, а не в собственном формате PostgreSQL.
	if !strings.Contains(moment, `"data": "2026-09-16T12:00:00`) || !strings.Contains(moment, `Z"`) {
		t.Fatalf("composite date = %s", moment)
	}

	// Составной → одиночный: снятие второго типа.
	backToNumber := cloneSchema(toComposite)
	backToNumber.Tables[0].Columns[1].Type = "numeric(10,2)"
	if err := apply(backToNumber, MigrationConsent{ValueRewrite: true}); err != nil {
		t.Fatalf("jsonb -> numeric: %v", err)
	}
	if err := pool.QueryRow(ctx, "SELECT amount::text FROM "+quotedSchema+".t_convert").Scan(&amount); err != nil {
		t.Fatal(err)
	}
	if amount != "42.50" {
		t.Fatalf("amount after returning from composite = %q", amount)
	}

	// uuid → составной остаётся отказом, но отказ объясняет причину и приходит
	// ДО начала миграции.
	ambiguous := cloneSchema(backToNumber)
	ambiguous.Tables[0].Columns[3].Type = "jsonb"
	err = apply(ambiguous, MigrationConsent{ValueRewrite: true})
	if err == nil || !strings.Contains(err.Error(), "cannot tell which") {
		t.Fatalf("uuid -> jsonb error = %v", err)
	}
	var rows int64
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+quotedSchema+".t_convert WHERE owner IS NOT NULL").Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Fatalf("a refused conversion touched the data: %d rows", rows)
	}
}
