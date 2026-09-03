package systemdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"
)

func TestDatabaseMigrationAndSettingsIntegration(t *testing.T) {
	databaseURL := os.Getenv("ML_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("ML_TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	database, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := database.Ping(ctx); err != nil {
		t.Fatal(err)
	}

	var migrations int
	if err := database.pool.QueryRow(ctx, "SELECT COUNT(*) FROM ml_system.schema_migrations").Scan(&migrations); err != nil {
		t.Fatal(err)
	}
	if migrations != 4 {
		t.Fatalf("migration count = %d, want 4", migrations)
	}

	key := fmt.Sprintf("test.setting-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = database.Settings.Delete(cleanupCtx, key)
	})
	created, err := database.Settings.Set(ctx, key, json.RawMessage(`{"language":"ru"}`))
	if err != nil {
		t.Fatal(err)
	}
	if created.Revision != 1 || string(created.Value) != `{"language": "ru"}` {
		t.Fatalf("created setting = %+v", created)
	}
	updated, err := database.Settings.Set(ctx, key, json.RawMessage(`{"language":"uk"}`))
	if err != nil {
		t.Fatal(err)
	}
	if updated.Revision != 2 {
		t.Fatalf("updated revision = %d, want 2", updated.Revision)
	}
	read, err := database.Settings.Get(ctx, key)
	if err != nil || read.Revision != updated.Revision || string(read.Value) != string(updated.Value) {
		t.Fatalf("read setting = %+v, error = %v", read, err)
	}
	deleted, err := database.Settings.Delete(ctx, key)
	if err != nil || !deleted {
		t.Fatalf("deleted = %v, error = %v", deleted, err)
	}
	if _, err := database.Settings.Get(ctx, key); !errors.Is(err, ErrSettingNotFound) {
		t.Fatalf("Get() after delete error = %v", err)
	}

	second, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("second idempotent Open() failed: %v", err)
	}
	second.Close()
}

func TestSettingsConcurrentUpdatesAreAtomic(t *testing.T) {
	databaseURL := os.Getenv("ML_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("ML_TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	database, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	key := fmt.Sprintf("test.concurrent-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = database.Settings.Delete(cleanupCtx, key)
	})

	const updates = 16
	var wait sync.WaitGroup
	errorsChannel := make(chan error, updates)
	for index := range updates {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, updateErr := database.Settings.Set(ctx, key, json.RawMessage(fmt.Sprintf(`{"value":%d}`, index)))
			errorsChannel <- updateErr
		}()
	}
	wait.Wait()
	close(errorsChannel)
	for updateErr := range errorsChannel {
		if updateErr != nil {
			t.Fatal(updateErr)
		}
	}
	setting, err := database.Settings.Get(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if setting.Revision != updates {
		t.Fatalf("revision = %d, want %d", setting.Revision, updates)
	}
}

func TestMigrationsRejectChangesAndRollbackFailure(t *testing.T) {
	databaseURL := os.Getenv("ML_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("ML_TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	database, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	connection, err := database.pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Release()

	migrations, err := loadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	changed := append([]migration(nil), migrations...)
	changed[0].checksum = "changed"
	if err := applyMigrations(ctx, connection.Conn(), changed); err == nil {
		t.Fatal("changed applied migration was accepted")
	}

	failing := append(append([]migration(nil), migrations...), migration{
		version: 5, name: "rollback_probe", checksum: "test",
		sql: "CREATE TABLE ml_system.rollback_probe(id BIGINT); SELECT 1 / 0;",
	})
	if err := applyMigrations(ctx, connection.Conn(), failing); err == nil {
		t.Fatal("failing migration succeeded")
	}
	var relation *string
	if err := database.pool.QueryRow(ctx, "SELECT to_regclass('ml_system.rollback_probe')::text").Scan(&relation); err != nil {
		t.Fatal(err)
	}
	if relation != nil {
		t.Fatalf("failed migration left table %q", *relation)
	}
}

func BenchmarkSettingsSet(b *testing.B) {
	databaseURL := os.Getenv("ML_TEST_DATABASE_URL")
	if databaseURL == "" {
		b.Skip("ML_TEST_DATABASE_URL is not set")
	}
	database, err := Open(context.Background(), databaseURL)
	if err != nil {
		b.Fatal(err)
	}
	defer database.Close()
	key := fmt.Sprintf("test.benchmark-%d", time.Now().UnixNano())
	defer func() { _, _ = database.Settings.Delete(context.Background(), key) }()
	value := json.RawMessage(`{"enabled":true}`)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := database.Settings.Set(context.Background(), key, value); err != nil {
			b.Fatal(err)
		}
	}
}
