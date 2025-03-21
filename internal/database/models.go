package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// Problem status constants
const (
	StatusSolved     = "Solved"
	StatusNeededHint = "Needed Hint"
	StatusStuck      = "Stuck"
)

// Difficulty constants
const (
	DifficultyEasy   = "Easy"
	DifficultyMedium = "Medium"
	DifficultyHard   = "Hard"
)

// ProblemEntry represents a solved problem
type ProblemEntry struct {
	ID             int        `json:"id"`
	UserID         string     `json:"user_id"`
	ProblemName    string     `json:"problem_name"`
	Link           string     `json:"link"`
	Difficulty     string     `json:"difficulty"`
	Category       string     `json:"category"`
	Status         string     `json:"status"`
	SolvedAt       time.Time  `json:"solved_at"`
	LastReviewedAt *time.Time `json:"last_reviewed_at"`
	ReviewCount    int        `json:"review_count"`
	Notes          string     `json:"notes"`
	Tags           []string   `json:"tags"`
}

// UserStats represents aggregated stats for a user
type UserStats struct {
	UserID          string     `json:"user_id"`
	TotalSolved     int        `json:"total_solved"`
	TotalNeededHint int        `json:"total_needed_hint"`
	TotalStuck      int        `json:"total_stuck"`
	EasyCount       int        `json:"easy_count"`
	MediumCount     int        `json:"medium_count"`
	HardCount       int        `json:"hard_count"`
	LastActiveAt    *time.Time `json:"last_active_at"`
}

// ValidateProblemEntry validates a problem entry
func ValidateProblemEntry(p *ProblemEntry) error {
	if p.UserID == "" {
		return errors.New("user ID is required")
	}
	if p.ProblemName == "" {
		return errors.New("problem name is required")
	}
	if p.Difficulty != DifficultyEasy && p.Difficulty != DifficultyMedium && p.Difficulty != DifficultyHard {
		return fmt.Errorf("invalid difficulty: %s", p.Difficulty)
	}
	if p.Status != StatusSolved && p.Status != StatusNeededHint && p.Status != StatusStuck {
		return fmt.Errorf("invalid status: %s", p.Status)
	}
	if p.Category == "" {
		return errors.New("category is required")
	}
	return nil
}

