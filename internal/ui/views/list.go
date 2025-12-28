package views

import (
	"fmt"
	"os"
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

// Debug logging (enable by setting KLONCH_DEBUG=1)
var debugLog *os.File

func init() {
	if os.Getenv("KLONCH_DEBUG") == "1" {
		debugLog, _ = os.OpenFile("/tmp/klonch-debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	}
}

func debugf(format string, args ...interface{}) {
	if debugLog != nil {
		fmt.Fprintf(debugLog, format+"\n", args...)
		debugLog.Sync()
	}
}

// ListMode represents the current input mode of the list view
type ListMode int

const (
	ListModeNormal ListMode = iota
	ListModeAdd
	ListModeAddSubtask
	ListModeEdit
	ListModeSearch
	ListModeCommand
	ListModeConfirmDelete
)

// ListViewMode represents what tasks are shown
type ListViewMode int

const (
	ViewModeAll    ListViewMode = iota // Show all tasks
	ViewModeActive                     // Hide completed tasks
	ViewModeRecent                     // Active + recently completed (7 days)
)

func (m ListViewMode) String() string {
	switch m {
	case ViewModeAll:
		return "All"
	case ViewModeActive:
		return "Active"
	case ViewModeRecent:
		return "Recent"
	default:
		return "Unknown"
	}
}

// projectColors is a palette of distinct colors for projects
// These are hex colors that work well on both dark and light terminals
var projectColors = []string{
	"#E06C75", // Red/coral
	"#98C379", // Green
	"#E5C07B", // Yellow/gold
	"#61AFEF", // Blue
	"#C678DD", // Purple
	"#56B6C2", // Cyan
	"#D19A66", // Orange
	"#BE5046", // Dark red
	"#7EC699", // Mint
	"#E6B450", // Amber
	"#5C99D6", // Steel blue
	"#B57EDC", // Lavender
}

// tagColors is a palette of distinct colors for tags
// Slightly different from project colors for visual distinction
var tagColors = []string{
	"#FF6B9D", // Pink
	"#9ECE6A", // Lime
	"#7DCFFF", // Sky blue
	"#BB9AF7", // Violet
	"#F7768E", // Rose
	"#73DACA", // Teal
	"#FF9E64", // Peach
	"#E0AF68", // Sand
	"#2AC3DE", // Aqua
	"#B4F9F8", // Mint
	"#C0CAF5", // Periwinkle
	"#A9B1D6", // Slate
}

// FocusTaskRequest is sent when user wants to focus on a task
// (Defined here to avoid circular import with ui package)
type FocusTaskRequest struct {
	Task model.Task
}

// CommandDef defines a command for the command palette
type CommandDef struct {
	Name        string   // Primary command name
	Aliases     []string // Alternative names
	Description string   // What the command does
	Usage       string   // Usage example
	HasArgs     bool     // Whether it takes arguments
}

// allCommands is the list of available commands
var allCommands = []CommandDef{
	{Name: "due", Aliases: []string{"d"}, Description: "Set due date", Usage: "due tomorrow", HasArgs: true},
	{Name: "priority", Aliases: []string{"pri", "p"}, Description: "Set priority", Usage: "priority high", HasArgs: true},
	{Name: "tag", Aliases: []string{"t"}, Description: "Add tag to task", Usage: "tag @work", HasArgs: true},
	{Name: "project", Aliases: []string{"proj", "mv", "move"}, Description: "Move to project", Usage: "project inbox", HasArgs: true},
	{Name: "newproject", Aliases: []string{"np", "addproject"}, Description: "Create new project", Usage: "newproject Work", HasArgs: true},
	{Name: "recolor", Aliases: []string{}, Description: "Reassign colors to all projects", Usage: "recolor", HasArgs: false},
	{Name: "newtag", Aliases: []string{"nt", "addtag"}, Description: "Create new tag", Usage: "newtag @urgent", HasArgs: true},
	{Name: "recolortags", Aliases: []string{}, Description: "Reassign colors to all tags", Usage: "recolortags", HasArgs: false},
	{Name: "done", Aliases: []string{"complete", "finish"}, Description: "Toggle done status", Usage: "done", HasArgs: false},
	{Name: "archive", Aliases: []string{"arch"}, Description: "Archive task(s)", Usage: "archive", HasArgs: false},
	{Name: "delete", Aliases: []string{"del", "rm"}, Description: "Delete task(s)", Usage: "delete", HasArgs: false},
	{Name: "theme", Aliases: []string{}, Description: "Change theme", Usage: "theme nord", HasArgs: true},
	{Name: "sort", Aliases: []string{}, Description: "Sort tasks", Usage: "sort priority", HasArgs: true},
	{Name: "filter", Aliases: []string{"f"}, Description: "Filter tasks by text", Usage: "filter @work", HasArgs: true},
	{Name: "filterproject", Aliases: []string{"fp"}, Description: "Filter by project", Usage: "filterproject", HasArgs: false},
	{Name: "filtertag", Aliases: []string{"ft"}, Description: "Filter by tag", Usage: "filtertag", HasArgs: false},
	{Name: "clear", Aliases: []string{}, Description: "Clear all filters", Usage: "clear", HasArgs: false},
	{Name: "projects", Aliases: []string{"lsp"}, Description: "List all projects", Usage: "projects", HasArgs: false},
	{Name: "tags", Aliases: []string{"lst"}, Description: "List all tags", Usage: "tags", HasArgs: false},
	{Name: "starttime", Aliases: []string{"start", "track"}, Description: "Start time tracking", Usage: "starttime", HasArgs: false},
	{Name: "stoptime", Aliases: []string{"stop"}, Description: "Stop time tracking", Usage: "stoptime", HasArgs: false},
	{Name: "addtime", Aliases: []string{"logtime"}, Description: "Log time manually", Usage: "addtime 30m", HasArgs: true},
	{Name: "help", Aliases: []string{"h", "?"}, Description: "Show available commands", Usage: "help", HasArgs: false},
}

// ListView displays tasks in a list format
type ListView struct {
	db     *db.DB
	width  int
	height int

	allTasks []model.Task    // All loaded tasks (top-level only)
	tasks    []model.Task    // Flattened tasks for display (includes expanded subtasks)
	projects []model.Project // All projects
	tags     []model.Tag     // All tags
	cursor   int
	selected map[string]bool   // Selected task IDs
	expanded map[string]bool   // Tasks with expanded subtasks
	blocked  map[string]bool   // Tasks with incomplete dependencies

	mode         ListMode
	input        textinput.Model
	editingID    string
	parentID     string // For creating subtasks
	searchFilter string // Current search filter
	statusMsg    string // Status message to display

	// Structured filters (combine with AND logic)
	filterProjectID string   // Filter by specific project (empty = all)
	filterTagIDs    []string // Filter by tags (all must match)

	// For project/tag selection
	selectingProject bool
	selectingTag     bool
	selectorCursor   int

	// For filter selection (vs assignment)
	selectingProjectFilter bool
	selectingTagFilter     bool

	// For dependency selection
	selectingDep     bool
	depTaskID        string   // Task to add/remove dependency for
	depTasks         []model.Task // Available tasks to depend on

	// For delete confirmation
	deleteIDs []string

	// For command palette
	cmdSuggestions []CommandDef // Filtered command suggestions
	cmdCursor      int          // Selected suggestion index

	// For time tracking
	activeTimeEntryID string    // Currently active time entry ID (empty if not tracking)
	activeTaskID      string    // Task being tracked
	trackingStarted   time.Time // When tracking started

	// For deferred sorting (e.g., after priority change)
	deferResortTaskID string // Task ID that was modified; resort when cursor moves away

	// For focusing on newly created task
	focusAfterLoadTaskID string // Task ID to focus after next loadTasks completes

	// View mode filter
	viewMode ListViewMode // What tasks to show (All, Active, Recent, etc.)
}

// NewListView creates a new list view
func NewListView(database *db.DB) ListView {
	ti := textinput.New()
	ti.Placeholder = "New task..."
	ti.CharLimit = 256

	return ListView{
		db:       database,
		selected: make(map[string]bool),
		expanded: make(map[string]bool),
		blocked:  make(map[string]bool),
		input:    ti,
	}
}

// Init initializes the list view
func (v ListView) Init() tea.Cmd {
	debugf("ListView.Init() called")
	return v.loadTasks
}

// IsInputMode returns true when the view is capturing text input
// (add, edit, subtask, search, command modes or any selector is active)
func (v ListView) IsInputMode() bool {
	if v.mode == ListModeAdd || v.mode == ListModeEdit || v.mode == ListModeAddSubtask || v.mode == ListModeSearch || v.mode == ListModeCommand {
		return true
	}
	if v.selectingProject || v.selectingTag || v.selectingDep || v.selectingProjectFilter || v.selectingTagFilter {
		return true
	}
	if v.mode == ListModeConfirmDelete {
		return true
	}
	return false
}

// SetSize updates the view dimensions
func (v ListView) SetSize(width, height int) ListView {
	v.width = width
	v.height = height
	v.input.Width = width - 4
	return v
}

// Update handles messages for the list view
func (v ListView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tasksLoadedMsg:
		debugf("tasksLoadedMsg received, err=%v, count=%d", msg.err, len(msg.tasks))
		if msg.err != nil {
			v.statusMsg = fmt.Sprintf("Error loading tasks: %v", msg.err)
			return v, func() tea.Msg {
				return errorMsg{err: msg.err}
			}
		}
		v.allTasks = msg.tasks
		v.projects = msg.projects
		v.tags = msg.tags
		v.blocked = msg.blocked
		v.applyFilter() // Apply hideDone and searchFilter
		v.statusMsg = fmt.Sprintf("Loaded %d tasks", len(v.tasks))
		debugf("v.tasks now has %d items (flattened)", len(v.tasks))

		// If we need to focus on a specific task (e.g., newly created)
		if v.focusAfterLoadTaskID != "" {
			for i, t := range v.tasks {
				if t.ID == v.focusAfterLoadTaskID {
					v.cursor = i
					// Enable deferred resort so user can change priority without immediate re-sort
					v.deferResortTaskID = v.focusAfterLoadTaskID
					break
				}
			}
			v.focusAfterLoadTaskID = ""
		} else if v.cursor >= len(v.tasks) {
			v.cursor = max(0, len(v.tasks)-1)
		}
		return v, nil

	case taskCreatedMsg:
		if msg.err != nil {
			return v, func() tea.Msg {
				return errorMsg{err: msg.err}
			}
		}
		// Focus on the new task after reload so user can adjust priority
		v.focusAfterLoadTaskID = msg.task.ID
		return v, v.loadTasks

	case taskUpdatedMsg:
		if msg.err != nil {
			return v, func() tea.Msg {
				return errorMsg{err: msg.err}
			}
		}
		return v, v.loadTasks

	case taskDeletedMsg:
		if msg.err != nil {
			return v, func() tea.Msg {
				return errorMsg{err: msg.err}
			}
		}
		v.selected = make(map[string]bool)
		return v, v.loadTasks

	case priorityChangedLocalMsg:
		debugf("priorityChangedLocalMsg received: taskID=%s, newPriority=%s", msg.taskID, msg.newPriority)
		if msg.err != nil {
			return v, func() tea.Msg {
				return errorMsg{err: msg.err}
			}
		}
		// Update local state without full reload - resort deferred until cursor moves
		v.updateLocalTaskPriority(msg.taskID, msg.newPriority)
		v.deferResortTaskID = msg.taskID
		v.statusMsg = fmt.Sprintf("Priority: %s (move cursor to resort)", msg.newPriority)
		debugf("Local state updated, deferResortTaskID=%s, NOT calling loadTasks", v.deferResortTaskID)
		return v, nil

	case timeTrackingStartedMsg:
		v.statusMsg = fmt.Sprintf("Started tracking: %s", msg.taskTitle)
		return v, nil

	case timeTrackingStoppedMsg:
		v.statusMsg = fmt.Sprintf("Stopped tracking: %d minutes logged", msg.duration)
		return v, nil

	case timeAddedMsg:
		v.statusMsg = fmt.Sprintf("Added %d minutes to task", msg.minutes)
		return v, nil

	case tea.KeyMsg:
		switch v.mode {
		case ListModeAdd:
			return v.handleAddMode(msg)
		case ListModeAddSubtask:
			return v.handleAddSubtaskMode(msg)
		case ListModeEdit:
			return v.handleEditMode(msg)
		case ListModeSearch:
			return v.handleSearchMode(msg)
		case ListModeCommand:
			return v.handleCommandMode(msg)
		case ListModeConfirmDelete:
			return v.handleDeleteConfirm(msg)
		default:
			return v.handleNormalMode(msg)
		}
	}

	// Update text input if in input mode
	if v.mode == ListModeAdd || v.mode == ListModeAddSubtask || v.mode == ListModeEdit || v.mode == ListModeSearch || v.mode == ListModeCommand {
		var cmd tea.Cmd
		v.input, cmd = v.input.Update(msg)
		cmds = append(cmds, cmd)
	}

	return v, tea.Batch(cmds...)
}

// handleNormalMode handles keypresses in normal mode
func (v ListView) handleNormalMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Clear status on any keypress
	v.statusMsg = ""

	// Handle project selector first if active
	if v.selectingProject {
		return v.handleProjectSelector(msg)
	}

	// Handle tag selector first if active
	if v.selectingTag {
		return v.handleTagSelector(msg)
	}

	// Handle dependency selector first if active
	if v.selectingDep {
		return v.handleDependencySelector(msg)
	}

	// Handle project filter selector
	if v.selectingProjectFilter {
		return v.handleProjectFilterSelector(msg)
	}

	// Handle tag filter selector
	if v.selectingTagFilter {
		return v.handleTagFilterSelector(msg)
	}

	switch msg.String() {
	// Navigation
	case "up", "k":
		if v.cursor > 0 {
			oldCursor := v.cursor
			v.cursor--
			if cmd := v.checkDeferredResort(oldCursor); cmd != nil {
				return v, cmd
			}
		}
	case "down", "j":
		if v.cursor < len(v.tasks)-1 {
			oldCursor := v.cursor
			v.cursor++
			if cmd := v.checkDeferredResort(oldCursor); cmd != nil {
				return v, cmd
			}
		}
	case "g":
		oldCursor := v.cursor
		v.cursor = 0
		if cmd := v.checkDeferredResort(oldCursor); cmd != nil {
			return v, cmd
		}
	case "G":
		oldCursor := v.cursor
		v.cursor = max(0, len(v.tasks)-1)
		if cmd := v.checkDeferredResort(oldCursor); cmd != nil {
			return v, cmd
		}

	// Selection
	case " ":
		if len(v.tasks) > 0 {
			id := v.tasks[v.cursor].ID
			if v.selected[id] {
				delete(v.selected, id)
			} else {
				v.selected[id] = true
			}
		}
	case "V":
		// Select all
		for _, t := range v.tasks {
			v.selected[t.ID] = true
		}
	case "esc":
		// Clear in order: selection -> filters -> expanded subtasks
		if len(v.selected) > 0 {
			v.selected = make(map[string]bool)
		} else if v.hasActiveFilters() {
			v.searchFilter = ""
			v.filterProjectID = ""
			v.filterTagIDs = nil
			v.applyFilter()
		} else if len(v.expanded) > 0 {
			// Collapse all expanded tasks
			v.expanded = make(map[string]bool)
			v.applyFilter() // Refresh flattened task list
		}

	// Actions
	case "a":
		v.mode = ListModeAdd
		v.input.SetValue("")
		v.input.Placeholder = "New task..."
		v.input.Focus()
		return v, textinput.Blink

	case "enter":
		if len(v.tasks) > 0 {
			v.mode = ListModeEdit
			v.editingID = v.tasks[v.cursor].ID
			v.input.SetValue(v.tasks[v.cursor].Title)
			v.input.Focus()
			return v, textinput.Blink
		}

	case "tab":
		// Toggle done
		return v, v.toggleSelected()

	case "d":
		// Delete
		if len(v.selected) > 0 {
			v.deleteIDs = make([]string, 0, len(v.selected))
			for id := range v.selected {
				v.deleteIDs = append(v.deleteIDs, id)
			}
		} else if len(v.tasks) > 0 {
			v.deleteIDs = []string{v.tasks[v.cursor].ID}
		}
		if len(v.deleteIDs) > 0 {
			v.mode = ListModeConfirmDelete
		}

	case "p":
		// Cycle priority
		return v, v.cyclePriority()

	case "/":
		v.mode = ListModeSearch
		v.input.SetValue(v.searchFilter)
		v.input.Placeholder = "Search tasks..."
		v.input.Focus()
		return v, textinput.Blink

	case ":":
		v.mode = ListModeCommand
		v.input.SetValue("")
		v.input.Placeholder = "Command..."
		v.input.Focus()
		v.cmdSuggestions = allCommands // Show all commands initially
		v.cmdCursor = 0
		return v, textinput.Blink

	case "m":
		// Move to project
		if len(v.tasks) > 0 && len(v.projects) > 0 {
			v.selectingProject = true
			v.selectorCursor = 0
		}
		return v, nil

	case "t":
		// Add/toggle tag
		if len(v.tasks) > 0 && len(v.tags) > 0 {
			v.selectingTag = true
			v.selectorCursor = 0
		} else if len(v.tags) == 0 {
			v.statusMsg = "No tags yet. Create one with: klonch add \"task @tagname\""
		}
		return v, nil

	case "M":
		// Filter by project
		if len(v.projects) > 0 {
			v.selectingProjectFilter = true
			v.selectorCursor = 0
		} else {
			v.statusMsg = "No projects yet. Create one with :newproject"
		}
		return v, nil

	case "T":
		// Filter by tag
		if len(v.tags) > 0 {
			v.selectingTagFilter = true
			v.selectorCursor = 0
		} else {
			v.statusMsg = "No tags yet. Create one with :newtag"
		}
		return v, nil

	case "s":
		// Add subtask
		if len(v.tasks) > 0 {
			task := v.tasks[v.cursor]
			// Can't add subtask to a subtask (only one level of nesting)
			if task.ParentID != nil {
				v.statusMsg = "Cannot create nested subtasks (only one level allowed)"
				return v, nil
			}
			v.mode = ListModeAddSubtask
			v.parentID = task.ID
			v.input.SetValue("")
			v.input.Placeholder = "New subtask..."
			v.input.Focus()
			// Auto-expand the parent task
			v.expanded[task.ID] = true
			return v, textinput.Blink
		}
		return v, nil

	case "o":
		// Toggle expand/collapse subtasks
		if len(v.tasks) > 0 {
			task := v.tasks[v.cursor]
			// Only toggle for parent tasks (not subtasks themselves)
			if task.ParentID == nil && len(task.Subtasks) > 0 {
				v.expanded[task.ID] = !v.expanded[task.ID]
				v.tasks = v.flattenTasks(v.allTasks)
			} else if len(task.Subtasks) == 0 && task.ParentID == nil {
				v.statusMsg = "No subtasks to expand"
			}
		}
		return v, nil

	case "b":
		// Manage dependencies (blocked by)
		if len(v.tasks) > 0 {
			task := v.tasks[v.cursor]
			// Don't allow dependencies on subtasks
			if task.ParentID != nil {
				v.statusMsg = "Subtasks cannot have dependencies"
				return v, nil
			}
			// Build list of possible dependencies (all top-level tasks except self)
			v.depTasks = nil
			for _, t := range v.allTasks {
				if t.ID != task.ID && t.ParentID == nil {
					v.depTasks = append(v.depTasks, t)
				}
			}
			if len(v.depTasks) == 0 {
				v.statusMsg = "No other tasks to depend on"
				return v, nil
			}
			v.selectingDep = true
			v.depTaskID = task.ID
			v.selectorCursor = 0
		}
		return v, nil

	case "f":
		// Focus mode - single task view
		if len(v.tasks) > 0 {
			task := v.tasks[v.cursor]
			return v, func() tea.Msg {
				return FocusTaskRequest{Task: task}
			}
		}
		return v, nil

	case "r":
		// Refresh/reload tasks
		return v, v.loadTasks

	case "H":
		// Cycle through view modes: All -> Active -> Recent -> All
		switch v.viewMode {
		case ViewModeAll:
			v.viewMode = ViewModeActive
		case ViewModeActive:
			v.viewMode = ViewModeRecent
		case ViewModeRecent:
			v.viewMode = ViewModeAll
		}
		v.applyFilter()
		v.statusMsg = fmt.Sprintf("View: %s (H to cycle)", v.viewMode.String())
		return v, nil
	}

	return v, nil
}

// handleAddMode handles keypresses in add mode
func (v ListView) handleAddMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		title := strings.TrimSpace(v.input.Value())
		if title != "" {
			v.mode = ListModeNormal
			v.input.Blur()
			return v, v.createTask(title)
		}
	case "esc":
		v.mode = ListModeNormal
		v.input.Blur()
		return v, nil
	}

	var cmd tea.Cmd
	v.input, cmd = v.input.Update(msg)
	return v, cmd
}

