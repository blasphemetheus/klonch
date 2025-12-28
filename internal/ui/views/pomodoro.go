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

// Timer durations
const (
	PomodoroWork       = 25 * time.Minute
	PomodoroShortBreak = 5 * time.Minute
	PomodoroLongBreak  = 15 * time.Minute
)

// PomodoroState represents the timer state
type PomodoroState int

const (
	PomodoroIdle PomodoroState = iota
	PomodoroRunning
	PomodoroPaused
	PomodoroBreak
)

// PomodoroView represents the Pomodoro timer view
type PomodoroView struct {
	db       *db.DB
	notifier *notify.Notifier
	width    int
	height   int

	// Available tasks
	tasks       []model.Task
	taskCursor  int
	selectedTask *model.Task

	// Timer state
	state        PomodoroState
	duration     time.Duration // Total duration for current session
	remaining    time.Duration // Time remaining
	startedAt    time.Time     // When current session started
	pausedAt     time.Time     // When paused (for resume calculation)

	// Session tracking
	completedPomodoros int
	currentEntryID     string // ID of current time entry being recorded

	// Status
	statusMsg string
}

// NewPomodoroView creates a new Pomodoro view
func NewPomodoroView(database *db.DB, notifier *notify.Notifier) PomodoroView {
	return PomodoroView{
		db:        database,
		notifier:  notifier,
		duration:  PomodoroWork,
		remaining: PomodoroWork,
		state:     PomodoroIdle,
	}
}

// Init initializes the Pomodoro view
func (v PomodoroView) Init() tea.Cmd {
	return v.loadTasks()
}

// SetSize sets the view dimensions
func (v PomodoroView) SetSize(width, height int) PomodoroView {
	v.width = width
	v.height = height
	return v
}

// loadTasks loads pending tasks for selection
func (v PomodoroView) loadTasks() tea.Cmd {
	return func() tea.Msg {
		rows, err := v.db.Query(`
			SELECT id, title, status, priority, project_id
			FROM tasks
			WHERE parent_id IS NULL
			  AND status IN ('pending', 'in_progress')
			ORDER BY
				CASE priority
					WHEN 'urgent' THEN 1
					WHEN 'high' THEN 2
					WHEN 'medium' THEN 3
					WHEN 'low' THEN 4
				END,
				created_at
			LIMIT 20
		`)
		if err != nil {
			return pomodoroErrorMsg{err: err}
		}
		defer rows.Close()

		var tasks []model.Task
		for rows.Next() {
			var t model.Task
			var projectID *string
			if err := rows.Scan(&t.ID, &t.Title, &t.Status, &t.Priority, &projectID); err != nil {
				continue
			}
			t.ProjectID = projectID
			tasks = append(tasks, t)
		}

		return pomodoroTasksLoadedMsg{tasks: tasks}
	}
}

type pomodoroErrorMsg struct{ err error }
type pomodoroTasksLoadedMsg struct{ tasks []model.Task }
type pomodoroTickMsg struct{}
type pomodoroCompleteMsg struct{}

// tickCmd sends tick messages every second
func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return pomodoroTickMsg{}
	})
}

