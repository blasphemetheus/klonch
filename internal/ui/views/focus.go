package views

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dori/klonch/internal/db"
	"github.com/dori/klonch/internal/model"
	"github.com/dori/klonch/internal/notify"
	"github.com/dori/klonch/internal/ui/theme"
	"github.com/google/uuid"
)

// FocusState represents the timer state in focus mode
type FocusState int

const (
	FocusIdle FocusState = iota
	FocusRunning
	FocusPaused
)

// FocusView represents the focus mode for a single task
type FocusView struct {
	db       *db.DB
	notifier *notify.Notifier
	width    int
	height   int

	// The task being focused on
	task     *model.Task
	subtasks []model.Task
	project  *model.Project
	tags     []model.Tag

	// Time tracking
	timeLogged    int // Total minutes already logged
	timerState    FocusState
	timerStart    time.Time
	timerElapsed  time.Duration
	timeEntryID   string

	// Subtask navigation
	subtaskCursor int

	// Status
	statusMsg string
}

// NewFocusView creates a new focus view
func NewFocusView(database *db.DB, notifier *notify.Notifier) FocusView {
	return FocusView{
		db:       database,
		notifier: notifier,
	}
}

// Init initializes the focus view
func (v FocusView) Init() tea.Cmd {
	if v.task == nil {
		return nil
	}
	return v.loadTaskDetails()
}

// SetTask sets the task to focus on
func (v FocusView) SetTask(task *model.Task) FocusView {
	v.task = task
	v.timerState = FocusIdle
	v.timerElapsed = 0
	v.subtaskCursor = 0
	return v
}

// SetSize sets the view dimensions
func (v FocusView) SetSize(width, height int) FocusView {
	v.width = width
	v.height = height
	return v
}

// HasTask returns whether a task is set
func (v FocusView) HasTask() bool {
	return v.task != nil
}

// IsTimerRunning returns whether the timer is currently running
func (v FocusView) IsTimerRunning() bool {
	return v.timerState == FocusRunning
}

// loadTaskDetails loads full details for the focused task
func (v FocusView) loadTaskDetails() tea.Cmd {
	if v.task == nil {
		return nil
	}

	taskID := v.task.ID
	return func() tea.Msg {
		// Load subtasks
		subtaskRows, err := v.db.Query(`
			SELECT id, title, status, priority
			FROM tasks
			WHERE parent_id = ?
			ORDER BY position, created_at
		`, taskID)
		if err != nil {
			return focusErrorMsg{err: err}
		}
		defer subtaskRows.Close()

		var subtasks []model.Task
		for subtaskRows.Next() {
			var t model.Task
			if err := subtaskRows.Scan(&t.ID, &t.Title, &t.Status, &t.Priority); err == nil {
				subtasks = append(subtasks, t)
			}
		}

		// Load project
		var project *model.Project
		row := v.db.QueryRow(`
			SELECT p.id, p.name, p.color
			FROM projects p
			JOIN tasks t ON t.project_id = p.id
			WHERE t.id = ?
		`, taskID)
		var p model.Project
		if err := row.Scan(&p.ID, &p.Name, &p.Color); err == nil {
			project = &p
		}

		// Load tags
		tagRows, err := v.db.Query(`
			SELECT t.id, t.name, t.color
			FROM tags t
			JOIN task_tags tt ON tt.tag_id = t.id
			WHERE tt.task_id = ?
		`, taskID)
		if err != nil {
			return focusErrorMsg{err: err}
		}
		defer tagRows.Close()

		var tags []model.Tag
		for tagRows.Next() {
			var tag model.Tag
			var color *string
			if err := tagRows.Scan(&tag.ID, &tag.Name, &color); err == nil {
				if color != nil {
					tag.Color = *color
				}
				tags = append(tags, tag)
			}
		}

		// Load time logged
		var timeLogged int
		row = v.db.QueryRow(`
			SELECT COALESCE(SUM(duration), 0)
			FROM time_entries
			WHERE task_id = ?
		`, taskID)
		row.Scan(&timeLogged)

		return focusLoadedMsg{
			subtasks:   subtasks,
			project:    project,
			tags:       tags,
			timeLogged: timeLogged,
		}
	}
}

