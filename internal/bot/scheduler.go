package bot

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-co-op/gocron"
	"github.com/rs/zerolog/log"
	"github.com/yugonline/grind_review_bot/config"
)

// Scheduler manages the daily review reminders
type Scheduler struct {
	cron    *gocron.Scheduler
	bot     *Bot
	config  config.SchedulerConfig
	stop    chan bool
	running bool
}

// StartScheduler initializes and starts the daily review scheduler
func StartScheduler(ctx context.Context, b *Bot, cfg config.SchedulerConfig) *Scheduler {
	s := &Scheduler{
		cron:    gocron.NewScheduler(time.Local),
		bot:     b,
		config:  cfg,
		stop:    make(chan bool),
		running: false,
	}

	if _, err := s.cron.Every(1).Day().At(cfg.ReviewTime).Do(s.sendDailyReviewReminder, ctx); err != nil {
		log.Error().Err(err).Str("review_time", cfg.ReviewTime).Msg("Failed to schedule daily review reminder")
		return s
	}

	s.cron.StartAsync()
	s.running = true
	log.Info().Str("review_time", cfg.ReviewTime).Msg("Daily review scheduler started")
	return s
}

// Stop halts the scheduler
func (s *Scheduler) Stop() {
	if s.running {
		s.cron.Stop()
		s.running = false
		log.Info().Msg("Daily review scheduler stopped")
	}
	close(s.stop)
}

// sendDailyReviewReminder fetches problems needing review and sends a message to Discord
func (s *Scheduler) sendDailyReviewReminder(ctx context.Context) {
	if s.config.ReviewChannel == "" {
		log.Warn().Msg("Review channel not configured, skipping daily reminder.")
		return
	}

	users, err := s.bot.repo.ListAllUsers(ctx) // Get all users who have added problems
	if err != nil {
		log.Error().Err(err).Msg("Failed to list users for review reminders")
		return
	}

	for _, userID := range users {
		problems, err := s.bot.repo.ListProblemsForReview(ctx, userID, s.config.LookbackPeriod)
		if err != nil {
			log.Error().Err(err).Str("user_id", userID).Msg("Failed to list problems for review")
			continue
		}

		if len(problems) > 0 {
			user, err := s.bot.session.User(userID)
			if err != nil {
				log.Error().Err(err).Str("user_id", userID).Msg("Failed to get Discord user")
				continue
			}

			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("Hey %s! Here are some problems you might want to review today:\n", user.Mention()))
			for _, p := range problems {
				sb.WriteString(fmt.Sprintf("- %s (Solved: %s)", p.ProblemName, p.SolvedAt.Format("2006-01-02")))
				if p.Link != "" {
					sb.WriteString(fmt.Sprintf(" - <%s>", p.Link))
				}
				sb.WriteString("\n")
			}
			sb.WriteString("\nRemember, consistent review helps reinforce your understanding!")

			_, err = s.bot.session.ChannelMessageSend(s.config.ReviewChannel, sb.String())
			if err != nil {
				log.Error().Err(err).Str("channel_id", s.config.ReviewChannel).Str("user_id", userID).Msg("Failed to send review reminder")
				// Implement retry logic if needed
				for i := 0; i < s.config.RetryAttempts; i++ {
					time.Sleep(s.config.RetryDelay)
					_, retryErr := s.bot.session.ChannelMessageSend(s.config.ReviewChannel, sb.String())
					if retryErr == nil {
						log.Info().Str("channel_id", s.config.ReviewChannel).Str("user_id", userID).Int("attempt", i+1).Msg("Successfully sent review reminder after retry")
						break
					}
					log.Error().Err(retryErr).Str("channel_id", s.config.ReviewChannel).Str("user_id", userID).Int("attempt", i+1).Msg("Failed to send review reminder (retry)")
				}
			} else {
				log.Info().Str("channel_id", s.config.ReviewChannel).Str("user_id", userID).Int("problem_count", len(problems)).Msg("Sent daily review reminder")
				// Update last reviewed at for these problems to avoid repeated reminders too soon
				for _, p := range problems {
					if err := s.bot.repo.IncrementReviewCount(ctx, p.ID); err != nil {
						log.Error().Err(err).Uint("problem_id", p.ID).Msg("Failed to update review count")
					}
				}
			}
		}
	}
}