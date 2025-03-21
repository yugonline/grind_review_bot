package database

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

// MigrationDir is the directory containing migration files
const MigrationDir = "internal/database/migrations"

// Migrate runs database migrations to ensure schema is up to date
func Migrate(ctx context.Context, repo *Repository) error {
	// Get the underlying SQL database instance from GORM
	sqlDB, err := repo.db.DB()
	if err != nil {
		return fmt.Errorf("failed to get database connection: %w", err)
	}

	// Create migration instance using the SQL DB
	driver, err := sqlite3.WithInstance(sqlDB, &sqlite3.Config{})
	if err != nil {
		return fmt.Errorf("failed to create sqlite driver: %w", err)
	}

	// Find the project root directory to locate migrations
	migrationPath, err := findMigrationDir()
	if err != nil {
		return fmt.Errorf("failed to find migrations directory: %w", err)
	}

	// Create migration source URL
	sourceURL := fmt.Sprintf("file://%s", migrationPath)
	log.Info().Str("source", sourceURL).Msg("Using migration source")

	// Create migration instance
	m, err := migrate.NewWithDatabaseInstance(
		sourceURL,
		"sqlite3", // This is just a name for the database instance
		driver,
	)
	if err != nil {
		return fmt.Errorf("failed to create migration instance: %w", err)
	}

	// Execute migration
	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			log.Info().Msg("Database schema is already up to date")
			return nil
		}
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	log.Info().Msg("Database migrations completed successfully")
	return nil
}

// findMigrationDir attempts to locate the migrations directory by walking up
// from the current directory until it finds one containing the migrations
func findMigrationDir() (string, error) {
	// Start with the current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}

	// First, check if migrations are directly in the expected path
	migPath := filepath.Join(cwd, MigrationDir)
	if dirExists(migPath) {
		return migPath, nil
	}

	// If not, search common relative paths
	possiblePaths := []string{
		migPath,
		filepath.Join(cwd, "database/migrations"),
		filepath.Join(cwd, "migrations"),
	}

	for _, path := range possiblePaths {
		if dirExists(path) {
			return path, nil
		}
	}

	// As a fallback, construct an absolute path from components
	// This handles cases where the code might be run from different directories
	path, _ := filepath.Abs(migPath)
	
	// URL encode the path to handle spaces and special characters
	encoded := url.PathEscape(path)
	return encoded, nil
}

// dirExists checks if a directory exists
func dirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// WithTransactionContext executes the given function within a transaction
// This is a helper for the repository pattern when you need custom transaction control
func WithTransactionContext(ctx context.Context, db *gorm.DB, fn func(tx *gorm.DB) error) error {
	tx := db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r) // Re-throw the panic after rollback
		}
	}()

	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}