// handleAddSubtaskMode handles keypresses when adding a subtask
func (v ListView) handleAddSubtaskMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		title := strings.TrimSpace(v.input.Value())
		if title != "" {
			v.mode = ListModeNormal
			v.input.Blur()
			return v, v.createSubtask(title, v.parentID)
		}
	case "esc":
		v.mode = ListModeNormal
		v.input.Blur()
		v.parentID = ""
		return v, nil
	}

	var cmd tea.Cmd
	v.input, cmd = v.input.Update(msg)
	return v, cmd
}

// handleEditMode handles keypresses in edit mode
func (v ListView) handleEditMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		title := strings.TrimSpace(v.input.Value())
		if title != "" {
			v.mode = ListModeNormal
			v.input.Blur()
			return v, v.updateTaskTitle(v.editingID, title)
		}
	case "esc":
		v.mode = ListModeNormal
		v.input.Blur()
		return v, nil
	}

	var cmd tea.Cmd
	v.input, cmd = v.input.Update(msg)
	return v, cmd
}

// handleSearchMode handles keypresses in search mode
func (v ListView) handleSearchMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		// Apply search and stay in normal mode
		v.searchFilter = strings.TrimSpace(v.input.Value())
		v.mode = ListModeNormal
		v.input.Blur()
		v.applyFilter()
		return v, nil

	case "esc":
		// Cancel search (but keep current filter if any)
		v.mode = ListModeNormal
		v.input.Blur()
		// Restore filter value in case user was editing
		v.input.SetValue(v.searchFilter)
		return v, nil
	}

	// Update input and apply filter in real-time
	var cmd tea.Cmd
	v.input, cmd = v.input.Update(msg)

	// Apply filter as user types
	v.searchFilter = v.input.Value()
	v.applyFilter()

	return v, cmd
}

