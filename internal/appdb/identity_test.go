package appdb

import (
	"context"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/postgresconn"
)

func TestEnsureIdentityValidatesConnectionBeforeOpening(t *testing.T) {
	t.Parallel()

	descriptor := postgresconn.Descriptor{Host: "localhost", Port: 5432, Database: "app", User: "app", SSLMode: "disable"}
	if _, err := EnsureIdentity(context.Background(), descriptor, "secret"); err == nil || !strings.Contains(err.Error(), "secret key") {
		t.Fatalf("EnsureIdentity() error = %v", err)
	}
}
