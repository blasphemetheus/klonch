package views

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dori/klonch/internal/db"
	"github.com/dori/klonch/internal/model"
	"github.com/dori/klonch/internal/ui/theme"
)

// Local message types for kanban view
type kanbanErrorMsg struct{ err error }

// KanbanColumn represents a column in the kanban board
type KanbanColumn int

const (
	ColumnBacklog KanbanColumn = iota
	ColumnTodo
	ColumnInProgress
	ColumnDone
)

// KanbanView represents the kanban board view
type KanbanView struct {
	db     *db.DB
	width  int
	height int

	// Tasks organized by column
	columns [4][]model.Task

	// Navigation state
	currentColumn KanbanColumn
	cursorRow     int

	// Per-column scroll offset
	columnScroll [4]int

	// Selected tasks
	selected map[string]bool

	// Status message
	statusMsg string
}

// NewKanbanView creates a new kanban view
func NewKanbanView(database *db.DB) KanbanView {
	return KanbanView{
		db:       database,
		selected: make(map[string]bool),
	}
}

// Init initializes the kanban view
func (v KanbanView) Init() tea.Cmd {
	return v.loadTasks()
}

// SetSize sets the view dimensions
func (v KanbanView) SetSize(width, height int) KanbanView {
	v.width = width
	v.height = height
	return v
}

// loadTasks loads tasks from database and organizes by status
func (v KanbanView) loadTasks() tea.Cmd {
	return func() tea.Msg {
		rows, err := v.db.Query(`
			SELECT id, title, description, status, priority, project_id, due_date
			FROM tasks
			WHERE parent_id IS NULL AND status != 'archived'
			ORDER BY position, created_at
		`)
		if err != nil {
			return kanbanErrorMsg{err: err}
		}
		defer rows.Close()

		columns := [4][]model.Task{}
		for rows.Next() {
			var t model.Task
			var desc, projectID *string
			var dueDate *string
			if err := rows.Scan(&t.ID, &t.Title, &desc, &t.Status, &t.Priority, &projectID, &dueDate); err != nil {
				continue
			}
			if desc != nil {
				t.Description = *desc
			}
			t.ProjectID = projectID

			// Assign to appropriate column based on status
			switch t.Status {
			case model.StatusPending:
				columns[ColumnTodo] = append(columns[ColumnTodo], t)
			case model.StatusInProgress:
				columns[ColumnInProgress] = append(columns[ColumnInProgress], t)
			case model.StatusDone:
				columns[ColumnDone] = append(columns[ColumnDone], t)
			default:
				// Treat unknown/other as backlog
				columns[ColumnBacklog] = append(columns[ColumnBacklog], t)
			}
		}

		return kanbanLoadedMsg{columns: columns}
	}
}

type kanbanLoadedMsg struct {
	columns [4][]model.Task
}

// Update handles messages
func (v KanbanView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case kanbanLoadedMsg:
		v.columns = msg.columns
		return v, nil

	case taskUpdatedMsg:
		return v, v.loadTasks()

	case tea.KeyMsg:
		switch msg.String() {
		// Column navigation
		case "h", "left":
			if v.currentColumn > 0 {
				v.currentColumn--
				v.clampCursor()
			}
			return v, nil

		case "l", "right":
			if v.currentColumn < 3 {
				v.currentColumn++
				v.clampCursor()
			}
			return v, nil

		// Row navigation
		case "j", "down":
			col := v.columns[v.currentColumn]
			if v.cursorRow < len(col)-1 {
				v.cursorRow++
				v.ensureCursorVisible()
			}
			return v, nil

		case "k", "up":
			if v.cursorRow > 0 {
				v.cursorRow--
				v.ensureCursorVisible()
			}
			return v, nil

		// Move task between columns
		case "H": // Move left
			return v, v.moveTask(-1)

		case "L": // Move right
			return v, v.moveTask(1)

		case "enter", " ":
			// Toggle done
			return v, v.toggleCurrentTask()

		case "g":
			v.cursorRow = 0
			v.columnScroll[v.currentColumn] = 0
			return v, nil

		case "G":
			col := v.columns[v.currentColumn]
			if len(col) > 0 {
				v.cursorRow = len(col) - 1
				v.ensureCursorVisible()
			}
			return v, nil
		}
	}

	return v, nil
}

