package theme

import "github.com/charmbracelet/lipgloss"

// Nord theme - Arctic, north-bluish color palette
// https://www.nordtheme.com/
var Nord = Theme{
	Name: "nord",

	// Polar Night (dark backgrounds)
	Background: lipgloss.Color("#2E3440"),
	Foreground: lipgloss.Color("#ECEFF4"),
	Subtle:     lipgloss.Color("#4C566A"),
	Highlight:  lipgloss.Color("#3B4252"),
	Border:     lipgloss.Color("#4C566A"),

	// Frost (primary blues)
	Primary:   lipgloss.Color("#88C0D0"), // Nord8 - bright cyan
	Secondary: lipgloss.Color("#81A1C1"), // Nord9 - desaturated blue
	Info:      lipgloss.Color("#5E81AC"), // Nord10 - dark blue

	// Aurora (accent colors)
	Success: lipgloss.Color("#A3BE8C"), // Nord14 - green
	Warning: lipgloss.Color("#EBCB8B"), // Nord13 - yellow
	Error:   lipgloss.Color("#BF616A"), // Nord11 - red

	// Priority colors
	PriorityLow:    lipgloss.Color("#A3BE8C"), // Green
	PriorityMedium: lipgloss.Color("#EBCB8B"), // Yellow
	PriorityHigh:   lipgloss.Color("#D08770"), // Orange
	PriorityUrgent: lipgloss.Color("#BF616A"), // Red

	// Eisenhower quadrants
	QuadrantDoFirst:   lipgloss.Color("#BF616A"), // Red - urgent + important
	QuadrantDelegate:  lipgloss.Color("#D08770"), // Orange - urgent + not important
	QuadrantSchedule:  lipgloss.Color("#5E81AC"), // Blue - not urgent + important
	QuadrantEliminate: lipgloss.Color("#4C566A"), // Gray - not urgent + not important

	// Status colors
	StatusPending:    lipgloss.Color("#EBCB8B"), // Yellow
	StatusInProgress: lipgloss.Color("#88C0D0"), // Cyan
	StatusDone:       lipgloss.Color("#A3BE8C"), // Green
	StatusArchived:   lipgloss.Color("#4C566A"), // Gray
}
