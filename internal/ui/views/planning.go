package views

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dori/klonch/internal/db"
	"github.com/dori/klonch/internal/model"
	"github.com/dori/klonch/internal/ui/theme"
)

// Local message types for planning view
type planningErrorMsg struct{ err error }

// PlanningSection represents a section of tasks in the planning view
type PlanningSection int

const (
	SectionOverdue PlanningSection = iota
	SectionUndated
	SectionToday
)

// PlanningView represents the daily planning view
type PlanningView struct {
	db     *db.DB
	width  int
	height int

	// Tasks organized by section
	overdueTasks  []model.Task
	undatedTasks  []model.Task
	todayTasks    []model.Task

	// Navigation
	currentSection PlanningSection
	cursor         int
	selected       map[string]bool // Multi-select for batch operations

	// Status
	statusMsg string
}

// NewPlanningView creates a new planning view
func NewPlanningView(database *db.DB) PlanningView {
	return PlanningView{
		db:       database,
		selected: make(map[string]bool),
	}
}

// Init initializes the planning view
func (v PlanningView) Init() tea.Cmd {
	return v.loadTasks()
}

// SetSize sets the view dimensions
func (v PlanningView) SetSize(width, height int) PlanningView {
	v.width = width
	v.height = height
	return v
}

// loadTasks loads tasks for daily planning
func (v PlanningView) loadTasks() tea.Cmd {
	return func() tea.Msg {
		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
		tomorrow := today.AddDate(0, 0, 1)

		// Load overdue tasks
		overdueRows, err := v.db.Query(`
			SELECT id, title, description, status, priority, due_date, project_id
			FROM tasks
			WHERE parent_id IS NULL
			  AND status IN ('pending', 'in_progress')
			  AND due_date IS NOT NULL
			  AND due_date < ?
			ORDER BY due_date, priority DESC
		`, today.Format("2006-01-02"))
		if err != nil {
			return planningErrorMsg{err: err}
		}
		defer overdueRows.Close()

		var overdueTasks []model.Task
		for overdueRows.Next() {
			t := scanPlanningTask(overdueRows)
			if t != nil {
				overdueTasks = append(overdueTasks, *t)
			}
		}

		// Load undated tasks (no due date)
		undatedRows, err := v.db.Query(`
			SELECT id, title, description, status, priority, due_date, project_id
			FROM tasks
			WHERE parent_id IS NULL
			  AND status IN ('pending', 'in_progress')
			  AND due_date IS NULL
			ORDER BY
				CASE priority
					WHEN 'urgent' THEN 1
					WHEN 'high' THEN 2
					WHEN 'medium' THEN 3
					WHEN 'low' THEN 4
				END,
				created_at DESC
			LIMIT 30
		`)
		if err != nil {
			return planningErrorMsg{err: err}
		}
		defer undatedRows.Close()

		var undatedTasks []model.Task
		for undatedRows.Next() {
			t := scanPlanningTask(undatedRows)
			if t != nil {
				undatedTasks = append(undatedTasks, *t)
			}
		}

		// Load today's tasks
		todayRows, err := v.db.Query(`
			SELECT id, title, description, status, priority, due_date, project_id
			FROM tasks
			WHERE parent_id IS NULL
			  AND status IN ('pending', 'in_progress')
			  AND due_date >= ?
			  AND due_date < ?
			ORDER BY
				CASE priority
					WHEN 'urgent' THEN 1
					WHEN 'high' THEN 2
					WHEN 'medium' THEN 3
					WHEN 'low' THEN 4
				END,
				created_at
		`, today.Format("2006-01-02"), tomorrow.Format("2006-01-02"))
		if err != nil {
			return planningErrorMsg{err: err}
		}
		defer todayRows.Close()

		var todayTasks []model.Task
		for todayRows.Next() {
			t := scanPlanningTask(todayRows)
			if t != nil {
				todayTasks = append(todayTasks, *t)
			}
		}

		return planningLoadedMsg{
			overdue: overdueTasks,
			undated: undatedTasks,
			today:   todayTasks,
		}
	}
}

