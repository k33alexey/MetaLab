package systemdb

import (
	"errors"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestApplicationRoleSelectionValidation(t *testing.T) {
	t.Parallel()
	project, role := uuid.MustNew(), uuid.MustNew()
	for _, test := range []struct {
		project  uuid.UUID
		roles    []uuid.UUID
		revision int64
	}{{uuid.UUID{}, nil, 0}, {project, []uuid.UUID{{}}, 0}, {project, []uuid.UUID{role, role}, 0}, {project, nil, -1}, {project, make([]uuid.UUID, MaxApplicationRoles+1), 0}} {
		if _, err := canonicalApplicationRoles(test.project, test.roles, test.revision); !errors.Is(err, ErrInvalidApplicationRoles) {
			t.Fatalf("invalid selection accepted: %v", err)
		}
	}
	input := []uuid.UUID{role, uuid.MustNew()}
	sorted, err := canonicalApplicationRoles(project, input, 0)
	if err != nil || len(sorted) != 2 || sorted[0].String() > sorted[1].String() {
		t.Fatalf("canonical roles: %v %v", sorted, err)
	}
	sorted[0] = uuid.UUID{}
	if input[0].IsZero() || input[1].IsZero() {
		t.Fatal("canonical selection aliases caller")
	}
}
