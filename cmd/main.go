package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/yugonline/grind_review_bot/config"
	"github.com/yugonline/grind_review_bot/internal/bot"
	"github.com/yugonline/grind_review_bot/internal/database"
	"github.com/yugonline/grind_review_bot/internal/metrics"
)

func main() {
	// Initialize structured logging
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339})

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	// Configure log level based on configuration
	logLevel, err := zerolog.ParseLevel(cfg.LogLevel)
	if err != nil {
		log.Warn().Err(err).Str("fallback", "info").Msg("Invalid log level, using INFO")
		logLevel = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(logLevel)

	// Create context that we can cancel on shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize metrics (if enabled)
	if cfg.Metrics.Enabled {
		metricsServer := metrics.New(cfg.Metrics)
		go func() {
			if err := metricsServer.Start(); err != nil {
				log.Error().Err(err).Msg("Metrics server failed")
			}
		}()
		defer metricsServer.Stop(ctx)
	}

	// Initialize database
	db, err := database.New(ctx, cfg.Database)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize database")
	}
	defer db.Close()

	// Run database migrations
	if err := database.Migrate(ctx, db); err != nil {
		log.Fatal().Err(err).Msg("Failed to run database migrations")
	}

	// Create and set up Discord bot
	discordBot, err := bot.New(ctx, cfg.Discord, db)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create Discord bot")
	}

	// Start the bot
	if err := discordBot.Start(ctx); err != nil {
		log.Fatal().Err(err).Msg("Failed to start bot")
	}
	log.Info().Msg("LeetCode Grind Review Bot is running! 🚀")

	// Start scheduler for daily reviews
	scheduler := bot.StartScheduler(ctx, discordBot, cfg.Scheduler)
	defer scheduler.Stop()

	// Wait for termination signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	// Create a shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	// Graceful shutdown
	log.Info().Msg("Shutting down gracefully...")
	if err := discordBot.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("Error during bot shutdown")
	}
}