// scanPlanningTask scans a task row for planning view
func scanPlanningTask(rows interface{ Scan(...interface{}) error }) *model.Task {
	var t model.Task
	var desc, dueDate, projectID *string
	if err := rows.Scan(&t.ID, &t.Title, &desc, &t.Status, &t.Priority, &dueDate, &projectID); err != nil {
		return nil
	}
	if desc != nil {
		t.Description = *desc
	}
	if dueDate != nil {
		if parsed, err := time.Parse("2006-01-02 15:04:05", *dueDate); err == nil {
			t.DueDate = &parsed
		} else if parsed, err := time.Parse("2006-01-02", *dueDate); err == nil {
			t.DueDate = &parsed
		}
	}
	t.ProjectID = projectID
	return &t
}

type planningLoadedMsg struct {
	overdue []model.Task
	undated []model.Task
	today   []model.Task
}

// Update handles messages
func (v PlanningView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case planningLoadedMsg:
		v.overdueTasks = msg.overdue
		v.undatedTasks = msg.undated
		v.todayTasks = msg.today
		v.clampCursor()
		return v, nil

	case planningErrorMsg:
		v.statusMsg = fmt.Sprintf("Error: %v", msg.err)
		return v, nil

	case taskUpdatedMsg:
		return v, v.loadTasks()

	case tea.KeyMsg:
		switch msg.String() {
		// Navigation between sections
		case "tab":
			v.nextSection()
			return v, nil

		case "shift+tab":
			v.prevSection()
			return v, nil

		// Navigation within section
		case "j", "down":
			v.moveCursor(1)
			return v, nil

		case "k", "up":
			v.moveCursor(-1)
			return v, nil

		case "g":
			v.cursor = 0
			return v, nil

		case "G":
			tasks := v.currentTasks()
			if len(tasks) > 0 {
				v.cursor = len(tasks) - 1
			}
			return v, nil

		// Selection
		case " ":
			if task := v.currentTask(); task != nil {
				if v.selected[task.ID] {
					delete(v.selected, task.ID)
				} else {
					v.selected[task.ID] = true
				}
			}
			return v, nil

		// Actions
		case "t", "enter": // Assign to today
			return v, v.assignToToday()

		case "T": // Assign to tomorrow
			return v, v.assignToTomorrow()

		case "x": // Remove due date
			return v, v.removeDueDate()

		case "d": // Mark as done
			return v, v.markDone()

		case "r": // Refresh
			return v, v.loadTasks()

		case "c": // Clear selection
			v.selected = make(map[string]bool)
			v.statusMsg = "Selection cleared"
			return v, nil
		}
	}

	return v, nil
}

// currentTasks returns tasks in the current section
func (v PlanningView) currentTasks() []model.Task {
	switch v.currentSection {
	case SectionOverdue:
		return v.overdueTasks
	case SectionUndated:
		return v.undatedTasks
	case SectionToday:
		return v.todayTasks
	}
	return nil
}

// currentTask returns the currently highlighted task
func (v PlanningView) currentTask() *model.Task {
	tasks := v.currentTasks()
	if len(tasks) == 0 || v.cursor >= len(tasks) {
		return nil
	}
	return &tasks[v.cursor]
}

// nextSection moves to the next section
func (v *PlanningView) nextSection() {
	switch v.currentSection {
	case SectionOverdue:
		if len(v.undatedTasks) > 0 {
			v.currentSection = SectionUndated
		} else if len(v.todayTasks) > 0 {
			v.currentSection = SectionToday
		}
	case SectionUndated:
		if len(v.todayTasks) > 0 {
			v.currentSection = SectionToday
		} else if len(v.overdueTasks) > 0 {
			v.currentSection = SectionOverdue
		}
	case SectionToday:
		if len(v.overdueTasks) > 0 {
			v.currentSection = SectionOverdue
		} else if len(v.undatedTasks) > 0 {
			v.currentSection = SectionUndated
		}
	}
	v.cursor = 0
}

// prevSection moves to the previous section
func (v *PlanningView) prevSection() {
	switch v.currentSection {
	case SectionOverdue:
		if len(v.todayTasks) > 0 {
			v.currentSection = SectionToday
		} else if len(v.undatedTasks) > 0 {
			v.currentSection = SectionUndated
		}
	case SectionUndated:
		if len(v.overdueTasks) > 0 {
			v.currentSection = SectionOverdue
		} else if len(v.todayTasks) > 0 {
			v.currentSection = SectionToday
		}
	case SectionToday:
		if len(v.undatedTasks) > 0 {
			v.currentSection = SectionUndated
		} else if len(v.overdueTasks) > 0 {
			v.currentSection = SectionOverdue
		}
	}
	v.cursor = 0
}