type focusErrorMsg struct{ err error }
type focusLoadedMsg struct {
	subtasks   []model.Task
	project    *model.Project
	tags       []model.Tag
	timeLogged int
}
type focusTickMsg struct{}

// BackToListMsg requests returning to list view from focus
type BackToListMsg struct{}

// tickCmd sends tick messages every second
func focusTickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return focusTickMsg{}
	})
}

// Update handles messages
func (v FocusView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case focusLoadedMsg:
		v.subtasks = msg.subtasks
		v.project = msg.project
		v.tags = msg.tags
		v.timeLogged = msg.timeLogged
		return v, nil

	case focusErrorMsg:
		v.statusMsg = fmt.Sprintf("Error: %v", msg.err)
		return v, nil

	case focusTickMsg:
		if v.timerState == FocusRunning {
			v.timerElapsed = time.Since(v.timerStart)
			return v, focusTickCmd()
		}
		return v, nil

	case taskUpdatedMsg:
		return v, v.loadTaskDetails()

	case tea.KeyMsg:
		switch msg.String() {
		// Timer controls
		case "s", " ": // Start/pause timer
			switch v.timerState {
			case FocusIdle:
				return v, v.startTimer()
			case FocusRunning:
				v.timerState = FocusPaused
				v.statusMsg = "Timer paused"
				return v, nil
			case FocusPaused:
				// Resume
				v.timerStart = time.Now().Add(-v.timerElapsed)
				v.timerState = FocusRunning
				v.statusMsg = "Timer resumed"
				return v, focusTickCmd()
			}

		case "r": // Reset timer
			if v.timerState != FocusIdle {
				v.timerState = FocusIdle
				v.timerElapsed = 0
				v.statusMsg = "Timer reset"
			}
			return v, nil

		case "S": // Stop and save time
			if v.timerState != FocusIdle {
				return v, v.stopAndSaveTimer()
			}
			return v, nil

		// Subtask navigation
		case "j", "down":
			if len(v.subtasks) > 0 && v.subtaskCursor < len(v.subtasks)-1 {
				v.subtaskCursor++
			}
			return v, nil

		case "k", "up":
			if v.subtaskCursor > 0 {
				v.subtaskCursor--
			}
			return v, nil

		// Subtask actions
		case "tab", "enter": // Toggle subtask
			if len(v.subtasks) > 0 {
				return v, v.toggleSubtask()
			}
			return v, nil

		case "d": // Mark main task done
			return v, v.markTaskDone()

		case "p": // Cycle priority
			return v, v.cyclePriority()

		case "esc", "q": // Return to list view
			return v, func() tea.Msg { return BackToListMsg{} }
		}
	}

	return v, nil
}

// startTimer starts the focus timer
func (v FocusView) startTimer() tea.Cmd {
	v.timerState = FocusRunning
	v.timerStart = time.Now()
	v.timerElapsed = 0
	v.statusMsg = "Timer started"

	// Create time entry
	if v.task != nil {
		v.timeEntryID = uuid.New().String()
		now := time.Now()
		v.db.Exec(`
			INSERT INTO time_entries (id, task_id, started_at, is_pomodoro, created_at)
			VALUES (?, ?, ?, 0, ?)
		`, v.timeEntryID, v.task.ID, now, now)
	}

	return focusTickCmd()
}

// stopAndSaveTimer stops the timer and saves the time entry
func (v FocusView) stopAndSaveTimer() tea.Cmd {
	duration := int(v.timerElapsed.Minutes())
	entryID := v.timeEntryID

	v.timerState = FocusIdle
	v.timerElapsed = 0
	v.timeEntryID = ""

	return func() tea.Msg {
		if entryID != "" && duration > 0 {
			now := time.Now()
			v.db.Exec(`
				UPDATE time_entries SET ended_at = ?, duration = ?
				WHERE id = ?
			`, now, duration, entryID)
		}
		return taskUpdatedMsg{}
	}
}

