package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/nstehr/canuckpunk/internal/menu"
	"github.com/nstehr/canuckpunk/internal/onboarding"
	"github.com/nstehr/canuckpunk/internal/session"
	"github.com/nstehr/canuckpunk/internal/state"
	"github.com/nstehr/canuckpunk/internal/user"
	"github.com/nstehr/canuckpunk/narratives"
)

const (
	testUser     = "nstehr"
	testSurveyor = "surveyor"
)

// fakeAccounts stands in for the database.
type fakeAccounts struct {
	users   []user.User
	created []user.NewAccount
	err     error
}

func (f *fakeAccounts) ForCredential(context.Context, string) ([]user.User, error) {
	return f.users, f.err
}

func (f *fakeAccounts) Create(_ context.Context, a user.NewAccount) (user.User, error) {
	f.created = append(f.created, a)
	u := user.User{ID: int64(len(f.users) + 1), Username: a.Username, Email: a.Email}
	f.users = append(f.users, u)

	return u, nil
}

func newTestModel(t *testing.T, width, height int, accounts onboarding.Accounts) model {
	t.Helper()

	us := session.UserSession{
		Username:   testUser,
		Client:     session.ClientSSH,
		RemoteAddr: "10.0.0.1:2222",
		Credential: session.Credential{ID: "SHA256:test"},
	}
	opts := Options{
		Display: Display{Term: "xterm-256color", Width: width, Height: height},
		Choices: onboarding.NewChooser(accounts),
	}

	return newModel(session.NewContext(t.Context(), us), us, state.New(onboarding.Start(accounts, narratives.Embedded())), opts)
}

// boot drains what Init would have run: the entry state and the choice lookup.
func boot(t *testing.T, m model) model {
	t.Helper()

	m = send(t, m, m.runState("")())

	if cmd := m.loadChoices(); cmd != nil {
		m = send(t, m, cmd())
	}

	return m
}

func send(t *testing.T, m model, msgs ...tea.Msg) model {
	t.Helper()

	var mm tea.Model = m
	for _, msg := range msgs {
		mm, _ = mm.Update(msg)
	}

	got, ok := mm.(model)
	if !ok {
		t.Fatalf("Update returned %T, want model", mm)
	}

	return got
}

// step sends a message and runs whatever command it produced, so async state
// output lands before the assertions.
func step(t *testing.T, m model, msg tea.Msg) model {
	t.Helper()

	next, cmd := m.Update(msg)

	got, ok := next.(model)
	if !ok {
		t.Fatalf("Update returned %T, want model", next)
	}

	if cmd == nil {
		return got
	}

	if out := cmd(); out != nil {
		return send(t, got, out)
	}

	return got
}

func typeIn(t *testing.T, m model, s string) model {
	t.Helper()

	msgs := make([]tea.Msg, 0, len(s))
	for _, r := range s {
		msgs = append(msgs, tea.KeyPressMsg{Code: r, Text: string(r)})
	}

	return send(t, m, msgs...)
}

