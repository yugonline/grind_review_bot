package database

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
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

// Problem represents a solved problem in the database
type Problem struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	UserID         string         `gorm:"index:idx_user_id;not null" json:"user_id"`
	ProblemName    string         `gorm:"not null" json:"problem_name"`
	Link           string         `json:"link"`
	Difficulty     string         `gorm:"index:idx_difficulty;not null" json:"difficulty"`
	Category       string         `gorm:"index:idx_category;not null" json:"category"`
	Status         string         `gorm:"index:idx_status;not null" json:"status"`
	SolvedAt       time.Time      `gorm:"index:idx_solved_at;not null" json:"solved_at"`
	LastReviewedAt *time.Time     `json:"last_reviewed_at"`
	ReviewCount    int            `gorm:"default:0;not null" json:"review_count"`
	Notes          string         `json:"notes"`
	Tags           []Tag          `gorm:"many2many:problem_tags;" json:"tags,omitempty"`
	CreatedAt      time.Time      `gorm:"autoCreateTime" json:"-"`
	UpdatedAt      time.Time      `gorm:"autoUpdateTime" json:"-"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName explicitly sets the table name for Problem
func (Problem) TableName() string {
	return "problems"
}

// Tag represents a tag in the database
type Tag struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Name      string         `gorm:"uniqueIndex;not null" json:"name"`
	Problems  []Problem      `gorm:"many2many:problem_tags;" json:"-"`
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"-"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"-"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName explicitly sets the table name for Tag
func (Tag) TableName() string {
	return "tags"
}

// ProblemEntry is a DTO (Data Transfer Object) used for API interactions
type ProblemEntry struct {
	ID             uint       `json:"id"`
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

// ToProblem converts a ProblemEntry to Problem model with related Tag entities
func (p *ProblemEntry) ToProblem() *Problem {
	tags := make([]Tag, 0, len(p.Tags))
	for _, tagName := range p.Tags {
		tagName = strings.TrimSpace(tagName)
		if tagName != "" {
			tags = append(tags, Tag{Name: tagName})
		}
	}

	return &Problem{
		ID:             p.ID,
		UserID:         p.UserID,
		ProblemName:    p.ProblemName,
		Link:           p.Link,
		Difficulty:     p.Difficulty,
		Category:       p.Category,
		Status:         p.Status,
		SolvedAt:       p.SolvedAt,
		LastReviewedAt: p.LastReviewedAt,
		ReviewCount:    p.ReviewCount,
		Notes:          p.Notes,
		Tags:           tags,
	}
}

// FromProblem converts a Problem model to ProblemEntry DTO
func FromProblem(p *Problem) *ProblemEntry {
	tags := make([]string, 0, len(p.Tags))
	for _, tag := range p.Tags {
		tags = append(tags, tag.Name)
	}

	return &ProblemEntry{
		ID:             p.ID,
		UserID:         p.UserID,
		ProblemName:    p.ProblemName,
		Link:           p.Link,
		Difficulty:     p.Difficulty,
		Category:       p.Category,
		Status:         p.Status,
		SolvedAt:       p.SolvedAt,
		LastReviewedAt: p.LastReviewedAt,
		ReviewCount:    p.ReviewCount,
		Notes:          p.Notes,
		Tags:           tags,
	}
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