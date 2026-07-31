package tui

import (
	"charm.land/bubbles/v2/key"
)

// Only the bindings the top-level model handles. List navigation, viewport
// scrolling, and line editing belong to the components themselves.
type keyMap struct {
	NextPane  key.Binding
	PrevPane  key.Binding
	Quit      key.Binding
	ForceQuit key.Binding

	// Never matched — the focused component owns these keys. They exist so
	// help can describe them.
	Select key.Binding
	Scroll key.Binding
	Choose key.Binding
	Run    key.Binding

	// Help and Leave hold typed words rather than keystrokes. Keeping them in
	// bindings means the accepted words and the help text cannot drift.
	Help  key.Binding
	Leave key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		NextPane: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next pane"),
		),
		PrevPane: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("shift+tab", "prev pane"),
		),
		// Only when the command line is blurred, or it would quit mid-word.
		Quit: key.NewBinding(
			key.WithKeys("q"),
			key.WithHelp("q", "quit"),
		),
		// Always applies, including while typing.
		ForceQuit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "quit"),
		),
		Select: key.NewBinding(
			key.WithKeys("up", "down"),
			key.WithHelp("↑/↓", "select"),
		),
		Scroll: key.NewBinding(
			key.WithKeys("up", "down"),
			key.WithHelp("↑/↓", "scroll"),
		),
		Choose: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "choose"),
		),
		Run: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "run"),
		),
		Help: key.NewBinding(
			key.WithKeys("help", "?"),
			key.WithHelp("help", "commands"),
		),
		Leave: key.NewBinding(
			key.WithKeys("quit", "exit"),
			key.WithHelp("quit/exit", "leave"),
		),
	}
}

// help.KeyMap, so the help bubble renders bindings for the focused pane.
var _ interface {
	ShortHelp() []key.Binding
	FullHelp() [][]key.Binding
} = model{}

func (m model) focusHelp() []key.Binding {
	switch m.focus {
	case paneContext:
		return []key.Binding{m.keys.Select, m.keys.Choose}
	case paneMain, paneActivity:
		return []key.Binding{m.keys.Scroll}
	case paneCommand:
		return []key.Binding{m.keys.Run, m.keys.Help, m.keys.Leave}
	default: // paneCount is a sentinel.
		return nil
	}
}

func (m model) ShortHelp() []key.Binding {
	return append(m.focusHelp(), m.keys.NextPane, m.keys.ForceQuit)
}

func (m model) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		m.focusHelp(),
		{m.keys.NextPane, m.keys.PrevPane},
		{m.keys.Quit, m.keys.ForceQuit},
	}
}
