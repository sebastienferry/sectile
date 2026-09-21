package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"tasks/internal/agentconfig"
	"tasks/internal/agentmcp"
	"tasks/internal/db"
	"tasks/internal/handlers"
	"testing"
	"time"

	"tasks/internal/mcptest"
	"tasks/internal/models"
)

func TestAgentCommandQuotesPrompt(t *testing.T) {
	prompt := "hello 'world'\n$(touch /tmp/sectile-should-not-exist) `whoami` $HOME"
	for _, template := range []string{"printf '%s' {prompt}", `printf '%s' "{prompt}"`, "printf '%s' '{prompt}'"} {
		line, err := agentCommandLine("custom", template, "", prompt)
		if err != nil {
			t.Fatal(err)
		}
		out, err := exec.Command("sh", "-c", line).Output()
		if err != nil || string(out) != prompt {
			t.Fatalf("prompt changed: %q %v", out, err)
		}
	}
}

func TestLocalWorktreeCreationAndBranchGuard(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	for _, args := range [][]string{{"init"}, {"-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-m", "Initial"}} {
		if _, err := gitLocal(ctx, root, args...); err != nil {
			t.Fatal(err)
		}
	}
	branch := "feat/test-task"
	task := models.Task{Key: "#46", BranchName: &branch}
	path, got, err := ensureLocalWorktree(ctx, root, task, true)
	if err != nil || got != branch || path != filepath.Join(root, ".tasks/worktrees/#46") {
		t.Fatalf("prepare %s %s %v", path, got, err)
	}
	if _, _, err := ensureLocalWorktree(ctx, root, task, true); err != nil {
		t.Fatal(err)
	}
	branch = "feat/other"
	if _, _, err := ensureLocalWorktree(ctx, root, task, true); err == nil {
		t.Fatal("mismatched branch reused")
	}
	task.Key = "../../escape"
	if _, _, err := ensureLocalWorktree(ctx, root, task, true); err == nil {
		t.Fatal("escaped worktree path")
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		t.Fatal(err)
	}
}

func TestLocalWorktreeReusesAssignedMainCheckout(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	branch := "feat/existing-task"
	for _, args := range [][]string{{"init", "-b", branch}, {"-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-m", "Initial"}} {
		if _, err := gitLocal(ctx, root, args...); err != nil {
			t.Fatal(err)
		}
	}
	file := filepath.Join(root, "unfinished.txt")
	if err := os.WriteFile(file, []byte("work in progress"), 0600); err != nil {
		t.Fatal(err)
	}
	workDir, got, err := ensureLocalWorktree(ctx, root, models.Task{Key: "#46", BranchName: &branch}, true)
	if err != nil || workDir != root || got != branch {
		t.Fatalf("assigned checkout not reused: %s %s %v", workDir, got, err)
	}
	if content, err := os.ReadFile(file); err != nil || string(content) != "work in progress" {
		t.Fatalf("local work changed: %q %v", content, err)
	}
	if _, err := os.Stat(filepath.Join(root, ".tasks", "worktrees", "#46")); !os.IsNotExist(err) {
		t.Fatalf("unexpected duplicate worktree: %v", err)
	}
}

