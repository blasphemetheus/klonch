package views

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dori/klonch/internal/db"
	"github.com/dori/klonch/internal/model"
	"github.com/dori/klonch/internal/ui/theme"
	"github.com/google/uuid"
)

// Local message types for kanban view
type kanbanErrorMsg struct{ err error }

// KanbanMode represents the current input mode
type KanbanMode int

const (
	KanbanModeNormal KanbanMode = iota
	KanbanModeAdd
	KanbanModeEdit
	KanbanModeSearch
	KanbanModeConfirmDelete
)

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

	// Input mode
	mode      KanbanMode
	textInput textinput.Model

	// For editing
	editTaskID string

	// For delete confirmation
	deleteTaskID string

	// Selectors
	selectingProject bool
	selectingTag     bool
	selectorCursor   int
	projects         []model.Project
	tags             []model.Tag

	// Filtering
	searchFilter    string
	filterProjectID string

	// Subtask counts: map[taskID] -> [total, done]
	subtaskCounts map[string][2]int
}

// NewKanbanView creates a new kanban view
func NewKanbanView(database *db.DB) KanbanView {
	ti := textinput.New()
	ti.Prompt = ""
	ti.CharLimit = 256

	return KanbanView{
		db:            database,
		selected:      make(map[string]bool),
		textInput:     ti,
		subtaskCounts: make(map[string][2]int),
	}
}