func enter(t *testing.T, m model) model {
	t.Helper()

	return step(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
}

func dims(s string) (int, int) { return lipgloss.Width(s), strings.Count(s, "\n") + 1 }

// Styles are applied to parts of a line, so assertions match against
// unstyled text.
func plain(s string) string {
	var b strings.Builder

	esc := false

	for _, r := range s {
		switch {
		case r == 0x1b:
			esc = true
		case esc && r == 'm':
			esc = false
		case esc:
		default:
			b.WriteRune(r)
		}
	}

	return b.String()
}

func labels(choices menu.Set) []string { return choices.Labels() }

// narrative returns the prose a screen is supposed to show. Tests compare
// against this rather than quoting the words, so rewriting the game's prose
// never fails a test about the flow.
func narrative(t *testing.T, name string) string {
	t.Helper()

	text, err := narratives.Embedded().Read(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}

	return strings.TrimSpace(text)
}

func shown(m model) string { return strings.Join(m.content, "\n") }

func TestUnknownKeyIsOfferedANewUser(t *testing.T) {
	m := boot(t, newTestModel(t, 100, 30, &fakeAccounts{}))

	if got := labels(m.choices); len(got) != 1 || got[0] != onboarding.LabelCreateUser {
		t.Errorf("choices = %v, want [%q]", got, onboarding.LabelCreateUser)
	}

	out := plain(m.View().Content)
	if !strings.Contains(out, onboarding.LabelCreateUser) {
		t.Errorf("context pane missing the choice:\n%s", out)
	}

	if !strings.Contains(shown(m), narrative(t, onboarding.NarrativeWelcome)) {
		t.Error("welcome greeting missing")
	}
}

func TestKnownKeyIsOfferedEveryAccount(t *testing.T) {
	accounts := &fakeAccounts{users: []user.User{
		{ID: 1, Username: testSurveyor},
		{ID: 2, Username: "auditor"},
	}}
	m := boot(t, newTestModel(t, 100, 30, accounts))

	want := []string{
		onboarding.ContinueLabel(testSurveyor),
		onboarding.ContinueLabel("auditor"),
		onboarding.LabelStartNewGame,
	}
	if got := labels(m.choices); !equal(got, want) {
		t.Errorf("choices = %v, want %v", got, want)
	}

	if out := plain(m.View().Content); !strings.Contains(out, "Continue as surveyor") {
		t.Errorf("context pane missing accounts:\n%s", out)
	}
}

// The opening screen must not depend on whether the key is known.
func TestFirstScreenIsTheSameForEveryone(t *testing.T) {
	unknown := boot(t, newTestModel(t, 100, 30, &fakeAccounts{}))
	known := boot(t, newTestModel(t, 100, 30, &fakeAccounts{
		users: []user.User{{ID: 1, Username: testSurveyor}},
	}))

	if !equal(unknown.content, known.content) {
		t.Errorf("greeting differs:\nunknown: %q\nknown:   %q", unknown.content, known.content)
	}

	if !strings.Contains(shown(unknown), narrative(t, onboarding.NarrativeWelcome)) {
		t.Errorf("greeting is not the welcome narrative:\n%s", shown(unknown))
	}

	// Orientation is earned by choosing, not shown on arrival.
	if strings.Contains(shown(unknown), narrative(t, onboarding.NarrativeOrientation)) {
		t.Error("orientation shown before any choice was made")
	}
}

// Typing a label verbatim and selecting it must reach the same state.
func TestTypedLabelAndSelectionAgree(t *testing.T) {
	typed := boot(t, newTestModel(t, 100, 30, &fakeAccounts{}))
	typed = enter(t, typeIn(t, typed, onboarding.LabelCreateUser))

	selected := boot(t, newTestModel(t, 100, 30, &fakeAccounts{}))
	selected = send(t, selected, tea.KeyPressMsg{Code: tea.KeyTab}) // command -> context
	selected = enter(t, selected)

	for name, m := range map[string]model{"typed": typed, "selected": selected} {
		if joined := strings.Join(m.content, "\n"); !strings.Contains(joined, "Open a file") {
			t.Errorf("%s: name prompt missing:\n%s", name, joined)
		}

		if len(m.choices) != 0 {
			t.Errorf("%s: choices still listed after taking one: %v", name, labels(m.choices))
		}
	}
}

// Selecting sends the choice ID, not the label, so a control front end works
// the same way. The log still shows the label.
func TestSelectingSendsTheIDAndLogsTheLabel(t *testing.T) {
	m := boot(t, newTestModel(t, 100, 30, &fakeAccounts{}))
	m = send(t, m, tea.KeyPressMsg{Code: tea.KeyTab}) // command -> context
	m = enter(t, m)

	log := strings.Join(m.transcript, "\n")
	if !strings.Contains(log, onboarding.LabelCreateUser) {
		t.Errorf("activity log does not show the label:\n%s", log)
	}

	if strings.Contains(log, onboarding.IDCreateUser) {
		t.Errorf("activity log leaked the internal id:\n%s", log)
	}

	if joined := strings.Join(m.content, "\n"); !strings.Contains(joined, "Open a file") {
		t.Errorf("selecting by id did not advance the flow:\n%s", joined)
	}
}

// Both paths end at the same orientation narrative.
func TestBothPathsReachOrientation(t *testing.T) {
	newPlayer := boot(t, newTestModel(t, 100, 40, &fakeAccounts{}))
	newPlayer = enter(t, typeIn(t, newPlayer, onboarding.LabelCreateUser))
	newPlayer = enter(t, typeIn(t, newPlayer, testSurveyor))
	newPlayer = enter(t, typeIn(t, newPlayer, onboarding.SkipEmail))

	accounts := &fakeAccounts{users: []user.User{{ID: 1, Username: testSurveyor}}}
	returning := boot(t, newTestModel(t, 100, 40, accounts))
	returning = enter(t, typeIn(t, returning, onboarding.ContinueLabel(testSurveyor)))

	want := narrative(t, onboarding.NarrativeOrientation)
	for name, m := range map[string]model{"new": newPlayer, "returning": returning} {
		if !strings.Contains(shown(m), want) {
			t.Errorf("%s: orientation not shown:\n%s", name, shown(m))
		}
	}
}

func TestCreatingAUserRecordsTheName(t *testing.T) {
	accounts := &fakeAccounts{}
	m := boot(t, newTestModel(t, 100, 40, accounts))
	m = enter(t, typeIn(t, m, onboarding.LabelCreateUser))
	m = enter(t, typeIn(t, m, testSurveyor))
	m = enter(t, typeIn(t, m, onboarding.SkipEmail))

	if len(accounts.created) != 1 || accounts.created[0].Username != testSurveyor {
		t.Errorf("created = %v, want [%s]", accounts.created, testSurveyor)
	}

	if !m.sm.Done() {
		t.Error("onboarding did not finish after the account was created")
	}
}

// The prompt says what the state is waiting for. It has to arrive on the same
// turn the question does, and clear when nothing is being asked.
func TestPromptShowsWhatIsExpected(t *testing.T) {
	m := boot(t, newTestModel(t, 100, 30, &fakeAccounts{}))

	if got := plain(m.input.Prompt); got != commandPrompt {
		t.Errorf("menu prompt = %q, want %q", got, commandPrompt)
	}

	m = enter(t, typeIn(t, m, onboarding.LabelCreateUser))
	if got := plain(m.input.Prompt); got != "("+onboarding.HintName+") "+commandPrompt {
		t.Errorf("prompt = %q, want the name hint — it must not lag a turn", got)
	}

	m = enter(t, typeIn(t, m, testSurveyor))
	if got := plain(m.input.Prompt); got != "("+onboarding.HintEmail+") "+commandPrompt {
		t.Errorf("prompt = %q, want the email hint", got)
	}

	// A rejected answer leaves the player on the same question.
	m = enter(t, typeIn(t, m, "not an address"))
	if got := plain(m.input.Prompt); got != "("+onboarding.HintEmail+") "+commandPrompt {
		t.Errorf("prompt = %q, want the email hint to persist through a retry", got)
	}

	m = enter(t, typeIn(t, m, onboarding.SkipEmail))
	if got := plain(m.input.Prompt); got != commandPrompt {
		t.Errorf("prompt = %q, want it cleared once nothing is being asked", got)
	}
}

// The hint changes the prompt width, so the input has to be resized with it.
func TestPromptWidthTracksTheHint(t *testing.T) {
	m := boot(t, newTestModel(t, 100, 30, &fakeAccounts{}))
	g := m.geometry()

	m = enter(t, typeIn(t, m, onboarding.LabelCreateUser))

	if got, want := m.input.Width(), g.commandInputWidth(m.input.Prompt); got != want {
		t.Errorf("input width = %d, want %d — the line would run past the pane", got, want)
	}
}

func TestOptionalEmailIsRecorded(t *testing.T) {
	accounts := &fakeAccounts{}
	m := boot(t, newTestModel(t, 100, 30, accounts))
	m = enter(t, typeIn(t, m, onboarding.LabelCreateUser))
	m = enter(t, typeIn(t, m, testSurveyor))

	if joined := strings.Join(m.content, "\n"); !strings.Contains(joined, "address") {
		t.Fatalf("no address prompt:\n%s", joined)
	}

	m = enter(t, typeIn(t, m, "Player@Example.CA"))

	if len(accounts.created) != 1 {
		t.Fatalf("created %d accounts, want 1", len(accounts.created))
	}

	if got := accounts.created[0].Email; got != "Player@Example.CA" {
		t.Errorf("email = %q, want it passed through for the service to normalise", got)
	}

	if !strings.Contains(shown(m), narrative(t, onboarding.NarrativeOrientation)) {
		t.Error("did not reach orientation after giving an address")
	}
}

func TestEmailCanBeSkipped(t *testing.T) {
	accounts := &fakeAccounts{}
	m := boot(t, newTestModel(t, 100, 30, accounts))
	m = enter(t, typeIn(t, m, onboarding.LabelCreateUser))
	m = enter(t, typeIn(t, m, testSurveyor))
	m = enter(t, typeIn(t, m, "SKIP"))

	if len(accounts.created) != 1 || accounts.created[0].Email != "" {
		t.Errorf("created = %+v, want an account with no email", accounts.created)
	}

	if !m.sm.Done() {
		t.Error("skipping did not finish onboarding")
	}
}

// A typo would hand this person's characters to whoever owns the address that
// was actually typed, so a bad one must not be recorded.
func TestBadEmailIsRejectedAndRetryable(t *testing.T) {
	accounts := &fakeAccounts{}
	m := boot(t, newTestModel(t, 100, 30, accounts))
	m = enter(t, typeIn(t, m, onboarding.LabelCreateUser))
	m = enter(t, typeIn(t, m, testSurveyor))
	m = enter(t, typeIn(t, m, "not an address"))

	if len(accounts.created) != 0 {
		t.Fatalf("account created with a bad address: %+v", accounts.created)
	}

	if joined := strings.Join(m.content, "\n"); !strings.Contains(joined, "not an address") {
		t.Errorf("no rejection notice:\n%s", joined)
	}

	// Still here, and a good address finishes the job.
	m = enter(t, typeIn(t, m, "player@example.ca"))

	if len(accounts.created) != 1 || accounts.created[0].Email != "player@example.ca" {
		t.Errorf("retry did not take: %+v", accounts.created)
	}
}

func TestUnknownInputIsRejectedWithoutLeavingTheMenu(t *testing.T) {
	m := boot(t, newTestModel(t, 100, 30, &fakeAccounts{}))
	m = enter(t, typeIn(t, m, "wander off"))

	if joined := strings.Join(m.content, "\n"); !strings.Contains(joined, "does not recognise") {
		t.Errorf("no rejection notice:\n%s", joined)
	}

	if m.sm.Done() {
		t.Error("machine left the menu on unrecognised input")
	}
}

// Glamour's stock styles leave "## " attached to headings, which reads as
// unrendered source.
func TestHeadingsRenderWithoutMarkdownMarkers(t *testing.T) {
	src := "# Orientation\n\nLead.\n\n## What is at stake\n\nBody.\n\n### Deeper\n\nMore."

	for _, dark := range []bool{true, false} {
		rendered, _ := newMarkdown(76, dark).render([]string{src})
		got := plain(rendered)
		if strings.Contains(got, "#") {
			t.Errorf("dark=%v: markdown markers still visible:\n%s", dark, got)
		}

		for _, want := range []string{"Orientation", "What is at stake", "Deeper"} {
			if !strings.Contains(got, want) {
				t.Errorf("dark=%v: heading %q missing", dark, want)
			}
		}
	}
}

// A new passage must open at its own top, not at the end of the transcript.
func TestNewPassageOpensAtItsTop(t *testing.T) {
	m := boot(t, newTestModel(t, 100, 24, &fakeAccounts{}))

	if got := m.main.YOffset(); got != 0 {
		t.Errorf("first passage opened at line %d, want the top", got)
	}

	m = enter(t, typeIn(t, m, onboarding.LabelCreateUser))
	m = enter(t, typeIn(t, m, testSurveyor))
	m = enter(t, typeIn(t, m, onboarding.SkipEmail))

	if len(m.offsets) != len(m.content) {
		t.Fatalf("%d offsets for %d passages", len(m.offsets), len(m.content))
	}

	want := m.offsets[len(m.offsets)-1]
	if got := m.main.YOffset(); got != want {
		t.Errorf("viewport at line %d, want %d (top of the newest passage)", got, want)
	}

	// The top of the view must be the first line of the newest passage, not
	// the tail of the one before it.
	newest, _ := m.md.render(m.content[len(m.content)-1:])
	wantFirst := strings.TrimSpace(strings.Split(plain(newest), "\n")[0])
	gotFirst := strings.TrimSpace(strings.Split(plain(m.main.View()), "\n")[0])

	if gotFirst != wantFirst {
		t.Errorf("view starts at %q, want %q", gotFirst, wantFirst)
	}

	if !strings.Contains(plain(m.main.View()), "Orientation") {
		t.Error("orientation is not on screen after the flow completes")
	}
}

func TestSizes(t *testing.T) {
	for _, d := range [][2]int{{80, 24}, {120, 40}, {60, 15}, {200, 60}} {
		m := boot(t, newTestModel(t, d[0], d[1], &fakeAccounts{}))
		m = send(t, m, tea.WindowSizeMsg{Width: d[0], Height: d[1]})

		if w, h := dims(m.View().Content); w != d[0] || h != d[1] {
			t.Errorf("%dx%d -> rendered %dx%d", d[0], d[1], w, h)
		}
	}
}

// A client that does not report a size must still render, rather than
// sitting at 0x0 until the first resize message.
func TestZeroOptionsRenderADefaultTerminal(t *testing.T) {
	us := session.UserSession{Username: testUser}
	m := newModel(session.NewContext(t.Context(), us), us, state.New(onboarding.Start(nil, narratives.Embedded())), Options{})

	if w, h := dims(m.View().Content); w != defaultWidth || h != defaultHeight {
		t.Errorf("rendered %dx%d, want %dx%d", w, h, defaultWidth, defaultHeight)
	}
}

func TestQuitKeys(t *testing.T) {
	m := newTestModel(t, 80, 24, &fakeAccounts{}) // starts focused on Command

	if _, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"}); cmd != nil {
		t.Error("q quit while typing in the command pane")
	}

	m = typeIn(t, m, "q")
	if m.input.Value() != "q" {
		t.Errorf("q not typed into input, value=%q", m.input.Value())
	}

	if _, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}); cmd == nil {
		t.Error("ctrl+c did not quit from the command pane")
	}

	m = send(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	if _, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"}); cmd == nil {
		t.Error("q did not quit once blurred")
	}
}

