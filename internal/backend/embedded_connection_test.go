package backend_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"tasks/internal/agent"
	"tasks/internal/backend"
	"tasks/internal/db"
	"tasks/internal/handlers"
)

func TestEmbeddedAgentConnectsThroughLocalBootstrapGate(t *testing.T) {
	root := filepath.Join(t.TempDir(), "local")
	runtime, err := backend.Open(backend.Config{
		Database: db.SQLiteConfig(filepath.Join(root, "tasks.db")),
		DataDir:  root, StoreRoot: root,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := runtime.Close(); err != nil {
			t.Error(err)
		}
	}()
	userID, err := runtime.Database.UpsertUser("local|embedded", "embedded@example.com", "Embedded")
	if err != nil {
		t.Fatal(err)
	}
	key, credential, err := runtime.Database.CreateAPIKey(userID, "embedded-device", db.DefaultAPIKeyTTL)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	backend.RegisterRoutes(mux, runtime.Handler)
	server := httptest.NewServer(runtime.HTTPHandler(mux))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	finished := make(chan error, 1)
	connected := make(chan struct{}, 1)
	go func() {
		finished <- agent.RunEmbedded(ctx, agent.EmbeddedConfig{
			ServerURL: server.URL, Token: key, ProjectID: "all",
			DeviceID:     credential.ID,
			DesktopInfo:  filepath.Join(root, "agent-connection.json"),
			DesktopToken: "desktop-test-token",
			SettingsRoot: filepath.Join(root, "agent-settings"),
			RepoRoot:     root,
			OnConnected: func() {
				select {
				case connected <- struct{}{}:
				default:
				}
			},
		})
	}()
	waitCtx, stopWait := context.WithTimeout(context.Background(), 10*time.Second)
	defer stopWait()
	if err := runtime.WaitForAgent(waitCtx, userID, credential.ID); err != nil {
		select {
		case agentErr := <-finished:
			t.Fatalf("embedded agent stopped before connecting: %v; wait: %v", agentErr, err)
		default:
			t.Fatal(err)
		}
	}
	select {
	case <-connected:
	case <-waitCtx.Done():
		t.Fatal("embedded agent did not finish its connection setup")
	}
	if err := runtime.ActivateWork(userID, credential.ID); err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Timeout: 2 * time.Second}
	response, err := client.Get(server.URL + handlers.ReadyPath)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("Local runtime did not become ready: %d", response.StatusCode)
	}
	cancel()
	select {
	case err := <-finished:
		if err != nil {
			t.Fatalf("embedded agent did not stop cleanly: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("embedded agent did not stop")
	}
}
