package db

import (
	"database/sql"
	"time"

	"github.com/dori/klonch/internal/model"
	"github.com/google/uuid"
)

// GetProjects returns all non-archived projects
func (db *DB) GetProjects() ([]model.Project, error) {
	rows, err := db.Query(`
		SELECT p.id, p.name, p.color, p.archived, p.position, p.created_at, p.updated_at,
		       (SELECT COUNT(*) FROM tasks WHERE project_id = p.id AND status != 'archived') as task_count,
		       (SELECT COUNT(*) FROM tasks WHERE project_id = p.id AND status = 'done') as completed_count
		FROM projects p
		WHERE p.archived = 0
		ORDER BY p.position, p.created_at
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []model.Project
	for rows.Next() {
		var p model.Project
		var archived int
		var color *string
		err := rows.Scan(
			&p.ID, &p.Name, &color, &archived, &p.Position,
			&p.CreatedAt, &p.UpdatedAt, &p.TaskCount, &p.CompletedCount,
		)
		if err != nil {
			return nil, err
		}
		p.Archived = archived == 1
		if color != nil {
			p.Color = *color
		}
		projects = append(projects, p)
	}

	return projects, nil
}

// GetProject returns a single project by ID
func (db *DB) GetProject(id string) (*model.Project, error) {
	var p model.Project
	var archived int
	var color *string

	err := db.QueryRow(`
		SELECT id, name, color, archived, position, created_at, updated_at
		FROM projects WHERE id = ?
	`, id).Scan(&p.ID, &p.Name, &color, &archived, &p.Position, &p.CreatedAt, &p.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	p.Archived = archived == 1
	if color != nil {
		p.Color = *color
	}

	return &p, nil
}

// CreateProject creates a new project
func (db *DB) CreateProject(name, color string) (*model.Project, error) {
	id := uuid.New().String()
	now := time.Now()

	// Get max position
	var maxPos sql.NullInt64
	db.QueryRow("SELECT MAX(position) FROM projects").Scan(&maxPos)
	position := 0
	if maxPos.Valid {
		position = int(maxPos.Int64) + 1
	}

	_, err := db.Exec(`
		INSERT INTO projects (id, name, color, position, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, id, name, color, position, now, now)

	if err != nil {
		return nil, err
	}

	return &model.Project{
		ID:        id,
		Name:      name,
		Color:     color,
		Position:  position,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// UpdateProject updates a project
func (db *DB) UpdateProject(id, name, color string) error {
	now := time.Now()
	_, err := db.Exec(`
		UPDATE projects SET name = ?, color = ?, updated_at = ? WHERE id = ?
	`, name, color, now, id)
	return err
}

// ArchiveProject archives a project
func (db *DB) ArchiveProject(id string) error {
	now := time.Now()
	_, err := db.Exec(`
		UPDATE projects SET archived = 1, updated_at = ? WHERE id = ?
	`, now, id)
	return err
}

// DeleteProject deletes a project (moves tasks to inbox)
func (db *DB) DeleteProject(id string) error {
	return db.Transaction(func(tx *sql.Tx) error {
		// Move tasks to inbox
		_, err := tx.Exec(`UPDATE tasks SET project_id = 'inbox' WHERE project_id = ?`, id)
		if err != nil {
			return err
		}

		// Delete project
		_, err = tx.Exec(`DELETE FROM projects WHERE id = ?`, id)
		return err
	})
}
