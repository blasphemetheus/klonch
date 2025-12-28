package theme

import (
	"github.com/charmbracelet/lipgloss"
)

// Theme defines the color scheme and styles for the UI
type Theme struct {
	Name string

	// Base colors
	Background    lipgloss.Color
	Foreground    lipgloss.Color
	Subtle        lipgloss.Color
	Highlight     lipgloss.Color
	Border        lipgloss.Color

	// Semantic colors
	Primary       lipgloss.Color
	Secondary     lipgloss.Color
	Success       lipgloss.Color
	Warning       lipgloss.Color
	Error         lipgloss.Color
	Info          lipgloss.Color

	// Priority colors
	PriorityLow    lipgloss.Color
	PriorityMedium lipgloss.Color
	PriorityHigh   lipgloss.Color
	PriorityUrgent lipgloss.Color

	// Eisenhower quadrant colors
	QuadrantDoFirst  lipgloss.Color
	QuadrantDelegate lipgloss.Color
	QuadrantSchedule lipgloss.Color
	QuadrantEliminate lipgloss.Color

	// Status colors
	StatusPending    lipgloss.Color
	StatusInProgress lipgloss.Color
	StatusDone       lipgloss.Color
	StatusArchived   lipgloss.Color
}

// Styles holds pre-computed lipgloss styles based on theme
type Styles struct {
	// Base styles
	App            lipgloss.Style
	Header         lipgloss.Style
	Footer         lipgloss.Style

	// Task styles
	TaskNormal     lipgloss.Style
	TaskSelected   lipgloss.Style
	TaskFocused    lipgloss.Style
	TaskDone       lipgloss.Style
	TaskOverdue    lipgloss.Style

	// Component styles
	Title          lipgloss.Style
	Subtitle       lipgloss.Style
	Label          lipgloss.Style
	Tag            lipgloss.Style
	DueDate        lipgloss.Style
	Priority       lipgloss.Style

	// Input styles
	Input          lipgloss.Style
	InputFocused   lipgloss.Style
	Placeholder    lipgloss.Style

	// Panel styles
	Panel          lipgloss.Style
	PanelTitle     lipgloss.Style
	PanelBorder    lipgloss.Style

	// Help styles
	HelpKey        lipgloss.Style
	HelpDesc       lipgloss.Style
	HelpSeparator  lipgloss.Style

	// Status bar
	StatusBar      lipgloss.Style
	StatusKey      lipgloss.Style
	StatusValue    lipgloss.Style
}

// NewStyles creates styles from a theme
func NewStyles(t Theme) Styles {
	return Styles{
		// Base styles
		App: lipgloss.NewStyle().
			Background(t.Background).
			Foreground(t.Foreground),

		Header: lipgloss.NewStyle().
			Foreground(t.Primary).
			Bold(true).
			Padding(0, 1),

		Footer: lipgloss.NewStyle().
			Foreground(t.Subtle).
			Padding(0, 1),

		// Task styles
		TaskNormal: lipgloss.NewStyle().
			Foreground(t.Foreground).
			Padding(0, 1),

		TaskSelected: lipgloss.NewStyle().
			Foreground(t.Foreground).
			Background(t.Highlight).
			Padding(0, 1),

		TaskFocused: lipgloss.NewStyle().
			Foreground(t.Primary).
			Bold(true).
			Padding(0, 1),

		TaskDone: lipgloss.NewStyle().
			Foreground(t.Subtle).
			Strikethrough(true).
			Padding(0, 1),

		TaskOverdue: lipgloss.NewStyle().
			Foreground(t.Error).
			Padding(0, 1),

		// Component styles
		Title: lipgloss.NewStyle().
			Foreground(t.Primary).
			Bold(true).
			MarginBottom(1),

		Subtitle: lipgloss.NewStyle().
			Foreground(t.Secondary).
			Italic(true),

		Label: lipgloss.NewStyle().
			Foreground(t.Subtle),

		Tag: lipgloss.NewStyle().
			Foreground(t.Info).
			Background(lipgloss.Color("#2E3440")).
			Padding(0, 1).
			MarginRight(1),

		DueDate: lipgloss.NewStyle().
			Foreground(t.Warning),

		Priority: lipgloss.NewStyle().
			Bold(true),

		// Input styles
		Input: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(t.Border).
			Padding(0, 1),

		InputFocused: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(t.Primary).
			Padding(0, 1),

		Placeholder: lipgloss.NewStyle().
			Foreground(t.Subtle),

		// Panel styles
		Panel: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(t.Border).
			Padding(1, 2),

		PanelTitle: lipgloss.NewStyle().
			Foreground(t.Primary).
			Bold(true).
			Padding(0, 1),

		PanelBorder: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(t.Border),

		// Help styles
		HelpKey: lipgloss.NewStyle().
			Foreground(t.Primary).
			Bold(true),

		HelpDesc: lipgloss.NewStyle().
			Foreground(t.Subtle),

		HelpSeparator: lipgloss.NewStyle().
			Foreground(t.Border),

		// Status bar
		StatusBar: lipgloss.NewStyle().
			Background(t.Highlight).
			Foreground(t.Foreground).
			Padding(0, 1),

		StatusKey: lipgloss.NewStyle().
			Foreground(t.Primary).
			Bold(true),

		StatusValue: lipgloss.NewStyle().
			Foreground(t.Foreground),
	}
}

// Current holds the current active theme and styles
var Current = struct {
	Theme  Theme
	Styles Styles
}{
	Theme:  Nord,
	Styles: NewStyles(Nord),
}

// SetTheme changes the current theme
func SetTheme(t Theme) {
	Current.Theme = t
	Current.Styles = NewStyles(t)
}

// Available returns all available themes
func Available() []Theme {
	return []Theme{
		Nord,
		Dracula,
		Gruvbox,
		Catppuccin,
	}
}

// ByName returns a theme by its name
func ByName(name string) (Theme, bool) {
	for _, t := range Available() {
		if t.Name == name {
			return t, true
		}
	}
	return Theme{}, false
}
