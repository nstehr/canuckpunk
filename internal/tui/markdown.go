package tui

import (
	"strings"

	"charm.land/glamour/v2"
	"charm.land/glamour/v2/ansi"
	"charm.land/glamour/v2/styles"
)

// Glamour's built-in style names, which double as our own theme labels.
const (
	themeDark  = "dark"
	themeLight = "light"
)

// Applied to h3 and below; glamour takes colours as ANSI strings.
var subheadingColor = "246"

// markdown renders narrative prose. Glamour holds the wrap width, so the
// renderer is rebuilt whenever the pane resizes or the theme changes.
type markdown struct {
	renderer *glamour.TermRenderer
	width    int
	dark     bool
}

func newMarkdown(width int, dark bool) markdown {
	m := markdown{width: width, dark: dark}
	m.rebuild()

	return m
}

func (m *markdown) resize(width int, dark bool) {
	if width == m.width && dark == m.dark {
		return
	}

	m.width, m.dark = width, dark
	m.rebuild()
}

func (m *markdown) rebuild() {
	cfg := styles.LightStyleConfig
	if m.dark {
		cfg = styles.DarkStyleConfig
	}

	// The pane already supplies padding; glamour's own document margin would
	// indent the prose a second time.
	var noMargin uint
	cfg.Document.Margin = &noMargin

	// Glamour's stock styles prefix headings with their markdown markers, so
	// "## Orientation" renders with the hashes still attached. In a game
	// window that reads as unrendered source.
	for _, h := range []*ansi.StyleBlock{&cfg.H2, &cfg.H3, &cfg.H4, &cfg.H5, &cfg.H6} {
		h.Prefix = ""
	}

	// Those markers were also the only thing separating the heading levels, so
	// dim the deeper ones to keep the hierarchy readable.
	for _, h := range []*ansi.StyleBlock{&cfg.H3, &cfg.H4, &cfg.H5, &cfg.H6} {
		h.Color = &subheadingColor
	}

	// A nil renderer is handled in render, so a failure here degrades to plain
	// text rather than taking the session down.
	r, err := glamour.NewTermRenderer(
		glamour.WithStyles(cfg),
		glamour.WithWordWrap(max(1, m.width)),
		glamour.WithEmoji(),
	)
	if err != nil {
		m.renderer = nil

		return
	}

	m.renderer = r
}

// render returns the rendered document and the line each chunk starts on, so
// the viewport can open a new passage at its beginning. Chunks are rendered
// one at a time and joined with our own rule; letting glamour lay out the
// whole document at once would make those offsets guesswork.
func (m markdown) render(chunks []string) (string, []int) {
	var (
		lines   []string
		offsets = make([]int, 0, len(chunks))
	)

	for i, chunk := range chunks {
		if i > 0 {
			lines = append(lines, "", m.rule(), "")
		}

		offsets = append(offsets, len(lines))
		lines = append(lines, strings.Split(m.renderOne(chunk), "\n")...)
	}

	return strings.Join(lines, "\n"), offsets
}

// renderOne falls back to the raw source whenever glamour cannot help, so
// prose is never lost to a rendering error.
func (m markdown) renderOne(src string) string {
	if m.renderer == nil {
		return src
	}

	out, err := m.renderer.Render(src)
	if err != nil {
		return src
	}

	return strings.Trim(out, "\n")
}

func (m markdown) rule() string {
	return dimStyle.Render(strings.Repeat("─", max(1, m.width)))
}
