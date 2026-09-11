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

func TestPersonalDatabaseAccessAndRevocationIntegration(t *testing.T) {
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
	t.Cleanup(database.Close)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	administratorID, memberID, outsiderID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	passwordHash, err := auth.HashPassword("personal database password")
	if err != nil {
		t.Fatal(err)
	}
	for _, user := range []struct {
		id       uuid.UUID
		login    string
		platform bool
	}{
		{administratorID, "access-admin-" + suffix, true},
		{memberID, "access-member-" + suffix, false},
		{outsiderID, "access-outsider-" + suffix, false},
	} {
		if _, err := database.pool.Exec(ctx, `
INSERT INTO ml_system.users(id, login, password_hash, platform_administrator)
VALUES ($1, $2, $3, $4)`, user.id.String(), user.login, passwordHash, user.platform); err != nil {
			t.Fatal(err)
		}
	}

	databaseID := uuid.MustNew()
	registered, err := database.Databases.Register(ctx, DatabaseRegistration{
		ID: databaseID, Name: "Personal " + suffix, PhysicalID: uuid.MustNew(), Mode: DatabasePrimary,
		Connection: postgresconn.Descriptor{
			Host: "localhost", Port: 5432, Database: "personal_" + suffix, User: "ml",
			SSLMode: "disable", SecretKey: "database." + suffix + ".personal",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = database.pool.Exec(cleanupCtx, "DELETE FROM ml_system.portal_sessions WHERE user_id IN ($1, $2, $3)", administratorID.String(), memberID.String(), outsiderID.String())
		_, _ = database.pool.Exec(cleanupCtx, "UPDATE ml_system.databases SET state = 'stopped' WHERE id = $1", databaseID.String())
		_, _ = database.Databases.Unregister(cleanupCtx, databaseID)
		_, _ = database.pool.Exec(cleanupCtx, "DELETE FROM ml_system.users WHERE id IN ($1, $2, $3)", administratorID.String(), memberID.String(), outsiderID.String())
	})

	var ownerIDText string
	if err := database.pool.QueryRow(ctx, `
SELECT user_id::text FROM ml_system.database_access
WHERE database_id = $1 AND access_level = 'owner' AND revoked_at IS NULL`, databaseID.String()).Scan(&ownerIDText); err != nil {
		t.Fatal(err)
	}
	ownerID, err := uuid.Parse(ownerIDText)
	if err != nil {
		t.Fatal(err)
	}
	ownerAccess, err := database.DatabaseAccess.Get(ctx, ownerID, databaseID)
	if err != nil || ownerAccess.AppAccess || !ownerAccess.StudioAccess || !ownerAccess.DatabaseAdmin {
		t.Fatalf("initial owner access=%+v error=%v", ownerAccess, err)
	}
	if _, err := database.DatabaseAccess.GrantApp(ctx, administratorID, memberID, databaseID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Users.Authenticate(ctx, "ACCESS-MEMBER-"+suffix, "personal database password"); err != nil {
		t.Fatalf("regular user authentication failed: %v", err)
	}
	memberAccess, err := database.DatabaseAccess.ListApp(ctx, memberID)
	if err != nil || len(memberAccess) != 1 || memberAccess[0].DatabaseID != databaseID || memberAccess[0].Level != DatabaseMember || !memberAccess[0].AppAccess {
		t.Fatalf("member access=%+v error=%v", memberAccess, err)
	}
	if _, err := database.DatabaseAccess.GetApp(ctx, outsiderID, databaseID); !errors.Is(err, ErrDatabaseAccessDenied) {
		t.Fatalf("outsider access error=%v", err)
	}

	starting, err := database.Databases.Transition(ctx, databaseID, registered.State, registered.StateRevision, DatabaseStarting, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Databases.Transition(ctx, databaseID, DatabaseStarting, starting.StateRevision, DatabaseRunning, ""); err != nil {
		t.Fatal(err)
	}
	memberDigest := auth.SessionTokenDigest("member-" + suffix)
	memberPortal, err := database.Sessions.CreatePortal(ctx, memberID, memberDigest[:], "127.0.0.1", "integration-test")
	if err != nil {
		t.Fatal(err)
	}
	outsiderDigest := auth.SessionTokenDigest("outsider-" + suffix)
	outsiderPortal, err := database.Sessions.CreatePortal(ctx, outsiderID, outsiderDigest[:], "127.0.0.2", "integration-test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Sessions.OpenDatabase(ctx, outsiderPortal.ID, databaseID); !errors.Is(err, ErrDatabaseAccessDenied) {
		t.Fatalf("outsider database open error=%v", err)
	}
	opened, err := database.Sessions.OpenDatabase(ctx, memberPortal.ID, databaseID)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.DatabaseAccess.RevokeApp(ctx, administratorID, memberID, databaseID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Sessions.ResumeDatabase(ctx, memberPortal.ID, databaseID); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("revoked session %s resume error=%v", opened.ID, err)
	}
	if access, err := database.DatabaseAccess.ListApp(ctx, memberID); err != nil || len(access) != 0 {
		t.Fatalf("revoked member access=%+v error=%v", access, err)
	}
	if _, err := database.DatabaseAccess.GrantApp(ctx, administratorID, ownerID, databaseID); err != nil {
		t.Fatal(err)
	}
	if err := database.DatabaseAccess.RevokeApp(ctx, administratorID, ownerID, databaseID); err != nil {
		t.Fatal(err)
	}
	ownerAccess, err = database.DatabaseAccess.Get(ctx, ownerID, databaseID)
	if err != nil || ownerAccess.AppAccess || ownerAccess.Level != DatabaseOwner {
		t.Fatalf("owner after App revocation=%+v error=%v", ownerAccess, err)
	}
}
