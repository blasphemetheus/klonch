package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dori/klonch/internal/app"
	"github.com/dori/klonch/internal/db"
	"github.com/dori/klonch/internal/model"
	"github.com/dori/klonch/internal/ui"
	"github.com/google/uuid"
)

var (
	version = "0.1.0"
)

func main() {
	// Subcommand handling
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "add":
			handleAdd(os.Args[2:])
			return
		case "version":
			fmt.Printf("klonch v%s\n", version)
			return
		case "help", "-h", "--help":
			printHelp()
			return
		}
	}

	// Parse flags for TUI mode
	viewFlag := flag.String("view", "list", "Starting view (list, kanban, eisenhower, calendar, pomodoro)")
	themeFlag := flag.String("theme", "", "Theme name (nord, dracula, gruvbox, catppuccin)")
	flag.Parse()

	// Run TUI
	if err := runTUI(*viewFlag, *themeFlag); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func printHelp() {
	help := `klonch - A feature-rich todo application

Usage:
  klonch                    Start the TUI
  klonch add <task>         Quick add a task
  klonch version            Show version
  klonch help               Show this help

Quick Add Syntax:
  klonch add "Buy groceries"
  klonch add "Review PR #work @urgent !high due:tomorrow"

  Project:   #name         (e.g., #work, #personal, #inbox)
  Tags:      @tag          (e.g., @home, @urgent, @review)
  Priority:  !low !medium !high !urgent (or !l !m !h !u)
  Due date:  due:tomorrow due:friday due:2024-01-15
             due:today due:mon due:nextweek

TUI Options:
  --view <name>     Starting view (list, kanban, eisenhower, calendar, pomodoro)
  --theme <name>    Theme (nord, dracula, gruvbox, catppuccin)

TUI Keybindings:
  Navigation:   j/k ↑/↓       Move cursor
                g/G           Top/bottom
                Space         Toggle selection

  Tasks:        a             Add task
                s             Add subtask
                Enter         Edit
                Tab           Toggle done
                d             Delete
                p             Cycle priority
                m             Move to project
                t             Toggle tag
                o             Expand/collapse subtasks
                E/C           Expand/collapse all
                f             Focus mode

  Filtering:    /             Search
                M             Filter by project
                T             Filter by tag
                A             Toggle Active/All
                Esc           Clear filters

  Views:        1-8           Switch views
                :             Command palette
                ?             Help
                q             Quit

For more: https://github.com/dori/klonch`

	fmt.Println(help)
}

func handleAdd(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: klonch add <task>")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Examples:")
		fmt.Fprintln(os.Stderr, "  klonch add \"Buy groceries\"")
		fmt.Fprintln(os.Stderr, "  klonch add \"Review PR #work @urgent !high due:tomorrow\"")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Syntax: #project @tag !priority due:date")
		os.Exit(1)
	}

	// Join all args as the task text
	text := strings.Join(args, " ")

	// Parse the task text
	task := parseQuickAdd(text)

	// Open database (no lock needed for quick add - just insert)
	database, err := db.Open(db.DefaultDBPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	// Find or create project
	projectID := "inbox"
	projectName := "Inbox"
	if task.parsedProject != "" {
		// Try to find existing project by name (case-insensitive)
		row := database.QueryRow(`
			SELECT id, name FROM projects
			WHERE LOWER(name) = LOWER(?) AND archived = 0
		`, task.parsedProject)

		var existingID, existingName string
		if err := row.Scan(&existingID, &existingName); err == nil {
			projectID = existingID
			projectName = existingName
		} else {
			// Create new project
			projectID = strings.ToLower(task.parsedProject)
			projectName = task.parsedProject
			now := time.Now()
			database.Exec(`
				INSERT OR IGNORE INTO projects (id, name, position, archived, created_at, updated_at)
				VALUES (?, ?, 0, 0, ?, ?)
			`, projectID, projectName, now, now)
		}
	}

	// Insert task
	now := time.Now()
	var dueDate interface{}
	if task.DueDate != nil {
		dueDate = task.DueDate.Format(time.RFC3339)
	}

	_, err = database.Exec(`
		INSERT INTO tasks (id, title, status, priority, project_id, due_date, created_at, updated_at)
		VALUES (?, ?, 'pending', ?, ?, ?, ?, ?)
	`, task.ID, task.Title, task.Priority, projectID, dueDate, now, now)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating task: %v\n", err)
		os.Exit(1)
	}

	// Create tags and associations
	for _, tagName := range task.parsedTags {
		// Ensure tag exists
		tagID := strings.ToLower(strings.TrimPrefix(tagName, "@"))
		database.Exec(`
			INSERT OR IGNORE INTO tags (id, name, created_at) VALUES (?, ?, ?)
		`, tagID, tagName, now)

		// Associate tag with task
		database.Exec(`
			INSERT OR IGNORE INTO task_tags (task_id, tag_id) VALUES (?, ?)
		`, task.ID, tagID)
	}

	// Output
	fmt.Printf("Created: %s\n", task.Title)
	if projectID != "inbox" {
		fmt.Printf("Project: %s\n", projectName)
	}
	if task.DueDate != nil {
		fmt.Printf("Due: %s\n", formatDueDate(*task.DueDate))
	}
	if task.Priority != model.PriorityMedium {
		fmt.Printf("Priority: %s\n", task.Priority)
	}
	if len(task.parsedTags) > 0 {
		fmt.Printf("Tags: %s\n", strings.Join(task.parsedTags, ", "))
	}
}

