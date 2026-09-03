package systemdb

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/k33alexey/MetaLab/internal/auth"
	"github.com/k33alexey/MetaLab/internal/postgresconn"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestStudioSessionIsExclusiveRenewableAndRecoverableIntegration(t *testing.T) {
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
	suffix := fmt.Sprint(time.Now().UnixNano())
	databaseID := uuid.MustNew()
	_, err = database.Databases.Register(ctx, DatabaseRegistration{
		ID: databaseID, Name: "Studio " + suffix, PhysicalID: uuid.MustNew(), Mode: DatabaseDebug,
		Connection: postgresconn.Descriptor{Host: "localhost", Port: 5432, Database: "studio", User: "ml", SSLMode: "disable", SecretKey: "studio." + suffix},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = database.pool.Exec(cleanupContext, "UPDATE ml_system.databases SET state = 'stopped' WHERE id = $1", databaseID.String())
		_, _ = database.Databases.Unregister(cleanupContext, databaseID)
	})
	projectID, firstID := uuid.MustNew(), uuid.MustNew()
	firstDigest := auth.SessionTokenDigest("first-studio-token")
	first, err := database.StudioSessions.Acquire(ctx, databaseID, projectID, firstID, firstDigest[:], "alexey", "workstation", 100)
	if err != nil || first.ID != firstID {
		t.Fatalf("first session=%+v error=%v", first, err)
	}
	secondDigest := auth.SessionTokenDigest("second-studio-token")
	_, err = database.StudioSessions.Acquire(ctx, databaseID, projectID, uuid.MustNew(), secondDigest[:], "second", "other-host", 200)
	if !errors.Is(err, ErrStudioSessionLocked) {
		t.Fatalf("second acquire error = %v", err)
	}
	var locked *StudioLockError
	if !errors.As(err, &locked) || locked.Session.OwnerName != "alexey" {
		t.Fatalf("lock details = %#v", locked)
	}
	if _, err := database.StudioSessions.Heartbeat(ctx, firstID, secondDigest[:], 101); !errors.Is(err, ErrStudioSessionNotFound) {
		t.Fatalf("wrong-token heartbeat error = %v", err)
	}
	renewed, err := database.StudioSessions.Heartbeat(ctx, firstID, firstDigest[:], 101)
	if err != nil || renewed.ProcessID != 101 || !renewed.ExpiresAt.After(first.ExpiresAt) {
		t.Fatalf("renewed session=%+v error=%v", renewed, err)
	}
	items, err := database.StudioSessions.ListActive(ctx)
	if err != nil || len(items) != 1 || items[0].ID != firstID {
		t.Fatalf("active sessions=%+v error=%v", items, err)
	}
	if err := database.StudioSessions.Terminate(ctx, databaseID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.StudioSessions.Heartbeat(ctx, firstID, firstDigest[:], 101); !errors.Is(err, ErrStudioSessionNotFound) {
		t.Fatalf("terminated heartbeat error = %v", err)
	}
	secondID := uuid.MustNew()
	if _, err := database.StudioSessions.Acquire(ctx, databaseID, projectID, secondID, secondDigest[:], "second", "other-host", 200); err != nil {
		t.Fatal(err)
	}
	if _, err := database.pool.Exec(ctx, "UPDATE ml_system.studio_sessions SET expires_at = clock_timestamp() - interval '1 second' WHERE database_id = $1", databaseID.String()); err != nil {
		t.Fatal(err)
	}
	thirdID := uuid.MustNew()
	thirdDigest := auth.SessionTokenDigest("third-studio-token")
	third, err := database.StudioSessions.Acquire(ctx, databaseID, projectID, thirdID, thirdDigest[:], "third", "third-host", 300)
	if err != nil || third.ID != thirdID {
		t.Fatalf("expired replacement=%+v error=%v", third, err)
	}
	if _, err := database.Databases.Unregister(ctx, databaseID); !errors.Is(err, ErrDatabaseHasStudioSession) {
		t.Fatalf("unregister with active Studio error = %v", err)
	}
	if err := database.StudioSessions.Terminate(ctx, databaseID); err != nil {
		t.Fatal(err)
	}

	type acquisition struct{ err error }
	start := make(chan struct{})
	results := make(chan acquisition, 2)
	for index := range 2 {
		go func(index int) {
			<-start
			tokenDigest := auth.SessionTokenDigest(fmt.Sprintf("concurrent-token-%d", index))
			_, acquireErr := database.StudioSessions.Acquire(
				ctx, databaseID, projectID, uuid.MustNew(), tokenDigest[:],
				fmt.Sprintf("concurrent-%d", index), "concurrent-host", int64(400+index),
			)
			results <- acquisition{err: acquireErr}
		}(index)
	}
	close(start)
	succeeded, lockedCount := 0, 0
	for range 2 {
		result := <-results
		switch {
		case result.err == nil:
			succeeded++
		case errors.Is(result.err, ErrStudioSessionLocked):
			lockedCount++
		default:
			t.Fatalf("concurrent acquire error = %v", result.err)
		}
	}
	if succeeded != 1 || lockedCount != 1 {
		t.Fatalf("concurrent acquire results: succeeded=%d locked=%d", succeeded, lockedCount)
	}
	if err := database.StudioSessions.Terminate(ctx, databaseID); err != nil {
		t.Fatal(err)
	}
}
