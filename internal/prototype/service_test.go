package prototype

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestServiceRunsEndToEndScenario(t *testing.T) {
	t.Parallel()

	store := &memoryStore{}
	service, err := NewService(store)
	if err != nil {
		t.Fatal(err)
	}
	createdAt := time.Date(2026, time.September, 2, 8, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return createdAt }

	response := request(t, service.Handler(), http.MethodPost, "/api/prototype/calculate", `{"left":20,"right":22}`)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var calculation Calculation
	if err := json.Unmarshal(response.Body.Bytes(), &calculation); err != nil {
		t.Fatal(err)
	}
	if calculation.ID != 1 || calculation.Result != 84 || !calculation.CreatedAt.Equal(createdAt) {
		t.Fatalf("calculation = %+v", calculation)
	}

	statsResponse := request(t, service.Handler(), http.MethodGet, "/api/prototype/stats", "")
	var stats Stats
	if err := json.Unmarshal(statsResponse.Body.Bytes(), &stats); err != nil {
		t.Fatal(err)
	}
	if stats.Count != 1 || stats.LastResult != 84 {
		t.Fatalf("stats = %+v", stats)
	}
}

func TestServiceExposesUIAndHealth(t *testing.T) {
	t.Parallel()

	service, err := NewService(&memoryStore{})
	if err != nil {
		t.Fatal(err)
	}
	page := request(t, service.Handler(), http.MethodGet, "/", "")
	if page.Code != http.StatusOK || !bytes.Contains(page.Body.Bytes(), []byte("ML Service")) {
		t.Fatalf("page = %d, %q", page.Code, page.Body.String())
	}
	health := request(t, service.Handler(), http.MethodGet, "/api/health", "")
	if health.Code != http.StatusOK || !bytes.Contains(health.Body.Bytes(), []byte(`"database":"postgresql"`)) {
		t.Fatalf("health = %d, %q", health.Code, health.Body.String())
	}
}

func TestServiceRejectsInvalidRequests(t *testing.T) {
	t.Parallel()

	service, err := NewService(&memoryStore{})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		body string
	}{
		{name: "malformed", body: `{`},
		{name: "unknown field", body: `{"left":1,"right":2,"other":3}`},
		{name: "multiple values", body: `{"left":1,"right":2} {}`},
		{name: "number outside BSL precision", body: `{"left":1e100,"right":2}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := request(t, service.Handler(), http.MethodPost, "/api/prototype/calculate", test.body)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestServiceReportsStoreFailures(t *testing.T) {
	t.Parallel()

	store := &memoryStore{failure: errors.New("database unavailable")}
	service, err := NewService(store)
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"/api/health", "/api/prototype/stats"} {
		response := request(t, service.Handler(), http.MethodGet, target, "")
		if response.Code < 500 {
			t.Fatalf("%s status = %d", target, response.Code)
		}
	}
	response := request(t, service.Handler(), http.MethodPost, "/api/prototype/calculate", `{"left":1,"right":2}`)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("calculate status = %d", response.Code)
	}
}

func TestNewServiceRejectsNilStore(t *testing.T) {
	t.Parallel()

	if _, err := NewService(nil); err == nil {
		t.Fatal("NewService(nil) succeeded")
	}
}

func request(t *testing.T, handler http.Handler, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

type memoryStore struct {
	mu           sync.Mutex
	calculations []Calculation
	failure      error
}

func (store *memoryStore) Migrate(context.Context) error { return store.failure }

func (store *memoryStore) Ping(context.Context) error { return store.failure }

func (store *memoryStore) Save(_ context.Context, calculation Calculation) (Calculation, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.failure != nil {
		return Calculation{}, store.failure
	}
	calculation.ID = int64(len(store.calculations) + 1)
	store.calculations = append(store.calculations, calculation)
	return calculation, nil
}

func (store *memoryStore) Stats(context.Context) (Stats, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.failure != nil {
		return Stats{}, store.failure
	}
	stats := Stats{Count: int64(len(store.calculations))}
	if len(store.calculations) != 0 {
		stats.LastResult = store.calculations[len(store.calculations)-1].Result
	}
	return stats, nil
}