// handleCommandMode handles keypresses in command mode
func (v ListView) handleCommandMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		command := strings.TrimSpace(v.input.Value())
		// If there are suggestions visible, use the selected one
		if len(v.cmdSuggestions) > 0 && v.cmdCursor < len(v.cmdSuggestions) {
			selectedCmd := v.cmdSuggestions[v.cmdCursor]
			// Check if input is a prefix filter (no space = no args yet)
			if !strings.Contains(command, " ") {
				// Use selected command name
				command = selectedCmd.Name
			} else {
				// User typed args - replace the command part with selected, keep args
				parts := strings.SplitN(command, " ", 2)
				if len(parts) == 2 {
					command = selectedCmd.Name + " " + parts[1]
				}
			}
		}
		v.mode = ListModeNormal
		v.input.Blur()
		v.cmdSuggestions = nil
		v.cmdCursor = 0
		if command != "" {
			return v.executeCommand(command)
		}
		return v, nil

	case "esc":
		v.mode = ListModeNormal
		v.input.Blur()
		v.cmdSuggestions = nil
		v.cmdCursor = 0
		return v, nil

	case "tab":
		// Auto-complete with selected suggestion
		if len(v.cmdSuggestions) > 0 && v.cmdCursor < len(v.cmdSuggestions) {
			cmd := v.cmdSuggestions[v.cmdCursor]
			if cmd.HasArgs {
				v.input.SetValue(cmd.Name + " ")
				v.input.CursorEnd()
			} else {
				v.input.SetValue(cmd.Name)
				v.input.CursorEnd()
			}
			v.updateCommandSuggestions()
		}
		return v, nil

	case "up", "ctrl+p":
		if v.cmdCursor > 0 {
			v.cmdCursor--
		}
		return v, nil

	case "down", "ctrl+n":
		if v.cmdCursor < len(v.cmdSuggestions)-1 {
			v.cmdCursor++
		}
		return v, nil
	}

	var cmd tea.Cmd
	v.input, cmd = v.input.Update(msg)

	// Update suggestions based on current input
	v.updateCommandSuggestions()

	return v, cmd
}

// updateCommandSuggestions filters commands based on current input
func (v *ListView) updateCommandSuggestions() {
	input := strings.TrimSpace(strings.ToLower(v.input.Value()))

	// If input contains a space, user is typing arguments - don't show suggestions
	if strings.Contains(input, " ") {
		v.cmdSuggestions = nil
		v.cmdCursor = 0
		return
	}

	var matches []CommandDef
	for _, cmd := range allCommands {
		// Check if command name or any alias starts with input
		if strings.HasPrefix(cmd.Name, input) {
			matches = append(matches, cmd)
			continue
		}
		for _, alias := range cmd.Aliases {
			if strings.HasPrefix(alias, input) {
				matches = append(matches, cmd)
				break
			}
		}
	}

	v.cmdSuggestions = matches
	// Clamp cursor
	if v.cmdCursor >= len(v.cmdSuggestions) {
		v.cmdCursor = 0
	}
}

// executeCommand parses and executes a command
func (v ListView) executeCommand(command string) (tea.Model, tea.Cmd) {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return v, nil
	}

	cmd := strings.ToLower(parts[0])
	args := parts[1:]

	switch cmd {
	case "due", "d":
		return v.cmdSetDue(args)
	case "priority", "pri", "p":
		return v.cmdSetPriority(args)
	case "tag", "t":
		return v.cmdAddTag(args)
	case "project", "proj", "mv":
		return v.cmdMoveToProject(args)
	case "newproject", "np", "addproject":
		return v.cmdNewProject(args)
	case "recolor":
		return v.cmdRecolorProjects()
	case "newtag", "nt", "addtag":
		return v.cmdNewTag(args)
	case "recolortags":
		return v.cmdRecolorTags()
	case "projects", "lsp":
		return v.cmdListProjects()
	case "tags", "lst":
		return v.cmdListTags()
	case "archive", "arch":
		return v.cmdArchive()
	case "done", "complete":
		return v.cmdToggleDone()
	case "theme":
		return v.cmdSetTheme(args)
	case "starttime", "start", "track":
		return v.cmdStartTime()
	case "stoptime", "stop":
		return v.cmdStopTime()
	case "addtime", "logtime":
		return v.cmdAddTime(args)
	case "help", "h", "?":
		return v.cmdShowHelp()
	case "filter", "f":
		return v.cmdFilter(args)
	case "filterproject", "fp":
		return v.cmdFilterProject()
	case "filtertag", "ft":
		return v.cmdFilterTag()
	case "clear":
		return v.cmdClearFilters()
	case "sort":
		return v.cmdSort(args)
	default:
		v.statusMsg = fmt.Sprintf("Unknown command: %s", cmd)
		return v, nil
	}
}

// cmdSetDue sets the due date for selected/current task
func (v ListView) cmdSetDue(args []string) (tea.Model, tea.Cmd) {
	if len(args) == 0 {
		v.statusMsg = "Usage: due <date> (e.g., due tomorrow, due friday, due 2024-01-15)"
		return v, nil
	}

	dateStr := strings.Join(args, " ")
	parsed := parseNaturalDate(dateStr)
	if parsed == nil {
		v.statusMsg = fmt.Sprintf("Could not parse date: %s", dateStr)
		return v, nil
	}

	taskIDs := v.getTargetTaskIDs()
	if len(taskIDs) == 0 {
		v.statusMsg = "No task selected"
		return v, nil
	}

	return v, v.setDueDate(taskIDs, *parsed)
}

// cmdSetPriority sets the priority for selected/current task
func (v ListView) cmdSetPriority(args []string) (tea.Model, tea.Cmd) {
	if len(args) == 0 {
		v.statusMsg = "Usage: priority <low|medium|high|urgent>"
		return v, nil
	}

	priority := strings.ToLower(args[0])
	var p model.Priority
	switch priority {
	case "low", "l":
		p = model.PriorityLow
	case "medium", "med", "m":
		p = model.PriorityMedium
	case "high", "hi", "h":
		p = model.PriorityHigh
	case "urgent", "u":
		p = model.PriorityUrgent
	default:
		v.statusMsg = fmt.Sprintf("Unknown priority: %s (use low, medium, high, or urgent)", priority)
		return v, nil
	}

	taskIDs := v.getTargetTaskIDs()
	if len(taskIDs) == 0 {
		v.statusMsg = "No task selected"
		return v, nil
	}

	return v, v.setPriority(taskIDs, p)
}

// cmdAddTag adds a tag to selected/current task
func (v ListView) cmdAddTag(args []string) (tea.Model, tea.Cmd) {
	if len(args) == 0 {
		v.statusMsg = "Usage: tag <tagname> (e.g., tag @work)"
		return v, nil
	}

	tagName := args[0]
	// Remove @ prefix if present
	tagName = strings.TrimPrefix(tagName, "@")

	taskIDs := v.getTargetTaskIDs()
	if len(taskIDs) == 0 {
		v.statusMsg = "No task selected"
		return v, nil
	}

	return v, v.addTagToTasks(taskIDs, tagName)
}

// cmdMoveToProject moves task to a project
func (v ListView) cmdMoveToProject(args []string) (tea.Model, tea.Cmd) {
	if len(args) == 0 {
		v.statusMsg = "Usage: project <name> (e.g., project work)"
		return v, nil
	}

	projectName := strings.ToLower(strings.Join(args, " "))

	// Find project by name
	var projectID string
	for _, p := range v.projects {
		if strings.ToLower(p.Name) == projectName || strings.ToLower(p.ID) == projectName {
			projectID = p.ID
			break
		}
	}

	if projectID == "" {
		v.statusMsg = fmt.Sprintf("Project not found: %s", projectName)
		return v, nil
	}

	taskIDs := v.getTargetTaskIDs()
	if len(taskIDs) == 0 {
		v.statusMsg = "No task selected"
		return v, nil
	}

	return v, v.moveToProject(taskIDs, projectID)
}

// cmdNewProject creates a new project
func (v ListView) cmdNewProject(args []string) (tea.Model, tea.Cmd) {
	if len(args) == 0 {
		v.statusMsg = "Usage: newproject <name> (e.g., newproject Work)"
		return v, nil
	}

	projectName := strings.Join(args, " ")

	// Check if project already exists
	for _, p := range v.projects {
		if strings.ToLower(p.Name) == strings.ToLower(projectName) {
			v.statusMsg = fmt.Sprintf("Project already exists: %s", p.Name)
			return v, nil
		}
	}

	// Auto-assign color from palette (cycle through based on existing project count)
	color := projectColors[len(v.projects)%len(projectColors)]

	// Create new project
	projectID := uuid.New().String()
	return v, func() tea.Msg {
		_, err := v.db.Exec(`
			INSERT INTO projects (id, name, color, position, created_at, updated_at)
			VALUES (?, ?, ?, ?, datetime('now'), datetime('now'))
		`, projectID, projectName, color, len(v.projects))
		if err != nil {
			return taskUpdatedMsg{err: err}
		}
		return taskUpdatedMsg{} // Triggers loadTasks in handler
	}
}

// cmdRecolorProjects assigns fresh colors to all projects
func (v ListView) cmdRecolorProjects() (tea.Model, tea.Cmd) {
	if len(v.projects) == 0 {
		v.statusMsg = "No projects to recolor"
		return v, nil
	}

	return v, func() tea.Msg {
		for i, p := range v.projects {
			color := projectColors[i%len(projectColors)]
			_, err := v.db.Exec(`UPDATE projects SET color = ? WHERE id = ?`, color, p.ID)
			if err != nil {
				return taskUpdatedMsg{err: err}
			}
		}
		return taskUpdatedMsg{} // Triggers reload
	}
}

