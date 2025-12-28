package ui

import (
	"github.com/dori/klonch/internal/model"
)

// View represents the current active view
type View int

const (
	ViewList View = iota
	ViewKanban
	ViewEisenhower
	ViewCalendar
	ViewPomodoro
	ViewPlanning
	ViewReview
	ViewStats
	ViewFocus
	ViewHelp
)

// String returns the display name for a view
func (v View) String() string {
	switch v {
	case ViewList:
		return "List"
	case ViewKanban:
		return "Kanban"
	case ViewEisenhower:
		return "Eisenhower"
	case ViewCalendar:
		return "Calendar"
	case ViewPomodoro:
		return "Pomodoro"
	case ViewPlanning:
		return "Planning"
	case ViewReview:
		return "Review"
	case ViewStats:
		return "Stats"
	case ViewFocus:
		return "Focus"
	case ViewHelp:
		return "Help"
	default:
		return "Unknown"
	}
}

// Messages for inter-component communication

// SwitchViewMsg requests a view change
type SwitchViewMsg struct {
	View View
}

// TasksLoadedMsg contains loaded tasks
type TasksLoadedMsg struct {
	Tasks []model.Task
	Err   error
}

// ProjectsLoadedMsg contains loaded projects
type ProjectsLoadedMsg struct {
	Projects []model.Project
	Err      error
}

// TagsLoadedMsg contains loaded tags
type TagsLoadedMsg struct {
	Tags []model.Tag
	Err  error
}

// TaskCreatedMsg indicates a task was created
type TaskCreatedMsg struct {
	Task model.Task
	Err  error
}

// TaskUpdatedMsg indicates a task was updated
type TaskUpdatedMsg struct {
	Task model.Task
	Err  error
}

// TaskDeletedMsg indicates a task was deleted
type TaskDeletedMsg struct {
	TaskID string
	Err    error
}

// TasksToggledMsg indicates task completion was toggled
type TasksToggledMsg struct {
	TaskIDs []string
	Done    bool
	Err     error
}

// ErrorMsg contains an error to display
type ErrorMsg struct {
	Err error
}

// StatusMsg contains a status message to display
type StatusMsg struct {
	Message string
}

// ThemeChangedMsg indicates the theme was changed
type ThemeChangedMsg struct {
	ThemeName string
}

// PomodoroTickMsg is sent every second during pomodoro
type PomodoroTickMsg struct{}

// PomodoroCompleteMsg indicates pomodoro session completed
type PomodoroCompleteMsg struct {
	TaskID   string
	Duration int // minutes
}

// RefreshMsg requests data refresh
type RefreshMsg struct{}

// FocusTaskMsg requests focus mode for a specific task
type FocusTaskMsg struct {
	Task model.Task
}
