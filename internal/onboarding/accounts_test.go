package onboarding_test

import (
	"context"
	"testing"

	"github.com/nstehr/canuckpunk/internal/onboarding"
	"github.com/nstehr/canuckpunk/internal/session"
	"github.com/nstehr/canuckpunk/internal/user"
)

const testSurveyor = "surveyor"

type fakeAccounts struct {
	users []user.User
}

func (f fakeAccounts) ForCredential(context.Context, string) ([]user.User, error) {
	return f.users, nil
}

func (f fakeAccounts) Create(_ context.Context, username, _, _ string) (user.User, error) {
	return user.User{ID: 1, Username: username}, nil
}

func ctxWithKey(t *testing.T) context.Context {
	t.Helper()

	return session.NewContext(t.Context(), session.UserSession{
		Credential: session.Credential{ID: "SHA256:test"},
	})
}

func TestUnknownCredentialOffersOnlyCreate(t *testing.T) {
	got, err := onboarding.NewChooser(fakeAccounts{}).Choices(ctxWithKey(t))
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	if len(got) != 1 || got[0].Label != onboarding.LabelCreateUser || got[0].Action != onboarding.ActionNewUser {
		t.Errorf("got %+v, want a single create-user choice", got)
	}

	if got[0].ID != onboarding.IDCreateUser {
		t.Errorf("ID = %q, want %q", got[0].ID, onboarding.IDCreateUser)
	}
}

func TestKnownCredentialOffersEveryAccountPlusANewGame(t *testing.T) {
	accounts := fakeAccounts{users: []user.User{
		{ID: 7, Username: testSurveyor},
		{ID: 9, Username: "auditor"},
	}}

	got, err := onboarding.NewChooser(accounts).Choices(ctxWithKey(t))
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	if len(got) != 3 {
		t.Fatalf("got %d choices, want 3", len(got))
	}

	if got[0].Value != "7" || got[0].Action != onboarding.ActionContinue {
		t.Errorf("first choice = %+v, want continue as user 7", got[0])
	}

	if got[0].ID != onboarding.ContinueID(7) {
		t.Errorf("first choice ID = %q, want %q", got[0].ID, onboarding.ContinueID(7))
	}

	// Every choice needs a selector, or a control front end cannot send one.
	for i, c := range got {
		if c.ID == "" {
			t.Errorf("choice %d (%q) has no ID", i, c.Label)
		}
	}

	if got[2].Label != onboarding.LabelStartNewGame || got[2].Action != onboarding.ActionNewUser {
		t.Errorf("last choice = %+v, want %q", got[2], onboarding.LabelStartNewGame)
	}
}

// A session with no credential at all is the new-player path, not a failure.
func TestMissingCredentialIsTreatedAsNew(t *testing.T) {
	got, err := onboarding.NewChooser(fakeAccounts{users: []user.User{{ID: 1, Username: testSurveyor}}}).
		Choices(t.Context())
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	if len(got) != 1 || got[0].Action != onboarding.ActionNewUser {
		t.Errorf("got %+v, want the new-user path", got)
	}
}
