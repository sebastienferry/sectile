package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tasks/internal/agentconfig"
	"tasks/internal/testhome"
)

func initMockServer(t *testing.T, projectID string, skills []agentconfig.Skill) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/agent/projects":
			_ = json.NewEncoder(w).Encode(agentconfig.Projects{
				SchemaVersion: agentconfig.Version,
				Projects: []agentconfig.Project{
					{ID: projectID, Name: "Test Project", GitRemoteURL: "git@github.com:example/test.git"},
				},
			})
		case "/api/v1/agent/config":
			qProject := r.URL.Query().Get("projectId")
			if qProject != projectID {
				http.Error(w, `{"error":"project not found"}`, http.StatusBadRequest)
				return
			}
			_ = json.NewEncoder(w).Encode(agentconfig.Config{
				SchemaVersion: agentconfig.Version,
				ProjectID:     projectID,
				ProjectName:   "Test Project",
				AIProvider:    "agy",
				Skills:        skills,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestInitRequiresProvider(t *testing.T) {
	_, err := Init([]string{})
	if err == nil || !strings.Contains(err.Error(), "--provider is required") {
		t.Fatalf("expected --provider is required error, got: %v", err)
	}
}

func TestInitRejectsUnsupportedProvider(t *testing.T) {
	_, err := Init([]string{"--provider", "unknown-cli-provider"})
	if err == nil || !strings.Contains(err.Error(), "unsupported for provider") {
		t.Fatalf("expected unsupported provider error, got: %v", err)
	}
}

func TestInitRequiresServerURLAndToken(t *testing.T) {
	testhome.Temp(t)
	t.Setenv("REMOTE_URL", "")
	t.Setenv("TOKEN", "")

	_, err := Init([]string{"--provider", "agy"})
	if err == nil || (!strings.Contains(err.Error(), "no server URL") && !strings.Contains(err.Error(), "no API key")) {
		t.Fatalf("expected missing server URL or API key error, got: %v", err)
	}

	_, err = Init([]string{"--provider", "agy", "--url", "http://127.0.0.1:8080"})
	if err == nil || !strings.Contains(err.Error(), "no API key") {
		t.Fatalf("expected no API key error, got: %v", err)
	}
}

func TestInitBootstrapsMCPAndSkills(t *testing.T) {
	home := t.TempDir()
	testhome.Set(t, home)

	skills := []agentconfig.Skill{
		{ID: "specify", Directory: "specify-issue", Command: "/specify-issue", Content: "Specification instructions"},
		{ID: "implement", Directory: "code-issue", Command: "/code-issue", Content: "Implementation instructions"},
	}

	srv := initMockServer(t, "proj-123", skills)

	repoDir := filepath.Join(home, "repo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	out, err := InitContext(context.Background(), []string{
		"--provider", "agy",
		"--url", srv.URL,
		"--token", "test-token",
		"--project", "proj-123",
		"--repo", repoDir,
	})
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if !strings.Contains(out, "Successfully initialized agy") {
		t.Fatalf("unexpected output: %s", out)
	}
	if !strings.Contains(out, "Skills installed: 2") {
		t.Fatalf("output should report 2 skills installed: %s", out)
	}

	// Verify MCP file created
	mcpPath := filepath.Join(home, ".gemini", "config", "mcp_config.json")
	if _, err := os.Stat(mcpPath); err != nil {
		t.Fatalf("MCP config file not found at %s: %v", mcpPath, err)
	}

	// Verify skill files created
	skill1 := filepath.Join(home, ".gemini", "config", "skills", "specify-issue", "SKILL.md")
	if raw, err := os.ReadFile(skill1); err != nil || !strings.Contains(string(raw), "Specification instructions") {
		t.Fatalf("skill file missing or invalid at %s: %v, content: %s", skill1, err, string(raw))
	}
	skill2 := filepath.Join(home, ".gemini", "config", "skills", "code-issue", "SKILL.md")
	if raw, err := os.ReadFile(skill2); err != nil || !strings.Contains(string(raw), "Implementation instructions") {
		t.Fatalf("skill file missing or invalid at %s: %v, content: %s", skill2, err, string(raw))
	}
}

// A retired provider (#614) is refused before anything is registered.
func TestInitRefusesARetiredProvider(t *testing.T) {
	home := t.TempDir()
	testhome.Set(t, home)

	srv := initMockServer(t, "proj-cursor", nil)

	repoDir := filepath.Join(home, "repo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	for _, provider := range []string{"gemini", "cursor", "vibe"} {
		out, err := InitContext(context.Background(), []string{
			"--provider", provider,
			"--url", srv.URL,
			"--token", "cursor-token",
			"--project", "proj-cursor",
			"--repo", repoDir,
		})
		if err == nil || !strings.Contains(err.Error(), "unsupported") {
			t.Fatalf("%s: Init must refuse a retired provider: %v %s", provider, err, out)
		}
	}
	for _, path := range []string{".cursor/mcp.json", ".gemini/settings.json", ".vibe/config.toml"} {
		if _, err := os.Stat(filepath.Join(home, path)); !os.IsNotExist(err) {
			t.Fatalf("%s written for a retired provider: %v", path, err)
		}
	}
}

func TestInitPositionalProviderAndAutoDiscovery(t *testing.T) {
	home := t.TempDir()
	testhome.Set(t, home)

	skills := []agentconfig.Skill{
		{ID: "specify", Directory: "specify-issue", Command: "/specify-issue", Content: "Spec instructions", CommandContent: "Spec command $ARGUMENTS"},
	}

	srv := initMockServer(t, "proj-auto", skills)

	repoDir := filepath.Join(home, "repo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Test positional provider ("claude" without --provider flag) and project discovery
	out, err := InitContext(context.Background(), []string{
		"claude",
		"--url", srv.URL,
		"--token", "test-token",
		"--repo", repoDir,
	})
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if !strings.Contains(out, "Successfully initialized claude") {
		t.Fatalf("unexpected output: %s", out)
	}
	if !strings.Contains(out, "Skills installed: 1") {
		t.Fatalf("output should report 1 skill installed: %s", out)
	}

	// Verify Claude MCP file (.claude.json)
	claudeMCP := filepath.Join(home, ".claude.json")
	if _, err := os.Stat(claudeMCP); err != nil {
		t.Fatalf("claude MCP file not found at %s: %v", claudeMCP, err)
	}

	// Verify Claude skill file (.claude/skills/specify-issue/SKILL.md) substituted arguments
	claudeSkill := filepath.Join(home, ".claude", "skills", "specify-issue", "SKILL.md")
	raw, err := os.ReadFile(claudeSkill)
	if err != nil || !strings.Contains(string(raw), "Spec command $ARGUMENTS") {
		t.Fatalf("claude skill file missing or content wrong at %s: %v, content: %s", claudeSkill, err, string(raw))
	}
}

// The CLI has no loopback: a saved local choice is kept, the entry is left as
// it is and Written is cleared so the next agent start rewrites it (#716).
func TestInitContextKeepsLocalChoiceWithoutLoopback(t *testing.T) {
	home := t.TempDir()
	testhome.Set(t, home)
	srv := initMockServer(t, "proj-123", nil)
	repoDir := filepath.Join(home, "repo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := agentconfig.UpdateSettings(repoDir, func(s *agentconfig.Settings) error {
		s.MCPConnections = map[string]agentconfig.MCPConnection{"claude": {Target: "local", Transport: "http", Written: "earlier"}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	mcpPath := filepath.Join(home, ".claude.json")
	original := []byte(`{"mcpServers": {"sectile": {"type": "http", "url": "http://127.0.0.1:8091/mcp"}}}`)
	if err := os.WriteFile(mcpPath, original, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := InitContext(context.Background(), []string{"--provider", "claude", "--url", srv.URL, "--token", "test-token", "--project", "proj-123", "--repo", repoDir}); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if raw, err := os.ReadFile(mcpPath); err != nil || string(raw) != string(original) {
		t.Fatalf("registration changed: %s %v", raw, err)
	}
	settings, err := agentconfig.ReadSettings(repoDir)
	if choice := settings.MCPConnections["claude"]; err != nil || choice.Target != "local" || choice.Transport != "http" || choice.Written != "" {
		t.Fatalf("choice: %+v %v", choice, err)
	}
}
