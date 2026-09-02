package prototype

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestPostgresStoreIntegration(t *testing.T) {
	databaseURL := os.Getenv("ML_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("ML_TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	store, err := OpenPostgresStore(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := store.pool.Exec(ctx, "TRUNCATE ml_prototype_calculations RESTART IDENTITY"); err != nil {
		t.Fatal(err)
	}
	createdAt := time.Date(2026, time.September, 2, 8, 0, 0, 0, time.UTC)
	calculation, err := store.Save(ctx, Calculation{Left: 20, Right: 22, Result: 84, CreatedAt: createdAt})
	if err != nil {
		t.Fatal(err)
	}
	if calculation.ID != 1 {
		t.Fatalf("id = %d, want 1", calculation.ID)
	}
	stats, err := store.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Count != 1 || stats.LastResult != 84 {
		t.Fatalf("stats = %+v", stats)
	}
}

func TestPostgresEndToEndIntegration(t *testing.T) {
	databaseURL := os.Getenv("ML_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("ML_TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	runtime, err := OpenRuntime(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	if _, err := runtime.Store.pool.Exec(ctx, "TRUNCATE ml_prototype_calculations RESTART IDENTITY"); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/prototype/calculate", bytes.NewBufferString(`{"left":20,"right":22}`))
	response := httptest.NewRecorder()
	runtime.Service.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var calculation Calculation
	if err := json.Unmarshal(response.Body.Bytes(), &calculation); err != nil {
		t.Fatal(err)
	}
	if calculation.ID != 1 || calculation.Result != 84 {
		t.Fatalf("calculation = %+v", calculation)
	}
	stats, err := runtime.Store.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Count != 1 || stats.LastResult != 84 {
		t.Fatalf("stats = %+v", stats)
	}
}

func BenchmarkPostgresEndToEnd(b *testing.B) {
	databaseURL := os.Getenv("ML_TEST_DATABASE_URL")
	if databaseURL == "" {
		b.Skip("ML_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	runtime, err := OpenRuntime(ctx, databaseURL)
	if err != nil {
		b.Fatal(err)
	}
	defer runtime.Close()
	if _, err := runtime.Store.pool.Exec(ctx, "TRUNCATE ml_prototype_calculations RESTART IDENTITY"); err != nil {
		b.Fatal(err)
	}
	payload := []byte(`{"left":20,"right":22}`)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		request := httptest.NewRequest(http.MethodPost, "/api/prototype/calculate", bytes.NewReader(payload))
		response := httptest.NewRecorder()
		runtime.Service.Handler().ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			b.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
		}
	}
}
