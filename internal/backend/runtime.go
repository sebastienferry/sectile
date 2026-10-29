package backend

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
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
	local        bool
	closing      bool
	workReady    atomic.Bool
	workAgent    atomic.Pointer[workAgentIdentity]
	closed       bool
}

type workAgentIdentity struct {
	userID   string
	deviceID string
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
	local := cfg.StoreRoot != ""
	if local {
		h.SetPullOnConnect(false)
	} else {
		h.TryPullLocalAgentTasks()
		database.StartAutoSync()
	}
	runtime := &Runtime{Database: database, Handler: h, stopInstance: stopInstance, stopEventBus: stopEventBus, storeLock: ownership, local: local}
	runtime.workReady.Store(!local)
	return runtime, nil
}

// HTTPHandler builds the backend HTTP stack. A Local runtime keeps all work
// routes closed until ActivateWork; profile and agent bootstrap remain open.
func (r *Runtime) HTTPHandler(mux *http.ServeMux) http.Handler {
	stack := WrapHTTP(mux, r.Handler, r.Database)
	if !r.local {
		return stack
	}
	return r.gateWork(stack)
}

// WaitForAgent waits for the expected authenticated workstation connection.
// The agent identity and WebSocket routes remain reachable while work is gated.
func (r *Runtime) WaitForAgent(ctx context.Context, userID, deviceID string) error {
	if r == nil || !r.local || userID == "" || deviceID == "" {
		return fmt.Errorf("local agent identity is required")
	}
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		r.closeMu.Lock()
		closing := r.closing || r.closed
		r.closeMu.Unlock()
		if closing {
			return fmt.Errorf("local backend is stopping")
		}
		if r.Handler.HasAgentConnection(userID, deviceID) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// ActivateWork opens Local work only after the expected agent has connected.
// Fleet starts normally in Open and does not use this transition.
func (r *Runtime) ActivateWork(userID, deviceID string) error {
	if r == nil || !r.local {
		return fmt.Errorf("work activation requires a Local backend")
	}
	r.closeMu.Lock()
	defer r.closeMu.Unlock()
	if r.closed || r.closing {
		return fmt.Errorf("local backend is stopping")
	}
	if !r.Handler.HasAgentConnection(userID, deviceID) {
		return fmt.Errorf("expected authenticated Local agent is not connected")
	}
	if r.workReady.Load() {
		identity := r.workAgent.Load()
		if identity == nil || identity.userID != userID || identity.deviceID != deviceID {
			return fmt.Errorf("Local work is owned by another agent")
		}
		return nil
	}
	r.Handler.SetPullOnConnect(true)
	r.workAgent.Store(&workAgentIdentity{userID: userID, deviceID: deviceID})
	r.workReady.Store(true)
	r.Database.StartAutoSync()
	r.Handler.PullAgentTasks(userID, deviceID)
	return nil
}

func (r *Runtime) readyForWork() bool {
	if !r.workReady.Load() {
		return false
	}
	identity := r.workAgent.Load()
	return identity != nil && r.Handler.HasAgentConnection(identity.userID, identity.deviceID)
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
	r.closing = true
	r.workReady.Store(false)
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
