package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/yugonline/grind_review_bot/config"

	// Database drivers
	_ "github.com/mattn/go-sqlite3"
)

// DB represents a database connection
type DB struct {
	*sql.DB
	config config.DatabaseConfig
}

// New creates a new database connection
func New(ctx context.Context, cfg config.DatabaseConfig) (*DB, error) {
	// Open database connection
	db, err := sql.Open(cfg.Driver, cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLife)

	// Verify connection with timeout
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := db.PingContext(pingCtx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Info().
		Str("driver", cfg.Driver).
		Str("dsn", maskDSN(cfg.DSN)).
		Int("max_conns", cfg.MaxOpenConns).
		Msg("Database connected successfully")

	return &DB{DB: db, config: cfg}, nil
}

// QueryContext executes a query with timeout from config
func (db *DB) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	queryCtx, cancel := context.WithTimeout(ctx, db.config.QueryTimeout)
	defer cancel()
	return db.DB.QueryContext(queryCtx, query, args...)
}

// ExecContext executes a statement with timeout from config
func (db *DB) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	execCtx, cancel := context.WithTimeout(ctx, db.config.QueryTimeout)
	defer cancel()
	return db.DB.ExecContext(execCtx, query, args...)
}

// Begin starts a transaction with default options
func (db *DB) Begin(ctx context.Context) (*sql.Tx, error) {
	txCtx, cancel := context.WithTimeout(ctx, db.config.QueryTimeout)
	defer cancel()
	return db.DB.BeginTx(txCtx, nil)
}

// maskDSN hides sensitive information in DSN for logging
func maskDSN(dsn string) string {
	// For SQLite, just return the filename
	// For other databases, you'd want to mask passwords
	return dsn
}

// ListAllUsers lists all unique user IDs in the database
func (db *DB) ListAllUsers(ctx context.Context) ([]string, error) {
	query := `SELECT DISTINCT user_id FROM problems`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query distinct user IDs: %w", err)
	}
	defer rows.Close()

	var users []string
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, fmt.Errorf("failed to scan user ID: %w", err)
		}
		users = append(users, userID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over user IDs: %w", err)
	}

	return users, nil
}
