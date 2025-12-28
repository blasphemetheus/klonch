package theme

import "github.com/charmbracelet/lipgloss"

// Catppuccin theme - Soothing pastel theme (Mocha variant)
// https://github.com/catppuccin/catppuccin
var Catppuccin = Theme{
	Name: "catppuccin",

	// Background colors (Mocha)
	Background: lipgloss.Color("#1E1E2E"),
	Foreground: lipgloss.Color("#CDD6F4"),
	Subtle:     lipgloss.Color("#6C7086"),
	Highlight:  lipgloss.Color("#313244"),
	Border:     lipgloss.Color("#45475A"),

	// Primary colors
	Primary:   lipgloss.Color("#89B4FA"), // Blue
	Secondary: lipgloss.Color("#CBA6F7"), // Mauve
	Info:      lipgloss.Color("#74C7EC"), // Sapphire

	// Semantic colors
	Success: lipgloss.Color("#A6E3A1"), // Green
	Warning: lipgloss.Color("#F9E2AF"), // Yellow
	Error:   lipgloss.Color("#F38BA8"), // Red

	// Priority colors
	PriorityLow:    lipgloss.Color("#A6E3A1"), // Green
	PriorityMedium: lipgloss.Color("#F9E2AF"), // Yellow
	PriorityHigh:   lipgloss.Color("#FAB387"), // Peach
	PriorityUrgent: lipgloss.Color("#F38BA8"), // Red

	// Eisenhower quadrants
	QuadrantDoFirst:   lipgloss.Color("#F38BA8"), // Red
	QuadrantDelegate:  lipgloss.Color("#FAB387"), // Peach
	QuadrantSchedule:  lipgloss.Color("#89B4FA"), // Blue
	QuadrantEliminate: lipgloss.Color("#6C7086"), // Overlay0

	// Status colors
	StatusPending:    lipgloss.Color("#F9E2AF"), // Yellow
	StatusInProgress: lipgloss.Color("#89B4FA"), // Blue
	StatusDone:       lipgloss.Color("#A6E3A1"), // Green
	StatusArchived:   lipgloss.Color("#6C7086"), // Overlay0
}
