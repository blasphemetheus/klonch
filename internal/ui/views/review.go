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

// Local message types for review view
type reviewErrorMsg struct{ err error }

// ReviewSection represents a section in the review view
type ReviewSection int

const (
	SectionCompleted ReviewSection = iota
	SectionOverdueReview
	SectionStale
)

// ReviewView represents the weekly review view
type ReviewView struct {
	db     *db.DB
	width  int
	height int

	// Tasks organized by section
	completedTasks []model.Task // Completed this week
	overdueTasks   []model.Task // Overdue and need attention
	staleTasks     []model.Task // Pending for >2 weeks

	// Week stats
	weekStart       time.Time
	weekEnd         time.Time
	totalCompleted  int
	totalTimeLogged int // minutes

	// Navigation
	currentSection ReviewSection
	cursor         int
	selected       map[string]bool

	// Status
	statusMsg string
}

// NewReviewView creates a new review view
func NewReviewView(database *db.DB) ReviewView {
	return ReviewView{
		db:       database,
		selected: make(map[string]bool),
	}
}

// Init initializes the review view
func (v ReviewView) Init() tea.Cmd {
	return v.loadTasks()
}

// SetSize sets the view dimensions
func (v ReviewView) SetSize(width, height int) ReviewView {
	v.width = width
	v.height = height
	return v
}

// loadTasks loads tasks for weekly review
func (v ReviewView) loadTasks() tea.Cmd {
	return func() tea.Msg {
		now := time.Now()

		// Calculate week boundaries (Monday to Sunday)
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7 // Sunday = 7
		}
		weekStart := time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, time.Local)
		weekEnd := weekStart.AddDate(0, 0, 7)

		// Two weeks ago for stale detection
		twoWeeksAgo := now.AddDate(0, 0, -14)

		// Load completed tasks this week
		completedRows, err := v.db.Query(`
			SELECT id, title, description, status, priority, completed_at, project_id
			FROM tasks
			WHERE parent_id IS NULL
			  AND status = 'done'
			  AND completed_at >= ?
			  AND completed_at < ?
			ORDER BY completed_at DESC
		`, weekStart.Format("2006-01-02"), weekEnd.Format("2006-01-02"))
		if err != nil {
			return reviewErrorMsg{err: err}
		}
		defer completedRows.Close()

		var completedTasks []model.Task
		for completedRows.Next() {
			t := scanReviewTask(completedRows)
			if t != nil {
				completedTasks = append(completedTasks, *t)
			}
		}

		// Load overdue tasks
		overdueRows, err := v.db.Query(`
			SELECT id, title, description, status, priority, due_date, project_id
			FROM tasks
			WHERE parent_id IS NULL
			  AND status IN ('pending', 'in_progress')
			  AND due_date IS NOT NULL
			  AND due_date < ?
			ORDER BY due_date
		`, now.Format("2006-01-02"))
		if err != nil {
			return reviewErrorMsg{err: err}
		}
		defer overdueRows.Close()

		var overdueTasks []model.Task
		for overdueRows.Next() {
			t := scanReviewTask(overdueRows)
			if t != nil {
				overdueTasks = append(overdueTasks, *t)
			}
		}

		// Load stale tasks (pending for >2 weeks, no recent updates)
		staleRows, err := v.db.Query(`
			SELECT id, title, description, status, priority, created_at, project_id
			FROM tasks
			WHERE parent_id IS NULL
			  AND status IN ('pending', 'in_progress')
			  AND created_at < ?
			  AND updated_at < ?
			ORDER BY created_at
			LIMIT 20
		`, twoWeeksAgo.Format("2006-01-02"), twoWeeksAgo.Format("2006-01-02"))
		if err != nil {
			return reviewErrorMsg{err: err}
		}
		defer staleRows.Close()

		var staleTasks []model.Task
		for staleRows.Next() {
			t := scanReviewTask(staleRows)
			if t != nil {
				staleTasks = append(staleTasks, *t)
			}
		}

		// Get total time logged this week
		var totalTime int
		row := v.db.QueryRow(`
			SELECT COALESCE(SUM(duration), 0) FROM time_entries
			WHERE started_at >= ? AND started_at < ?
		`, weekStart.Format("2006-01-02"), weekEnd.Format("2006-01-02"))
		row.Scan(&totalTime)

		return reviewLoadedMsg{
			completed:       completedTasks,
			overdue:         overdueTasks,
			stale:           staleTasks,
			weekStart:       weekStart,
			weekEnd:         weekEnd,
			totalCompleted:  len(completedTasks),
			totalTimeLogged: totalTime,
		}
	}
}

