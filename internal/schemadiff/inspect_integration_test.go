package schemadiff

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestInspectReadsTablesColumnsIndexesAndConstraintsIntegration(t *testing.T) {
	databaseURL := os.Getenv("ML_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("ML_TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	schemaName := fmt.Sprintf("ml_diff_%x", time.Now().UnixNano())
	quotedSchema := pgx.Identifier{schemaName}.Sanitize()
	t.Cleanup(func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupContext, "DROP SCHEMA "+quotedSchema+" CASCADE")
		pool.Close()
	})
	if _, err := pool.Exec(ctx, `CREATE SCHEMA `+quotedSchema+`;
CREATE TABLE `+quotedSchema+`.t_customer (
    id uuid NOT NULL,
    name character varying(100) NOT NULL DEFAULT ''::character varying,
    amount numeric(15,2),
    CONSTRAINT pk_customer PRIMARY KEY (id),
    CONSTRAINT ck_amount CHECK (amount >= 0)
);
CREATE UNIQUE INDEX ix_customer_name ON `+quotedSchema+`.t_customer USING btree (name) INCLUDE (amount) WHERE name <> ''::text;`); err != nil {
		t.Fatal(err)
	}
	actual, err := Inspect(ctx, pool, schemaName)
	if err != nil {
		t.Fatal(err)
	}
	if !actual.Exists || len(actual.Tables) != 1 || len(actual.Tables[0].Columns) != 3 || len(actual.Tables[0].Indexes) != 1 || len(actual.Tables[0].Constraints) != 2 {
		t.Fatalf("inspected schema = %+v", actual)
	}
	index := actual.Tables[0].Indexes[0]
	if index.Name != "ix_customer_name" || !index.Unique || index.Method != "btree" || len(index.Keys) != 1 || index.Keys[0] != "name" || len(index.Include) != 1 || index.Include[0] != "amount" || !strings.Contains(index.Predicate, "name") {
		t.Fatalf("inspected index = %+v", index)
	}
	plan, err := Compare(cloneSchema(actual), actual)
	if err != nil || len(plan.Changes) != 0 {
		t.Fatalf("equal schema plan=%+v error=%v", plan, err)
	}
	missing, err := Inspect(ctx, pool, schemaName+"_missing")
	if err != nil || missing.Exists || len(missing.Tables) != 0 {
		t.Fatalf("missing schema=%+v error=%v", missing, err)
	}
}
