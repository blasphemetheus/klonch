package model

import (
	"time"
)

// Status represents the current state of a task
type Status string

const (
	StatusBacklog    Status = "backlog"
	StatusPending    Status = "pending"
	StatusInProgress Status = "in_progress"
	StatusDone       Status = "done"
	StatusArchived   Status = "archived"
)

// Priority represents task priority level
type Priority string

const (
	PriorityLow    Priority = "low"
	PriorityMedium Priority = "medium"
	PriorityHigh   Priority = "high"
	PriorityUrgent Priority = "urgent"
)

// Task represents a todo item
type Task struct {
	ID           string     `json:"id"`
	Title        string     `json:"title"`
	Description  string     `json:"description,omitempty"`
	Status       Status     `json:"status"`
	Priority     Priority   `json:"priority"`
	Urgency      bool       `json:"urgency"`      // For Eisenhower matrix
	Importance   bool       `json:"importance"`   // For Eisenhower matrix
	ProjectID    *string    `json:"project_id,omitempty"`
	ParentID     *string    `json:"parent_id,omitempty"` // For subtasks
	DueDate      *time.Time `json:"due_date,omitempty"`
	StartDate    *time.Time `json:"start_date,omitempty"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	TimeEstimate *int       `json:"time_estimate,omitempty"` // Minutes
	Recurrence   *string    `json:"recurrence,omitempty"`    // JSON string
	Position     int        `json:"position"`
	GCalEventID  *string    `json:"gcal_event_id,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`

	// Loaded relationships (not stored in tasks table)
	Tags         []Tag  `json:"tags,omitempty"`
	Subtasks     []Task `json:"subtasks,omitempty"`
	Dependencies []Task `json:"dependencies,omitempty"`
	Project      *Project `json:"project,omitempty"`
}

// IsOverdue returns true if the task is past its due date
func (t *Task) IsOverdue() bool {
	if t.DueDate == nil || t.Status == StatusDone || t.Status == StatusArchived {
		return false
	}
	return time.Now().After(*t.DueDate)
}

// IsDueToday returns true if the task is due today
func (t *Task) IsDueToday() bool {
	if t.DueDate == nil {
		return false
	}
	now := time.Now()
	return t.DueDate.Year() == now.Year() &&
		t.DueDate.YearDay() == now.YearDay()
}

// IsVisible returns true if the task should be shown based on start date
func (t *Task) IsVisible() bool {
	if t.StartDate == nil {
		return true
	}
	return time.Now().After(*t.StartDate) || time.Now().Equal(*t.StartDate)
}

// EisenhowerQuadrant returns which quadrant the task belongs to
// 1: Urgent + Important (Do First)
// 2: Urgent + Not Important (Delegate)
// 3: Not Urgent + Important (Schedule)
// 4: Not Urgent + Not Important (Eliminate)
func (t *Task) EisenhowerQuadrant() int {
	if t.Urgency && t.Importance {
		return 1
	}
	if t.Urgency && !t.Importance {
		return 2
	}
	if !t.Urgency && t.Importance {
		return 3
	}
	return 4
}

// PriorityWeight returns a numeric weight for sorting by priority
func (t *Task) PriorityWeight() int {
	switch t.Priority {
	case PriorityUrgent:
		return 4
	case PriorityHigh:
		return 3
	case PriorityMedium:
		return 2
	case PriorityLow:
		return 1
	default:
		return 2
	}
}
