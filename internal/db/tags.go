package db

import (
	"database/sql"
	"strings"
	"time"

	"github.com/dori/klonch/internal/model"
	"github.com/google/uuid"
)

// GetTags returns all tags
func (db *DB) GetTags() ([]model.Tag, error) {
	rows, err := db.Query(`
		SELECT id, name, color, created_at
		FROM tags
		ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []model.Tag
	for rows.Next() {
		var t model.Tag
		var color *string
		err := rows.Scan(&t.ID, &t.Name, &color, &t.CreatedAt)
		if err != nil {
			return nil, err
		}
		if color != nil {
			t.Color = *color
		}
		tags = append(tags, t)
	}

	return tags, nil
}

// GetTag returns a single tag by ID
func (db *DB) GetTag(id string) (*model.Tag, error) {
	var t model.Tag
	var color *string

	err := db.QueryRow(`
		SELECT id, name, color, created_at
		FROM tags WHERE id = ?
	`, id).Scan(&t.ID, &t.Name, &color, &t.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if color != nil {
		t.Color = *color
	}

	return &t, nil
}

// GetTagByName returns a tag by name
func (db *DB) GetTagByName(name string) (*model.Tag, error) {
	// Normalize name (ensure @ prefix)
	if !strings.HasPrefix(name, "@") {
		name = "@" + name
	}

	var t model.Tag
	var color *string

	err := db.QueryRow(`
		SELECT id, name, color, created_at
		FROM tags WHERE name = ?
	`, name).Scan(&t.ID, &t.Name, &color, &t.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if color != nil {
		t.Color = *color
	}

	return &t, nil
}

// CreateTag creates a new tag
func (db *DB) CreateTag(name, color string) (*model.Tag, error) {
	// Normalize name (ensure @ prefix)
	if !strings.HasPrefix(name, "@") {
		name = "@" + name
	}

	id := uuid.New().String()
	now := time.Now()

	_, err := db.Exec(`
		INSERT INTO tags (id, name, color, created_at)
		VALUES (?, ?, ?, ?)
	`, id, name, color, now)

	if err != nil {
		return nil, err
	}

	return &model.Tag{
		ID:        id,
		Name:      name,
		Color:     color,
		CreatedAt: now,
	}, nil
}

// GetOrCreateTag gets a tag by name or creates it if it doesn't exist
func (db *DB) GetOrCreateTag(name, color string) (*model.Tag, error) {
	tag, err := db.GetTagByName(name)
	if err != nil {
		return nil, err
	}
	if tag != nil {
		return tag, nil
	}
	return db.CreateTag(name, color)
}

// UpdateTag updates a tag
func (db *DB) UpdateTag(id, name, color string) error {
	// Normalize name
	if !strings.HasPrefix(name, "@") {
		name = "@" + name
	}

	_, err := db.Exec(`
		UPDATE tags SET name = ?, color = ? WHERE id = ?
	`, name, color, id)
	return err
}

// DeleteTag deletes a tag
func (db *DB) DeleteTag(id string) error {
	return db.Transaction(func(tx *sql.Tx) error {
		// Remove tag associations
		_, err := tx.Exec(`DELETE FROM task_tags WHERE tag_id = ?`, id)
		if err != nil {
			return err
		}

		// Delete tag
		_, err = tx.Exec(`DELETE FROM tags WHERE id = ?`, id)
		return err
	})
}

// GetTaskTags returns tags for a task
func (db *DB) GetTaskTags(taskID string) ([]model.Tag, error) {
	rows, err := db.Query(`
		SELECT t.id, t.name, t.color, t.created_at
		FROM tags t
		JOIN task_tags tt ON t.id = tt.tag_id
		WHERE tt.task_id = ?
		ORDER BY t.name
	`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []model.Tag
	for rows.Next() {
		var t model.Tag
		var color *string
		err := rows.Scan(&t.ID, &t.Name, &color, &t.CreatedAt)
		if err != nil {
			return nil, err
		}
		if color != nil {
			t.Color = *color
		}
		tags = append(tags, t)
	}

	return tags, nil
}

// AddTagToTask adds a tag to a task
func (db *DB) AddTagToTask(taskID, tagID string) error {
	_, err := db.Exec(`
		INSERT OR IGNORE INTO task_tags (task_id, tag_id) VALUES (?, ?)
	`, taskID, tagID)
	return err
}

// RemoveTagFromTask removes a tag from a task
func (db *DB) RemoveTagFromTask(taskID, tagID string) error {
	_, err := db.Exec(`
		DELETE FROM task_tags WHERE task_id = ? AND tag_id = ?
	`, taskID, tagID)
	return err
}

// SetTaskTags replaces all tags on a task
func (db *DB) SetTaskTags(taskID string, tagIDs []string) error {
	return db.Transaction(func(tx *sql.Tx) error {
		// Remove existing tags
		_, err := tx.Exec(`DELETE FROM task_tags WHERE task_id = ?`, taskID)
		if err != nil {
			return err
		}

		// Add new tags
		for _, tagID := range tagIDs {
			_, err = tx.Exec(`INSERT INTO task_tags (task_id, tag_id) VALUES (?, ?)`, taskID, tagID)
			if err != nil {
				return err
			}
		}

		return nil
	})
}
