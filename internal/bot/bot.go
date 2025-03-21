package bot

import (
	"context"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog/log"
	"github.com/yugonline/grind_review_bot/config"
	"github.com/yugonline/grind_review_bot/internal/database"
)

// Bot represents the Discord bot
type Bot struct {
	session         *discordgo.Session
	db              *database.DB
	cfg             config.DiscordConfig
	commandHandlers map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate) (*discordgo.InteractionResponse, error)
}

// New creates a new Discord bot instance
func New(ctx context.Context, cfg config.DiscordConfig, db *database.DB) (*Bot, error) {
	// Create Discord session
	session, err := discordgo.New("Bot " + cfg.Token)
	if err != nil {
		return nil, fmt.Errorf("failed to create Discord session: %w", err)
	}

	// Create bot instance
	bot := &Bot{
		session: session,
		db:      db,
		cfg:     cfg,
	}

	// Register command handlers
	bot.registerCommandHandlers()

	// Add handlers for Discord events
	session.AddHandler(bot.interactionCreate)
	session.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		log.Info().Str("username", s.State.User.Username).Str("id", s.State.User.ID).Msg("Bot is ready")
	})

	// Identify with intents
	session.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsGuilds

	return bot, nil
}

// Start connects to Discord and registers commands
func (b *Bot) Start(ctx context.Context) error {
	// Connect to Discord
	if err := b.session.Open(); err != nil {
		return fmt.Errorf("failed to connect to Discord: %w", err)
	}

	// Register commands
	if err := b.registerCommands(ctx); err != nil {
		return fmt.Errorf("failed to register commands: %w", err)
	}

	log.Info().Msg("Bot connected to Discord")
	return nil
}

// Shutdown gracefully shuts down the bot
func (b *Bot) Shutdown(ctx context.Context) error {
	log.Info().Msg("Shutting down Discord connection...")
	return b.session.Close()
}
