package theme

import "github.com/charmbracelet/lipgloss"

// Dracula theme - Dark theme with vibrant colors
// https://draculatheme.com/
var Dracula = Theme{
	Name: "dracula",

	// Background colors
	Background: lipgloss.Color("#282A36"),
	Foreground: lipgloss.Color("#F8F8F2"),
	Subtle:     lipgloss.Color("#6272A4"),
	Highlight:  lipgloss.Color("#44475A"),
	Border:     lipgloss.Color("#6272A4"),

	// Primary colors
	Primary:   lipgloss.Color("#BD93F9"), // Purple
	Secondary: lipgloss.Color("#8BE9FD"), // Cyan
	Info:      lipgloss.Color("#8BE9FD"), // Cyan

	// Semantic colors
	Success: lipgloss.Color("#50FA7B"), // Green
	Warning: lipgloss.Color("#F1FA8C"), // Yellow
	Error:   lipgloss.Color("#FF5555"), // Red

	// Priority colors
	PriorityLow:    lipgloss.Color("#50FA7B"), // Green
	PriorityMedium: lipgloss.Color("#F1FA8C"), // Yellow
	PriorityHigh:   lipgloss.Color("#FFB86C"), // Orange
	PriorityUrgent: lipgloss.Color("#FF5555"), // Red

	// Eisenhower quadrants
	QuadrantDoFirst:   lipgloss.Color("#FF5555"), // Red
	QuadrantDelegate:  lipgloss.Color("#FFB86C"), // Orange
	QuadrantSchedule:  lipgloss.Color("#BD93F9"), // Purple
	QuadrantEliminate: lipgloss.Color("#6272A4"), // Comment gray

	// Status colors
	StatusPending:    lipgloss.Color("#F1FA8C"), // Yellow
	StatusInProgress: lipgloss.Color("#8BE9FD"), // Cyan
	StatusDone:       lipgloss.Color("#50FA7B"), // Green
	StatusArchived:   lipgloss.Color("#6272A4"), // Gray
}