// cmdNewTag creates a new tag
func (v ListView) cmdNewTag(args []string) (tea.Model, tea.Cmd) {
	if len(args) == 0 {
		v.statusMsg = "Usage: newtag <name> (e.g., newtag @urgent)"
		return v, nil
	}

	tagName := args[0]
	tagName = strings.TrimPrefix(tagName, "@")

	// Check if tag already exists
	for _, t := range v.tags {
		if strings.ToLower(t.Name) == strings.ToLower(tagName) {
			v.statusMsg = fmt.Sprintf("Tag already exists: @%s", t.Name)
			return v, nil
		}
	}

	// Auto-assign color from palette
	color := tagColors[len(v.tags)%len(tagColors)]

	// Create new tag
	tagID := uuid.New().String()
	return v, func() tea.Msg {
		_, err := v.db.Exec(`
			INSERT INTO tags (id, name, color, created_at)
			VALUES (?, ?, ?, datetime('now'))
		`, tagID, tagName, color)
		if err != nil {
			return taskUpdatedMsg{err: err}
		}
		return taskUpdatedMsg{} // Triggers loadTasks in handler
	}
}

// cmdRecolorTags assigns fresh colors to all tags
func (v ListView) cmdRecolorTags() (tea.Model, tea.Cmd) {
	if len(v.tags) == 0 {
		v.statusMsg = "No tags to recolor"
		return v, nil
	}

	return v, func() tea.Msg {
		for i, t := range v.tags {
			color := tagColors[i%len(tagColors)]
			_, err := v.db.Exec(`UPDATE tags SET color = ? WHERE id = ?`, color, t.ID)
			if err != nil {
				return taskUpdatedMsg{err: err}
			}
		}
		return taskUpdatedMsg{} // Triggers reload
	}
}

// cmdListProjects lists all projects
func (v ListView) cmdListProjects() (tea.Model, tea.Cmd) {
	if len(v.projects) == 0 {
		v.statusMsg = "No projects. Create one with: newproject <name>"
		return v, nil
	}

	var names []string
	for _, p := range v.projects {
		names = append(names, p.Name)
	}
	v.statusMsg = fmt.Sprintf("Projects: %s", strings.Join(names, ", "))
	return v, nil
}

// cmdListTags lists all tags
func (v ListView) cmdListTags() (tea.Model, tea.Cmd) {
	if len(v.tags) == 0 {
		v.statusMsg = "No tags. Create one with: newtag <name>"
		return v, nil
	}

	var names []string
	for _, t := range v.tags {
		names = append(names, "@"+t.Name)
	}
	v.statusMsg = fmt.Sprintf("Tags: %s", strings.Join(names, ", "))
	return v, nil
}

// cmdArchive archives selected/current task
func (v ListView) cmdArchive() (tea.Model, tea.Cmd) {
	taskIDs := v.getTargetTaskIDs()
	if len(taskIDs) == 0 {
		v.statusMsg = "No task selected"
		return v, nil
	}

	return v, v.archiveTasks(taskIDs)
}

// cmdToggleDone toggles done status
func (v ListView) cmdToggleDone() (tea.Model, tea.Cmd) {
	if len(v.tasks) == 0 {
		v.statusMsg = "No task selected"
		return v, nil
	}
	return v, v.toggleSelected()
}

// cmdSetTheme changes the theme
func (v ListView) cmdSetTheme(args []string) (tea.Model, tea.Cmd) {
	if len(args) == 0 {
		themes := theme.Available()
		names := make([]string, len(themes))
		for i, t := range themes {
			names[i] = t.Name
		}
		v.statusMsg = fmt.Sprintf("Usage: theme <%s>", strings.Join(names, "|"))
		return v, nil
	}

	themeName := strings.ToLower(args[0])
	if t, ok := theme.ByName(themeName); ok {
		theme.SetTheme(t)
		v.statusMsg = fmt.Sprintf("Theme set to: %s", t.Name)
	} else {
		v.statusMsg = fmt.Sprintf("Unknown theme: %s", themeName)
	}
	return v, nil
}

// cmdShowHelp shows available commands
func (v ListView) cmdShowHelp() (tea.Model, tea.Cmd) {
	v.statusMsg = "Commands: due, priority, tag, project, archive, done, theme, help"
	return v, nil
}

// cmdStartTime starts time tracking for current/selected task
func (v ListView) cmdStartTime() (tea.Model, tea.Cmd) {
	// Stop any active tracking first
	if v.activeTimeEntryID != "" {
		v.cmdStopTime()
	}

	taskIDs := v.getTargetTaskIDs()
	if len(taskIDs) == 0 {
		v.statusMsg = "No task selected to track time"
		return v, nil
	}

	taskID := taskIDs[0] // Only track one task at a time
	entryID := uuid.New().String()
	now := time.Now()

	v.activeTimeEntryID = entryID
	v.activeTaskID = taskID
	v.trackingStarted = now

	// Find task title for status message
	var taskTitle string
	for _, t := range v.tasks {
		if t.ID == taskID {
			taskTitle = t.Title
			break
		}
	}

	return v, func() tea.Msg {
		_, err := v.db.Exec(`
			INSERT INTO time_entries (id, task_id, started_at, is_pomodoro, created_at)
			VALUES (?, ?, ?, 0, ?)
		`, entryID, taskID, now, now)
		if err != nil {
			return taskUpdatedMsg{err: err}
		}
		return timeTrackingStartedMsg{taskID: taskID, taskTitle: taskTitle}
	}
}

// cmdStopTime stops the active time tracking
func (v ListView) cmdStopTime() (tea.Model, tea.Cmd) {
	if v.activeTimeEntryID == "" {
		v.statusMsg = "No active time tracking"
		return v, nil
	}

	entryID := v.activeTimeEntryID
	now := time.Now()
	duration := int(now.Sub(v.trackingStarted).Minutes())

	// Reset tracking state
	v.activeTimeEntryID = ""
	v.activeTaskID = ""
	v.trackingStarted = time.Time{}

	return v, func() tea.Msg {
		_, err := v.db.Exec(`
			UPDATE time_entries SET ended_at = ?, duration = ?
			WHERE id = ?
		`, now, duration, entryID)
		if err != nil {
			return taskUpdatedMsg{err: err}
		}
		return timeTrackingStoppedMsg{duration: duration}
	}
}

// cmdAddTime adds a manual time entry
func (v ListView) cmdAddTime(args []string) (tea.Model, tea.Cmd) {
	if len(args) == 0 {
		v.statusMsg = "Usage: addtime <duration> (e.g., addtime 30m, addtime 1h30m)"
		return v, nil
	}

	durationStr := args[0]
	minutes := parseTimeInput(durationStr)
	if minutes <= 0 {
		v.statusMsg = fmt.Sprintf("Invalid duration: %s (use e.g., 30m, 1h, 1h30m)", durationStr)
		return v, nil
	}

	taskIDs := v.getTargetTaskIDs()
	if len(taskIDs) == 0 {
		v.statusMsg = "No task selected"
		return v, nil
	}

	taskID := taskIDs[0]
	entryID := uuid.New().String()
	now := time.Now()

	return v, func() tea.Msg {
		_, err := v.db.Exec(`
			INSERT INTO time_entries (id, task_id, started_at, ended_at, duration, is_pomodoro, created_at)
			VALUES (?, ?, ?, ?, ?, 0, ?)
		`, entryID, taskID, now.Add(-time.Duration(minutes)*time.Minute), now, minutes, now)
		if err != nil {
			return taskUpdatedMsg{err: err}
		}
		return timeAddedMsg{minutes: minutes}
	}
}

// cmdFilter sets the text search filter
func (v ListView) cmdFilter(args []string) (tea.Model, tea.Cmd) {
	if len(args) == 0 {
		v.statusMsg = "Usage: filter <text> (e.g., filter @work)"
		return v, nil
	}
	v.searchFilter = strings.Join(args, " ")
	v.applyFilter()
	v.statusMsg = fmt.Sprintf("Filter: %s", v.searchFilter)
	return v, nil
}

// cmdFilterProject opens the project filter selector
func (v ListView) cmdFilterProject() (tea.Model, tea.Cmd) {
	v.selectingProjectFilter = true
	v.selectorCursor = 0
	return v, nil
}

// cmdFilterTag opens the tag filter selector
func (v ListView) cmdFilterTag() (tea.Model, tea.Cmd) {
	v.selectingTagFilter = true
	v.selectorCursor = 0
	return v, nil
}

// cmdClearFilters clears all active filters
func (v ListView) cmdClearFilters() (tea.Model, tea.Cmd) {
	v.searchFilter = ""
	v.filterProjectID = ""
	v.filterTagIDs = nil
	v.applyFilter()
	v.statusMsg = "Filters cleared"
	return v, nil
}

// cmdSort sorts tasks by the given field
func (v ListView) cmdSort(args []string) (tea.Model, tea.Cmd) {
	if len(args) == 0 {
		v.statusMsg = "Usage: sort <priority|due|title|status>"
		return v, nil
	}
	field := strings.ToLower(args[0])
	switch field {
	case "priority", "pri", "p":
		v.statusMsg = "Sorted by priority"
	case "due", "d":
		v.statusMsg = "Sorted by due date"
	case "title", "t":
		v.statusMsg = "Sorted by title"
	case "status", "s":
		v.statusMsg = "Sorted by status"
	default:
		v.statusMsg = fmt.Sprintf("Unknown sort field: %s", field)
	}
	return v, nil
}

// parseTimeInput parses time input like "30m", "1h", "1h30m"
func parseTimeInput(s string) int {
	s = strings.ToLower(strings.TrimSpace(s))

	var totalMinutes int
	var current string

	for _, c := range s {
		if c >= '0' && c <= '9' {
			current += string(c)
		} else if c == 'h' {
			if n := parseInt(current); n > 0 {
				totalMinutes += n * 60
			}
			current = ""
		} else if c == 'm' {
			if n := parseInt(current); n > 0 {
				totalMinutes += n
			}
			current = ""
		}
	}

	// Handle bare number (assume minutes)
	if current != "" {
		if n := parseInt(current); n > 0 {
			totalMinutes += n
		}
	}

	return totalMinutes
}

func parseInt(s string) int {
	var n int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	return n
}

// Time tracking message types
type timeTrackingStartedMsg struct {
	taskID    string
	taskTitle string
}

type timeTrackingStoppedMsg struct {
	duration int
}

type timeAddedMsg struct {
	minutes int
}

// getTargetTaskIDs returns IDs of selected tasks, or current task if none selected
func (v ListView) getTargetTaskIDs() []string {
	var ids []string
	for id := range v.selected {
		ids = append(ids, id)
	}
	if len(ids) == 0 && len(v.tasks) > 0 && v.cursor < len(v.tasks) {
		ids = append(ids, v.tasks[v.cursor].ID)
	}
	return ids
}

