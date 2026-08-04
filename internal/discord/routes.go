package discord

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/disgo/rest"
)

// The lobby button lives on a pinned message that outlives every restart, so
// its id is part of the contract rather than something generated per session.
const lobbyCustomID = "/start/begin"

func (b *Bot) routes(mux *handler.Mux) {
	mux.SlashCommand("/begin", b.onBegin)
	mux.ButtonComponent(lobbyCustomID, b.onLobby)
	mux.ButtonComponent(choiceRoute+"{id}", b.onChoice)
	mux.SelectMenuComponent(choiceRoute, b.onSelect)
}

// onBegin and onLobby are the two ways in. Both start a clean conversation:
// somebody pressing Begin again means they want to start over.
func (b *Bot) onBegin(_ discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	return b.openDesk(e.Ctx, e, e.User())
}

func (b *Bot) onLobby(_ discord.ButtonInteractionData, e *handler.ComponentEvent) error {
	return b.openDesk(e.Ctx, e, e.User())
}

// replier is the part of an interaction the entry points need. Narrowing it
// lets one function serve both the slash command and the lobby button.
type replier interface {
	CreateMessage(discord.MessageCreate, ...rest.RequestOpt) error
}

// openDesk answers in the channel the player pressed in, then moves the game
// to their DM.
func (b *Bot) openDesk(ctx context.Context, e replier, u discord.User) error {
	channelID, err := b.dmChannel(ctx, u.ID)
	if err != nil {
		return e.CreateMessage(ephemeral(
			"I can't send you a direct message. Turn on **Allow direct messages " +
				"from server members** for this server, then press Begin again."))
	}

	c := b.convos.fresh(u.ID.String(), userSession(u))

	s, err := b.advance(ctx, c, "")
	if err != nil {
		return fmt.Errorf("open desk: %w", err)
	}

	if err := b.send(ctx, channelID, s); err != nil {
		return err
	}

	return e.CreateMessage(ephemeral("The desk is open — check your direct messages."))
}

// onChoice handles a button press. The clicked message loses its buttons, so
// an old screen cannot be replayed a week later.
func (b *Bot) onChoice(_ discord.ButtonInteractionData, e *handler.ComponentEvent) error {
	return b.take(e, choiceID(e.Vars["id"]))
}

func (b *Bot) onSelect(data discord.SelectMenuInteractionData, e *handler.ComponentEvent) error {
	// Only string menus are registered on this route; anything else is a
	// component we did not send.
	selected, ok := data.(discord.StringSelectMenuInteractionData)
	if !ok || len(selected.Values) == 0 {
		return nil
	}

	return b.take(e, choiceFromValue(selected.Values[0]))
}

func (b *Bot) take(e *handler.ComponentEvent, id string) error {
	ctx := e.Ctx

	if err := e.UpdateMessage(discord.MessageUpdate{
		Components: &[]discord.LayoutComponent{},
	}); err != nil {
		slog.Warn("could not clear buttons", "error", err)
	}

	c := b.convos.get(e.User().ID.String(), userSession(e.User()))

	s, err := b.advance(ctx, c, id)
	if err != nil {
		return fmt.Errorf("take choice: %w", err)
	}

	return b.send(ctx, e.Channel().ID(), s)
}

func ephemeral(text string) discord.MessageCreate {
	return discord.MessageCreate{Content: text, Flags: discord.MessageFlagEphemeral}
}
