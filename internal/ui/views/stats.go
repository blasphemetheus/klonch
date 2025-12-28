package views

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dori/klonch/internal/db"
	"github.com/dori/klonch/internal/ui/theme"
)

// Local message types for stats view
type statsErrorMsg struct{ err error }

// TimePeriod represents a time range for stats
type TimePeriod int

const (
	PeriodWeek TimePeriod = iota
	PeriodMonth
	PeriodYear
)

// StatsView represents the statistics view
type StatsView struct {
	db     *db.DB
	width  int
	height int

	// Selected time period
	period TimePeriod

	// Stats data
	tasksCompleted   int
	tasksCreated     int
	tasksPending     int
	pomodoroCount    int
	totalMinutes     int
	avgMinutesPerDay float64

	// Time by project
	projectTime map[string]int // project name -> minutes

	// Daily completions (last 7 days)
	dailyCompletions []int

	// Streak
	currentStreak int
	longestStreak int

	// Status message
	statusMsg string
}

// NewStatsView creates a new stats view
func NewStatsView(database *db.DB) StatsView {
	return StatsView{
		db:          database,
		period:      PeriodWeek,
		projectTime: make(map[string]int),
	}
}

// Init initializes the stats view
func (v StatsView) Init() tea.Cmd {
	return v.loadStats()
}

// SetSize sets the view dimensions
func (v StatsView) SetSize(width, height int) StatsView {
	v.width = width
	v.height = height
	return v
}

// loadStats loads statistics from database
func (v StatsView) loadStats() tea.Cmd {
	return func() tea.Msg {
		now := time.Now()
		var startDate time.Time

		switch v.period {
		case PeriodWeek:
			startDate = now.AddDate(0, 0, -7)
		case PeriodMonth:
			startDate = now.AddDate(0, -1, 0)
		case PeriodYear:
			startDate = now.AddDate(-1, 0, 0)
		}

		startDateStr := startDate.Format("2006-01-02")

		// Count completed tasks
		var completed int
		row := v.db.QueryRow(`
			SELECT COUNT(*) FROM tasks
			WHERE status = 'done' AND completed_at >= ?
		`, startDateStr)
		row.Scan(&completed)

		// Count created tasks
		var created int
		row = v.db.QueryRow(`
			SELECT COUNT(*) FROM tasks
			WHERE created_at >= ?
		`, startDateStr)
		row.Scan(&created)

		// Count pending tasks
		var pending int
		row = v.db.QueryRow(`
			SELECT COUNT(*) FROM tasks
			WHERE status IN ('pending', 'in_progress')
		`)
		row.Scan(&pending)

		// Count pomodoros
		var pomodoros int
		row = v.db.QueryRow(`
			SELECT COUNT(*) FROM time_entries
			WHERE is_pomodoro = 1 AND started_at >= ?
		`, startDateStr)
		row.Scan(&pomodoros)

		// Total time tracked
		var totalMins int
		row = v.db.QueryRow(`
			SELECT COALESCE(SUM(duration), 0) FROM time_entries
			WHERE started_at >= ?
		`, startDateStr)
		row.Scan(&totalMins)

		// Calculate average per day
		days := int(now.Sub(startDate).Hours() / 24)
		if days < 1 {
			days = 1
		}
		avgPerDay := float64(totalMins) / float64(days)

		// Time by project
		projectTime := make(map[string]int)
		rows, err := v.db.Query(`
			SELECT COALESCE(p.name, 'No Project'), COALESCE(SUM(te.duration), 0)
			FROM time_entries te
			LEFT JOIN tasks t ON te.task_id = t.id
			LEFT JOIN projects p ON t.project_id = p.id
			WHERE te.started_at >= ?
			GROUP BY p.name
			ORDER BY SUM(te.duration) DESC
		`, startDateStr)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var name string
				var mins int
				if err := rows.Scan(&name, &mins); err == nil {
					projectTime[name] = mins
				}
			}
		}

		// Daily completions (last 7 days)
		dailyCompletions := make([]int, 7)
		for i := 6; i >= 0; i-- {
			day := now.AddDate(0, 0, -i)
			dayStart := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.Local)
			dayEnd := dayStart.AddDate(0, 0, 1)

			var count int
			row = v.db.QueryRow(`
				SELECT COUNT(*) FROM tasks
				WHERE status = 'done'
				AND completed_at >= ? AND completed_at < ?
			`, dayStart.Format("2006-01-02"), dayEnd.Format("2006-01-02"))
			row.Scan(&count)
			dailyCompletions[6-i] = count
		}

		// Calculate streak (consecutive days with at least 1 completion)
		currentStreak := 0
		longestStreak := 0
		tempStreak := 0

		for i := 0; i < 30; i++ {
			day := now.AddDate(0, 0, -i)
			dayStart := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.Local)
			dayEnd := dayStart.AddDate(0, 0, 1)

			var count int
			row = v.db.QueryRow(`
				SELECT COUNT(*) FROM tasks
				WHERE status = 'done'
				AND completed_at >= ? AND completed_at < ?
			`, dayStart.Format("2006-01-02"), dayEnd.Format("2006-01-02"))
			row.Scan(&count)

			if count > 0 {
				tempStreak++
				if i == 0 || currentStreak > 0 {
					currentStreak = tempStreak
				}
				if tempStreak > longestStreak {
					longestStreak = tempStreak
				}
			} else {
				if i == 0 {
					currentStreak = 0
				}
				tempStreak = 0
			}
		}

		return statsLoadedMsg{
			completed:        completed,
			created:          created,
			pending:          pending,
			pomodoros:        pomodoros,
			totalMins:        totalMins,
			avgPerDay:        avgPerDay,
			projectTime:      projectTime,
			dailyCompletions: dailyCompletions,
			currentStreak:    currentStreak,
			longestStreak:    longestStreak,
		}
	}
}

