package systemdb

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/k33alexey/MetaLab/internal/auth"
	"github.com/k33alexey/MetaLab/internal/postgresconn"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestApplicationRolesIsolationRevocationAndConcurrencyIntegration(t *testing.T) {
	url := os.Getenv("ML_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("ML_TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	database, err := Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(database.Close)
	admin, member, other := uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	one, two, project := uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	users, databases := []string{admin.String(), member.String(), other.String()}, []string{one.String(), two.String()}
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		for _, query := range []struct {
			sql string
			ids []string
		}{
			{`DELETE FROM ml_system.audit_events WHERE database_id=ANY($1::uuid[])`, databases},
			{`DELETE FROM ml_system.databases WHERE id=ANY($1::uuid[])`, databases},
			{`DELETE FROM ml_system.users WHERE id=ANY($1::uuid[])`, users},
		} {
			if _, err := database.pool.Exec(cleanup, query.sql, query.ids); err != nil {
				t.Error(err)
			}
		}
	})
	hash, err := auth.HashPassword("application role integration password")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []uuid.UUID{admin, member, other} {
		if _, err := database.pool.Exec(ctx, `INSERT INTO ml_system.users(id,login,password_hash,platform_administrator) VALUES($1,$2,$3,$4)`, id.String(), "roles-"+id.String(), hash, id == admin); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []uuid.UUID{one, two} {
		registered, err := database.Databases.Register(ctx, DatabaseRegistration{ID: id, Name: "Roles " + id.String(), PhysicalID: uuid.MustNew(), Mode: DatabaseDebug, OwnerUserID: &admin,
			Connection: postgresconn.Descriptor{Host: "localhost", Port: 5432, Database: "roles", User: "ml", SSLMode: "disable", SecretKey: "test.roles." + id.String()}})
		if err != nil {
			t.Fatal(err)
		}
		starting, err := database.Databases.Transition(ctx, id, registered.State, registered.StateRevision, DatabaseStarting, "")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := database.Databases.Transition(ctx, id, DatabaseStarting, starting.StateRevision, DatabaseRunning, ""); err != nil {
			t.Fatal(err)
		}
		for _, user := range []uuid.UUID{admin, member, other} {
			if err := database.DatabaseAccess.SetPermissions(ctx, admin, user, id, DatabasePermissions{App: true, Admin: user == admin}); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, user := range []uuid.UUID{admin, member} {
		assignment, err := database.DatabaseAccess.ApplicationRoles(ctx, user, one)
		if err != nil || assignment.ProjectID != nil || assignment.Revision != 0 || len(assignment.RoleIDs) != 0 {
			t.Fatalf("implicit application grant: %+v %v", assignment, err)
		}
	}
	roleA, roleB := uuid.MustNew(), uuid.MustNew()
	if _, err := database.DatabaseAccess.SetApplicationRoles(ctx, other, member, one, project, []uuid.UUID{roleA}, 0); !errors.Is(err, ErrDatabaseOwnerOnly) {
		t.Fatalf("ordinary user changed roles: %v", err)
	}
	if _, err := database.DatabaseAccess.GetApplicationRoles(ctx, other, member, one); !errors.Is(err, ErrDatabaseAccessDenied) {
		t.Fatalf("ordinary user read role directory: %v", err)
	}
	if _, err := database.DatabaseAccess.SetApplicationRoles(ctx, admin, member, uuid.MustNew(), project, []uuid.UUID{roleA}, 0); !errors.Is(err, ErrDatabaseAccessDenied) {
		t.Fatalf("unknown database: %v", err)
	}
	assigned, err := database.DatabaseAccess.SetApplicationRoles(ctx, admin, member, one, project, []uuid.UUID{roleA}, 0)
	if err != nil || assigned.Revision != 1 || assigned.ProjectID == nil || *assigned.ProjectID != project {
		t.Fatalf("assign: %+v %v", assigned, err)
	}
	for _, pair := range [][2]uuid.UUID{{member, two}, {other, one}, {admin, one}} {
		assignment, err := database.DatabaseAccess.ApplicationRoles(ctx, pair[0], pair[1])
		if err != nil || len(assignment.RoleIDs) != 0 {
			t.Fatalf("roles leaked to another user/database: %+v %v", assignment, err)
		}
	}
	memberDigest, otherDigest := auth.SessionTokenDigest("roles-member-"+member.String()), auth.SessionTokenDigest("roles-other-"+other.String())
	portal, err := database.Sessions.CreatePortal(ctx, member, memberDigest[:], "local", "roles test")
	if err != nil {
		t.Fatal(err)
	}
	otherPortal, err := database.Sessions.CreatePortal(ctx, other, otherDigest[:], "local", "roles test")
	if err != nil {
		t.Fatal(err)
	}
	for _, pair := range [][2]uuid.UUID{{portal.ID, one}, {portal.ID, two}, {otherPortal.ID, one}} {
		if _, err := database.Sessions.OpenDatabase(ctx, pair[0], pair[1]); err != nil {
			t.Fatal(err)
		}
	}
	unchanged, err := database.DatabaseAccess.SetApplicationRoles(ctx, admin, member, one, project, []uuid.UUID{roleA}, 1)
	if err != nil || unchanged.Revision != 1 {
		t.Fatalf("no-op: %+v %v", unchanged, err)
	}
	if _, err := database.Sessions.ResumeDatabase(ctx, portal.ID, one); err != nil {
		t.Fatalf("no-op terminated session: %v", err)
	}
	if _, err := database.DatabaseAccess.SetApplicationRoles(ctx, admin, member, one, project, []uuid.UUID{roleB}, 0); !errors.Is(err, ErrApplicationRolesChanged) {
		t.Fatalf("stale write: %v", err)
	}
	if _, err := database.Sessions.ResumeDatabase(ctx, portal.ID, one); err != nil {
		t.Fatalf("stale write terminated session: %v", err)
	}
	var writers sync.WaitGroup
	start, results := make(chan struct{}), make(chan error, 2)
	for _, roles := range [][]uuid.UUID{{roleB}, {roleA, roleB}} {
		writers.Add(1)
		go func(roles []uuid.UUID) {
			defer writers.Done()
			<-start
			_, err := database.DatabaseAccess.SetApplicationRoles(ctx, admin, member, one, project, roles, 1)
			results <- err
		}(roles)
	}
	close(start)
	writers.Wait()
	close(results)
	succeeded, conflicted := 0, 0
	for err := range results {
		if err == nil {
			succeeded++
		} else if errors.Is(err, ErrApplicationRolesChanged) {
			conflicted++
		} else {
			t.Fatal(err)
		}
	}
	if succeeded != 1 || conflicted != 1 {
		t.Fatalf("concurrent saves: successes=%d conflicts=%d", succeeded, conflicted)
	}
	if _, err := database.Sessions.ResumeDatabase(ctx, portal.ID, one); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("old session retained roles: %v", err)
	}
	for _, pair := range [][2]uuid.UUID{{portal.ID, two}, {otherPortal.ID, one}} {
		if _, err := database.Sessions.ResumeDatabase(ctx, pair[0], pair[1]); err != nil {
			t.Fatalf("unrelated session revoked: %v", err)
		}
	}
	if _, err := database.Sessions.AuthenticatePortal(ctx, memberDigest[:]); err != nil {
		t.Fatalf("role edit revoked Portal: %v", err)
	}
	var events int
	if err := database.pool.QueryRow(ctx, `SELECT count(*) FROM ml_system.audit_events WHERE database_id=$1 AND event_code='application.roles_changed'`, one.String()).Scan(&events); err != nil || events != 2 {
		t.Fatalf("audit count=%d error=%v", events, err)
	}
	configuration, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	configuration.MaxConns = 1
	limited, err := pgxpool.NewWithConfig(ctx, configuration)
	if err != nil {
		t.Fatal(err)
	}
	defer limited.Close()
	repository := &DatabaseAccessRepository{pool: limited}
	bounded, stop := context.WithTimeout(ctx, 2*time.Second)
	cleared, err := repository.SetApplicationRoles(bounded, admin, member, one, project, nil, 2)
	stop()
	if err != nil || cleared.Revision != 3 || len(cleared.RoleIDs) != 0 {
		t.Fatalf("one-connection clear: %+v %v", cleared, err)
	}
	if _, err := database.DatabaseAccess.GetApplicationRoles(ctx, admin, member, one); err != nil {
		t.Fatal(err)
	}
	if err := database.DatabaseAccess.RevokeApp(ctx, admin, member, one); err != nil {
		t.Fatal(err)
	}
	if _, err := database.DatabaseAccess.ApplicationRoles(ctx, member, one); !errors.Is(err, ErrDatabaseAccessDenied) {
		t.Fatalf("revoked member retained roles: %v", err)
	}
	if _, err := database.DatabaseAccess.SetApplicationRoles(ctx, admin, member, one, project, []uuid.UUID{roleA}, 3); !errors.Is(err, ErrDatabaseAccessDenied) {
		t.Fatalf("roles granted without App admission: %v", err)
	}
	if _, err := database.pool.Exec(ctx, `UPDATE ml_system.users SET enabled=FALSE WHERE id=$1`, other.String()); err != nil {
		t.Fatal(err)
	}
	if _, err := database.DatabaseAccess.SetApplicationRoles(ctx, admin, other, one, project, []uuid.UUID{roleA}, 0); !errors.Is(err, ErrDatabaseAccessDenied) {
		t.Fatalf("disabled recipient: %v", err)
	}
}
