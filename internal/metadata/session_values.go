package metadata

import (
	"context"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

type mlUserContextKey struct{}

// WithMLUser attaches the ML platform user of the session. It travels apart from
// session parameters on purpose: session parameters belong to the configuration,
// which may declare any name, ТекущийПользователь included, and fills them in its
// own session module. The ML user is the platform's layer and never enters the
// configuration's namespace, so nothing the configuration declares can shadow it
// or be shadowed by it.
func WithMLUser(ctx context.Context, user uuid.UUID) context.Context {
	return context.WithValue(ctx, mlUserContextKey{}, user)
}

// MLUserFromContext returns the ML platform user attached by WithMLUser. A
// context without one reports false, and a restriction comparing against the
// ML user then refuses the read rather than matching nothing or everything.
func MLUserFromContext(ctx context.Context) (uuid.UUID, bool) {
	user, ok := ctx.Value(mlUserContextKey{}).(uuid.UUID)
	return user, ok && !user.IsZero()
}

type sessionResolverContextKey struct{}

// SessionValueResolver supplies the values the PROJECT computes in its session
// module. Resolving one runs application code -
// the session module - so it is a function called only if a restriction actually
// names such a parameter, never work done up front for every read.
type SessionValueResolver func(ctx context.Context, name string) ([]Value, bool, error)

// WithSessionResolver attaches the project's own supplier of session parameter
// values. Every session parameter is the configuration's, so this is where the
// value of any of them comes from. Nothing reads it yet: a restriction does
// not run until its text is executed (BLOCKS, block 10, «исполнение текста
// ограничения»), and the reader is written there.
func WithSessionResolver(ctx context.Context, resolver SessionValueResolver) context.Context {
	return context.WithValue(ctx, sessionResolverContextKey{}, resolver)
}