// Update handles messages
func (v PomodoroView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case pomodoroTasksLoadedMsg:
		v.tasks = msg.tasks
		return v, nil

	case pomodoroTickMsg:
		if v.state == PomodoroRunning || v.state == PomodoroBreak {
			elapsed := time.Since(v.startedAt)
			v.remaining = v.duration - elapsed

			if v.remaining <= 0 {
				v.remaining = 0
				return v, v.completeSession()
			}
			return v, tickCmd()
		}
		return v, nil

	case pomodoroCompleteMsg:
		if v.state == PomodoroRunning {
			// Completed a work session
			v.completedPomodoros++
			v.state = PomodoroIdle
			v.statusMsg = fmt.Sprintf("Pomodoro #%d complete! Take a break.", v.completedPomodoros)

			// Record time entry
			if v.selectedTask != nil {
				v.recordTimeEntry()
			}

			// Send notification
			if v.notifier != nil {
				taskTitle := ""
				if v.selectedTask != nil {
					taskTitle = v.selectedTask.Title
				}
				v.notifier.SendPomodoroComplete(taskTitle, int(PomodoroWork.Minutes()))
			}
		} else if v.state == PomodoroBreak {
			v.state = PomodoroIdle
			v.statusMsg = "Break over! Ready for next pomodoro."

			// Send notification
			if v.notifier != nil {
				v.notifier.SendBreakComplete()
			}
		}
		v.remaining = PomodoroWork
		v.duration = PomodoroWork
		return v, nil

	case taskUpdatedMsg:
		return v, v.loadTasks()

	case tea.KeyMsg:
		switch msg.String() {
		// Task selection
		case "j", "down":
			if v.state == PomodoroIdle && v.taskCursor < len(v.tasks)-1 {
				v.taskCursor++
			}
			return v, nil

		case "k", "up":
			if v.state == PomodoroIdle && v.taskCursor > 0 {
				v.taskCursor--
			}
			return v, nil

		case "enter":
			if v.state == PomodoroIdle && len(v.tasks) > 0 {
				v.selectedTask = &v.tasks[v.taskCursor]
			}
			return v, nil

		// Timer controls
		case "s", " ": // Start/pause
			switch v.state {
			case PomodoroIdle:
				return v, v.startTimer(PomodoroWork)
			case PomodoroRunning:
				v.state = PomodoroPaused
				v.pausedAt = time.Now()
				v.statusMsg = "Paused"
				return v, nil
			case PomodoroPaused:
				// Resume: adjust startedAt by pause duration
				pauseDuration := time.Since(v.pausedAt)
				v.startedAt = v.startedAt.Add(pauseDuration)
				v.state = PomodoroRunning
				v.statusMsg = "Resumed"
				return v, tickCmd()
			}

		case "r": // Reset
			v.state = PomodoroIdle
			v.remaining = PomodoroWork
			v.duration = PomodoroWork
			v.statusMsg = "Timer reset"
			return v, nil

		case "b": // Short break
			if v.state == PomodoroIdle {
				return v, v.startBreak(PomodoroShortBreak)
			}

		case "B": // Long break
			if v.state == PomodoroIdle {
				return v, v.startBreak(PomodoroLongBreak)
			}

		case "c": // Clear selected task
			v.selectedTask = nil
			v.statusMsg = "Task cleared"
			return v, nil

		case "g":
			v.taskCursor = 0
			return v, nil

		case "G":
			if len(v.tasks) > 0 {
				v.taskCursor = len(v.tasks) - 1
			}
			return v, nil
		}
	}

	return v, nil
}

// startTimer starts a work session
func (v PomodoroView) startTimer(duration time.Duration) tea.Cmd {
	v.state = PomodoroRunning
	v.duration = duration
	v.remaining = duration
	v.startedAt = time.Now()
	v.statusMsg = "Focus time started!"

	// Create time entry
	if v.selectedTask != nil {
		v.currentEntryID = uuid.New().String()
		v.db.Exec(`
			INSERT INTO time_entries (id, task_id, started_at, is_pomodoro)
			VALUES (?, ?, ?, 1)
		`, v.currentEntryID, v.selectedTask.ID, v.startedAt)
	}

	return tickCmd()
}

// startBreak starts a break session
func (v PomodoroView) startBreak(duration time.Duration) tea.Cmd {
	v.state = PomodoroBreak
	v.duration = duration
	v.remaining = duration
	v.startedAt = time.Now()
	if duration == PomodoroShortBreak {
		v.statusMsg = "Short break started"
	} else {
		v.statusMsg = "Long break started"
	}
	return tickCmd()
}

// completeSession handles session completion
func (v PomodoroView) completeSession() tea.Cmd {
	return func() tea.Msg {
		return pomodoroCompleteMsg{}
	}
}

// recordTimeEntry saves the completed time entry
func (v *PomodoroView) recordTimeEntry() {
	if v.currentEntryID == "" {
		return
	}

	now := time.Now()
	duration := int(PomodoroWork.Minutes())

	v.db.Exec(`
		UPDATE time_entries
		SET ended_at = ?, duration = ?
		WHERE id = ?
	`, now, duration, v.currentEntryID)

	v.currentEntryID = ""
}

// View renders the Pomodoro view
func (v PomodoroView) View() string {
	if v.width == 0 || v.height == 0 {
		return "Loading..."
	}

	t := theme.Current.Theme

	var sections []string

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Primary).
		MarginBottom(1)
	sections = append(sections, titleStyle.Render("Pomodoro Timer"))

	// Timer display
	timerSection := v.renderTimer()
	sections = append(sections, timerSection)

	// Selected task
	if v.selectedTask != nil {
		taskStyle := lipgloss.NewStyle().
			Foreground(t.Info).
			MarginTop(1)
		sections = append(sections, taskStyle.Render(fmt.Sprintf("Working on: %s", v.selectedTask.Title)))
	}

	// Session stats
	statsStyle := lipgloss.NewStyle().
		Foreground(t.Subtle).
		MarginTop(1)
	tomatoes := strings.Repeat("🍅", v.completedPomodoros)
	if tomatoes == "" {
		tomatoes = "(none yet)"
	}
	sections = append(sections, statsStyle.Render(fmt.Sprintf("Completed today: %s", tomatoes)))

	// Task list (when idle)
	if v.state == PomodoroIdle && len(v.tasks) > 0 {
		taskList := v.renderTaskList()
		sections = append(sections, taskList)
	}

	// Status message
	if v.statusMsg != "" {
		statusStyle := lipgloss.NewStyle().
			Foreground(t.Success).
			MarginTop(1)
		sections = append(sections, statusStyle.Render(v.statusMsg))
	}

	// Controls hint
	controls := v.renderControls()
	sections = append(sections, controls)

	return strings.Join(sections, "\n")
}