func TestGatewayForwardsMCPAndOwnCredential(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer daemon-token" {
			t.Errorf("wrong upstream token")
		}
		if r.URL.Path != "/mcp" {
			t.Errorf("wrong path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()
	d := &agentDaemon{link: serverLink{serverURL: upstream.URL, token: "daemon-token"}}
	if err := d.startLocalProxy(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer d.loopback.server.Close()
	// Local callers present the workstation API key, the same credential the
	// gateway forwards upstream; anything else is refused before proxying.
	req, _ := http.NewRequest("POST", d.loopback.url+"/mcp", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer daemon-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("proxy %d", resp.StatusCode)
	}
	req, _ = http.NewRequest("POST", d.loopback.url+"/mcp", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer caller-token")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("unknown caller %d, want 401", resp.StatusCode)
	}

	req, _ = http.NewRequest("POST", d.loopback.url+"/mcp", nil)
	req.Header.Set("Origin", "https://evil.example")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Fatalf("origin %d", resp.StatusCode)
	}
}

// The test executable acts as the stdio child so this exercises real process
// pipes without requiring a prebuilt Sectile binary or a database in the child.
func TestMCPStdioHelper(t *testing.T) {
	if os.Getenv("SECTILE_MCP_HELPER") != "1" {
		return
	}
	if err := agentmcp.Run(context.Background(), nil); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}

func TestMCPStdioBridge(t *testing.T) {
	database, err := db.NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	h := handlers.NewHandler(database)
	upstream := httptest.NewServer(h.MCPHandler())
	defer upstream.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	d := &agentDaemon{link: serverLink{serverURL: upstream.URL, token: "test-token"}}
	if err := d.startLocalProxy(ctx); err != nil {
		t.Fatal(err)
	}
	defer d.loopback.server.Close()
	task, err := database.CreateTask(models.CreateTaskRequest{ProjectID: "default", Title: "Local agent MCP integration", Description: "Read this description through the local agent", Status: models.StatusToClarify})
	if err != nil {
		t.Fatal(err)
	}
	connect := func() *mcp.ClientSession {
		t.Helper()
		command := exec.Command(os.Args[0], "-test.run=^TestMCPStdioHelper$")
		command.Env = append(os.Environ(), "SECTILE_MCP_HELPER=1", "SECTILE_AGENT_URL="+d.loopback.url, "SECTILE_AGENT_TOKEN="+d.link.token)
		session, err := mcp.NewClient(&mcp.Implementation{Name: "stdio-test", Version: "1"}, nil).Connect(ctx, &mcp.CommandTransport{Command: command}, nil)
		if err != nil {
			t.Fatal(err)
		}

		return session
	}
	session := connect()
	defer session.Close()
	mcptest.AssertNaming(t, ctx, session, database, task, connect)
	list, err := session.ListTools(ctx, nil)
	if err != nil || len(list.Tools) != 10 {
		t.Fatalf("stdio discovery %v %v", list, err)
	}
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "list_tasks", Arguments: map[string]any{"projectId": "default"}})
	if err != nil || result.IsError {
		t.Fatalf("stdio call %v %v", result, err)
	}
	for _, call := range []struct {
		name string
		args map[string]any
	}{
		{"add_comment", map[string]any{"taskKey": task.ID, "body": "Comment through local MCP"}},
		{"transition_stage", map[string]any{"taskKey": task.ID, "stage": "clarified", "note": "Verified via local agent"}},
		{"get_task", map[string]any{"taskKey": task.ID}},
	} {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: call.name, Arguments: call.args})
		if err != nil || result.IsError {
			t.Fatalf("%s: %v %v", call.name, result, err)
		}
		if call.name == "get_task" {
			raw, _ := json.Marshal(result)
			for _, want := range []string{"Read this description through the local agent", "Comment through local MCP"} {
				if !strings.Contains(string(raw), want) {
					t.Fatalf("missing %s", want)
				}
			}
		}
	}
}

func TestDispatchPreparesFromAPIContract(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	for _, args := range [][]string{{"init"}, {"-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-m", "Initial"}} {
		if _, err := gitLocal(ctx, root, args...); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, ".taskflow"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".taskflow/agent.json"), []byte(`{"aiProvider":"claude","terminal":"pty"}`), 0644); err != nil {
		t.Fatal(err)
	}
	config := agentconfig.Config{SchemaVersion: 1, ProjectID: "remote-project", UseWorktrees: true, AIProvider: "agy", Skills: []agentconfig.Skill{{ID: "implement", Directory: "code-issue", Command: "/code-issue", Content: "remote instructions"}}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Error("API credential missing")
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/agent/config" {
			_ = json.NewEncoder(w).Encode(config)
			return
		}
		remotePath := "/server/unavailable/checkout"
		_ = json.NewEncoder(w).Encode(models.Task{ID: "task", Key: "TASK-46", ProjectID: "remote-project", RepoPath: &remotePath, WorktreePath: &remotePath})
	}))
	defer srv.Close()
	d := &agentDaemon{repoRoot: root, loopback: loopbackServer{url: "http://127.0.0.1:8091"}, link: serverLink{serverURL: srv.URL, token: "token", projectID: "remote-project"}}
	effective, path, branch, task, err := d.prepareDispatch(ctx, "TASK-46")
	if err != nil {
		t.Fatal(err)
	}
	if task.ID != "task" || task.Key != "TASK-46" || effective.AIProvider != "claude" || effective.ExternalTerminalCommand != "pty" || branch != "feat/task-46" || !strings.HasPrefix(path, root) {
		t.Fatalf("invalid execution config %+v %s %s", effective, path, branch)
	}
	// The override selects Claude, so the skills land in its user configuration.
	if _, err := os.Stat(filepath.Join(os.Getenv("HOME"), ".claude/skills/code-issue/SKILL.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(path, ".agents/skills/code-issue/SKILL.md")); !os.IsNotExist(err) {
		t.Fatal("the checkout must receive no managed skill")
	}
	for _, template := range []string{`printf '%s\000' {repoPath} {branchName} {issueKey} {prompt}`, `printf '%s\000' "{repoPath}" '{branchName}' {issueKey} {prompt}`} {
		effective.AICommandTemplate = template
		line, err := dispatchCommand(effective, task.ID, "implement", "implement", "", "", models.SkillModeInteractive, "", agentCommandContext{Task: task, Branch: branch, Directory: path})
		if err != nil {
			t.Fatal(err)
		}
		args := shellArguments(t, "sh", line)
		if len(args) != 4 || args[0] != path || args[1] != branch || args[2] != "TASK-46" || args[3] != "/code-issue task" {
			t.Fatalf("local context: %#v", args)
		}
	}
	config.SchemaVersion = 99
	if _, _, _, _, err := d.prepareDispatch(ctx, "TASK-46"); err == nil {
		t.Fatal("unknown remote contract version accepted")
	}
}

