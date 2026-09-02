package postgresadmin

import (
	"context"
	"net/url"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k33alexey/MetaLab/internal/postgresconn"
)

func TestProvisionTechnicalDatabaseIntegration(t *testing.T) {
	databaseURL := os.Getenv("ML_TEST_ADMIN_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("ML_TEST_ADMIN_DATABASE_URL is not set")
	}
	administrator, password := descriptorFromURL(t, databaseURL)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	check, err := Validate(ctx, administrator, password)
	if err != nil {
		t.Fatal(err)
	}
	if !check.CanProvision() {
		t.Fatalf("administrator check = %+v", check)
	}
	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	databaseName, roleName := "ml_it_db_"+suffix, "ml_it_role_"+suffix
	provisioned, err := Provision(ctx, administrator, password, databaseName, roleName)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		cleanupPool, cleanupErr := open(cleanupCtx, administrator, password)
		if cleanupErr != nil {
			t.Errorf("open cleanup connection: %v", cleanupErr)
			return
		}
		defer cleanupPool.Close()
		if cleanupErr := cleanupProvisioned(cleanupCtx, cleanupPool, pgx.Identifier{databaseName}.Sanitize(), pgx.Identifier{roleName}.Sanitize()); cleanupErr != nil {
			t.Errorf("cleanup provisioned PostgreSQL: %v", cleanupErr)
		}
	})
	technicalPool, err := open(ctx, provisioned.Connection, provisioned.Password)
	if err != nil {
		t.Fatal(err)
	}
	defer technicalPool.Close()
	var database, role string
	var superuser, createDB, createRole, replication, bypassRLS bool
	err = technicalPool.QueryRow(ctx, `
SELECT current_database(), current_user, rolsuper, rolcreatedb, rolcreaterole, rolreplication, rolbypassrls
FROM pg_catalog.pg_roles WHERE rolname = current_user`).Scan(
		&database, &role, &superuser, &createDB, &createRole, &replication, &bypassRLS,
	)
	if err != nil {
		t.Fatal(err)
	}
	if database != databaseName || role != roleName || superuser || createDB || createRole || replication || bypassRLS {
		t.Fatalf("database=%q role=%q privileges=%v/%v/%v/%v/%v", database, role, superuser, createDB, createRole, replication, bypassRLS)
	}
}

func descriptorFromURL(t *testing.T, databaseURL string) (postgresconn.Descriptor, string) {
	t.Helper()
	configuration, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	sslMode := parsed.Query().Get("sslmode")
	if sslMode == "" {
		sslMode = "require"
	}
	return postgresconn.Descriptor{
		Host: configuration.ConnConfig.Host, Port: configuration.ConnConfig.Port,
		Database: configuration.ConnConfig.Database, User: configuration.ConnConfig.User,
		SSLMode: sslMode, SecretKey: "test.admin.password",
	}, configuration.ConnConfig.Password
}
