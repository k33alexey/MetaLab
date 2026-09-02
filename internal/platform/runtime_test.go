package platform

import (
	"context"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/appconfig"
)

func TestRuntimeReportsUnavailableSecretStore(t *testing.T) {
	t.Parallel()

	runtime := New(context.Background(), appconfig.Default(), nil)
	defer runtime.Close()
	if state := runtime.State(); state.Connected || !strings.Contains(state.Error, "credential store") {
		t.Fatalf("state = %+v", state)
	}
}

func TestProvisionRequestRequiresTLSForRemoteServer(t *testing.T) {
	t.Parallel()

	request := ProvisionRequest{
		Host: "db.example.test", Port: 5432, AdministratorDatabase: "postgres",
		AdministratorUser: "postgres", AdministratorPassword: "secret", SSLMode: "disable",
	}
	if _, err := request.administratorDescriptor(); err == nil || !strings.Contains(err.Error(), "TLS") {
		t.Fatalf("administratorDescriptor() error = %v", err)
	}
}
