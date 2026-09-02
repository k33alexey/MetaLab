package systemdb

import (
	"context"
	"strings"
	"testing"
)

func TestOpenRequiresExplicitPostgreSQLConfiguration(t *testing.T) {
	t.Parallel()

	if _, err := Open(context.Background(), " "); err == nil || !strings.Contains(err.Error(), "configuration is required") {
		t.Fatalf("Open() error = %v", err)
	}
}