// InsertProblem inserts a new problem entry with transaction support
func (db *DB) InsertProblem(ctx context.Context, p *ProblemEntry) error {
	if err := ValidateProblemEntry(p); err != nil {
		return err
	}

	// Start transaction
	tx, err := db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert problem
	query := `
	INSERT INTO problems (
		user_id, problem_name, link, difficulty, category, status, solved_at, notes
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	RETURNING id
	`

	// For SQLite which might not support RETURNING
	var result sql.Result
	var problemID int

	if db.config.Driver == "sqlite3" {
		result, err = tx.ExecContext(ctx, `
		INSERT INTO problems (
			user_id, problem_name, link, difficulty, category, status, solved_at, notes
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			p.UserID, p.ProblemName, p.Link, p.Difficulty, p.Category, p.Status, p.SolvedAt, p.Notes)
		if err != nil {
			return fmt.Errorf("failed to insert problem: %w", err)
		}

		// Get last inserted ID
		lastID, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("failed to get last insert ID: %w", err)
		}
		problemID = int(lastID)
	} else {
		// For databases supporting RETURNING
		err = tx.QueryRowContext(ctx, query,
			p.UserID, p.ProblemName, p.Link, p.Difficulty, p.Category, p.Status, p.SolvedAt, p.Notes).Scan(&problemID)
		if err != nil {
			return fmt.Errorf("failed to insert problem: %w", err)
		}
	}

	// Insert tags if any
	if len(p.Tags) > 0 {
		for _, tag := range p.Tags {
			tag = strings.TrimSpace(tag)
			if tag == "" {
				continue
			}

			// Insert tag if not exists
			var tagID int
			err := tx.QueryRowContext(ctx, `
				INSERT INTO tags (name) VALUES (?)
				ON CONFLICT (name) DO UPDATE SET name=name
				RETURNING id`,
				tag).Scan(&tagID)

			if err != nil {
				// Try alternative approach for SQLite
				if db.config.Driver == "sqlite3" {
					// Check if tag exists
					err = tx.QueryRowContext(ctx, `SELECT id FROM tags WHERE name = ?`, tag).Scan(&tagID)
					if err != nil {
						if errors.Is(err, sql.ErrNoRows) {
							// Insert new tag
							res, err := tx.ExecContext(ctx, `INSERT INTO tags (name) VALUES (?)`, tag)
							if err != nil {
								return fmt.Errorf("failed to insert tag: %w", err)
							}
							lastID, err := res.LastInsertId()
							if err != nil {
								return fmt.Errorf("failed to get tag ID: %w", err)
							}
							tagID = int(lastID)
						} else {
							return fmt.Errorf("failed to check tag existence: %w", err)
						}
					}
				} else {
					return fmt.Errorf("failed to upsert tag: %w", err)
				}
			}

			// Link problem to tag
			_, err = tx.ExecContext(ctx, `
				INSERT INTO problem_tags (problem_id, tag_id) VALUES (?, ?)`,
				problemID, tagID)
			if err != nil {
				return fmt.Errorf("failed to link problem to tag: %w", err)
			}
		}
	}

	// Update user stats
	err = updateUserStats(ctx, tx, p)
	if err != nil {
		return fmt.Errorf("failed to update user stats: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Update the ID in the struct
	p.ID = problemID
	return nil
}

// updateUserStats updates user statistics based on the new problem
func updateUserStats(ctx context.Context, tx *sql.Tx, p *ProblemEntry) error {
	now := time.Now()

	// Check if user stats exist
	var exists bool
	err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM user_stats WHERE user_id = ?)`, p.UserID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check user stats existence: %w", err)
	}

	if !exists {
		// Create new user stats
		_, err = tx.ExecContext(ctx, `
			INSERT INTO user_stats (
				user_id, last_active_at
			) VALUES (?, ?)`,
			p.UserID, now)
		if err != nil {
			return fmt.Errorf("failed to create user stats: %w", err)
		}
	}

	// Prepare the update statement based on problem properties
	var updates []string
	var args []interface{}

	// Update counters based on status
	switch p.Status {
	case StatusSolved:
		updates = append(updates, "total_solved = total_solved + 1")
	case StatusNeededHint:
		updates = append(updates, "total_needed_hint = total_needed_hint + 1")
	case StatusStuck:
		updates = append(updates, "total_stuck = total_stuck + 1")
	}

	// Update counters based on difficulty
	switch p.Difficulty {
	case DifficultyEasy:
		updates = append(updates, "easy_count = easy_count + 1")
	case DifficultyMedium:
		updates = append(updates, "medium_count = medium_count + 1")
	case DifficultyHard:
		updates = append(updates, "hard_count = hard_count + 1")
	}

	// Always update last active timestamp
	updates = append(updates, "last_active_at = ?")
	args = append(args, now)

	// Add WHERE clause
	args = append(args, p.UserID)

	// Execute the update
	query := fmt.Sprintf(`
		UPDATE user_stats
		SET %s
		WHERE user_id = ?`, strings.Join(updates, ", "))

	_, err = tx.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update user stats: %w", err)
	}

	return nil
}

