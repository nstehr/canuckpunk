package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

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

func typeIn(t *testing.T, m model, s string) model {
	t.Helper()

	msgs := make([]tea.Msg, 0, len(s))
	for _, r := range s {
		msgs = append(msgs, tea.KeyPressMsg{Code: r, Text: string(r)})
	}

	return send(t, m, msgs...)
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

func TestLayoutWiring(t *testing.T) {
	m := newModel("xterm-256color", 80, 24)
	m = send(t, m, tea.BackgroundColorMsg{}) // light

	// Run a few commands so Activity has content.
	for _, c := range []string{"boot reactor", "scan sector", "vent plasma"} {
		m = typeIn(t, m, c)
		m = send(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	}

	// Tab to Context, move down twice -> "Crew".
	m = send(t, m, tea.KeyPressMsg{Code: tea.KeyTab}) // command -> context
	m = send(t, m, tea.KeyPressMsg{Code: tea.KeyDown}, tea.KeyPressMsg{Code: tea.KeyDown})

	out := m.View().Content
	w, h := dims(out)
	if w != 80 || h != 24 {
		t.Errorf("bad size %dx%d", w, h)
	}
	if m.mainTitle() != "Crew" {
		t.Errorf("mainTitle = %q, want Crew", m.mainTitle())
	}
	if !strings.Contains(out, "(crew)") {
		t.Error("main view did not follow list selection")
	}
	if !strings.Contains(out, "select") || !strings.Contains(out, "next pane") {
		t.Error("contextual help missing for Context focus")
	}
	for _, c := range []string{"boot reactor", "scan sector", "vent plasma"} {
		if !strings.Contains(out, c) {
			t.Errorf("activity missing %q", c)
		}
	}
}

func TestSelectLogSection(t *testing.T) {
	m := newModel("xterm-256color", 80, 24)
	m = typeIn(t, m, "hello")
	m = send(t, m, tea.KeyPressMsg{Code: tea.KeyEnter}, tea.KeyPressMsg{Code: tea.KeyTab})
	// Down to the last item, "Log".
	for range 4 {
		m = send(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if m.mainTitle() != "Log" {
		t.Fatalf("mainTitle = %q", m.mainTitle())
	}
	if !strings.Contains(plain(m.View().Content), "1 hello") {
		t.Error("Log section did not render the command history")
	}
}

func TestSizes(t *testing.T) {
	for _, d := range [][2]int{{80, 24}, {120, 40}, {60, 15}, {200, 60}} {
		m := newModel("xterm", d[0], d[1])
		m = send(t, m, tea.WindowSizeMsg{Width: d[0], Height: d[1]})
		w, h := dims(m.View().Content)
		if w != d[0] || h != d[1] {
			t.Errorf("%dx%d -> rendered %dx%d", d[0], d[1], w, h)
		}
	}
}

func TestQuitKeys(t *testing.T) {
	m := newModel("xterm", 80, 24) // starts focused on Command
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

func TestActivityAutoScroll(t *testing.T) {
	m := newModel("xterm", 80, 24)
	for i := range 20 {
		m = typeIn(t, m, fmt.Sprintf("cmd-%02d", i))
		m = send(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	}
	body := m.activity.View()
	if !strings.Contains(body, "cmd-19") {
		t.Errorf("activity did not auto-scroll to newest:\n%s", body)
	}
	if strings.Contains(body, "cmd-00") {
		t.Error("activity still showing the oldest line")
	}
	if !m.activity.AtBottom() {
		t.Error("activity not at bottom")
	}
}

func TestMainViewportScrolls(t *testing.T) {
	m := newModel("xterm", 80, 24)
	for i := range 40 {
		m = typeIn(t, m, fmt.Sprintf("line-%02d", i))
		m = send(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	}
	// Focus Context, pick "Log" (40 lines), then focus Main and scroll.
	m = send(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	for range 4 {
		m = send(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	m = send(t, m, tea.KeyPressMsg{Code: tea.KeyTab}) // -> Main
	top := m.main.View()
	m = send(t, m, tea.KeyPressMsg{Code: tea.KeyPgDown})
	if m.main.View() == top {
		t.Error("main viewport did not scroll")
	}
	if !strings.Contains(m.View().Content, "scroll") {
		t.Error("help did not switch to the scroll hint for Main")
	}
}

func TestOverviewShowsTerminalInfo(t *testing.T) {
	m := newModel("xterm-256color", 80, 24)
	out := m.View().Content
	if !strings.Contains(out, "xterm-256color") || !strings.Contains(out, "80×24") {
		t.Errorf("overview missing terminal info:\n%s", out)
	}
}

func TestTooSmall(t *testing.T) {
	m := newModel("xterm", 10, 5)
	if got := m.View().Content; !strings.Contains(got, "too small") {
		t.Errorf("got %q", got)
	}
	if c := m.View().Cursor; c != nil {
		t.Error("cursor placed while too small")
	}
}
