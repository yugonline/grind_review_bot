package bot

import (
	"github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog/log"
)

// Middleware for handling errors during command execution.
// This is a placeholder and can be expanded with more sophisticated error handling.
func (b *Bot) errorMiddleware(next func(s *discordgo.Session, i *discordgo.InteractionCreate) (*discordgo.InteractionResponse, error)) func(s *discordgo.Session, i *discordgo.InteractionCreate) (*discordgo.InteractionResponse, error) {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) (*discordgo.InteractionResponse, error) {
		response, err := next(s, i)
		if err != nil {
			log.Error().Err(err).Str("command", i.ApplicationCommandData().Name).Msg("Error during command execution")
			// Optionally send a user-friendly error message back to Discord
			return &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "An unexpected error occurred while processing your command.",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			}, nil // Return nil error to prevent further propagation if already handled
		}
		return response, nil
	}
}