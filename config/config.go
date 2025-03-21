package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application
type Config struct {
	Discord   DiscordConfig   `mapstructure:"discord"`
	Database  DatabaseConfig  `mapstructure:"database"`
	Scheduler SchedulerConfig `mapstructure:"scheduler"`
	Metrics   MetricsConfig   `mapstructure:"metrics"`
	LogLevel  string          `mapstructure:"log_level"`
}

// DiscordConfig holds Discord-specific configuration
type DiscordConfig struct {
	Token             string        `mapstructure:"token"`
	GuildID           string        `mapstructure:"guild_id"`
	CommandsTimeout   time.Duration `mapstructure:"commands_timeout"`
	InteractionExpiry time.Duration `mapstructure:"interaction_expiry"`
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Driver         string        `mapstructure:"driver"`
	DSN            string        `mapstructure:"dsn"`
	MaxOpenConns   int           `mapstructure:"max_open_conns"`
	MaxIdleConns   int           `mapstructure:"max_idle_conns"`
	ConnMaxLife    time.Duration `mapstructure:"conn_max_life"`
	QueryTimeout   time.Duration `mapstructure:"query_timeout"`
	MigrationsPath string        `mapstructure:"migrations_path"`
}

// SchedulerConfig holds configuration for the scheduler
type SchedulerConfig struct {
	ReviewTime     string        `mapstructure:"review_time"`
	ReviewChannel  string        `mapstructure:"review_channel"`
	RetryAttempts  int           `mapstructure:"retry_attempts"`
	RetryDelay     time.Duration `mapstructure:"retry_delay"`
	LookbackPeriod time.Duration `mapstructure:"lookback_period"`
}

// MetricsConfig holds configuration for metrics collection
type MetricsConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Address string `mapstructure:"address"`
}

// Load reads in config file and ENV variables if set
func Load() (*Config, error) {
	// Set defaults first
	setDefaults()

	// Try to read from config file if it exists
	configPaths := []string{
		".",
		"./config",
		"/etc/grind_review_bot",
	}

	for _, path := range configPaths {
		viper.AddConfigPath(path)
	}

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	// Read from environment variables too
	viper.AutomaticEnv()
	viper.SetEnvPrefix("GRIND_REVIEW")

	// Try to read the config file
	if err := viper.ReadInConfig(); err != nil {
		// It's okay if config file doesn't exist, we might use env vars
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	// Unmarshal config
	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("unable to decode config: %w", err)
	}

	// Override token from environment if set
	if token := os.Getenv("DISCORD_BOT_TOKEN"); token != "" {
		config.Discord.Token = token
	}

	// Validate config
	if config.Discord.Token == "" {
		return nil, fmt.Errorf("Discord bot token is required")
	}

	return &config, nil
}

// setDefaults sets default values for configuration
func setDefaults() {
	// Discord defaults
	viper.SetDefault("discord.commands_timeout", 5*time.Second)
	viper.SetDefault("discord.interaction_expiry", 15*time.Minute)

	// Database defaults
	viper.SetDefault("database.driver", "sqlite3")
	viper.SetDefault("database.dsn", "grind_review.db")
	viper.SetDefault("database.max_open_conns", 10)
	viper.SetDefault("database.max_idle_conns", 5)
	viper.SetDefault("database.conn_max_life", 1*time.Hour)
	viper.SetDefault("database.query_timeout", 30*time.Second)
	viper.SetDefault("database.migrations_path", "./internal/database/migrations")

	// Scheduler defaults
	viper.SetDefault("scheduler.review_time", "08:00")
	viper.SetDefault("scheduler.retry_attempts", 3)
	viper.SetDefault("scheduler.retry_delay", 2*time.Second)
	viper.SetDefault("scheduler.lookback_period", 24*time.Hour)

	// Metrics defaults
	viper.SetDefault("metrics.enabled", false)
	viper.SetDefault("metrics.address", ":9090")

	// Logging defaults
	viper.SetDefault("log_level", "info")
}