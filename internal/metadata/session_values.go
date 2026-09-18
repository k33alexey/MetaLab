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
// permissions. What goes here is what the PLATFORM knows by itself, so an
// ordinary read never builds a BSL runtime; values the project computes come
// from WithSessionResolver instead, and only when a restriction asks.
func WithSessionValues(ctx context.Context, values map[string][]Value) context.Context {
	folded := make(map[string][]Value, len(values))
	for name, value := range values {
		folded[strings.ToLower(name)] = value
	}
	return context.WithValue(ctx, sessionValuesContextKey{}, folded)
}

type sessionResolverContextKey struct{}

// SessionValueResolver supplies the values the PROJECT computes, as opposed to
// the ones the platform knows by itself. Resolving one runs application code -
// the session module - so it is a function called only if a restriction actually
// names such a parameter, never work done up front for every read.
type SessionValueResolver func(ctx context.Context, name string) ([]Value, bool, error)

// WithSessionResolver attaches the project's own supplier of session parameter
// values. Platform-owned names are answered from WithSessionValues and never
// reach the resolver: the platform's answer for ТекущийПользователь must not
// depend on application code.
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
