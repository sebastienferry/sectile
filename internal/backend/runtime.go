package backend

import (
	"fmt"
	"sync"

	"tasks/internal/auth"
	"tasks/internal/db"
	"tasks/internal/handlers"
)

// Config contains the backend inputs resolved by the composition root.
// It does not read deployment environment variables or choose data paths.
type Config struct {
	Database         db.Config
	DataDir          string
	InstanceAddress  string
	IdentityProvider *auth.Provider
}

// Runtime owns the database and its instance registration. The caller owns
// HTTP listeners, signals, and the decision to admit or drain requests.
type Runtime struct {
	Database     *db.DB
	Handler      *handlers.Handler
	stopInstance func()
	stopEventBus func()
	closeOnce    sync.Once
	closeErr     error
}

// Open initializes the same backend services used by the standalone server.
// The composition root must establish any exclusive store ownership first.
func Open(cfg Config) (*Runtime, error) {
	database, err := db.Open(cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("open backend database: %w", err)
	}
	if database.Shared() {
		database.SetInstanceAddress(cfg.InstanceAddress)
	}
	stopInstance, err := database.StartInstance()
	if err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("start backend instance: %w", err)
	}
	stopEventBus, err := database.StartEventBus()
	if err != nil {
		stopInstance()
		_ = database.Close()
		return nil, fmt.Errorf("start backend event bus: %w", err)
	}
	h := handlers.NewHandler(database)
	h.SetDataDir(cfg.DataDir)
	h.SetPullOnConnect(true)
	if cfg.IdentityProvider != nil {
		h.SetIdentityProvider(cfg.IdentityProvider)
	}
	h.TryPullLocalAgentTasks()
	database.StartAutoSync()
	return &Runtime{Database: database, Handler: h, stopInstance: stopInstance, stopEventBus: stopEventBus}, nil
}

// StopInstance releases the Fleet presence row after admission has stopped.
// It is idempotent so the command can preserve its existing drain order.
func (r *Runtime) StopInstance() {
	if r != nil && r.stopInstance != nil {
		r.stopInstance()
	}
}

// Close releases the database after its HTTP listener has stopped. Full
// worker joining and Local store locks are introduced in the lifecycle phase.
func (r *Runtime) Close() error {
	if r == nil {
		return nil
	}
	r.closeOnce.Do(func() {
		r.StopInstance()
		if r.stopEventBus != nil {
			r.stopEventBus()
		}
		r.closeErr = r.Database.Close()
	})
	return r.closeErr
}