type statsLoadedMsg struct {
	completed        int
	created          int
	pending          int
	pomodoros        int
	totalMins        int
	avgPerDay        float64
	projectTime      map[string]int
	dailyCompletions []int
	currentStreak    int
	longestStreak    int
}

// Update handles messages
func (v StatsView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case statsLoadedMsg:
		v.tasksCompleted = msg.completed
		v.tasksCreated = msg.created
		v.tasksPending = msg.pending
		v.pomodoroCount = msg.pomodoros
		v.totalMinutes = msg.totalMins
		v.avgMinutesPerDay = msg.avgPerDay
		v.projectTime = msg.projectTime
		v.dailyCompletions = msg.dailyCompletions
		v.currentStreak = msg.currentStreak
		v.longestStreak = msg.longestStreak
		return v, nil

	case taskUpdatedMsg:
		return v, v.loadStats()

	case tea.KeyMsg:
		switch msg.String() {
		case "w":
			v.period = PeriodWeek
			return v, v.loadStats()
		case "m":
			v.period = PeriodMonth
			return v, v.loadStats()
		case "y":
			v.period = PeriodYear
			return v, v.loadStats()
		case "r":
			return v, v.loadStats()
		}
	}

	return v, nil
}

// View renders the stats view
func (v StatsView) View() string {
	if v.width == 0 || v.height == 0 {
		return "Loading..."
	}

	t := theme.Current.Theme

	var sections []string

	// Title with period selector
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(t.Primary)
	periodLabels := []string{"Week", "Month", "Year"}
	periodStr := periodLabels[v.period]
	sections = append(sections, titleStyle.Render(fmt.Sprintf("Statistics ─ %s", periodStr)))
	sections = append(sections, "")

	// Summary cards (side by side)
	cardStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(t.Border).
		Padding(0, 2).
		Width(18)

	valueStyle := lipgloss.NewStyle().Bold(true).Foreground(t.Primary)
	labelStyle := lipgloss.NewStyle().Foreground(t.Subtle)

	completedCard := cardStyle.Render(
		valueStyle.Render(fmt.Sprintf("%d", v.tasksCompleted)) + "\n" +
			labelStyle.Render("Completed"),
	)

	pomodoroCard := cardStyle.Render(
		valueStyle.Render(fmt.Sprintf("%d", v.pomodoroCount)) + "\n" +
			labelStyle.Render("Pomodoros"),
	)

	hours := v.totalMinutes / 60
	mins := v.totalMinutes % 60
	timeCard := cardStyle.Render(
		valueStyle.Render(fmt.Sprintf("%dh %dm", hours, mins)) + "\n" +
			labelStyle.Render("Time Tracked"),
	)

	streakCard := cardStyle.Render(
		valueStyle.Render(fmt.Sprintf("%d days", v.currentStreak)) + "\n" +
			labelStyle.Render("Current Streak"),
	)

	cardRow := lipgloss.JoinHorizontal(lipgloss.Top, completedCard, pomodoroCard, timeCard, streakCard)
	sections = append(sections, cardRow)
	sections = append(sections, "")

	// Activity chart (last 7 days)
	chartSection := v.renderActivityChart()
	sections = append(sections, chartSection)
	sections = append(sections, "")

	// Time by project
	if len(v.projectTime) > 0 {
		projectSection := v.renderProjectTime()
		sections = append(sections, projectSection)
	}

	// Footer hints
	hints := lipgloss.NewStyle().Foreground(t.Subtle).Render(
		"w: week • m: month • y: year • r: refresh",
	)
	sections = append(sections, hints)

	return strings.Join(sections, "\n")
}

