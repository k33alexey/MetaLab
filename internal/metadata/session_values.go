package metadata

import (
	"context"
	"strings"

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

type sessionValuesContextKey struct{}

// WithSessionValues attaches resolved session parameter values to ctx. Values
// are stored as slices from the start: a rule may compare a field against one
// value or against a set, and a parameter holding a collection is the ordinary
// case rather than the exception.
//
// Values given here answer before the resolver is asked, so a caller that
// already knows a value - a test, a request that has computed it - does not
// make a restriction run the session module for it again.
func WithSessionValues(ctx context.Context, values map[string][]Value) context.Context {
	folded := make(map[string][]Value, len(values))
	for name, value := range values {
		folded[strings.ToLower(name)] = value
	}
	return context.WithValue(ctx, sessionValuesContextKey{}, folded)
}

type sessionResolverContextKey struct{}

// SessionValueResolver supplies the values the PROJECT computes in its session
// module. Resolving one runs application code -
// the session module - so it is a function called only if a restriction actually
// names such a parameter, never work done up front for every read.
type SessionValueResolver func(ctx context.Context, name string) ([]Value, bool, error)

// WithSessionResolver attaches the project's own supplier of session parameter
// values. Every session parameter is the configuration's, so this is where the
// value of any of them comes from unless WithSessionValues already gave it.
func WithSessionResolver(ctx context.Context, resolver SessionValueResolver) context.Context {
	return context.WithValue(ctx, sessionResolverContextKey{}, resolver)
}

// sessionValue returns the resolved value of one parameter. A missing parameter
// is reported as absent rather than as an empty set, so a restriction that
// cannot be evaluated refuses the read instead of silently matching nothing or
// everything.
func sessionValue(ctx context.Context, name string) ([]Value, bool, error) {
	if values, ok := ctx.Value(sessionValuesContextKey{}).(map[string][]Value); ok {
		if value, ok := values[strings.ToLower(name)]; ok {
			return value, true, nil
		}
	}
	resolver, ok := ctx.Value(sessionResolverContextKey{}).(SessionValueResolver)
	if !ok || resolver == nil {
		return nil, false, nil
	}
	return resolver(ctx, name)
}
