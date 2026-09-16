package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"tasks/internal/agentconfig"
	"tasks/internal/models"
	"tasks/internal/runner"
	"time"
)

type desktopRun struct {
	Branch          string    `json:"branch,omitempty"`
	Kind            string    `json:"kind,omitempty"`
	Provider        string    `json:"provider,omitempty"`
	CancelRequested bool      `json:"cancelRequested,omitempty"`
	QueueSequence   uint64    `json:"queueSequence,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
	StartedAt       time.Time `json:"startedAt,omitzero"`
	Prompt          string    `json:"prompt,omitempty"`
	ID              string    `json:"id"`
	TaskID          string    `json:"taskId"`
	TaskKey         string    `json:"taskKey"`
	ProjectID       string    `json:"projectId"`
	Skill           string    `json:"skill"`
	SessionID       string    `json:"sessionId"`
	Directory       string    `json:"directory"`
	Status          string    `json:"status"`
}

func (d *agentDaemon) desktopHandler(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if r.Header.Get("Origin") != "" || d.desktopToken == "" || subtle.ConstantTimeCompare([]byte(token), []byte(d.desktopToken)) != 1 {
		http.Error(w, "Unauthorized", 401)
		return
	}
	if r.URL.Path == "/desktop/status" && r.Method == http.MethodGet {
		d.connMu.Lock()
		connected := d.conn != nil
		d.connMu.Unlock()
		settings, err := agentconfig.ReadSettings(d.localSettingsRoot())
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		disconnected := []string{}
		for id, value := range settings.DisconnectedProjects {
			if value {
				disconnected = append(disconnected, id)
			}
		}
		sort.Strings(disconnected)
		_ = json.NewEncoder(w).Encode(map[string]any{"connected": connected, "server": d.serverURL, "capabilities": []string{"git-diff", "create-task", "remove-project", "free-console"}, "disconnectedProjects": disconnected})
		return
	}
	if (r.URL.Path == "/desktop/restart" || r.URL.Path == "/desktop/shutdown") && r.Method == http.MethodPost {
		if !d.prepareMu.TryLock() {
			http.Error(w, "Project preparation or deployment is in progress", 409)
			return
		}
		defer d.prepareMu.Unlock()
		d.runsMu.Lock()
		defer d.runsMu.Unlock()
		if d.restartAgent == nil || d.shuttingDown {
			http.Error(w, "Restart unavailable", http.StatusConflict)
			return
		}
		for _, run := range d.runs {
			select {
			case <-run.exited:
			default:
				http.Error(w, "Stop active executions before restarting", http.StatusConflict)
				return
			}
		}
		d.shuttingDown = true
		d.restartRequested = r.URL.Path == "/desktop/restart"
		w.WriteHeader(http.StatusNoContent)
		d.restartAgent()
		return
	}
	if r.URL.Path == "/desktop/git-diff" {
		d.desktopGitDiff(w, r)
		return
	}
	if r.URL.Path == "/desktop/consoles" {
		d.desktopConsole(w, r)
		return
	}
	if r.URL.Path == "/desktop/create-task" {
		d.desktopCreateTask(w, r)
		return
	}
	if r.URL.Path == "/desktop/run-result" && r.Method == http.MethodGet {
		d.desktopRunResult(w, r)
		return
	}
	if r.URL.Path == "/desktop/tasks" {
		d.desktopTasks(w, r)
		return
	}
	if r.URL.Path == "/desktop/project" {
		d.desktopProject(w, r)
		return
	}
	if r.URL.Path == "/desktop/projects" {
		d.desktopProjects(w, r)
		return
	}
	if r.URL.Path == "/desktop/history" && r.Method == http.MethodDelete {
		d.runsMu.Lock()
		removed := []string{}
		for id, run := range d.runs {
			select {
			case <-run.exited:
				if d.terminalMgr != nil && run.desktop.SessionID != "" {
					_ = d.terminalMgr.CloseSession(run.desktop.SessionID)
				}
				delete(d.runs, id)
				removed = append(removed, id)
			default:
			}
		}
		d.runsMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"removed": removed})
		return
	}
	if r.URL.Path != "/desktop/runs" && r.URL.Path != "/desktop/stop" && r.URL.Path != "/desktop/terminal" {
		http.Error(w, "Unknown local agent endpoint", 404)
		return
	}
	id := r.URL.Query().Get("id")
	d.runsMu.Lock()
	if r.URL.Path == "/desktop/runs" && r.Method == http.MethodGet {
		runs := []desktopRun{}
		for key, run := range d.runs {
			entry := run.desktop
			entry.ID = key
			entry.QueueSequence = run.sequence
			entry.CancelRequested = run.canceled && (entry.Status == "queued" || entry.Status == "preparing" || entry.Status == "running")
			if entry.Status != "" {
				runs = append(runs, entry)
			}
		}
		d.runsMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(runs)
		return
	}
	run := d.runs[id]
	if run == nil {
		d.runsMu.Unlock()
		http.Error(w, "Run not found", 404)
		return
	}
	entry := run.desktop
	if r.URL.Path == "/desktop/stop" && r.Method == http.MethodPost {
		run.canceled = true
		d.runsMu.Unlock()
		select {
		case <-run.exited:
			// The native client may have already reported completion via MCP.
			_ = d.finishDesktopRun(context.Background(), entry.TaskID, id, "canceled", "Execution canceled")
			w.WriteHeader(http.StatusNoContent)
		case <-time.After(12 * time.Second):
			http.Error(w, "Exit not confirmed", 504)
		}
		return
	}
	d.runsMu.Unlock()
	if r.URL.Path == "/desktop/terminal" && r.Method == http.MethodGet {
		exists := false
		for _, session := range d.terminalMgr.ListSessions() {
			if session.ID == entry.SessionID {
				exists = true
				break
			}
		}
		if !exists {
			http.Error(w, "Console is not ready", 409)
			return
		}

		d.terminalMgr.HandleWebSocket(w, r, entry.SessionID, entry.Directory, nil)
		return
	}
	http.Error(w, "Not found", 404)
}

func (d *agentDaemon) writeDesktopInfo() error {
	if d.desktopInfo == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(d.desktopInfo), 0700); err != nil {
		return err
	}
	// Publish atomically so a companion never reads a partially written credential.
	raw, _ := json.Marshal(map[string]string{"url": d.agentURL, "token": d.desktopToken})
	file, err := os.CreateTemp(filepath.Dir(d.desktopInfo), ".agent-connection-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(raw); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), d.desktopInfo)
}

// Report process exit using the server's authenticated MCP endpoint.
func (d *agentDaemon) finishDesktopRun(ctx context.Context, taskID, runID, status, note string) error {
	if taskID == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "sectile-desktop-agent", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: d.serverURL + "/mcp", HTTPClient: agentHTTPClient(d.token)}, nil)
	if err != nil {
		return err
	}
	defer session.Close()
	if strings.TrimSpace(note) == "" {
		note = "Local console process exited"
	}
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "finish_run", Arguments: map[string]string{"taskKey": taskID, "runId": runID, "status": status, "note": note}})
	if err != nil {
		return err
	}
	if result.IsError {
		return fmt.Errorf("remote run completion was rejected: %v", result.Content)
	}
	return nil
}

func mustJSON(v any) string { raw, _ := json.Marshal(v); return string(raw) }

// desktopProjects keeps workstation paths in the local configuration only.
func (d *agentDaemon) desktopProjects(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodDelete {
		d.disconnectProject(w, r)
		return
	}
	if r.Method == http.MethodGet {
		projects, err := d.discoverProjects(r.Context())
		if err != nil {
			http.Error(w, err.Error(), 502)
			return
		}
		type entry struct {
			ID           string `json:"id"`
			Name         string `json:"name"`
			Path         string `json:"path"`
			Configured   bool   `json:"configured"`
			Disconnected bool   `json:"disconnected"`
		}
		entries := []entry{}
		base := d.repoRoot
		if base == "" {
			base, _ = os.Getwd()
			base = findRepoRoot(base)
		}
		settings, err := agentconfig.ReadSettings(base)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		for _, p := range projects.Projects {
			root, _, _ := d.localProjectRoot(r.Context(), agentconfig.Config{ProjectID: p.ID, GitRemoteURL: p.GitRemoteURL})
			mapped, configured := settings.Projects[p.ID]
			if root == "" {
				root = mapped
			}
			disconnected := settings.DisconnectedProjects[p.ID]
			if disconnected {
				root = ""
				configured = false
			}
			entries = append(entries, entry{p.ID, p.Name, root, configured || root != "", disconnected})
		}
		_ = json.NewEncoder(w).Encode(entries)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	var input struct {
		ProjectID          string  `json:"projectId"`
		Path               string  `json:"path"`
		AICommandTemplate  *string `json:"aiCommandTemplate"`
		InheritCommand     bool    `json:"inheritCommand"`
		InheritWorktrees   bool    `json:"inheritWorktrees"`
		InheritParallelism bool    `json:"inheritParallelism"`
		Parallelism        *int    `json:"parallelism"`
		UseWorktrees       *bool   `json:"useWorktrees"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&input) != nil || input.ProjectID == "" || !filepath.IsAbs(input.Path) {
		http.Error(w, "Project and absolute repository path required", 400)
		return
	}
	if _, err := d.fetchConfig(r.Context(), input.ProjectID, ""); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if _, err := gitLocal(r.Context(), input.Path, "rev-parse", "--show-toplevel"); err != nil {
		http.Error(w, "Select a local Git repository", 400)
		return
	}
	d.prepareMu.Lock()
	defer d.prepareMu.Unlock()
	root := d.repoRoot
	if root == "" {
		root, _ = os.Getwd()
		root = findRepoRoot(root)
	}
	overrides, err := agentconfig.ReadSettings(root)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if overrides.Projects == nil {
		overrides.Projects = map[string]string{}
	}
	if input.AICommandTemplate != nil || input.InheritCommand {
		if overrides.Commands == nil {
			overrides.Commands = map[string]string{}
		}
		command := ""
		if input.AICommandTemplate != nil {
			command = strings.TrimSpace(*input.AICommandTemplate)
		}
		if len(command) > 4096 {
			http.Error(w, "CLI command is too long", 400)
			return
		}
		if input.InheritCommand {
			command = ""
		}
		overrides.Commands[input.ProjectID] = command
	}
	if input.Parallelism != nil {
		if *input.Parallelism < 1 || *input.Parallelism > models.MaxParallelism {
			http.Error(w, fmt.Sprintf("Parallelism must be between 1 and %d", models.MaxParallelism), 400)
			return
		}
		if overrides.Parallelism == nil {
			overrides.Parallelism = map[string]int{}
		}
		overrides.Parallelism[input.ProjectID] = *input.Parallelism
	}
	if input.InheritParallelism {
		delete(overrides.Parallelism, input.ProjectID)
	}
	overrides.Projects[input.ProjectID] = input.Path
	if input.UseWorktrees != nil {
		if overrides.Worktrees == nil {
			overrides.Worktrees = map[string]bool{}
		}
		overrides.Worktrees[input.ProjectID] = *input.UseWorktrees
	}
	if input.InheritWorktrees {
		delete(overrides.Worktrees, input.ProjectID)
	}
	delete(overrides.DisconnectedProjects, input.ProjectID)
	if err := agentconfig.WriteSettings(overrides); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.WriteHeader(204)
}

