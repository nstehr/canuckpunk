package discord

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nstehr/canuckpunk/internal/onboarding"
	"github.com/nstehr/canuckpunk/internal/session"
	"github.com/nstehr/canuckpunk/internal/state"
	"github.com/nstehr/canuckpunk/internal/user"
	"github.com/nstehr/canuckpunk/narratives"
)

const testPlayer = "discord-user-1"

type fakeAccounts struct {
	mu      sync.Mutex
	users   []user.User
	created []user.NewAccount
}

func (f *fakeAccounts) ForCredential(context.Context, string) ([]user.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.users, nil
}

func (f *fakeAccounts) Create(_ context.Context, a user.NewAccount) (user.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.created = append(f.created, a)
	u := user.User{ID: int64(len(f.users) + 1), Username: a.Username, Email: a.Email}
	f.users = append(f.users, u)

	return u, nil
}

// newTestBot builds a Bot with no Discord client. Every part under test is
// reachable without connecting.
func newTestBot(t *testing.T, accounts onboarding.Accounts) *Bot {
	t.Helper()

	prose := narratives.Embedded()

	b := &Bot{
		cfg:     Config{Accounts: accounts, Prose: prose},
		chooser: onboarding.NewChooser(accounts),
	}
	b.convos = newConversations(conversationTTL, func() *state.Machine {
		return state.New(onboarding.Start(accounts, prose))
	})

	return b
}

func playerSession() session.UserSession {
	return session.UserSession{
		ID:         testPlayer,
		Client:     session.ClientDiscord,
		Credential: session.Credential{ID: "discord:" + testPlayer},
	}
}

func narrative(t *testing.T, name string) string {
	t.Helper()

	text, err := narratives.Embedded().Read(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}

	return strings.TrimSpace(text)
}

// turn advances the flow and fails the test if the state machine errors.
func turn(t *testing.T, b *Bot, input string) screen {
	t.Helper()

	c := b.convos.get(testPlayer, playerSession())

	s, err := b.advance(t.Context(), c, input)
	if err != nil {
		t.Fatalf("advance(%q): %v", input, err)
	}

	return s
}

// The opening screen is the welcome narrative plus something to press.
func TestOpeningScreen(t *testing.T) {
	b := newTestBot(t, &fakeAccounts{})

	s := turn(t, b, "")

	if !strings.Contains(s.text, narrative(t, onboarding.NarrativeWelcome)) {
		t.Errorf("opening screen is not the welcome narrative:\n%s", s.text)
	}

	if len(s.choices) != 1 || s.choices[0].Label != onboarding.LabelCreateUser {
		t.Errorf("choices = %v, want a single create-user option", s.choices.Labels())
	}
}

// A known credential is offered its accounts, exactly as over SSH.
func TestReturningPlayerIsOfferedAccounts(t *testing.T) {
	accounts := &fakeAccounts{users: []user.User{{ID: 1, Username: "surveyor"}}}
	b := newTestBot(t, accounts)

	s := turn(t, b, "")

	want := []string{onboarding.ContinueLabel("surveyor"), onboarding.LabelStartNewGame}
	if got := s.choices.Labels(); len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("choices = %v, want %v", got, want)
	}
}

// Clicking sends the choice ID; typing sends the label. Both must land in the
// same place — that is what menu.Resolve is for.
func TestClickAndTypeAgree(t *testing.T) {
	clicked := newTestBot(t, &fakeAccounts{})
	turn(t, clicked, "")
	byID := turn(t, clicked, onboarding.IDCreateUser)

	typed := newTestBot(t, &fakeAccounts{})
	turn(t, typed, "")
	byLabel := turn(t, typed, onboarding.LabelCreateUser)

	if byID.text != byLabel.text {
		t.Errorf("click and type diverged:\n id: %q\n label: %q", byID.text, byLabel.text)
	}

	if byID.hint != onboarding.HintName {
		t.Errorf("hint = %q, want %q", byID.hint, onboarding.HintName)
	}
}

// The full flow, ending on the same orientation the terminal client reaches.
func TestFullOnboarding(t *testing.T) {
	accounts := &fakeAccounts{}
	b := newTestBot(t, accounts)

	turn(t, b, "")
	turn(t, b, onboarding.IDCreateUser)

	email := turn(t, b, "beaver")
	if email.hint != onboarding.HintEmail {
		t.Errorf("hint = %q, want %q", email.hint, onboarding.HintEmail)
	}

	done := turn(t, b, "player@example.ca")

	if !strings.Contains(done.text, narrative(t, onboarding.NarrativeOrientation)) {
		t.Errorf("did not reach orientation:\n%s", done.text)
	}

	if !done.done {
		t.Error("flow did not finish")
	}

	if len(accounts.created) != 1 || accounts.created[0].Username != "beaver" {
		t.Fatalf("created = %+v", accounts.created)
	}

	if got := accounts.created[0].Email; got != "player@example.ca" {
		t.Errorf("email = %q", got)
	}
}

