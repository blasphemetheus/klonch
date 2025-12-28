package ui

import (
	"github.com/charmbracelet/bubbles/key"
)

// KeyMap defines all keybindings for the application
type KeyMap struct {
	// Navigation
	Up       key.Binding
	Down     key.Binding
	Top      key.Binding
	Bottom   key.Binding
	PageUp   key.Binding
	PageDown key.Binding

	// Selection (Helix-style)
	Select      key.Binding
	MultiSelect key.Binding
	SelectAll   key.Binding
	ClearSelect key.Binding

	// Task Actions
	Add      key.Binding
	Edit     key.Binding
	Delete   key.Binding
	Toggle   key.Binding
	Move     key.Binding
	Tag      key.Binding
	Priority key.Binding
	Eisenhower key.Binding
	Schedule key.Binding
	Recur    key.Binding
	Undo     key.Binding

	// Views
	ListView       key.Binding
	KanbanView     key.Binding
	EisenhowerView key.Binding
	CalendarView   key.Binding
	PomodoroView   key.Binding
	PlanningView   key.Binding
	ReviewView     key.Binding
	StatsView      key.Binding

	// Power User
	Search       key.Binding
	Command      key.Binding
	Help         key.Binding
	Focus        key.Binding
	Sync         key.Binding
	Export       key.Binding
	ThemeCycle   key.Binding

	// General
	Quit   key.Binding
	Back   key.Binding
	Confirm key.Binding
	Cancel  key.Binding
}

// DefaultKeyMap returns the default keybindings
func DefaultKeyMap() KeyMap {
	return KeyMap{
		// Navigation
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Top: key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("g", "top"),
		),
		Bottom: key.NewBinding(
			key.WithKeys("G"),
			key.WithHelp("G", "bottom"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup", "ctrl+u"),
			key.WithHelp("PgUp", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown", "ctrl+d"),
			key.WithHelp("PgDn", "page down"),
		),

		// Selection (Helix-style)
		Select: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("space", "select"),
		),
		MultiSelect: key.NewBinding(
			key.WithKeys("v"),
			key.WithHelp("v", "multi-select"),
		),
		SelectAll: key.NewBinding(
			key.WithKeys("V"),
			key.WithHelp("V", "select all"),
		),
		ClearSelect: key.NewBinding(
			key.WithKeys("escape"),
			key.WithHelp("esc", "clear"),
		),

		// Task Actions
		Add: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "add"),
		),
		Edit: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "edit"),
		),
		Delete: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "delete"),
		),
		Toggle: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "toggle done"),
		),
		Move: key.NewBinding(
			key.WithKeys("m"),
			key.WithHelp("m", "move"),
		),
		Tag: key.NewBinding(
			key.WithKeys("t"),
			key.WithHelp("t", "tag"),
		),
		Priority: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "priority"),
		),
		Eisenhower: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "eisenhower"),
		),
		Schedule: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "schedule"),
		),
		Recur: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "recurrence"),
		),
		Undo: key.NewBinding(
			key.WithKeys("u"),
			key.WithHelp("u", "undo"),
		),

		// Views
		ListView: key.NewBinding(
			key.WithKeys("1"),
			key.WithHelp("1", "list"),
		),
		KanbanView: key.NewBinding(
			key.WithKeys("2"),
			key.WithHelp("2", "kanban"),
		),
		EisenhowerView: key.NewBinding(
			key.WithKeys("3"),
			key.WithHelp("3", "eisenhower"),
		),
		CalendarView: key.NewBinding(
			key.WithKeys("4"),
			key.WithHelp("4", "calendar"),
		),
		PomodoroView: key.NewBinding(
			key.WithKeys("5"),
			key.WithHelp("5", "pomodoro"),
		),
		PlanningView: key.NewBinding(
			key.WithKeys("6"),
			key.WithHelp("6", "planning"),
		),
		ReviewView: key.NewBinding(
			key.WithKeys("7"),
			key.WithHelp("7", "review"),
		),
		StatsView: key.NewBinding(
			key.WithKeys("8"),
			key.WithHelp("8", "stats"),
		),

		// Power User
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search"),
		),
		Command: key.NewBinding(
			key.WithKeys(":"),
			key.WithHelp(":", "command"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Focus: key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "focus"),
		),
		Sync: key.NewBinding(
			key.WithKeys("ctrl+s"),
			key.WithHelp("C-s", "sync"),
		),
		Export: key.NewBinding(
			key.WithKeys("ctrl+e"),
			key.WithHelp("C-e", "export"),
		),
		ThemeCycle: key.NewBinding(
			key.WithKeys("ctrl+t"),
			key.WithHelp("C-t", "theme"),
		),

		// General
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Back: key.NewBinding(
			key.WithKeys("escape"),
			key.WithHelp("esc", "back"),
		),
		Confirm: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "confirm"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("escape"),
			key.WithHelp("esc", "cancel"),
		),
	}
}

// ShortHelp returns short help bindings (for status bar)
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit}
}

// FullHelp returns full help bindings (for help view)
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Top, k.Bottom},
		{k.Select, k.MultiSelect, k.SelectAll},
		{k.Add, k.Edit, k.Delete, k.Toggle},
		{k.Move, k.Tag, k.Priority, k.Schedule},
		{k.ListView, k.KanbanView, k.EisenhowerView, k.CalendarView},
		{k.PomodoroView, k.PlanningView, k.ReviewView, k.StatsView},
		{k.Search, k.Command, k.Focus, k.Undo},
		{k.Help, k.Quit},
	}
}
