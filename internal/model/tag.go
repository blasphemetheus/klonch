package model

import (
	"time"
)

// Tag represents a context tag like @home, @work, @errands
type Tag struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Color     string    `json:"color,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// DisplayName returns the tag name with @ prefix if not already present
func (t *Tag) DisplayName() string {
	if len(t.Name) > 0 && t.Name[0] == '@' {
		return t.Name
	}
	return "@" + t.Name
}