func TestHelpCommand(t *testing.T) {
	for _, word := range []string{"help", "?", "HELP"} {
		m := boot(t, newTestModel(t, 100, 26, &fakeAccounts{}))
		before := len(m.content)
		m = enter(t, typeIn(t, m, word))

		if len(m.content) != before+1 {
			t.Errorf("%q did not add a help passage", word)
		}

		body := plain(m.main.View())
		if !strings.Contains(body, "Help") {
			t.Errorf("%q did not show help:\n%s", word, body)
		}
	}
}

// Help is generated from the table, so every command must describe itself
// there — otherwise a command exists that nothing tells the player about.
func TestHelpDescribesEveryCommand(t *testing.T) {
	m := boot(t, newTestModel(t, 100, 26, &fakeAccounts{}))
	doc := m.helpDocument()

	for _, c := range m.commands() {
		word, desc := c.help()
		if !strings.Contains(doc, word) {
			t.Errorf("help does not list %q", word)
		}

		if !strings.Contains(doc, desc) {
			t.Errorf("help does not describe %q", word)
		}
	}
}

// The state machine must never see a global command, wherever the flow is.
func TestGlobalCommandsBeatTheStateMachine(t *testing.T) {
	m := boot(t, newTestModel(t, 100, 26, &fakeAccounts{}))
	m = enter(t, typeIn(t, m, onboarding.LabelCreateUser)) // now waiting on a name
	before := m.sm.Done()
	m = enter(t, typeIn(t, m, "help"))

	if m.sm.Done() != before {
		t.Error("help advanced the state machine")
	}

	if joined := strings.Join(m.content, "\n"); strings.Contains(joined, "does not recognise") {
		t.Error("help reached the state machine as ordinary input")
	}
}

