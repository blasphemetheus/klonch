package app

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dori/klonch/internal/db"
	"github.com/dori/klonch/internal/notify"
	"github.com/gofrs/flock"
)

// App holds the application state and dependencies
type App struct {
	DB       *db.DB
	Notifier *notify.Notifier
	DataDir  string
	lockFile *flock.Flock
}

// Config holds application configuration
type Config struct {
	DataDir string
	DBPath  string
}

// DefaultConfig returns the default application configuration
func DefaultConfig() *Config {
	dataDir := db.DefaultDataDir()
	return &Config{
		DataDir: dataDir,
		DBPath:  filepath.Join(dataDir, "klonch.db"),
	}
}

// New creates a new application instance
func New(cfg *Config) (*App, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	// Ensure data directory exists
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	app := &App{
		DataDir:  cfg.DataDir,
		Notifier: notify.NewNotifier(),
	}

	// Acquire lock to ensure single instance
	if err := app.acquireLock(); err != nil {
		return nil, err
	}

	// Open database
	database, err := db.Open(cfg.DBPath)
	if err != nil {
		app.releaseLock()
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	app.DB = database

	return app, nil
}

// acquireLock acquires an exclusive file lock to prevent multiple instances
func (a *App) acquireLock() error {
	lockPath := filepath.Join(a.DataDir, "klonch.lock")
	a.lockFile = flock.New(lockPath)

	locked, err := a.lockFile.TryLock()
	if err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}

	if !locked {
		return fmt.Errorf("another instance of klonch is already running")
	}

	return nil
}

// releaseLock releases the file lock
func (a *App) releaseLock() {
	if a.lockFile != nil {
		a.lockFile.Unlock()
	}
}

// Close cleans up application resources
func (a *App) Close() error {
	var errs []error

	if a.DB != nil {
		if err := a.DB.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close database: %w", err))
		}
	}

	a.releaseLock()

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}
