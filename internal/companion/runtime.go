// Package companion owns the backend and workstation agent in one Local process.
package companion

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"tasks/internal/agent"
	"tasks/internal/backend"
	"tasks/internal/db"
	"tasks/internal/localruntime"
)

var ErrRestartRequested = errors.New("companion restart requested")

type Installation struct {
	Port          int    `json:"port"`
	Key           string `json:"key"`
	Token         string `json:"token"`
	DeviceID      string `json:"device_id"`
	AgentDeviceID string `json:"agent_device_id,omitempty"`
	UserID        string `json:"user_id"`
}

func readInstallation(root string) (Installation, error) {
	var value Installation
	raw, err := os.ReadFile(filepath.Join(root, "installation.json"))
	if err != nil {
		return value, err
	}
	if err = json.Unmarshal(raw, &value); err != nil {
		return value, err
	}
	if value.Port < 1 || value.Port > 65535 || len(value.Key) < 32 {
		return value, errors.New("invalid Local installation")
	}
	return value, nil
}

// Run consumes the desktop's existing installation manifest. The caller owns
// signals and must provide a private absolute root. Existing deployments are
// never discovered through home-directory or environment fallbacks.
func Run(parent context.Context, root string) (result error) {
	var restart atomic.Bool
	defer func() {
		if restart.Load() && result == nil && parent.Err() == nil {
			result = ErrRestartRequested
		}
	}()
	if !filepath.IsAbs(root) {
		return errors.New("companion data root must be absolute")
	}
	installation, err := readInstallation(root)
	if err != nil {
		return fmt.Errorf("read Local installation: %w", err)
	}
	address := fmt.Sprintf("127.0.0.1:%d", installation.Port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	defer listener.Close()
	if err := os.MkdirAll(filepath.Join(root, "server"), 0700); err != nil {
		return err
	}
	runtime, err := backend.Open(backend.Config{
		Database: db.SQLiteConfig(filepath.Join(root, "server", "tasks.db")),
		DataDir:  filepath.Join(root, "server"), StoreRoot: root,
	})
	if err != nil {
		return err
	}
	agentStopped := true
	defer func() {
		if agentStopped {
			result = errors.Join(result, runtime.Close())
		}
	}()
	mux := http.NewServeMux()
	backend.RegisterRoutes(mux, runtime.Handler)
	stack := runtime.HTTPHandler(mux)
	probeKey := installation.Key
	server := &http.Server{ReadHeaderTimeout: 10 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/local-runtime" {
			stack.ServeHTTP(w, r)
			return
		}
		if r.Method != http.MethodGet || r.Header.Get("Origin") != "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+probeKey)) != 1 {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(struct {
			localruntime.Metadata
			Ready     bool `json:"ready"`
			Companion bool `json:"companion"`
		}{localruntime.Info(), true, true})
	})}
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			_ = server.Close()
			result = errors.Join(result, err)
		}
	}()
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	// Tauri atomically replaces the manifest after profile sign-in and pairing.
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for installation.Token == "" {
		select {
		case <-ctx.Done():
			return nil
		case err := <-served:
			return fmt.Errorf("companion HTTP stopped: %w", err)
		case <-ticker.C:
			candidate, err := readInstallation(root)
			if err != nil {
				return err
			}
			if candidate.Port != installation.Port || candidate.Key != installation.Key {
				return errors.New("Local installation identity changed during bootstrap")
			}
			installation = candidate
		}
	}
	if installation.UserID == "" || installation.DeviceID == "" {
		return errors.New("paired Local installation identity is incomplete")
	}
	agentDeviceID := installation.AgentDeviceID
	if agentDeviceID == "" {
		agentDeviceID = installation.DeviceID
	}
	connected := make(chan struct{}, 1)
	finished := make(chan struct{})
	var agentErr error
	agentStopped = false
	go func() {
		agentErr = agent.RunEmbedded(ctx, agent.EmbeddedConfig{
			ServerURL: "http://" + address, Token: installation.Token, ProjectID: "all", DeviceID: agentDeviceID,
			DesktopInfo: filepath.Join(root, "agent-connection.json"), DesktopToken: installation.Key,
			SettingsRoot: filepath.Join(root, "agent"), RepoRoot: root,
			OnConnected: func() {
				select {
				case connected <- struct{}{}:
				default:
				}
			}, OnRestart: func() { restart.Store(true); cancel() },
		})
		close(finished)
	}()
	defer func() {
		runtime.BeginDrain()
		cancel()
		select {
		case <-finished:
			agentStopped = agentErr == nil
			result = errors.Join(result, agentErr)
		case <-time.After(15 * time.Second):
			result = errors.Join(result, errors.New("embedded agent shutdown deadline exceeded"))
		}
	}()
	startup, stopStartup := context.WithTimeout(ctx, 30*time.Second)
	defer stopStartup()
	select {
	case <-connected:
	case <-finished:
		return errors.Join(errors.New("embedded agent stopped during startup"), agentErr)
	case <-startup.Done():
		return startup.Err()
	case err := <-served:
		return err
	}
	if err := runtime.WaitForAgent(startup, installation.UserID, agentDeviceID); err != nil {
		return err
	}
	if err := runtime.ActivateWork(installation.UserID, agentDeviceID); err != nil {
		return err
	}
	select {
	case <-finished:
		if restart.Load() {
			return nil
		}
		return errors.Join(errors.New("embedded agent stopped"), agentErr)
	case <-ctx.Done():
		return nil
	case err := <-served:
		return err
	}
}