func (d *agentDaemon) localSettingsRoot() string {
	root := d.repoRoot
	if root == "" {
		root, _ = os.Getwd()
		root = findRepoRoot(root)
	}
	return root
}

func (d *agentDaemon) disconnectProject(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		http.Error(w, "Project ID required", 400)
		return
	}
	if !d.prepareMu.TryLock() {
		http.Error(w, "Project preparation or configuration is in progress; retry removal", 409)
		return
	}
	defer d.prepareMu.Unlock()
	d.runsMu.Lock()
	defer d.runsMu.Unlock()
	for _, run := range d.runs {
		if run.desktop.ProjectID != id {
			continue
		}
		select {
		case <-run.exited:
		default:
			http.Error(w, "Stop active executions and wait for them to exit before removing this project", 409)
			return
		}
	}
	settings, err := agentconfig.ReadSettings(d.localSettingsRoot())
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if settings.DisconnectedProjects == nil {
		settings.DisconnectedProjects = map[string]bool{}
	}
	settings.DisconnectedProjects[id] = true
	delete(settings.Projects, id)
	delete(settings.Worktrees, id)
	delete(settings.Parallelism, id)
	delete(settings.Commands, id)
	if err := agentconfig.WriteSettings(settings); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Probe only loopback endpoints from the private discovery file.