// clampCursor ensures cursor is valid for current column
func (v *KanbanView) clampCursor() {
	col := v.columns[v.currentColumn]
	if v.cursorRow >= len(col) {
		if len(col) > 0 {
			v.cursorRow = len(col) - 1
		} else {
			v.cursorRow = 0
		}
	}
	v.ensureCursorVisible()
}

// ensureCursorVisible adjusts scroll to keep cursor in view
func (v *KanbanView) ensureCursorVisible() {
	visibleItems := v.visibleItemCount()
	if visibleItems <= 0 {
		visibleItems = 5
	}

	col := int(v.currentColumn)

	// Scroll down if cursor is below visible area
	if v.cursorRow >= v.columnScroll[col]+visibleItems {
		v.columnScroll[col] = v.cursorRow - visibleItems + 1
	}

	// Scroll up if cursor is above visible area
	if v.cursorRow < v.columnScroll[col] {
		v.columnScroll[col] = v.cursorRow
	}
}

// visibleItemCount returns how many items fit in the column height
func (v *KanbanView) visibleItemCount() int {
	// Each item takes ~2 lines (content + margin), column has height - 5 for borders/padding
	availableHeight := v.height - 7
	if availableHeight < 2 {
		return 1
	}
	return availableHeight / 2
}

// moveTask moves the current task to an adjacent column
func (v KanbanView) moveTask(direction int) tea.Cmd {
	col := v.columns[v.currentColumn]
	if len(col) == 0 || v.cursorRow >= len(col) {
		return nil
	}

	newColumn := int(v.currentColumn) + direction
	if newColumn < 0 || newColumn > 3 {
		return nil
	}

	task := col[v.cursorRow]
	var newStatus model.Status

	switch KanbanColumn(newColumn) {
	case ColumnBacklog:
		newStatus = model.StatusPending // Backlog is pending but visually separated
	case ColumnTodo:
		newStatus = model.StatusPending
	case ColumnInProgress:
		newStatus = model.StatusInProgress
	case ColumnDone:
		newStatus = model.StatusDone
	}

	return func() tea.Msg {
		_, err := v.db.Exec(`
			UPDATE tasks SET status = ?, updated_at = datetime('now')
			WHERE id = ?
		`, newStatus, task.ID)
		if err != nil {
			return kanbanErrorMsg{err: err}
		}

		// If marked done, set completed_at
		if newStatus == model.StatusDone {
			v.db.Exec(`UPDATE tasks SET completed_at = datetime('now') WHERE id = ?`, task.ID)
		} else {
			v.db.Exec(`UPDATE tasks SET completed_at = NULL WHERE id = ?`, task.ID)
		}

		return taskUpdatedMsg{}
	}
}

// toggleCurrentTask toggles the done status of the current task
func (v KanbanView) toggleCurrentTask() tea.Cmd {
	col := v.columns[v.currentColumn]
	if len(col) == 0 || v.cursorRow >= len(col) {
		return nil
	}

	task := col[v.cursorRow]
	var newStatus model.Status

	if task.Status == model.StatusDone {
		newStatus = model.StatusPending
	} else {
		newStatus = model.StatusDone
	}

	return func() tea.Msg {
		_, err := v.db.Exec(`
			UPDATE tasks SET status = ?, updated_at = datetime('now')
			WHERE id = ?
		`, newStatus, task.ID)
		if err != nil {
			return kanbanErrorMsg{err: err}
		}

		if newStatus == model.StatusDone {
			v.db.Exec(`UPDATE tasks SET completed_at = datetime('now') WHERE id = ?`, task.ID)
		} else {
			v.db.Exec(`UPDATE tasks SET completed_at = NULL WHERE id = ?`, task.ID)
		}

		return taskUpdatedMsg{}
	}
}

