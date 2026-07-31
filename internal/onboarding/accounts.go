// Package onboarding gets a player from a connection to a character: it
// offers the accounts their credential can reach, opens a new one, and hands
// off to the game. It renders nothing.
package onboarding

import (
	"context"
	"fmt"
	"strconv"

	"github.com/nstehr/canuckpunk/internal/menu"
	"github.com/nstehr/canuckpunk/internal/session"
	"github.com/nstehr/canuckpunk/internal/user"
)

// Narratives supplies the prose. Declared here so the caller decides whether
// it comes from disk or from the binary.
type Narratives interface {
	Read(name string) (string, error)
}

// Accounts is the slice of user lookup that onboarding needs. Declared here so
// the caller can substitute a fake.
type Accounts interface {
	ForCredential(ctx context.Context, credentialID string) ([]user.User, error)
	Create(ctx context.Context, username, credentialID, material string) (user.User, error)
}

// Chooser builds the options offered at the start of a session.
type Chooser struct {
	accounts Accounts
}

// NewChooser returns a Chooser backed by accounts.
func NewChooser(accounts Accounts) *Chooser {
	return &Chooser{accounts: accounts}
}

// Choices lists one entry per account already reachable from the player's
// credential, plus a way to begin a new one. An unrecognised credential offers
// only that.
func (c *Chooser) Choices(ctx context.Context) (menu.Set, error) {
	us, _ := session.FromContext(ctx)

	accounts, err := c.lookup(ctx, us.Credential.ID)
	if err != nil {
		return nil, err
	}

	if len(accounts) == 0 {
		return menu.Set{{
			ID:     IDCreateUser,
			Label:  LabelCreateUser,
			Action: ActionNewUser,
		}}, nil
	}

	choices := make(menu.Set, 0, len(accounts)+1)
	for _, a := range accounts {
		choices = append(choices, menu.Choice{
			ID:     ContinueID(a.ID),
			Label:  ContinueLabel(a.Username),
			Action: ActionContinue,
			Value:  strconv.FormatInt(a.ID, 10),
		})
	}

	return append(choices, menu.Choice{
		ID:     IDStartNewGame,
		Label:  LabelStartNewGame,
		Action: ActionNewUser,
	}), nil
}

// An unrecognised credential is the new-player path, not an error.
func (c *Chooser) lookup(ctx context.Context, credentialID string) ([]user.User, error) {
	if c.accounts == nil || credentialID == "" {
		return nil, nil
	}

	accounts, err := c.accounts.ForCredential(ctx, credentialID)
	if err != nil {
		return nil, fmt.Errorf("look up accounts: %w", err)
	}

	return accounts, nil
}