type quickAddTask struct {
	model.Task
	parsedTags    []string
	parsedProject string
}

func parseQuickAdd(text string) quickAddTask {
	task := quickAddTask{
		Task: model.Task{
			ID:       uuid.New().String(),
			Priority: model.PriorityMedium,
		},
	}

	words := strings.Fields(text)
	var titleParts []string

	for _, word := range words {
		switch {
		// Project (#work, #personal, etc.)
		case strings.HasPrefix(word, "#"):
			task.parsedProject = strings.TrimPrefix(word, "#")

		// Tags (@home, @work, etc.)
		case strings.HasPrefix(word, "@"):
			task.parsedTags = append(task.parsedTags, word)

		// Priority (!low, !high, etc.)
		case strings.HasPrefix(word, "!"):
			priority := strings.ToLower(strings.TrimPrefix(word, "!"))
			switch priority {
			case "low", "l":
				task.Priority = model.PriorityLow
			case "medium", "med", "m":
				task.Priority = model.PriorityMedium
			case "high", "hi", "h":
				task.Priority = model.PriorityHigh
			case "urgent", "u":
				task.Priority = model.PriorityUrgent
			default:
				titleParts = append(titleParts, word)
			}

		// Due date (due:tomorrow, due:friday, due:2024-01-15)
		case strings.HasPrefix(strings.ToLower(word), "due:"):
			dateStr := strings.TrimPrefix(strings.ToLower(word), "due:")
			if parsed := parseNaturalDate(dateStr); parsed != nil {
				task.DueDate = parsed
			} else {
				titleParts = append(titleParts, word)
			}

		default:
			titleParts = append(titleParts, word)
		}
	}

	task.Title = strings.Join(titleParts, " ")
	return task
}

func parseNaturalDate(s string) *time.Time {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())

	switch strings.ToLower(s) {
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
	case "nextweek":
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
			// If no year, use current year
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

func formatDueDate(t time.Time) string {
	now := time.Now()

	if t.Year() == now.Year() && t.YearDay() == now.YearDay() {
		return "today"
	}

	tomorrow := now.AddDate(0, 0, 1)
	if t.Year() == tomorrow.Year() && t.YearDay() == tomorrow.YearDay() {
		return "tomorrow"
	}

	if t.Year() == now.Year() {
		return t.Format("Mon, Jan 2")
	}

	return t.Format("Jan 2, 2006")
}

func runTUI(startView, themeName string) error {
	// Create application
	application, err := app.New(nil)
	if err != nil {
		return err
	}
	defer application.Close()

	// Set theme if specified
	if themeName != "" {
		// Theme setting would happen here
		// For now, theme is set in the root model
	}

	// Create root model
	model := ui.NewRootModel(application)

	// Create and run program
	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	_, err = p.Run()
	return err
}