// Init initializes the kanban view
func (v KanbanView) Init() tea.Cmd {
	return tea.Batch(v.loadTasks(), v.loadProjectsAndTags())
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
			SELECT
				t.id, t.title, t.description, t.status, t.priority, t.project_id, t.due_date,
				(SELECT COUNT(*) FROM tasks st WHERE st.parent_id = t.id) as subtask_total,
				(SELECT COUNT(*) FROM tasks st WHERE st.parent_id = t.id AND st.status = 'done') as subtask_done
			FROM tasks t
			WHERE t.parent_id IS NULL AND t.status != 'archived'
			ORDER BY t.position, t.created_at
		`)
		if err != nil {
			return kanbanErrorMsg{err: err}
		}
		defer rows.Close()

		columns := [4][]model.Task{}
		subtaskCounts := make(map[string][2]int)

		for rows.Next() {
			var t model.Task
			var desc, projectID *string
			var dueDate *string
			var subtaskTotal, subtaskDone int
			if err := rows.Scan(&t.ID, &t.Title, &desc, &t.Status, &t.Priority, &projectID, &dueDate, &subtaskTotal, &subtaskDone); err != nil {
				continue
			}
			if desc != nil {
				t.Description = *desc
			}
			t.ProjectID = projectID

			// Store subtask counts
			if subtaskTotal > 0 {
				subtaskCounts[t.ID] = [2]int{subtaskTotal, subtaskDone}
			}

			// Assign to appropriate column based on status
			switch t.Status {
			case model.StatusBacklog:
				columns[ColumnBacklog] = append(columns[ColumnBacklog], t)
			case model.StatusPending:
				columns[ColumnTodo] = append(columns[ColumnTodo], t)
			case model.StatusInProgress:
				columns[ColumnInProgress] = append(columns[ColumnInProgress], t)
			case model.StatusDone:
				columns[ColumnDone] = append(columns[ColumnDone], t)
			}
		}

		return kanbanLoadedMsg{columns: columns, subtaskCounts: subtaskCounts}
	}
}

type kanbanLoadedMsg struct {
	columns       [4][]model.Task
	subtaskCounts map[string][2]int
}

// kanbanProjectsLoadedMsg is sent when projects are loaded
type kanbanProjectsLoadedMsg struct {
	projects []model.Project
	tags     []model.Tag
}

// Update handles messages
func (v KanbanView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case kanbanLoadedMsg:
		v.columns = msg.columns
		v.subtaskCounts = msg.subtaskCounts
		return v, nil

	case kanbanProjectsLoadedMsg:
		v.projects = msg.projects
		v.tags = msg.tags
		return v, nil

	case taskUpdatedMsg:
		return v, v.loadTasks()

	case tea.KeyMsg:
		// Handle different modes
		switch v.mode {
		case KanbanModeAdd:
			return v.handleAddMode(msg)
		case KanbanModeEdit:
			return v.handleEditMode(msg)
		case KanbanModeSearch:
			return v.handleSearchMode(msg)
		case KanbanModeConfirmDelete:
			return v.handleConfirmDeleteMode(msg)
		default:
			// Handle selectors
			if v.selectingProject {
				return v.handleProjectSelector(msg)
			}
			if v.selectingTag {
				return v.handleTagSelector(msg)
			}
			return v.handleNormalMode(msg)
		}
	}

	// Update text input if in input mode
	if v.mode == KanbanModeAdd || v.mode == KanbanModeEdit || v.mode == KanbanModeSearch {
		var cmd tea.Cmd
		v.textInput, cmd = v.textInput.Update(msg)
		return v, cmd
	}

	return v, nil
}

// handleNormalMode handles keys in normal mode
func (v KanbanView) handleNormalMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
		col := v.filteredColumn(int(v.currentColumn))
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

	case "tab":
		// Toggle done
		return v, v.toggleCurrentTask()

	case "g":
		v.cursorRow = 0
		v.columnScroll[v.currentColumn] = 0
		return v, nil

	case "G":
		col := v.filteredColumn(int(v.currentColumn))
		if len(col) > 0 {
			v.cursorRow = len(col) - 1
			v.ensureCursorVisible()
		}
		return v, nil

	// Add task
	case "a":
		v.mode = KanbanModeAdd
		v.textInput.SetValue("")
		v.textInput.Placeholder = "New task..."
		v.textInput.Focus()
		return v, nil

	// Edit task
	case "enter":
		col := v.filteredColumn(int(v.currentColumn))
		if len(col) > 0 && v.cursorRow < len(col) {
			task := col[v.cursorRow]
			v.mode = KanbanModeEdit
			v.editTaskID = task.ID
			v.textInput.SetValue(task.Title)
			v.textInput.Placeholder = ""
			v.textInput.Focus()
			v.textInput.CursorEnd()
		}
		return v, nil

	// Delete task
	case "d":
		col := v.filteredColumn(int(v.currentColumn))
		if len(col) > 0 && v.cursorRow < len(col) {
			task := col[v.cursorRow]
			v.deleteTaskID = task.ID
			v.mode = KanbanModeConfirmDelete
		}
		return v, nil

	// Cycle priority
	case "p":
		return v, v.cyclePriority()

	// Move to project
	case "m":
		col := v.filteredColumn(int(v.currentColumn))
		if len(col) > 0 && len(v.projects) > 0 {
			v.selectingProject = true
			v.selectorCursor = 0
		}
		return v, nil

	// Toggle tag
	case "t":
		col := v.filteredColumn(int(v.currentColumn))
		if len(col) > 0 && len(v.tags) > 0 {
			v.selectingTag = true
			v.selectorCursor = 0
		}
		return v, nil

	// Filter by project
	case "M":
		if len(v.projects) > 0 {
			v.selectingProject = true
			v.selectorCursor = 0
			// Mark that we're filtering, not assigning
			v.filterProjectID = "selecting"
		}
		return v, nil

	// Search
	case "/":
		v.mode = KanbanModeSearch
		v.textInput.SetValue(v.searchFilter)
		v.textInput.Placeholder = "Search..."
		v.textInput.Focus()
		return v, nil

	// Clear filters
	case "esc":
		if v.searchFilter != "" || v.filterProjectID != "" {
			v.searchFilter = ""
			v.filterProjectID = ""
			v.statusMsg = "Filters cleared"
		}
		return v, nil
	}

	return v, nil
}

// handleAddMode handles keys in add mode
func (v KanbanView) handleAddMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		title := strings.TrimSpace(v.textInput.Value())
		if title != "" {
			v.mode = KanbanModeNormal
			v.textInput.Blur()
			return v, v.createTask(title)
		}
		return v, nil
	case "esc":
		v.mode = KanbanModeNormal
		v.textInput.Blur()
		return v, nil
	}

	var cmd tea.Cmd
	v.textInput, cmd = v.textInput.Update(msg)
	return v, cmd
}

// handleEditMode handles keys in edit mode
func (v KanbanView) handleEditMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		title := strings.TrimSpace(v.textInput.Value())
		if title != "" && v.editTaskID != "" {
			v.mode = KanbanModeNormal
			v.textInput.Blur()
			taskID := v.editTaskID
			v.editTaskID = ""
			return v, v.updateTaskTitle(taskID, title)
		}
		return v, nil
	case "esc":
		v.mode = KanbanModeNormal
		v.textInput.Blur()
		v.editTaskID = ""
		return v, nil
	}

	var cmd tea.Cmd
	v.textInput, cmd = v.textInput.Update(msg)
	return v, cmd
}

// handleSearchMode handles keys in search mode
func (v KanbanView) handleSearchMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "esc":
		v.searchFilter = strings.TrimSpace(v.textInput.Value())
		v.mode = KanbanModeNormal
		v.textInput.Blur()
		// Reset cursor positions when filter changes
		v.cursorRow = 0
		for i := range v.columnScroll {
			v.columnScroll[i] = 0
		}
		return v, nil
	}

	var cmd tea.Cmd
	v.textInput, cmd = v.textInput.Update(msg)
	return v, cmd
}

// handleConfirmDeleteMode handles keys in delete confirmation mode
func (v KanbanView) handleConfirmDeleteMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		v.mode = KanbanModeNormal
		taskID := v.deleteTaskID
		v.deleteTaskID = ""
		return v, v.deleteTask(taskID)
	case "n", "N", "esc":
		v.mode = KanbanModeNormal
		v.deleteTaskID = ""
		return v, nil
	}
	return v, nil
}

// handleProjectSelector handles project selection
func (v KanbanView) handleProjectSelector(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if v.selectorCursor < len(v.projects)-1 {
			v.selectorCursor++
		}
	case "k", "up":
		if v.selectorCursor > 0 {
			v.selectorCursor--
		}
	case "enter":
		if v.selectorCursor < len(v.projects) {
			project := v.projects[v.selectorCursor]
			v.selectingProject = false

			// Check if filtering or assigning
			if v.filterProjectID == "selecting" {
				v.filterProjectID = project.ID
				v.cursorRow = 0
				for i := range v.columnScroll {
					v.columnScroll[i] = 0
				}
				v.statusMsg = fmt.Sprintf("Filtering by: %s", project.Name)
			} else {
				// Assign task to project
				col := v.filteredColumn(int(v.currentColumn))
				if len(col) > 0 && v.cursorRow < len(col) {
					task := col[v.cursorRow]
					return v, v.assignProject(task.ID, project.ID)
				}
			}
		}
	case "esc":
		v.selectingProject = false
		if v.filterProjectID == "selecting" {
			v.filterProjectID = ""
		}
	}
	return v, nil
}

// handleTagSelector handles tag selection
func (v KanbanView) handleTagSelector(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if v.selectorCursor < len(v.tags)-1 {
			v.selectorCursor++
		}
	case "k", "up":
		if v.selectorCursor > 0 {
			v.selectorCursor--
		}
	case "enter":
		if v.selectorCursor < len(v.tags) {
			tag := v.tags[v.selectorCursor]
			v.selectingTag = false

			col := v.filteredColumn(int(v.currentColumn))
			if len(col) > 0 && v.cursorRow < len(col) {
				task := col[v.cursorRow]
				return v, v.toggleTag(task.ID, tag.ID)
			}
		}
	case "esc":
		v.selectingTag = false
	}
	return v, nil
}

// clampCursor ensures cursor is valid for current column
func (v *KanbanView) clampCursor() {
	col := v.filteredColumn(int(v.currentColumn))
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
	// Column height is v.height - 3 (for header row and footer hints)
	// Border takes 2 lines, leaving v.height - 5 for content
	// Reserve 2 lines for scroll indicators (top/bottom)
	// Each item takes 1 line
	availableHeight := v.height - 7
	if availableHeight < 1 {
		return 1
	}
	return availableHeight
}

// moveTask moves the current task to an adjacent column
func (v KanbanView) moveTask(direction int) tea.Cmd {
	col := v.filteredColumn(int(v.currentColumn))
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
		newStatus = model.StatusBacklog
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
	col := v.filteredColumn(int(v.currentColumn))
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

// filteredColumn returns tasks for a column after applying filters
func (v *KanbanView) filteredColumn(colIndex int) []model.Task {
	tasks := v.columns[colIndex]
	if v.searchFilter == "" && v.filterProjectID == "" {
		return tasks
	}

	var filtered []model.Task
	searchLower := strings.ToLower(v.searchFilter)

	for _, task := range tasks {
		// Apply search filter
		if v.searchFilter != "" {
			if !strings.Contains(strings.ToLower(task.Title), searchLower) {
				continue
			}
		}

		// Apply project filter
		if v.filterProjectID != "" && v.filterProjectID != "selecting" {
			if task.ProjectID == nil || *task.ProjectID != v.filterProjectID {
				continue
			}
		}

		filtered = append(filtered, task)
	}
	return filtered
}

// createTask creates a new task in the current column
func (v KanbanView) createTask(title string) tea.Cmd {
	// Determine status based on current column
	var status model.Status
	switch v.currentColumn {
	case ColumnBacklog:
		status = model.StatusBacklog
	case ColumnTodo:
		status = model.StatusPending
	case ColumnInProgress:
		status = model.StatusInProgress
	case ColumnDone:
		status = model.StatusDone
	}

	return func() tea.Msg {
		id := uuid.New().String()
		now := time.Now()

		_, err := v.db.Exec(`
			INSERT INTO tasks (id, title, status, priority, project_id, created_at, updated_at)
			VALUES (?, ?, ?, 'medium', 'inbox', ?, ?)
		`, id, title, status, now, now)
		if err != nil {
			return kanbanErrorMsg{err: err}
		}
		return taskUpdatedMsg{}
	}
}

// updateTaskTitle updates a task's title
func (v KanbanView) updateTaskTitle(taskID, title string) tea.Cmd {
	return func() tea.Msg {
		_, err := v.db.Exec(`
			UPDATE tasks SET title = ?, updated_at = datetime('now')
			WHERE id = ?
		`, title, taskID)
		if err != nil {
			return kanbanErrorMsg{err: err}
		}
		return taskUpdatedMsg{}
	}
}

// deleteTask deletes a task
func (v KanbanView) deleteTask(taskID string) tea.Cmd {
	return func() tea.Msg {
		_, err := v.db.Exec(`DELETE FROM tasks WHERE id = ?`, taskID)
		if err != nil {
			return kanbanErrorMsg{err: err}
		}
		return taskUpdatedMsg{}
	}
}

// cyclePriority cycles the priority of the current task
func (v KanbanView) cyclePriority() tea.Cmd {
	col := v.filteredColumn(int(v.currentColumn))
	if len(col) == 0 || v.cursorRow >= len(col) {
		return nil
	}

	task := col[v.cursorRow]

	// Cycle: low -> medium -> high -> urgent -> low
	var newPriority model.Priority
	switch task.Priority {
	case model.PriorityLow:
		newPriority = model.PriorityMedium
	case model.PriorityMedium:
		newPriority = model.PriorityHigh
	case model.PriorityHigh:
		newPriority = model.PriorityUrgent
	case model.PriorityUrgent:
		newPriority = model.PriorityLow
	default:
		newPriority = model.PriorityMedium
	}

	return func() tea.Msg {
		_, err := v.db.Exec(`
			UPDATE tasks SET priority = ?, updated_at = datetime('now')
			WHERE id = ?
		`, newPriority, task.ID)
		if err != nil {
			return kanbanErrorMsg{err: err}
		}
		return taskUpdatedMsg{}
	}
}

// assignProject assigns a task to a project
func (v KanbanView) assignProject(taskID, projectID string) tea.Cmd {
	return func() tea.Msg {
		_, err := v.db.Exec(`
			UPDATE tasks SET project_id = ?, updated_at = datetime('now')
			WHERE id = ?
		`, projectID, taskID)
		if err != nil {
			return kanbanErrorMsg{err: err}
		}
		return taskUpdatedMsg{}
	}
}

// toggleTag toggles a tag on a task
func (v KanbanView) toggleTag(taskID, tagID string) tea.Cmd {
	return func() tea.Msg {
		// Check if tag exists on task
		var count int
		err := v.db.QueryRow(`
			SELECT COUNT(*) FROM task_tags WHERE task_id = ? AND tag_id = ?
		`, taskID, tagID).Scan(&count)
		if err != nil {
			return kanbanErrorMsg{err: err}
		}

		if count > 0 {
			// Remove tag
			_, err = v.db.Exec(`DELETE FROM task_tags WHERE task_id = ? AND tag_id = ?`, taskID, tagID)
		} else {
			// Add tag
			_, err = v.db.Exec(`INSERT INTO task_tags (task_id, tag_id) VALUES (?, ?)`, taskID, tagID)
		}

		if err != nil {
			return kanbanErrorMsg{err: err}
		}
		return taskUpdatedMsg{}
	}
}

// getProjectByID looks up a project by ID from the loaded projects
func (v *KanbanView) getProjectByID(id string) *model.Project {
	for i := range v.projects {
		if v.projects[i].ID == id {
			return &v.projects[i]
		}
	}
	return nil
}

// loadProjectsAndTags loads projects and tags from database
func (v KanbanView) loadProjectsAndTags() tea.Cmd {
	return func() tea.Msg {
		// Load projects
		var projects []model.Project
		rows, err := v.db.Query(`SELECT id, name, color FROM projects ORDER BY position, name`)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var p model.Project
				if err := rows.Scan(&p.ID, &p.Name, &p.Color); err == nil {
					projects = append(projects, p)
				}
			}
		}

		// Load tags
		var tags []model.Tag
		rows, err = v.db.Query(`SELECT id, name, color FROM tags ORDER BY name`)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var t model.Tag
				if err := rows.Scan(&t.ID, &t.Name, &t.Color); err == nil {
					tags = append(tags, t)
				}
			}
		}

		return kanbanProjectsLoadedMsg{projects: projects, tags: tags}
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

	// Responsive layout: show 2 columns when narrow, 4 when wide
	numVisibleCols := 4
	if v.width < 120 {
		numVisibleCols = 2
	}

	// Calculate which columns to show (centered on current column)
	startCol := 0
	if numVisibleCols == 2 {
		// Show pair containing current column: 0-1 or 2-3
		if v.currentColumn >= 2 {
			startCol = 2
		}
	}
	endCol := startCol + numVisibleCols

	// Calculate column width based on visible columns
	colWidth := (v.width - 4) / numVisibleCols
	if colWidth < 25 {
		colWidth = 25
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

	// Render headers - show "(filtered)" if filters active
	filterIndicator := ""
	if v.searchFilter != "" || (v.filterProjectID != "" && v.filterProjectID != "selecting") {
		filterIndicator = " *"
	}

	var headers []string
	for i := startCol; i < endCol; i++ {
		name := columnNames[i]
		tasks := v.filteredColumn(i)
		totalTasks := len(v.columns[i])
		header := fmt.Sprintf("%s (%d)", name, len(tasks))
		if len(tasks) != totalTasks && filterIndicator != "" {
			header = fmt.Sprintf("%s (%d/%d)", name, len(tasks), totalTasks)
		}
		headers = append(headers, headerStyle(i, i == int(v.currentColumn)).Render(header))
	}
	headerRow := lipgloss.JoinHorizontal(lipgloss.Top, headers...)

	// Render columns using filtered tasks
	visibleItems := v.visibleItemCount()
	var cols []string
	for i := startCol; i < endCol; i++ {
		tasks := v.filteredColumn(i)
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

			// Project name (inline, with color)
			var projectStr string
			if task.ProjectID != nil && *task.ProjectID != "inbox" {
				project := v.getProjectByID(*task.ProjectID)
				if project != nil {
					projectStyle := lipgloss.NewStyle().Foreground(t.Secondary)
					if project.Color != "" {
						projectStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(project.Color))
					}
					projectStr = projectStyle.Render("[" + project.Name + "] ")
				}
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

			// Subtask indicator (done/total)
			var subtaskStr string
			subtaskLen := 0
			if counts, ok := v.subtaskCounts[task.ID]; ok {
				total, done := counts[0], counts[1]
				subtaskStyle := lipgloss.NewStyle().Foreground(t.Subtle)
				if done == total {
					subtaskStyle = lipgloss.NewStyle().Foreground(t.Success)
				}
				subtaskStr = subtaskStyle.Render(fmt.Sprintf(" (%d/%d)", done, total))
				subtaskLen = len(fmt.Sprintf(" (%d/%d)", done, total))
			}

			// Truncate title to fit (account for project name and subtask indicator length)
			title := task.Title
			projectLen := 0
			if projectStr != "" {
				// Rough estimate of visible chars in project string
				if task.ProjectID != nil {
					if p := v.getProjectByID(*task.ProjectID); p != nil {
						projectLen = len(p.Name) + 3 // brackets + space
					}
				}
			}
			maxTitleLen := colWidth - 8 - projectLen - subtaskLen
			if maxTitleLen < 10 {
				maxTitleLen = 10
			}
			if len(title) > maxTitleLen {
				title = title[:maxTitleLen-3] + "..."
			}

			// Build card: priority + project + title + subtasks (single line)
			cardContent := fmt.Sprintf("%s %s%s%s", priorityChar, projectStr, title, subtaskStr)
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

	// Build footer based on mode
	var footer string
	inputStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(t.Primary).
		Padding(0, 1).
		Width(v.width - 4)

	switch v.mode {
	case KanbanModeAdd:
		footer = inputStyle.Render("Add task: " + v.textInput.View())
	case KanbanModeEdit:
		footer = inputStyle.Render("Edit: " + v.textInput.View())
	case KanbanModeSearch:
		footer = inputStyle.Render("Search: " + v.textInput.View())
	case KanbanModeConfirmDelete:
		// Find task title for confirmation
		taskTitle := ""
		col := v.filteredColumn(int(v.currentColumn))
		if v.cursorRow < len(col) {
			taskTitle = col[v.cursorRow].Title
		}
		confirmStyle := lipgloss.NewStyle().
			Foreground(t.Error).
			Bold(true)
		footer = confirmStyle.Render(fmt.Sprintf("Delete '%s'? (y/n)", taskTitle))
	default:
		// Show selector popup or normal hints
		if v.selectingProject {
			footer = v.renderProjectSelector(colWidth)
		} else if v.selectingTag {
			footer = v.renderTagSelector(colWidth)
		} else {
			// Show filter status and hints
			var filterStatus string
			if v.searchFilter != "" {
				filterStatus += fmt.Sprintf("Search: %s", v.searchFilter)
			}
			if v.filterProjectID != "" && v.filterProjectID != "selecting" {
				for _, p := range v.projects {
					if p.ID == v.filterProjectID {
						if filterStatus != "" {
							filterStatus += " | "
						}
						filterStatus += fmt.Sprintf("Project: %s", p.Name)
						break
					}
				}
			}

			// Show column position indicator for 2-column mode
			var colIndicator string
			if numVisibleCols == 2 {
				if startCol == 0 {
					colIndicator = "[1-2/4] "
				} else {
					colIndicator = "[3-4/4] "
				}
			}

			hints := "h/l: column • j/k: nav • H/L: move • a: add • enter: edit • d: del • p: priority • m: project • /: search"
			if filterStatus != "" {
				filterStatus = lipgloss.NewStyle().Foreground(t.Info).Render("[" + filterStatus + "] ")
				hints = filterStatus + "esc: clear"
			} else if colIndicator != "" {
				hints = lipgloss.NewStyle().Foreground(t.Info).Render(colIndicator) + hints
			}
			footer = lipgloss.NewStyle().Foreground(t.Subtle).Render(hints)
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left, headerRow, columnsRow, footer)
}

// renderProjectSelector renders the project selector popup
func (v KanbanView) renderProjectSelector(colWidth int) string {
	t := theme.Current.Theme

	var lines []string
	title := "Select Project:"
	if v.filterProjectID == "selecting" {
		title = "Filter by Project:"
	}
	lines = append(lines, lipgloss.NewStyle().Bold(true).Render(title))

	for i, p := range v.projects {
		style := lipgloss.NewStyle()
		if i == v.selectorCursor {
			style = style.Background(t.Highlight).Foreground(t.Foreground)
		}
		colorDot := lipgloss.NewStyle().Foreground(lipgloss.Color(p.Color)).Render("●")
		lines = append(lines, style.Render(fmt.Sprintf(" %s %s", colorDot, p.Name)))
	}
	lines = append(lines, lipgloss.NewStyle().Foreground(t.Subtle).Render("j/k: navigate • enter: select • esc: cancel"))

	return lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(t.Primary).
		Padding(0, 1).
		Render(strings.Join(lines, "\n"))
}

// renderTagSelector renders the tag selector popup
func (v KanbanView) renderTagSelector(colWidth int) string {
	t := theme.Current.Theme

	var lines []string
	lines = append(lines, lipgloss.NewStyle().Bold(true).Render("Toggle Tag:"))

	for i, tag := range v.tags {
		style := lipgloss.NewStyle()
		if i == v.selectorCursor {
			style = style.Background(t.Highlight).Foreground(t.Foreground)
		}
		colorDot := lipgloss.NewStyle().Foreground(lipgloss.Color(tag.Color)).Render("●")
		lines = append(lines, style.Render(fmt.Sprintf(" %s %s", colorDot, tag.Name)))
	}
	lines = append(lines, lipgloss.NewStyle().Foreground(t.Subtle).Render("j/k: navigate • enter: toggle • esc: cancel"))

	return lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(t.Primary).
		Padding(0, 1).
		Render(strings.Join(lines, "\n"))
}

// IsInputMode returns whether the view is in input mode
func (v KanbanView) IsInputMode() bool {
	return v.mode == KanbanModeAdd ||
		v.mode == KanbanModeEdit ||
		v.mode == KanbanModeSearch ||
		v.mode == KanbanModeConfirmDelete ||
		v.selectingProject ||
		v.selectingTag
}
