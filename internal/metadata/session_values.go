package metadata

import (
	"context"
	"strings"
)

// CurrentUserParameter is the one session parameter the platform resolves by
// itself, from the authenticated session rather than from application code.
//
// It carries the ML platform user identifier, not a reference to an application
// "Users" catalog: the platform has no opinion about whether such a catalog
// exists. An applied solution that wants to restrict rows by their owner must
// therefore store that identifier in its own data.
const CurrentUserParameter = "ТекущийПользователь"
const currentUserParameterEN = "CurrentUser"

// ReservedSessionParameter reports whether a name belongs to the platform and
// may not be declared by a project. Reserving the name is what lets a read path
// resolve it without building a BSL runtime.
func ReservedSessionParameter(name string) bool {
	return strings.EqualFold(name, CurrentUserParameter) || strings.EqualFold(name, currentUserParameterEN)
}

type sessionValuesContextKey struct{}

// WithSessionValues attaches resolved session parameter values to ctx. Values
// are stored as slices from the start: a rule may compare a field against one
// value or against a set, and a parameter holding a collection is the ordinary
// case rather than the exception.
//
// The hosting layer resolves these once per request, the same way it resolves
// permissions - a read path must never need a BSL runtime just to evaluate a
// row restriction.
func WithSessionValues(ctx context.Context, values map[string][]Value) context.Context {
	folded := make(map[string][]Value, len(values))
	for name, value := range values {
		folded[strings.ToLower(name)] = value
	}
	return context.WithValue(ctx, sessionValuesContextKey{}, folded)
}

// sessionValue returns the resolved value of one parameter. A missing parameter
// is reported as absent rather than as an empty set, so a restriction that
// cannot be evaluated refuses the read instead of silently matching nothing or
// everything.
func sessionValue(ctx context.Context, name string) ([]Value, bool) {
	values, ok := ctx.Value(sessionValuesContextKey{}).(map[string][]Value)
	if !ok {
		return nil, false
	}
	value, ok := values[strings.ToLower(name)]
	return value, ok
}
