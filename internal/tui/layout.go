package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"
)

// Row heights include borders.
const (
	headerHeight = 3
	bottomHeight = 6

	sidebarRatio = 0.28
	sidebarMin   = 16
	sidebarMax   = 40
)

var (
	borderColor      = lipgloss.Color("240")
	focusBorderColor = lipgloss.Color("62")
	titleColor       = lipgloss.Color("110")
	dimColor         = lipgloss.Color("244")
	errColor         = lipgloss.Color("203")

	paneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderColor).
			Padding(0, 1)

	focusedPaneStyle = paneStyle.BorderForeground(focusBorderColor)

	titleStyle = lipgloss.NewStyle().Foreground(titleColor).Bold(true)
	dimStyle   = lipgloss.NewStyle().Foreground(dimColor)
	errStyle   = lipgloss.NewStyle().Foreground(errColor)

	navItemStyle     = lipgloss.NewStyle()
	navSelectedStyle = lipgloss.NewStyle().Foreground(focusBorderColor).Bold(true)
)

// The layout, the component sizes, and the cursor placement all derive from
// geometry, so they cannot drift apart.
type geometry struct {
	ok           bool
	leftWidth    int
	rightWidth   int
	middleHeight int
}

func (m model) geometry() geometry {
	if m.width < 20 || m.height < headerHeight+bottomHeight+3 {
		return geometry{}
	}
	left := clamp(int(float64(m.width)*sidebarRatio), sidebarMin, sidebarMax)
	return geometry{
		ok:           true,
		leftWidth:    left,
		rightWidth:   m.width - left,
		middleHeight: m.height - headerHeight - bottomHeight,
	}
}

// Less border and padding.
func (geometry) bodyWidth(outerWidth int) int { return max(1, outerWidth-4) }

// Less borders and the title line.
func (geometry) bodyHeight(outerHeight int) int { return max(1, outerHeight-3) }

// The border, padding, and title line all shift the body down and right; the
// cursor has to be placed past them.
func (g geometry) commandContentOrigin() (x, y int) {
	return g.leftWidth + 2, headerHeight + g.middleHeight + 2
}

func (g geometry) commandInputWidth() int {
	return max(1, g.bodyWidth(g.rightWidth)-lipgloss.Width(commandPrompt))
}

// The five regions:
//
//	┌──────────────────────────────┐
//	│ Header                       │
//	├──────────┬───────────────────┤
//	│ Context  │ Main View         │
//	├──────────┼───────────────────┤
//	│ Activity │ Command           │
//	└──────────┴───────────────────┘
func (m model) layout() string {
	g := m.geometry()
	if !g.ok {
		return "terminal too small"
	}

	header := m.renderPane(-1, m.width, headerHeight, "", "", m.headerContent())

	context := m.renderPane(paneContext, g.leftWidth, g.middleHeight, "Context", "", m.nav.View())
	main := m.renderPane(paneMain, g.rightWidth, g.middleHeight,
		m.mainTitle(), scrollHint(m.main), m.main.View())

	activity := m.renderPane(paneActivity, g.leftWidth, bottomHeight,
		"Activity", scrollHint(m.activity), m.activity.View())
	command := m.renderPane(paneCommand, g.rightWidth, bottomHeight, "Command", "", m.input.View())

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		lipgloss.JoinHorizontal(lipgloss.Top, context, main),
		lipgloss.JoinHorizontal(lipgloss.Top, activity, command),
	)
}

// Pass a focus target of -1 for a pane that can never hold focus. status is
// right-aligned on the title row; pass "" for none.
func (m model) renderPane(p pane, outerWidth, outerHeight int, title, status, body string) string {
	style := paneStyle
	if p >= 0 && p == m.focus {
		style = focusedPaneStyle
	}

	content := body
	if title != "" {
		content = m.titleRow(outerWidth, title, status) + "\n" + body
	}

	// In lipgloss v2 Width/Height include border and padding.
	return style.Width(outerWidth).Height(outerHeight).Render(content)
}

func (m model) titleRow(outerWidth int, title, status string) string {
	head := titleStyle.Render(title)
	if status == "" {
		return head
	}

	gap := geometry{}.bodyWidth(outerWidth) - lipgloss.Width(head) - lipgloss.Width(status)
	if gap < 1 {
		return head
	}

	return head + strings.Repeat(" ", gap) + status
}

// scrollHint shows where the reader is and which way there is more to read.
// Empty when it all fits, so a pane that needs no scrolling stays quiet. The
// arrow slots keep their width either way, so the percentage does not jitter
// as it scrolls.
func scrollHint(v viewport.Model) string {
	if v.TotalLineCount() <= v.Height() {
		return ""
	}

	up, down := " ", " "
	if !v.AtTop() {
		up = "▲"
	}

	if !v.AtBottom() {
		down = "▼"
	}

	return dimStyle.Render(fmt.Sprintf("%s%s %3.0f%%", up, down, v.ScrollPercent()*100))
}

// The Main pane is the narrative surface; it is not named after the sidebar.
func (m model) mainTitle() string {
	return "Terminal"
}

func (m model) headerContent() string {
	left := titleStyle.Render("canuckpunk")
	right := m.help.View(m)

	gap := m.geometry().bodyWidth(m.width) - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		return left
	}
	return left + strings.Repeat(" ", gap) + right
}

func clamp(v, low, high int) int {
	if v < low {
		return low
	}
	if v > high {
		return high
	}
	return v
}
