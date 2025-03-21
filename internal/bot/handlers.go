package bot

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog/log"
	"github.com/yugonline/grind_review_bot/internal/database"
)

// Error constants
var (
	ErrInvalidDateFormat = fmt.Errorf("invalid date format, please use YYYY-MM-DD")
)

func (b *Bot) registerCommandHandlers() {
	b.commandHandlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate) (*discordgo.InteractionResponse, error){
		"add":    b.handleAddCommand,
		"list":   b.handleListCommand,
		"get":    b.handleGetCommand,
		"edit":   b.handleEditCommand,
		"delete": b.handleDeleteCommand,
	}
}

func (b *Bot) handleAddCommand(s *discordgo.Session, i *discordgo.InteractionCreate) (*discordgo.InteractionResponse, error) {
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	solvedAtStr, ok := optionMap["solved_at"]
	if !ok || solvedAtStr.StringValue() == "" {
		return errorResponse("Missing or invalid solved_at date."), nil
	}
	solvedAt, err := time.Parse("2006-01-02", solvedAtStr.StringValue())
	if err != nil {
		return errorResponse(ErrInvalidDateFormat.Error()), nil
	}

	// Initialize problem with required fields
	problem := &database.ProblemEntry{
		UserID:      i.Member.User.ID,
		ProblemName: optionMap["name"].StringValue(),
		Difficulty:  optionMap["difficulty"].StringValue(),
		Category:    optionMap["category"].StringValue(),
		Status:      optionMap["status"].StringValue(),
		SolvedAt:    solvedAt,
		Link:        "", // Default empty string for optional fields
		Notes:       "",
		Tags:        make([]string, 0),
	}

	// Add optional fields if they exist
	if linkOpt, ok := optionMap["link"]; ok {
		problem.Link = linkOpt.StringValue()
	}

	if notesOpt, ok := optionMap["notes"]; ok {
		problem.Notes = notesOpt.StringValue()
	}

	if tagsOpt, ok := optionMap["tags"]; ok && tagsOpt.StringValue() != "" {
		tagStrings := strings.Split(tagsOpt.StringValue(), ",")
		for i := range tagStrings {
			tagStrings[i] = strings.TrimSpace(tagStrings[i])
		}
		problem.Tags = tagStrings
	}

	err = b.repo.CreateProblem(context.Background(), problem)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create problem")
		return errorResponse("Failed to add problem to the database."), nil
	}

	return messageResponse(fmt.Sprintf("Successfully added problem '%s'!", problem.ProblemName)), nil
}

func (b *Bot) handleListCommand(s *discordgo.Session, i *discordgo.InteractionCreate) (*discordgo.InteractionResponse, error) {
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	status := ""
	if statusOpt, ok := optionMap["status"]; ok {
		status = statusOpt.StringValue()
	}
	difficulty := ""
	if difficultyOpt, ok := optionMap["difficulty"]; ok {
		difficulty = difficultyOpt.StringValue()
	}
	category := ""
	if categoryOpt, ok := optionMap["category"]; ok {
		category = categoryOpt.StringValue()
	}

	limit := 10
	if limitOpt, ok := optionMap["limit"]; ok {
		limit = int(limitOpt.IntValue())
	}

	var tags []string
	if tagsOpt, ok := optionMap["tags"]; ok && tagsOpt.StringValue() != "" {
		tagStrings := strings.Split(tagsOpt.StringValue(), ",")
		for i := range tagStrings {
			tagStrings[i] = strings.TrimSpace(tagStrings[i])
		}
		tags = tagStrings
	}

	// Get problems
	problems, err := b.repo.ListProblems(
		context.Background(),
		i.Member.User.ID,
		status,
		difficulty,
		category,
		tags,
		limit,
		0, // No offset for simple listing
	)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list problems")
		return errorResponse("Failed to retrieve problems from the database."), nil
	}

	if len(problems) == 0 {
		return messageResponse("No problems found matching your criteria."), nil
	}

	// Format problems as a table
	var sb strings.Builder
	sb.WriteString("Your Problems:\n```\n")
	sb.WriteString(fmt.Sprintf("%-5s | %-30s | %-8s | %-15s | %-10s | %-20s\n", "ID", "Name", "Status", "Category", "Difficulty", "Solved At"))
	sb.WriteString(strings.Repeat("-", 100) + "\n")

	for _, p := range problems {
		sb.WriteString(fmt.Sprintf("%-5d | %-30s | %-8s | %-15s | %-10s | %-20s\n",
			p.ID,
			truncateString(p.ProblemName, 28),
			truncateString(p.Status, 8),
			truncateString(p.Category, 15),
			truncateString(p.Difficulty, 10),
			p.SolvedAt.Format("2006-01-02"),
		))
	}
	sb.WriteString("```")

	return messageResponse(sb.String()), nil
}

