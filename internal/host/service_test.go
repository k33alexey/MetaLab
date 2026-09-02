package host

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/k33alexey/MetaLab/internal/appconfig"
	"github.com/k33alexey/MetaLab/internal/postgresconn"
)

func TestBuildHandlerAllowsFirstRunWithoutDatabase(t *testing.T) {
	t.Setenv("ML_SYSTEM_DATABASE_URL", "")
	t.Setenv("ML_DATABASE_URL", "")
	handler, closeRuntime, err := buildHandler(context.Background(), appconfig.Default(), fakeSecrets{})
	if err != nil {
		t.Fatal(err)
	}
	defer closeRuntime()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if response.Code != http.StatusOK || response.Body.String() != "{\"database\":\"unconfigured\",\"status\":\"degraded\"}\n" {
		t.Fatalf("status=%d body=%q", response.Code, response.Body.String())
	}
}

func TestBuildHandlerDegradesWhenConfiguredSecretIsUnavailable(t *testing.T) {
	t.Parallel()

	configuration := appconfig.Default()
	configuration.SystemDatabase = &postgresconn.Descriptor{
		Host: "localhost", Port: 5432, Database: "metalab", User: "metalab",
		SSLMode: "disable", SecretKey: postgresconn.DefaultSystemSecretKey,
	}
	handler, closeRuntime, err := buildHandler(context.Background(), configuration, fakeSecrets{failure: errors.New("locked")})
	if err == nil {
		t.Fatal("missing protected secret did not report an error")
	}
	defer closeRuntime()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if response.Code != http.StatusOK || response.Body.String() != "{\"database\":\"unconfigured\",\"status\":\"degraded\"}\n" {
		t.Fatalf("status=%d body=%q", response.Code, response.Body.String())
	}
}

type fakeSecrets struct {
	password string
	failure  error
}

func (secrets fakeSecrets) Get(string) (string, error) { return secrets.password, secrets.failure }