// toggleSubtask toggles the current subtask's done status
func (v FocusView) toggleSubtask() tea.Cmd {
	if len(v.subtasks) == 0 || v.subtaskCursor >= len(v.subtasks) {
		return nil
	}

	subtask := v.subtasks[v.subtaskCursor]
	newStatus := model.StatusDone
	if subtask.Status == model.StatusDone {
		newStatus = model.StatusPending
	}

	return func() tea.Msg {
		if newStatus == model.StatusDone {
			_, err := v.db.Exec(`
				UPDATE tasks SET status = ?, completed_at = datetime('now'), updated_at = datetime('now')
				WHERE id = ?
			`, newStatus, subtask.ID)
			if err != nil {
				return focusErrorMsg{err: err}
			}
		} else {
			_, err := v.db.Exec(`
				UPDATE tasks SET status = ?, completed_at = NULL, updated_at = datetime('now')
				WHERE id = ?
			`, newStatus, subtask.ID)
			if err != nil {
				return focusErrorMsg{err: err}
			}
		}
		return taskUpdatedMsg{}
	}
}

// markTaskDone marks the main task as done
func (v FocusView) markTaskDone() tea.Cmd {
	if v.task == nil {
		return nil
	}

	taskID := v.task.ID
	return func() tea.Msg {
		_, err := v.db.Exec(`
			UPDATE tasks SET status = 'done', completed_at = datetime('now'), updated_at = datetime('now')
			WHERE id = ?
		`, taskID)
		if err != nil {
			return focusErrorMsg{err: err}
		}

		// Send notification
		if v.notifier != nil {
			v.notifier.SendSimple("Task Complete!", v.task.Title)
		}

		return taskUpdatedMsg{}
	}
}

// cyclePriority cycles through priority levels
func (v FocusView) cyclePriority() tea.Cmd {
	if v.task == nil {
		return nil
	}

	priorities := []model.Priority{
		model.PriorityLow,
		model.PriorityMedium,
		model.PriorityHigh,
		model.PriorityUrgent,
	}

	currentIdx := 0
	for i, p := range priorities {
		if p == v.task.Priority {
			currentIdx = i
			break
		}
	}
	newPriority := priorities[(currentIdx+1)%len(priorities)]
	v.task.Priority = newPriority

	taskID := v.task.ID
	return func() tea.Msg {
		_, err := v.db.Exec(`
			UPDATE tasks SET priority = ?, updated_at = datetime('now')
			WHERE id = ?
		`, newPriority, taskID)
		if err != nil {
			return focusErrorMsg{err: err}
		}
		return taskUpdatedMsg{}
	}
}

// View renders the focus view
func (v FocusView) View() string {
	if v.width == 0 || v.height == 0 {
		return "Loading..."
	}

	if v.task == nil {
		return v.renderNoTask()
	}

	t := theme.Current.Theme

	var sections []string

	// Centered container
	containerWidth := min(80, v.width-4)

	// Task title (large, centered)
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Primary).
		Width(containerWidth).
		Align(lipgloss.Center).
		MarginBottom(1)

	sections = append(sections, titleStyle.Render(v.task.Title))

	// Status and priority badges
	badgeRow := v.renderBadges(containerWidth)
	sections = append(sections, badgeRow)
	sections = append(sections, "")

	// Timer display
	timerSection := v.renderTimer(containerWidth)
	sections = append(sections, timerSection)
	sections = append(sections, "")

	// Description (if any)
	if v.task.Description != "" {
		descStyle := lipgloss.NewStyle().
			Foreground(t.Foreground).
			Width(containerWidth).
			Padding(1, 2).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(t.Border)

		sections = append(sections, descStyle.Render(v.task.Description))
		sections = append(sections, "")
	}

	// Subtasks (if any)
	if len(v.subtasks) > 0 {
		subtaskSection := v.renderSubtasks(containerWidth)
		sections = append(sections, subtaskSection)
		sections = append(sections, "")
	}

	// Metadata (project, tags, due date, time logged)
	metaSection := v.renderMetadata(containerWidth)
	sections = append(sections, metaSection)

	// Status message
	if v.statusMsg != "" {
		statusStyle := lipgloss.NewStyle().
			Foreground(t.Info).
			Width(containerWidth).
			Align(lipgloss.Center).
			MarginTop(1)
		sections = append(sections, statusStyle.Render(v.statusMsg))
	}

	// Center the content
	content := strings.Join(sections, "\n")
	centered := lipgloss.NewStyle().
		Width(v.width).
		Align(lipgloss.Center).
		Render(content)

	return centered
}

