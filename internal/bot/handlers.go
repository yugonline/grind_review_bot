package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog/log"
	"github.com/yugonline/grind_review_bot/internal/database"
)

// commandHandlers holds the mapping of command names to their handler functions
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
		"stats":  b.handleStatsCommand,
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

	problem := &database.ProblemEntry{
		UserID:      i.Member.User.ID,
		ProblemName: optionMap["name"].StringValue(),
		Link:        optionMap["link"].StringValue(),
		Difficulty:  optionMap["difficulty"].StringValue(),
		Category:    optionMap["category"].StringValue(),
		Status:      optionMap["status"].StringValue(),
		SolvedAt:    solvedAt,
		Notes:       optionMap["notes"].StringValue(),
	}

	if tagsOpt, ok := optionMap["tags"]; ok && tagsOpt.StringValue() != "" {
		tagStrings := strings.Split(tagsOpt.StringValue(), ",")
		for i := range tagStrings {
			tagStrings[i] = strings.TrimSpace(tagStrings[i])
		}
		problem.Tags = tagStrings
	}

	err = b.db.InsertProblem(context.Background(), problem)
	if err != nil {
		log.Error().Err(err).Msg("Failed to insert problem")
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

	var tags []string
	if tagsOpt, ok := optionMap["tags"]; ok && tagsOpt.StringValue() != "" {
		tagStrings := strings.Split(tagsOpt.StringValue(), ",")
		for i := range tagStrings {
			tagStrings[i] = strings.TrimSpace(tagStrings[i])
		}
		tags = tagStrings
	}

	problems, err := b.db.ListProblems(context.Background(), i.Member.User.ID, status, difficulty, category, tags, 0, 0)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list problems")
		return errorResponse("Failed to retrieve your problem list."), nil
	}

	if len(problems) == 0 {
		return messageResponse("You haven't added any problems yet, or no problems match your filter."), nil
	}

	var sb strings.Builder
	sb.WriteString("Your Solved LeetCode Problems:\n")
	for _, p := range problems {
		sb.WriteString(fmt.Sprintf("- ID: %d, Name: %s, Difficulty: %s, Status: %s, Solved At: %s",
			p.ID, p.ProblemName, p.Difficulty, p.Status, p.SolvedAt.Format("2006-01-02")))
		if len(p.Tags) > 0 {
			sb.WriteString(fmt.Sprintf(", Tags: %s", strings.Join(p.Tags, ", ")))
		}
		sb.WriteString("\n")
	}

	return messageResponse(sb.String()), nil
}

