package views

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dori/klonch/internal/db"
	"github.com/dori/klonch/internal/model"
	"github.com/dori/klonch/internal/ui/theme"
)

// Local message types for calendar view
type calendarErrorMsg struct{ err error }

// CalendarView represents the calendar view
type CalendarView struct {
	db     *db.DB
	width  int
	height int

	// Current month being displayed
	year  int
	month time.Month

	// Selected day
	selectedDay int

	// Tasks indexed by day of month
	tasksByDay map[int][]model.Task

	// Status message
	statusMsg string
}

// NewCalendarView creates a new calendar view
func NewCalendarView(database *db.DB) CalendarView {
	now := time.Now()
	return CalendarView{
		db:          database,
		year:        now.Year(),
		month:       now.Month(),
		selectedDay: now.Day(),
		tasksByDay:  make(map[int][]model.Task),
	}
}

// Init initializes the calendar view
func (v CalendarView) Init() tea.Cmd {
	return v.loadTasks()
}

// SetSize sets the view dimensions
func (v CalendarView) SetSize(width, height int) CalendarView {
	v.width = width
	v.height = height
	return v
}

// loadTasks loads tasks for the current month
func (v CalendarView) loadTasks() tea.Cmd {
	return func() tea.Msg {
		// Get first and last day of month
		firstDay := time.Date(v.year, v.month, 1, 0, 0, 0, 0, time.Local)
		lastDay := firstDay.AddDate(0, 1, 0).Add(-time.Second)

		rows, err := v.db.Query(`
			SELECT id, title, description, status, priority, due_date
			FROM tasks
			WHERE parent_id IS NULL
			  AND status != 'archived'
			  AND due_date IS NOT NULL
			  AND due_date >= ?
			  AND due_date <= ?
			ORDER BY due_date, priority DESC
		`, firstDay.Format("2006-01-02"), lastDay.Format("2006-01-02 23:59:59"))
		if err != nil {
			return calendarErrorMsg{err: err}
		}
		defer rows.Close()

		tasksByDay := make(map[int][]model.Task)
		for rows.Next() {
			var t model.Task
			var desc *string
			var dueDate string
			if err := rows.Scan(&t.ID, &t.Title, &desc, &t.Status, &t.Priority, &dueDate); err != nil {
				continue
			}
			if desc != nil {
				t.Description = *desc
			}

			// Parse due date
			if parsed, err := time.Parse("2006-01-02 15:04:05", dueDate); err == nil {
				t.DueDate = &parsed
				day := parsed.Day()
				tasksByDay[day] = append(tasksByDay[day], t)
			} else if parsed, err := time.Parse("2006-01-02", dueDate); err == nil {
				t.DueDate = &parsed
				day := parsed.Day()
				tasksByDay[day] = append(tasksByDay[day], t)
			}
		}

		return calendarLoadedMsg{tasksByDay: tasksByDay}
	}
}

type calendarLoadedMsg struct {
	tasksByDay map[int][]model.Task
}

// Update handles messages
func (v CalendarView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case calendarLoadedMsg:
		v.tasksByDay = msg.tasksByDay
		return v, nil

	case taskUpdatedMsg:
		return v, v.loadTasks()

	case tea.KeyMsg:
		daysInMonth := v.daysInMonth()

		switch msg.String() {
		// Navigate days
		case "h", "left":
			if v.selectedDay > 1 {
				v.selectedDay--
			}
			return v, nil

		case "l", "right":
			if v.selectedDay < daysInMonth {
				v.selectedDay++
			}
			return v, nil

		case "k", "up":
			if v.selectedDay > 7 {
				v.selectedDay -= 7
			}
			return v, nil

		case "j", "down":
			if v.selectedDay+7 <= daysInMonth {
				v.selectedDay += 7
			}
			return v, nil

		// Navigate months
		case "H", "pgup":
			v.month--
			if v.month < 1 {
				v.month = 12
				v.year--
			}
			v.clampSelectedDay()
			return v, v.loadTasks()

		case "L", "pgdown":
			v.month++
			if v.month > 12 {
				v.month = 1
				v.year++
			}
			v.clampSelectedDay()
			return v, v.loadTasks()

		case "t": // Today
			now := time.Now()
			v.year = now.Year()
			v.month = now.Month()
			v.selectedDay = now.Day()
			return v, v.loadTasks()

		case "g":
			v.selectedDay = 1
			return v, nil

		case "G":
			v.selectedDay = daysInMonth
			return v, nil
		}
	}

	return v, nil
}

// daysInMonth returns the number of days in the current month
func (v CalendarView) daysInMonth() int {
	return time.Date(v.year, v.month+1, 0, 0, 0, 0, 0, time.Local).Day()
}

// clampSelectedDay ensures selected day is valid for current month
func (v *CalendarView) clampSelectedDay() {
	daysInMonth := v.daysInMonth()
	if v.selectedDay > daysInMonth {
		v.selectedDay = daysInMonth
	}
}

