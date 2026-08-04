// Package state drives a session as a chain of states. A state reads a line of
// input and writes to an io.Writer, so it works for any front end.
//
// A Machine holds its current state as a func value, which lives only as long
// as the process. That is free over a connection that stays open and
// impossible over one that does not, so supporting a web or chat front end
// later means naming states and persisting the name. Keep every state
// reconstructible from its name plus the session: capturing a singleton the
// caller can rebuild is fine, capturing anything player-specific is what would
// make the machine unresumable.
package state

import (
	"context"
	"io"
	"log/slog"
)

type (
	// Fn runs one state and returns the next. A nil Fn ends the session.
	Fn func(ctx context.Context, input string, out io.Writer) (Fn, error)

	// Machine is not safe for concurrent use; drive it from one goroutine.
	Machine struct {
		currentState Fn
	}
)

// Hinter is an optional capability of the writer a state is given. A state
// that wants a particular kind of input can say so without Fn growing a return
// value, and a front end that cares can show it — a prompt, a form field, a
// keyboard type. Writers that do not implement it simply miss the hint.
type Hinter interface {
	Hint(kind string)
}

// Hint tells the front end what the state is waiting for. It is per turn: a
// state that says nothing leaves the front end with no hint, rather than the
// last one.
func Hint(out io.Writer, kind string) {
	if h, ok := out.(Hinter); ok {
		h.Hint(kind)
	}
}

// New returns a Machine that begins at start.
func New(start Fn) *Machine {
	return &Machine{currentState: start}
}

// Next runs the current state. A failing state does not advance, so the
// caller can retry it.
func (sm *Machine) Next(ctx context.Context, input string, out io.Writer) error {
	if sm.currentState == nil {
		return nil
	}

	nextState, err := sm.currentState(ctx, input, out)
	if err != nil {
		slog.Error("state failed", "error", err)

		return err
	}

	sm.currentState = nextState

	return nil
}

// Done reports whether the machine has run out of states.
func (sm *Machine) Done() bool {
	return sm.currentState == nil
}