// GetProblem fetches a problem by ID
func (db *DB) GetProblem(ctx context.Context, id int) (*ProblemEntry, error) {
	query := `
	SELECT id, user_id, problem_name, link, difficulty, category, status,
	       solved_at, last_reviewed_at, review_count, notes
	FROM problems
	WHERE id = ?
	`

	var p ProblemEntry
	var solvedAt string
	var lastReviewedAt sql.NullString

	err := db.QueryRowContext(ctx, query, id).Scan(
		&p.ID, &p.UserID, &p.ProblemName, &p.Link,
		&p.Difficulty, &p.Category, &p.Status, &solvedAt,
		&lastReviewedAt, &p.ReviewCount, &p.Notes,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("problem not found: %d", id)
		}
		return nil, fmt.Errorf("failed to fetch problem: %w", err)
	}

	// Parse timestamps
	parsedTime, err := time.Parse("2006-01-02 15:04:05", solvedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse solved_at time: %w", err)
	}
	p.SolvedAt = parsedTime

	if lastReviewedAt.Valid {
		reviewedTime, err := time.Parse("2006-01-02 15:04:05", lastReviewedAt.String)
		if err != nil {
			return nil, fmt.Errorf("failed to parse last_reviewed_at time: %w", err)
		}
		p.LastReviewedAt = &reviewedTime
	}

	// Fetch tags
	tags, err := db.GetProblemTags(ctx, p.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get problem tags: %w", err)
	}
	p.Tags = tags

	return &p, nil
}

// GetProblemTags fetches tags associated with a problem ID
func (db *DB) GetProblemTags(ctx context.Context, problemID int) ([]string, error) {
	query := `
	SELECT t.name
	FROM problem_tags pt
	JOIN tags t ON pt.tag_id = t.id
	WHERE pt.problem_id = ?
	`

	rows, err := db.QueryContext(ctx, query, problemID)
	if err != nil {
		return nil, fmt.Errorf("failed to query problem tags: %w", err)
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, fmt.Errorf("failed to scan tag: %w", err)
		}
		tags = append(tags, tag)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over tags: %w", err)
	}

	return tags, nil
}

// UpdateProblem updates an existing problem entry
func (db *DB) UpdateProblem(ctx context.Context, p *ProblemEntry) error {
	if err := ValidateProblemEntry(p); err != nil {
		return err
	}

	// Start transaction
	tx, err := db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	query := `
	UPDATE problems
	SET user_id = ?,
		problem_name = ?,
		link = ?,
		difficulty = ?,
		category = ?,
		status = ?,
		solved_at = ?,
		last_reviewed_at = ?,
		review_count = ?,
		notes = ?
	WHERE id = ?
	`

	_, err = tx.ExecContext(ctx, query,
		p.UserID, p.ProblemName, p.Link, p.Difficulty, p.Category, p.Status,
		p.SolvedAt, p.LastReviewedAt, p.ReviewCount, p.Notes, p.ID)
	if err != nil {
		return fmt.Errorf("failed to update problem: %w", err)
	}

	// Update tags
	if err := db.updateProblemTags(ctx, tx, p.ID, p.Tags); err != nil {
		return fmt.Errorf("failed to update problem tags: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// updateProblemTags updates the tags associated with a problem
func (db *DB) updateProblemTags(ctx context.Context, tx *sql.Tx, problemID int, tags []string) error {
	// Delete existing tags for the problem
	_, err := tx.ExecContext(ctx, `DELETE FROM problem_tags WHERE problem_id = ?`, problemID)
	if err != nil {
		return fmt.Errorf("failed to delete existing problem tags: %w", err)
	}

	// Insert new tags
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}

		var tagID int
		err := tx.QueryRowContext(ctx, `
			INSERT INTO tags (name) VALUES (?)
			ON CONFLICT (name) DO UPDATE SET name=name
			RETURNING id`,
			tag).Scan(&tagID)

		if err != nil {
			// Try alternative approach for SQLite
			if db.config.Driver == "sqlite3" {
				// Check if tag exists
				err = tx.QueryRowContext(ctx, `SELECT id FROM tags WHERE name = ?`, tag).Scan(&tagID)
				if err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						// Insert new tag
						res, err := tx.ExecContext(ctx, `INSERT INTO tags (name) VALUES (?)`, tag)
						if err != nil {
							return fmt.Errorf("failed to insert tag: %w", err)
						}
						lastID, err := res.LastInsertId()
						if err != nil {
							return fmt.Errorf("failed to get tag ID: %w", err)
						}
						tagID = int(lastID)
					} else {
						return fmt.Errorf("failed to check tag existence: %w", err)
					}
				}
			} else {
				return fmt.Errorf("failed to upsert tag: %w", err)
			}
		}

		// Link problem to tag
		_, err = tx.ExecContext(ctx, `
			INSERT INTO problem_tags (problem_id, tag_id) VALUES (?, ?)`,
			problemID, tagID)
		if err != nil {
			return fmt.Errorf("failed to link problem to tag: %w", err)
		}
	}
	return nil
}

