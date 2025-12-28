package theme

import "github.com/charmbracelet/lipgloss"

// Gruvbox theme - Retro groove color scheme
// https://github.com/morhetz/gruvbox
var Gruvbox = Theme{
	Name: "gruvbox",

	// Background colors (dark mode)
	Background: lipgloss.Color("#282828"),
	Foreground: lipgloss.Color("#EBDBB2"),
	Subtle:     lipgloss.Color("#928374"),
	Highlight:  lipgloss.Color("#3C3836"),
	Border:     lipgloss.Color("#504945"),

	// Primary colors
	Primary:   lipgloss.Color("#83A598"), // Aqua
	Secondary: lipgloss.Color("#8EC07C"), // Green
	Info:      lipgloss.Color("#83A598"), // Aqua

	// Semantic colors
	Success: lipgloss.Color("#B8BB26"), // Green
	Warning: lipgloss.Color("#FABD2F"), // Yellow
	Error:   lipgloss.Color("#FB4934"), // Red

	// Priority colors
	PriorityLow:    lipgloss.Color("#B8BB26"), // Green
	PriorityMedium: lipgloss.Color("#FABD2F"), // Yellow
	PriorityHigh:   lipgloss.Color("#FE8019"), // Orange
	PriorityUrgent: lipgloss.Color("#FB4934"), // Red

	// Eisenhower quadrants
	QuadrantDoFirst:   lipgloss.Color("#FB4934"), // Red
	QuadrantDelegate:  lipgloss.Color("#FE8019"), // Orange
	QuadrantSchedule:  lipgloss.Color("#83A598"), // Aqua
	QuadrantEliminate: lipgloss.Color("#928374"), // Gray

	// Status colors
	StatusPending:    lipgloss.Color("#FABD2F"), // Yellow
	StatusInProgress: lipgloss.Color("#83A598"), // Aqua
	StatusDone:       lipgloss.Color("#B8BB26"), // Green
	StatusArchived:   lipgloss.Color("#928374"), // Gray
}
