package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"tasks/internal/agentconfig"
	"tasks/internal/agentmcp"
	"tasks/internal/db"
	"tasks/internal/handlers"
	"tasks/internal/testhome"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"tasks/internal/mcptest"
	"tasks/internal/models"
	"tasks/internal/runner"
	"tasks/internal/skills"
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
	path, got, err := ensureLocalWorktree(ctx, root, task, true, "")
	if err != nil || got != branch || path != filepath.Join(root, ".tasks/worktrees/issue-46") {
		t.Fatalf("prepare %s %s %v", path, got, err)
	}
	if _, _, err := ensureLocalWorktree(ctx, root, task, true, ""); err != nil {
		t.Fatal(err)
	}
	// A key path sitting on another branch no longer refuses the launch: the
	// assigned branch is nowhere, so a worktree is created beside the stale one.
	branch = "feat/other"
	beside, got, err := ensureLocalWorktree(ctx, root, task, true, "")
	if err != nil || got != branch {
		t.Fatalf("stale key path refused the launch: %s %s %v", beside, got, err)
	}
	if beside == filepath.Join(root, ".tasks/worktrees/issue-46") {
		t.Fatalf("new worktree collided with the stale path: %s", beside)
	}
	if current, err := gitLocal(ctx, beside, "branch", "--show-current"); err != nil || current != branch {
		t.Fatalf("worktree beside the stale path is on %s: %v", current, err)
	}
	task.Key = "../../escape"
	if _, _, err := ensureLocalWorktree(ctx, root, task, true, ""); err == nil {
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
	workDir, got, err := ensureLocalWorktree(ctx, root, models.Task{Key: "#46", BranchName: &branch}, true, "")
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

// A task whose branch the server never named still resolves to a branch derived
// from its key. When the main checkout already sits on it, git refuses a second
// worktree and the dispatch used to fail before a console ever existed.
func TestLocalWorktreeReusesMainCheckoutForDerivedBranch(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	for _, args := range [][]string{{"init", "-b", "feat/281"}, {"-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-m", "Initial"}} {
		if _, err := gitLocal(ctx, root, args...); err != nil {
			t.Fatal(err)
		}
	}
	for _, task := range []models.Task{{Key: "#281"}, {Key: "#281", BranchName: new(string)}} {
		workDir, got, err := ensureLocalWorktree(ctx, root, task, true, "")
		if err != nil || workDir != root || got != "feat/281" {
			t.Fatalf("derived branch not reused: %s %s %v", workDir, got, err)
		}
		if _, err := os.Stat(filepath.Join(root, ".tasks", "worktrees", "#281")); !os.IsNotExist(err) {
			t.Fatalf("unexpected duplicate worktree: %v", err)
		}
	}
}

func TestGatewayForwardsMCPAndOwnCredential(t *testing.T) {
	testhome.Temp(t)
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

// upstreamAgentKey issues the workstation key a test agent reaches the server
// with. The legacy open mode, where any nonempty token named the implicit user,
// is gone, so the upstream only opens to a key it actually issued.
func upstreamAgentKey(t *testing.T, database *db.DB) string {
	t.Helper()
	if err := database.EnsureUser(db.ImplicitUserID); err != nil {
		t.Fatalf("ensure user: %v", err)
	}
	key, _, err := database.CreateAPIKey(db.ImplicitUserID, "test-workstation", db.DefaultAPIKeyTTL)
	if err != nil {
		t.Fatalf("create api key: %v", err)
	}
	return key
}

// stdioBridgeBudget bounds the stdio bridge test. The test starts eighteen
// bridge processes, one per legacy name plus the main session, each a fresh
// copy of the test binary. That takes about two seconds on a workstation under
// -race, but the whole test took 38 seconds on a loaded CI runner, where a
// fixed fifteen seconds failed while the calls were still progressing. Two
// minutes still turns a real hang into a failure naming the stuck call, well
// before the package timeout, which is honoured when it is closer.
func stdioBridgeBudget(t *testing.T) time.Duration {
	budget := 2 * time.Minute
	if deadline, ok := t.Deadline(); ok {
		// Leave room to report the failure before the test binary is killed.
		if left := time.Until(deadline) - 10*time.Second; left < budget {
			budget = max(left, time.Second)
		}
	}
	return budget
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
	ctx, cancel := context.WithTimeout(context.Background(), stdioBridgeBudget(t))
	defer cancel()
	d := &agentDaemon{link: serverLink{serverURL: upstream.URL, token: upstreamAgentKey(t, database)}}
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
		// Under -race every process sleeps a second before exiting
		// (atexit_sleep_ms), and Close waits for the child. With one child
		// per legacy name that alone nearly spends the budget, so CI timed out.
		command.Env = append(os.Environ(), "SECTILE_MCP_HELPER=1", "SECTILE_AGENT_URL="+d.loopback.url, "SECTILE_AGENT_TOKEN="+d.link.token,
			"GORACE="+strings.TrimSpace(os.Getenv("GORACE")+" atexit_sleep_ms=0"))
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
	if err != nil || len(list.Tools) != 20 {
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
	testhome.Temp(t)
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
	homeBefore := treeHash(t, os.Getenv("HOME"), filepath.Join(".config", "sectile", "runs"))
	effective, path, branch, task, err := d.prepareDispatch(ctx, "TASK-46")
	if err != nil {
		t.Fatal(err)
	}
	if task.ID != "task" || task.Key != "TASK-46" || effective.AIProvider != "claude" || effective.ExternalTerminalCommand != "pty" || branch != "feat/task-46" || !strings.HasPrefix(path, root) {
		t.Fatalf("invalid execution config %+v %s %s", effective, path, branch)
	}
	// A dispatch installs nothing (#267), not even for the provider it runs:
	// no skill, no MCP registration, no setting.
	if homeAfter := treeHash(t, os.Getenv("HOME"), filepath.Join(".config", "sectile", "runs")); homeAfter != homeBefore {
		t.Fatal("a dispatch wrote into the user configuration")
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
	testhome.Temp(t)
	for _, provider := range []string{"codex", "claude"} {
		t.Run(provider, func(t *testing.T) {
			home := t.TempDir()
			testhome.Set(t, home)
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

// The terminal is the workstation's own (#305): the project section, then the
// workstation defaults, then the agent's default, then detection. The value
// picked for the action and an explicit --terminal outrank them.
func TestTerminalPrecedence(t *testing.T) {
	testhome.Temp(t)
	d := &agentDaemon{repoRoot: t.TempDir(), terminal: terminalChoice{app: "terminal"}}
	if got := d.resolveTerminalForProject(context.Background(), "p", ""); got != "terminal" {
		t.Fatal(got)
	}
	if err := agentconfig.WriteSettings(agentconfig.Settings{
		Defaults:        agentconfig.Defaults{Execution: agentconfig.Execution{Terminal: "ghostty"}},
		ProjectSettings: map[string]agentconfig.ProjectSettings{"p": {Execution: agentconfig.Execution{Terminal: "iterm"}}},
	}); err != nil {
		t.Fatal(err)
	}
	if got := d.resolveTerminalForProject(context.Background(), "p", ""); got != "iterm" {
		t.Fatal(got)
	}
	if got := d.resolveTerminalForProject(context.Background(), "other", ""); got != "ghostty" {
		t.Fatal(got)
	}
	if got := d.resolveTerminalForProject(context.Background(), "p", "warp"); got != "warp" {
		t.Fatal(got)
	}
	d.terminal.explicit = true
	if got := d.resolveTerminalForProject(context.Background(), "p", ""); got != "terminal" {
		t.Fatal(got)
	}
}

func TestEditorPrecedence(t *testing.T) {
	if got := editorFor(agentconfig.Settings{}, ""); got != "code" {
		t.Fatal(got)
	}
	if got := editorFor(agentconfig.Settings{}, "zed"); got != "zed" {
		t.Fatal("an older server's editor must still open:", got)
	}
	if got := editorFor(agentconfig.Settings{Defaults: agentconfig.Defaults{EditorCommand: "cursor"}}, "zed"); got != "cursor" {
		t.Fatal("the workstation's editor must win:", got)
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

// A ticket of several projects runs for the project its launch chose (#741):
// the configuration is asked for that project and that ticket together.
func TestTheAgentFetchesTheConfigOfTheDispatchedProject(t *testing.T) {
	var query url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.Query()
		projectID := query.Get("projectId")
		if projectID == "" {
			projectID = "delivery"
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(agentconfig.Config{SchemaVersion: agentconfig.Version, ProjectID: projectID, AIProvider: "codex"})
	}))
	defer server.Close()
	d := &agentDaemon{link: serverLink{serverURL: server.URL, token: "token"}}

	config, err := d.fetchConfig(context.Background(), "bidder", "GODE-12")
	if err != nil {
		t.Fatal(err)
	}
	if query.Get("projectId") != "bidder" || query.Get("taskKey") != "GODE-12" || config.ProjectID != "bidder" {
		t.Fatalf("asked %v, got the configuration of %q", query, config.ProjectID)
	}
	if _, err := d.fetchConfig(context.Background(), "", "GODE-12"); err != nil {
		t.Fatal(err)
	}
	if query.Has("projectId") {
		t.Fatalf("a task alone must not name a project: %v", query)
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
	_, err := d.connect(context.Background())
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
		// The contract only sets guardrails: correcting, checking and pushing are the skill's call.
		if !strings.Contains(line, "Preserve work on failure") || !strings.Contains(line, "Never merge, approve, close the task") {
			t.Fatalf("%s: contract lost a guardrail: %s", id, line)
		}
		for _, work := range []string{"commit and push", "build/lint/test", "Review the complete branch", "reconcile"} {
			if strings.Contains(line, work) {
				t.Fatalf("%s: contract prescribes %q: %s", id, work, line)
			}
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

// A discussion and a bare terminal open the engine with the task's other
// folders, when its option for them is attested (#676).
func TestDiscussionLaunchCarriesTheTaskFolders(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake engines are POSIX scripts")
	}
	bin := t.TempDir()
	for _, name := range []string{"claude", "codex", "agy", "my-cli"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	dirs := []string{"/repo/b", "/notes with space"}
	for _, tt := range []struct {
		provider, template string
		folders            bool
	}{
		{"claude", "", true}, {"codex", "", true}, {"agy", "", false}, {"custom", "my-cli {prompt}", false},
	} {
		config := agentconfig.Config{AIProvider: tt.provider, AICommandTemplate: tt.template}
		bare, err := dispatchCommand(config, "TASK-1", "discuss", "discuss", "", "", models.SkillModeInteractive, "")
		if err != nil {
			t.Fatal(err)
		}
		for _, launch := range []struct{ skill, action string }{{"discuss", "discuss"}, {"", "open_terminal"}} {
			line, err := dispatchCommand(config, "TASK-1", launch.skill, launch.action, "", "", models.SkillModeInteractive, "", agentCommandContext{AddDirs: dirs})
			if err != nil {
				t.Fatal(err)
			}
			want := bare
			if tt.folders {
				want = bare + ` --add-dir='/repo/b' --add-dir='/notes with space'`
			}
			if line != want {
				t.Errorf("%s %s: %q, want %q", tt.provider, launch.action, line, want)
			}
		}
		if line, _ := dispatchCommand(config, "TASK-1", "discuss", "discuss", "", "", models.SkillModeInteractive, "", agentCommandContext{}); line != bare {
			t.Errorf("%s without folders: %q, want %q", tt.provider, line, bare)
		}
	}
}

// The failure this ticket is about: the assigned branch lives in a worktree at
// a path nobody would think to probe, while .tasks/worktrees/<key> is occupied
// by an unrelated branch. The launch must land in the tree that holds the work.
func TestLocalWorktreeResolvesBranchWhereverItLives(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	for _, args := range [][]string{{"init", "-b", "main"}, {"-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-m", "Initial"}} {
		if _, err := gitLocal(ctx, root, args...); err != nil {
			t.Fatal(err)
		}
	}
	elsewhere := filepath.Join(t.TempDir(), "unrelated-path")
	branch := "claude/clarify-issue-workflow-bbb85f"
	if _, err := gitLocal(ctx, root, "worktree", "add", "-b", branch, elsewhere, "HEAD"); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(root, ".tasks", "worktrees", "#296")
	if _, err := gitLocal(ctx, root, "worktree", "add", "-b", "feat/296", stale, "HEAD"); err != nil {
		t.Fatal(err)
	}

	workDir, got, err := ensureLocalWorktree(ctx, root, models.Task{Key: "#296", BranchName: &branch}, true, "")
	if err != nil || got != branch {
		t.Fatalf("branch not resolved where it lives: %s %s %v", workDir, got, err)
	}
	if !sameDirectory(workDir, elsewhere) {
		t.Fatalf("resolved %s, want the worktree at %s", workDir, elsewhere)
	}
	// The stale path is a warning, not a move: it keeps its own branch.
	if current, err := gitLocal(ctx, stale, "branch", "--show-current"); err != nil || current != "feat/296" {
		t.Fatalf("stale worktree disturbed: %s %v", current, err)
	}
}

// A detached worktree carries no branch and must never be matched as one, or a
// launch would land in a tree checked out at an arbitrary commit.
func TestWorktreeForBranchIgnoresDetachedWorktrees(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	for _, args := range [][]string{{"init", "-b", "main"}, {"-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-m", "Initial"}} {
		if _, err := gitLocal(ctx, root, args...); err != nil {
			t.Fatal(err)
		}
	}
	detached := filepath.Join(t.TempDir(), "detached")
	if _, err := gitLocal(ctx, root, "worktree", "add", "--detach", detached, "HEAD"); err != nil {
		t.Fatal(err)
	}
	if path, err := worktreeForBranch(ctx, root, "feat/nowhere"); err != nil || path != "" {
		t.Fatalf("a branch that lives nowhere resolved to %q: %v", path, err)
	}
	if path, err := worktreeForBranch(ctx, root, "main"); err != nil || !sameDirectory(path, root) {
		t.Fatalf("main checkout not resolved: %q %v", path, err)
	}
}

// A branch derived from the task key is written back onto the task, once, so a
// later launch resolves the same branch instead of deriving it again against a
// record that has since been assigned one.
func TestDerivedBranchIsRecordedOnceOnTheTask(t *testing.T) {
	assigned := "feat/already-there"
	blank := "   "
	worktrees := agentconfig.Config{UseWorktrees: true}
	for _, c := range []struct {
		name   string
		config agentconfig.Config
		task   models.Task
		branch string
		want   string
	}{
		{"no branch on the task", worktrees, models.Task{Key: "#308"}, "feat/308", "feat/308"},
		{"blank branch on the task", worktrees, models.Task{Key: "#308", BranchName: &blank}, "feat/308", "feat/308"},
		{"branch already assigned", worktrees, models.Task{Key: "#308", BranchName: &assigned}, assigned, ""},
		{"project without worktrees", agentconfig.Config{}, models.Task{Key: "#308"}, "main", ""},
		{"no branch resolved", worktrees, models.Task{Key: "#308"}, "", ""},
	} {
		if got := derivedBranchToRecord(c.config, c.task, c.branch); got != c.want {
			t.Fatalf("%s: recorded %q, want %q", c.name, got, c.want)
		}
	}

	var patched []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.EscapedPath() != "/api/tasks/%23308" {
			t.Errorf("unexpected call %s %s", r.Method, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		patched = append(patched, string(body))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	d := &agentDaemon{link: serverLink{serverURL: srv.URL, token: "token"}}
	if err := d.patchTask(context.Background(), "#308", map[string]string{"branchName": "feat/308"}); err != nil {
		t.Fatal(err)
	}
	if len(patched) != 1 || !strings.Contains(patched[0], `"branchName":"feat/308"`) {
		t.Fatalf("task update: %#v", patched)
	}

	refused := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer refused.Close()
	d = &agentDaemon{link: serverLink{serverURL: refused.URL, token: "token"}}
	if err := d.patchTask(context.Background(), "#308", map[string]string{"branchName": "feat/308"}); err == nil {
		t.Fatal("a refused update reported success")
	}
}

// sync_config registers Claude as a managed remote HTTP choice, so a new key
// reaches ~/.claude.json at the next refresh (#716).
func TestBootstrapLocalMCPRecordsClaudeDefault(t *testing.T) {
	home := testhome.Temp(t)
	d := &agentDaemon{repoRoot: t.TempDir(), link: serverLink{serverURL: "https://sectile.example.test", token: "first-key"}}
	config := agentconfig.Config{AIProvider: "claude"}
	if err := d.bootstrapLocalMCP(&config); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	settings, err := agentconfig.ReadSettings(d.localSettingsRoot())
	want := agentconfig.MCPConnection{Target: "remote", Transport: "http", Written: mcpFingerprint(d.link.serverURL, "first-key", executable)}
	if err != nil || settings.MCPConnections["claude"] != want {
		t.Fatalf("choice: %+v %v", settings.MCPConnections, err)
	}
	d.link.token = "second-key"
	if err := d.refreshMCPConnections(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(home, ".claude.json"))
	if err != nil || !strings.Contains(string(raw), "Bearer second-key") || strings.Contains(string(raw), "first-key") {
		t.Fatalf("key not refreshed: %s %v", raw, err)
	}
}

// A foreign command (skillCommands) replaces the whole skill, so its launch
// prompt carries the Sectile stage contract: /plan-jira still advances the card
// (#732).
func TestForeignCommandCarriesTheStageContract(t *testing.T) {
	stage, _ := skills.StageSkillByID("clarify")
	contract := skills.StageLaunchContract(stage)
	c := agentconfig.Config{Skills: []agentconfig.Skill{{ID: "clarify", Directory: "clarify-issue", Command: "/plan-jira", CommandOverridden: true}}}
	for name, contexts := range map[string][]agentCommandContext{
		"command choice": {{Skill: &skillChoice{Kind: skillKindCommand, Command: "plan-jira", Directory: "clarify-issue"}}},
		"no choice":      nil,
	} {
		// The dispatch names the skill by its ID, by its directory, or only
		// through its action: the contract is the matched skill's each time.
		for _, named := range []struct{ skillID, action string }{{"clarify", "clarify"}, {"clarify-issue", "clarify"}, {"plan-jira", "clarify"}} {
			prompt, _, err := dispatchPrompt(c, "T-1", named.skillID, named.action, "", contexts)
			if err != nil || !strings.HasPrefix(prompt, "/plan-jira T-1") || !strings.Contains(prompt, contract) || !strings.Contains(prompt, "transition_stage") {
				t.Fatalf("%s, %s: %s %v", name, named.skillID, prompt, err)
			}
			if !strings.Contains(prompt, "## Specifications workspace") {
				t.Fatalf("%s, %s: the prompt does not say where the issue artefacts go: %s", name, named.skillID, prompt)
			}
		}
	}
}

// A catalogue command runs the installed Sectile skill, which carries the
// contract itself, and a custom skill's file does too: no stage contract is
// added to either (#732).
func TestCatalogueCommandAddsNoStageContract(t *testing.T) {
	c := agentconfig.Config{Skills: []agentconfig.Skill{{ID: "clarify", Directory: "clarify-issue", Command: "/clarify-issue"}}}
	prompt, _, err := dispatchPrompt(c, "T-1", "clarify", "clarify", "", []agentCommandContext{{Skill: &skillChoice{Kind: skillKindDirect, Command: "clarify-issue"}}})
	if err != nil || strings.Contains(prompt, "Sectile stage contract") {
		t.Fatalf("catalogue command: %s %v", prompt, err)
	}
	c.Skills[0].Command, c.Skills[0].CommandOverridden = "/plan-jira", true
	file := filepath.Join(t.TempDir(), "run-1", "clarify-issue", "SKILL.md")
	prompt, _, err = dispatchPrompt(c, "T-1", "clarify", "clarify", "", []agentCommandContext{{Skill: &skillChoice{Kind: skillKindCustom, File: file}}})
	if err != nil || strings.Contains(prompt, "Sectile stage contract") {
		t.Fatalf("custom skill: %s %v", prompt, err)
	}
}

// A foreign adjust command gets the stage contract and still exactly one
// adjustment contract, and the runId line stays the launch's own (#732).
func TestForeignAdjustCommandKeepsOneAdjustmentContract(t *testing.T) {
	c := agentconfig.Config{Skills: []agentconfig.Skill{{ID: "adjust", Directory: "adjust-issue", Command: "/fix-review", CommandOverridden: true}}}
	// The dispatch may name the skill by its directory: the contracts are the
	// matched skill's all the same.
	for _, skillID := range []string{"adjust", "adjust-issue"} {
		prompt, _, err := dispatchPrompt(c, "T-1", skillID, "adjust", "Remote execution runId: run-1", []agentCommandContext{{Skill: &skillChoice{Kind: skillKindCommand, Command: "fix-review"}}})
		if err != nil {
			t.Fatal(err)
		}
		if strings.Count(prompt, runner.AdjustmentContract) != 1 || strings.Count(prompt, "Sectile stage contract") != 1 || strings.Count(prompt, "Remote execution runId") != 1 {
			t.Fatalf("%s contracts: %s", skillID, prompt)
		}
	}
}
