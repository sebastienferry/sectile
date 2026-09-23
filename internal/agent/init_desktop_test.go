package agent

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tasks/internal/agentconfig"
	"tasks/internal/testhome"
)

func TestDesktopInitialization(t *testing.T) {
	for _, tc := range []struct {
		name, provider, fault string
		code                  int
		mcp, skills           string
	}{
		{"success and retry", "claude", "", 200, "success", "success"},
		{"MCP only", "cursor", "", 200, "success", "skipped"},
		{"invalid provider", "custom", "", 400, "", ""},
		{"missing provider", "", "", 400, "", ""},
		{"busy", "claude", "busy", 409, "", ""},
		{"unmapped", "claude", "unmapped", 400, "", ""},
		{"MCP failure", "claude", "mcp", 200, "failed", "not_run"},
		{"skills failure", "claude", "skills", 200, "success", "failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := testhome.Temp(t)
			root := t.TempDir()
			if _, err := gitLocal(context.Background(), root, "init"); err != nil {
				t.Fatal(err)
			}
			skills := []agentconfig.Skill{{ID: "implement", Directory: "code-issue", Content: "Fresh server skill"}}
			server := initMockServer(t, "project", skills)
			d := &agentDaemon{repoRoot: root, link: serverLink{serverURL: server.URL, token: "test-token", projectID: "project"}, loopback: loopbackServer{desktopToken: "private"}}
			if tc.fault == "unmapped" {
				d.link.projectID = "other"
			}
			if tc.fault == "busy" {
				d.queue.runs = map[string]*controlledRun{"active": {exited: make(chan struct{})}}
			}
			if tc.fault == "mcp" {
				if err := os.WriteFile(filepath.Join(home, ".claude.json"), []byte("invalid json"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if tc.fault == "skills" {
				if err := os.MkdirAll(filepath.Join(home, ".claude"), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(home, ".claude", "skills"), []byte("blocked"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			for attempt := 0; attempt < 2; attempt++ {
				if attempt == 1 {
					skills[0].Content = "Updated server skill"
				}
				req := httptest.NewRequest("POST", "/desktop/project?id=project&action=initialize&provider="+tc.provider, nil)
				req.Header.Set("Authorization", "Bearer private")
				res := httptest.NewRecorder()
				d.desktopHandler(res, req)
				if res.Code != tc.code {
					t.Fatalf("status %d: %s", res.Code, res.Body.String())
				}
				if tc.code != 200 {
					continue
				}
				var result initializationResult
				if err := json.Unmarshal(res.Body.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
				if result.MCP.Status != tc.mcp || result.Skills.Status != tc.skills {
					t.Fatalf("unexpected result: %+v", result)
				}
				if result.Success != (tc.fault == "") {
					t.Fatalf("incorrect success: %+v", result)
				}
				if tc.skills == "success" {
					content, err := os.ReadFile(filepath.Join(home, ".claude", "skills", "code-issue", "SKILL.md"))
					if err != nil || !strings.Contains(string(content), skills[0].Content) {
						t.Fatalf("skill content: %s, %v", content, err)
					}
				}
			}
			if _, err := os.Stat(filepath.Join(home, ".gemini", "config", "mcp_config.json")); !os.IsNotExist(err) {
				t.Fatalf("server default provider initialized: %v", err)
			}
			if tc.code != 200 || tc.fault == "mcp" {
				if _, err := os.Stat(filepath.Join(home, ".claude", "skills", "code-issue", "SKILL.md")); !os.IsNotExist(err) {
					t.Fatalf("skills changed on refused attempt: %v", err)
				}
			}
		})
	}
}

func TestInitializationOverridesAdditionalServerProviders(t *testing.T) {
	home := testhome.Temp(t)
	config := agentconfig.Config{SchemaVersion: agentconfig.Version, AIProvider: "agy", SetupProviders: []string{"agy", "codex"}, Skills: []agentconfig.Skill{{ID: "implement", Directory: "code-issue", Content: "Selected provider only"}}}
	d := &agentDaemon{link: serverLink{serverURL: "https://example.test", token: "test-token"}}
	result, err := d.initializeProvider(t.TempDir(), config, " CLAUDE ")
	if err != nil || !result.Success {
		t.Fatalf("%+v: %v", result, err)
	}
	for _, dir := range []string{".gemini/config/skills", ".agents/skills"} {
		if _, err := os.Stat(filepath.Join(home, dir)); !os.IsNotExist(err) {
			t.Fatalf("unexpected additional provider %s: %v", dir, err)
		}
	}
}

func TestInitializationPreservesOtherProviderSkills(t *testing.T) {
	home := testhome.Temp(t)
	root := t.TempDir()
	config := agentconfig.Config{SchemaVersion: agentconfig.Version, Skills: []agentconfig.Skill{{ID: "implement", Directory: "code-issue", Content: "Original"}}}
	d := &agentDaemon{link: serverLink{serverURL: "https://example.test", token: "test-token"}}
	for _, provider := range []string{"codex", "claude", "cursor"} {
		if _, err := d.initializeProvider(root, config, provider); err != nil {
			t.Fatal(err)
		}
	}
	for _, dir := range []string{".agents/skills", ".claude/skills"} {
		content, err := os.ReadFile(filepath.Join(home, dir, "code-issue", "SKILL.md"))
		if err != nil || !strings.Contains(string(content), "Original") {
			t.Fatalf("other provider skills lost at %s: %s %v", dir, content, err)
		}
	}
	// A refresh still retires removed skills for the selected provider alone.
	config.Skills = nil
	if _, err := d.initializeProvider(root, config, "claude"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude/skills/code-issue/SKILL.md")); !os.IsNotExist(err) {
		t.Fatalf("removed selected skill retained: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".agents/skills/code-issue/SKILL.md")); err != nil {
		t.Fatalf("other provider skill retired: %v", err)
	}
}
