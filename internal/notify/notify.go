package notify

import (
	"os/exec"
	"strconv"
	"time"
)

// Urgency levels for notifications
type Urgency int

const (
	UrgencyLow Urgency = iota
	UrgencyNormal
	UrgencyCritical
)

// Notification represents a desktop notification
type Notification struct {
	Title   string
	Body    string
	Urgency Urgency
	Timeout time.Duration
	Icon    string // Optional icon name
}

// Notifier handles sending desktop notifications
type Notifier struct {
	enabled bool
}

// NewNotifier creates a new notifier
func NewNotifier() *Notifier {
	return &Notifier{
		enabled: true,
	}
}

// SetEnabled enables or disables notifications
func (n *Notifier) SetEnabled(enabled bool) {
	n.enabled = enabled
}

// IsEnabled returns whether notifications are enabled
func (n *Notifier) IsEnabled() bool {
	return n.enabled
}

// Send sends a desktop notification using notify-send
func (n *Notifier) Send(notification Notification) error {
	if !n.enabled {
		return nil
	}

	args := []string{}

	// Add urgency
	switch notification.Urgency {
	case UrgencyLow:
		args = append(args, "-u", "low")
	case UrgencyCritical:
		args = append(args, "-u", "critical")
	default:
		args = append(args, "-u", "normal")
	}

	// Add timeout (in milliseconds)
	if notification.Timeout > 0 {
		args = append(args, "-t", strconv.Itoa(int(notification.Timeout.Milliseconds())))
	}

	// Add icon if specified
	if notification.Icon != "" {
		args = append(args, "-i", notification.Icon)
	}

	// Add app name
	args = append(args, "-a", "klonch")

	// Add title and body
	args = append(args, notification.Title)
	if notification.Body != "" {
		args = append(args, notification.Body)
	}

	// Execute notify-send
	cmd := exec.Command("notify-send", args...)
	return cmd.Run()
}

// SendSimple sends a simple notification with title and body
func (n *Notifier) SendSimple(title, body string) error {
	return n.Send(Notification{
		Title:   title,
		Body:    body,
		Urgency: UrgencyNormal,
		Timeout: 5 * time.Second,
	})
}

// SendPomodoroComplete sends a pomodoro completion notification
func (n *Notifier) SendPomodoroComplete(taskTitle string, duration int) error {
	return n.Send(Notification{
		Title:   "Pomodoro Complete!",
		Body:    taskTitle,
		Urgency: UrgencyNormal,
		Timeout: 10 * time.Second,
		Icon:    "alarm-symbolic",
	})
}

// SendBreakComplete sends a break completion notification
func (n *Notifier) SendBreakComplete() error {
	return n.Send(Notification{
		Title:   "Break Over",
		Body:    "Time to get back to work!",
		Urgency: UrgencyNormal,
		Timeout: 10 * time.Second,
		Icon:    "appointment-soon-symbolic",
	})
}

// SendDueReminder sends a task due reminder
func (n *Notifier) SendDueReminder(taskTitle string, dueIn time.Duration) error {
	var body string
	if dueIn <= 0 {
		body = "Task is now overdue!"
	} else if dueIn < time.Hour {
		body = "Task due in less than an hour"
	} else {
		body = "Task due soon"
	}

	urgency := UrgencyNormal
	if dueIn <= 0 {
		urgency = UrgencyCritical
	}

	return n.Send(Notification{
		Title:   taskTitle,
		Body:    body,
		Urgency: urgency,
		Timeout: 15 * time.Second,
		Icon:    "emblem-important-symbolic",
	})
}
