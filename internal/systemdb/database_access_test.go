package systemdb

import (
	"errors"
	"testing"
)

func TestDatabaseAccessErrorsAreMatchable(t *testing.T) {
	t.Parallel()
	for _, target := range []error{ErrDatabaseAccessDenied, ErrDatabaseOwnerOnly} {
		if !errors.Is(errors.Join(target, errors.New("context")), target) {
			t.Fatalf("error %v is not matchable", target)
		}
	}
}
