package tui

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// command is a word a player can type at any time. Global commands are
// resolved before the state machine sees the line, so a state waiting on free
// text cannot swallow one.
type command struct {
	binding key.Binding
	run     func(m *model) tea.Cmd
}

// words are the spellings that reach this command, taken from the binding so
// the accepted words and the help text cannot drift.
func (c command) words() []string { return c.binding.Keys() }

func (c command) help() (word, desc string) {
	h := c.binding.Help()

	return h.Key, h.Desc
}

// commands is the whole global vocabulary. Everything else is passed to the
// state machine, which decides what it means where the player is standing.
func (m model) commands() []command {
	return []command{
		{binding: m.keys.Help, run: (*model).showHelp},
		{binding: m.keys.Leave, run: func(*model) tea.Cmd { return tea.Quit }},
	}
}

// lookup matches a typed line the way a player would type it.
func (m model) lookup(input string) (command, bool) {
	want := strings.ToLower(strings.TrimSpace(input))

	for _, c := range m.commands() {
		if slices.Contains(c.words(), want) {
			return c, true
		}
	}

	return command{}, false
}

// showHelp appends the help to the transcript rather than taking over the
// screen, so it reads in place and the narrative is still there behind it.
func (m *model) showHelp() tea.Cmd {
	m.content = append(m.content, m.helpDocument())
	m.refresh()
	m.showLatestPassage()

	return nil
}

// helpDocument is generated from the bindings, so it cannot describe a
// vocabulary the session does not actually have.
func (m model) helpDocument() string {
	var b strings.Builder

	b.WriteString("## Help\n\n### Commands\n\n")

	for _, c := range m.commands() {
		word, desc := c.help()
		fmt.Fprintf(&b, "- **%s** — %s\n", word, desc)
	}

	b.WriteString("\nYou can also type any option listed under **Context**, ")
	b.WriteString("exactly as it appears there.\n\n### Keys\n\n")

	// Several keys share a keystroke and differ only by pane, so the doc says
	// where each one applies. The header hint has no room for that.
	for _, row := range []struct {
		binding key.Binding
		where   string
	}{
		{m.keys.Select, "in Context"},
		{m.keys.Choose, "in Context"},
		{m.keys.Scroll, "in Terminal and Activity"},
		{m.keys.Run, "in Command"},
		{m.keys.NextPane, ""},
		{m.keys.PrevPane, ""},
		{m.keys.ForceQuit, "anywhere"},
	} {
		h := row.binding.Help()
		if row.where == "" {
			fmt.Fprintf(&b, "- **%s** — %s\n", h.Key, h.Desc)

			continue
		}

		fmt.Fprintf(&b, "- **%s** — %s, %s\n", h.Key, h.Desc, row.where)
	}

	return b.String()
}