// Helper command functions
func (v ListView) setDueDate(taskIDs []string, dueDate time.Time) tea.Cmd {
	return func() tea.Msg {
		for _, id := range taskIDs {
			_, err := v.db.Exec(`UPDATE tasks SET due_date = ?, updated_at = ? WHERE id = ?`,
				dueDate.Format(time.RFC3339), time.Now(), id)
			if err != nil {
				return taskUpdatedMsg{err: err}
			}
		}
		return taskUpdatedMsg{}
	}
}

func (v ListView) setPriority(taskIDs []string, priority model.Priority) tea.Cmd {
	return func() tea.Msg {
		for _, id := range taskIDs {
			err := v.db.UpdateTaskPriority(id, priority)
			if err != nil {
				return taskUpdatedMsg{err: err}
			}
		}
		return taskUpdatedMsg{}
	}
}

func (v ListView) addTagToTasks(taskIDs []string, tagName string) tea.Cmd {
	return func() tea.Msg {
		now := time.Now()
		tagID := strings.ToLower(tagName)

		// Ensure tag exists
		v.db.Exec(`INSERT OR IGNORE INTO tags (id, name, created_at) VALUES (?, ?, ?)`,
			tagID, "@"+tagName, now)

		for _, taskID := range taskIDs {
			v.db.Exec(`INSERT OR IGNORE INTO task_tags (task_id, tag_id) VALUES (?, ?)`,
				taskID, tagID)
		}
		return taskUpdatedMsg{}
	}
}

func (v ListView) moveToProject(taskIDs []string, projectID string) tea.Cmd {
	return func() tea.Msg {
		for _, id := range taskIDs {
			err := v.db.UpdateTaskProject(id, projectID)
			if err != nil {
				return taskUpdatedMsg{err: err}
			}
		}
		return taskUpdatedMsg{}
	}
}

func (v ListView) archiveTasks(taskIDs []string) tea.Cmd {
	return func() tea.Msg {
		now := time.Now()
		for _, id := range taskIDs {
			_, err := v.db.Exec(`UPDATE tasks SET status = 'archived', updated_at = ? WHERE id = ?`, now, id)
			if err != nil {
				return taskUpdatedMsg{err: err}
			}
		}
		return taskUpdatedMsg{}
	}
}

// parseNaturalDate parses natural language dates
func parseNaturalDate(s string) *time.Time {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())

	s = strings.ToLower(strings.TrimSpace(s))

	switch s {
	case "today":
		return &today
	case "tomorrow", "tom":
		t := today.AddDate(0, 0, 1)
		return &t
	case "monday", "mon":
		return nextWeekday(time.Monday)
	case "tuesday", "tue":
		return nextWeekday(time.Tuesday)
	case "wednesday", "wed":
		return nextWeekday(time.Wednesday)
	case "thursday", "thu":
		return nextWeekday(time.Thursday)
	case "friday", "fri":
		return nextWeekday(time.Friday)
	case "saturday", "sat":
		return nextWeekday(time.Saturday)
	case "sunday", "sun":
		return nextWeekday(time.Sunday)
	case "next week", "nextweek":
		t := today.AddDate(0, 0, 7)
		return &t
	}

	// Try parsing as date
	formats := []string{
		"2006-01-02",
		"01/02/2006",
		"01-02-2006",
		"Jan 2",
		"Jan 2, 2006",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			if t.Year() == 0 {
				t = time.Date(now.Year(), t.Month(), t.Day(), 23, 59, 59, 0, now.Location())
			}
			return &t
		}
	}

	return nil
}

func nextWeekday(day time.Weekday) *time.Time {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())

	daysUntil := int(day - now.Weekday())
	if daysUntil <= 0 {
		daysUntil += 7
	}

	t := today.AddDate(0, 0, daysUntil)
	return &t
}

// checkDeferredResort checks if cursor moved away from a task with deferred resort
// Returns a loadTasks command if resort is needed, nil otherwise
func (v *ListView) checkDeferredResort(oldCursor int) tea.Cmd {
	if v.deferResortTaskID == "" {
		return nil
	}
	// Check if old cursor was on the deferred task
	if oldCursor >= 0 && oldCursor < len(v.tasks) {
		if v.tasks[oldCursor].ID == v.deferResortTaskID {
			// We moved away from the deferred task - trigger resort
			v.deferResortTaskID = ""
			return v.loadTasks
		}
	}
	return nil
}

// updateLocalTaskPriority updates a task's priority in local state without reload
func (v *ListView) updateLocalTaskPriority(taskID string, newPriority model.Priority) {
	// Update in allTasks
	for i := range v.allTasks {
		if v.allTasks[i].ID == taskID {
			v.allTasks[i].Priority = newPriority
			break
		}
	}
	// Update in flattened tasks
	for i := range v.tasks {
		if v.tasks[i].ID == taskID {
			v.tasks[i].Priority = newPriority
			break
		}
	}
}

// applyFilter filters tasks based on the current viewMode and searchFilter
func (v *ListView) applyFilter() {
	var filtered []model.Task
	now := time.Now()
	sevenDaysAgo := now.AddDate(0, 0, -7)

	for _, task := range v.allTasks {
		// Apply view mode filter
		switch v.viewMode {
		case ViewModeActive:
			// Hide all completed tasks
			if task.Status == model.StatusDone {
				continue
			}
		case ViewModeRecent:
			// Show active tasks + tasks completed in last 7 days
			if task.Status == model.StatusDone {
				if task.CompletedAt == nil || task.CompletedAt.Before(sevenDaysAgo) {
					continue
				}
			}
		// ViewModeAll: show everything
		}

		// Apply project filter if set
		if v.filterProjectID != "" {
			if task.ProjectID == nil || *task.ProjectID != v.filterProjectID {
				continue
			}
		}

		// Apply tag filter if set (all tags must match)
		if len(v.filterTagIDs) > 0 {
			if !v.taskHasAllTags(task, v.filterTagIDs) {
				continue
			}
		}

		// Apply search filter if set
		if v.searchFilter != "" {
			filter := strings.ToLower(v.searchFilter)
			if !v.matchesFilter(task, filter) {
				continue
			}
		}

		filtered = append(filtered, task)
	}

	v.tasks = v.flattenTasks(filtered)

	// Ensure cursor is within bounds
	if v.cursor >= len(v.tasks) {
		v.cursor = max(0, len(v.tasks)-1)
	}
}

// matchesFilter returns true if task matches the search filter
func (v ListView) matchesFilter(task model.Task, filter string) bool {
	// Match against title
	if strings.Contains(strings.ToLower(task.Title), filter) {
		return true
	}

	// Match against description
	if strings.Contains(strings.ToLower(task.Description), filter) {
		return true
	}

	// Match against project name
	if task.Project != nil && strings.Contains(strings.ToLower(task.Project.Name), filter) {
		return true
	}

	// Match against tags (with @ prefix support)
	for _, tag := range task.Tags {
		tagName := strings.ToLower(tag.Name)
		// Support both "@tag" and "tag" searches
		if strings.Contains(tagName, strings.TrimPrefix(filter, "@")) {
			return true
		}
	}

	// Match against status
	if strings.Contains(strings.ToLower(string(task.Status)), filter) {
		return true
	}

	// Match against priority (support "!high" syntax)
	priority := strings.ToLower(string(task.Priority))
	if strings.Contains(priority, strings.TrimPrefix(filter, "!")) {
		return true
	}

	// Check subtasks
	for _, subtask := range task.Subtasks {
		if v.matchesFilter(subtask, filter) {
			return true
		}
	}

	return false
}

// hasActiveFilters returns true if any filter is active
func (v ListView) hasActiveFilters() bool {
	return v.searchFilter != "" || v.filterProjectID != "" || len(v.filterTagIDs) > 0
}

// formatActiveFilters returns a formatted string of all active filters
func (v ListView) formatActiveFilters() string {
	var parts []string

	// Project filter
	if v.filterProjectID != "" {
		for _, p := range v.projects {
			if p.ID == v.filterProjectID {
				parts = append(parts, fmt.Sprintf("Project: %s", p.Name))
				break
			}
		}
	}

	// Tag filters
	if len(v.filterTagIDs) > 0 {
		var tagNames []string
		for _, tagID := range v.filterTagIDs {
			for _, tag := range v.tags {
				if tag.ID == tagID {
					tagNames = append(tagNames, tag.Name)
					break
				}
			}
		}
		if len(tagNames) > 0 {
			parts = append(parts, fmt.Sprintf("Tags: %s", strings.Join(tagNames, ", ")))
		}
	}

	// Text filter
	if v.searchFilter != "" {
		parts = append(parts, fmt.Sprintf("Text: %s", v.searchFilter))
	}

	return "Filters: " + strings.Join(parts, " | ")
}

// taskHasAllTags returns true if the task has all the specified tag IDs
func (v ListView) taskHasAllTags(task model.Task, tagIDs []string) bool {
	for _, filterTagID := range tagIDs {
		found := false
		for _, taskTag := range task.Tags {
			if taskTag.ID == filterTagID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// handleDeleteConfirm handles keypresses in delete confirmation
func (v ListView) handleDeleteConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		v.mode = ListModeNormal
		return v, v.deleteTasks(v.deleteIDs)
	case "n", "N", "esc":
		v.mode = ListModeNormal
		v.deleteIDs = nil
	}
	return v, nil
}

// handleProjectSelector handles project selection
func (v ListView) handleProjectSelector(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if v.selectorCursor > 0 {
			v.selectorCursor--
		}
	case "down", "j":
		if v.selectorCursor < len(v.projects)-1 {
			v.selectorCursor++
		}
	case "enter":
		if v.selectorCursor < len(v.projects) {
			project := v.projects[v.selectorCursor]
			v.selectingProject = false
			return v, v.moveTaskToProject(v.tasks[v.cursor].ID, project.ID)
		}
	case "esc":
		v.selectingProject = false
	}
	return v, nil
}

// handleTagSelector handles tag selection
func (v ListView) handleTagSelector(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if v.selectorCursor > 0 {
			v.selectorCursor--
		}
	case "down", "j":
		if v.selectorCursor < len(v.tags)-1 {
			v.selectorCursor++
		}
	case "enter", " ":
		if v.selectorCursor < len(v.tags) {
			tag := v.tags[v.selectorCursor]
			task := v.tasks[v.cursor]
			// Toggle tag on task
			hasTag := false
			for _, t := range task.Tags {
				if t.ID == tag.ID {
					hasTag = true
					break
				}
			}
			if hasTag {
				return v, v.removeTagFromTask(task.ID, tag.ID)
			}
			return v, v.addTagToTask(task.ID, tag.ID)
		}
	case "esc":
		v.selectingTag = false
	}
	return v, nil
}