// Leaving has to work from anywhere, including from a state that is waiting
// on free text — otherwise a player mid-flow has no way out but ctrl+c.
func TestQuitAndExitCommands(t *testing.T) {
	for _, word := range []string{"quit", "exit", "QUIT", "  Exit  "} {
		m := boot(t, newTestModel(t, 100, 20, &fakeAccounts{}))
		m = typeIn(t, m, word)

		if _, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}); cmd == nil {
			t.Errorf("%q did not quit", word)
		}
	}

	// Mid-flow, where the state is waiting for a username.
	m := boot(t, newTestModel(t, 100, 20, &fakeAccounts{}))
	m = enter(t, typeIn(t, m, onboarding.LabelCreateUser))
	m = typeIn(t, m, "quit")

	if _, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}); cmd == nil {
		t.Error("quit was swallowed by the state waiting for a name")
	}
}

// A word that merely starts with one of them is an ordinary command.
func TestQuitDoesNotMatchLongerWords(t *testing.T) {
	accounts := &fakeAccounts{}
	m := boot(t, newTestModel(t, 100, 20, accounts))
	m = enter(t, typeIn(t, m, onboarding.LabelCreateUser))
	m = enter(t, typeIn(t, m, "quitter"))
	m = enter(t, typeIn(t, m, onboarding.SkipEmail))

	if len(accounts.created) != 1 || accounts.created[0].Username != "quitter" {
		t.Errorf("created = %v, want [quitter] — the name was treated as a quit", accounts.created)
	}

	if !m.sm.Done() {
		t.Error("the flow did not finish, so the name was not accepted")
	}
}

