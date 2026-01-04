package db

import (
	"database/sql"
	"time"

	"github.com/dori/klonch/internal/model"
	"github.com/google/uuid"
)

// GetTasks returns all non-archived top-level tasks
func (db *DB) GetTasks() ([]model.Task, error) {
	rows, err := db.Query(`
		SELECT id, title, description, status, priority, urgency, importance,
		       project_id, parent_id, due_date, start_date, completed_at,
		       time_estimate, recurrence, position, gcal_event_id,
		       created_at, updated_at
		FROM tasks
		WHERE status != 'archived' AND parent_id IS NULL
		ORDER BY
			CASE status WHEN 'done' THEN 1 ELSE 0 END,
			CASE priority
				WHEN 'urgent' THEN 0
				WHEN 'high' THEN 1
				WHEN 'medium' THEN 2
				WHEN 'low' THEN 3
			END,
			position,
			created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return db.scanTasks(rows)
}

// GetTasksByProject returns tasks for a specific project
func (db *DB) GetTasksByProject(projectID string) ([]model.Task, error) {
	rows, err := db.Query(`
		SELECT id, title, description, status, priority, urgency, importance,
		       project_id, parent_id, due_date, start_date, completed_at,
		       time_estimate, recurrence, position, gcal_event_id,
		       created_at, updated_at
		FROM tasks
		WHERE status != 'archived' AND parent_id IS NULL AND project_id = ?
		ORDER BY
			CASE status WHEN 'done' THEN 1 ELSE 0 END,
			position,
			created_at DESC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return db.scanTasks(rows)
}

// GetSubtasks returns subtasks for a parent task
func (db *DB) GetSubtasks(parentID string) ([]model.Task, error) {
	rows, err := db.Query(`
		SELECT id, title, description, status, priority, urgency, importance,
		       project_id, parent_id, due_date, start_date, completed_at,
		       time_estimate, recurrence, position, gcal_event_id,
		       created_at, updated_at
		FROM tasks
		WHERE parent_id = ?
		ORDER BY
			CASE status WHEN 'done' THEN 1 ELSE 0 END,
			CASE priority
				WHEN 'urgent' THEN 0
				WHEN 'high' THEN 1
				WHEN 'medium' THEN 2
				WHEN 'low' THEN 3
			END,
			position,
			created_at DESC
	`, parentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return db.scanTasks(rows)
}

// GetTask returns a single task by ID
func (db *DB) GetTask(id string) (*model.Task, error) {
	row := db.QueryRow(`
		SELECT id, title, description, status, priority, urgency, importance,
		       project_id, parent_id, due_date, start_date, completed_at,
		       time_estimate, recurrence, position, gcal_event_id,
		       created_at, updated_at
		FROM tasks WHERE id = ?
	`, id)

	t, err := db.scanTask(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return t, err
}

// CreateTask creates a new task
func (db *DB) CreateTask(title string, projectID *string) (*model.Task, error) {
	id := uuid.New().String()
	now := time.Now()

	// Use inbox as default project
	if projectID == nil {
		inbox := "inbox"
		projectID = &inbox
	}

	_, err := db.Exec(`
		INSERT INTO tasks (id, title, status, priority, project_id, created_at, updated_at)
		VALUES (?, ?, 'pending', 'medium', ?, ?, ?)
	`, id, title, *projectID, now, now)

	if err != nil {
		return nil, err
	}

	return &model.Task{
		ID:        id,
		Title:     title,
		Status:    model.StatusPending,
		Priority:  model.PriorityMedium,
		ProjectID: projectID,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// CreateSubtask creates a subtask under a parent task
func (db *DB) CreateSubtask(title, parentID string) (*model.Task, error) {
	id := uuid.New().String()
	now := time.Now()

	// Get parent's project
	var projectID *string
	db.QueryRow("SELECT project_id FROM tasks WHERE id = ?", parentID).Scan(&projectID)

	_, err := db.Exec(`
		INSERT INTO tasks (id, title, status, priority, project_id, parent_id, created_at, updated_at)
		VALUES (?, ?, 'pending', 'medium', ?, ?, ?, ?)
	`, id, title, projectID, parentID, now, now)

	if err != nil {
		return nil, err
	}

	return &model.Task{
		ID:        id,
		Title:     title,
		Status:    model.StatusPending,
		Priority:  model.PriorityMedium,
		ProjectID: projectID,
		ParentID:  &parentID,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// UpdateTaskTitle updates a task's title
func (db *DB) UpdateTaskTitle(id, title string) error {
	now := time.Now()
	_, err := db.Exec(`UPDATE tasks SET title = ?, updated_at = ? WHERE id = ?`, title, now, id)
	return err
}

// UpdateTaskProject moves a task to a different project
func (db *DB) UpdateTaskProject(id, projectID string) error {
	now := time.Now()
	_, err := db.Exec(`UPDATE tasks SET project_id = ?, updated_at = ? WHERE id = ?`, projectID, now, id)
	return err
}

// UpdateTaskPriority updates a task's priority
func (db *DB) UpdateTaskPriority(id string, priority model.Priority) error {
	now := time.Now()
	_, err := db.Exec(`UPDATE tasks SET priority = ?, updated_at = ? WHERE id = ?`, priority, now, id)
	return err
}

// UpdateTaskEisenhower updates a task's Eisenhower matrix values
func (db *DB) UpdateTaskEisenhower(id string, urgent, important bool) error {
	now := time.Now()
	urgency := 0
	importance := 0
	if urgent {
		urgency = 1
	}
	if important {
		importance = 1
	}
	_, err := db.Exec(`UPDATE tasks SET urgency = ?, importance = ?, updated_at = ? WHERE id = ?`,
		urgency, importance, now, id)
	return err
}

// ToggleTaskStatus toggles a task between pending and done
func (db *DB) ToggleTaskStatus(id string) error {
	now := time.Now()

	var status string
	err := db.QueryRow("SELECT status FROM tasks WHERE id = ?", id).Scan(&status)
	if err != nil {
		return err
	}

	var newStatus string
	var completedAt interface{}
	if status == string(model.StatusDone) {
		newStatus = string(model.StatusPending)
		completedAt = nil
	} else {
		newStatus = string(model.StatusDone)
		completedAt = now
	}

	_, err = db.Exec(`UPDATE tasks SET status = ?, completed_at = ?, updated_at = ? WHERE id = ?`,
		newStatus, completedAt, now, id)
	return err
}

// DeleteTask deletes a task and its subtasks
func (db *DB) DeleteTask(id string) error {
	// SQLite cascade will handle subtasks
	_, err := db.Exec(`DELETE FROM tasks WHERE id = ?`, id)
	return err
}

// Helper functions

func (db *DB) scanTasks(rows *sql.Rows) ([]model.Task, error) {
	var tasks []model.Task
	for rows.Next() {
		t, err := db.scanTaskRow(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, *t)
	}
	return tasks, nil
}

type scanner interface {
	Scan(dest ...interface{}) error
}

func (db *DB) scanTask(row *sql.Row) (*model.Task, error) {
	return db.scanTaskRow(row)
}

func (db *DB) scanTaskRow(s scanner) (*model.Task, error) {
	var t model.Task
	var description, projectID, parentID, dueDate, startDate, completedAt, recurrence, gcalID *string
	var timeEstimate, position *int
	var urgency, importance int

	err := s.Scan(
		&t.ID, &t.Title, &description, &t.Status, &t.Priority,
		&urgency, &importance, &projectID, &parentID,
		&dueDate, &startDate, &completedAt, &timeEstimate,
		&recurrence, &position, &gcalID, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	t.Urgency = urgency == 1
	t.Importance = importance == 1
	if description != nil {
		t.Description = *description
	}
	t.ProjectID = projectID
	t.ParentID = parentID
	t.TimeEstimate = timeEstimate
	t.Recurrence = recurrence
	t.GCalEventID = gcalID
	if position != nil {
		t.Position = *position
	}

	if dueDate != nil {
		if parsed, err := time.Parse(time.RFC3339, *dueDate); err == nil {
			t.DueDate = &parsed
		}
	}
	if startDate != nil {
		if parsed, err := time.Parse(time.RFC3339, *startDate); err == nil {
			t.StartDate = &parsed
		}
	}
	if completedAt != nil {
		if parsed, err := time.Parse(time.RFC3339, *completedAt); err == nil {
			t.CompletedAt = &parsed
		}
	}

	return &t, nil
}

// GetTaskDependencies returns tasks that this task depends on
func (db *DB) GetTaskDependencies(taskID string) ([]model.Task, error) {
	rows, err := db.Query(`
		SELECT t.id, t.title, t.description, t.status, t.priority, t.urgency, t.importance,
		       t.project_id, t.parent_id, t.due_date, t.start_date, t.completed_at,
		       t.time_estimate, t.recurrence, t.position, t.gcal_event_id,
		       t.created_at, t.updated_at
		FROM tasks t
		JOIN task_dependencies td ON t.id = td.depends_on_id
		WHERE td.task_id = ?
	`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return db.scanTasks(rows)
}

// AddTaskDependency adds a dependency
func (db *DB) AddTaskDependency(taskID, dependsOnID string) error {
	_, err := db.Exec(`
		INSERT OR IGNORE INTO task_dependencies (task_id, depends_on_id) VALUES (?, ?)
	`, taskID, dependsOnID)
	return err
}

// RemoveTaskDependency removes a dependency
func (db *DB) RemoveTaskDependency(taskID, dependsOnID string) error {
	_, err := db.Exec(`
		DELETE FROM task_dependencies WHERE task_id = ? AND depends_on_id = ?
	`, taskID, dependsOnID)
	return err
}

// IsTaskBlocked returns true if any dependencies are not done
func (db *DB) IsTaskBlocked(taskID string) (bool, error) {
	var count int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM task_dependencies td
		JOIN tasks t ON td.depends_on_id = t.id
		WHERE td.task_id = ? AND t.status != 'done'
	`, taskID).Scan(&count)
	return count > 0, err
}