// scanReviewTask scans a task row for review view
func scanReviewTask(rows interface{ Scan(...interface{}) error }) *model.Task {
	var t model.Task
	var desc, dateCol, projectID *string
	if err := rows.Scan(&t.ID, &t.Title, &desc, &t.Status, &t.Priority, &dateCol, &projectID); err != nil {
		return nil
	}
	if desc != nil {
		t.Description = *desc
	}
	if dateCol != nil {
		if parsed, err := time.Parse("2006-01-02 15:04:05", *dateCol); err == nil {
			if t.Status == model.StatusDone {
				t.CompletedAt = &parsed
			} else {
				t.DueDate = &parsed
			}
		} else if parsed, err := time.Parse("2006-01-02", *dateCol); err == nil {
			if t.Status == model.StatusDone {
				t.CompletedAt = &parsed
			} else {
				t.DueDate = &parsed
			}
		}
	}
	t.ProjectID = projectID
	return &t
}

type reviewLoadedMsg struct {
	completed       []model.Task
	overdue         []model.Task
	stale           []model.Task
	weekStart       time.Time
	weekEnd         time.Time
	totalCompleted  int
	totalTimeLogged int
}

// Update handles messages
func (v ReviewView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case reviewLoadedMsg:
		v.completedTasks = msg.completed
		v.overdueTasks = msg.overdue
		v.staleTasks = msg.stale
		v.weekStart = msg.weekStart
		v.weekEnd = msg.weekEnd
		v.totalCompleted = msg.totalCompleted
		v.totalTimeLogged = msg.totalTimeLogged
		v.clampCursor()
		return v, nil

	case reviewErrorMsg:
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
		case "n": // Reschedule to next week
			return v, v.rescheduleNextWeek()

		case "t": // Reschedule to today
			return v, v.rescheduleToday()

		case "a": // Archive
			return v, v.archiveTasks()

		case "u": // Uncheck (mark as pending)
			return v, v.uncheckTasks()

		case "x": // Remove due date
			return v, v.removeDueDate()

		case "d": // Delete
			return v, v.deleteTasks()

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
func (v ReviewView) currentTasks() []model.Task {
	switch v.currentSection {
	case SectionCompleted:
		return v.completedTasks
	case SectionOverdueReview:
		return v.overdueTasks
	case SectionStale:
		return v.staleTasks
	}
	return nil
}

// currentTask returns the currently highlighted task
func (v ReviewView) currentTask() *model.Task {
	tasks := v.currentTasks()
	if len(tasks) == 0 || v.cursor >= len(tasks) {
		return nil
	}
	return &tasks[v.cursor]
}

// nextSection moves to the next section
func (v *ReviewView) nextSection() {
	sections := []ReviewSection{SectionCompleted, SectionOverdueReview, SectionStale}
	counts := []int{len(v.completedTasks), len(v.overdueTasks), len(v.staleTasks)}

	currentIdx := int(v.currentSection)
	for i := 1; i <= 3; i++ {
		nextIdx := (currentIdx + i) % 3
		if counts[nextIdx] > 0 {
			v.currentSection = sections[nextIdx]
			v.cursor = 0
			return
		}
	}
}

// prevSection moves to the previous section
func (v *ReviewView) prevSection() {
	sections := []ReviewSection{SectionCompleted, SectionOverdueReview, SectionStale}
	counts := []int{len(v.completedTasks), len(v.overdueTasks), len(v.staleTasks)}

	currentIdx := int(v.currentSection)
	for i := 1; i <= 3; i++ {
		prevIdx := (currentIdx - i + 3) % 3
		if counts[prevIdx] > 0 {
			v.currentSection = sections[prevIdx]
			v.cursor = 0
			return
		}
	}
}

// moveCursor moves the cursor within the current section
func (v *ReviewView) moveCursor(delta int) {
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
func (v *ReviewView) clampCursor() {
	tasks := v.currentTasks()
	if len(tasks) == 0 {
		v.cursor = 0
		// Move to a section that has tasks
		if len(v.completedTasks) > 0 {
			v.currentSection = SectionCompleted
		} else if len(v.overdueTasks) > 0 {
			v.currentSection = SectionOverdueReview
		} else if len(v.staleTasks) > 0 {
			v.currentSection = SectionStale
		}
	} else if v.cursor >= len(tasks) {
		v.cursor = len(tasks) - 1
	}
}

// getTargetIDs returns IDs of selected tasks or current task
func (v ReviewView) getTargetIDs() []string {
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

// rescheduleNextWeek reschedules tasks to next Monday
func (v ReviewView) rescheduleNextWeek() tea.Cmd {
	ids := v.getTargetIDs()
	if len(ids) == 0 {
		return nil
	}

	// Calculate next Monday
	now := time.Now()
	daysUntilMonday := (8 - int(now.Weekday())) % 7
	if daysUntilMonday == 0 {
		daysUntilMonday = 7
	}
	nextMonday := time.Date(now.Year(), now.Month(), now.Day()+daysUntilMonday, 12, 0, 0, 0, time.Local)

	return func() tea.Msg {
		for _, id := range ids {
			_, err := v.db.Exec(`
				UPDATE tasks SET due_date = ?, updated_at = datetime('now')
				WHERE id = ?
			`, nextMonday.Format("2006-01-02 15:04:05"), id)
			if err != nil {
				return reviewErrorMsg{err: err}
			}
		}
		return taskUpdatedMsg{}
	}
}

// rescheduleToday reschedules tasks to today
func (v ReviewView) rescheduleToday() tea.Cmd {
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
				return reviewErrorMsg{err: err}
			}
		}
		return taskUpdatedMsg{}
	}
}

