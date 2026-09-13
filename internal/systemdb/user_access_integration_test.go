package systemdb

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestManagedUserAccessAndRevocationIntegration(t *testing.T) {
	database := openAdministratorTestDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, _, err := database.Administrators.CreateInitial(ctx, "admin", "administrator password")
	if err != nil {
		t.Fatal(err)
	}
	user, err := database.Users.Create(ctx, UserCreation{ID: uuid.MustNew(), Login: "developer", Password: "developer password", MetadataAdministrator: true, MustChangePassword: true})
	if err != nil {
		t.Fatal(err)
	}
	if !user.MustChangePassword {
		t.Fatal("initial password was not temporary")
	}
	manager, err := database.Sessions.CreateManager(ctx, user.ID, authDigest(user.ID), "local", "test")
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Users.ChangePasswordKeepingSession(ctx, user.Login, "developer password", "private new password", manager.ID); err != nil {
		t.Fatal(err)
	}
	if session, err := database.Sessions.AuthenticateManager(ctx, authDigest(user.ID)); err != nil || session.MustChangePassword {
		t.Fatalf("password change: %+v %v", session, err)
	}
	portalDigest := authDigest(uuid.MustNew())
	if _, err := database.Sessions.CreatePortal(ctx, user.ID, portalDigest, "local", "test"); err != nil {
		t.Fatal(err)
	}
	if err := database.Users.UpdateAccess(ctx, user.ID, user.ID, UserAccessUpdate{Enabled: true, PlatformAdministrator: true}); !errors.Is(err, ErrManagerAccessDenied) {
		t.Fatalf("self elevation: %v", err)
	}
	if err := database.Users.UpdateAccess(ctx, admin.ID, admin.ID, UserAccessUpdate{Enabled: true}); !errors.Is(err, ErrLastPlatformAdministrator) {
		t.Fatalf("last admin demotion: %v", err)
	}
	if err := database.Users.UpdateAccess(ctx, admin.ID, admin.ID, UserAccessUpdate{PlatformAdministrator: true}); !errors.Is(err, ErrLastPlatformAdministrator) {
		t.Fatalf("last admin disabling: %v", err)
	}
	if err := database.Users.UpdateAccess(ctx, admin.ID, user.ID, UserAccessUpdate{}); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Sessions.AuthenticateManager(ctx, authDigest(user.ID)); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("disabled Manager: %v", err)
	}
	if _, err := database.Sessions.AuthenticatePortal(ctx, portalDigest); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("disabled Portal: %v", err)
	}
	if err := database.Users.UpdateAccess(ctx, admin.ID, user.ID, UserAccessUpdate{Enabled: true, MetadataAdministrator: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Sessions.AuthenticateManager(ctx, authDigest(user.ID)); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("old token revived: %v", err)
	}
	if _, err := database.Sessions.AuthenticatePortal(ctx, portalDigest); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("old Portal revived: %v", err)
	}
	var audited int
	if err := database.pool.QueryRow(ctx, `SELECT count(*) FROM ml_system.audit_events WHERE event_code='user.access_changed' AND user_id=$1`, admin.ID.String()).Scan(&audited); err != nil || audited != 2 {
		t.Fatalf("audit count=%d error=%v", audited, err)
	}
}

func TestConcurrentAdministratorDemotionsKeepOneIntegration(t *testing.T) {
	database := openAdministratorTestDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	first, _, err := database.Administrators.CreateInitial(ctx, "first", "administrator password")
	if err != nil {
		t.Fatal(err)
	}
	second, err := database.Users.Create(ctx, UserCreation{ID: uuid.MustNew(), Login: "second", Password: "administrator password", PlatformAdministrator: true})
	if err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 2)
	var wait sync.WaitGroup
	for _, id := range []uuid.UUID{first.ID, second.ID} {
		wait.Add(1)
		go func() {
			defer wait.Done()
			results <- database.Users.UpdateAccess(ctx, id, id, UserAccessUpdate{Enabled: true})
		}()
	}
	wait.Wait()
	close(results)
	updated, denied := 0, 0
	for err := range results {
		switch {
		case err == nil:
			updated++
		case errors.Is(err, ErrLastPlatformAdministrator):
			denied++
		default:
			t.Fatal(err)
		}
	}
	if updated != 1 || denied != 1 {
		t.Fatalf("updated=%d denied=%d", updated, denied)
	}
}
