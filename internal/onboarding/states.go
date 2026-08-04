package onboarding

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/mail"
	"strings"

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

	ask = func(_ context.Context, input string, out io.Writer) (state.Fn, error) {
		if input == "" {
			return ask, writeNamePrompt(out)
		}

		if accounts == nil {
			return nil, writeOrientation(out, prose)
		}

		return askEmail(accounts, prose, input), writeEmailPrompt(out)
	}

	return ask
}

// askEmail collects the optional address that lets another client find this
// person's accounts later. The account is not created until this step
// finishes, so a rejected name and a rejected address are both recoverable
// without a half-written record.
func askEmail(accounts Accounts, prose Narratives, username string) state.Fn {
	var ask state.Fn

	ask = func(ctx context.Context, input string, out io.Writer) (state.Fn, error) {
		if input == "" {
			return ask, writeEmailPrompt(out)
		}

		email := input
		if strings.EqualFold(strings.TrimSpace(input), SkipEmail) {
			email = ""
		} else if !validEmail(email) {
			state.Hint(out, HintEmail)

			_, werr := fmt.Fprintf(out,
				"%q is not an address the Registry can use. Type an address, or `%s`.",
				input, SkipEmail)

			return ask, werr
		}

		return create(ctx, accounts, prose, out, username, email)
	}

	return ask
}

// create is the end of onboarding: an account exists after this, or the player
// is still standing somewhere they can retry from.
func create(
	ctx context.Context,
	accounts Accounts,
	prose Narratives,
	out io.Writer,
	username, email string,
) (state.Fn, error) {
	us, _ := session.FromContext(ctx)

	created, err := accounts.Create(ctx, user.NewAccount{
		Username:     username,
		Email:        email,
		CredentialID: us.Credential.ID,
		Material:     us.Credential.Material,
	})
	if err != nil {
		if errors.Is(err, user.ErrUsernameTaken) {
			state.Hint(out, HintName)

			_, werr := fmt.Fprintf(out,
				"The Registry already holds a file under %q. Choose another name.", username)

			return askName(accounts, prose), werr
		}

		return nil, err
	}

	if _, err := fmt.Fprintf(out, "File opened for **%s**.\n\n", created.Username); err != nil {
		return nil, fmt.Errorf("write confirmation: %w", err)
	}

	return nil, writeOrientation(out, prose)
}

// A misspelled address is worse than none: it would hand this person's
// characters to whoever owns the address that was actually typed. Bare
// addresses only — "Name <a@b.ca>" is not what was asked for.
func validEmail(email string) bool {
	trimmed := strings.TrimSpace(email)

	addr, err := mail.ParseAddress(trimmed)

	return err == nil && addr.Address == trimmed
}

func writeWelcome(out io.Writer, prose Narratives) error {
	return writeNarrative(out, prose, NarrativeWelcome)
}

func writeOrientation(out io.Writer, prose Narratives) error {
	return writeNarrative(out, prose, NarrativeOrientation)
}

// The hint rides with the prompt: whoever writes the question is the one that
// knows what answer it wants, and the front end is told on the same turn the
// player is asked.
func writeNamePrompt(out io.Writer) error {
	state.Hint(out, HintName)

	_, err := io.WriteString(out, "## Open a file\n\nType the name the Registry should write down.")
	if err != nil {
		return fmt.Errorf("write name prompt: %w", err)
	}

	return nil
}

func writeEmailPrompt(out io.Writer) error {
	state.Hint(out, HintEmail)

	_, err := fmt.Fprintf(out, "## An address, if you want one on file\n\n"+
		"The Registry can hold an address so your file can be found from "+
		"somewhere other than this terminal. It is optional, and it is not "+
		"checked.\n\nType an address, or `%s`.", SkipEmail)
	if err != nil {
		return fmt.Errorf("write email prompt: %w", err)
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