// moveCursor moves the cursor within the current section
func (v *PlanningView) moveCursor(delta int) {
	tasks := v.currentTasks()
	if len(tasks) == 0 {
		return
	}

	v.cursor += delta
	if v.cursor < 0 {
		v.cursor = 0
	}
	if v.cursor >= len(tasks) {
		v.cursor = len(tasks) - 1
	}
}

// clampCursor ensures cursor is valid for current section
func (v *PlanningView) clampCursor() {
	tasks := v.currentTasks()
	if len(tasks) == 0 {
		v.cursor = 0
		// Move to a section that has tasks
		if len(v.overdueTasks) > 0 {
			v.currentSection = SectionOverdue
		} else if len(v.undatedTasks) > 0 {
			v.currentSection = SectionUndated
		} else if len(v.todayTasks) > 0 {
			v.currentSection = SectionToday
		}
	} else if v.cursor >= len(tasks) {
		v.cursor = len(tasks) - 1
	}
}

// getTargetIDs returns IDs of selected tasks or current task
func (v PlanningView) getTargetIDs() []string {
	var ids []string
	for id := range v.selected {
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		if task := v.currentTask(); task != nil {
			ids = append(ids, task.ID)
		}
	}
	return ids
}

// assignToToday assigns selected tasks to today
func (v PlanningView) assignToToday() tea.Cmd {
	ids := v.getTargetIDs()
	if len(ids) == 0 {
		return nil
	}

	today := time.Now()
	today = time.Date(today.Year(), today.Month(), today.Day(), 12, 0, 0, 0, time.Local)

	return func() tea.Msg {
		for _, id := range ids {
			_, err := v.db.Exec(`
				UPDATE tasks SET due_date = ?, updated_at = datetime('now')
				WHERE id = ?
			`, today.Format("2006-01-02 15:04:05"), id)
			if err != nil {
				return planningErrorMsg{err: err}
			}
		}
		return taskUpdatedMsg{}
	}
}

// assignToTomorrow assigns selected tasks to tomorrow
func (v PlanningView) assignToTomorrow() tea.Cmd {
	ids := v.getTargetIDs()
	if len(ids) == 0 {
		return nil
	}

	tomorrow := time.Now().AddDate(0, 0, 1)
	tomorrow = time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 12, 0, 0, 0, time.Local)

	return func() tea.Msg {
		for _, id := range ids {
			_, err := v.db.Exec(`
				UPDATE tasks SET due_date = ?, updated_at = datetime('now')
				WHERE id = ?
			`, tomorrow.Format("2006-01-02 15:04:05"), id)
			if err != nil {
				return planningErrorMsg{err: err}
			}
		}
		return taskUpdatedMsg{}
	}
}

// removeDueDate removes due date from selected tasks
func (v PlanningView) removeDueDate() tea.Cmd {
	ids := v.getTargetIDs()
	if len(ids) == 0 {
		return nil
	}

	return func() tea.Msg {
		for _, id := range ids {
			_, err := v.db.Exec(`
				UPDATE tasks SET due_date = NULL, updated_at = datetime('now')
				WHERE id = ?
			`, id)
			if err != nil {
				return planningErrorMsg{err: err}
			}
		}
		return taskUpdatedMsg{}
	}
}

// markDone marks selected tasks as done
func (v PlanningView) markDone() tea.Cmd {
	ids := v.getTargetIDs()
	if len(ids) == 0 {
		return nil
	}

	return func() tea.Msg {
		for _, id := range ids {
			_, err := v.db.Exec(`
				UPDATE tasks SET status = 'done', completed_at = datetime('now'), updated_at = datetime('now')
				WHERE id = ?
			`, id)
			if err != nil {
				return planningErrorMsg{err: err}
			}
		}
		return taskUpdatedMsg{}
	}
}

