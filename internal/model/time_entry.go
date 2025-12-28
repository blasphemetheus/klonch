package model

import (
	"time"
)

// TimeEntry represents a time tracking entry (manual or pomodoro)
type TimeEntry struct {
	ID          string     `json:"id"`
	TaskID      string     `json:"task_id"`
	Description string     `json:"description,omitempty"`
	StartedAt   time.Time  `json:"started_at"`
	EndedAt     *time.Time `json:"ended_at,omitempty"`
	Duration    *int       `json:"duration,omitempty"` // Minutes
	IsPomodoro  bool       `json:"is_pomodoro"`
	CreatedAt   time.Time  `json:"created_at"`
}

// CalculatedDuration returns the duration in minutes
// If Duration is set, returns that; otherwise calculates from StartedAt/EndedAt
func (te *TimeEntry) CalculatedDuration() int {
	if te.Duration != nil {
		return *te.Duration
	}
	if te.EndedAt == nil {
		// Still running, calculate from now
		return int(time.Since(te.StartedAt).Minutes())
	}
	return int(te.EndedAt.Sub(te.StartedAt).Minutes())
}

// IsRunning returns true if this time entry is still active
func (te *TimeEntry) IsRunning() bool {
	return te.EndedAt == nil
}
