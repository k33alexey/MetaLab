package host

import (
	"context"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/appconfig"
)

func TestRunServiceRequiresDatabaseURL(t *testing.T) {
	t.Setenv("ML_DATABASE_URL", "")
	err := RunService(context.Background(), appconfig.Default())
	if err == nil || !strings.Contains(err.Error(), "ML_DATABASE_URL") {
		t.Fatalf("RunService() error = %v", err)
	}
}
