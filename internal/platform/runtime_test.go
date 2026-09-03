package platform

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/appconfig"
	"github.com/k33alexey/MetaLab/internal/postgresconn"
)

func TestRuntimeReportsUnavailableSecretStore(t *testing.T) {
	t.Parallel()

	runtime := New(context.Background(), appconfig.Default(), nil)
	defer runtime.Close()
	if state := runtime.State(); state.Connected || !strings.Contains(state.Error, "credential store") {
		t.Fatalf("state = %+v", state)
	}
}

func TestApplicationConnectionRequiresRemoteTLSAndTechnicalPassword(t *testing.T) {
	t.Parallel()

	descriptor := postgresconn.Descriptor{
		Host: "db.example.test", Port: 5432, Database: "app", User: "app",
		SSLMode: "disable", SecretKey: "placeholder",
	}
	if err := validateApplicationConnection(descriptor, "secret"); err == nil || !strings.Contains(err.Error(), "TLS") {
		t.Fatalf("remote connection error = %v", err)
	}
	descriptor.Host = "localhost"
	if err := validateApplicationConnection(descriptor, ""); err == nil || !strings.Contains(err.Error(), "password") {
		t.Fatalf("passwordless TCP error = %v", err)
	}
	descriptor.Host = "/tmp"
	if err := validateApplicationConnection(descriptor, ""); err == nil || !strings.Contains(err.Error(), "password") {
		t.Fatalf("passwordless Unix socket error = %v", err)
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

func TestOperationalErrorsRedactKnownSecrets(t *testing.T) {
	t.Parallel()
	message := safeOperationalError(errors.New("connection failed with password super-secret"), "super-secret")
	if strings.Contains(message, "super-secret") || !strings.Contains(message, "[REDACTED]") {
		t.Fatalf("safe error = %q", message)
	}
}
