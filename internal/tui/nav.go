package tui

import (
	"fmt"
	"io"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/nstehr/canuckpunk/internal/menu"
)

type choiceItem struct {
	choice menu.Choice
}

func (c choiceItem) FilterValue() string { return c.choice.Label }

// list.DefaultDelegate is two lines plus spacing, too tall for a sidebar this
// narrow, so entries render on one line.
type choiceDelegate struct{}

func (choiceDelegate) Height() int                         { return 1 }
func (choiceDelegate) Spacing() int                        { return 0 }
func (choiceDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

func (choiceDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	it, ok := item.(choiceItem)
	if !ok {
		return
	}

	prefix, style := "  ", navItemStyle
	if index == m.Index() {
		prefix, style = "▸ ", navSelectedStyle
	}

	// The list writes into an in-memory buffer; the error is not actionable.
	_, _ = fmt.Fprint(w, style.MaxWidth(m.Width()).Render(prefix+it.choice.Label))
}

// The pane frame already draws a title, help, and status, so the list's own
// chrome is switched off.
func newChoiceList(width, height int) list.Model {
	l := list.New(nil, choiceDelegate{}, width, height)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)
	l.SetFilteringEnabled(false)
	l.Styles.PaginationStyle = lipgloss.NewStyle()

	return l
}

func (m *model) setChoices(choices menu.Set) {
	m.choices = choices

	items := make([]list.Item, 0, len(choices))
	for _, c := range choices {
		items = append(items, choiceItem{choice: c})
	}

	_ = m.nav.SetItems(items)
	m.nav.Select(0)
}

func (m model) selectedChoice() (menu.Choice, bool) {
	it, ok := m.nav.SelectedItem().(choiceItem)
	if !ok {
		return menu.Choice{}, false
	}

	return it.choice, true
}