// The words have to be discoverable, not folklore.
func TestLeaveCommandsAppearInHelp(t *testing.T) {
	m := boot(t, newTestModel(t, 120, 20, &fakeAccounts{}))

	out := plain(m.View().Content)
	for _, want := range []string{"quit/exit", "help"} {
		if !strings.Contains(out, want) {
			t.Errorf("header does not mention %q:\n%s", want, firstLines(out, 3))
		}
	}
}

func TestActivityLogsWhatWasTyped(t *testing.T) {
	m := boot(t, newTestModel(t, 100, 30, &fakeAccounts{}))
	for i := range 20 {
		m = enter(t, typeIn(t, m, fmt.Sprintf("cmd-%02d", i)))
	}

	if body := plain(m.activity.View()); !strings.Contains(body, "cmd-19") {
		t.Errorf("activity did not auto-scroll to newest:\n%s", body)
	}

	if !m.activity.AtBottom() {
		t.Error("activity not at bottom")
	}
}

// The Activity pane is narrow and notices can be long; truncation would hide
// the part of an error that says what actually went wrong.
func TestLongNoticesWrapInTheActivityPane(t *testing.T) {
	m := boot(t, newTestModel(t, 100, 20, &fakeAccounts{}))
	m = send(t, m, choicesLoaded{
		err: errors.New("list users by fingerprint: SQL logic error: no such table: users"),
	})

	body := plain(m.activity.View())
	if strings.Count(strings.TrimRight(body, "\n"), "\n") < 1 {
		t.Errorf("notice did not wrap onto more than one line:\n%s", body)
	}

	// Wrapping breaks the phrase across lines, so compare with whitespace
	// collapsed. The tail is the part that names the cause.
	flat := strings.Join(strings.Fields(body), " ")
	if !strings.Contains(flat, "no such table: users") {
		t.Errorf("end of the notice was lost:\n%s", body)
	}

	g := m.geometry()
	for _, line := range strings.Split(body, "\n") {
		if w := lipgloss.Width(line); w > g.bodyWidth(g.leftWidth) {
			t.Errorf("line %q is %d wide, pane body is %d", line, w, g.bodyWidth(g.leftWidth))
		}
	}
}