// DeleteProblem deletes a problem by ID
func (db *DB) DeleteProblem(ctx context.Context, id int) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `DELETE FROM problems WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete problem: %w", err)
	}

	// Optionally delete orphaned tags (tags with no associated problems)
	// This can be done with a separate query if needed.

	return tx.Commit()
}

// ListProblems retrieves a list of problems based on filters
func (db *DB) ListProblems(ctx context.Context, userID string, status string, difficulty string, category string, tags []string, limit int, offset int) ([]*ProblemEntry, error) {
	var conditions []string
	var args []interface{}

	if userID != "" {
		conditions = append(conditions, "user_id = ?")
		args = append(args, userID)
	}
	if status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, status)
	}
	if difficulty != "" {
		conditions = append(conditions, "difficulty = ?")
		args = append(args, difficulty)
	}
	if category != "" {
		conditions = append(conditions, "category = ?")
		args = append(args, category)
	}

	query := `
	SELECT p.id, p.user_id, p.problem_name, p.link, p.difficulty, p.category, p.status,
	       p.solved_at, p.last_reviewed_at, p.review_count, p.notes
	FROM problems p
	`

	if len(tags) > 0 {
		query += `
		JOIN problem_tags pt ON p.id = pt.problem_id
		JOIN tags t ON pt.tag_id = t.id
		WHERE t.name IN (` + strings.Join(strings.Split(strings.Repeat("?", len(tags)), ""), ", ") + `)
		`
		for _, tag := range tags {
			args = append(args, tag)
		}
		if len(conditions) > 0 {
			query += " AND " + strings.Join(conditions, " AND ")
		} else {
			// Need to handle the case where there are tags but no other conditions
			// The WHERE clause is already added for tags.
		}
	} else if len(conditions) > 0 {
		query += "WHERE " + strings.Join(conditions, " AND ")
	}

	query += ` ORDER BY solved_at DESC`

	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}
	if offset > 0 {
		query += ` OFFSET ?`
		args = append(args, offset)
	}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list problems: %w", err)
	}
	defer rows.Close()

	var problems []*ProblemEntry
	for rows.Next() {
		var p ProblemEntry
		var solvedAt string
		var lastReviewedAt sql.NullString

		err := rows.Scan(
			&p.ID, &p.UserID, &p.ProblemName, &p.Link,
			&p.Difficulty, &p.Category, &p.Status, &solvedAt,
			&lastReviewedAt, &p.ReviewCount, &p.Notes,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan problem: %w", err)
		}

		// Parse timestamps
		parsedTime, err := time.Parse("2006-01-02 15:04:05", solvedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to parse solved_at time: %w", err)
		}
		p.SolvedAt = parsedTime

		if lastReviewedAt.Valid {
			reviewedTime, err := time.Parse("2006-01-02 15:04:05", lastReviewedAt.String)
			if err != nil {
				return nil, fmt.Errorf("failed to parse last_reviewed_at time: %w", err)
			}
			p.LastReviewedAt = &reviewedTime
		}

		// Fetch tags for each problem
		p.Tags, err = db.GetProblemTags(ctx, p.ID)
		if err != nil {
			log.Error().Err(err).Int("problem_id", p.ID).Msg("Failed to get tags for problem")
			// Continue with the problem entry even if tags fail to load
		}

		problems = append(problems, &p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over problems: %w", err)
	}

	return problems, nil
}

// GetUserStats retrieves user statistics
func (db *DB) GetUserStats(ctx context.Context, userID string) (*UserStats, error) {
	query := `
	SELECT user_id, total_solved, total_needed_hint, total_stuck, easy_count, medium_count, hard_count, last_active_at
	FROM user_stats
	WHERE user_id = ?
	`

	var stats UserStats
	var lastActiveAt sql.NullString

	err := db.QueryRowContext(ctx, query, userID).Scan(
		&stats.UserID, &stats.TotalSolved, &stats.TotalNeededHint, &stats.TotalStuck,
		&stats.EasyCount, &stats.MediumCount, &stats.HardCount, &lastActiveAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // User stats not found, return nil
		}
		return nil, fmt.Errorf("failed to get user stats: %w", err)
	}

	if lastActiveAt.Valid {
		parsedTime, err := time.Parse("2006-01-02 15:04:05", lastActiveAt.String)
		if err != nil {
			return nil, fmt.Errorf("failed to parse last_active_at time: %w", err)
		}
		stats.LastActiveAt = &parsedTime
	}

	return &stats, nil
}

// UpdateUserLastActive updates the last active timestamp for a user
func (db *DB) UpdateUserLastActive(ctx context.Context, userID string) error {
	now := time.Now()
	_, err := db.ExecContext(ctx, `
		UPDATE user_stats
		SET last_active_at = ?
		WHERE user_id = ?`,
		now, userID)
	if err != nil {
		return fmt.Errorf("failed to update last active time: %w", err)
	}
	return nil
}

// ListProblemsForReview retrieves problems that need to be reviewed based on the lookback period
func (db *DB) ListProblemsForReview(ctx context.Context, userID string, lookbackPeriod time.Duration) ([]*ProblemEntry, error) {
	cutoff := time.Now().Add(-lookbackPeriod)
	query := `
	SELECT id, user_id, problem_name, link, difficulty, category, status,
	       solved_at, last_reviewed_at, review_count, notes
	FROM problems
	WHERE user_id = ? AND solved_at <= ? AND (last_reviewed_at IS NULL OR last_reviewed_at <= ?)
	ORDER BY solved_at ASC
	`

	rows, err := db.QueryContext(ctx, query, userID, cutoff, cutoff)
	if err != nil {
		return nil, fmt.Errorf("failed to list problems for review: %w", err)
	}
	defer rows.Close()

	var problems []*ProblemEntry
	for rows.Next() {
		var p ProblemEntry
		var solvedAt string
		var lastReviewedAt sql.NullString

		err := rows.Scan(
			&p.ID, &p.UserID, &p.ProblemName, &p.Link,
			&p.Difficulty, &p.Category, &p.Status, &solvedAt,
			&lastReviewedAt, &p.ReviewCount, &p.Notes,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan problem: %w", err)
		}

		// Parse timestamps
		parsedTime, err := time.Parse("2006-01-02 15:04:05", solvedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to parse solved_at time: %w", err)
		}
		p.SolvedAt = parsedTime

		if lastReviewedAt.Valid {
			reviewedTime, err := time.Parse("2006-01-02 15:04:05", lastReviewedAt.String)
			if err != nil {
				return nil, fmt.Errorf("failed to parse last_reviewed_at time: %w", err)
			}
			p.LastReviewedAt = &reviewedTime
		}

		// Fetch tags for each problem
		p.Tags, err = db.GetProblemTags(ctx, p.ID)
		if err != nil {
			log.Error().Err(err).Int("problem_id", p.ID).Msg("Failed to get tags for review problem")
			// Continue even if tags fail to load
		}

		problems = append(problems, &p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over review problems: %w", err)
	}

	return problems, nil
}

// IncrementReviewCount increments the review count and updates the last reviewed at timestamp
func (db *DB) IncrementReviewCount(ctx context.Context, problemID int) error {
	now := time.Now()
	_, err := db.ExecContext(ctx, `
		UPDATE problems
		SET review_count = review_count + 1,
			last_reviewed_at = ?
		WHERE id = ?`,
		now, problemID)
	if err != nil {
		return fmt.Errorf("failed to increment review count: %w", err)
	}
	return nil
}