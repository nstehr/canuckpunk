package onboarding

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/nstehr/canuckpunk/internal/session"
	"github.com/nstehr/canuckpunk/internal/state"
	"github.com/nstehr/canuckpunk/internal/user"
)

// Start is the entry state: it greets the player and waits for a choice.
//
// Machine.Next runs a state once per input, so a state that hands off also
// writes what the player should see on arrival — the state it names is not run
// until the next line arrives. Adding a state means honouring that, or the
// screen stays blank until the player types again.
func Start(accounts Accounts, prose Narratives) state.Fn {
	chooser := NewChooser(accounts)

	var start state.Fn

	start = func(ctx context.Context, input string, out io.Writer) (state.Fn, error) {
		// The opening screen is the same for everyone, so it needs no account
		// lookup.
		if input == "" {
			return start, writeWelcome(out, prose)
		}

		choices, err := chooser.Choices(ctx)
		if err != nil {
			return nil, err
		}

		choice, ok := choices.Resolve(input)
		if !ok {
			_, err := fmt.Fprintf(out, "The desk does not recognise %q. Choose one of the listed files.", input)

			return start, err
		}

		switch choice.Action {
		case ActionContinue:
			return nil, writeOrientation(out, prose)
		case ActionNewUser:
			return askName(accounts, prose), writeNamePrompt(out)
		}

		return start, nil
	}

	return start
}

// Returning itself on a bad name keeps the player here rather than dropping
// them out of the flow with no way back.
func askName(accounts Accounts, prose Narratives) state.Fn {
	var ask state.Fn

	ask = func(ctx context.Context, input string, out io.Writer) (state.Fn, error) {
		if input == "" {
			return ask, writeNamePrompt(out)
		}

		if accounts == nil {
			return nil, writeOrientation(out, prose)
		}

		us, _ := session.FromContext(ctx)

		created, err := accounts.Create(ctx, input, us.Credential.ID, us.Credential.Material)
		if err != nil {
			if errors.Is(err, user.ErrUsernameTaken) {
				_, err := fmt.Fprintf(out, "The Registry already holds a file under %q. Choose another name.", input)

				return ask, err
			}

			return ask, err
		}

		if _, err := fmt.Fprintf(out, "File opened for **%s**.\n\n", created.Username); err != nil {
			return nil, fmt.Errorf("write confirmation: %w", err)
		}

		return nil, writeOrientation(out, prose)
	}

	return ask
}

func writeWelcome(out io.Writer, prose Narratives) error {
	return writeNarrative(out, prose, narrativeWelcome)
}

func writeOrientation(out io.Writer, prose Narratives) error {
	return writeNarrative(out, prose, narrativeOrientation)
}

func writeNamePrompt(out io.Writer) error {
	_, err := io.WriteString(out, "## Open a file\n\nType the name the Registry should write down.")
	if err != nil {
		return fmt.Errorf("write name prompt: %w", err)
	}

	return nil
}

func writeNarrative(out io.Writer, prose Narratives, name string) error {
	text, err := prose.Read(name)
	if err != nil {
		return err
	}

	if _, err := io.WriteString(out, text); err != nil {
		return fmt.Errorf("write narrative: %w", err)
	}

	return nil
}
