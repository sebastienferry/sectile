package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tasks/internal/agentexec"
	"tasks/internal/agentprotocol"
	"tasks/internal/db"
	"tasks/internal/handlers"
	"tasks/internal/testsqlite"
)

// TestV076AgentBinaryConnectsToCurrentServer runs a released v0.7.6 agent
// process against the current server's actual WebSocket handler. Set
// SECTILE_V076_AGENT_BINARY to a v0.7.6 agent executable to enable it.
func TestV076AgentBinaryConnectsToCurrentServer(t *testing.T) {
	binary := os.Getenv("SECTILE_V076_AGENT_BINARY")
	if binary == "" {
		t.Skip("set SECTILE_V076_AGENT_BINARY to run the released-agent integration fixture")
	}
	if _, err := os.Stat(binary); err != nil {
		t.Fatalf("agent binary: %v", err)
	}

	root := t.TempDir()
	home := filepath.Join(root, "home")
	if err := os.MkdirAll(home, 0700); err != nil {
		t.Fatal(err)
	}
	database, err := testsqlite.New(t, filepath.Join(root, "server.db"), db.NewDB)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	h := handlers.NewHandler(database)
	mux := http.NewServeMux()
	mux.HandleFunc("/ws/agent-connect", h.HandleAgentConnect)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	cmd := agentexec.Hidden(exec.Command(binary,
		"--url", server.URL,
		"--token", defaultAgentKey(t, database),
		"--project", "default",
		"--device", "agent-v076-integration",
		"--desktop-info", filepath.Join(root, "agent-connection.json"),
		"--repo", root,
	))
	cmd.Dir = root
	cmd.Env = isolatedAgentEnvironment(home)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Start(); err != nil {
		t.Fatalf("start v0.7.6 agent: %v", err)
	}
	wait := make(chan error, 1)
	go func() { wait <- cmd.Wait() }()
	waitConsumed := false
	t.Cleanup(func() {
		if !waitConsumed {
			_ = cmd.Process.Signal(os.Interrupt)
			select {
			case <-wait:
			case <-time.After(3 * time.Second):
				_ = cmd.Process.Kill()
				<-wait
			}
		}
		h.CloseAgentConnections()
		if t.Failed() {
			t.Logf("v0.7.6 agent output:\n%s", output.String())
		}
	})

	session := defaultSession(t, database)
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case err := <-wait:
			waitConsumed = true
			t.Fatalf("v0.7.6 agent exited before registering: %v", err)
		default:
		}
		req := httptest.NewRequest(http.MethodGet, "/api/agent/status", nil)
		req.AddCookie(session)
		response := httptest.NewRecorder()
		h.HandleAgentStatus(response, req)
		var status struct {
			Agents []handlers.AgentConnInfo `json:"agents"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &status); err != nil {
			t.Fatalf("decode agent status: %v", err)
		}
		for _, agent := range status.Agents {
			if agent.DeviceID != "agent-v076-integration" {
				continue
			}
			if agent.AgentVersion != "v0.7.6" {
				t.Fatalf("connected agent version = %q, want v0.7.6", agent.AgentVersion)
			}
			if agent.Outdated == nil || *agent.Outdated {
				t.Fatalf("v0.7.6 operations were not accepted: %+v", agent)
			}
			for _, operation := range agentprotocol.Operations {
				if !containsOperation(agent.Operations, operation) {
					t.Errorf("connected v0.7.6 agent did not announce %q", operation)
				}
			}
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("current server did not register the v0.7.6 agent process")
}

func isolatedAgentEnvironment(home string) []string {
	var env []string
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
		if key == "HOME" || key == "USERPROFILE" || key == "APPDATA" || key == "LOCALAPPDATA" ||
			key == "TOKEN" || key == "REMOTE_URL" || strings.HasPrefix(key, "SECTILE_") {
			continue
		}
		env = append(env, item)
	}
	return append(env,
		"HOME="+home,
		"USERPROFILE="+home,
		"APPDATA="+filepath.Join(home, "AppData", "Roaming"),
		"LOCALAPPDATA="+filepath.Join(home, "AppData", "Local"),
	)
}

func containsOperation(operations []string, expected string) bool {
	for _, operation := range operations {
		if operation == expected {
			return true
		}
	}
	return false
}
