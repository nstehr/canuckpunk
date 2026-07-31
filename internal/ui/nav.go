package ui

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Selecting a navItem swaps what the Main View shows.
type navItem struct {
	title string
	body  func(m model) string
}

func (n navItem) FilterValue() string { return n.title }

// list.DefaultDelegate is two lines plus spacing, too tall for a sidebar this
// narrow, so entries render on one line.
type navDelegate struct{}

func (navDelegate) Height() int                         { return 1 }
func (navDelegate) Spacing() int                        { return 0 }
func (navDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

func (navDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	it, ok := item.(navItem)
	if !ok {
		return
	}

	prefix, style := "  ", navItemStyle
	if index == m.Index() {
		prefix, style = "▸ ", navSelectedStyle
	}

	// The list writes into an in-memory buffer; the error is not actionable.
	_, _ = fmt.Fprint(w, style.MaxWidth(m.Width()).Render(prefix+it.title))
}

// Each section supplies its own Main View body, which is where real content
// gets wired in later.
func navItems() []list.Item {
	return []list.Item{
		navItem{"Session", func(m model) string {
			if len(m.transcript) == 0 {
				return dimStyle.Render("connecting…")
			}

			return strings.Join(m.transcript, "\n")
		}},
		navItem{"Overview", func(m model) string {
			rows := [][2]string{
				{"User", m.sess.Name()},
				{"Client", string(m.sess.Client)},
				{"Remote", m.sess.RemoteAddr},
				{"Terminal", m.display.Term},
				{"Size", fmt.Sprintf("%d×%d", m.width, m.height)},
				{"Background", m.bg},
				{"Color profile", m.profile},
				{"Commands run", strconv.Itoa(m.commands)},
			}
			var b strings.Builder
			for _, r := range rows {
				if r[1] == "" {
					continue
				}
				fmt.Fprintf(&b, "%s %s\n", dimStyle.Render(fmt.Sprintf("%-14s", r[0])), r[1])
			}
			return strings.TrimRight(b.String(), "\n")
		}},
		navItem{"Systems", placeholder("systems")},
		navItem{"Crew", placeholder("crew")},
		navItem{"Cargo", placeholder("cargo")},
	}
}

func placeholder(name string) func(model) string {
	return func(model) string {
		return dimStyle.Render("(" + name + ")")
	}
}

// The pane frame already draws a title, help, and status, so the list's own
// chrome is switched off.
func newNavList(width, height int) list.Model {
	l := list.New(navItems(), navDelegate{}, width, height)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)
	l.SetFilteringEnabled(false)
	l.Styles.PaginationStyle = lipgloss.NewStyle()
	return l
}

func (m model) selectedBody() string {
	it, ok := m.nav.SelectedItem().(navItem)
	if !ok {
		return ""
	}
	return it.body(m)
}
