package bot

import (
	"context"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog/log"
	"github.com/yugonline/grind_review_bot/config"
	"github.com/yugonline/grind_review_bot/internal/database"
)

// Bot represents the Discord bot
type Bot struct {
	session         *discordgo.Session
	repo            *database.Repository
	cfg             config.DiscordConfig
	reviewChannelID string // ID of the channel where commands are allowed
	commandHandlers map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate) (*discordgo.InteractionResponse, error)
}

// New creates a new Discord bot instance
func New(ctx context.Context, cfg config.DiscordConfig, repo *database.Repository) (*Bot, error) {
	// Create Discord session
	session, err := discordgo.New("Bot " + cfg.Token)
	if err != nil {
		return nil, fmt.Errorf("failed to create Discord session: %w", err)
	}

	// Create bot instance
	bot := &Bot{
		session:         session,
		repo:            repo,
		cfg:             cfg,
		reviewChannelID: cfg.ReviewChannelID,
	}

	// Register command handlers
	bot.registerCommandHandlers()

	// Add handlers for Discord events
	session.AddHandler(bot.interactionCreate)
	session.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		log.Info().Str("username", s.State.User.Username).Str("id", s.State.User.ID).Msg("Bot is ready")
	})

	// Identify with intents
	session.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsGuilds | discordgo.IntentsGuildMembers

	return bot, nil
}

// Start starts the Discord bot
func (b *Bot) Start(ctx context.Context) error {
	// Connect to Discord
	if err := b.session.Open(); err != nil {
		return fmt.Errorf("failed to connect to Discord: %w", err)
	}

	// Register slash commands
	if err := b.registerCommands(); err != nil {
		return fmt.Errorf("failed to register commands: %w", err)
	}

	return nil
}

// Shutdown gracefully shuts down the bot
func (b *Bot) Shutdown(ctx context.Context) error {
	// Unregister commands if needed and close session
	if err := b.unregisterCommands(); err != nil {
		log.Warn().Err(err).Msg("Failed to unregister commands during shutdown")
	}

	return b.session.Close()
}

// interactionCreate handles Discord interactions (slash commands)
func (b *Bot) interactionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Validate interaction type
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}

	// Check if interaction is in the review channel (if configured)
	if b.reviewChannelID != "" && i.ChannelID != b.reviewChannelID {
		response := &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("Please use commands in the <#%s> channel.", b.reviewChannelID),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		}
		s.InteractionRespond(i.Interaction, response)
		return
	}
	
	// Verify user is a member of this server
	if !b.isServerMember(i.Member.User.ID) {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You must be a member of this server to use commands.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	// Get command name and find handler
	cmdName := i.ApplicationCommandData().Name
	handler, ok := b.commandHandlers[cmdName]
	if !ok {
		log.Error().Str("command", cmdName).Msg("No handler for command")
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Unknown command. Please try again.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	// Get start time for metrics
	log.Debug().Str("command", cmdName).Str("user", i.Member.User.Username).Msg("Command received")

	// Execute handler
	response, err := handler(s, i)
	if err != nil {
		log.Error().Err(err).Str("command", cmdName).Msg("Error handling command")
		
		// Send error response if one wasn't already provided
		if response == nil {
			response = &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Error processing command: " + err.Error(),
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			}
		}
	}

	// Respond to the interaction
	if err := s.InteractionRespond(i.Interaction, response); err != nil {
		log.Error().Err(err).Str("command", cmdName).Msg("Failed to respond to interaction")
	}
}