func TestExternalTerminalCommandWithoutSkill(t *testing.T) {
	config := agentconfig.Config{AIProvider: "custom", AICommandTemplate: "/bin/sh {prompt}"}
	command, err := dispatchCommand(config, "TASK-46", "", "open_terminal", "", "", models.SkillModeInteractive, "")
	if err != nil || !strings.Contains(command, "/bin/sh") {
		t.Fatalf("plain launch rejected: %q %v", command, err)
	}
	explicit := "printf 'custom command'"
	command, err = dispatchCommand(config, "TASK-46", "", "open_terminal", "", explicit, models.SkillModeInteractive, "")
	if err != nil || command != explicit {
		t.Fatalf("explicit command changed: %q %v", command, err)
	}
	if _, err = dispatchCommand(config, "TASK-46", "missing", "implement", "", "", models.SkillModeInteractive, ""); err == nil {
		t.Fatal("invalid workflow skill accepted")
	}
	config.AIProvider = "claude"
	config.AICommandTemplate = "claude {prompt}"
	config.Skills = []agentconfig.Skill{{ID: "implement", Directory: "code-issue", Command: "/code-issue"}}
	command, err = dispatchCommand(config, "TASK-46", "implement", "open_terminal", "", "", models.SkillModeInteractive, "")
	if err != nil || !strings.Contains(command, "/code-issue TASK-46") {
		t.Fatalf("skill launch: %q %v", command, err)
	}
}

func TestNativePickupBootstrapAndLaunch(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, provider := range []string{"codex", "claude"} {
		t.Run(provider, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			d := &agentDaemon{link: serverLink{serverURL: "http://sectile.example.test:8090", token: "sectile_test_key"}}
			config := agentconfig.Config{AIProvider: provider, Skills: []agentconfig.Skill{{ID: "pickup-issue", Directory: "pickup-issue", Command: "/pickup-issue"}}}
			if err := d.bootstrapLocalMCP(&config); err != nil {
				t.Fatal(err)
			}
			file := ".codex/config.toml"
			if provider == "claude" {
				file = ".claude.json"
			}
			// The registration addresses the server with the key, never a
			// local gateway, so it outlives this agent process.
			raw, err := os.ReadFile(filepath.Join(home, file))
			if err != nil || !strings.Contains(string(raw), d.link.serverURL) || !strings.Contains(string(raw), d.link.token) || strings.Contains(string(raw), "8091") {
				t.Fatalf("native MCP bootstrap: %s %v", raw, err)
			}
			command, err := dispatchCommand(config, "#48", "pickup-issue", "pickup-issue", "", "", models.SkillModeInteractive, "")
			if err != nil || command != provider+" '/pickup-issue #48'" {
				t.Fatalf("pickup launch: %q %v", command, err)
			}
		})
	}
}

func TestTerminalContractPrecedence(t *testing.T) {
	d := &agentDaemon{terminal: terminalChoice{app: "terminal"}}
	c := agentconfig.Config{ExternalTerminalCommand: "iterm"}
	if got := d.dispatchTerminal(c, ""); got != "iterm" {
		t.Fatal(got)
	}
	c = agentconfig.ApplyOverrides(c, agentconfig.Overrides{Terminal: "ghostty"})
	if got := d.dispatchTerminal(c, ""); got != "ghostty" {
		t.Fatal(got)
	}
	if got := d.dispatchTerminal(c, "warp"); got != "warp" {
		t.Fatal(got)
	}
	d.terminal.explicit = true
	if got := d.dispatchTerminal(c, "warp"); got != "terminal" {
		t.Fatal(got)
	}
	d.terminal.explicit = false
	c.ExternalTerminalCommand = "pty"
	if got := d.dispatchTerminal(c, ""); got != "pty" {
		t.Fatal(got)
	}
}

