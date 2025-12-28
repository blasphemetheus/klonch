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

// Local message types for eisenhower view
type eisenhowerErrorMsg struct{ err error }

// Quadrant represents an Eisenhower matrix quadrant
type Quadrant int

const (
	QuadrantDoFirst  Quadrant = iota // Urgent + Important
	QuadrantDelegate                 // Urgent + Not Important
	QuadrantSchedule                 // Not Urgent + Important
	QuadrantEliminate                // Not Urgent + Not Important
)

// EisenhowerView represents the Eisenhower matrix view
type EisenhowerView struct {
	db     *db.DB
	width  int
	height int

	// Tasks organized by quadrant
	quadrants [4][]model.Task

	// Navigation state
	currentQuadrant Quadrant
	cursorRow       int

	// Selected tasks
	selected map[string]bool

	// Status message
	statusMsg string
}

// NewEisenhowerView creates a new Eisenhower view
func NewEisenhowerView(database *db.DB) EisenhowerView {
	return EisenhowerView{
		db:       database,
		selected: make(map[string]bool),
	}
}

// Init initializes the Eisenhower view
func (v EisenhowerView) Init() tea.Cmd {
	return v.loadTasks()
}

// SetSize sets the view dimensions
func (v EisenhowerView) SetSize(width, height int) EisenhowerView {
	v.width = width
	v.height = height
	return v
}

// loadTasks loads tasks from database and organizes by quadrant
func (v EisenhowerView) loadTasks() tea.Cmd {
	return func() tea.Msg {
		rows, err := v.db.Query(`
			SELECT id, title, description, status, priority, urgency, importance, project_id
			FROM tasks
			WHERE parent_id IS NULL AND status != 'archived' AND status != 'done'
			ORDER BY priority DESC, created_at
		`)
		if err != nil {
			return eisenhowerErrorMsg{err: err}
		}
		defer rows.Close()

		quadrants := [4][]model.Task{}
		for rows.Next() {
			var t model.Task
			var desc, projectID *string
			if err := rows.Scan(&t.ID, &t.Title, &desc, &t.Status, &t.Priority, &t.Urgency, &t.Importance, &projectID); err != nil {
				continue
			}
			if desc != nil {
				t.Description = *desc
			}
			t.ProjectID = projectID

			// Assign to appropriate quadrant based on urgency/importance
			q := t.EisenhowerQuadrant() - 1 // Convert 1-4 to 0-3
			if q >= 0 && q < 4 {
				quadrants[q] = append(quadrants[q], t)
			}
		}

		return eisenhowerLoadedMsg{quadrants: quadrants}
	}
}

type eisenhowerLoadedMsg struct {
	quadrants [4][]model.Task
}

// Update handles messages
func (v EisenhowerView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case eisenhowerLoadedMsg:
		v.quadrants = msg.quadrants
		return v, nil

	case taskUpdatedMsg:
		return v, v.loadTasks()

	case tea.KeyMsg:
		switch msg.String() {
		// Quadrant navigation (2x2 grid)
		case "h", "left":
			if v.currentQuadrant == QuadrantDelegate {
				v.currentQuadrant = QuadrantDoFirst
				v.clampCursor()
			} else if v.currentQuadrant == QuadrantEliminate {
				v.currentQuadrant = QuadrantSchedule
				v.clampCursor()
			}
			return v, nil

		case "l", "right":
			if v.currentQuadrant == QuadrantDoFirst {
				v.currentQuadrant = QuadrantDelegate
				v.clampCursor()
			} else if v.currentQuadrant == QuadrantSchedule {
				v.currentQuadrant = QuadrantEliminate
				v.clampCursor()
			}
			return v, nil

		case "k", "up":
			if v.cursorRow > 0 {
				v.cursorRow--
			} else if v.currentQuadrant == QuadrantSchedule {
				v.currentQuadrant = QuadrantDoFirst
				v.clampCursor()
			} else if v.currentQuadrant == QuadrantEliminate {
				v.currentQuadrant = QuadrantDelegate
				v.clampCursor()
			}
			return v, nil

		case "j", "down":
			quad := v.quadrants[v.currentQuadrant]
			if v.cursorRow < len(quad)-1 {
				v.cursorRow++
			} else if v.currentQuadrant == QuadrantDoFirst {
				v.currentQuadrant = QuadrantSchedule
				v.cursorRow = 0
			} else if v.currentQuadrant == QuadrantDelegate {
				v.currentQuadrant = QuadrantEliminate
				v.cursorRow = 0
			}
			return v, nil

		// Set quadrant for current task
		case "1":
			return v, v.setQuadrant(QuadrantDoFirst)
		case "2":
			return v, v.setQuadrant(QuadrantDelegate)
		case "3":
			return v, v.setQuadrant(QuadrantSchedule)
		case "4":
			return v, v.setQuadrant(QuadrantEliminate)

		// Toggle done
		case "enter", " ", "tab":
			return v, v.toggleCurrentTask()

		case "g":
			v.cursorRow = 0
			return v, nil

		case "G":
			quad := v.quadrants[v.currentQuadrant]
			if len(quad) > 0 {
				v.cursorRow = len(quad) - 1
			}
			return v, nil
		}
	}

	return v, nil
}

// clampCursor ensures cursor is valid for current quadrant
func (v *EisenhowerView) clampCursor() {
	quad := v.quadrants[v.currentQuadrant]
	if v.cursorRow >= len(quad) {
		if len(quad) > 0 {
			v.cursorRow = len(quad) - 1
		} else {
			v.cursorRow = 0
		}
	}
}

