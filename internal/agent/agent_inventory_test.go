package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"tasks/internal/agentconfig"
	"tasks/internal/agentprotocol"
	"tasks/internal/testhome"

	"github.com/gorilla/websocket"
)

func TestLocalProjectInventoryScopeWithdrawalAndRecovery(t *testing.T) {
	testhome.Temp(t)
	root := t.TempDir()
	mapped := t.TempDir()
	for _, dir := range []string{root, mapped} {
		if _, err := gitLocal(context.Background(), dir, "init"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := gitLocal(context.Background(), root, "remote", "add", "origin", "https://example.com/org/root.git"); err != nil {
		t.Fatal(err)
	}
	settings := agentconfig.Settings{ProjectSettings: map[string]agentconfig.ProjectSettings{"mapped": {Path: mapped}}, DisconnectedProjects: map[string]bool{"disconnected": true}}
	if err := agentconfig.WriteSettings(settings); err != nil {
		t.Fatal(err)
	}
	var fail atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail.Load() {
			w.WriteHeader(503)
			return
		}
		if r.URL.Path != "/api/v1/agent/projects" {
			w.WriteHeader(404)
			return
		}
		_ = json.NewEncoder(w).Encode(agentconfig.Projects{SchemaVersion: 1, Projects: []agentconfig.Project{{ID: "fallback", GitRemoteURL: "https://example.com/org/root.git"}, {ID: "mapped"}, {ID: "absent"}, {ID: "disconnected", GitRemoteURL: "https://example.com/org/root.git"}}})
	}))
	defer server.Close()
	d := &agentDaemon{repoRoot: root, link: serverLink{serverURL: server.URL, projectID: "all"}}
	check := func(wanted ...string) {
		t.Helper()
		p := d.projectInventory(context.Background())
		if !slices.Equal(p.ProjectIDs, wanted) {
			t.Fatalf("inventory=%v want=%v", p.ProjectIDs, wanted)
		}
		raw, _ := json.Marshal(p)
		if strings.Contains(string(raw), root) || strings.Contains(string(raw), mapped) {
			t.Fatal("local paths leaked")
		}
	}
	check("fallback", "mapped")
	d.link.projectID = "default"
	check("fallback", "mapped")
	d.link.projectID = "mapped"
	check("mapped")
	d.link.projectID = "all"
	if err := os.RemoveAll(filepath.Join(mapped, ".git")); err != nil {
		t.Fatal(err)
	}
	check("fallback")
	settings.DisconnectedProjects["fallback"] = true
	if err := agentconfig.WriteSettings(settings); err != nil {
		t.Fatal(err)
	}
	check()
	settings.DisconnectedProjects["fallback"] = false
	if err := agentconfig.WriteSettings(settings); err != nil {
		t.Fatal(err)
	}
	check("fallback")
	fail.Store(true)
	check()
	fail.Store(false)
	check("fallback")
	settingsPath, err := agentconfig.SettingsPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settingsPath, []byte(`{broken`), 0600); err != nil {
		t.Fatal(err)
	}
	check()
	if err := os.Remove(settingsPath); err != nil {
		t.Fatal(err)
	}
	if err := agentconfig.WriteSettings(settings); err != nil {
		t.Fatal(err)
	}
	check("fallback")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := d.projectInventory(ctx); len(got.ProjectIDs) != 0 {
		t.Fatal("canceled computation published projects")
	}
}

func TestInventoryRefreshWorkerAndConnectionCancellation(t *testing.T) {
	testhome.Temp(t)
	root := t.TempDir()
	if _, err := gitLocal(context.Background(), root, "init"); err != nil {
		t.Fatal(err)
	}
	if err := agentconfig.WriteSettings(agentconfig.Settings{ProjectSettings: map[string]agentconfig.ProjectSettings{"p": {Path: root}}}); err != nil {
		t.Fatal(err)
	}
	messages := make(chan agentprotocol.ProjectInventory, 10)
	var discoveryFailed atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/agent/projects" {
			if discoveryFailed.Load() {
				w.WriteHeader(503)
				return
			}
			_, _ = w.Write([]byte(`{"schemaVersion":1,"projects":[{"id":"p"}]}`))
			return
		}
		if r.URL.Path != "/relay" {
			w.WriteHeader(404)
			return
		}
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			var msg agentprotocol.Message
			if conn.ReadJSON(&msg) != nil {
				return
			}
			var p agentprotocol.ProjectInventory
			if json.Unmarshal(msg.Payload, &p) == nil {
				messages <- p
			}
		}
	}))
	defer server.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/relay", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	d := &agentDaemon{repoRoot: root, link: serverLink{serverURL: server.URL, projectID: "all", conn: conn}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d.sendProjectInventory(ctx, conn)
	next := func(wanted ...string) {
		t.Helper()
		select {
		case p := <-messages:
			if !slices.Equal(p.ProjectIDs, wanted) {
				t.Fatalf("snapshot=%v want=%v", p.ProjectIDs, wanted)
			}
		case <-time.After(time.Second):
			t.Fatal("inventory not refreshed")
		}
	}
	next("p")
	done := make(chan struct{})
	go func() { d.inventoryLoopEvery(ctx, conn, 100*time.Millisecond); close(done) }()
	discoveryFailed.Store(true)
	d.inventory.notify()
	next()
	discoveryFailed.Store(false)
	next("p")
	if err := os.RemoveAll(filepath.Join(root, ".git")); err != nil {
		t.Fatal(err)
	}
	next()
	cancel()
	<-done
	for len(messages) > 0 {
		<-messages
	}
	d.sendProjectInventory(ctx, conn)
	select {
	case <-messages:
		t.Fatal("closed worker wrote another snapshot")
	case <-time.After(150 * time.Millisecond):
	}
}