// View renders the kanban board
func (v KanbanView) View() string {
	if v.width == 0 || v.height == 0 {
		return "Loading..."
	}

	t := theme.Current.Theme

	// Column headers
	columnNames := []string{"Backlog", "Todo", "In Progress", "Done"}
	columnColors := []lipgloss.Color{t.Subtle, t.Info, t.Warning, t.Success}

	// Calculate column width
	colWidth := (v.width - 4) / 4 // 4 columns with some margin
	if colWidth < 20 {
		colWidth = 20
	}

	// Style for column headers
	headerStyle := func(i int, active bool) lipgloss.Style {
		s := lipgloss.NewStyle().
			Bold(true).
			Foreground(columnColors[i]).
			Width(colWidth).
			Align(lipgloss.Center)
		if active {
			s = s.Background(t.Highlight)
		}
		return s
	}

	// Style for column content
	columnStyle := lipgloss.NewStyle().
		Width(colWidth).
		Height(v.height - 3). // Leave room for header and margins
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(t.Border)

	// Render headers
	var headers []string
	for i, name := range columnNames {
		count := len(v.columns[i])
		header := fmt.Sprintf("%s (%d)", name, count)
		headers = append(headers, headerStyle(i, i == int(v.currentColumn)).Render(header))
	}
	headerRow := lipgloss.JoinHorizontal(lipgloss.Top, headers...)

	// Render columns
	visibleItems := v.visibleItemCount()
	var cols []string
	for i, tasks := range v.columns {
		isActiveCol := i == int(v.currentColumn)
		scrollOffset := v.columnScroll[i]

		// Calculate visible range
		startIdx := scrollOffset
		endIdx := scrollOffset + visibleItems
		if startIdx > len(tasks) {
			startIdx = len(tasks)
		}
		if endIdx > len(tasks) {
			endIdx = len(tasks)
		}

		var items []string

		// Show scroll indicator at top if scrolled
		if scrollOffset > 0 {
			scrollIndicator := lipgloss.NewStyle().
				Foreground(t.Subtle).
				Width(colWidth - 4).
				Align(lipgloss.Center).
				Render(fmt.Sprintf("↑ %d more", scrollOffset))
			items = append(items, scrollIndicator)
		}

		for j := startIdx; j < endIdx; j++ {
			task := tasks[j]
			isSelected := isActiveCol && j == v.cursorRow

			// Task card style
			cardStyle := lipgloss.NewStyle().
				Width(colWidth - 4).
				Padding(0, 1)

			if isSelected {
				cardStyle = cardStyle.
					Background(t.Highlight).
					Foreground(t.Foreground)
			} else {
				cardStyle = cardStyle.
					Foreground(t.Foreground)
			}

			// Priority indicator
			priorityChar := ""
			priorityStyle := lipgloss.NewStyle()
			switch task.Priority {
			case model.PriorityUrgent:
				priorityChar = priorityStyle.Foreground(t.PriorityUrgent).Render("!")
			case model.PriorityHigh:
				priorityChar = priorityStyle.Foreground(t.PriorityHigh).Render("▲")
			case model.PriorityMedium:
				priorityChar = priorityStyle.Foreground(t.PriorityMedium).Render("●")
			case model.PriorityLow:
				priorityChar = priorityStyle.Foreground(t.PriorityLow).Render("▽")
			}

			// Truncate title to fit
			title := task.Title
			maxTitleLen := colWidth - 8
			if len(title) > maxTitleLen {
				title = title[:maxTitleLen-3] + "..."
			}

			cardContent := fmt.Sprintf("%s %s", priorityChar, title)
			items = append(items, cardStyle.Render(cardContent))
		}

		// Show scroll indicator at bottom if more items below
		if endIdx < len(tasks) {
			scrollIndicator := lipgloss.NewStyle().
				Foreground(t.Subtle).
				Width(colWidth - 4).
				Align(lipgloss.Center).
				Render(fmt.Sprintf("↓ %d more", len(tasks)-endIdx))
			items = append(items, scrollIndicator)
		}

		content := strings.Join(items, "\n")
		if len(tasks) == 0 {
			content = lipgloss.NewStyle().
				Foreground(t.Subtle).
				Italic(true).
				Render("(empty)")
		}

		// Apply active column styling
		cs := columnStyle
		if isActiveCol {
			cs = cs.BorderForeground(t.Primary)
		}

		cols = append(cols, cs.Render(content))
	}
	columnsRow := lipgloss.JoinHorizontal(lipgloss.Top, cols...)

	// Footer with hints
	hints := lipgloss.NewStyle().Foreground(t.Subtle).Render(
		"h/l: switch column • j/k: navigate • H/L: move task • enter: toggle done",
	)

	return lipgloss.JoinVertical(lipgloss.Left, headerRow, columnsRow, hints)
}

// IsInputMode returns whether the view is in input mode
func (v KanbanView) IsInputMode() bool {
	return false
}