func localAgentAvailable(file string) bool {
	raw, err := os.ReadFile(file)
	if err != nil {
		return false
	}
	var connection struct {
		URL   string
		Token string
	}
	if json.Unmarshal(raw, &connection) != nil || connection.Token == "" {
		return false
	}
	endpoint, err := url.Parse(connection.URL)
	if err != nil || endpoint.Scheme != "http" || endpoint.Hostname() != "127.0.0.1" || endpoint.User != nil {
		return false
	}
	req, err := http.NewRequest(http.MethodGet, connection.URL+"/desktop/status", nil)
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "Bearer "+connection.Token)
	client := &http.Client{Timeout: time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(req)
	if err != nil {
		return false
	}
	defer response.Body.Close()
	return response.StatusCode == http.StatusOK
}

// desktopProject exposes server metadata without allowing server configuration edits.
func (d *agentDaemon) desktopProject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Project required", 400)
		return
	}
	config, err := d.fetchConfig(r.Context(), id, "")
	if err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	d.prepareMu.Lock()
	defer d.prepareMu.Unlock()
	root, overrides, mappingErr := d.localProjectRoot(r.Context(), config)
	if r.Method == http.MethodGet {
		var project models.Project
		if err := d.readAPI(r.Context(), "/api/projects/"+url.PathEscape(id), &project); err != nil {
			http.Error(w, err.Error(), 502)
			return
		}
		effective := agentconfig.ApplyOverrides(config, overrides)
		_, worktreeOverride := overrides.Worktrees[id]
		_, parallelismOverride := overrides.Parallelism[id]
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"server": config, "monoRepo": project.MonoRepo, "path": root, "useWorktrees": effective.UseWorktrees, "configured": mappingErr == nil, "aiCommandTemplate": effective.AICommandTemplate, "commandOverride": overrides.Commands[id] != "", "worktreeOverride": worktreeOverride, "parallelismOverride": parallelismOverride, "parallelism": agentconfig.ExecutionLimit(id, effective.UseWorktrees, overrides, config.Parallelism)})
		return
	}
	if mappingErr != nil {
		http.Error(w, mappingErr.Error(), 400)
		return
	}
	// Do not change scaffolding underneath active native clients.
	d.runsMu.Lock()
	for _, run := range d.runs {
		select {
		case <-run.exited:
		default:
			d.runsMu.Unlock()
			http.Error(w, "Stop active executions before deploying project tooling", 409)
			return
		}
	}
	if d.shuttingDown {
		d.runsMu.Unlock()
		http.Error(w, "Agent is stopping", 409)
		return
	}
	d.runsMu.Unlock()
	switch r.URL.Query().Get("action") {
	case "skills":
		_, err = agentconfig.Scaffold(root, config)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "Server skills deployed"})
	case "framework":
		if config.SpecFramework != "openspec" && config.SpecFramework != "speckit" {
			http.Error(w, "No supported SDD framework configured", 400)
			return
		}
		result := runner.NewRunner().InstallSpecFramework(models.SpecFrameworkInstallRequest{Framework: config.SpecFramework, RepoPath: root, ProjectID: id, AIAgent: config.AIProvider})
		if result.Error != "" {
			http.Error(w, result.Error, 500)
			return
		}
		_ = json.NewEncoder(w).Encode(result)
	default:
		http.Error(w, "Unknown deployment action", 400)
	}
}

