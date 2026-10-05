package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"tasks/internal/agentconfig"
	"tasks/internal/models"
)

// runFoldersCapability tells the desktop this agent serves
// /desktop/run-folder (#676).
const runFoldersCapability = "run-folders"

// runFoldersTerminalsCapability tells the desktop that /desktop/run-folder also
// serves a free console and a run detached to the native terminal (#689).
const runFoldersTerminalsCapability = "run-folders-terminals"

// When a folder attached from a run reaches it.
const (
	// appliesNextTurn: a conversation reads the project's folders at each turn.
	appliesNextTurn = "next-turn"
	// appliesNow: the folder was typed into the running Claude Code session.
	appliesNow = "now"
	// appliesNextLaunch: the run is given it when it is launched again.
	appliesNextLaunch = "next-launch"
)

// How long a session's output must stay quiet before /add-dir is typed into
// it, and how long the agent waits for that at most. Past the cap the line is
// typed anyway: Claude Code queues what is typed while it works.
var (
	runFolderQuiet    = time.Second
	runFolderQuietCap = 10 * time.Second
	// typedLineSubmitDelay separates a typed line from the Enter that submits
	// it, so the prompt does not read both as one paste.
	typedLineSubmitDelay = 150 * time.Millisecond
)

// runFolderAnswer is what the desktop shows once a folder is attached from a
// run: the repository it became the folder of, if any, and when the run sees it.
type runFolderAnswer struct {
	MappedAs  string `json:"mappedAs,omitempty"`
	Typed     bool   `json:"typed"`
	AppliesAt string `json:"appliesAt"`
}

// desktopRunFolder attaches a folder to the project of a conversation, of a
// running ticket discussion or of a running free console, through the same
// checks as the project settings (#676, #689). Nothing reaches the server. A
// conversation sees the folder from its next turn; a Claude Code discussion or
// console has /add-dir typed into it, so it sees the folder at once and keeps
// its context. A run detached to a native terminal is typed into all the same:
// the terminal only attaches to the session the agent owns.
//
// POST {runId, path} answers runFolderAnswer, or the reason of a refusal.
func (d *agentDaemon) desktopRunFolder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var input struct {
		RunID string `json:"runId"`
		Path  string `json:"path"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&input) != nil || strings.TrimSpace(input.RunID) == "" {
		http.Error(w, "Execution and folder required", http.StatusBadRequest)
		return
	}
	d.queue.mu.Lock()
	run := d.queue.runs[input.RunID]
	if run == nil {
		d.queue.mu.Unlock()
		http.Error(w, "Execution not found", http.StatusNotFound)
		return
	}
	ended := run.restored || run.canceled
	select {
	case <-run.exited:
		ended = true
	default:
	}
	conversation := run.desktop.Conversation && run.conversation != nil
	live := !run.desktop.Conversation && run.desktop.SessionID != "" && run.desktop.Status == "running"
	terminalRun := live && (models.NormalizeSkillID(run.desktop.Skill) == "discuss" || run.isConsole())
	projectID, sessionID, provider := run.desktop.ProjectID, run.desktop.SessionID, run.interactiveProvider
	d.queue.mu.Unlock()
	if ended {
		http.Error(w, "This execution has ended", http.StatusConflict)
		return
	}
	if !conversation && !terminalRun {
		http.Error(w, "A folder is added from a conversation, a running ticket discussion or a Project prompt", http.StatusConflict)
		return
	}
	config, err := d.fetchConfig(r.Context(), projectID, "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	path := strings.TrimSpace(input.Path)
	mappedAs, status, err := d.attachFolder(r, config, path)
	if err != nil {
		http.Error(w, err.Error(), status)
		return
	}
	answer := runFolderAnswer{MappedAs: mappedAs, AppliesAt: appliesNextLaunch}
	switch {
	case conversation:
		answer.AppliesAt = appliesNextTurn
	case d.typeAddDir(r.Context(), sessionID, provider, filepath.Clean(path)):
		answer.Typed, answer.AppliesAt = true, appliesNow
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(answer)
}

// typeAddDir types /add-dir and the folder into a Claude Code session, once
// the session has settled. It types nothing into another engine, whose command
// for it is not attested, nor a path that would not arrive as one line. The
// folder is attached either way; false says the run sees it at its next
// launch.
func (d *agentDaemon) typeAddDir(ctx context.Context, sessionID, provider, path string) bool {
	if provider != "claude" || !typeablePath(path) || d.terminal.manager == nil {
		return false
	}
	if !d.terminal.manager.WaitQuiet(ctx, sessionID, runFolderQuiet, runFolderQuietCap) {
		return false
	}
	// Claude Code reads its prompt in raw mode, where Enter is a carriage
	// return: a newline typed with the text would be read as part of it.
	if d.terminal.manager.SendInput(sessionID, "/add-dir "+claudePromptPath(path)) != nil {
		return false
	}
	time.Sleep(typedLineSubmitDelay)
	return d.terminal.manager.SendInput(sessionID, "\r") == nil
}

// addDirToTaskRuns types /add-dir and path into every live Claude Code session
// this agent runs for the task (#737), so a worktree prepared mid-run in a
// folder the session was not launched with can be written to. The typing
// waits for the session to settle, which a session waiting on the tool call
// that asked for the worktree does not do before the cap: it runs in the
// background, and the answer only says a session was found.
func (d *agentDaemon) addDirToTaskRuns(task models.Task, path string) bool {
	if d.terminal.manager == nil || !typeablePath(path) {
		return false
	}
	var sessions []string
	d.queue.mu.Lock()
	for _, run := range d.queue.runs {
		if run.taskID != task.ID && run.desktop.TaskID != task.ID && (task.Key == "" || run.desktop.TaskKey != task.Key) {
			continue
		}
		ended := run.restored || run.canceled
		select {
		case <-run.exited:
			ended = true
		default:
		}
		provider := run.interactiveProvider
		if provider == "" {
			provider = strings.ToLower(strings.TrimSpace(run.desktop.Provider))
		}
		if ended || run.desktop.Conversation || run.desktop.Headless || run.desktop.SessionID == "" || run.desktop.Status != "running" || provider != "claude" {
			continue
		}
		sessions = append(sessions, run.desktop.SessionID)
	}
	d.queue.mu.Unlock()
	for _, sessionID := range sessions {
		go d.typeAddDir(context.Background(), sessionID, "claude", path)
	}
	return len(sessions) > 0
}

// typeablePath says whether a path can be typed into a terminal as it is: a
// control character would act on the session instead of being read as text.
func typeablePath(path string) bool {
	if path == "" {
		return false
	}
	for _, r := range path {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

// claudePromptPath is a path as Claude Code's /add-dir reads it: as it is when
// it holds no whitespace and no quote, else in double quotes, with an inner
// quote or backslash escaped.
func claudePromptPath(path string) string {
	if !strings.ContainsAny(path, " \t\"") {
		return path
	}
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(path) + `"`
}

// discussionProvider is the engine a ticket discussion opens, which the run
// records so a folder attached from it can be typed in; empty for every other
// launch but a free console, which records its own.
func discussionProvider(config agentconfig.Config, skillID string) string {
	if models.NormalizeSkillID(skillID) != "discuss" {
		return ""
	}
	return liveProvider(config)
}
