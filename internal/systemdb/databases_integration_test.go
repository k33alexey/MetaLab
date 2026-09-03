package systemdb

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/k33alexey/MetaLab/internal/postgresconn"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestDatabaseRegistryLifecycleIntegration(t *testing.T) {
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
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	registration := DatabaseRegistration{
		ID: uuid.MustNew(), Name: "Test " + suffix, PhysicalID: uuid.MustNew(),
		Connection: postgresconn.Descriptor{
			Host: "localhost", Port: 5432, Database: "test_" + suffix, User: "test",
			SSLMode: "disable", SecretKey: "database." + suffix + ".password",
		},
	}
	created, err := database.Databases.Register(ctx, registration)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		current, getErr := database.Databases.Get(cleanupCtx, registration.ID)
		if getErr == nil && current.State != DatabaseStopped {
			_, _ = database.pool.Exec(cleanupCtx, "UPDATE ml_system.databases SET state = 'stopped' WHERE id = $1", registration.ID.String())
		}
		_, _ = database.Databases.Unregister(cleanupCtx, registration.ID)
	})
	if created.State != DatabaseStopped || created.StateRevision != 1 || created.PhysicalID != registration.PhysicalID {
		t.Fatalf("created = %+v", created)
	}
	if _, err := database.Databases.Register(ctx, DatabaseRegistration{
		ID: uuid.MustNew(), Name: registration.Name, PhysicalID: uuid.MustNew(), Connection: postgresconn.Descriptor{
			Host: "localhost", Port: 5432, Database: "other", User: "test", SSLMode: "disable", SecretKey: "database." + suffix + ".name",
		},
	}); !errors.Is(err, ErrDatabaseNameExists) {
		t.Fatalf("duplicate name error = %v", err)
	}
	if _, err := database.Databases.Register(ctx, DatabaseRegistration{
		ID: uuid.MustNew(), Name: "Other " + suffix, PhysicalID: registration.PhysicalID, Connection: postgresconn.Descriptor{
			Host: "alias", Port: 5433, Database: "alias", User: "test", SSLMode: "require", SecretKey: "database." + suffix + ".physical",
		},
	}); !errors.Is(err, ErrPhysicalDatabaseExists) {
		t.Fatalf("duplicate physical database error = %v", err)
	}
	starting, err := database.Databases.Transition(ctx, created.ID, DatabaseStopped, created.StateRevision, DatabaseStarting, "")
	if err != nil || starting.StateRevision != 2 {
		t.Fatalf("starting=%+v error=%v", starting, err)
	}
	if _, err := database.Databases.Unregister(ctx, created.ID); !errors.Is(err, ErrDatabaseCannotUnregister) {
		t.Fatalf("unregister starting database error = %v", err)
	}
	if _, err := database.Databases.Transition(ctx, created.ID, DatabaseStopped, starting.StateRevision, DatabaseRunning, ""); err == nil {
		t.Fatal("invalid transition succeeded")
	}
	if _, err := database.Databases.Transition(ctx, created.ID, DatabaseStarting, created.StateRevision, DatabaseStopped, ""); !errors.Is(err, ErrDatabaseStateConflict) {
		t.Fatalf("stale revision error = %v", err)
	}
	if _, err := database.Databases.Transition(ctx, created.ID, DatabaseStarting, starting.StateRevision, DatabaseStopped, ""); err != nil {
		t.Fatal(err)
	}
	items, err := database.Databases.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range items {
		found = found || item.ID == created.ID
	}
	if !found {
		t.Fatalf("registered database missing from list: %+v", items)
	}
	removed, err := database.Databases.Unregister(ctx, created.ID)
	if err != nil || removed.ID != created.ID {
		t.Fatalf("removed=%+v error=%v", removed, err)
	}
	if _, err := database.Databases.Get(ctx, created.ID); !errors.Is(err, ErrDatabaseNotFound) {
		t.Fatalf("Get() after unregister error = %v", err)
	}
}

func TestConcurrentPhysicalDatabaseRegistrationHasSingleWinner(t *testing.T) {
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
	physicalID := uuid.MustNew()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	const attempts = 12
	type result struct {
		item RegisteredDatabase
		err  error
	}
	results := make(chan result, attempts)
	var wait sync.WaitGroup
	for index := range attempts {
		wait.Add(1)
		go func() {
			defer wait.Done()
			item, registerErr := database.Databases.Register(ctx, DatabaseRegistration{
				ID: uuid.MustNew(), Name: fmt.Sprintf("Concurrent %s %d", suffix, index), PhysicalID: physicalID,
				Connection: postgresconn.Descriptor{
					Host: "localhost", Port: 5432, Database: "concurrent", User: "test", SSLMode: "disable",
					SecretKey: fmt.Sprintf("database.%s.%d.password", suffix, index),
				},
			})
			results <- result{item: item, err: registerErr}
		}()
	}
	wait.Wait()
	close(results)
	var winner RegisteredDatabase
	wins := 0
	for attempt := range results {
		switch {
		case attempt.err == nil:
			winner, wins = attempt.item, wins+1
		case errors.Is(attempt.err, ErrPhysicalDatabaseExists):
		default:
			t.Fatalf("unexpected registration error = %v", attempt.err)
		}
	}
	if wins != 1 {
		t.Fatalf("successful registrations = %d, want 1", wins)
	}
	if _, err := database.Databases.Unregister(ctx, winner.ID); err != nil {
		t.Fatal(err)
	}
}

func TestPrimaryDatabaseCannotBeRemovedBeforeItsDebugCopiesIntegration(t *testing.T) {
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
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	primaryID := uuid.MustNew()
	primary, err := database.Databases.Register(ctx, DatabaseRegistration{
		ID: primaryID, Name: "Primary " + suffix, PhysicalID: uuid.MustNew(), Mode: DatabasePrimary,
		Connection: postgresconn.Descriptor{
			Host: "localhost", Port: 5432, Database: "primary_" + suffix, User: "test",
			SSLMode: "disable", SecretKey: "database." + suffix + ".primary",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	debug, err := database.Databases.Register(ctx, DatabaseRegistration{
		ID: uuid.MustNew(), Name: "Debug " + suffix, PhysicalID: uuid.MustNew(), Mode: DatabaseDebug,
		SourceDatabaseID: &primaryID,
		Connection: postgresconn.Descriptor{
			Host: "localhost", Port: 5432, Database: "debug_" + suffix, User: "test",
			SSLMode: "disable", SecretKey: "database." + suffix + ".debug",
		},
	})
	if err != nil {
		_, _ = database.Databases.Unregister(ctx, primary.ID)
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = database.Databases.Unregister(cleanupCtx, debug.ID)
		_, _ = database.Databases.Unregister(cleanupCtx, primary.ID)
	})
	if debug.SourceDatabaseID == nil || *debug.SourceDatabaseID != primary.ID {
		t.Fatalf("debug source = %v, want %s", debug.SourceDatabaseID, primary.ID)
	}
	if _, err := database.Databases.Unregister(ctx, primary.ID); !errors.Is(err, ErrDatabaseHasDebugCopies) {
		t.Fatalf("unregister primary error = %v", err)
	}
	if _, err := database.Databases.Unregister(ctx, debug.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Databases.Unregister(ctx, primary.ID); err != nil {
		t.Fatal(err)
	}
}