// archiveTasks archives the selected tasks
func (v ReviewView) archiveTasks() tea.Cmd {
	ids := v.getTargetIDs()
	if len(ids) == 0 {
		return nil
	}

	return func() tea.Msg {
		for _, id := range ids {
			_, err := v.db.Exec(`
				UPDATE tasks SET status = 'archived', updated_at = datetime('now')
				WHERE id = ?
			`, id)
			if err != nil {
				return reviewErrorMsg{err: err}
			}
		}
		return taskUpdatedMsg{}
	}
}

// uncheckTasks marks completed tasks as pending again
func (v ReviewView) uncheckTasks() tea.Cmd {
	ids := v.getTargetIDs()
	if len(ids) == 0 {
		return nil
	}

	return func() tea.Msg {
		for _, id := range ids {
			_, err := v.db.Exec(`
				UPDATE tasks SET status = 'pending', completed_at = NULL, updated_at = datetime('now')
				WHERE id = ?
			`, id)
			if err != nil {
				return reviewErrorMsg{err: err}
			}
		}
		return taskUpdatedMsg{}
	}
}

// removeDueDate removes due date from tasks
func (v ReviewView) removeDueDate() tea.Cmd {
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
				return reviewErrorMsg{err: err}
			}
		}
		return taskUpdatedMsg{}
	}
}

// deleteTasks deletes the selected tasks
func (v ReviewView) deleteTasks() tea.Cmd {
	ids := v.getTargetIDs()
	if len(ids) == 0 {
		return nil
	}

	return func() tea.Msg {
		for _, id := range ids {
			_, err := v.db.Exec(`DELETE FROM tasks WHERE id = ?`, id)
			if err != nil {
				return reviewErrorMsg{err: err}
			}
		}
		return taskUpdatedMsg{}
	}
}

