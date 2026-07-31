package menu_test

import (
	"testing"

	"github.com/nstehr/canuckpunk/internal/menu"
)

const continueLabel = "Continue as surveyor"

func testSet() menu.Set {
	return menu.Set{
		{ID: "continue:7", Label: continueLabel, Action: "continue", Value: "7"},
		{ID: "start-new-game", Label: "Start New Game", Action: "new-user"},
	}
}

// Labels are meant to be retyped by hand, so matching forgives case and space.
func TestMatchIsForgiving(t *testing.T) {
	for _, input := range []string{
		continueLabel,
		"continue as surveyor",
		"  CONTINUE AS SURVEYOR  ",
	} {
		got, ok := testSet().Match(input)
		if !ok || got.Value != "7" {
			t.Errorf("Match(%q) = %+v, %v; want the continue choice", input, got, ok)
		}
	}
}

// A partial or unknown label must not select anything, or a stray word would
// take an action the player did not choose.
func TestMatchRejectsAnythingElse(t *testing.T) {
	for _, input := range []string{"", "   ", "continue", "wander off", "continue as someone"} {
		if got, ok := testSet().Match(input); ok {
			t.Errorf("Match(%q) = %+v, want no match", input, got)
		}
	}
}

// A control sends the ID back untouched, so it must select exactly.
func TestByID(t *testing.T) {
	got, ok := testSet().ByID("continue:7")
	if !ok || got.Value != "7" {
		t.Errorf("ByID = %+v, %v; want the continue choice", got, ok)
	}

	for _, id := range []string{"", "Continue:7", "continue:8", continueLabel} {
		if _, ok := testSet().ByID(id); ok {
			t.Errorf("ByID(%q) matched, want no match", id)
		}
	}
}

// Resolve is what states call, so both front-end styles reach one choice.
func TestResolveAcceptsIDsAndLabels(t *testing.T) {
	for _, input := range []string{"continue:7", continueLabel, "  continue as surveyor  "} {
		got, ok := testSet().Resolve(input)
		if !ok || got.Value != "7" {
			t.Errorf("Resolve(%q) = %+v, %v; want the continue choice", input, got, ok)
		}
	}

	if _, ok := testSet().Resolve("continue:99"); ok {
		t.Error("Resolve matched an unknown id")
	}
}

func TestMatchOnEmptySet(t *testing.T) {
	if _, ok := (menu.Set{}).Match("anything"); ok {
		t.Error("matched against an empty set")
	}
}

func TestLabels(t *testing.T) {
	got := testSet().Labels()
	want := []string{continueLabel, "Start New Game"}

	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("label %d = %q, want %q", i, got[i], want[i])
		}
	}
}