// registerCommands registers slash commands with Discord
func (b *Bot) registerCommands() error {
	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "add",
			Description: "Add a solved problem to your review list",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "name",
					Description: "Problem name",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "difficulty",
					Description: "Problem difficulty",
					Required:    true,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{
							Name:  "Easy",
							Value: "Easy",
						},
						{
							Name:  "Medium",
							Value: "Medium",
						},
						{
							Name:  "Hard",
							Value: "Hard",
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "category",
					Description: "Problem category/topic",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "status",
					Description: "How did you solve it?",
					Required:    true,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{
							Name:  "Solved",
							Value: "Solved",
						},
						{
							Name:  "Needed Hint",
							Value: "Needed Hint",
						},
						{
							Name:  "Stuck",
							Value: "Stuck",
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "solved_at",
					Description: "Date you solved it (YYYY-MM-DD)",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "link",
					Description: "Link to the problem",
					Required:    false,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "tags",
					Description: "Tags, comma separated (e.g. 'dp,recursion,trees')",
					Required:    false,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "notes",
					Description: "Your notes about the problem",
					Required:    false,
				},
			},
		},
		{
			Name:        "list",
			Description: "List your solved problems",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "status",
					Description: "Filter by status",
					Required:    false,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{
							Name:  "Solved",
							Value: "Solved",
						},
						{
							Name:  "Needed Hint",
							Value: "Needed Hint",
						},
						{
							Name:  "Stuck",
							Value: "Stuck",
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "difficulty",
					Description: "Filter by difficulty",
					Required:    false,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{
							Name:  "Easy",
							Value: "Easy",
						},
						{
							Name:  "Medium",
							Value: "Medium",
						},
						{
							Name:  "Hard",
							Value: "Hard",
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "category",
					Description: "Filter by category/topic",
					Required:    false,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "tags",
					Description: "Filter by tags, comma separated (e.g. 'dp,recursion')",
					Required:    false,
				},
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "limit",
					Description: "Maximum number of problems to show",
					Required:    false,
					MinValue:    &[]float64{1}[0],
					MaxValue:    50,
				},
			},
		},
		{
			Name:        "get",
			Description: "Get details of a specific problem",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "id",
					Description: "Problem ID",
					Required:    true,
					MinValue:    &[]float64{1}[0],
				},
			},
		},
		{
			Name:        "edit",
			Description: "Edit a problem entry",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "id",
					Description: "Problem ID",
					Required:    true,
					MinValue:    &[]float64{1}[0],
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "name",
					Description: "Problem name",
					Required:    false,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "difficulty",
					Description: "Problem difficulty",
					Required:    false,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{
							Name:  "Easy",
							Value: "Easy",
						},
						{
							Name:  "Medium",
							Value: "Medium",
						},
						{
							Name:  "Hard",
							Value: "Hard",
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "category",
					Description: "Problem category/topic",
					Required:    false,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "status",
					Description: "How did you solve it?",
					Required:    false,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{
							Name:  "Solved",
							Value: "Solved",
						},
						{
							Name:  "Needed Hint",
							Value: "Needed Hint",
						},
						{
							Name:  "Stuck",
							Value: "Stuck",
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "solved_at",
					Description: "Date you solved it (YYYY-MM-DD)",
					Required:    false,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "link",
					Description: "Link to the problem",
					Required:    false,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "tags",
					Description: "Tags, comma separated (e.g. 'dp,recursion,trees')",
					Required:    false,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "notes",
					Description: "Your notes about the problem",
					Required:    false,
				},
			},
		},
		{
			Name:        "delete",
			Description: "Delete a problem entry",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "id",
					Description: "Problem ID",
					Required:    true,
					MinValue:    &[]float64{1}[0],
				},
			},
		},
	}

	for _, command := range commands {
		_, err := b.session.ApplicationCommandCreate(b.session.State.User.ID, b.cfg.GuildID, command)
		if err != nil {
			return fmt.Errorf("failed to create command %s: %w", command.Name, err)
		}
	}

	return nil
}

// unregisterCommands removes all slash commands from Discord
func (b *Bot) unregisterCommands() error {
	// Get all commands
	commands, err := b.session.ApplicationCommands(b.session.State.User.ID, b.cfg.GuildID)
	if err != nil {
		return fmt.Errorf("failed to get commands: %w", err)
	}

	// Delete each command
	for _, cmd := range commands {
		err := b.session.ApplicationCommandDelete(b.session.State.User.ID, b.cfg.GuildID, cmd.ID)
		if err != nil {
			return fmt.Errorf("failed to delete command %s: %w", cmd.Name, err)
		}
	}

	return nil
}