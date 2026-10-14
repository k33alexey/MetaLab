package platform

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/appconfig"
	"github.com/k33alexey/MetaLab/internal/systemdb"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// Defect: a method that checks the ML System connection itself answers "not
// configured" and hides why the connection failed (CreateInitial did).
func TestRuntimeMethodsReportWhyMLSystemIsNotConnected(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	runtime := New(ctx, appconfig.Default(), nil) // fails with "OS credential store is unavailable"
	defer runtime.Close()
	id := uuid.MustNew()
	errorOf := func(_ any, err error) error { return err }
	calls := map[string]func() error{
		"InitialSetupRequired": func() error { return errorOf(runtime.InitialSetupRequired(ctx)) },
		"CreateInitial": func() error {
			_, _, err := runtime.CreateInitial(ctx, "admin", "password")
			return err
		},
		"ListDatabases": func() error { return errorOf(runtime.ListDatabases(ctx)) },
		"OpenDebugDatabase": func() error {
			_, _, err := runtime.OpenDebugDatabase(ctx, id)
			return err
		},
		"RegisterDatabase": func() error { return errorOf(runtime.RegisterDatabase(ctx, RegisterDatabaseRequest{})) },
		"CreateDebugDatabase": func() error {
			return errorOf(runtime.CreateDebugDatabase(ctx, id, CreateDebugDatabaseRequest{}))
		},
		"UnregisterDatabase":       func() error { return runtime.UnregisterDatabase(ctx, id) },
		"LoginPortal":              func() error { return errorOf(runtime.LoginPortal(ctx, "user", "password", "", "")) },
		"LoginManager":             func() error { return errorOf(runtime.LoginManager(ctx, "user", "password", "", "")) },
		"StartDatabase":            func() error { return errorOf(runtime.StartDatabase(ctx, id)) },
		"StopDatabase":             func() error { return errorOf(runtime.StopDatabase(ctx, id)) },
		"SetDatabaseSessionAccess": func() error { return errorOf(runtime.SetDatabaseSessionAccess(ctx, id, true)) },
		"CheckDatabaseHealth":      func() error { return errorOf(runtime.CheckDatabaseHealth(ctx, id)) },
		"ListDatabaseSessions":     func() error { return errorOf(runtime.ListDatabaseSessions(ctx, nil)) },
		"SendSessionMessage":       func() error { return runtime.SendSessionMessage(ctx, id, "message") },
		"TerminateDatabaseSession": func() error { return runtime.TerminateDatabaseSession(ctx, id) },
		"ListAuditEvents":          func() error { return errorOf(runtime.ListAuditEvents(ctx, nil, 10)) },
		"CreateDatabaseBackup":     func() error { return errorOf(runtime.CreateDatabaseBackup(ctx, id)) },
		"ListDatabaseBackups":      func() error { return errorOf(runtime.ListDatabaseBackups(ctx, id)) },
		"DeleteDatabaseBackup":     func() error { return runtime.DeleteDatabaseBackup(ctx, id, id) },
		"RestoreDatabaseBackup":    func() error { return runtime.RestoreDatabaseBackup(ctx, id, id) },
		"ListStudioSessions":       func() error { return errorOf(runtime.ListStudioSessions(ctx)) },
		"TerminateStudioSession":   func() error { return runtime.TerminateStudioSession(ctx, id) },
		"openApplicationPool": func() error {
			_, _, err := runtime.openApplicationPool(ctx, id)
			return err
		},
	}
	for name, call := range calls {
		if err := call(); err == nil || !strings.Contains(err.Error(), "credential store is unavailable") {
			t.Errorf("%s error = %v, want the connection failure", name, err)
		}
	}
}

// Defect: a token method that reaches a missing connection panics or leaks
// the connection failure instead of refusing the token.
func TestRuntimeTokenMethodsRefuseTokensWithoutMLSystem(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	runtime := New(ctx, appconfig.Default(), nil)
	defer runtime.Close()
	id := uuid.MustNew()
	if _, err := runtime.AuthenticatePortal(ctx, "token"); !errors.Is(err, systemdb.ErrSessionNotFound) {
		t.Errorf("AuthenticatePortal error = %v", err)
	}
	if err := runtime.LogoutPortal(ctx, "token"); !errors.Is(err, systemdb.ErrSessionNotFound) {
		t.Errorf("LogoutPortal error = %v", err)
	}
	if _, _, err := runtime.LoadPortal(ctx, "token"); !errors.Is(err, systemdb.ErrSessionNotFound) {
		t.Errorf("LoadPortal error = %v", err)
	}
	if _, err := runtime.HeartbeatStudioSession(ctx, id, "token", 1); !errors.Is(err, systemdb.ErrStudioSessionNotFound) {
		t.Errorf("HeartbeatStudioSession error = %v", err)
	}
	if err := runtime.ReleaseStudioSession(ctx, id, "token"); !errors.Is(err, systemdb.ErrStudioSessionNotFound) {
		t.Errorf("ReleaseStudioSession error = %v", err)
	}
}
