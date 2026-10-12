package metadata

import (
	"context"
	"fmt"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// readRestricted says the caller's roles restrict reading the object: no role
// of theirs reads it without a restriction. A context without permissions
// (Studio, CLI, role-free tests) reads unrestricted, the same convention
// requireObject follows - the hosting layer decides whether a caller is
// subject to application permissions at all.
func readRestricted(ctx context.Context, objectID uuid.UUID) bool {
	permissions, ok := PermissionsFromContext(ctx)
	return ok && !permissions.ReadsEveryRow(objectID)
}

// readRowPredicate is the extra AND-condition a read of the object takes for
// the caller, "" when nothing is restricted. Conditions of restrictions are not
// run yet (block 10), so a restricted read admits no row: a hidden row simply
// does not come back, which is exactly what "not found" has to look like, and
// a restriction never shows more than it would.
//
// It returns the fragment rather than a finished statement because these reads
// end in ORDER BY/LIMIT, and the condition has to go before that, not after.
func readRowPredicate(ctx context.Context, objectID uuid.UUID) string {
	if readRestricted(ctx, objectID) {
		return " AND FALSE"
	}
	return ""
}

// requireUnrestrictedRegisterRead refuses to read a register RECORD SET while a
// row restriction is active. This is not a gap waiting to be filled - filtering
// here would be the wrong thing to do.
//
// A record set is a read-modify-write unit: application code reads the movements
// of one recorder, changes them and writes the set back, and the write replaces
// everything the set holds. Handing back a filtered set would therefore delete
// the rows the restriction hid, the moment that set was saved. Losing data is a
// far worse outcome than refusing to read, so the read refuses.
func requireUnrestrictedRegisterRead(ctx context.Context, objectID uuid.UUID, register string) error {
	if !readRestricted(ctx, objectID) {
		return nil
	}
	return fmt.Errorf("register %s has a row-level access restriction, which this read cannot apply; use a query instead", register)
}