// handleDependencySelector handles dependency selection
func (v ListView) handleDependencySelector(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if v.selectorCursor > 0 {
			v.selectorCursor--
		}
	case "down", "j":
		if v.selectorCursor < len(v.depTasks)-1 {
			v.selectorCursor++
		}
	case "enter", " ":
		if v.selectorCursor < len(v.depTasks) {
			depTask := v.depTasks[v.selectorCursor]
			// Toggle dependency
			hasDep := false
			for _, d := range v.tasks[v.cursor].Dependencies {
				if d.ID == depTask.ID {
					hasDep = true
					break
				}
			}
			if hasDep {
				return v, v.removeDependency(v.depTaskID, depTask.ID)
			}
			return v, v.addDependency(v.depTaskID, depTask.ID)
		}
	case "esc":
		v.selectingDep = false
		v.depTaskID = ""
	}
	return v, nil
}

// handleProjectFilterSelector handles project filter selection
func (v ListView) handleProjectFilterSelector(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Include "All" option at index 0
	maxIndex := len(v.projects) // projects + "All" option
	switch msg.String() {
	case "up", "k":
		if v.selectorCursor > 0 {
			v.selectorCursor--
		}
	case "down", "j":
		if v.selectorCursor < maxIndex {
			v.selectorCursor++
		}
	case "enter":
		v.selectingProjectFilter = false
		if v.selectorCursor == 0 {
			// "All" selected - clear project filter
			v.filterProjectID = ""
			v.statusMsg = "Showing all projects"
		} else if v.selectorCursor <= len(v.projects) {
			project := v.projects[v.selectorCursor-1]
			v.filterProjectID = project.ID
			v.statusMsg = fmt.Sprintf("Filtering by project: %s", project.Name)
		}
		v.applyFilter()
	case "esc":
		v.selectingProjectFilter = false
	}
	return v, nil
}

// handleTagFilterSelector handles tag filter selection
func (v ListView) handleTagFilterSelector(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Include "Clear" option at index 0
	maxIndex := len(v.tags) // tags + "Clear" option
	switch msg.String() {
	case "up", "k":
		if v.selectorCursor > 0 {
			v.selectorCursor--
		}
	case "down", "j":
		if v.selectorCursor < maxIndex {
			v.selectorCursor++
		}
	case "enter", " ":
		if v.selectorCursor == 0 {
			// "Clear" selected - remove all tag filters
			v.filterTagIDs = nil
			v.statusMsg = "Tag filter cleared"
			v.applyFilter()
		} else if v.selectorCursor <= len(v.tags) {
			tag := v.tags[v.selectorCursor-1]
			// Toggle tag in filter
			found := false
			newTags := make([]string, 0, len(v.filterTagIDs))
			for _, id := range v.filterTagIDs {
				if id == tag.ID {
					found = true
				} else {
					newTags = append(newTags, id)
				}
			}
			if found {
				v.filterTagIDs = newTags
				v.statusMsg = fmt.Sprintf("Removed %s from filter", tag.Name)
			} else {
				v.filterTagIDs = append(v.filterTagIDs, tag.ID)
				v.statusMsg = fmt.Sprintf("Added %s to filter", tag.Name)
			}
			v.applyFilter()
		}
	case "esc":
		v.selectingTagFilter = false
	}
	return v, nil
}

// View renders the list view
func (v ListView) View() string {
	debugf("ListView.View() called, len(v.tasks)=%d", len(v.tasks))
	styles := theme.Current.Styles
	t := theme.Current.Theme

	var b strings.Builder

	// Input field (if in add/edit/addsubtask mode)
	if v.mode == ListModeAdd || v.mode == ListModeAddSubtask || v.mode == ListModeEdit {
		inputStyle := styles.InputFocused
		b.WriteString(inputStyle.Render(v.input.View()))
		b.WriteString("\n\n")
	}

	// Search bar
	if v.mode == ListModeSearch {
		searchStyle := lipgloss.NewStyle().
			Foreground(t.Primary).
			Bold(true)
		b.WriteString(searchStyle.Render("/"))
		b.WriteString(v.input.View())
		b.WriteString("\n\n")
	} else if v.hasActiveFilters() {
		// Show active filter indicator
		filterStyle := lipgloss.NewStyle().
			Foreground(t.Info).
			Italic(true)
		clearHint := lipgloss.NewStyle().
			Foreground(t.Subtle)
		b.WriteString(filterStyle.Render(v.formatActiveFilters()))
		b.WriteString(clearHint.Render(" (:clear to reset)"))
		b.WriteString("\n\n")
	}

	// Command bar
	if v.mode == ListModeCommand {
		cmdStyle := lipgloss.NewStyle().
			Foreground(t.Primary).
			Bold(true)
		b.WriteString(cmdStyle.Render(":"))
		b.WriteString(v.input.View())
		b.WriteString("\n")

		// Render command suggestions
		if len(v.cmdSuggestions) > 0 {
			suggestionBox := lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(t.Border).
				Padding(0, 1).
				Width(v.width - 4)

			var suggestions []string
			maxShow := 8 // Max suggestions to show
			for i, cmd := range v.cmdSuggestions {
				if i >= maxShow {
					remaining := len(v.cmdSuggestions) - maxShow
					suggestions = append(suggestions, lipgloss.NewStyle().
						Foreground(t.Subtle).
						Render(fmt.Sprintf("  ... +%d more", remaining)))
					break
				}

				// Build suggestion line
				nameStyle := lipgloss.NewStyle().Bold(true).Width(12)
				descStyle := lipgloss.NewStyle().Foreground(t.Subtle)
				aliasStyle := lipgloss.NewStyle().Foreground(t.Info).Italic(true)

				if i == v.cmdCursor {
					nameStyle = nameStyle.Background(t.Highlight).Foreground(t.Foreground)
					descStyle = descStyle.Background(t.Highlight)
				}

				line := nameStyle.Render(cmd.Name)
				line += descStyle.Render(" " + cmd.Description)

				// Show aliases if any
				if len(cmd.Aliases) > 0 {
					aliasStr := " (" + strings.Join(cmd.Aliases, ", ") + ")"
					line += aliasStyle.Render(aliasStr)
				}

				suggestions = append(suggestions, line)
			}

			b.WriteString(suggestionBox.Render(strings.Join(suggestions, "\n")))
			b.WriteString("\n")

			// Hint for navigation
			hintStyle := lipgloss.NewStyle().Foreground(t.Subtle).Italic(true)
			b.WriteString(hintStyle.Render("↑/↓ select • tab complete • enter execute"))
		}
		b.WriteString("\n")
	}

	// Delete confirmation
	if v.mode == ListModeConfirmDelete {
		confirmStyle := lipgloss.NewStyle().
			Foreground(t.Warning).
			Bold(true)
		count := len(v.deleteIDs)
		msg := fmt.Sprintf("Delete %d task(s)? (y/n)", count)
		b.WriteString(confirmStyle.Render(msg))
		b.WriteString("\n\n")
	}

	// Project selector
	if v.selectingProject {
		b.WriteString(v.renderProjectSelector())
		b.WriteString("\n\n")
	}

	// Tag selector
	if v.selectingTag {
		b.WriteString(v.renderTagSelector())
		b.WriteString("\n\n")
	}

	// Dependency selector
	if v.selectingDep {
		b.WriteString(v.renderDependencySelector())
		b.WriteString("\n\n")
	}

	// Project filter selector
	if v.selectingProjectFilter {
		b.WriteString(v.renderProjectFilterSelector())
		b.WriteString("\n\n")
	}

	// Tag filter selector
	if v.selectingTagFilter {
		b.WriteString(v.renderTagFilterSelector())
		b.WriteString("\n\n")
	}

	// Status message
	if v.statusMsg != "" {
		statusStyle := lipgloss.NewStyle().
			Foreground(t.Info).
			Italic(true)
		b.WriteString(statusStyle.Render(v.statusMsg))
		b.WriteString("\n\n")
	}

	// Task list
	if len(v.tasks) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(t.Subtle).
			Italic(true).
			Padding(2, 0)
		if v.hasActiveFilters() {
			b.WriteString(emptyStyle.Render("No tasks match current filters. Use :clear to reset."))
		} else {
			b.WriteString(emptyStyle.Render("No tasks. Press 'a' to add one."))
		}
	} else {
		for i, task := range v.tasks {
			line := v.renderTask(task, i == v.cursor, v.selected[task.ID])
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	return b.String()
}

// renderProjectSelector renders the project selection popup
func (v ListView) renderProjectSelector() string {
	t := theme.Current.Theme

	titleStyle := lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true)

	var b strings.Builder
	b.WriteString(titleStyle.Render("Move to project:"))
	b.WriteString("\n")

	for i, project := range v.projects {
		cursor := "  "
		if i == v.selectorCursor {
			cursor = "> "
		}

		projectStyle := lipgloss.NewStyle().Foreground(t.Foreground)
		if project.Color != "" {
			projectStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(project.Color))
		}
		if i == v.selectorCursor {
			projectStyle = projectStyle.Bold(true)
		}

		b.WriteString(cursor)
		b.WriteString(projectStyle.Render(project.Name))
		b.WriteString("\n")
	}

	hintStyle := lipgloss.NewStyle().Foreground(t.Subtle).Italic(true)
	b.WriteString(hintStyle.Render("(Enter to select, Esc to cancel)"))

	return b.String()
}

// renderTagSelector renders the tag selection popup
func (v ListView) renderTagSelector() string {
	t := theme.Current.Theme

	titleStyle := lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true)

	var b strings.Builder
	b.WriteString(titleStyle.Render("Toggle tag:"))
	b.WriteString("\n")

	// Get current task's tags for checkmark display
	currentTask := v.tasks[v.cursor]
	taskTagIDs := make(map[string]bool)
	for _, tag := range currentTask.Tags {
		taskTagIDs[tag.ID] = true
	}

	for i, tag := range v.tags {
		cursor := "  "
		if i == v.selectorCursor {
			cursor = "> "
		}

		// Checkmark if task has this tag
		check := "[ ]"
		if taskTagIDs[tag.ID] {
			check = "[x]"
		}

		tagStyle := lipgloss.NewStyle().Foreground(t.Info)
		if tag.Color != "" {
			tagStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(tag.Color))
		}
		if i == v.selectorCursor {
			tagStyle = tagStyle.Bold(true)
		}

		b.WriteString(cursor)
		b.WriteString(check)
		b.WriteString(" ")
		b.WriteString(tagStyle.Render(tag.DisplayName()))
		b.WriteString("\n")
	}

	hintStyle := lipgloss.NewStyle().Foreground(t.Subtle).Italic(true)
	b.WriteString(hintStyle.Render("(Enter/Space to toggle, Esc to close)"))

	return b.String()
}

