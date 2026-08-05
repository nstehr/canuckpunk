package discord

import (
	"net/url"
	"strings"

	"github.com/disgoorg/disgo/discord"

	"github.com/nstehr/canuckpunk/internal/menu"
)

// Discord's own limits. Messages are cut below the real 2000 so a chunk can
// carry a continuation marker without tipping over.
const (
	maxMessageRunes = 1900
	maxButtons      = 5
	maxSelectItems  = 25
	maxButtonLabel  = 80
	maxOptionLabel  = 100
)

// Choice IDs are routed like paths, so the adapter wraps them rather than
// making the game's vocabulary path-shaped.
const choiceRoute = "/choice/"

func choiceCustomID(choiceID string) string {
	return choiceRoute + url.PathEscape(choiceID)
}

// choiceID reverses choiceCustomID. A malformed id is treated as absent rather
// than an error: it can only come from a mangled custom_id, and the flow
// recovers by re-showing the menu.
func choiceID(escaped string) string {
	id, err := url.PathUnescape(escaped)
	if err != nil {
		return ""
	}

	return id
}

// choiceFromValue reverses the value stored on a select menu option.
func choiceFromValue(value string) string {
	return choiceID(strings.TrimPrefix(value, choiceRoute))
}

// clip keeps a label inside Discord's limit. Over-long labels are rejected for
// the whole message, so one player with a long name would otherwise make the
// opening screen fail to send. The id is untouched, so the choice still
// resolves.
func clip(label string, limit int) string {
	runes := []rune(label)
	if len(runes) <= limit {
		return label
	}

	return string(runes[:limit-1]) + "…"
}

// chunk splits markdown to fit Discord's message limit, preferring paragraph
// breaks so a passage never splits mid-sentence.
func chunk(md string) []string {
	md = strings.TrimSpace(md)
	if md == "" {
		return nil
	}

	var (
		out     []string
		current strings.Builder
	)

	flush := func() {
		if current.Len() > 0 {
			out = append(out, strings.TrimSpace(current.String()))
			current.Reset()
		}
	}

	for _, para := range strings.Split(md, "\n\n") {
		for _, piece := range split(para, maxMessageRunes) {
			if current.Len() > 0 && current.Len()+len("\n\n")+len(piece) > maxMessageRunes {
				flush()
			}

			if current.Len() > 0 {
				current.WriteString("\n\n")
			}

			current.WriteString(piece)
		}
	}

	flush()

	return out
}

// split breaks a single paragraph that is itself too long, on line boundaries
// where it can and mid-line only when it must.
func split(para string, limit int) []string {
	if len([]rune(para)) <= limit {
		return []string{para}
	}

	var (
		out     []string
		current strings.Builder
	)

	for _, line := range strings.Split(para, "\n") {
		for len([]rune(line)) > limit {
			runes := []rune(line)
			out = append(out, string(runes[:limit]))
			line = string(runes[limit:])
		}

		if current.Len() > 0 && current.Len()+1+len(line) > limit {
			out = append(out, current.String())
			current.Reset()
		}

		if current.Len() > 0 {
			current.WriteString("\n")
		}

		current.WriteString(line)
	}

	if current.Len() > 0 {
		out = append(out, current.String())
	}

	return out
}

// components turns choices into something clickable: buttons while there are
// few, a select menu once there are too many to fit a row.
func components(choices menu.Set) []discord.LayoutComponent {
	if len(choices) == 0 {
		return nil
	}

	if len(choices) > maxSelectItems {
		choices = choices[:maxSelectItems]
	}

	if len(choices) <= maxButtons {
		buttons := make([]discord.InteractiveComponent, 0, len(choices))
		for _, c := range choices {
			buttons = append(buttons,
				discord.NewSecondaryButton(clip(c.Label, maxButtonLabel), choiceCustomID(c.ID)))
		}

		return []discord.LayoutComponent{discord.NewActionRow(buttons...)}
	}

	options := make([]discord.StringSelectMenuOption, 0, len(choices))
	for _, c := range choices {
		options = append(options,
			discord.NewStringSelectMenuOption(clip(c.Label, maxOptionLabel), choiceCustomID(c.ID)))
	}

	return []discord.LayoutComponent{
		discord.NewActionRow(discord.NewStringSelectMenu(choiceRoute, "Choose a file", options...)),
	}
}

// messages renders a screen as the sequence of messages to send. Only the last
// carries the choices, so the buttons sit under the text that explains them.
func messages(s screen) []discord.MessageCreate {
	parts := chunk(s.text)
	if len(parts) == 0 {
		return nil
	}

	out := make([]discord.MessageCreate, 0, len(parts))
	for i, part := range parts {
		msg := discord.MessageCreate{Content: part}
		if i == len(parts)-1 {
			msg.Components = components(s.choices)
		}

		out = append(out, msg)
	}

	return out
}
