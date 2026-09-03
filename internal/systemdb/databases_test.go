package systemdb

import (
	"errors"
	"testing"

	"github.com/k33alexey/MetaLab/internal/postgresconn"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestDatabaseRegistrationValidation(t *testing.T) {
	t.Parallel()

	valid := DatabaseRegistration{
		ID: uuid.MustNew(), Name: "Продажи", PhysicalID: uuid.MustNew(),
		Connection: postgresconn.Descriptor{
			Host: "localhost", Port: 5432, Database: "sales", User: "sales",
			SSLMode: "disable", SecretKey: "database.example.password",
		},
	}
	if err := validateDatabaseRegistration(valid); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"", " Sales", "Sales\n"} {
		invalid := valid
		invalid.Name = name
		if err := validateDatabaseRegistration(invalid); err == nil {
			t.Fatalf("name %q was accepted", name)
		}
	}
}

func TestDatabaseStateTransitionGraph(t *testing.T) {
	t.Parallel()

	allowed := [][2]DatabaseState{
		{DatabaseStopped, DatabaseStarting}, {DatabaseStarting, DatabaseRunning},
		{DatabaseRunning, DatabaseStopping}, {DatabaseStopping, DatabaseStopped},
		{DatabaseRunning, DatabaseError}, {DatabaseError, DatabaseMaintenance},
	}
	for _, transition := range allowed {
		if !validStateTransition(transition[0], transition[1]) {
			t.Fatalf("transition %q -> %q rejected", transition[0], transition[1])
		}
	}
	for _, transition := range [][2]DatabaseState{{DatabaseStopped, DatabaseRunning}, {DatabaseRunning, DatabaseMaintenance}, {DatabaseStopped, DatabaseStopped}} {
		if validStateTransition(transition[0], transition[1]) {
			t.Fatalf("transition %q -> %q accepted", transition[0], transition[1])
		}
	}
}

func TestDatabaseRegistryErrorsAreMatchable(t *testing.T) {
	t.Parallel()

	for _, target := range []error{ErrDatabaseNotFound, ErrDatabaseNameExists, ErrPhysicalDatabaseExists, ErrDatabaseStateConflict, ErrDatabaseCannotUnregister} {
		if !errors.Is(errors.Join(target, errors.New("context")), target) {
			t.Fatalf("error %v is not matchable", target)
		}
	}
}