// renderNoTask renders the view when no task is selected
func (v FocusView) renderNoTask() string {
	t := theme.Current.Theme

	style := lipgloss.NewStyle().
		Foreground(t.Subtle).
		Width(v.width).
		Height(v.height).
		Align(lipgloss.Center, lipgloss.Center)

	return style.Render("No task selected\n\nPress 'f' on a task in list view to focus on it")
}

// renderBadges renders status and priority badges
func (v FocusView) renderBadges(width int) string {
	t := theme.Current.Theme

	var badges []string

	// Status badge
	statusStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Bold(true)

	switch v.task.Status {
	case model.StatusDone:
		statusStyle = statusStyle.Background(t.Success).Foreground(lipgloss.Color("#000"))
		badges = append(badges, statusStyle.Render("DONE"))
	case model.StatusInProgress:
		statusStyle = statusStyle.Background(t.Info).Foreground(lipgloss.Color("#000"))
		badges = append(badges, statusStyle.Render("IN PROGRESS"))
	default:
		statusStyle = statusStyle.Background(t.Subtle).Foreground(lipgloss.Color("#000"))
		badges = append(badges, statusStyle.Render("PENDING"))
	}

	// Priority badge
	priorityStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Bold(true)

	switch v.task.Priority {
	case model.PriorityUrgent:
		priorityStyle = priorityStyle.Background(t.PriorityUrgent).Foreground(lipgloss.Color("#000"))
		badges = append(badges, priorityStyle.Render("URGENT"))
	case model.PriorityHigh:
		priorityStyle = priorityStyle.Background(t.PriorityHigh).Foreground(lipgloss.Color("#000"))
		badges = append(badges, priorityStyle.Render("HIGH"))
	case model.PriorityMedium:
		priorityStyle = priorityStyle.Background(t.PriorityMedium).Foreground(lipgloss.Color("#000"))
		badges = append(badges, priorityStyle.Render("MEDIUM"))
	case model.PriorityLow:
		priorityStyle = priorityStyle.Background(t.PriorityLow).Foreground(lipgloss.Color("#000"))
		badges = append(badges, priorityStyle.Render("LOW"))
	}

	row := strings.Join(badges, "  ")
	return lipgloss.NewStyle().
		Width(width).
		Align(lipgloss.Center).
		Render(row)
}

// renderTimer renders the timer display
func (v FocusView) renderTimer(width int) string {
	t := theme.Current.Theme

	// Calculate display time
	elapsed := v.timerElapsed
	hours := int(elapsed.Hours())
	mins := int(elapsed.Minutes()) % 60
	secs := int(elapsed.Seconds()) % 60

	timeStr := fmt.Sprintf("%02d:%02d:%02d", hours, mins, secs)

	// Timer color based on state
	var timerColor lipgloss.Color
	var stateLabel string
	switch v.timerState {
	case FocusRunning:
		timerColor = t.Success
		stateLabel = "RUNNING"
	case FocusPaused:
		timerColor = t.Warning
		stateLabel = "PAUSED"
	default:
		timerColor = t.Subtle
		stateLabel = "READY"
	}

	// Big timer display
	timerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(timerColor).
		Padding(1, 4).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(timerColor)

	labelStyle := lipgloss.NewStyle().
		Foreground(timerColor).
		Bold(true)

	timerBox := timerStyle.Render(timeStr)

	// Timer controls hint
	var hint string
	switch v.timerState {
	case FocusIdle:
		hint = "s/space: start"
	case FocusRunning:
		hint = "s/space: pause • S: stop & save • r: reset"
	case FocusPaused:
		hint = "s/space: resume • S: stop & save • r: reset"
	}
	hintStyle := lipgloss.NewStyle().Foreground(t.Subtle)

	content := lipgloss.JoinVertical(lipgloss.Center,
		labelStyle.Render(stateLabel),
		timerBox,
		hintStyle.Render(hint),
	)

	return lipgloss.NewStyle().
		Width(width).
		Align(lipgloss.Center).
		Render(content)
}

