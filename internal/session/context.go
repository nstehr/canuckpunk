package session

import "context"

// Unexported so no other package can collide with or overwrite the value.
type contextKey struct{}

// NewContext returns ctx carrying us. A session is per-connection and
// immutable, which is what makes it safe to carry here rather than threading
// it through every call.
func NewContext(ctx context.Context, us UserSession) context.Context {
	return context.WithValue(ctx, contextKey{}, us)
}

// FromContext returns the session carried by ctx. The zero UserSession is
// usable, so callers that do not care whether one was set can ignore ok.
func FromContext(ctx context.Context) (UserSession, bool) {
	us, ok := ctx.Value(contextKey{}).(UserSession)

	return us, ok
}
