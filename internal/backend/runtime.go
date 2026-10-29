package backend

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"tasks/internal/auth"
	"tasks/internal/db"
	"tasks/internal/handlers"
	"tasks/internal/storelock"
)

// Config contains the backend inputs resolved by the composition root.
// It does not read deployment environment variables or choose data paths.
type Config struct {
	Database         db.Config
	DataDir          string
	InstanceAddress  string
	IdentityProvider *auth.Provider
	// StoreRoot enables exclusive Local ownership before db.Open. Fleet leaves
	// it empty because PostgreSQL coordinates its own independent instances.
	StoreRoot string
}

// Runtime owns the database and its instance registration. The caller owns
// HTTP listeners, signals, and the decision to admit or drain requests.
type Runtime struct {
	Database     *db.DB
	Handler      *handlers.Handler
	stopInstance func()
	stopEventBus func()
	storeLock    *storelock.Lock
	closeMu      sync.Mutex
	closed       bool
}

// Open initializes the same backend services used by the standalone server.
// The composition root must establish any exclusive store ownership first.
func Open(cfg Config) (*Runtime, error) {
	var ownership *storelock.Lock
	var err error
	if cfg.StoreRoot != "" {
		if cfg.Database.Driver != db.DriverSQLite || !filepath.IsAbs(cfg.Database.Path) {
			return nil, fmt.Errorf("local backend requires an absolute SQLite database path")
		}
		relative, err := filepath.Rel(cfg.StoreRoot, cfg.Database.Path)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
			return nil, fmt.Errorf("local database must be inside its owned store root")
		}
		ownership, err = storelock.Acquire(cfg.StoreRoot)
		if err != nil {
			return nil, fmt.Errorf("own local backend store: %w", err)
		}
	}
	database, err := db.Open(cfg.Database)
	if err != nil {
		_ = ownership.Close()
		return nil, fmt.Errorf("open backend database: %w", err)
	}
	if database.Shared() {
		database.SetInstanceAddress(cfg.InstanceAddress)
	}
	stopInstance, err := database.StartInstance()
	if err != nil {
		return nil, errors.Join(fmt.Errorf("start backend instance: %w", err), closeFailedOpen(database, ownership))
	}
	stopEventBus, err := database.StartEventBus()
	if err != nil {
		stopInstance()
		return nil, errors.Join(fmt.Errorf("start backend event bus: %w", err), closeFailedOpen(database, ownership))
	}
	h := handlers.NewHandler(database)
	h.SetDataDir(cfg.DataDir)
	h.SetPullOnConnect(true)
	if cfg.IdentityProvider != nil {
		h.SetIdentityProvider(cfg.IdentityProvider)
	}
	h.TryPullLocalAgentTasks()
	database.StartAutoSync()
	return &Runtime{Database: database, Handler: h, stopInstance: stopInstance, stopEventBus: stopEventBus, storeLock: ownership}, nil
}

// closeFailedOpen retains Local ownership if a database worker cannot be
// joined. A later process must never open the same store while one survives.
func closeFailedOpen(database *db.DB, ownership *storelock.Lock) error {
	if err := database.CloseContext(context.Background()); err != nil {
		return fmt.Errorf("close failed backend startup: %w", err)
	}
	return ownership.Close()
}

// StopInstance releases the Fleet presence row after admission has stopped.
// It is idempotent so the command can preserve its existing drain order.
func (r *Runtime) StopInstance() {
	if r != nil && r.stopInstance != nil {
		r.stopInstance()
	}
}

// Close bounds shutdown for callers without an explicit owner deadline.
func (r *Runtime) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return r.CloseContext(ctx)
}

// CloseContext joins backend maintenance and database workers after HTTP has
// stopped. Local ownership remains held if any join or database close fails.
func (r *Runtime) CloseContext(ctx context.Context) error {
	if r == nil {
		return nil
	}
	r.closeMu.Lock()
	defer r.closeMu.Unlock()
	if r.closed {
		return nil
	}
	if err := r.Handler.StopBackground(ctx); err != nil {
		return fmt.Errorf("stop backend handlers: %w", err)
	}
	r.StopInstance()
	if r.stopEventBus != nil {
		r.stopEventBus()
	}
	if err := r.Database.CloseContext(ctx); err != nil {
		return err
	}
	if err := r.storeLock.Close(); err != nil {
		return err
	}
	r.closed = true
	return nil
}