// renderSubtasks renders the subtask list
func (v FocusView) renderSubtasks(width int) string {
	t := theme.Current.Theme

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Secondary)

	// Count completed
	completed := 0
	for _, st := range v.subtasks {
		if st.Status == model.StatusDone {
			completed++
		}
	}

	header := headerStyle.Render(fmt.Sprintf("Subtasks (%d/%d)", completed, len(v.subtasks)))

	var lines []string
	lines = append(lines, header)

	for i, st := range v.subtasks {
		isCursor := i == v.subtaskCursor

		itemStyle := lipgloss.NewStyle().Width(width - 4)
		if isCursor {
			itemStyle = itemStyle.Background(t.Highlight).Bold(true)
		}

		checkbox := "[ ]"
		textStyle := lipgloss.NewStyle()
		if st.Status == model.StatusDone {
			checkbox = "[x]"
			textStyle = textStyle.Strikethrough(true).Foreground(t.Subtle)
		}

		line := fmt.Sprintf("  %s %s", checkbox, textStyle.Render(st.Title))
		lines = append(lines, itemStyle.Render(line))
	}

	boxStyle := lipgloss.NewStyle().
		Width(width).
		Padding(0, 2).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(t.Border)

	return boxStyle.Render(strings.Join(lines, "\n"))
}

// renderMetadata renders project, tags, due date, time logged
func (v FocusView) renderMetadata(width int) string {
	t := theme.Current.Theme

	var items []string

	labelStyle := lipgloss.NewStyle().Foreground(t.Subtle)
	valueStyle := lipgloss.NewStyle().Foreground(t.Foreground)

	// Project
	if v.project != nil {
		items = append(items, labelStyle.Render("Project: ")+valueStyle.Render(v.project.Name))
	}

	// Tags
	if len(v.tags) > 0 {
		var tagNames []string
		for _, tag := range v.tags {
			tagNames = append(tagNames, "@"+tag.Name)
		}
		items = append(items, labelStyle.Render("Tags: ")+valueStyle.Render(strings.Join(tagNames, ", ")))
	}

	// Due date
	if v.task.DueDate != nil {
		dueStr := v.task.DueDate.Format("Mon, Jan 2")
		daysUntil := int(time.Until(*v.task.DueDate).Hours() / 24)

		dueLabelStyle := valueStyle
		if daysUntil < 0 {
			dueLabelStyle = lipgloss.NewStyle().Foreground(t.Error)
			dueStr += fmt.Sprintf(" (%d days overdue)", -daysUntil)
		} else if daysUntil == 0 {
			dueLabelStyle = lipgloss.NewStyle().Foreground(t.Warning)
			dueStr += " (today)"
		} else if daysUntil == 1 {
			dueStr += " (tomorrow)"
		}

		items = append(items, labelStyle.Render("Due: ")+dueLabelStyle.Render(dueStr))
	}

	// Time logged
	totalMins := v.timeLogged
	if v.timerState == FocusRunning {
		totalMins += int(v.timerElapsed.Minutes())
	}
	hours := totalMins / 60
	mins := totalMins % 60
	timeStr := fmt.Sprintf("%dh %dm", hours, mins)
	items = append(items, labelStyle.Render("Time logged: ")+valueStyle.Render(timeStr))

	// Join with separator
	content := strings.Join(items, "  •  ")
	return lipgloss.NewStyle().
		Width(width).
		Align(lipgloss.Center).
		Foreground(t.Subtle).
		Render(content)
}

// IsInputMode returns whether the view is in input mode
// Returns true to prevent global 'q' from quitting - focus view handles its own exit
func (v FocusView) IsInputMode() bool {
	return true
}
