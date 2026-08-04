// Package discord serves the game over Discord: a direct message is the
// player's desk, the way an SSH connection is in the terminal front end.
//
// Everything player-facing comes from the shared core — onboarding writes the
// prose and supplies the choices, and this package only decides how they look
// as messages and buttons.
package discord

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/gateway"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/snowflake/v2"

	"github.com/nstehr/canuckpunk/internal/menu"
	"github.com/nstehr/canuckpunk/internal/onboarding"
	"github.com/nstehr/canuckpunk/internal/session"
	"github.com/nstehr/canuckpunk/internal/state"
)

// Config is what the bot needs to run. GuildID is optional: with one, slash
// commands register to that guild and appear instantly, which is what makes
// the dev loop bearable. Without one they register globally and can take up
// to an hour to propagate.
type Config struct {
	Token          string
	GuildID        string
	StartChannelID string

	Accounts onboarding.Accounts
	Prose    onboarding.Narratives
}

// Bot is the Discord front end.
type Bot struct {
	cfg     Config
	chooser menu.Source
	convos  *conversations
	client  *bot.Client
}

// New builds a Bot. It does not connect; Run does that.
func New(cfg Config) (*Bot, error) {
	if cfg.Token == "" {
		return nil, errors.New("no discord token")
	}

	b := &Bot{
		cfg:     cfg,
		chooser: onboarding.NewChooser(cfg.Accounts),
	}

	b.convos = newConversations(conversationTTL, func() *state.Machine {
		return state.New(onboarding.Start(cfg.Accounts, cfg.Prose))
	})

	return b, nil
}

// Run connects and serves until ctx is cancelled.
func (b *Bot) Run(ctx context.Context) error {
	mux := handler.New()
	b.routes(mux)

	client, err := disgo.New(b.cfg.Token,
		// Interactions arrive regardless of intents. Direct messages are the
		// one thing we do need, so a player can type instead of clicking —
		// message content in a DM is not privileged.
		bot.WithGatewayConfigOpts(gateway.WithIntents(gateway.IntentDirectMessages)),
		bot.WithEventListeners(mux),
		bot.WithEventListenerFunc(b.onMessage),
	)
	if err != nil {
		return fmt.Errorf("create discord client: %w", err)
	}

	b.client = client

	defer client.Close(context.WithoutCancel(ctx))

	if err := client.OpenGateway(ctx); err != nil {
		return fmt.Errorf("open discord gateway: %w", err)
	}

	if err := b.registerCommands(); err != nil {
		return err
	}

	slog.Info("Discord client connected", "guild", b.cfg.GuildID)

	<-ctx.Done()
	slog.Info("Stopping Discord client")

	return nil
}

// registerCommands installs the one slash command. Guild-scoped when a guild
// is configured, so a changed definition is live immediately.
func (b *Bot) registerCommands() error {
	commands := []discord.ApplicationCommandCreate{
		discord.SlashCommandCreate{
			Name:        "begin",
			Description: "Sit down at the telegraph desk",
		},
	}

	appID := b.client.ApplicationID

	if b.cfg.GuildID == "" {
		if _, err := b.client.Rest.SetGlobalCommands(appID, commands); err != nil {
			return fmt.Errorf("register global commands: %w", err)
		}

		return nil
	}

	guildID, err := snowflake.Parse(b.cfg.GuildID)
	if err != nil {
		return fmt.Errorf("parse guild id %q: %w", b.cfg.GuildID, err)
	}

	if _, err := b.client.Rest.SetGuildCommands(appID, guildID, commands); err != nil {
		// The bot scope alone is enough to connect but not to own commands in
		// a guild, so this is almost always a missing applications.commands
		// scope rather than anything about the guild itself.
		return fmt.Errorf("register commands in guild %s (is the bot invited with the "+
			"applications.commands scope?): %w", b.cfg.GuildID, err)
	}

	return nil
}

// userSession describes a Discord player to the core. The credential is their
// user id: it is what the client proved, and what accounts are found by.
func userSession(u discord.User) session.UserSession {
	return session.UserSession{
		ID:          u.ID.String(),
		Username:    u.Username,
		Client:      session.ClientDiscord,
		Credential:  session.Credential{ID: "discord:" + u.ID.String()},
		ConnectedAt: time.Now(),
	}
}

// send delivers a screen to a channel, in order.
func (b *Bot) send(_ context.Context, channelID snowflake.ID, s screen) error {
	for _, msg := range messages(s) {
		if _, err := b.client.Rest.CreateMessage(channelID, msg); err != nil {
			return fmt.Errorf("send message: %w", err)
		}
	}

	return nil
}

// dmChannel opens the player's direct message channel. It fails when they do
// not accept messages from server members, which is common enough that the
// caller has to say so rather than swallow it.
func (b *Bot) dmChannel(_ context.Context, userID snowflake.ID) (snowflake.ID, error) {
	ch, err := b.client.Rest.CreateDMChannel(userID)
	if err != nil {
		return 0, fmt.Errorf("open dm: %w", err)
	}

	return ch.ID(), nil
}

// onMessage handles a player typing in their DM, which is how they answer a
// prompt or take an option by name rather than by button.
func (b *Bot) onMessage(e *events.MessageCreate) {
	if e.Message.Author.Bot || e.Message.GuildID != nil {
		return
	}

	input := e.Message.Content
	if input == "" {
		return
	}

	ctx := context.Background()
	userID := e.Message.Author.ID.String()

	if s, ok := b.global(userID, input); ok {
		b.reply(ctx, e.ChannelID, s)

		return
	}

	c := b.convos.get(userID, userSession(e.Message.Author))

	s, err := b.advance(ctx, c, input)
	if err != nil {
		slog.Error("discord turn failed", "user", userID, "error", err)

		return
	}

	b.reply(ctx, e.ChannelID, s)
}

func (b *Bot) reply(ctx context.Context, channelID snowflake.ID, s screen) {
	if err := b.send(ctx, channelID, s); err != nil {
		slog.Error("discord send failed", "error", err)
	}
}
