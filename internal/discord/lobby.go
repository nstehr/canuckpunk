package discord

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
)

// The lobby is the one room a new player can see before anything is opened to
// them. It carries the only entry point that needs no command registration and
// survives every restart.
const lobbyText = "## The Survey Office\n\n" +
	"The telegraph desk is unattended. Press **Begin** and it will wake up.\n\n" +
	"Everything after this happens in a direct message."

// How far back to look for a lobby message we posted before.
const lobbyScanLimit = 50

// PostLobby puts the entry message in the configured start channel. Running it
// again edits the message it left last time rather than posting another, so a
// half-finished attempt is fixed by repeating it.
func (b *Bot) PostLobby(ctx context.Context) error {
	if b.cfg.StartChannelID == "" {
		return errors.New("no start channel configured: set DISCORD_START_CHANNEL_ID")
	}

	channelID, err := snowflake.Parse(b.cfg.StartChannelID)
	if err != nil {
		return fmt.Errorf("parse start channel id %q: %w", b.cfg.StartChannelID, err)
	}

	client, err := disgo.New(b.cfg.Token, bot.WithDefaultGateway())
	if err != nil {
		return fmt.Errorf("create discord client: %w", err)
	}

	defer client.Close(ctx)

	message := discord.MessageCreate{
		Content: lobbyText,
		Components: []discord.LayoutComponent{
			discord.NewActionRow(discord.NewPrimaryButton("Begin", lobbyCustomID)),
		},
	}

	existing, err := findLobbies(client, channelID)
	if err != nil {
		return err
	}

	if len(existing) == 0 {
		posted, err := client.Rest.CreateMessage(channelID, message)
		if err != nil {
			return fmt.Errorf("post lobby message: %w", err)
		}

		slog.Info("Posted the lobby message", "channel", b.cfg.StartChannelID)

		return pin(client, channelID, posted.ID)
	}

	// Keep the newest and clear out whatever an earlier run left behind, so the
	// channel ends up with exactly one Begin button however often this is run.
	keep, stale := existing[0], existing[1:]

	_, err = client.Rest.UpdateMessage(channelID, keep, discord.MessageUpdate{
		Content:    &message.Content,
		Components: &message.Components,
	})
	if err != nil {
		return fmt.Errorf("update lobby message: %w", err)
	}

	slog.Info("Updated the lobby message", "channel", b.cfg.StartChannelID)

	for _, id := range stale {
		// Deleting our own message needs no special permission.
		if err := client.Rest.DeleteMessage(channelID, id); err != nil {
			slog.Warn("Could not remove an older lobby message", "message", id, "error", err)
		}
	}

	return pin(client, channelID, keep)
}

// findLobbies returns the lobby messages this bot has already posted, newest
// first. It matches on the application id rather than client.ID, which is only
// populated once a gateway has connected — and this command never opens one.
func findLobbies(client *bot.Client, channelID snowflake.ID) ([]snowflake.ID, error) {
	messages, err := client.Rest.GetMessages(channelID, 0, 0, 0, lobbyScanLimit)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", channelID, err)
	}

	var found []snowflake.ID

	for _, m := range messages {
		if m.Author.ID == client.ApplicationID && hasLobbyButton(m) {
			found = append(found, m.ID)
		}
	}

	return found, nil
}

func hasLobbyButton(m discord.Message) bool {
	for _, layout := range m.Components {
		row, ok := layout.(discord.ActionRowComponent)
		if !ok {
			continue
		}

		for component := range row.SubComponents() {
			button, isButton := component.(discord.ButtonComponent)
			if isButton && button.CustomID == lobbyCustomID {
				return true
			}
		}
	}

	return false
}

// pin is a convenience, not a requirement: the button works either way. It
// needs Manage Messages, so a server that has not granted it should still end
// up with a working lobby.
func pin(client *bot.Client, channelID, messageID snowflake.ID) error {
	if err := client.Rest.PinMessage(channelID, messageID); err != nil {
		slog.Warn("Could not pin the lobby message; it still works unpinned",
			"error", err, "hint", "grant Manage Messages to pin it, or pin it by hand")
	}

	return nil
}
