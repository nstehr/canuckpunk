package onboarding_test

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/nstehr/canuckpunk/internal/onboarding"
	"github.com/nstehr/canuckpunk/internal/state"
	"github.com/nstehr/canuckpunk/internal/user"
	"github.com/nstehr/canuckpunk/narratives"
)

// recordingProse notes every name the flow asks for.
type recordingProse struct {
	inner *narratives.Store
	read  []string
}

func (r *recordingProse) Read(name string) (string, error) {
	r.read = append(r.read, name)

	return r.inner.Read(name)
}

// RequiredNarratives is hand-maintained, so drive the whole flow and check
// nothing was read that the list does not mention. Otherwise a new state's
// prose would pass the startup check and fail in front of a player.
func TestEveryNarrativeReadIsDeclared(t *testing.T) {
	prose := &recordingProse{inner: narratives.Embedded()}
	accounts := fakeAccounts{}
	sm := state.New(onboarding.Start(accounts, prose))

	ctx := ctxWithKey(t)

	var buf bytes.Buffer
	for _, input := range []string{"", onboarding.LabelCreateUser, testSurveyor} {
		if err := sm.Next(ctx, input, &buf); err != nil {
			t.Fatalf("Next(%q): %v", input, err)
		}
	}

	// Walk the other branch too: continuing an existing account.
	returning := state.New(onboarding.Start(
		fakeAccounts{users: []user.User{{ID: 1, Username: testSurveyor}}}, prose,
	))
	if err := returning.Next(ctx, "", &buf); err != nil {
		t.Fatalf("returning greeting: %v", err)
	}

	if err := returning.Next(ctx, onboarding.ContinueLabel(testSurveyor), &buf); err != nil {
		t.Fatalf("continue: %v", err)
	}

	declared := onboarding.RequiredNarratives()
	for _, name := range prose.read {
		if !slices.Contains(declared, name) {
			t.Errorf("flow read %q, which RequiredNarratives does not list", name)
		}
	}

	// And the reverse: a name nobody reads is dead weight in the check.
	for _, name := range declared {
		if !slices.Contains(prose.read, name) {
			t.Errorf("RequiredNarratives lists %q, which the flow never reads", name)
		}
	}
}

// The startup check has to actually fail when prose is missing.
func TestRequiredNarrativesAreAllPresent(t *testing.T) {
	if err := narratives.Embedded().Check(onboarding.RequiredNarratives()...); err != nil {
		t.Errorf("embedded prose is incomplete: %v", err)
	}
}

func TestCheckNamesEveryMissingFile(t *testing.T) {
	err := narratives.Embedded().Check("onboarding/nope.md", "onboarding/welcome.md", "gone.md")
	if err == nil {
		t.Fatal("Check passed with missing narratives")
	}

	for _, want := range []string{"onboarding/nope.md", "gone.md"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not name %q: %v", want, err)
		}
	}

	if strings.Contains(err.Error(), "onboarding/welcome.md") {
		t.Errorf("error names a file that exists: %v", err)
	}
}