// renderDependencySelector renders the dependency selection popup
func (v ListView) renderDependencySelector() string {
	t := theme.Current.Theme

	titleStyle := lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true)

	var b strings.Builder
	b.WriteString(titleStyle.Render("Depends on (blocked by):"))
	b.WriteString("\n")

	// Get current task's dependencies for checkmark display
	currentTask := v.tasks[v.cursor]
	depTaskIDs := make(map[string]bool)
	for _, dep := range currentTask.Dependencies {
		depTaskIDs[dep.ID] = true
	}

	for i, task := range v.depTasks {
		cursor := "  "
		if i == v.selectorCursor {
			cursor = "> "
		}

		// Checkmark if this task is already a dependency
		check := "[ ]"
		if depTaskIDs[task.ID] {
			check = "[x]"
		}

		// Status indicator for the potential dependency
		statusIndicator := " "
		if task.Status == model.StatusDone {
			statusIndicator = "✓"
		}

		taskStyle := lipgloss.NewStyle().Foreground(t.Foreground)
		if task.Status == model.StatusDone {
			taskStyle = lipgloss.NewStyle().Foreground(t.Subtle)
		}
		if i == v.selectorCursor {
			taskStyle = taskStyle.Bold(true)
		}

		b.WriteString(cursor)
		b.WriteString(check)
		b.WriteString(" ")
		b.WriteString(statusIndicator)
		b.WriteString(" ")
		b.WriteString(taskStyle.Render(task.Title))
		b.WriteString("\n")
	}

	hintStyle := lipgloss.NewStyle().Foreground(t.Subtle).Italic(true)
	b.WriteString(hintStyle.Render("(Enter/Space to toggle, Esc to close)"))

	return b.String()
}

// renderProjectFilterSelector renders the project filter selection popup
func (v ListView) renderProjectFilterSelector() string {
	t := theme.Current.Theme

	titleStyle := lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true)

	var b strings.Builder
	b.WriteString(titleStyle.Render("Filter by project:"))
	b.WriteString("\n")

	// "All" option at index 0
	cursor := "  "
	if v.selectorCursor == 0 {
		cursor = "> "
	}
	allStyle := lipgloss.NewStyle().Foreground(t.Foreground)
	if v.selectorCursor == 0 {
		allStyle = allStyle.Bold(true)
	}
	checkMark := " "
	if v.filterProjectID == "" {
		checkMark = "●"
	}
	b.WriteString(fmt.Sprintf("%s%s %s\n", cursor, checkMark, allStyle.Render("All Projects")))

	// Projects
	for i, project := range v.projects {
		cursor := "  "
		if i+1 == v.selectorCursor {
			cursor = "> "
		}

		checkMark := " "
		if v.filterProjectID == project.ID {
			checkMark = "●"
		}

		projectStyle := lipgloss.NewStyle().Foreground(t.Foreground)
		if project.Color != "" {
			projectStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(project.Color))
		}
		if i+1 == v.selectorCursor {
			projectStyle = projectStyle.Bold(true)
		}

		b.WriteString(cursor)
		b.WriteString(checkMark)
		b.WriteString(" ")
		b.WriteString(projectStyle.Render(project.Name))
		b.WriteString("\n")
	}

	hintStyle := lipgloss.NewStyle().Foreground(t.Subtle).Italic(true)
	b.WriteString(hintStyle.Render("(Enter to select, Esc to cancel)"))

	return b.String()
}

// renderTagFilterSelector renders the tag filter selection popup
func (v ListView) renderTagFilterSelector() string {
	t := theme.Current.Theme

	titleStyle := lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true)

	var b strings.Builder
	b.WriteString(titleStyle.Render("Filter by tags (multi-select):"))
	b.WriteString("\n")

	// Build set of currently filtered tag IDs
	filterTagIDSet := make(map[string]bool)
	for _, id := range v.filterTagIDs {
		filterTagIDSet[id] = true
	}

	// "Clear" option at index 0
	cursor := "  "
	if v.selectorCursor == 0 {
		cursor = "> "
	}
	clearStyle := lipgloss.NewStyle().Foreground(t.Foreground)
	if v.selectorCursor == 0 {
		clearStyle = clearStyle.Bold(true)
	}
	b.WriteString(fmt.Sprintf("%s  %s\n", cursor, clearStyle.Render("Clear tag filters")))

	// Tags
	for i, tag := range v.tags {
		cursor := "  "
		if i+1 == v.selectorCursor {
			cursor = "> "
		}

		// Checkmark if tag is in filter
		check := "[ ]"
		if filterTagIDSet[tag.ID] {
			check = "[x]"
		}

		tagStyle := lipgloss.NewStyle().Foreground(t.Info)
		if tag.Color != "" {
			tagStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(tag.Color))
		}
		if i+1 == v.selectorCursor {
			tagStyle = tagStyle.Bold(true)
		}

		b.WriteString(cursor)
		b.WriteString(check)
		b.WriteString(" ")
		b.WriteString(tagStyle.Render(tag.DisplayName()))
		b.WriteString("\n")
	}

	hintStyle := lipgloss.NewStyle().Foreground(t.Subtle).Italic(true)
	b.WriteString(hintStyle.Render("(Enter/Space to toggle, Esc to close)"))

	return b.String()
}

// renderTask renders a single task line
func (v ListView) renderTask(task model.Task, isCursor, isSelected bool) string {
	t := theme.Current.Theme
	styles := theme.Current.Styles

	// Indentation for subtasks
	indent := ""
	if task.ParentID != nil {
		indent = "    " // 4 spaces for subtask indentation
	}

	// Expand/collapse indicator for parent tasks with subtasks
	expandIndicator := " "
	if task.ParentID == nil && len(task.Subtasks) > 0 {
		if v.expanded[task.ID] {
			expandIndicator = "▼"
		} else {
			expandIndicator = "▶"
		}
	} else if task.ParentID != nil {
		expandIndicator = "└"
	}

	// Checkbox
	checkbox := "[ ]"
	if task.Status == model.StatusDone {
		checkbox = "[x]"
	}

	// Selection indicator
	selectIndicator := " "
	if isSelected {
		selectIndicator = ">"
	}

	// Priority indicator
	var priorityColor lipgloss.Color
	var priorityChar string
	switch task.Priority {
	case model.PriorityUrgent:
		priorityColor = t.PriorityUrgent
		priorityChar = "‼" // Double exclamation (U+203C)
	case model.PriorityHigh:
		priorityColor = t.PriorityHigh
		priorityChar = "!"
	case model.PriorityMedium:
		priorityColor = t.PriorityMedium
		priorityChar = "-"
	case model.PriorityLow:
		priorityColor = t.PriorityLow
		priorityChar = "."
	default:
		priorityColor = t.Subtle
		priorityChar = "-"
	}
	priority := lipgloss.NewStyle().Foreground(priorityColor).Render(priorityChar)

	// Title
	title := task.Title
	titleStyle := styles.TaskNormal
	if task.Status == model.StatusDone {
		titleStyle = styles.TaskDone
	} else if task.IsOverdue() {
		titleStyle = styles.TaskOverdue
	}

	// Project name
	var projectStr string
	if task.Project != nil && !task.Project.IsInbox() {
		projectStyle := lipgloss.NewStyle().Foreground(t.Secondary)
		if task.Project.Color != "" {
			projectStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(task.Project.Color))
		}
		projectStr = projectStyle.Render("[" + task.Project.Name + "]")
	}

	// Tags (each with its own color)
	var tagsStr string
	if len(task.Tags) > 0 {
		var tagParts []string
		for _, tag := range task.Tags {
			tagStyle := lipgloss.NewStyle().Foreground(t.Info)
			if tag.Color != "" {
				tagStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(tag.Color))
			}
			tagParts = append(tagParts, tagStyle.Render(tag.DisplayName()))
		}
		tagsStr = strings.Join(tagParts, " ")
	}

	// Due date
	var dueStr string
	if task.DueDate != nil {
		dueStyle := lipgloss.NewStyle().Foreground(t.Subtle)
		if task.IsOverdue() {
			dueStyle = lipgloss.NewStyle().Foreground(t.Error)
		} else if task.IsDueToday() {
			dueStyle = lipgloss.NewStyle().Foreground(t.Warning)
		}
		dueStr = dueStyle.Render(formatDate(*task.DueDate))
	}

	// Build line
	line := fmt.Sprintf("%s%s%s %s %s %s",
		indent,
		selectIndicator,
		expandIndicator,
		checkbox,
		priority,
		titleStyle.Render(title),
	)

	if projectStr != "" {
		line += " " + projectStr
	}
	if tagsStr != "" {
		line += " " + tagsStr
	}
	if dueStr != "" {
		line += " " + dueStr
	}

	// Blocked indicator
	if v.blocked[task.ID] {
		blockedStyle := lipgloss.NewStyle().Foreground(t.Warning).Bold(true)
		line += " " + blockedStyle.Render("⊘ BLOCKED")
	} else if len(task.Dependencies) > 0 {
		// Has dependencies but they're all complete
		depStyle := lipgloss.NewStyle().Foreground(t.Subtle)
		line += " " + depStyle.Render("⦿")
	}

	// Highlight cursor line
	if isCursor {
		line = styles.TaskFocused.Render(line)
	}

	return line
}

// Database commands

type tasksLoadedMsg struct {
	tasks    []model.Task
	projects []model.Project
	tags     []model.Tag
	blocked  map[string]bool
	err      error
}

type taskCreatedMsg struct {
	task model.Task
	err  error
}

type taskUpdatedMsg struct {
	task model.Task
	err  error
}

type taskDeletedMsg struct {
	ids []string
	err error
}

// priorityChangedLocalMsg is for local priority updates (no resort until cursor moves)
type priorityChangedLocalMsg struct {
	taskID      string
	newPriority model.Priority
	err         error
}

type errorMsg struct {
	err error
}