func (b *Bot) handleGetCommand(s *discordgo.Session, i *discordgo.InteractionCreate) (*discordgo.InteractionResponse, error) {
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	problemID := uint(optionMap["id"].IntValue())
	problem, err := b.repo.GetProblem(context.Background(), problemID)
	if err != nil {
		log.Error().Err(err).Uint("id", problemID).Msg("Failed to get problem")
		return errorResponse(fmt.Sprintf("Problem with ID %d not found or you don't have permission to view it.", problemID)), nil
	}

	// Check if the user is the owner of the problem
	if problem.UserID != i.Member.User.ID {
		return errorResponse("You don't have permission to view this problem."), nil
	}

	// Format problem details
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Problem: %s\n", problem.ProblemName))
	sb.WriteString(fmt.Sprintf("**Difficulty:** %s\n", problem.Difficulty))
	sb.WriteString(fmt.Sprintf("**Category:** %s\n", problem.Category))
	sb.WriteString(fmt.Sprintf("**Status:** %s\n", problem.Status))
	sb.WriteString(fmt.Sprintf("**Solved On:** %s\n", problem.SolvedAt.Format("2006-01-02")))

	if problem.Link != "" {
		sb.WriteString(fmt.Sprintf("**Link:** %s\n", problem.Link))
	}

	if len(problem.Tags) > 0 {
		sb.WriteString(fmt.Sprintf("**Tags:** %s\n", strings.Join(problem.Tags, ", ")))
	}

	if problem.LastReviewedAt != nil {
		sb.WriteString(fmt.Sprintf("**Last Reviewed:** %s\n", problem.LastReviewedAt.Format("2006-01-02")))
		sb.WriteString(fmt.Sprintf("**Review Count:** %d\n", problem.ReviewCount))
	} else {
		sb.WriteString("**Last Reviewed:** Never\n")
		sb.WriteString("**Review Count:** 0\n")
	}

	if problem.Notes != "" {
		sb.WriteString("\n**Notes:**\n")
		sb.WriteString(problem.Notes)
	}

	return messageResponse(sb.String()), nil
}

func (b *Bot) handleEditCommand(s *discordgo.Session, i *discordgo.InteractionCreate) (*discordgo.InteractionResponse, error) {
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	problemID := uint(optionMap["id"].IntValue())

	// Get the existing problem
	existing, err := b.repo.GetProblem(context.Background(), problemID)
	if err != nil {
		log.Error().Err(err).Uint("id", problemID).Msg("Failed to get problem for editing")
		return errorResponse(fmt.Sprintf("Problem with ID %d not found or you don't have permission to edit it.", problemID)), nil
	}

	// Check if the user is the owner of the problem
	if existing.UserID != i.Member.User.ID {
		return errorResponse("You don't have permission to edit this problem."), nil
	}

	// Update fields that are specified
	if nameOpt, ok := optionMap["name"]; ok {
		existing.ProblemName = nameOpt.StringValue()
	}
	if difficultyOpt, ok := optionMap["difficulty"]; ok {
		existing.Difficulty = difficultyOpt.StringValue()
	}
	if categoryOpt, ok := optionMap["category"]; ok {
		existing.Category = categoryOpt.StringValue()
	}
	if statusOpt, ok := optionMap["status"]; ok {
		existing.Status = statusOpt.StringValue()
	}
	if linkOpt, ok := optionMap["link"]; ok {
		existing.Link = linkOpt.StringValue()
	}
	if notesOpt, ok := optionMap["notes"]; ok {
		existing.Notes = notesOpt.StringValue()
	}
	if tagsOpt, ok := optionMap["tags"]; ok {
		tagStrings := strings.Split(tagsOpt.StringValue(), ",")
		existing.Tags = make([]string, 0, len(tagStrings))
		for i := range tagStrings {
			tag := strings.TrimSpace(tagStrings[i])
			if tag != "" {
				existing.Tags = append(existing.Tags, tag)
			}
		}
	}
	if solvedAtOpt, ok := optionMap["solved_at"]; ok {
		solvedAt, err := time.Parse("2006-01-02", solvedAtOpt.StringValue())
		if err != nil {
			return errorResponse(ErrInvalidDateFormat.Error()), nil
		}
		existing.SolvedAt = solvedAt
	}

	// Update the problem
	if err := b.repo.UpdateProblem(context.Background(), existing); err != nil {
		log.Error().Err(err).Uint("id", problemID).Msg("Failed to update problem")
		return errorResponse("Failed to update problem in the database."), nil
	}

	return messageResponse(fmt.Sprintf("Successfully updated problem '%s'!", existing.ProblemName)), nil
}

func (b *Bot) handleDeleteCommand(s *discordgo.Session, i *discordgo.InteractionCreate) (*discordgo.InteractionResponse, error) {
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	problemID := uint(optionMap["id"].IntValue())

	// Get the problem to verify ownership
	problem, err := b.repo.GetProblem(context.Background(), problemID)
	if err != nil {
		log.Error().Err(err).Uint("id", problemID).Msg("Failed to get problem for deletion")
		return errorResponse(fmt.Sprintf("Problem with ID %d not found or you don't have permission to delete it.", problemID)), nil
	}

	// Check if the user is the owner of the problem
	if problem.UserID != i.Member.User.ID {
		return errorResponse("You don't have permission to delete this problem."), nil
	}

	// Delete the problem
	if err := b.repo.DeleteProblem(context.Background(), problemID); err != nil {
		log.Error().Err(err).Uint("id", problemID).Msg("Failed to delete problem")
		return errorResponse("Failed to delete problem from the database."), nil
	}

	return messageResponse(fmt.Sprintf("Successfully deleted problem '%s'!", problem.ProblemName)), nil
}

// Helper functions

// truncateString truncates a string to max length and adds ellipsis if needed
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// errorResponse creates a ephemeral error response
func errorResponse(content string) *discordgo.InteractionResponse {
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Error: " + content,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	}
}

// messageResponse creates a standard message response
func messageResponse(content string) *discordgo.InteractionResponse {
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
		},
	}
}