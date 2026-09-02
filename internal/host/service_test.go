package host

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBuildHandlerAllowsFirstRunWithoutDatabase(t *testing.T) {
	t.Parallel()

	handler, closeRuntime, err := buildHandler(context.Background(), "")
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