// A failed lookup must not leave the player staring at an empty sidebar with
// no explanation.
func TestFailedChoiceLookupIsReported(t *testing.T) {
	m := boot(t, newTestModel(t, 100, 20, &fakeAccounts{}))
	m = send(t, m, choicesLoaded{err: errors.New("boom")})

	if !strings.Contains(plain(m.activity.View()), "boom") {
		t.Error("lookup failure was not surfaced to the player")
	}
}

// The indicator has to say both where the reader is and which way there is
// more, or a full-height pane looks like the whole document.
func TestScrollIndicator(t *testing.T) {
	m := boot(t, newTestModel(t, 100, 20, &fakeAccounts{}))

	// The welcome narrative is taller than the pane, and opens at the top.
	m.main.GotoTop()

	top := plain(m.View().Content)
	if !strings.Contains(top, "▼") {
		t.Errorf("no down arrow while there is more below:\n%s", firstLines(top, 6))
	}

	if strings.Contains(top, "▲") {
		t.Error("up arrow shown while at the top")
	}

	if !strings.Contains(top, "0%") {
		t.Error("percentage does not read 0% at the top")
	}

	m.main.GotoBottom()

	bottom := plain(m.View().Content)
	if !strings.Contains(bottom, "▲") || strings.Contains(bottom, "▼") {
		t.Errorf("arrows wrong at the bottom:\n%s", firstLines(bottom, 6))
	}

	if !strings.Contains(bottom, "100%") {
		t.Error("percentage does not read 100% at the bottom")
	}
}

