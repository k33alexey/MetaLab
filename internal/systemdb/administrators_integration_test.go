package systemdb

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"
)

func TestInitialAdministratorRecoveryAndEmergencyReset(t *testing.T) {
	database := openAdministratorTestDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	required, err := database.Administrators.InitialSetupRequired(ctx)
	if err != nil || !required {
		t.Fatalf("required = %v, error = %v", required, err)
	}
	administrator, codes, err := database.Administrators.CreateInitial(ctx, "admin", "initial password")
	if err != nil {
		t.Fatal(err)
	}
	if administrator.ID.IsZero() || administrator.Login != "admin" || administrator.MustChangePassword || len(codes) != 10 {
		t.Fatalf("administrator = %+v, code count = %d", administrator, len(codes))
	}
	required, err = database.Administrators.InitialSetupRequired(ctx)
	if err != nil || required {
		t.Fatalf("required = %v, error = %v", required, err)
	}
	if _, _, err := database.Administrators.CreateInitial(ctx, "second", "second password"); !errors.Is(err, ErrInitialAdministratorExists) {
		t.Fatalf("second CreateInitial() error = %v", err)
	}
	authenticated, err := database.Administrators.Authenticate(ctx, "ADMIN", "initial password")
	if err != nil || authenticated.ID != administrator.ID {
		t.Fatalf("authenticated = %+v, error = %v", authenticated, err)
	}
	if _, err := database.Administrators.Authenticate(ctx, "admin", "wrong password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong password error = %v", err)
	}

	if err := database.Administrators.RecoverPassword(ctx, "admin", codes[0], "recovered password"); err != nil {
		t.Fatal(err)
	}
	if err := database.Administrators.RecoverPassword(ctx, "admin", codes[0], "another password"); !errors.Is(err, ErrInvalidRecoveryCode) {
		t.Fatalf("reused recovery code error = %v", err)
	}
	if _, err := database.Administrators.Authenticate(ctx, "admin", "recovered password"); err != nil {
		t.Fatal(err)
	}

	emergency, err := database.Administrators.EmergencyReset(ctx, "admin")
	if err != nil {
		t.Fatal(err)
	}
	if emergency.TemporaryPassword == "" || len(emergency.RecoveryCodes) != 10 {
		t.Fatalf("emergency credentials = %+v", emergency)
	}
	authenticated, err = database.Administrators.Authenticate(ctx, "admin", emergency.TemporaryPassword)
	if err != nil || !authenticated.MustChangePassword {
		t.Fatalf("authenticated = %+v, error = %v", authenticated, err)
	}
	var recoveryCodes int
	if err := database.pool.QueryRow(ctx, "SELECT COUNT(*) FROM ml_system.recovery_codes WHERE user_id = $1", administrator.ID.String()).Scan(&recoveryCodes); err != nil {
		t.Fatal(err)
	}
	if recoveryCodes != 10 {
		t.Fatalf("recovery code count = %d, want 10", recoveryCodes)
	}
	if err := database.Administrators.ChangePassword(ctx, "admin", emergency.TemporaryPassword, "final administrator password"); err != nil {
		t.Fatal(err)
	}
	authenticated, err = database.Administrators.Authenticate(ctx, "admin", "final administrator password")
	if err != nil || authenticated.MustChangePassword {
		t.Fatalf("authenticated after change = %+v, error = %v", authenticated, err)
	}
}

func TestInitialAdministratorCreationIsExclusive(t *testing.T) {
	database := openAdministratorTestDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	const attempts = 2
	var wait sync.WaitGroup
	results := make(chan error, attempts)
	for index := range attempts {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, _, err := database.Administrators.CreateInitial(ctx, "admin"+string(rune('a'+index)), "concurrent password")
			results <- err
		}()
	}
	wait.Wait()
	close(results)
	created, rejected := 0, 0
	for err := range results {
		switch {
		case err == nil:
			created++
		case errors.Is(err, ErrInitialAdministratorExists):
			rejected++
		default:
			t.Fatalf("CreateInitial() error = %v", err)
		}
	}
	if created != 1 || rejected != 1 {
		t.Fatalf("created = %d, rejected = %d", created, rejected)
	}
}

func openAdministratorTestDatabase(t *testing.T) *Database {
	t.Helper()
	databaseURL := os.Getenv("ML_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("ML_TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	database, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.pool.Exec(ctx, "DELETE FROM ml_system.users"); err != nil {
		database.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = database.pool.Exec(cleanupCtx, "DELETE FROM ml_system.users")
		database.Close()
	})
	return database
}
