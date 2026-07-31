package state_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/nstehr/canuckpunk/internal/session"
	"github.com/nstehr/canuckpunk/internal/state"
)

func TestMachineAdvancesThroughStates(t *testing.T) {
	var second state.Fn

	first := func(_ context.Context, input string, out io.Writer) (state.Fn, error) {
		_, _ = io.WriteString(out, "first:"+input)

		return second, nil
	}
	second = func(_ context.Context, input string, out io.Writer) (state.Fn, error) {
		_, _ = io.WriteString(out, "second:"+input)

		return nil, nil
	}

	sm := state.New(first)

	var buf bytes.Buffer
	if err := sm.Next(t.Context(), "a", &buf); err != nil {
		t.Fatalf("first: %v", err)
	}
	if sm.Done() {
		t.Error("machine finished after the first state")
	}

	if err := sm.Next(t.Context(), "b", &buf); err != nil {
		t.Fatalf("second: %v", err)
	}
	if !sm.Done() {
		t.Error("machine did not finish after a nil next state")
	}

	if got := buf.String(); got != "first:asecond:b" {
		t.Errorf("output = %q", got)
	}
}

// A finished machine has to stay callable, since input keeps arriving.
func TestNextOnFinishedMachineIsNoop(t *testing.T) {
	sm := state.New(nil)

	var buf bytes.Buffer
	if err := sm.Next(t.Context(), "anything", &buf); err != nil {
		t.Fatalf("Next: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("wrote %q", buf.String())
	}
}

// A failing state must not advance, so the caller can retry it.
func TestFailingStateDoesNotAdvance(t *testing.T) {
	errBoom := errors.New("boom")
	calls := 0

	boom := func(_ context.Context, _ string, _ io.Writer) (state.Fn, error) {
		calls++

		return nil, errBoom
	}

	sm := state.New(boom)

	var buf bytes.Buffer
	for range 2 {
		if err := sm.Next(t.Context(), "", &buf); !errors.Is(err, errBoom) {
			t.Fatalf("err = %v, want errBoom", err)
		}
	}

	if calls != 2 {
		t.Errorf("state ran %d times, want 2", calls)
	}
	if sm.Done() {
		t.Error("machine advanced past a failing state")
	}
}

func TestWelcomeGreetsTheUser(t *testing.T) {
	us := session.UserSession{Username: "nstehr"}
	ctx := session.NewContext(t.Context(), us)
	sm := state.New(state.Welcome)

	var buf bytes.Buffer
	if err := sm.Next(ctx, "", &buf); err != nil {
		t.Fatalf("Next: %v", err)
	}

	if got := buf.String(); !strings.Contains(got, "nstehr") {
		t.Errorf("welcome = %q, want it to name the user", got)
	}
}

// An anonymous client still has to be greeted with something.
func TestWelcomeWithoutUsername(t *testing.T) {
	ctx := session.NewContext(t.Context(), session.UserSession{})
	sm := state.New(state.Welcome)

	var buf bytes.Buffer
	if err := sm.Next(ctx, "", &buf); err != nil {
		t.Fatalf("Next: %v", err)
	}

	if got := buf.String(); !strings.Contains(got, "stranger") {
		t.Errorf("welcome = %q", got)
	}
}

// A context with no session at all must degrade, not panic.
func TestWelcomeWithoutSessionInContext(t *testing.T) {
	sm := state.New(state.Welcome)

	var buf bytes.Buffer
	if err := sm.Next(t.Context(), "", &buf); err != nil {
		t.Fatalf("Next: %v", err)
	}

	if got := buf.String(); !strings.Contains(got, "stranger") {
		t.Errorf("welcome = %q", got)
	}
}
