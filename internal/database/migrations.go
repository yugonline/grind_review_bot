package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
)

// Schema version
const currentVersion = 1

// Migrate runs database migrations to ensure schema is up to date
func Migrate(ctx context.Context, db *DB) error {
	// Create migration table if it doesn't exist
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create migration table: %w", err)
	}

	// Get current schema version
	var version int
	err = db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&version)
	if err != nil {
		return fmt.Errorf("failed to get schema version: %w", err)
	}

	log.Info().Int("current_version", version).Int("target_version", currentVersion).Msg("Checking database migrations")

	// Apply migrations if needed
	if version < currentVersion {
		log.Info().Msg("Running database migrations")
		if err := runMigrations(ctx, db, version); err != nil {
			return err
		}
	}

	return nil
}

// runMigrations applies all necessary migrations in sequence
func runMigrations(ctx context.Context, db *DB, currentVersion int) error {
	// Start a transaction for all migrations
	tx, err := db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Apply each migration in sequence
	for v := currentVersion + 1; v <= currentVersion; v++ {
		log.Info().Int("version", v).Msg("Applying migration")
		
		switch v {
		case 1:
			if err := migrateV1(ctx, tx); err != nil {
				return err
			}
		// Add future migrations here
		default:
			return fmt.Errorf("unknown migration version: %d", v)
		}

		// Update schema version
		_, err = tx.ExecContext(ctx, `INSERT INTO schema_migrations (version) VALUES (?)`, v)
		if err != nil {
			return fmt.Errorf("failed to update schema version: %w", err)
		}
	}

	// Commit all migrations
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit migrations: %w", err)
	}

	log.Info().Int("version", currentVersion).Msg("Database schema is up to date")
	return nil
}

// Migration to create initial schema
func migrateV1(ctx context.Context, tx *sql.Tx) error {
	// Create problems table
	_, err := tx.ExecContext(ctx, `
	CREATE TABLE IF NOT EXISTS problems (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id TEXT NOT NULL,
		problem_name TEXT NOT NULL,
		link TEXT,
		difficulty TEXT NOT NULL,
		category TEXT NOT NULL,
		status TEXT NOT NULL,
		solved_at TIMESTAMP NOT NULL,
		last_reviewed_at TIMESTAMP,
		review_count INTEGER NOT NULL DEFAULT 0,
		notes TEXT
	)`)
	if err != nil {
		return fmt.Errorf("failed to create problems table: %w", err)
	}

	// Create tags table
	_, err = tx.ExecContext(ctx, `
	CREATE TABLE IF NOT EXISTS tags (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE
	)`)
	if err != nil {
		return fmt.Errorf("failed to create tags table: %w", err)
	}

	// Create problem_tags table (many-to-many)
	_, err = tx.ExecContext(ctx, `
	CREATE TABLE IF NOT EXISTS problem_tags (
		problem_id INTEGER NOT NULL,
		tag_id INTEGER NOT NULL,
		PRIMARY KEY (problem_id, tag_id),
		FOREIGN KEY (problem_id) REFERENCES problems(id) ON DELETE CASCADE,
		FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE
	)`)
	if err != nil {
		return fmt.Errorf("failed to create problem_tags table: %w", err)
	}

	// Create user_stats table for aggregated metrics
	_, err = tx.ExecContext(ctx, `
	CREATE TABLE IF NOT EXISTS user_stats (
		user_id TEXT PRIMARY KEY,
		total_solved INTEGER NOT NULL DEFAULT 0,
		total_needed_hint INTEGER NOT NULL DEFAULT 0,
		total_stuck INTEGER NOT NULL DEFAULT 0,
		easy_count INTEGER NOT NULL DEFAULT 0,
		medium_count INTEGER NOT NULL DEFAULT 0,
		hard_count INTEGER NOT NULL DEFAULT 0,
		last_active_at TIMESTAMP
	)`)
	if err != nil {
		return fmt.Errorf("failed to create user_stats table: %w", err)
	}

	// Create indices for common queries
	queries := []string{
		`CREATE INDEX IF NOT EXISTS idx_problems_user_id ON problems(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_problems_status ON problems(status)`,
		`CREATE INDEX IF NOT EXISTS idx_problems_solved_at ON problems(solved_at)`,
		`CREATE INDEX IF NOT EXISTS idx_problems_difficulty ON problems(difficulty)`,
		`CREATE INDEX IF NOT EXISTS idx_problems_category ON problems(category)`,
		`CREATE INDEX IF NOT EXISTS idx_problems_status_solved_at ON problems(status, solved_at)`,
		`CREATE INDEX IF NOT EXISTS idx_problem_tags_problem_id ON problem_tags(problem_id)`,
		`CREATE INDEX IF NOT EXISTS idx_problem_tags_tag_id ON problem_tags(tag_id)`,
	}

	for _, query := range queries {
		if _, err := tx.ExecContext(ctx, query); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	return nil
}