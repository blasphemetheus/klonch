package model

import (
	"time"
)

// Project represents a task list/project
type Project struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Color     string    `json:"color,omitempty"`
	Archived  bool      `json:"archived"`
	Position  int       `json:"position"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Computed fields (not stored)
	TaskCount     int `json:"task_count,omitempty"`
	CompletedCount int `json:"completed_count,omitempty"`
}

// IsInbox returns true if this is the default inbox project
func (p *Project) IsInbox() bool {
	return p.ID == "inbox"
}