func (b *Bot) handleGetCommand(s *discordgo.Session, i *discordgo.InteractionCreate) (*discordgo.InteractionResponse, error) {
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	idOpt, ok := optionMap["id"]
	if !ok {
		return errorResponse("Missing problem ID."), nil
	}
	problemID := int(idOpt.IntValue())

	problem, err := b.db.GetProblem(context.Background(), problemID)
	if err != nil {
		log.Error().Err(err).Int("problem_id", problemID).Msg("Failed to get problem")
		return errorResponse(fmt.Sprintf("Could not find problem with ID %d.", problemID)), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Problem ID: %d\n", problem.ID))
	sb.WriteString(fmt.Sprintf("Name: %s\n", problem.ProblemName))
	if problem.Link != "" {
		sb.WriteString(fmt.Sprintf("Link: %s\n", problem.Link))
	}
	sb.WriteString(fmt.Sprintf("Difficulty: %s\n", problem.Difficulty))
	sb.WriteString(fmt.Sprintf("Category: %s\n", problem.Category))
	sb.WriteString(fmt.Sprintf("Status: %s\n", problem.Status))
	sb.WriteString(fmt.Sprintf("Solved At: %s\n", problem.SolvedAt.Format("2006-01-02")))
	if problem.LastReviewedAt != nil {
		sb.WriteString(fmt.Sprintf("Last Reviewed At: %s\n", problem.LastReviewedAt.Format("2006-01-02")))
	}
	sb.WriteString(fmt.Sprintf("Review Count: %d\n", problem.ReviewCount))
	if problem.Notes != "" {
		sb.WriteString(fmt.Sprintf("Notes: %s\n", problem.Notes))
	}
	if len(problem.Tags) > 0 {
		sb.WriteString(fmt.Sprintf("Tags: %s\n", strings.Join(problem.Tags, ", ")))
	}

	return messageResponse(sb.String()), nil
}

func (b *Bot) handleEditCommand(s *discordgo.Session, i *discordgo.InteractionCreate) (*discordgo.InteractionResponse, error) {
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	idOpt, ok := optionMap["id"]
	if !ok {
		return errorResponse("Missing problem ID to edit."), nil
	}
	problemID := int(idOpt.IntValue())

	existingProblem, err := b.db.GetProblem(context.Background(), problemID)
	if err != nil {
		log.Error().Err(err).Int("problem_id", problemID).Msg("Failed to get problem for editing")
		return errorResponse(fmt.Sprintf("Could not find problem with ID %d to edit.", problemID)), nil
	}

	// Create a copy to avoid modifying the fetched problem directly
	updatedProblem := *existingProblem

	if nameOpt, ok := optionMap["name"]; ok {
		updatedProblem.ProblemName = nameOpt.StringValue()
	}
	if linkOpt, ok := optionMap["link"]; ok {
		updatedProblem.Link = linkOpt.StringValue()
	}
	if difficultyOpt, ok := optionMap["difficulty"]; ok {
		updatedProblem.Difficulty = difficultyOpt.StringValue()
	}
	if categoryOpt, ok := optionMap["category"]; ok {
		updatedProblem.Category = categoryOpt.StringValue()
	}
	if statusOpt, ok := optionMap["status"]; ok {
		updatedProblem.Status = statusOpt.StringValue()
	}
	if solvedAtOpt, ok := optionMap["solved_at"]; ok {
		solvedAt, err := time.Parse("2006-01-02", solvedAtOpt.StringValue())
		if err != nil {
			return errorResponse(ErrInvalidDateFormat.Error()), nil
		}
		updatedProblem.SolvedAt = solvedAt
	}
	if tagsOpt, ok := optionMap["tags"]; ok {
		tagStrings := strings.Split(tagsOpt.StringValue(), ",")
		var tags []string
		for i := range tagStrings {
			tagStrings[i] = strings.TrimSpace(tagStrings[i])
			if tagStrings[i] != "" {
				tags = append(tags, tagStrings[i])
			}
		}
		updatedProblem.Tags = tags
	}
	if notesOpt, ok := optionMap["notes"]; ok {
		updatedProblem.Notes = notesOpt.StringValue()
	}

	updatedProblem.UserID = i.Member.User.ID // Ensure user ID is correct

	err = b.db.UpdateProblem(context.Background(), &updatedProblem)
	if err != nil {
		log.Error().Err(err).Int("problem_id", problemID).Msg("Failed to update problem")
		return errorResponse(fmt.Sprintf("Failed to update problem with ID %d.", problemID)), nil
	}

	return messageResponse(fmt.Sprintf("Successfully updated problem '%s' (ID: %d)!", updatedProblem.ProblemName, problemID)), nil
}

func (b *Bot) handleDeleteCommand(s *discordgo.Session, i *discordgo.InteractionCreate) (*discordgo.InteractionResponse, error) {
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	idOpt, ok := optionMap["id"]
	if !ok {
		return errorResponse("Missing problem ID to delete."), nil
	}
	problemID := int(idOpt.IntValue())

	err := b.db.DeleteProblem(context.Background(), problemID)
	if err != nil {
		log.Error().Err(err).Int("problem_id", problemID).Msg("Failed to delete problem")
		return errorResponse(fmt.Sprintf("Failed to delete problem with ID %d.", problemID)), nil
	}

	return messageResponse(fmt.Sprintf("Successfully deleted problem with ID %d!", problemID)), nil
}

func (b *Bot) handleStatsCommand(s *discordgo.Session, i *discordgo.InteractionCreate) (*discordgo.InteractionResponse, error) {
	stats, err := b.db.GetUserStats(context.Background(), i.Member.User.ID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get user stats")
		return errorResponse("Failed to retrieve your statistics."), nil
	}

	if stats == nil {
		return messageResponse("No statistics found for you yet. Start adding problems!"), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Your LeetCode Statistics:\n"))
	sb.WriteString(fmt.Sprintf("Total Solved: %d\n", stats.TotalSolved))
	sb.WriteString(fmt.Sprintf("Needed Hint: %d\n", stats.TotalNeededHint))
	sb.WriteString(fmt.Sprintf("Stuck: %d\n", stats.TotalStuck))
	sb.WriteString(fmt.Sprintf("Easy: %d\n", stats.EasyCount))
	sb.WriteString(fmt.Sprintf("Medium: %d\n", stats.MediumCount))
	sb.WriteString(fmt.Sprintf("Hard: %d\n", stats.HardCount))
	if stats.LastActiveAt != nil {
		sb.WriteString(fmt.Sprintf("Last Active: %s\n", stats.LastActiveAt.Format("2006-01-02 15:04:05 MST")))
	}

	return messageResponse(sb.String()), nil
}

func messageResponse(content string) *discordgo.InteractionResponse {
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
		},
	}
}

func errorResponse(content string) *discordgo.InteractionResponse {
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	}
}
