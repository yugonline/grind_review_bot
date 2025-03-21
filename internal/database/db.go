package database

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/yugonline/grind_review_bot/config"

	// Database drivers
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Repository represents a database repository with ORM
type Repository struct {
	db     *gorm.DB
	config config.DatabaseConfig
}

// New creates a new database repository
func New(ctx context.Context, cfg config.DatabaseConfig) (*Repository, error) {
	// Configure GORM logger
	gormLogger := logger.New(
		&GormLogWriter{},
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logger.Info,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		},
	)

	gormConfig := &gorm.Config{
		Logger: gormLogger,
	}

	var db *gorm.DB
	var err error

	// Open database connection based on driver
	switch cfg.Driver {
	case "sqlite3":
		db, err = gorm.Open(sqlite.Open(cfg.DSN), gormConfig)
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.Driver)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Get generic database object
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database connection: %w", err)
	}

	// Configure connection pool
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLife)

	// Verify connection with timeout
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(pingCtx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Info().
		Str("driver", cfg.Driver).
		Str("dsn", maskDSN(cfg.DSN)).
		Int("max_conns", cfg.MaxOpenConns).
		Msg("Database connected successfully")

	return &Repository{
		db:     db,
		config: cfg,
	}, nil
}

// GormLogWriter adapts zerolog to GORM's logger interface
type GormLogWriter struct{}

// Printf implements the logger.Writer interface
func (w *GormLogWriter) Printf(format string, args ...interface{}) {
	log.Debug().Msgf(format, args...)
}

// GetDB returns the underlying GORM DB instance
func (r *Repository) GetDB() *gorm.DB {
	return r.db
}

// maskDSN hides sensitive information in DSN for logging
func maskDSN(dsn string) string {
	return dsn // For SQLite, just return the filename
}

// withContext creates a new DB instance with the given context
func (r *Repository) withContext(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx)
}

// CreateProblem creates a new problem entry with transaction support
func (r *Repository) CreateProblem(ctx context.Context, entry *ProblemEntry) error {
	if err := ValidateProblemEntry(entry); err != nil {
		return err
	}

	// Convert DTO to model
	problem := entry.ToProblem()

	// Execute in a transaction
	err := r.withContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Create problem with associations
		if err := tx.Create(problem).Error; err != nil {
			return fmt.Errorf("failed to create problem: %w", err)
		}

		// Update the ID in the entry
		entry.ID = problem.ID
		return nil
	})

	return err
}

// GetProblem retrieves a problem by ID with its associated tags
func (r *Repository) GetProblem(ctx context.Context, id uint) (*ProblemEntry, error) {
	var problem Problem
	err := r.withContext(ctx).Preload("Tags").First(&problem, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("problem not found: %d", id)
		}
		return nil, fmt.Errorf("failed to get problem: %w", err)
	}

	return FromProblem(&problem), nil
}

// UpdateProblem updates an existing problem entry with its tags
func (r *Repository) UpdateProblem(ctx context.Context, entry *ProblemEntry) error {
	if err := ValidateProblemEntry(entry); err != nil {
		return err
	}

	// Convert DTO to model
	problem := entry.ToProblem()

	// Execute in a transaction
	err := r.withContext(ctx).Transaction(func(tx *gorm.DB) error {
		// First, find the existing problem to update
		var existingProblem Problem
		if err := tx.First(&existingProblem, problem.ID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return fmt.Errorf("problem not found: %d", problem.ID)
			}
			return fmt.Errorf("failed to find problem: %w", err)
		}

		// Update the problem fields (excluding associations)
		if err := tx.Model(&existingProblem).Omit("Tags").Updates(map[string]interface{}{
			"UserID":         problem.UserID,
			"ProblemName":    problem.ProblemName,
			"Link":           problem.Link,
			"Difficulty":     problem.Difficulty,
			"Category":       problem.Category,
			"Status":         problem.Status,
			"SolvedAt":       problem.SolvedAt,
			"LastReviewedAt": problem.LastReviewedAt,
			"ReviewCount":    problem.ReviewCount,
			"Notes":          problem.Notes,
		}).Error; err != nil {
			return fmt.Errorf("failed to update problem: %w", err)
		}

		// Remove existing tag associations
		if err := tx.Model(&existingProblem).Association("Tags").Clear(); err != nil {
			return fmt.Errorf("failed to clear tags: %w", err)
		}

		// Add new tags
		for _, tag := range problem.Tags {
			var existingTag Tag
			// First check if the tag exists
			result := tx.Where("name = ?", tag.Name).First(&existingTag)
			if result.Error != nil {
				if result.Error == gorm.ErrRecordNotFound {
					// Create the tag if it doesn't exist
					if err := tx.Create(&tag).Error; err != nil {
						return fmt.Errorf("failed to create tag: %w", err)
					}
					existingTag = tag
				} else {
					return fmt.Errorf("failed to query tag: %w", result.Error)
				}
			}

			// Associate the tag with the problem
			if err := tx.Model(&existingProblem).Association("Tags").Append(&existingTag); err != nil {
				return fmt.Errorf("failed to associate tag: %w", err)
			}
		}

		return nil
	})

	return err
}