// desktopTasks browses server tasks and reuses the web dispatch path.
func (d *agentDaemon) desktopTasks(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("projectId")
	if projectID == "" {
		http.Error(w, "Project required", 400)
		return
	}
	config, err := d.fetchConfig(r.Context(), projectID, "")
	if err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	if r.Method == http.MethodGet {
		var tasks []models.Task
		q := url.Values{"projectId": {projectID}, "q": {r.URL.Query().Get("q")}}
		if err := d.readAPI(r.Context(), "/api/tasks?"+q.Encode(), &tasks); err != nil {
			http.Error(w, err.Error(), 502)
			return
		}
		if tasks == nil {
			tasks = []models.Task{}
		}
		if r.URL.Query().Get("launchable") == "true" {
			filtered := []models.Task{}
			for _, task := range tasks {
				if !desktopTaskFinished(task) {
					filtered = append(filtered, task)
				}
			}
			tasks = filtered
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(tasks)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	var input struct {
		TaskID  string
		SkillID string
		Prompt  string
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&input) != nil || input.TaskID == "" {
		http.Error(w, "Task and skill required", 400)
		return
	}
	var task models.Task
	if err := d.readAPI(r.Context(), "/api/tasks/"+url.PathEscape(input.TaskID), &task); err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	if desktopTaskFinished(task) {
		http.Error(w, "This task is finished. Reopen it on the server before launching an execution.", 409)
		return
	}
	if task.ProjectID != projectID {
		http.Error(w, "Task does not belong to project", 400)
		return
	}
	if !launchableSkill(config, input.SkillID, input.Prompt) {
		http.Error(w, "Unknown project skill", 400)
		return
	}
	if _, _, err := d.localProjectRoot(r.Context(), config); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	body := mustJSON(map[string]any{"skillId": input.SkillID, "prompt": input.Prompt})
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, d.serverURL+"/api/tasks/"+url.PathEscape(task.ID)+"/run-skill", strings.NewReader(body))
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := agentHTTPClient(d.token).Do(req)
	if err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	defer response.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(response.StatusCode)
	_, _ = io.Copy(w, io.LimitReader(response.Body, 1<<20))
}

func (d *agentDaemon) desktopCreateTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	var input struct {
		ProjectID   string
		Title       string
		Description string
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536)).Decode(&input) != nil || input.ProjectID == "" || strings.TrimSpace(input.Title) == "" {
		http.Error(w, "Project and title required", 400)
		return
	}
	if _, err := d.fetchConfig(r.Context(), input.ProjectID, ""); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	body := mustJSON(models.CreateTaskRequest{ProjectID: input.ProjectID, Title: strings.TrimSpace(input.Title), Description: input.Description, RequireRemoteCreation: true})
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, d.serverURL+"/api/tasks", strings.NewReader(body))
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := agentHTTPClient(d.token).Do(req)
	if err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	defer response.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(response.StatusCode)
	_, _ = io.Copy(w, io.LimitReader(response.Body, 1<<20))
}