// renderActivityChart renders the 7-day activity chart
func (v StatsView) renderActivityChart() string {
	t := theme.Current.Theme

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(t.Secondary)

	var lines []string
	lines = append(lines, headerStyle.Render("Activity (Last 7 Days)"))

	// Find max for scaling
	maxCount := 1
	for _, count := range v.dailyCompletions {
		if count > maxCount {
			maxCount = count
		}
	}

	// Bar chart
	chartHeight := 5
	barWidth := 4
	days := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}

	// Get current day of week to label correctly
	now := time.Now()
	currentDow := int(now.Weekday())
	if currentDow == 0 {
		currentDow = 7 // Sunday = 7
	}

	for row := chartHeight; row >= 1; row-- {
		var rowStr strings.Builder
		threshold := float64(row) / float64(chartHeight)

		for i, count := range v.dailyCompletions {
			ratio := float64(count) / float64(maxCount)

			var block string
			if ratio >= threshold {
				block = lipgloss.NewStyle().Foreground(t.Success).Render(strings.Repeat("█", barWidth))
			} else if ratio >= threshold-0.2 && ratio > 0 {
				block = lipgloss.NewStyle().Foreground(t.Info).Render(strings.Repeat("▄", barWidth))
			} else {
				block = strings.Repeat(" ", barWidth)
			}

			rowStr.WriteString(block)
			if i < len(v.dailyCompletions)-1 {
				rowStr.WriteString(" ")
			}
		}
		lines = append(lines, rowStr.String())
	}

	// Day labels
	var labelStr strings.Builder
	for i := range v.dailyCompletions {
		// Calculate which day this represents
		dayIndex := (currentDow - 6 + i + 7) % 7
		if dayIndex == 0 {
			dayIndex = 7
		}
		label := days[dayIndex-1][:3]
		labelStr.WriteString(lipgloss.NewStyle().Foreground(t.Subtle).Width(barWidth).Align(lipgloss.Center).Render(label))
		if i < len(v.dailyCompletions)-1 {
			labelStr.WriteString(" ")
		}
	}
	lines = append(lines, labelStr.String())

	// Count labels
	var countStr strings.Builder
	for i, count := range v.dailyCompletions {
		countLabel := fmt.Sprintf("%d", count)
		countStr.WriteString(lipgloss.NewStyle().Foreground(t.Foreground).Width(barWidth).Align(lipgloss.Center).Render(countLabel))
		if i < len(v.dailyCompletions)-1 {
			countStr.WriteString(" ")
		}
	}
	lines = append(lines, countStr.String())

	return strings.Join(lines, "\n")
}

// renderProjectTime renders time tracked per project
func (v StatsView) renderProjectTime() string {
	t := theme.Current.Theme

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(t.Secondary)

	var lines []string
	lines = append(lines, headerStyle.Render("Time by Project"))

	// Find max for bar scaling
	maxMins := 1
	for _, mins := range v.projectTime {
		if mins > maxMins {
			maxMins = mins
		}
	}

	barMaxWidth := 30
	for name, mins := range v.projectTime {
		hours := mins / 60
		m := mins % 60

		// Calculate bar width
		ratio := float64(mins) / float64(maxMins)
		barWidth := int(ratio * float64(barMaxWidth))
		if barWidth < 1 && mins > 0 {
			barWidth = 1
		}

		bar := lipgloss.NewStyle().Foreground(t.Info).Render(strings.Repeat("█", barWidth))
		timeStr := fmt.Sprintf("%2dh %02dm", hours, m)

		line := fmt.Sprintf("%-15s %s %s", name, bar, timeStr)
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

// IsInputMode returns whether the view is in input mode
func (v StatsView) IsInputMode() bool {
	return false
}