// A pane whose content fits should stay quiet.
func TestNoScrollIndicatorWhenEverythingFits(t *testing.T) {
	m := boot(t, newTestModel(t, 100, 20, &fakeAccounts{}))

	if got := scrollHint(m.activity); got != "" {
		t.Errorf("activity hint = %q, want none while it fits", got)
	}

	// Fill it past its three body lines.
	for i := range 10 {
		m = enter(t, typeIn(t, m, fmt.Sprintf("cmd-%02d", i)))
	}

	if got := scrollHint(m.activity); got == "" {
		t.Error("no activity hint once it overflows")
	}
}

func firstLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[:n]
	}

	return strings.Join(lines, "\n")
}

func TestCursorSitsInTheCommandPane(t *testing.T) {
	m := boot(t, newTestModel(t, 80, 24, &fakeAccounts{}))
	m = typeIn(t, m, "status")

	v := m.View()
	if v.Cursor == nil {
		t.Fatal("no cursor")
	}

	row := []rune(plain(strings.Split(v.Content, "\n")[v.Cursor.Y]))
	if got := string(row[:v.Cursor.X]); !strings.HasSuffix(got, "> status") {
		t.Errorf("cursor not after input: %q", got)
	}

	if c := send(t, m, tea.KeyPressMsg{Code: tea.KeyTab}).View().Cursor; c != nil {
		t.Errorf("cursor shown while command pane blurred: %+v", c)
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}