// View renders the planning view
func (v PlanningView) View() string {
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

	now := time.Now()
	dateStr := now.Format("Monday, January 2")
	sections = append(sections, titleStyle.Render(fmt.Sprintf("Daily Planning - %s", dateStr)))

	// Summary line
	summaryStyle := lipgloss.NewStyle().Foreground(t.Subtle)
	summary := fmt.Sprintf("%d overdue • %d undated • %d planned for today",
		len(v.overdueTasks), len(v.undatedTasks), len(v.todayTasks))
	sections = append(sections, summaryStyle.Render(summary))
	sections = append(sections, "")

	// Calculate section height
	availableHeight := v.height - 8 // Reserve space for title, summary, status, hints
	sectionHeight := availableHeight / 3

	// Render three columns side by side
	colWidth := (v.width - 6) / 3

	overdueCol := v.renderSection("Overdue", v.overdueTasks, SectionOverdue, colWidth, sectionHeight, t.Error)
	undatedCol := v.renderSection("Undated", v.undatedTasks, SectionUndated, colWidth, sectionHeight, t.Warning)
	todayCol := v.renderSection("Today", v.todayTasks, SectionToday, colWidth, sectionHeight, t.Success)

	columns := lipgloss.JoinHorizontal(lipgloss.Top, overdueCol, undatedCol, todayCol)
	sections = append(sections, columns)

	// Status message
	if v.statusMsg != "" {
		statusStyle := lipgloss.NewStyle().Foreground(t.Info).MarginTop(1)
		sections = append(sections, statusStyle.Render(v.statusMsg))
	}

	return strings.Join(sections, "\n")
}

// renderSection renders a section of tasks
func (v PlanningView) renderSection(title string, tasks []model.Task, section PlanningSection, width, height int, accentColor lipgloss.Color) string {
	t := theme.Current.Theme

	isActive := v.currentSection == section

	// Header style
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(accentColor).
		Width(width).
		Align(lipgloss.Center)

	// Box style
	borderColor := t.Border
	if isActive {
		borderColor = accentColor
	}

	boxStyle := lipgloss.NewStyle().
		Width(width).
		Height(height).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1)

	var lines []string

	// Header with count
	headerText := fmt.Sprintf("%s (%d)", title, len(tasks))
	lines = append(lines, headerStyle.Render(headerText))

	if len(tasks) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(t.Subtle).
			Italic(true)
		lines = append(lines, emptyStyle.Render("  (empty)"))
	} else {
		maxItems := height - 4
		for i, task := range tasks {
			if i >= maxItems {
				moreStyle := lipgloss.NewStyle().Foreground(t.Subtle)
				lines = append(lines, moreStyle.Render(fmt.Sprintf("  ... +%d more", len(tasks)-maxItems)))
				break
			}

			isSelected := v.selected[task.ID]
			isCursor := isActive && i == v.cursor

			itemStyle := lipgloss.NewStyle().Width(width - 4)
			if isCursor {
				itemStyle = itemStyle.Background(t.Highlight).Bold(true)
			}

			// Checkbox for selection
			checkbox := "[ ]"
			if isSelected {
				checkbox = "[x]"
			}

			// Priority indicator
			priorityChar := ""
			switch task.Priority {
			case model.PriorityUrgent:
				priorityChar = lipgloss.NewStyle().Foreground(t.PriorityUrgent).Render("!")
			case model.PriorityHigh:
				priorityChar = lipgloss.NewStyle().Foreground(t.PriorityHigh).Render("▲")
			case model.PriorityMedium:
				priorityChar = lipgloss.NewStyle().Foreground(t.PriorityMedium).Render("●")
			case model.PriorityLow:
				priorityChar = lipgloss.NewStyle().Foreground(t.PriorityLow).Render("▽")
			}

			// Truncate title
			titleText := task.Title
			maxLen := width - 12
			if len(titleText) > maxLen {
				titleText = titleText[:maxLen-3] + "..."
			}

			line := fmt.Sprintf("%s %s %s", checkbox, priorityChar, titleText)
			lines = append(lines, itemStyle.Render(line))
		}
	}

	content := strings.Join(lines, "\n")
	return boxStyle.Render(content)
}

// IsInputMode returns whether the view is in input mode
func (v PlanningView) IsInputMode() bool {
	return false
}