func (v ListView) loadTasks() tea.Msg {
	debugf("=== loadTasks() START ===")
	debugf("loadTasks() called, db=%v", v.db != nil)
	defer debugf("=== loadTasks() END ===")

	// Load projects
	debugf("Loading projects...")
	projects, err := v.db.GetProjects()
	if err != nil {
		debugf("GetProjects error: %v", err)
		return tasksLoadedMsg{err: err}
	}
	debugf("Loaded %d projects", len(projects))

	// Load all tags
	debugf("Loading tags...")
	tags, err := v.db.GetTags()
	if err != nil {
		debugf("GetTags error: %v", err)
		return tasksLoadedMsg{err: err}
	}
	debugf("Loaded %d tags", len(tags))

	// Build project lookup
	projectMap := make(map[string]model.Project)
	for _, p := range projects {
		projectMap[p.ID] = p
	}

	rows, err := v.db.Query(`
		SELECT id, title, description, status, priority, urgency, importance,
		       project_id, parent_id, due_date, start_date, completed_at,
		       time_estimate, recurrence, position, gcal_event_id,
		       created_at, updated_at
		FROM tasks
		WHERE status != 'archived' AND parent_id IS NULL
		ORDER BY
			CASE status WHEN 'done' THEN 1 ELSE 0 END,
			CASE priority
				WHEN 'urgent' THEN 0
				WHEN 'high' THEN 1
				WHEN 'medium' THEN 2
				WHEN 'low' THEN 3
			END,
			position,
			created_at DESC
	`)
	debugf("Query executed, err=%v", err)
	if err != nil {
		return tasksLoadedMsg{err: err}
	}
	defer rows.Close()

	// First pass: collect basic task data (without nested queries)
	// This avoids SQLite deadlock from nested queries while rows are open
	var tasks []model.Task
	debugf("Starting rows.Next() loop")
	for rows.Next() {
		debugf("Processing row...")
		var t model.Task
		var description, projectID, parentID, dueDate, startDate, completedAt, recurrence, gcalID *string
		var timeEstimate, position *int
		var urgency, importance int

		err := rows.Scan(
			&t.ID, &t.Title, &description, &t.Status, &t.Priority,
			&urgency, &importance, &projectID, &parentID,
			&dueDate, &startDate, &completedAt, &timeEstimate,
			&recurrence, &position, &gcalID, &t.CreatedAt, &t.UpdatedAt,
		)
		if err != nil {
			return tasksLoadedMsg{err: err}
		}

		t.Urgency = urgency == 1
		t.Importance = importance == 1
		if description != nil {
			t.Description = *description
		}
		t.ProjectID = projectID
		t.ParentID = parentID
		t.TimeEstimate = timeEstimate
		t.Recurrence = recurrence
		t.GCalEventID = gcalID
		if position != nil {
			t.Position = *position
		}

		if dueDate != nil {
			if parsed, err := time.Parse(time.RFC3339, *dueDate); err == nil {
				t.DueDate = &parsed
			}
		}
		if startDate != nil {
			if parsed, err := time.Parse(time.RFC3339, *startDate); err == nil {
				t.StartDate = &parsed
			}
		}
		if completedAt != nil {
			if parsed, err := time.Parse(time.RFC3339, *completedAt); err == nil {
				t.CompletedAt = &parsed
			}
		}

		// Enrich with project (from in-memory map, no DB call)
		if t.ProjectID != nil {
			if proj, ok := projectMap[*t.ProjectID]; ok {
				t.Project = &proj
			}
		}

		tasks = append(tasks, t)
	}
	rows.Close() // Close rows before making any more DB queries
	debugf("Collected %d tasks from rows", len(tasks))

	// Second pass: enrich tasks with related data (now rows are closed)
	for i := range tasks {
		// Load tags for this task
		taskTags, _ := v.db.GetTaskTags(tasks[i].ID)
		tasks[i].Tags = taskTags

		// Load subtasks for this task
		subtasks, _ := v.db.GetSubtasks(tasks[i].ID)
		for j := range subtasks {
			// Enrich subtasks with project info
			if subtasks[j].ProjectID != nil {
				if proj, ok := projectMap[*subtasks[j].ProjectID]; ok {
					subtasks[j].Project = &proj
				}
			}
			// Load tags for subtask
			subtaskTags, _ := v.db.GetTaskTags(subtasks[j].ID)
			subtasks[j].Tags = subtaskTags
		}
		tasks[i].Subtasks = subtasks

		// Load dependencies for this task
		deps, _ := v.db.GetTaskDependencies(tasks[i].ID)
		tasks[i].Dependencies = deps
	}

	// Compute blocked status for each task
	blockedMap := make(map[string]bool)
	for _, t := range tasks {
		if len(t.Dependencies) > 0 {
			isBlocked, _ := v.db.IsTaskBlocked(t.ID)
			blockedMap[t.ID] = isBlocked
		}
	}

	debugf("loadTasks returning %d tasks, %d projects, %d tags", len(tasks), len(projects), len(tags))
	msg := tasksLoadedMsg{tasks: tasks, projects: projects, tags: tags, blocked: blockedMap}
	debugf("Created tasksLoadedMsg, returning it now")
	return msg
}

func (v ListView) createTask(title string) tea.Cmd {
	return func() tea.Msg {
		id := uuid.New().String()
		now := time.Now()

		_, err := v.db.Exec(`
			INSERT INTO tasks (id, title, status, priority, created_at, updated_at)
			VALUES (?, ?, 'pending', 'medium', ?, ?)
		`, id, title, now, now)

		if err != nil {
			return taskCreatedMsg{err: err}
		}

		return taskCreatedMsg{task: model.Task{
			ID:        id,
			Title:     title,
			Status:    model.StatusPending,
			Priority:  model.PriorityMedium,
			CreatedAt: now,
			UpdatedAt: now,
		}}
	}
}

func (v ListView) createSubtask(title, parentID string) tea.Cmd {
	return func() tea.Msg {
		id := uuid.New().String()
		now := time.Now()

		// Get parent's project_id
		var projectID *string
		v.db.QueryRow("SELECT project_id FROM tasks WHERE id = ?", parentID).Scan(&projectID)

		_, err := v.db.Exec(`
			INSERT INTO tasks (id, title, status, priority, project_id, parent_id, created_at, updated_at)
			VALUES (?, ?, 'pending', 'medium', ?, ?, ?, ?)
		`, id, title, projectID, parentID, now, now)

		if err != nil {
			return taskCreatedMsg{err: err}
		}

		return taskCreatedMsg{task: model.Task{
			ID:        id,
			Title:     title,
			Status:    model.StatusPending,
			Priority:  model.PriorityMedium,
			ParentID:  &parentID,
			ProjectID: projectID,
			CreatedAt: now,
			UpdatedAt: now,
		}}
	}
}

func (v ListView) updateTaskTitle(id, title string) tea.Cmd {
	return func() tea.Msg {
		now := time.Now()
		_, err := v.db.Exec(`
			UPDATE tasks SET title = ?, updated_at = ? WHERE id = ?
		`, title, now, id)

		if err != nil {
			return taskUpdatedMsg{err: err}
		}

		return taskUpdatedMsg{task: model.Task{ID: id, Title: title}}
	}
}

func (v ListView) toggleSelected() tea.Cmd {
	return func() tea.Msg {
		var ids []string
		if len(v.selected) > 0 {
			for id := range v.selected {
				ids = append(ids, id)
			}
		} else if len(v.tasks) > 0 {
			ids = []string{v.tasks[v.cursor].ID}
		}

		if len(ids) == 0 {
			return nil
		}

		now := time.Now()
		for _, id := range ids {
			// Find current status
			var status string
			v.db.QueryRow("SELECT status FROM tasks WHERE id = ?", id).Scan(&status)

			var newStatus string
			var completedAt interface{}
			if status == string(model.StatusDone) {
				newStatus = string(model.StatusPending)
				completedAt = nil
			} else {
				newStatus = string(model.StatusDone)
				completedAt = now
			}

			v.db.Exec(`
				UPDATE tasks SET status = ?, completed_at = ?, updated_at = ? WHERE id = ?
			`, newStatus, completedAt, now, id)
		}

		return taskUpdatedMsg{}
	}
}

func (v ListView) deleteTasks(ids []string) tea.Cmd {
	return func() tea.Msg {
		for _, id := range ids {
			_, err := v.db.Exec("DELETE FROM tasks WHERE id = ?", id)
			if err != nil {
				return taskDeletedMsg{err: err}
			}
		}
		return taskDeletedMsg{ids: ids}
	}
}

func (v ListView) cyclePriority() tea.Cmd {
	return func() tea.Msg {
		if len(v.tasks) == 0 {
			return nil
		}

		task := v.tasks[v.cursor]
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

		now := time.Now()
		_, err := v.db.Exec(`
			UPDATE tasks SET priority = ?, updated_at = ? WHERE id = ?
		`, newPriority, now, task.ID)

		if err != nil {
			return priorityChangedLocalMsg{taskID: task.ID, err: err}
		}

		// Return local msg to update UI without full reload/resort
		return priorityChangedLocalMsg{taskID: task.ID, newPriority: newPriority}
	}
}

func (v ListView) moveTaskToProject(taskID, projectID string) tea.Cmd {
	return func() tea.Msg {
		err := v.db.UpdateTaskProject(taskID, projectID)
		if err != nil {
			return taskUpdatedMsg{err: err}
		}
		return taskUpdatedMsg{task: model.Task{ID: taskID}}
	}
}

func (v ListView) addTagToTask(taskID, tagID string) tea.Cmd {
	return func() tea.Msg {
		err := v.db.AddTagToTask(taskID, tagID)
		if err != nil {
			return taskUpdatedMsg{err: err}
		}
		return taskUpdatedMsg{task: model.Task{ID: taskID}}
	}
}

func (v ListView) removeTagFromTask(taskID, tagID string) tea.Cmd {
	return func() tea.Msg {
		err := v.db.RemoveTagFromTask(taskID, tagID)
		if err != nil {
			return taskUpdatedMsg{err: err}
		}
		return taskUpdatedMsg{task: model.Task{ID: taskID}}
	}
}

func (v ListView) addDependency(taskID, dependsOnID string) tea.Cmd {
	return func() tea.Msg {
		err := v.db.AddTaskDependency(taskID, dependsOnID)
		if err != nil {
			return taskUpdatedMsg{err: err}
		}
		return taskUpdatedMsg{task: model.Task{ID: taskID}}
	}
}

func (v ListView) removeDependency(taskID, dependsOnID string) tea.Cmd {
	return func() tea.Msg {
		err := v.db.RemoveTaskDependency(taskID, dependsOnID)
		if err != nil {
			return taskUpdatedMsg{err: err}
		}
		return taskUpdatedMsg{task: model.Task{ID: taskID}}
	}
}

// Helper functions

// flattenTasks creates a flat list of tasks for display,
// including expanded subtasks indented under their parents
func (v ListView) flattenTasks(tasks []model.Task) []model.Task {
	var result []model.Task
	for _, task := range tasks {
		result = append(result, task)
		// If this task is expanded, add its subtasks
		if v.expanded[task.ID] && len(task.Subtasks) > 0 {
			for _, subtask := range task.Subtasks {
				result = append(result, subtask)
			}
		}
	}
	return result
}

func formatDate(t time.Time) string {
	now := time.Now()
	diff := t.Sub(now)

	if t.Year() == now.Year() && t.YearDay() == now.YearDay() {
		return "today"
	}

	tomorrow := now.AddDate(0, 0, 1)
	if t.Year() == tomorrow.Year() && t.YearDay() == tomorrow.YearDay() {
		return "tomorrow"
	}

	if diff < 0 {
		days := int(-diff.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}

	if diff.Hours() < 24*7 {
		return t.Format("Mon")
	}

	if t.Year() == now.Year() {
		return t.Format("Jan 2")
	}

	return t.Format("Jan 2, 2006")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