// renderTimer renders the timer display
func (v PomodoroView) renderTimer() string {
	t := theme.Current.Theme

	minutes := int(v.remaining.Minutes())
	seconds := int(v.remaining.Seconds()) % 60

	// Big time display
	timeStr := fmt.Sprintf("%02d:%02d", minutes, seconds)

	// Color based on state
	var color lipgloss.Color
	switch v.state {
	case PomodoroRunning:
		color = t.Error // Red for focus
	case PomodoroBreak:
		color = t.Success // Green for break
	case PomodoroPaused:
		color = t.Warning // Yellow for paused
	default:
		color = t.Foreground
	}

	timeStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(color).
		MarginTop(1).
		MarginBottom(1)

	// ASCII art style big digits
	bigTime := lipgloss.NewStyle().
		Bold(true).
		Foreground(color).
		Padding(1, 4).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(color)

	// Progress bar
	progress := 1.0 - (float64(v.remaining) / float64(v.duration))
	barWidth := 30
	filled := int(progress * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
	barStyle := lipgloss.NewStyle().Foreground(color)

	// State label
	var stateLabel string
	switch v.state {
	case PomodoroRunning:
		stateLabel = "FOCUS"
	case PomodoroBreak:
		stateLabel = "BREAK"
	case PomodoroPaused:
		stateLabel = "PAUSED"
	default:
		stateLabel = "READY"
	}

	labelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(color)

	return lipgloss.JoinVertical(lipgloss.Center,
		labelStyle.Render(stateLabel),
		bigTime.Render(timeStyle.Render(timeStr)),
		barStyle.Render(bar),
	)
}

// renderTaskList renders the task selection list
func (v PomodoroView) renderTaskList() string {
	t := theme.Current.Theme

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Secondary).
		MarginTop(2)

	var lines []string
	lines = append(lines, headerStyle.Render("Select a task to focus on:"))

	maxShow := 8
	for i, task := range v.tasks {
		if i >= maxShow {
			lines = append(lines, lipgloss.NewStyle().
				Foreground(t.Subtle).
				Render(fmt.Sprintf("  ... +%d more", len(v.tasks)-maxShow)))
			break
		}

		isSelected := i == v.taskCursor
		isCurrent := v.selectedTask != nil && v.selectedTask.ID == task.ID

		itemStyle := lipgloss.NewStyle()
		if isSelected {
			itemStyle = itemStyle.Background(t.Highlight).Bold(true)
		}
		if isCurrent {
			itemStyle = itemStyle.Foreground(t.Success)
		}

		// Priority indicator
		priorityChar := ""
		switch task.Priority {
		case model.PriorityUrgent:
			priorityChar = "!"
		case model.PriorityHigh:
			priorityChar = "▲"
		case model.PriorityMedium:
			priorityChar = "●"
		case model.PriorityLow:
			priorityChar = "▽"
		}

		cursor := "  "
		if isSelected {
			cursor = "> "
		}

		line := fmt.Sprintf("%s%s %s", cursor, priorityChar, task.Title)
		lines = append(lines, itemStyle.Render(line))
	}

	return strings.Join(lines, "\n")
}

// renderControls renders the control hints
func (v PomodoroView) renderControls() string {
	t := theme.Current.Theme

	controlStyle := lipgloss.NewStyle().
		Foreground(t.Subtle).
		MarginTop(2)

	var controls string
	switch v.state {
	case PomodoroIdle:
		controls = "s/space start • b short break • B long break • j/k select task • enter pick task"
	case PomodoroRunning:
		controls = "s/space pause • r reset"
	case PomodoroPaused:
		controls = "s/space resume • r reset"
	case PomodoroBreak:
		controls = "r reset (end break early)"
	}

	return controlStyle.Render(controls)
}

// IsInputMode returns whether the view is in input mode
func (v PomodoroView) IsInputMode() bool {
	return false
}
