// Package ui implements the terminal user interface.
package ui

import (
	"bytes"
	"context"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"

	tea "charm.land/bubbletea/v2"

	"github.com/nstehr/canuckpunk/internal/session"
	"github.com/nstehr/canuckpunk/internal/state"
)

type pane int

const (
	paneContext pane = iota
	paneMain
	paneActivity
	paneCommand

	paneCount
)

const commandPrompt = "> "

type stateOutput struct {
	text string
	err  error
}

// Options is the per-session front-end configuration. The zero value renders,
// so a client that knows nothing about its terminal still works.
type Options struct {
	Display Display
}

// Display only seeds the initial geometry; tea.WindowSizeMsg is authoritative
// from the first render onward.
type Display struct {
	Term   string
	Width  int
	Height int
}

// Assumed when a client does not report a size.
const (
	defaultWidth  = 80
	defaultHeight = 24
)

func (o Options) withDefaults() Options {
	if o.Display.Width <= 0 {
		o.Display.Width = defaultWidth
	}

	if o.Display.Height <= 0 {
		o.Display.Height = defaultHeight
	}

	return o
}

type model struct {
	// The session context cancels in-flight states when the client goes away.
	ctx  context.Context
	sess session.UserSession
	sm   *state.Machine

	display Display
	profile string
	width   int
	height  int
	bg      string

	focus pane
	keys  keyMap

	nav      list.Model     // Context
	main     viewport.Model // Main View
	activity viewport.Model // Activity
	input    textinput.Model
	help     help.Model

	// Everything the session has said, echoed input included.
	transcript []string
	commands   int
}

// New returns the root model for one session as a tea.Model, so callers do
// not depend on the concrete type.
func New(ctx context.Context, us session.UserSession, sm *state.Machine, opts Options) tea.Model {
	return newModel(ctx, us, sm, opts)
}

func newModel(ctx context.Context, us session.UserSession, sm *state.Machine, opts Options) model {
	opts = opts.withDefaults()
	in := textinput.New()
	in.Prompt = commandPrompt
	in.Placeholder = "type a command…"
	in.SetStyles(textinput.DefaultStyles(false))
	// A real cursor blinks and shapes natively; View places it on screen.
	in.SetVirtualCursor(false)

	m := model{
		ctx:      ctx,
		sess:     us,
		sm:       sm,
		display:  opts.Display,
		width:    opts.Display.Width,
		height:   opts.Display.Height,
		bg:       "light",
		focus:    paneCommand,
		keys:     defaultKeyMap(),
		nav:      newNavList(1, 1),
		main:     viewport.New(),
		activity: viewport.New(),
		input:    in,
		help:     help.New(),
	}
	m.resize()
	m.refresh()
	m.input.Focus()
	return m
}

func (m model) Init() tea.Cmd {
	// Run the entry state so its output is up before the first key.
	return tea.Batch(
		tea.RequestBackgroundColor,
		m.runState(""),
	)
}

// States can block, so they run off the update loop.
func (m model) runState(input string) tea.Cmd {
	return func() tea.Msg {
		var buf bytes.Buffer

		err := m.sm.Next(m.ctx, input, &buf)

		return stateOutput{text: buf.String(), err: err}
	}
}

func (m *model) resize() {
	g := m.geometry()
	if !g.ok {
		return
	}

	m.nav.SetSize(g.bodyWidth(g.leftWidth), g.bodyHeight(g.middleHeight))

	m.main.SetWidth(g.bodyWidth(g.rightWidth))
	m.main.SetHeight(g.bodyHeight(g.middleHeight))

	m.activity.SetWidth(g.bodyWidth(g.leftWidth))
	m.activity.SetHeight(g.bodyHeight(bottomHeight))

	m.input.SetWidth(g.commandInputWidth())
	m.help.SetWidth(g.bodyWidth(m.width))
}

// Viewports hold their own copy of the text, so this must run whenever the
// underlying data or the selection changes.
func (m *model) refresh() {
	m.main.SetContent(m.selectedBody())

	if len(m.transcript) == 0 {
		m.activity.SetContent(dimStyle.Render("(activity)"))
		return
	}
	atBottom := m.activity.AtBottom()
	m.activity.SetContent(strings.Join(m.transcript, "\n"))
	if atBottom {
		m.activity.GotoBottom()
	}
}

// The textinput has its own focus state that has to stay in sync with ours.
func (m *model) setFocus(p pane) tea.Cmd {
	m.focus = p
	if p == paneCommand {
		return m.input.Focus()
	}
	m.input.Blur()
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.ColorProfileMsg:
		m.profile = msg.String()
		m.refresh()

	case tea.BackgroundColorMsg:
		if msg.IsDark() {
			m.bg = "dark"
		}
		m.input.SetStyles(textinput.DefaultStyles(msg.IsDark()))
		m.help.Styles = help.DefaultStyles(msg.IsDark())
		m.nav.Styles = list.DefaultStyles(msg.IsDark())
		m.refresh()

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resize()
		m.refresh()

	case stateOutput:
		m.appendOutput(msg)
		m.refresh()
		m.activity.GotoBottom()

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.ForceQuit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.NextPane):
		return m, m.setFocus((m.focus + 1) % paneCount)
	case key.Matches(msg, m.keys.PrevPane):
		return m, m.setFocus((m.focus + paneCount - 1) % paneCount)
	}

	// The textinput takes every remaining key, including "q", which would
	// otherwise quit mid-word.
	if m.focus == paneCommand {
		if key.Matches(msg, m.keys.Run) {
			return m, m.submit()
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	if key.Matches(msg, m.keys.Quit) {
		return m, tea.Quit
	}

	var cmd tea.Cmd
	switch m.focus {
	case paneContext:
		selected := m.nav.Index()
		m.nav, cmd = m.nav.Update(msg)
		if m.nav.Index() != selected {
			m.refresh()
		}
	case paneMain:
		m.main, cmd = m.main.Update(msg)
	case paneActivity:
		m.activity, cmd = m.activity.Update(msg)
	default: // paneCommand is handled above; paneCount is a sentinel.
	}
	return m, cmd
}

func (m *model) submit() tea.Cmd {
	line := strings.TrimSpace(m.input.Value())
	if line == "" {
		return nil
	}

	m.commands++
	m.transcript = append(m.transcript, commandPrompt+line)
	m.input.Reset()
	m.refresh()
	m.activity.GotoBottom() // a newly run command should always be visible

	return m.runState(line)
}

func (m *model) appendOutput(o stateOutput) {
	if o.err != nil {
		m.transcript = append(m.transcript, errStyle.Render("! "+o.err.Error()))

		return
	}

	for line := range strings.SplitSeq(strings.TrimRight(o.text, "\n"), "\n") {
		if line != "" {
			m.transcript = append(m.transcript, line)
		}
	}
}

func (m model) View() tea.View {
	v := tea.NewView(m.layout())
	v.AltScreen = true

	// The textinput reports its cursor relative to its own origin, so
	// translate it to a screen position.
	if g := m.geometry(); g.ok && m.focus == paneCommand {
		if c := m.input.Cursor(); c != nil {
			x, y := g.commandContentOrigin()
			c.X += x
			c.Y += y
			v.Cursor = c
		}
	}

	return v
}
