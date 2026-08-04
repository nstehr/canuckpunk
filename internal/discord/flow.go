package discord

import (
	"context"
	"fmt"
	"strings"

	"github.com/nstehr/canuckpunk/internal/menu"
	"github.com/nstehr/canuckpunk/internal/session"
)

// screen is one turn's worth of output: the prose, whatever the player can
// pick next, and what the state is waiting for them to type.
type screen struct {
	text    string
	choices menu.Set
	hint    string
	done    bool
}

// Global words, matching the terminal front end. Checked before the machine
// sees the line so a state waiting on free text cannot swallow one.
const (
	cmdHelp = "help"
	cmdQuit = "quit"
	cmdExit = "exit"
)

// advance runs one turn. It is the only place the state machine is touched,
// and it holds the conversation lock for the whole turn because Discord will
// happily deliver two clicks at once.
func (b *Bot) advance(ctx context.Context, c *conversation, input string) (screen, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	ctx = session.NewContext(ctx, c.session)

	var w writer
	if err := c.machine.Next(ctx, input, &w); err != nil {
		return screen{}, fmt.Errorf("advance: %w", err)
	}

	s := screen{text: w.text(), hint: w.hint, done: c.machine.Done()}

	// Choices belong to the opening screen. Once one is taken the flow is
	// asking for free text, and a stale row of buttons would only mislead.
	if input == "" {
		choices, err := b.chooser.Choices(ctx)
		if err != nil {
			return screen{}, fmt.Errorf("look up choices: %w", err)
		}

		s.choices = choices
	}

	return s, nil
}

// global handles the words that work anywhere, returning false when the line
// is ordinary input for the machine.
func (b *Bot) global(userID, input string) (screen, bool) {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case cmdHelp:
		return screen{text: helpText}, true
	case cmdQuit, cmdExit:
		b.convos.drop(userID)

		return screen{text: quitText, done: true}, true
	default:
		return screen{}, false
	}
}

const helpText = "## Help\n\n" +
	"- **help** — this list\n" +
	"- **quit** / **exit** — put the file down\n\n" +
	"Press a button, or type an option exactly as it is labelled. " +
	"Use `/begin` to start over."

const quitText = "The desk goes quiet. Use `/begin` when you want to pick it up again."
