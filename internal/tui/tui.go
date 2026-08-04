// Package tui implements the terminal user interface.
package tui

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
	"charm.land/lipgloss/v2"

	"github.com/nstehr/canuckpunk/internal/menu"
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
	hint string
	err  error
}

// stateWriter is what a state writes into. Implementing state.Hinter is how a
// state tells the front end what it is waiting for.
type stateWriter struct {
	buf  bytes.Buffer
	hint string
}

func (w *stateWriter) Write(p []byte) (int, error) { return w.buf.Write(p) }
func (w *stateWriter) Hint(kind string)            { w.hint = kind }

type choicesLoaded struct {
	choices menu.Set
	err     error
}

// Options is the per-session front-end configuration. The zero value renders,
// so a client that knows nothing about its terminal still works.
type Options struct {
	Display Display

	// Choices supplies the options offered in the Context pane. Nil means the
	// session runs without any.
	Choices menu.Source
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

// Bubble Tea passes the model by value, so Update works on a copy: mutating
// helpers take a pointer receiver and only affect the copy that Update
// returns. Anything that changes content or transcript must call refresh
// before the next render, or the panes keep showing the previous state.
type model struct {
	// The session context cancels in-flight states when the client goes away.
	ctx  context.Context
	sess session.UserSession
	sm   *state.Machine

	display Display
	hint    string
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
	md       markdown

	chooser menu.Source
	choices menu.Set

	// Narrative markdown, oldest first; the Main pane renders all of it.
	content []string

	// The line each passage starts on, so a new one opens at its top.
	offsets []int

	// Plain lines for the Activity pane: what was typed, and any trouble.
	transcript []string
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
		bg:       themeDark,
		focus:    paneCommand,
		keys:     defaultKeyMap(),
		nav:      newChoiceList(1, 1),
		main:     viewport.New(),
		activity: viewport.New(),
		input:    in,
		help:     help.New(),
		chooser:  opts.Choices,
	}
	m.md = newMarkdown(1, true)
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
		m.loadChoices(),
	)
}

// Off the update loop: the lookup hits the database.
func (m model) loadChoices() tea.Cmd {
	if m.chooser == nil {
		return nil
	}

	return func() tea.Msg {
		choices, err := m.chooser.Choices(m.ctx)

		return choicesLoaded{choices: choices, err: err}
	}
}

// States can block, so they run off the update loop.
func (m model) runState(input string) tea.Cmd {
	return func() tea.Msg {
		var w stateWriter

		err := m.sm.Next(m.ctx, input, &w)

		return stateOutput{text: w.buf.String(), hint: w.hint, err: err}
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

	m.input.SetWidth(g.commandInputWidth(m.input.Prompt))
	m.help.SetWidth(g.bodyWidth(m.width))
	m.md.resize(g.bodyWidth(g.rightWidth), m.bg == themeDark)
}

// Viewports hold their own copy of the text, so this must run whenever the
// underlying data or the selection changes.
func (m *model) refresh() {
	if len(m.content) == 0 {
		m.main.SetContent(dimStyle.Render("connecting…"))
	} else {
		rendered, offsets := m.md.render(m.content)
		m.offsets = offsets
		m.main.SetContent(rendered)
	}

	if len(m.transcript) == 0 {
		m.activity.SetContent(dimStyle.Render("(activity)"))
		return
	}

	atBottom := m.activity.AtBottom()
	// The pane is narrow and notices can be long; without wrapping the tail of
	// an error is simply lost.
	g := m.geometry()
	wrap := lipgloss.NewStyle().Width(g.bodyWidth(g.leftWidth))
	m.activity.SetContent(wrap.Render(strings.Join(m.transcript, "\n")))
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
		m.bg = themeLight
		if msg.IsDark() {
			m.bg = themeDark
		}
		m.input.SetStyles(textinput.DefaultStyles(msg.IsDark()))
		m.help.Styles = help.DefaultStyles(msg.IsDark())
		m.nav.Styles = list.DefaultStyles(msg.IsDark())
		m.resize()
		m.refresh()

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resize()
		m.refresh()

	case stateOutput:
		m.appendOutput(msg)
		m.setPrompt(msg.hint)
		m.refresh()
		m.showLatestPassage()
		m.activity.GotoBottom()

	case choicesLoaded:
		if msg.err != nil {
			m.note("the desk could not read the Registry: " + msg.err.Error())
		}

		m.setChoices(msg.choices)
		m.refresh()

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

	// Enter on a highlighted choice runs it, so selecting and typing the label
	// reach the same place.
	if m.focus == paneContext && key.Matches(msg, m.keys.Run) {
		return m, m.activate()
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
	line := m.input.Value()
	m.input.Reset()

	return m.run(line, line)
}

// Selecting sends the choice's ID rather than its label — the same thing a
// web or chat front end would send — while the log still shows what the
// player saw.
func (m *model) activate() tea.Cmd {
	choice, ok := m.selectedChoice()
	if !ok {
		return nil
	}

	return m.run(choice.Label, choice.ID)
}

// run is the single path into the state machine, so a typed label and a
// selected choice behave identically. shown is what goes in the log; input is
// what the machine resolves.
//
// Order matters: global commands win over choices, and choices are only
// consumed here so the sidebar can be cleared — the machine still resolves the
// line itself.
func (m *model) run(shown, input string) tea.Cmd {
	shown, input = strings.TrimSpace(shown), strings.TrimSpace(input)
	if input == "" {
		return nil
	}

	m.transcript = append(m.transcript, commandPrompt+shown)

	// Global commands are resolved before the machine sees the line, so a
	// state waiting on free text cannot swallow one.
	if c, ok := m.lookup(input); ok {
		m.refresh()

		return c.run(m)
	}

	// Taking a choice moves the flow on, so the old options no longer apply.
	if _, ok := m.choices.Resolve(input); ok {
		m.setChoices(nil)
	}

	m.refresh()
	m.activity.GotoBottom() // a newly run command should always be visible

	return m.runState(input)
}

// State output is markdown for the Main pane; failures are operator detail
// and belong in the Activity log instead.
func (m *model) appendOutput(o stateOutput) {
	if o.err != nil {
		m.note(o.err.Error())
	}

	if text := strings.TrimSpace(o.text); text != "" {
		m.content = append(m.content, text)
	}
}

// A passage is read from its beginning, so the viewport opens at the top of
// the newest one rather than the bottom of the document.
func (m *model) showLatestPassage() {
	if len(m.offsets) == 0 {
		m.main.GotoTop()

		return
	}

	m.main.SetYOffset(m.offsets[len(m.offsets)-1])
}

// setPrompt shows what the current state is waiting for, so the player can see
// it without re-reading the passage above.
func (m *model) setPrompt(hint string) {
	m.hint = hint
	m.input.Prompt = m.promptFor(hint)

	// The prompt just changed width, so the text area has to be resized with
	// it or the line runs past the pane.
	if g := m.geometry(); g.ok {
		m.input.SetWidth(g.commandInputWidth(m.input.Prompt))
	}
}

func (m model) promptFor(hint string) string {
	if hint == "" {
		return commandPrompt
	}

	return dimStyle.Render("("+hint+")") + " " + commandPrompt
}

func (m *model) note(text string) {
	m.transcript = append(m.transcript, errStyle.Render("! "+text))
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