// Choices belong to the opening screen only; a stale row under a later
// message would offer options the flow has moved past.
func TestChoicesOnlyOnTheOpeningScreen(t *testing.T) {
	b := newTestBot(t, &fakeAccounts{})

	if s := turn(t, b, ""); len(s.choices) == 0 {
		t.Fatal("opening screen has no choices")
	}

	if s := turn(t, b, onboarding.IDCreateUser); len(s.choices) != 0 {
		t.Errorf("later screen still offers %v", s.choices.Labels())
	}
}

// help and quit work wherever the player is, without the machine seeing them.
func TestGlobalCommands(t *testing.T) {
	b := newTestBot(t, &fakeAccounts{})
	turn(t, b, "")
	turn(t, b, onboarding.IDCreateUser) // now waiting on a name

	for _, word := range []string{"help", "HELP"} {
		s, ok := b.global(testPlayer, word)
		if !ok || !strings.Contains(s.text, "Help") {
			t.Errorf("%q did not produce help", word)
		}
	}

	if b.convos.len() != 1 {
		t.Fatalf("expected a live conversation, got %d", b.convos.len())
	}

	s, ok := b.global(testPlayer, " Quit ")
	if !ok || !s.done {
		t.Error("quit did not end the conversation")
	}

	if b.convos.len() != 0 {
		t.Error("quit left the conversation in place")
	}

	if _, ok := b.global(testPlayer, "beaver"); ok {
		t.Error("an ordinary line was treated as a global command")
	}
}

// Typing a label leaves the buttons on screen. Clicking one afterwards must
// not be fed to a state that is asking for something else — otherwise the
// choice id becomes the username.
func TestStaleButtonAfterTypingIsIgnored(t *testing.T) {
	accounts := &fakeAccounts{}
	b := newTestBot(t, accounts)

	turn(t, b, "")                         // opening screen; buttons shown
	turn(t, b, onboarding.LabelCreateUser) // typed, so the row stays live on screen

	// The player now presses the button that is still sitting above.
	c := b.convos.get(testPlayer, playerSession())

	stale, err := b.click(t.Context(), c, onboarding.IDCreateUser)
	if err != nil {
		t.Fatalf("click: %v", err)
	}

	// It should have re-asked the current question, not consumed the id.
	if stale.hint != onboarding.HintName {
		t.Errorf("hint = %q, want the name prompt to be repeated", stale.hint)
	}

	turn(t, b, "beaver")
	turn(t, b, onboarding.SkipEmail)

	if len(accounts.created) != 1 {
		t.Fatalf("created %d accounts: %+v", len(accounts.created), accounts.created)
	}

	if got := accounts.created[0].Username; got != "beaver" {
		t.Errorf("username = %q, want beaver — a stale button was taken as input", got)
	}
}

// A live button still works, which is what the guard must not break.
func TestLiveButtonIsHonoured(t *testing.T) {
	b := newTestBot(t, &fakeAccounts{})
	turn(t, b, "")

	c := b.convos.get(testPlayer, playerSession())

	s, err := b.click(t.Context(), c, onboarding.IDCreateUser)
	if err != nil {
		t.Fatalf("click: %v", err)
	}

	if s.hint != onboarding.HintName {
		t.Errorf("hint = %q, want the flow to have advanced to the name", s.hint)
	}
}

// A button clicked long after its conversation expired must restart the flow
// rather than fail: Discord messages outlive sessions.
func TestStaleInteractionRestarts(t *testing.T) {
	b := newTestBot(t, &fakeAccounts{})
	turn(t, b, "")
	b.convos.drop(testPlayer)

	s := turn(t, b, onboarding.IDCreateUser)

	if s.text == "" {
		t.Fatal("stale interaction produced nothing")
	}

	if b.convos.len() != 1 {
		t.Error("a fresh conversation was not started")
	}
}

func TestConversationTTL(t *testing.T) {
	now := time.Now()
	cs := newConversations(time.Hour, func() *state.Machine { return state.New(nil) })
	cs.now = func() time.Time { return now }

	cs.get("a", playerSession())
	cs.get("b", playerSession())

	if cs.len() != 2 {
		t.Fatalf("len = %d, want 2", cs.len())
	}

	// Move past the TTL and touch one of them: the other is swept.
	now = now.Add(2 * time.Hour)
	cs.get("a", playerSession())

	if cs.len() != 1 {
		t.Errorf("len = %d, want 1 — expired conversations were kept", cs.len())
	}
}

// Discord delivers interactions concurrently; the machine is single-goroutine.
func TestConcurrentTurnsAreSerialised(t *testing.T) {
	b := newTestBot(t, &fakeAccounts{})
	c := b.convos.get(testPlayer, playerSession())

	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			if _, err := b.advance(context.Background(), c, ""); err != nil {
				t.Errorf("advance: %v", err)
			}
		}()
	}

	wg.Wait()
}
