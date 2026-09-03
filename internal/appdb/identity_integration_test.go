package appdb

import (
	"context"
	"errors"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k33alexey/MetaLab/internal/postgresconn"
)

func TestEnsureIdentityRejectsMLSystemIntegration(t *testing.T) {
	databaseURL := os.Getenv("ML_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("ML_TEST_DATABASE_URL is not set")
	}
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
	descriptor := postgresconn.Descriptor{
		Host: configuration.ConnConfig.Host, Port: configuration.ConnConfig.Port,
		Database: configuration.ConnConfig.Database, User: configuration.ConnConfig.User,
		SSLMode: sslMode, SecretKey: "test.password",
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := EnsureIdentity(ctx, descriptor, configuration.ConnConfig.Password); !errors.Is(err, ErrSystemDatabase) {
		t.Fatalf("EnsureIdentity() error = %v", err)
	}
}
