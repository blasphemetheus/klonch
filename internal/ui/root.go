package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dori/klonch/internal/app"
	"github.com/dori/klonch/internal/ui/theme"
	"github.com/dori/klonch/internal/ui/views"
)

// Debug logging (enable by setting KLONCH_DEBUG=1)
var rootDebugLog *os.File

func init() {
	if os.Getenv("KLONCH_DEBUG") == "1" {
		rootDebugLog, _ = os.OpenFile("/tmp/klonch-root-debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	}
}

func rootDebugf(format string, args ...interface{}) {
	if rootDebugLog != nil {
		fmt.Fprintf(rootDebugLog, format+"\n", args...)
		rootDebugLog.Sync()
	}
}

// RootModel is the main application model that manages views
type RootModel struct {
	app        *app.App
	keys       KeyMap
	help       help.Model
	width      int
	height     int

	currentView     View
	listView        views.ListView
	kanbanView      views.KanbanView
	eisenhowerView  views.EisenhowerView
	calendarView    views.CalendarView
	pomodoroView    views.PomodoroView
	planningView    views.PlanningView
	reviewView      views.ReviewView
	statsView       views.StatsView
	focusView       views.FocusView
	helpVisible     bool

	// Status message
	statusMsg   string
	errorMsg    string
}

// NewRootModel creates a new root model
func NewRootModel(application *app.App) RootModel {
	h := help.New()
	h.ShowAll = false

	return RootModel{
		app:            application,
		keys:           DefaultKeyMap(),
		help:           h,
		currentView:    ViewList,
		listView:       views.NewListView(application.DB),
		kanbanView:     views.NewKanbanView(application.DB),
		eisenhowerView: views.NewEisenhowerView(application.DB),
		calendarView:   views.NewCalendarView(application.DB),
		pomodoroView:   views.NewPomodoroView(application.DB, application.Notifier),
		planningView:   views.NewPlanningView(application.DB),
		reviewView:     views.NewReviewView(application.DB),
		statsView:      views.NewStatsView(application.DB),
		focusView:      views.NewFocusView(application.DB, application.Notifier),
	}
}

// Init initializes the model
func (m RootModel) Init() tea.Cmd {
	// Initialize the current view
	cmd := m.listView.Init()
	rootDebugf("RootModel.Init() returning cmd: %v", cmd != nil)
	return cmd
}

// Update handles messages
func (m RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	rootDebugf("RootModel.Update received msg type: %T", msg)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.Width = msg.Width

		// Update child views with new size
		// Reserve space for header (2 lines) and footer (2 lines)
		contentHeight := m.height - 4
		m.listView = m.listView.SetSize(m.width, contentHeight)
		m.kanbanView = m.kanbanView.SetSize(m.width, contentHeight)
		m.eisenhowerView = m.eisenhowerView.SetSize(m.width, contentHeight)
		m.calendarView = m.calendarView.SetSize(m.width, contentHeight)
		m.pomodoroView = m.pomodoroView.SetSize(m.width, contentHeight)
		m.planningView = m.planningView.SetSize(m.width, contentHeight)
		m.reviewView = m.reviewView.SetSize(m.width, contentHeight)
		m.statsView = m.statsView.SetSize(m.width, contentHeight)
		m.focusView = m.focusView.SetSize(m.width, contentHeight)

	case tea.KeyMsg:
		// Clear status/error on any keypress
		m.statusMsg = ""
		m.errorMsg = ""

		// Check if current view is in input mode
		isInputMode := false
		switch m.currentView {
		case ViewList:
			isInputMode = m.listView.IsInputMode()
		case ViewKanban:
			isInputMode = m.kanbanView.IsInputMode()
		case ViewEisenhower:
			isInputMode = m.eisenhowerView.IsInputMode()
		case ViewCalendar:
			isInputMode = m.calendarView.IsInputMode()
		case ViewPomodoro:
			isInputMode = m.pomodoroView.IsInputMode()
		case ViewPlanning:
			isInputMode = m.planningView.IsInputMode()
		case ViewReview:
			isInputMode = m.reviewView.IsInputMode()
		case ViewStats:
			isInputMode = m.statsView.IsInputMode()
		case ViewFocus:
			isInputMode = m.focusView.IsInputMode()
		}

		// Global keybindings
		switch {
		case key.Matches(msg, m.keys.Quit):
			// ctrl+c always quits, but 'q' only quits when not in input mode
			if msg.String() == "ctrl+c" || !isInputMode {
				return m, tea.Quit
			}
			// Otherwise, let the view handle 'q' as a character

		case key.Matches(msg, m.keys.ThemeCycle):
			// ctrl+t always works (unlikely to type)
			m.cycleTheme()
			return m, nil
		}

		// Skip other global keys when in input mode
		if isInputMode {
			break // Fall through to view delegation
		}

		// These only work when NOT in input mode
		switch {
		case key.Matches(msg, m.keys.Help):
			m.helpVisible = !m.helpVisible
			m.help.ShowAll = m.helpVisible
			return m, nil

		// View switching (1-8 keys)
		case key.Matches(msg, m.keys.ListView):
			m.currentView = ViewList
			return m, m.listView.Init() // Reload tasks when switching to list
		case key.Matches(msg, m.keys.KanbanView):
			m.currentView = ViewKanban
			return m, m.kanbanView.Init()
		case key.Matches(msg, m.keys.EisenhowerView):
			m.currentView = ViewEisenhower
			return m, m.eisenhowerView.Init()
		case key.Matches(msg, m.keys.CalendarView):
			m.currentView = ViewCalendar
			return m, m.calendarView.Init()
		case key.Matches(msg, m.keys.PomodoroView):
			m.currentView = ViewPomodoro
			return m, m.pomodoroView.Init()
		case key.Matches(msg, m.keys.PlanningView):
			m.currentView = ViewPlanning
			return m, m.planningView.Init()
		case key.Matches(msg, m.keys.ReviewView):
			m.currentView = ViewReview
			return m, m.reviewView.Init()
		case key.Matches(msg, m.keys.StatsView):
			m.currentView = ViewStats
			return m, m.statsView.Init()
		}

	case ErrorMsg:
		m.errorMsg = msg.Err.Error()
		return m, nil

	case StatusMsg:
		m.statusMsg = msg.Message
		return m, nil

	case ThemeChangedMsg:
		m.statusMsg = fmt.Sprintf("Theme: %s", msg.ThemeName)
		return m, nil

	case FocusTaskMsg:
		m.focusView = m.focusView.SetTask(&msg.Task)
		m.currentView = ViewFocus
		return m, m.focusView.Init()

	case views.FocusTaskRequest:
		// Handle focus request from list view
		m.focusView = m.focusView.SetTask(&msg.Task)
		m.currentView = ViewFocus
		return m, m.focusView.Init()
	}

	// Delegate to current view
	rootDebugf("Delegating to view: %v", m.currentView)
	switch m.currentView {
	case ViewList:
		rootDebugf("Before listView.Update, msg type: %T", msg)
		newListView, cmd := m.listView.Update(msg)
		m.listView = newListView.(views.ListView)
		rootDebugf("After listView.Update, got cmd: %v", cmd != nil)
		cmds = append(cmds, cmd)
	case ViewKanban:
		newKanbanView, cmd := m.kanbanView.Update(msg)
		m.kanbanView = newKanbanView.(views.KanbanView)
		cmds = append(cmds, cmd)
	case ViewEisenhower:
		newEisenhowerView, cmd := m.eisenhowerView.Update(msg)
		m.eisenhowerView = newEisenhowerView.(views.EisenhowerView)
		cmds = append(cmds, cmd)
	case ViewCalendar:
		newCalendarView, cmd := m.calendarView.Update(msg)
		m.calendarView = newCalendarView.(views.CalendarView)
		cmds = append(cmds, cmd)
	case ViewPomodoro:
		newPomodoroView, cmd := m.pomodoroView.Update(msg)
		m.pomodoroView = newPomodoroView.(views.PomodoroView)
		cmds = append(cmds, cmd)
	case ViewPlanning:
		newPlanningView, cmd := m.planningView.Update(msg)
		m.planningView = newPlanningView.(views.PlanningView)
		cmds = append(cmds, cmd)
	case ViewReview:
		newReviewView, cmd := m.reviewView.Update(msg)
		m.reviewView = newReviewView.(views.ReviewView)
		cmds = append(cmds, cmd)
	case ViewStats:
		newStatsView, cmd := m.statsView.Update(msg)
		m.statsView = newStatsView.(views.StatsView)
		cmds = append(cmds, cmd)
	case ViewFocus:
		newFocusView, cmd := m.focusView.Update(msg)
		m.focusView = newFocusView.(views.FocusView)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// View renders the UI
func (m RootModel) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	styles := theme.Current.Styles
	var sections []string

	// Header
	header := m.renderHeader()
	sections = append(sections, header)

	// Content area
	// Reserve: 1 line for header + 3 lines for footer (status + 2 hint lines)
	contentHeight := m.height - 4
	if m.errorMsg != "" || m.statusMsg != "" {
		contentHeight-- // Extra line for status message
	}
	var content string

	if m.helpVisible {
		content = m.renderHelp(contentHeight)
	} else {
		switch m.currentView {
		case ViewList:
			content = m.listView.View()
		case ViewKanban:
			content = m.kanbanView.View()
		case ViewEisenhower:
			content = m.eisenhowerView.View()
		case ViewCalendar:
			content = m.calendarView.View()
		case ViewPomodoro:
			content = m.pomodoroView.View()
		case ViewPlanning:
			content = m.planningView.View()
		case ViewReview:
			content = m.reviewView.View()
		case ViewStats:
			content = m.statsView.View()
		case ViewFocus:
			content = m.focusView.View()
		default:
			content = styles.Panel.Render("View not implemented")
		}
	}

	// Ensure content fills available space
	contentLines := strings.Count(content, "\n") + 1
	if contentLines < contentHeight {
		content += strings.Repeat("\n", contentHeight-contentLines)
	}
	sections = append(sections, content)

	// Footer
	footer := m.renderFooter()
	sections = append(sections, footer)

	return strings.Join(sections, "\n")
}

// renderHeader renders the header bar
func (m RootModel) renderHeader() string {
	styles := theme.Current.Styles
	t := theme.Current.Theme

	title := styles.Header.Render("klonch")

	// View indicator
	viewStyle := lipgloss.NewStyle().
		Foreground(t.Subtle).
		Padding(0, 1)
	viewIndicator := viewStyle.Render(fmt.Sprintf("[%s]", m.currentView.String()))

	// Theme indicator
	themeIndicator := viewStyle.Render(fmt.Sprintf("theme: %s", t.Name))

	// Combine header elements
	leftSide := lipgloss.JoinHorizontal(lipgloss.Center, title, viewIndicator)
	rightSide := themeIndicator

	gap := m.width - lipgloss.Width(leftSide) - lipgloss.Width(rightSide)
	if gap < 0 {
		gap = 0
	}

	header := leftSide + strings.Repeat(" ", gap) + rightSide
	return header
}

// renderFooter renders the footer/status bar
func (m RootModel) renderFooter() string {
	styles := theme.Current.Styles
	t := theme.Current.Theme

	// Helper to format key hints
	key := func(k, desc string) string {
		return styles.HelpKey.Render(k) + styles.HelpDesc.Render(" "+desc)
	}
	sep := styles.HelpSeparator.Render(" │ ")

	// Show error or status message on first line if present
	var statusLine string
	if m.errorMsg != "" {
		statusLine = lipgloss.NewStyle().Foreground(t.Error).Render(m.errorMsg)
	} else if m.statusMsg != "" {
		statusLine = lipgloss.NewStyle().Foreground(t.Info).Render(m.statusMsg)
	}

	// Build context-aware hint lines
	var line1, line2 string

	switch m.currentView {
	case ViewList:
		// Check if list view is in a special mode
		if m.listView.IsInputMode() {
			line1 = key("enter", "confirm") + sep + key("esc", "cancel")
			line2 = ""
		} else {
			// Primary actions
			line1 = key("a", "add") + sep +
				key("enter", "edit") + sep +
				key("tab", "done") + sep +
				key("d", "del") + sep +
				key("space", "select") + sep +
				key("/", "search") + sep +
				key(":", "cmd")
			// Secondary actions
			line2 = key("p", "priority") + sep +
				key("m", "move") + sep +
				key("t", "tag") + sep +
				key("f", "focus") + sep +
				key("1-8", "views") + sep +
				key("?", "help")
		}

	case ViewKanban:
		line1 = key("h/l", "columns") + sep +
			key("j/k", "navigate") + sep +
			key("H/L", "move task") + sep +
			key("enter", "toggle done")
		line2 = key("1-4", "views") + sep +
			key("ctrl+t", "theme") + sep +
			key("?", "help")

	case ViewEisenhower:
		line1 = key("h/j/k/l", "navigate") + sep +
			key("1-4", "set quadrant") + sep +
			key("enter", "complete")
		line2 = key("1-4", "views") + sep +
			key("ctrl+t", "theme") + sep +
			key("?", "help")

	case ViewCalendar:
		line1 = key("h/j/k/l", "days") + sep +
			key("H/L", "months") + sep +
			key("t", "today")
		line2 = key("1-4", "views") + sep +
			key("ctrl+t", "theme") + sep +
			key("?", "help")

	case ViewPomodoro:
		line1 = key("s/space", "start/pause") + sep +
			key("r", "reset") + sep +
			key("b", "short break") + sep +
			key("B", "long break")
		line2 = key("j/k", "select task") + sep +
			key("enter", "pick task") + sep +
			key("1-8", "views") + sep +
			key("?", "help")

	case ViewPlanning:
		line1 = key("t/enter", "today") + sep +
			key("T", "tomorrow") + sep +
			key("x", "clear date") + sep +
			key("d", "done")
		line2 = key("tab", "section") + sep +
			key("space", "select") + sep +
			key("j/k", "navigate") + sep +
			key("1-8", "views")

	case ViewReview:
		line1 = key("n", "next week") + sep +
			key("t", "today") + sep +
			key("a", "archive") + sep +
			key("u", "uncheck") + sep +
			key("d", "delete")
		line2 = key("tab", "section") + sep +
			key("space", "select") + sep +
			key("j/k", "navigate") + sep +
			key("1-8", "views")

	case ViewStats:
		line1 = key("w", "week") + sep +
			key("m", "month") + sep +
			key("y", "year") + sep +
			key("r", "refresh")
		line2 = key("1-8", "views") + sep +
			key("ctrl+t", "theme") + sep +
			key("?", "help")

	case ViewFocus:
		if m.focusView.IsTimerRunning() {
			line1 = key("space", "pause") + sep +
				key("s", "stop+save") + sep +
				key("x", "discard")
		} else {
			line1 = key("space", "start") + sep +
				key("s", "save time") + sep +
				key("r", "reset")
		}
		line2 = key("tab", "done") + sep +
			key("j/k", "subtasks") + sep +
			key("esc", "back") + sep +
			key("1-8", "views")

	default:
		line1 = key("1-5", "views") + sep + key("?", "help")
	}

	// Build footer
	var lines []string

	// Status/error line (if present)
	if statusLine != "" {
		lines = append(lines, statusLine)
	}

	// Hint lines
	if line1 != "" {
		lines = append(lines, line1)
	}
	if line2 != "" {
		lines = append(lines, line2)
	}

	return strings.Join(lines, "\n")
}

// renderHelp renders the help overlay
func (m RootModel) renderHelp(height int) string {
	t := theme.Current.Theme

	// Styles
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Primary).
		MarginBottom(1)

	sectionStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Secondary).
		MarginTop(1)

	keyStyle := lipgloss.NewStyle().
		Foreground(t.Foreground).
		Bold(true).
		Width(12)

	descStyle := lipgloss.NewStyle().
		Foreground(t.Subtle)

	cmdKeyStyle := lipgloss.NewStyle().
		Foreground(t.Info).
		Bold(true).
		Width(20)

	var b strings.Builder

	// Title
	b.WriteString(titleStyle.Render("Klonch Help"))
	b.WriteString("\n\n")

	// Navigation section
	b.WriteString(sectionStyle.Render("Navigation"))
	b.WriteString("\n")
	navKeys := [][]string{
		{"↑/k ↓/j", "Navigate up/down"},
		{"g / G", "Go to top/bottom"},
		{"PgUp/PgDn", "Page up/down"},
	}
	for _, kv := range navKeys {
		b.WriteString(keyStyle.Render(kv[0]))
		b.WriteString(descStyle.Render(kv[1]))
		b.WriteString("\n")
	}

	// Selection section
	b.WriteString(sectionStyle.Render("Selection"))
	b.WriteString("\n")
	selKeys := [][]string{
		{"space", "Toggle selection"},
		{"v", "Enter multi-select mode"},
		{"V", "Select all visible"},
		{"esc", "Clear selection"},
	}
	for _, kv := range selKeys {
		b.WriteString(keyStyle.Render(kv[0]))
		b.WriteString(descStyle.Render(kv[1]))
		b.WriteString("\n")
	}

	// Task Actions section
	b.WriteString(sectionStyle.Render("Task Actions"))
	b.WriteString("\n")
	actionKeys := [][]string{
		{"a", "Add new task"},
		{"enter", "Edit task"},
		{"tab", "Toggle done/pending"},
		{"d", "Delete task(s)"},
		{"p", "Cycle priority"},
		{"m", "Move to project"},
		{"t", "Add/remove tags"},
	}
	for _, kv := range actionKeys {
		b.WriteString(keyStyle.Render(kv[0]))
		b.WriteString(descStyle.Render(kv[1]))
		b.WriteString("\n")
	}

	// Views section
	b.WriteString(sectionStyle.Render("Views"))
	b.WriteString("\n")
	viewKeys := [][]string{
		{"1-8", "Switch views (list, kanban, eisenhower...)"},
		{"/", "Search/filter tasks"},
		{":", "Command palette"},
		{"?", "Toggle this help"},
	}
	for _, kv := range viewKeys {
		b.WriteString(keyStyle.Render(kv[0]))
		b.WriteString(descStyle.Render(kv[1]))
		b.WriteString("\n")
	}

	// System section
	b.WriteString(sectionStyle.Render("System"))
	b.WriteString("\n")
	sysKeys := [][]string{
		{"ctrl+t", "Cycle theme"},
		{"q / ctrl+c", "Quit"},
	}
	for _, kv := range sysKeys {
		b.WriteString(keyStyle.Render(kv[0]))
		b.WriteString(descStyle.Render(kv[1]))
		b.WriteString("\n")
	}

	// Command palette section
	b.WriteString("\n")
	b.WriteString(titleStyle.Render("Command Palette (:)"))
	b.WriteString("\n")
	commands := [][]string{
		{":due <date>", "Set due date (tomorrow, friday, 2024-01-15)"},
		{":priority <p>", "Set priority (low, medium, high, urgent)"},
		{":tag <name>", "Add tag to task(s)"},
		{":project <name>", "Move to project"},
		{":done", "Toggle done status"},
		{":archive", "Archive task(s)"},
		{":theme <name>", "Change theme (nord, dracula, gruvbox...)"},
	}
	for _, kv := range commands {
		b.WriteString(cmdKeyStyle.Render(kv[0]))
		b.WriteString(descStyle.Render(kv[1]))
		b.WriteString("\n")
	}

	// Footer
	b.WriteString("\n")
	b.WriteString(descStyle.Render("Press ? or esc to close"))

	return b.String()
}

// cycleTheme cycles through available themes
func (m *RootModel) cycleTheme() {
	themes := theme.Available()
	current := theme.Current.Theme.Name

	for i, t := range themes {
		if t.Name == current {
			next := themes[(i+1)%len(themes)]
			theme.SetTheme(next)
			m.statusMsg = fmt.Sprintf("Theme: %s", next.Name)
			return
		}
	}
}
