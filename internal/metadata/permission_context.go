package metadata

import (
	"context"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

type permissionsContextKey struct{}

// WithPermissions attaches an enforced application permission policy to ctx.
// Its absence (not merely a restrictive policy) means unrestricted execution:
// ML Studio debugging, the CLI and plain automated tests keep working exactly
// as before. Only a context that explicitly carries a compiled policy is
// checked against object, field and command grants.
func WithPermissions(ctx context.Context, permissions *Permissions) context.Context {
	return context.WithValue(ctx, permissionsContextKey{}, permissions)
}

// PermissionsFromContext returns the policy attached by WithPermissions, if any.
func PermissionsFromContext(ctx context.Context) (*Permissions, bool) {
	if ctx == nil {
		return nil, false
	}
	permissions, ok := ctx.Value(permissionsContextKey{}).(*Permissions)
	return permissions, ok
}

// requireObject enforces an object-level operation only when ctx carries an
// enforced policy. A context without one (Studio, CLI, role-free tests) skips
// the check entirely, preserving unrestricted execution.
func requireObject(ctx context.Context, id uuid.UUID, operation PermissionOperation) error {
	if permissions, ok := PermissionsFromContext(ctx); ok {
		return permissions.RequireObject(id, operation)
	}
	return nil
}

// requireFields enforces a field-level operation only when ctx carries an
// enforced policy.
func requireFields(ctx context.Context, id uuid.UUID, operation PermissionOperation, fields ...string) error {
	if permissions, ok := PermissionsFromContext(ctx); ok {
		return permissions.RequireFields(id, operation, fields...)
	}
	return nil
}

// requireCommand enforces a form command grant only when ctx carries an
// enforced policy.
func requireCommand(ctx context.Context, form, command uuid.UUID) error {
	if permissions, ok := PermissionsFromContext(ctx); ok {
		return permissions.RequireCommand(form, command)
	}
	return nil
}

// requireTablePartWrites gates each non-empty table part written by a catalog
// or document Write() as a whole, since BSL edits table part rows through the
// generic ТаблицаЗначений collection API rather than through this runtime, so
// per-cell enforcement during editing is not reachable from here.
func requireTablePartWrites(ctx context.Context, id uuid.UUID, operation PermissionOperation, tableParts []TablePart, rows map[uuid.UUID][]ObjectRow) error {
	for _, part := range tableParts {
		if len(rows[part.ID]) == 0 {
			continue
		}
		if err := requireFields(ctx, id, operation, part.ID.String()); err != nil {
			return err
		}
	}
	return nil
}