// View renders the review view
func (v ReviewView) View() string {
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

	sections = append(sections, titleStyle.Render("Weekly Review"))

	// Week summary
	summaryStyle := lipgloss.NewStyle().Foreground(t.Subtle)
	weekRange := fmt.Sprintf("%s - %s",
		v.weekStart.Format("Jan 2"),
		v.weekEnd.Add(-24*time.Hour).Format("Jan 2"))

	hours := v.totalTimeLogged / 60
	mins := v.totalTimeLogged % 60
	timeStr := fmt.Sprintf("%dh %dm", hours, mins)

	summary := fmt.Sprintf("Week of %s • %d tasks completed • %s tracked",
		weekRange, v.totalCompleted, timeStr)
	sections = append(sections, summaryStyle.Render(summary))

	// Action needed summary
	actionStyle := lipgloss.NewStyle().Foreground(t.Warning)
	if len(v.overdueTasks) > 0 || len(v.staleTasks) > 0 {
		actionSummary := fmt.Sprintf("%d overdue • %d stale tasks need attention",
			len(v.overdueTasks), len(v.staleTasks))
		sections = append(sections, actionStyle.Render(actionSummary))
	}
	sections = append(sections, "")

	// Calculate section height
	availableHeight := v.height - 10
	sectionHeight := availableHeight / 3

	// Render three columns
	colWidth := (v.width - 6) / 3

	completedCol := v.renderSection("Completed", v.completedTasks, SectionCompleted, colWidth, sectionHeight, t.Success)
	overdueCol := v.renderSection("Overdue", v.overdueTasks, SectionOverdueReview, colWidth, sectionHeight, t.Error)
	staleCol := v.renderSection("Stale (>2 weeks)", v.staleTasks, SectionStale, colWidth, sectionHeight, t.Warning)

	columns := lipgloss.JoinHorizontal(lipgloss.Top, completedCol, overdueCol, staleCol)
	sections = append(sections, columns)

	// Status message
	if v.statusMsg != "" {
		statusStyle := lipgloss.NewStyle().Foreground(t.Info).MarginTop(1)
		sections = append(sections, statusStyle.Render(v.statusMsg))
	}

	return strings.Join(sections, "\n")
}

// renderSection renders a section of tasks
func (v ReviewView) renderSection(title string, tasks []model.Task, section ReviewSection, width, height int, accentColor lipgloss.Color) string {
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
		if section == SectionCompleted {
			lines = append(lines, emptyStyle.Render("  No completions yet"))
		} else {
			lines = append(lines, emptyStyle.Render("  All clear!"))
		}
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

			// Checkbox/status indicator
			indicator := "[ ]"
			if isSelected {
				indicator = "[x]"
			} else if section == SectionCompleted {
				indicator = "[✓]"
			}

			// Date info
			var dateInfo string
			if section == SectionCompleted && task.CompletedAt != nil {
				dateInfo = task.CompletedAt.Format("Mon")
			} else if section == SectionOverdueReview && task.DueDate != nil {
				daysOverdue := int(time.Since(*task.DueDate).Hours() / 24)
				if daysOverdue == 1 {
					dateInfo = "1d ago"
				} else {
					dateInfo = fmt.Sprintf("%dd ago", daysOverdue)
				}
			} else if section == SectionStale && !task.CreatedAt.IsZero() {
				weeksOld := int(time.Since(task.CreatedAt).Hours() / 24 / 7)
				dateInfo = fmt.Sprintf("%dw old", weeksOld)
			}

			// Truncate title
			titleText := task.Title
			maxLen := width - 18
			if len(titleText) > maxLen {
				titleText = titleText[:maxLen-3] + "..."
			}

			line := fmt.Sprintf("%s %s", indicator, titleText)
			if dateInfo != "" {
				// Right-align date info
				padding := width - 6 - len(line) - len(dateInfo)
				if padding > 0 {
					line += strings.Repeat(" ", padding) + lipgloss.NewStyle().Foreground(t.Subtle).Render(dateInfo)
				}
			}

			lines = append(lines, itemStyle.Render(line))
		}
	}

	content := strings.Join(lines, "\n")
	return boxStyle.Render(content)
}

// IsInputMode returns whether the view is in input mode
func (v ReviewView) IsInputMode() bool {
	return false
}
