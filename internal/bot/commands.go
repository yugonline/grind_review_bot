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

// registerCommands registers the bot's commands with Discord
func (b *Bot) registerCommands(ctx context.Context) error {
	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "add",
			Description: "Add a LeetCode problem you've solved",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "name",
					Description: "The name of the LeetCode problem",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "link",
					Description: "Optional link to the problem",
					Required:    false,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "difficulty",
					Description: "Difficulty of the problem",
					Required:    true,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{Name: database.DifficultyEasy, Value: database.DifficultyEasy},
						{Name: database.DifficultyMedium, Value: database.DifficultyMedium},
						{Name: database.DifficultyHard, Value: database.DifficultyHard},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "category",
					Description: "Category or topic of the problem",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "status",
					Description: "Status of the problem",
					Required:    true,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{Name: database.StatusSolved, Value: database.StatusSolved},
						{Name: database.StatusNeededHint, Value: database.StatusNeededHint},
						{Name: database.StatusStuck, Value: database.StatusStuck},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "solved_at",
					Description: "Date when you solved the problem (YYYY-MM-DD)",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "tags",
					Description: "Optional comma-separated tags for the problem",
					Required:    false,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "notes",
					Description: "Optional notes about the problem",
					Required:    false,
				},
			},
		},
		{
			Name:        "list",
			Description: "List your solved LeetCode problems",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "status",
					Description: "Filter by status",
					Required:    false,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{Name: database.StatusSolved, Value: database.StatusSolved},
						{Name: database.StatusNeededHint, Value: database.StatusNeededHint},
						{Name: database.StatusStuck, Value: database.StatusStuck},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "difficulty",
					Description: "Filter by difficulty",
					Required:    false,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{Name: database.DifficultyEasy, Value: database.DifficultyEasy},
						{Name: database.DifficultyMedium, Value: database.DifficultyMedium},
						{Name: database.DifficultyHard, Value: database.DifficultyHard},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "category",
					Description: "Filter by category",
					Required:    false,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "tags",
					Description: "Filter by comma-separated tags",
					Required:    false,
				},
			},
		},
		{
			Name:        "get",
			Description: "Get details of a solved problem by ID",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "id",
					Description: "The ID of the problem",
					Required:    true,
				},
			},
		},
		{
			Name:        "edit",
			Description: "Edit an existing LeetCode problem",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "id",
					Description: "The ID of the problem to edit",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "name",
					Description: "The name of the LeetCode problem",
					Required:    false,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "link",
					Description: "Optional link to the problem",
					Required:    false,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "difficulty",
					Description: "Difficulty of the problem",
					Required:    false,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{Name: database.DifficultyEasy, Value: database.DifficultyEasy},
						{Name: database.DifficultyMedium, Value: database.DifficultyMedium},
						{Name: database.DifficultyHard, Value: database.DifficultyHard},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "category",
					Description: "Category or topic of the problem",
					Required:    false,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "status",
					Description: "Status of the problem",
					Required:    false,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{Name: database.StatusSolved, Value: database.StatusSolved},
						{Name: database.StatusNeededHint, Value: database.StatusNeededHint},
						{Name: database.StatusStuck, Value: database.StatusStuck},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "solved_at",
					Description: "Date when you solved the problem (YYYY-MM-DD)",
					Required:    false,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "tags",
					Description: "Optional comma-separated tags for the problem",
					Required:    false,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "notes",
					Description: "Optional notes about the problem",
					Required:    false,
				},
			},
		},
		{
			Name:        "delete",
			Description: "Delete a solved problem by ID",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "id",
					Description: "The ID of the problem to delete",
					Required:    true,
				},
			},
		},
		{
			Name:        "stats",
			Description: "View your LeetCode problem solving statistics",
		},
	}

	registeredCommands := make([]*discordgo.ApplicationCommand, len(commands))
	for i, cmd := range commands {
		cmd.Version = "1.0.0" // Example version
		if b.cfg.GuildID == "" {
			registered, err := b.session.ApplicationCommandCreate(b.session.State.User.ID, "", cmd)
			if err != nil {
				return fmt.Errorf("failed to register global command '%s': %w", cmd.Name, err)
			}
			registeredCommands[i] = registered
			log.Info().Str("command", cmd.Name).Msg("Registered global command")
		} else {
			registered, err := b.session.ApplicationCommandCreate(b.session.State.User.ID, b.cfg.GuildID, cmd)
			if err != nil {
				return fmt.Errorf("failed to register guild command '%s': %w", cmd.Name, err)
			}
			registeredCommands[i] = registered
			log.Info().Str("command", cmd.Name).Str("guild_id", b.cfg.GuildID).Msg("Registered guild command")
		}
	}

	return nil
}

// deleteCommands deletes all registered commands (useful for development)
func (b *Bot) deleteCommands() {
	if b.cfg.GuildID == "" {
		cmds, err := b.session.ApplicationCommands(b.session.State.User.ID, "")
		if err != nil {
			log.Error().Err(err).Msg("Failed to get global commands")
			return
		}
		for _, cmd := range cmds {
			err := b.session.ApplicationCommandDelete(b.session.State.User.ID, "", cmd.ID)
			if err != nil {
				log.Error().Err(err).Str("command", cmd.Name).Msg("Failed to delete global command")
			} else {
				log.Info().Str("command", cmd.Name).Msg("Deleted global command")
			}
		}
	} else {
		cmds, err := b.session.ApplicationCommands(b.session.State.User.ID, b.cfg.GuildID)
		if err != nil {
			log.Error().Err(err).Str("guild_id", b.cfg.GuildID).Msg("Failed to get guild commands")
			return
		}
		for _, cmd := range cmds {
			err := b.session.ApplicationCommandDelete(b.session.State.User.ID, b.cfg.GuildID, cmd.ID)
			if err != nil {
				log.Error().Err(err).Str("command", cmd.Name).Str("guild_id", b.cfg.GuildID).Msg("Failed to delete guild command")
			} else {
				log.Info().Str("command", cmd.Name).Str("guild_id", b.cfg.GuildID).Msg("Deleted guild command")
			}
		}
	}
}

// interactionCreate is the handler for all incoming Discord interactions
func (b *Bot) interactionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		resChan := make(chan *discordgo.InteractionResponse)
		errChan := make(chan error)
		timeout := time.NewTimer(b.cfg.CommandsTimeout)

		go func() {
			handler, ok := b.commandHandlers[i.ApplicationCommandData().Name]
			if !ok {
				errChan <- fmt.Errorf("unknown command: %s", i.ApplicationCommandData().Name)
				return
			}
			response, err := handler(s, i)
			if err != nil {
				errChan <- err
				return
			}
			resChan <- response
		}()

		select {
		case res := <-resChan:
			err := s.InteractionRespond(i.Interaction, res)
			if err != nil {
				log.Error().Err(err).Str("command", i.ApplicationCommandData().Name).Msg("Failed to respond to interaction")
			}
		case err := <-errChan:
			log.Error().Err(err).Str("command", i.ApplicationCommandData().Name).Msg("Error handling command")
			b.sendErrorResponse(s, i, "An error occurred while processing your command.")
		case <-timeout.C:
			log.Warn().Str("command", i.ApplicationCommandData().Name).Msg("Command timed out")
			b.sendErrorResponse(s, i, "Command processing timed out.")
		}
		timeout.Stop()
	}
}

func (b *Bot) sendErrorResponse(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: message,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to send error response")
	}
}