// launchableSkill admits a project skill or one of the reserved identifiers.
// A discussion carries nothing, unlike custom instructions.
func launchableSkill(config agentconfig.Config, skillID, prompt string) bool {
	if skillID == "discuss" {
		return true
	}
	if skillID == "custom" {
		return strings.TrimSpace(prompt) != ""
	}
	for _, skill := range config.Skills {
		if skill.ID == skillID {
			return true
		}
	}
	return false
}

func desktopTaskFinished(task models.Task) bool {
	if task.Status == models.StatusFinished || task.Status == models.StatusDone {
		return true
	}
	for _, label := range task.Labels {
		if strings.EqualFold(strings.TrimPrefix(strings.TrimSpace(label), "#"), "finished") {
			return true
		}
	}
	return false
}

// Resolve the selected execution against server activity, independently of PTY exit.
func (d *agentDaemon) desktopRunResult(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	d.runsMu.Lock()
	run := d.runs[id]
	if run == nil {
		d.runsMu.Unlock()
		http.Error(w, "Run not found", 404)
		return
	}
	taskID, projectID := run.taskID, run.desktop.ProjectID
	d.runsMu.Unlock()
	if taskID == "" {
		http.Error(w, "Free consoles have no task result", http.StatusNotFound)
		return
	}
	var task models.Task
	if err := d.readAPI(r.Context(), "/api/tasks/"+url.PathEscape(taskID), &task); err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	if task.ID != taskID || task.ProjectID != projectID {
		http.Error(w, "Task does not match execution", 409)
		return
	}
	var activities []models.TaskActivity
	if err := d.readAPI(r.Context(), "/api/tasks/"+url.PathEscape(taskID)+"/activities", &activities); err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	var activity any
	for _, item := range activities {
		if item.ID == id && item.TaskID == taskID {
			activity = map[string]string{"id": item.ID, "taskId": item.TaskID, "skillId": item.SkillID, "status": item.Status}
			break
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"activity": activity, "task": map[string]any{"status": task.Status, "labels": task.Labels}})
}