// setQuadrant sets the current task's quadrant
func (v EisenhowerView) setQuadrant(target Quadrant) tea.Cmd {
	quad := v.quadrants[v.currentQuadrant]
	if len(quad) == 0 || v.cursorRow >= len(quad) {
		return nil
	}

	task := quad[v.cursorRow]

	var urgency, importance bool
	switch target {
	case QuadrantDoFirst:
		urgency, importance = true, true
	case QuadrantDelegate:
		urgency, importance = true, false
	case QuadrantSchedule:
		urgency, importance = false, true
	case QuadrantEliminate:
		urgency, importance = false, false
	}

	return func() tea.Msg {
		_, err := v.db.Exec(`
			UPDATE tasks SET urgency = ?, importance = ?, updated_at = datetime('now')
			WHERE id = ?
		`, urgency, importance, task.ID)
		if err != nil {
			return eisenhowerErrorMsg{err: err}
		}
		return taskUpdatedMsg{}
	}
}

// toggleCurrentTask toggles the done status of the current task
func (v EisenhowerView) toggleCurrentTask() tea.Cmd {
	quad := v.quadrants[v.currentQuadrant]
	if len(quad) == 0 || v.cursorRow >= len(quad) {
		return nil
	}

	task := quad[v.cursorRow]

	return func() tea.Msg {
		_, err := v.db.Exec(`
			UPDATE tasks SET status = 'done', completed_at = datetime('now'), updated_at = datetime('now')
			WHERE id = ?
		`, task.ID)
		if err != nil {
			return eisenhowerErrorMsg{err: err}
		}
		return taskUpdatedMsg{}
	}
}

// View renders the Eisenhower matrix
func (v EisenhowerView) View() string {
	if v.width == 0 || v.height == 0 {
		return "Loading..."
	}

	t := theme.Current.Theme

	// Quadrant labels and colors
	labels := []string{
		"DO FIRST",     // Urgent + Important
		"DELEGATE",     // Urgent + Not Important
		"SCHEDULE",     // Not Urgent + Important
		"ELIMINATE",    // Not Urgent + Not Important
	}
	colors := []lipgloss.Color{
		t.QuadrantDoFirst,
		t.QuadrantDelegate,
		t.QuadrantSchedule,
		t.QuadrantEliminate,
	}

	// Calculate quadrant dimensions (2x2 grid)
	quadWidth := (v.width - 3) / 2  // -3 for borders
	quadHeight := (v.height - 4) / 2 // -4 for borders and hints

	// Render each quadrant
	var quads [4]string
	for i := 0; i < 4; i++ {
		isActive := int(v.currentQuadrant) == i
		quads[i] = v.renderQuadrant(i, labels[i], colors[i], quadWidth, quadHeight, isActive)
	}

	// Build the 2x2 layout
	topRow := lipgloss.JoinHorizontal(lipgloss.Top, quads[0], quads[1])
	bottomRow := lipgloss.JoinHorizontal(lipgloss.Top, quads[2], quads[3])
	matrix := lipgloss.JoinVertical(lipgloss.Left, topRow, bottomRow)

	// Footer with hints
	hints := lipgloss.NewStyle().Foreground(t.Subtle).Render(
		"h/j/k/l: navigate • 1-4: set quadrant • enter: complete task",
	)

	return lipgloss.JoinVertical(lipgloss.Left, matrix, hints)
}

// renderQuadrant renders a single quadrant
func (v EisenhowerView) renderQuadrant(index int, label string, color lipgloss.Color, width, height int, active bool) string {
	t := theme.Current.Theme

	// Header style
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(color).
		Width(width).
		Align(lipgloss.Center)

	// Box style
	boxStyle := lipgloss.NewStyle().
		Width(width).
		Height(height).
		BorderStyle(lipgloss.RoundedBorder())

	if active {
		boxStyle = boxStyle.BorderForeground(color)
	} else {
		boxStyle = boxStyle.BorderForeground(t.Border)
	}

	// Build content
	tasks := v.quadrants[index]
	var items []string

	// Header
	items = append(items, headerStyle.Render(fmt.Sprintf("%s (%d)", label, len(tasks))))

	// Task list
	maxItems := height - 3 // Leave room for header and padding
	for j, task := range tasks {
		if j >= maxItems {
			items = append(items, lipgloss.NewStyle().Foreground(t.Subtle).Render(
				fmt.Sprintf("  ... +%d more", len(tasks)-maxItems),
			))
			break
		}

		isSelected := active && j == v.cursorRow

		itemStyle := lipgloss.NewStyle().Width(width - 4)
		if isSelected {
			itemStyle = itemStyle.
				Background(t.Highlight).
				Foreground(t.Foreground)
		} else {
			itemStyle = itemStyle.Foreground(t.Foreground)
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

		// Truncate title
		title := task.Title
		maxLen := width - 8
		if len(title) > maxLen {
			title = title[:maxLen-3] + "..."
		}

		items = append(items, itemStyle.Render(fmt.Sprintf(" %s %s", priorityChar, title)))
	}

	if len(tasks) == 0 {
		items = append(items, lipgloss.NewStyle().
			Foreground(t.Subtle).
			Italic(true).
			Render("  (empty)"))
	}

	content := strings.Join(items, "\n")
	return boxStyle.Render(content)
}

// IsInputMode returns whether the view is in input mode
func (v EisenhowerView) IsInputMode() bool {
	return false
}