// DeleteProblem deletes a problem by ID
func (r *Repository) DeleteProblem(ctx context.Context, id uint) error {
	return r.withContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Delete the problem (this will automatically handle the problem_tags junction table)
		result := tx.Delete(&Problem{}, id)
		if result.Error != nil {
			return fmt.Errorf("failed to delete problem: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("problem not found: %d", id)
		}

		// Optionally, clean up orphaned tags
		if err := tx.Exec("DELETE FROM tags WHERE id NOT IN (SELECT tag_id FROM problem_tags)").Error; err != nil {
			return fmt.Errorf("failed to clean up orphaned tags: %w", err)
		}

		return nil
	})
}

// ListProblems retrieves a list of problems based on filters
func (r *Repository) ListProblems(ctx context.Context, userID, status, difficulty, category string, tagNames []string, limit, offset int) ([]*ProblemEntry, error) {
	query := r.withContext(ctx).Model(&Problem{}).Preload("Tags")

	// Apply filters
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if difficulty != "" {
		query = query.Where("difficulty = ?", difficulty)
	}
	if category != "" {
		query = query.Where("category = ?", category)
	}

	// Filter by tags if provided
	if len(tagNames) > 0 {
		// Join with problem_tags and tags tables to filter by tag names
		query = query.Joins("JOIN problem_tags ON problems.id = problem_tags.problem_id").
			Joins("JOIN tags ON problem_tags.tag_id = tags.id").
			Where("tags.name IN ?", tagNames)
	}

	// Apply pagination
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	// Execute query
	var problems []Problem
	if err := query.Order("solved_at DESC").Find(&problems).Error; err != nil {
		return nil, fmt.Errorf("failed to list problems: %w", err)
	}

	// Convert to DTOs
	result := make([]*ProblemEntry, len(problems))
	for i, problem := range problems {
		result[i] = FromProblem(&problem)
	}

	return result, nil
}

// ListProblemsForReview retrieves problems that need to be reviewed based on the lookback period
func (r *Repository) ListProblemsForReview(ctx context.Context, userID string, lookbackPeriod time.Duration) ([]*ProblemEntry, error) {
	cutoff := time.Now().Add(-lookbackPeriod)

	var problems []Problem
	err := r.withContext(ctx).Model(&Problem{}).
		Preload("Tags").
		Where("user_id = ?", userID).
		Where("solved_at <= ?", cutoff).
		Where(func(db *gorm.DB) *gorm.DB {
			return db.Where("last_reviewed_at IS NULL OR last_reviewed_at <= ?", cutoff)
		}).
		Order("solved_at ASC").
		Find(&problems).Error

	if err != nil {
		return nil, fmt.Errorf("failed to list problems for review: %w", err)
	}

	// Convert to DTOs
	result := make([]*ProblemEntry, len(problems))
	for i, problem := range problems {
		result[i] = FromProblem(&problem)
	}

	return result, nil
}

// IncrementReviewCount increments the review count and updates the last reviewed timestamp
func (r *Repository) IncrementReviewCount(ctx context.Context, problemID uint) error {
	now := time.Now()
	err := r.withContext(ctx).Model(&Problem{}).
		Where("id = ?", problemID).
		Updates(map[string]interface{}{
			"review_count":     gorm.Expr("review_count + 1"),
			"last_reviewed_at": now,
		}).Error

	if err != nil {
		return fmt.Errorf("failed to increment review count: %w", err)
	}
	return nil
}

// ListAllUsers lists all unique user IDs in the database
func (r *Repository) ListAllUsers(ctx context.Context) ([]string, error) {
	var userIDs []string
	err := r.withContext(ctx).Model(&Problem{}).
		Distinct("user_id").
		Pluck("user_id", &userIDs).Error

	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	return userIDs, nil
}

// AutoMigrate runs GORM's auto-migration for database tables
// Note: We're keeping the existing migration system, but this is useful for development
func (r *Repository) AutoMigrate() error {
	if err := r.db.AutoMigrate(&Problem{}, &Tag{}); err != nil {
		return fmt.Errorf("failed to auto-migrate: %w", err)
	}
	return nil
}