func TestConfigFetchIsFreshAndReportsServerError(t *testing.T) {
	var version int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		version++
		if version == 3 {
			w.WriteHeader(400)
			_, _ = w.Write([]byte(`{"error":"project not found: wrong-id"}`))
			return
		}
		_ = json.NewEncoder(w).Encode(agentconfig.Config{SchemaVersion: 1, ProjectID: "project", AIProvider: "codex", Description: fmt.Sprintf("version %d", version)})
	}))
	defer server.Close()
	d := &agentDaemon{link: serverLink{serverURL: server.URL, token: "token"}}
	first, err := d.fetchConfig(context.Background(), "project", "")
	if err != nil {
		t.Fatal(err)
	}
	second, err := d.fetchConfig(context.Background(), "project", "")
	if err != nil {
		t.Fatal(err)
	}
	if first.Description == second.Description {
		t.Fatal("stale configuration reused")
	}
	_, err = d.fetchConfig(context.Background(), "wrong-id", "")
	if err == nil || !strings.Contains(err.Error(), "project not found: wrong-id") {
		t.Fatalf("missing API diagnostic: %v", err)
	}
}

func TestInvalidProjectFailsBeforeAgentRegistration(t *testing.T) {
	registrations := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ws/agent-connect" {
			registrations++
			t.Error("invalid project registered")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"error":"project not found: taskativ"}`))
	}))
	defer srv.Close()
	d := &agentDaemon{repoRoot: t.TempDir(), link: serverLink{serverURL: srv.URL, token: "test", projectID: "taskativ"}}
	err := d.connect(context.Background())
	if err == nil || !strings.Contains(err.Error(), "project not found: taskativ") {
		t.Fatalf("missing initialization error: %v", err)
	}
	if registrations != 0 {
		t.Fatal("announced an agent that could not initialize")
	}
}

func TestDiscoverProjects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/agent/projects" || r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("unexpected discovery request: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"schemaVersion":1,"projects":[{"id":"project-a","name":"A"},{"id":"project-b","name":"B"}]}`))
	}))
	defer srv.Close()
	daemon := &agentDaemon{link: serverLink{serverURL: srv.URL, token: "test-token"}}
	projects, err := daemon.discoverProjects(context.Background())
	if err != nil || len(projects.Projects) != 2 {
		t.Fatalf("projects: %+v, %v", projects, err)
	}
}

func TestNativeAdjustmentAliasesAndReconciliation(t *testing.T) {
	c := agentconfig.Config{AIProvider: "custom", AICommandTemplate: "/bin/echo {prompt}", Skills: []agentconfig.Skill{{ID: "adjust", Directory: "adjust-issue", Command: "/adjust-issue"}}}
	for _, id := range []string{"adjust", "adjust-issue", "review"} {
		line, err := dispatchCommand(c, "task-61", id, "", "", "", models.SkillModeInteractive, "")
		if err != nil || !strings.Contains(line, "adjust-issue") || !strings.Contains(line, "Never create or replace a PR") {
			t.Fatalf("%s: %s %v", id, line, err)
		}
	}
	c.Skills[0].RequiresReconciliation = true
	if _, err := dispatchCommand(c, "task-61", "review", "", "", "", models.SkillModeInteractive, ""); err == nil {
		t.Fatal("unreconciled legacy customization launched")
	}
}

func TestNativeCreatePRDoesNotInvokeAdjustment(t *testing.T) {
	c := agentconfig.Config{AIProvider: "custom", AICommandTemplate: "/bin/echo {prompt}", Skills: []agentconfig.Skill{{ID: "create_pr", Directory: "create-pr", Command: "/create-pr"}}}
	for _, id := range []string{"create_pr", "create-pr"} {
		line, err := dispatchCommand(c, "task-61", id, "", "", "", models.SkillModeInteractive, "")
		if err != nil || !strings.Contains(line, "create-pr") || strings.Contains(line, "Never create or replace a PR") {
			t.Fatalf("%s: %s %v", id, line, err)
		}
	}
}

func TestDiscussionLaunchesTheProviderAlone(t *testing.T) {
	config := agentconfig.Config{
		AIProvider: "custom", AICommandTemplate: "/bin/sh {prompt}",
		Skills: []agentconfig.Skill{{ID: "implement", Directory: "code-issue", Command: "/code-issue"}},
	}
	command, err := dispatchCommand(config, "TASK-46", "discuss", "discuss", "", "", models.SkillModeInteractive, "")
	if err != nil || command != "'/bin/sh'" {
		t.Fatalf("discussion is not a bare launch: %q %v", command, err)
	}
	// A dispatch prompt exists for skills; a discussion must not inherit it.
	command, err = dispatchCommand(config, "TASK-46", "discuss", "discuss", "Remote execution runId: 42", "", models.SkillModeInteractive, "")
	if err != nil || strings.Contains(command, "42") || strings.Contains(command, "TASK-46") {
		t.Fatalf("discussion carried a prompt: %q %v", command, err)
	}
	if _, err = dispatchCommand(config, "TASK-46", "discussion", "discussion", "", "", models.SkillModeInteractive, ""); err == nil {
		t.Fatal("unknown identifier accepted as a discussion")
	}
}
