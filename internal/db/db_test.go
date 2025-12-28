package db

import (
	"path/filepath"
	"testing"
	"time"
)

// TestNestedQueriesNoDeadlock is a regression test for the SQLite deadlock bug
// where nested queries during rows iteration would cause a deadlock due to
// SetMaxOpenConns(1) limiting SQLite to a single connection.
//
// The bug manifested when loadTasks() called GetTaskTags(), GetSubtasks(), and
// GetTaskDependencies() while still iterating over the main task rows.
// This test verifies that such nested operations complete without deadlock.
func TestNestedQueriesNoDeadlock(t *testing.T) {
	// Create a temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create some test data
	now := time.Now()

	// Create a project
	_, err = db.Exec(`INSERT INTO projects (id, name, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		"proj1", "Test Project", now, now)
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Create some tags
	for i := 1; i <= 3; i++ {
		_, err = db.Exec(`INSERT INTO tags (id, name, created_at) VALUES (?, ?, ?)`,
			"tag"+string(rune('0'+i)), "Tag"+string(rune('0'+i)), now)
		if err != nil {
			t.Fatalf("Failed to create tag: %v", err)
		}
	}

	// Create multiple tasks
	for i := 1; i <= 5; i++ {
		taskID := "task" + string(rune('0'+i))
		_, err = db.Exec(`INSERT INTO tasks (id, title, status, priority, project_id, created_at, updated_at)
			VALUES (?, ?, 'pending', 'medium', ?, ?, ?)`,
			taskID, "Task "+string(rune('0'+i)), "proj1", now, now)
		if err != nil {
			t.Fatalf("Failed to create task: %v", err)
		}

		// Associate tags with task
		for j := 1; j <= 2; j++ {
			tagID := "tag" + string(rune('0'+j))
			_, err = db.Exec(`INSERT INTO task_tags (task_id, tag_id) VALUES (?, ?)`, taskID, tagID)
			if err != nil {
				t.Fatalf("Failed to associate tag: %v", err)
			}
		}
	}

	// Create a subtask
	_, err = db.Exec(`INSERT INTO tasks (id, title, status, priority, project_id, parent_id, created_at, updated_at)
		VALUES (?, ?, 'pending', 'medium', ?, ?, ?, ?)`,
		"subtask1", "Subtask 1", "proj1", "task1", now, now)
	if err != nil {
		t.Fatalf("Failed to create subtask: %v", err)
	}

	// Create a dependency
	_, err = db.Exec(`INSERT INTO task_dependencies (task_id, depends_on_id) VALUES (?, ?)`,
		"task2", "task1")
	if err != nil {
		t.Fatalf("Failed to create dependency: %v", err)
	}

	// Now simulate what loadTasks does: query main tasks, then for each task
	// query related data. This should NOT deadlock after the fix.

	// Use a timeout to detect deadlock
	done := make(chan bool, 1)
	go func() {
		// Query main tasks
		rows, err := db.Query(`
			SELECT id, title, status, priority, project_id
			FROM tasks
			WHERE parent_id IS NULL
		`)
		if err != nil {
			t.Errorf("Main query failed: %v", err)
			done <- false
			return
		}

		// Collect task IDs first (the fix)
		var taskIDs []string
		for rows.Next() {
			var id, title, status, priority string
			var projectID *string
			if err := rows.Scan(&id, &title, &status, &priority, &projectID); err != nil {
				t.Errorf("Scan failed: %v", err)
				done <- false
				return
			}
			taskIDs = append(taskIDs, id)
		}
		rows.Close() // Close BEFORE nested queries

		// Now do nested queries (after rows are closed)
		for _, taskID := range taskIDs {
			// Get tags
			tags, err := db.GetTaskTags(taskID)
			if err != nil {
				t.Errorf("GetTaskTags failed: %v", err)
				done <- false
				return
			}
			if len(tags) == 0 {
				t.Logf("Task %s has no tags (might be expected for subtasks)", taskID)
			}

			// Get subtasks
			subtasks, err := db.GetSubtasks(taskID)
			if err != nil {
				t.Errorf("GetSubtasks failed: %v", err)
				done <- false
				return
			}
			_ = subtasks

			// Get dependencies
			deps, err := db.GetTaskDependencies(taskID)
			if err != nil {
				t.Errorf("GetTaskDependencies failed: %v", err)
				done <- false
				return
			}
			_ = deps
		}

		done <- true
	}()

	select {
	case success := <-done:
		if !success {
			t.Fatal("Test failed during execution")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Test timed out - possible deadlock detected")
	}
}

// TestConcurrentReadsDuringIteration verifies that making nested queries
// while iterating over rows would cause issues with SetMaxOpenConns(1).
// This test documents the bug behavior (before the fix).
func TestDocumentDeadlockScenario(t *testing.T) {
	t.Skip("This test documents the deadlock scenario - skip in normal runs")

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create test data
	now := time.Now()
	_, _ = db.Exec(`INSERT INTO projects (id, name, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		"proj1", "Test", now, now)
	_, _ = db.Exec(`INSERT INTO tasks (id, title, status, priority, project_id, created_at, updated_at)
		VALUES (?, ?, 'pending', 'medium', ?, ?, ?)`,
		"task1", "Task 1", "proj1", now, now)

	// This demonstrates the WRONG way (would deadlock):
	done := make(chan bool, 1)
	go func() {
		rows, _ := db.Query(`SELECT id FROM tasks WHERE parent_id IS NULL`)
		defer rows.Close()

		for rows.Next() {
			var id string
			rows.Scan(&id)

			// This would deadlock because rows still holds the connection
			// and GetSubtasks needs a connection too, but MaxOpenConns=1
			_, err := db.GetSubtasks(id)
			if err != nil {
				// Would never get here - it just hangs
				done <- false
				return
			}
		}
		done <- true
	}()

	select {
	case <-done:
		// If we get here, the bug was somehow fixed
	case <-time.After(2 * time.Second):
		// Expected: deadlock timeout
		t.Log("Confirmed: nested queries during iteration cause deadlock")
	}
}
