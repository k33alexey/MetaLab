package systemdb

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/k33alexey/MetaLab/internal/auth"
	"github.com/k33alexey/MetaLab/internal/postgresconn"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestSessionsAuditAndBackupCatalogIntegration(t *testing.T) {
	databaseURL := os.Getenv("ML_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("ML_TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	database, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	userID, databaseID := uuid.MustNew(), uuid.MustNew()
	passwordHash, err := auth.HashPassword("integration password")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.pool.Exec(ctx, `
INSERT INTO ml_system.users(id, login, password_hash, platform_administrator, metadata_administrator)
VALUES ($1, $2, $3, TRUE, TRUE)`, userID.String(), "operations-"+suffix, passwordHash); err != nil {
		t.Fatal(err)
	}
	registered, err := database.Databases.Register(ctx, DatabaseRegistration{
		ID: databaseID, Name: "Operations " + suffix, PhysicalID: uuid.MustNew(), Mode: DatabasePrimary,
		Connection: postgresconn.Descriptor{
			Host: "localhost", Port: 5432, Database: "operations", User: "ml", SSLMode: "disable",
			SecretKey: "database." + suffix + ".password",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = database.pool.Exec(cleanupCtx, "DELETE FROM ml_system.backups WHERE database_id = $1", databaseID.String())
		_, _ = database.pool.Exec(cleanupCtx, "DELETE FROM ml_system.audit_events WHERE database_id = $1 OR user_id = $2", databaseID.String(), userID.String())
		_, _ = database.pool.Exec(cleanupCtx, "DELETE FROM ml_system.portal_sessions WHERE user_id = $1", userID.String())
		_, _ = database.pool.Exec(cleanupCtx, "UPDATE ml_system.databases SET state = 'stopped' WHERE id = $1", databaseID.String())
		_, _ = database.Databases.Unregister(cleanupCtx, databaseID)
		_, _ = database.pool.Exec(cleanupCtx, "DELETE FROM ml_system.users WHERE id = $1", userID.String())
	})
	starting, err := database.Databases.Transition(ctx, registered.ID, registered.State, registered.StateRevision, DatabaseStarting, "")
	if err != nil {
		t.Fatal(err)
	}
	registered, err = database.Databases.Transition(ctx, registered.ID, DatabaseStarting, starting.StateRevision, DatabaseRunning, "")
	if err != nil {
		t.Fatal(err)
	}
	digest := auth.SessionTokenDigest("integration-token")
	portalSession, err := database.Sessions.CreatePortal(ctx, userID, digest[:], "127.0.0.1", "integration-test")
	if err != nil {
		t.Fatal(err)
	}
	authenticated, err := database.Sessions.AuthenticatePortal(ctx, digest[:])
	if err != nil || authenticated.ID != portalSession.ID || !authenticated.MetadataAdmin {
		t.Fatalf("authenticated=%+v error=%v", authenticated, err)
	}
	applicationSession, err := database.Sessions.OpenDatabase(ctx, portalSession.ID, databaseID)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Sessions.SendMessage(ctx, applicationSession.ID, "Maintenance soon"); err != nil {
		t.Fatal(err)
	}
	sessions, err := database.Sessions.ListDatabaseSessions(ctx, &databaseID)
	if err != nil || len(sessions) != 1 || sessions[0].Message != "Maintenance soon" {
		t.Fatalf("sessions=%+v error=%v", sessions, err)
	}
	if _, err := database.Databases.SetSessionAccess(ctx, databaseID, false); err != nil {
		t.Fatal(err)
	}
	if resumed, err := database.Sessions.ResumeDatabase(ctx, portalSession.ID, databaseID); err != nil || resumed.ID != applicationSession.ID {
		t.Fatalf("existing session after access block=%+v error=%v", resumed, err)
	}
	if err := database.Sessions.AcknowledgeMessage(ctx, portalSession.ID, applicationSession.ID); err != nil {
		t.Fatal(err)
	}
	secondDigest := auth.SessionTokenDigest("second-token")
	secondPortal, err := database.Sessions.CreatePortal(ctx, userID, secondDigest[:], "127.0.0.2", "integration-test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Sessions.OpenDatabase(ctx, secondPortal.ID, databaseID); !errors.Is(err, ErrNewSessionsForbidden) {
		t.Fatalf("forbidden open error = %v", err)
	}
	if err := database.Administrators.ChangePasswordKeepingSession(
		ctx, "operations-"+suffix, "integration password", "changed integration password", portalSession.ID,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Sessions.AuthenticatePortal(ctx, secondDigest[:]); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("other session after password change error = %v", err)
	}
	if _, err := database.Sessions.AuthenticatePortal(ctx, digest[:]); err != nil {
		t.Fatalf("kept session after password change error = %v", err)
	}
	if err := database.Sessions.TerminateDatabaseSession(ctx, applicationSession.ID); err != nil {
		t.Fatal(err)
	}
	if sessions, err := database.Sessions.ListDatabaseSessions(ctx, &databaseID); err != nil || len(sessions) != 0 {
		t.Fatalf("active sessions=%+v error=%v", sessions, err)
	}
	event, err := database.Audit.Write(ctx, AuditEvent{
		Level: "info", Code: "integration.event", DatabaseID: &databaseID, UserID: &userID,
		Message: "Integration event", Details: map[string]any{"safe": true},
	})
	if err != nil || event.ID == 0 {
		t.Fatalf("event=%+v error=%v", event, err)
	}
	events, err := database.Audit.List(ctx, &databaseID, 10)
	if err != nil || len(events) == 0 || events[0].Code != "integration.event" {
		t.Fatalf("events=%+v error=%v", events, err)
	}
	backup, err := database.Backups.Add(ctx, Backup{
		ID: uuid.MustNew(), DatabaseID: databaseID, FileName: "integration-" + suffix + ".mlbackup",
		SizeBytes: 128, SHA256: strings.Repeat("a", 64),
	})
	if err != nil {
		t.Fatal(err)
	}
	backups, err := database.Backups.List(ctx, databaseID)
	if err != nil || len(backups) != 1 || backups[0].ID != backup.ID {
		t.Fatalf("backups=%+v error=%v", backups, err)
	}
	if _, err := database.pool.Exec(ctx, "UPDATE ml_system.databases SET state = 'stopped' WHERE id = $1", databaseID.String()); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Databases.Unregister(ctx, databaseID); !errors.Is(err, ErrDatabaseHasBackups) {
		t.Fatalf("unregister with backup error = %v", err)
	}
	if removed, err := database.Backups.Delete(ctx, backup.ID); err != nil || removed.ID != backup.ID {
		t.Fatalf("removed backup=%+v error=%v", removed, err)
	}
	if _, err := database.Databases.Unregister(ctx, databaseID); err != nil {
		t.Fatal(err)
	}
}
