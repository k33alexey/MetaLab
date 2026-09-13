package systemdb

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/k33alexey/MetaLab/internal/auth"
	"github.com/k33alexey/MetaLab/internal/postgresconn"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestManagerSessionAndPermissionBoundariesIntegration(t *testing.T) {
	databaseURL := os.Getenv("ML_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("ML_TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	database, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(database.Close)
	admin, developer, ordinary := uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	databaseID, foreignID := uuid.MustNew(), uuid.MustNew()
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		_, err := database.pool.Exec(cleanup, `DELETE FROM ml_system.databases WHERE id=ANY($1::uuid[])`, []string{databaseID.String(), foreignID.String()})
		if err != nil {
			t.Error(err)
		}
		_, err = database.pool.Exec(cleanup, `DELETE FROM ml_system.users WHERE id=ANY($1::uuid[])`, []string{admin.String(), developer.String(), ordinary.String()})
		if err != nil {
			t.Error(err)
		}
	})
	hash, err := auth.HashPassword("manager testing password")
	if err != nil {
		t.Fatal(err)
	}
	for _, user := range []struct {
		id                 uuid.UUID
		platform, metadata bool
	}{{admin, true, true}, {developer, false, true}, {ordinary, false, false}} {
		_, err := database.pool.Exec(ctx, `INSERT INTO ml_system.users(id,login,password_hash,platform_administrator,metadata_administrator) VALUES($1,$2,$3,$4,$5)`, user.id.String(), "manager-"+user.id.String(), hash, user.platform, user.metadata)
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []uuid.UUID{databaseID, foreignID} {
		owner := admin
		if id == databaseID {
			owner = developer
		}
		_, err := database.Databases.Register(ctx, DatabaseRegistration{ID: id, Name: "Manager " + id.String(), PhysicalID: uuid.MustNew(), Mode: DatabaseDebug, OwnerUserID: &owner, Connection: postgresconn.Descriptor{Host: "localhost", Port: 5432, Database: "test", User: "ml", SSLMode: "disable", SecretKey: "test." + id.String()}})
		if err != nil {
			t.Fatal(err)
		}
	}
	access, err := database.DatabaseAccess.Get(ctx, developer, databaseID)
	if err != nil || access.AppAccess || !access.StudioAccess || !access.DatabaseAdmin || access.Level != DatabaseOwner {
		t.Fatalf("developer ownership: %+v %v", access, err)
	}
	owners, err := database.DatabaseAccess.OwnersForDatabases(ctx, []uuid.UUID{databaseID})
	if err != nil || len(owners) != 1 || owners[databaseID].UserID != developer || owners[databaseID].Login != "manager-"+developer.String() {
		t.Fatalf("scoped owners: %+v %v", owners, err)
	}
	if owners, err := database.DatabaseAccess.OwnersForDatabases(ctx, nil); err != nil || len(owners) != 0 {
		t.Fatalf("empty scope: %+v %v", owners, err)
	}
	if owners, err := database.DatabaseAccess.OwnersForDatabases(ctx, []uuid.UUID{uuid.MustNew()}); err != nil || len(owners) != 0 {
		t.Fatalf("unknown database owners: %+v %v", owners, err)
	}
	portalDigest, managerDigest := auth.SessionTokenDigest("portal-"+developer.String()), auth.SessionTokenDigest("manager-"+developer.String())
	portal, err := database.Sessions.CreatePortal(ctx, developer, portalDigest[:], "local", "test")
	if err != nil {
		t.Fatal(err)
	}
	manager, err := database.Sessions.CreateManager(ctx, developer, managerDigest[:], "local", "test")
	if err != nil {
		t.Fatal(err)
	}
	if portal.ID == manager.ID {
		t.Fatal("Portal and Manager shared a session")
	}
	if _, err := database.Sessions.AuthenticateManager(ctx, portalDigest[:]); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("Portal token accepted by Manager: %v", err)
	}
	if _, err := database.Sessions.AuthenticatePortal(ctx, managerDigest[:]); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("Manager token accepted by Portal: %v", err)
	}
	if _, err := database.Sessions.CreateManager(ctx, ordinary, authDigest(ordinary), "local", "test"); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("ordinary Manager login: %v", err)
	}
	if err := database.Sessions.RevokeManager(ctx, portalDigest[:]); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("cross-purpose logout: %v", err)
	}
	if err := database.Sessions.RevokeManager(ctx, managerDigest[:]); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Sessions.AuthenticatePortal(ctx, portalDigest[:]); err != nil {
		t.Fatalf("Manager logout revoked Portal: %v", err)
	}
	if err := database.DatabaseAccess.SetPermissions(ctx, developer, ordinary, foreignID, DatabasePermissions{Admin: true}); !errors.Is(err, ErrDatabaseOwnerOnly) {
		t.Fatalf("foreign grant: %v", err)
	}
	if err := database.DatabaseAccess.SetPermissions(ctx, admin, ordinary, databaseID, DatabasePermissions{Studio: true}); err == nil {
		t.Fatal("Studio granted without system role")
	}
	if err := database.DatabaseAccess.SetPermissions(ctx, developer, ordinary, databaseID, DatabasePermissions{App: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Sessions.CreateManager(ctx, ordinary, authDigest(ordinary), "local", "test"); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("App grant admitted user to Manager: %v", err)
	}
	leaseID, leaseDigest := uuid.MustNew(), auth.SessionTokenDigest("studio-"+developer.String())
	lease, err := database.StudioSessions.AcquireForUser(ctx, developer, databaseID, uuid.MustNew(), leaseID, leaseDigest[:], "test-host", 100)
	if err != nil {
		t.Fatal(err)
	}
	if lease.OwnerName != "manager-"+developer.String() {
		t.Fatalf("wrong ML owner: %s", lease.OwnerName)
	}
	configuration, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	configuration.MaxConns = 1
	limited, err := pgxpool.NewWithConfig(ctx, configuration)
	if err != nil {
		t.Fatal(err)
	}
	defer limited.Close()
	repository := &StudioSessionRepository{pool: limited}
	bounded, stop := context.WithTimeout(ctx, 2*time.Second)
	_, err = repository.AcquireForUser(bounded, developer, databaseID, lease.ProjectID, uuid.MustNew(), authDigest(uuid.MustNew()), "test-host", 102)
	stop()
	if !errors.Is(err, ErrStudioSessionLocked) {
		t.Fatalf("occupied Studio with one connection: %v", err)
	}
	if _, err := database.StudioSessions.Authenticate(ctx, leaseID, leaseDigest[:]); err != nil {
		t.Fatalf("authorized lease: %v", err)
	}
	if _, err := database.StudioSessions.AcquireForUser(ctx, developer, foreignID, uuid.MustNew(), uuid.MustNew(), leaseDigest[:], "test-host", 101); !errors.Is(err, ErrDatabaseAccessDenied) {
		t.Fatalf("foreign Studio access: %v", err)
	}
	if _, err := database.StudioSessions.HeartbeatAuthorized(ctx, leaseID, leaseDigest[:], 101); err != nil {
		t.Fatal(err)
	}
	if err := database.DatabaseAccess.SetPermissions(ctx, admin, developer, databaseID, DatabasePermissions{App: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := database.StudioSessions.HeartbeatAuthorized(ctx, leaseID, leaseDigest[:], 101); !errors.Is(err, ErrStudioSessionNotFound) {
		t.Fatalf("revoked Studio renewed: %v", err)
	}
	if _, err := database.StudioSessions.Authenticate(ctx, leaseID, leaseDigest[:]); !errors.Is(err, ErrStudioSessionNotFound) {
		t.Fatalf("revoked lease read/write: %v", err)
	}
	if err := database.DatabaseAccess.SetPermissions(ctx, developer, developer, databaseID, DatabasePermissions{Admin: true}); !errors.Is(err, ErrDatabaseOwnerOnly) {
		t.Fatalf("ownership elevated removed Admin grant: %v", err)
	}
	items, err := database.DatabaseAccess.ListForDatabase(ctx, admin, databaseID)
	if err != nil || len(items) != 2 {
		t.Fatalf("assignments: %+v %v", items, err)
	}
	for _, item := range items {
		if item.Login == "" {
			t.Fatal("assignment missing login")
		}
	}
	if err := database.DatabaseAccess.SetPermissions(ctx, admin, ordinary, databaseID, DatabasePermissions{Admin: true}); err != nil {
		t.Fatal(err)
	}
	ordinaryManager, err := database.Sessions.CreateManager(ctx, ordinary, authDigest(ordinary), "local", "test")
	if err != nil {
		t.Fatal(err)
	}
	if ordinaryManager.PlatformAdmin || ordinaryManager.MetadataAdmin {
		t.Fatal("database grant changed system roles")
	}
	if err := database.DatabaseAccess.SetPermissions(ctx, admin, ordinary, databaseID, DatabasePermissions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Sessions.AuthenticateManager(ctx, authDigest(ordinary)); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("Manager remained authorized after final grant revoked: %v", err)
	}
}

func authDigest(id uuid.UUID) []byte {
	digest := auth.SessionTokenDigest(id.String())
	return digest[:]
}