// View renders the calendar
func (v CalendarView) View() string {
	if v.width == 0 || v.height == 0 {
		return "Loading..."
	}

	t := theme.Current.Theme

	// Split into two panels: calendar (left) and task list (right)
	calWidth := 28  // Fixed width for calendar grid
	listWidth := v.width - calWidth - 4

	// Render calendar
	calendar := v.renderCalendar(calWidth, v.height-2)

	// Render task list for selected day
	taskList := v.renderTaskList(listWidth, v.height-2)

	// Join panels
	panels := lipgloss.JoinHorizontal(lipgloss.Top, calendar, taskList)

	// Footer with hints
	hints := lipgloss.NewStyle().Foreground(t.Subtle).Render(
		"h/j/k/l: navigate days • H/L: change month • t: today",
	)

	return lipgloss.JoinVertical(lipgloss.Left, panels, hints)
}

// renderCalendar renders the calendar grid
func (v CalendarView) renderCalendar(width, height int) string {
	t := theme.Current.Theme

	// Month header
	monthName := fmt.Sprintf("%s %d", v.month.String(), v.year)
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Primary).
		Width(width).
		Align(lipgloss.Center)

	// Day labels
	dayLabelStyle := lipgloss.NewStyle().
		Foreground(t.Subtle).
		Width(4).
		Align(lipgloss.Center)

	dayLabels := "Su Mo Tu We Th Fr Sa"

	// Build calendar grid
	var lines []string
	lines = append(lines, headerStyle.Render(monthName))
	lines = append(lines, dayLabelStyle.Render(dayLabels))

	// Get first day of month (weekday)
	firstDay := time.Date(v.year, v.month, 1, 0, 0, 0, 0, time.Local)
	startWeekday := int(firstDay.Weekday()) // 0 = Sunday

	// Get days in month
	daysInMonth := v.daysInMonth()

	// Get today
	now := time.Now()
	isCurrentMonth := v.year == now.Year() && v.month == now.Month()
	today := now.Day()

	// Build weeks
	var week []string
	// Add empty cells for days before the 1st
	for i := 0; i < startWeekday; i++ {
		week = append(week, "   ")
	}

	for day := 1; day <= daysInMonth; day++ {
		dayStyle := lipgloss.NewStyle().Width(3).Align(lipgloss.Center)

		// Check if this day has tasks
		hasTasks := len(v.tasksByDay[day]) > 0

		// Check if this is selected day
		isSelected := day == v.selectedDay

		// Check if this is today
		isToday := isCurrentMonth && day == today

		// Apply styles
		if isSelected {
			dayStyle = dayStyle.Background(t.Highlight).Bold(true)
		}
		if isToday {
			dayStyle = dayStyle.Foreground(t.Primary)
		}
		if hasTasks && !isSelected {
			dayStyle = dayStyle.Foreground(t.Info)
		}

		dayStr := fmt.Sprintf("%2d", day)
		if hasTasks {
			dayStr += "•"
		} else {
			dayStr += " "
		}

		week = append(week, dayStyle.Render(dayStr))

		// Start new week on Saturday
		if (startWeekday+day)%7 == 0 {
			lines = append(lines, strings.Join(week, ""))
			week = nil
		}
	}

	// Add remaining days of last week
	if len(week) > 0 {
		for len(week) < 7 {
			week = append(week, "   ")
		}
		lines = append(lines, strings.Join(week, ""))
	}

	// Wrap in a box
	content := strings.Join(lines, "\n")
	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(t.Border).
		Padding(0, 1)

	return boxStyle.Render(content)
}

// renderTaskList renders the task list for the selected day
func (v CalendarView) renderTaskList(width, height int) string {
	t := theme.Current.Theme

	// Header
	date := time.Date(v.year, v.month, v.selectedDay, 0, 0, 0, 0, time.Local)
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Primary).
		Width(width)

	header := headerStyle.Render(date.Format("Monday, January 2"))

	// Tasks for this day
	tasks := v.tasksByDay[v.selectedDay]

	var lines []string
	lines = append(lines, header)
	lines = append(lines, "")

	if len(tasks) == 0 {
		lines = append(lines, lipgloss.NewStyle().
			Foreground(t.Subtle).
			Italic(true).
			Render("No tasks due this day"))
	} else {
		for _, task := range tasks {
			// Status checkbox
			checkbox := "☐"
			if task.Status == model.StatusDone {
				checkbox = "☑"
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

			// Truncate title if needed
			title := task.Title
			maxLen := width - 10
			if len(title) > maxLen {
				title = title[:maxLen-3] + "..."
			}

			taskStyle := lipgloss.NewStyle().Foreground(t.Foreground)
			if task.Status == model.StatusDone {
				taskStyle = taskStyle.Strikethrough(true).Foreground(t.Subtle)
			}

			line := fmt.Sprintf("%s %s %s", checkbox, priorityChar, taskStyle.Render(title))
			lines = append(lines, line)
		}
	}

	content := strings.Join(lines, "\n")
	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(t.Border).
		Padding(0, 1).
		Width(width)

	return boxStyle.Render(content)
}

// IsInputMode returns whether the view is in input mode
func (v CalendarView) IsInputMode() bool {
	return false
}
