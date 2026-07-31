package state

import (
	"context"
	"fmt"
	"io"

	"github.com/nstehr/canuckpunk/internal/session"
)

// Welcome is the entry state for a new session. A missing session degrades to
// the zero value rather than failing, so the greeting is always safe to write.
func Welcome(ctx context.Context, _ string, out io.Writer) (Fn, error) {
	us, _ := session.FromContext(ctx)

	if _, err := fmt.Fprintf(out, "Welcome, %s.", us.Name()); err != nil {
		return nil, fmt.Errorf("write welcome: %w", err)
	}

	// Nothing follows yet; the next state slots in here.
	return nil, nil
}
