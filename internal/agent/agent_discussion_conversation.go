package agent

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"tasks/internal/agentconfig"
)

// conversationControlsCapability tells Desktop this agent interrupts a
// conversation's turn and opens a terminal beside it.
const conversationControlsCapability = "conversation-controls"

// conversationDiscussionTTL bounds how long a ticket discussion launched from
// Desktop waits for its dispatch to come back from the server. A preference
// older than this belongs to a launch the server refused or never sent.
const conversationDiscussionTTL = 2 * time.Minute

// pendingDiscussionViews remembers which ticket discussions Desktop asked to
// open in the conversation view. The request goes through the server, which
// knows nothing of the view, so the agent keeps the preference until the
// dispatch of that task's discussion arrives and consumes it.
type pendingDiscussionViews struct {
	mu sync.Mutex
	at map[string]time.Time
}

func (p *pendingDiscussionViews) mark(taskID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.at == nil {
		p.at = map[string]time.Time{}
	}
	p.at[taskID] = time.Now()
}

// take consumes the preference recorded for any of ids, reporting whether a
// fresh one was found. Expired entries are dropped on the way.
func (p *pendingDiscussionViews) take(ids ...string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	found := false
	for _, id := range ids {
		if at, ok := p.at[id]; ok && id != "" {
			delete(p.at, id)
			found = found || time.Since(at) < conversationDiscussionTTL
		}
	}
	for id, at := range p.at {
		if time.Since(at) >= conversationDiscussionTTL {
			delete(p.at, id)
		}
	}
	return found
}

// conversationDiscussionEngine reports whether a project's discussion can run
// as a conversation: any engine whose provider is Claude can. A launch
// template is not run by the conversation, which keeps only its model; the
// conversation says so in its first notice.
func conversationDiscussionEngine(config agentconfig.Config) bool {
	return liveProvider(config) == "claude"
}

// conversationModel is the model a conversation of this engine runs with:
// the one the project resolves, else the engine's own.
func conversationModel(config agentconfig.Config) string {
	if model := strings.TrimSpace(agentconfig.ResolveModel(config, "")); model != "" {
		return model
	}
	return strings.TrimSpace(config.AIModel)
}

// conversationOrigin completes a conversation's first notice: where it runs,
// and that the engine's launch template does not apply to it.
func conversationOrigin(config agentconfig.Config, where string) string {
	if strings.TrimSpace(config.AICommandTemplate) == "" {
		return where
	}
	return where + " The engine's launch template does not apply here: only its model is kept."
}

// conversationStoppedStatus is how a stopped conversation ends: a ticket
// discussion completes, as its terminal counterpart does; a free conversation
// is canceled.
func conversationStoppedStatus(run *controlledRun) string {
	if run.desktop.TaskID != "" {
		return stoppedStatus(run.desktop.Skill)
	}
	return "canceled"
}

// extraConversationDirs are the folders a skill launch added to the project's
// own, which a conversation's turns would otherwise not be given.
func extraConversationDirs(launch, project []string) []string {
	known := map[string]bool{}
	for _, dir := range project {
		known[dir] = true
	}
	var extra []string
	for _, dir := range launch {
		if !known[dir] {
			known[dir] = true
			extra = append(extra, dir)
		}
	}
	return extra
}

// conversationShellID names the shell session opened beside a conversation.
func conversationShellID(runID string) string { return "shell-" + runID }

// desktopConversationTerminal opens a native terminal on a shell in a
// conversation's directory, with the environment its turns carry. The window
// is visible on purpose: the user asked for it, as for "Detach to native
// terminal". A second click reuses the same shell.
func (d *agentDaemon) desktopConversationTerminal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var input struct {
		RunID    string `json:"runId"`
		Terminal string `json:"terminal"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&input) != nil || strings.TrimSpace(input.RunID) == "" {
		http.Error(w, "Run ID required", http.StatusBadRequest)
		return
	}
	d.queue.mu.Lock()
	run := d.queue.runs[input.RunID]
	if run == nil || run.conversation == nil || run.restored || run.canceled || run.desktop.Directory == "" {
		d.queue.mu.Unlock()
		http.Error(w, "Conversation not found or ended", http.StatusNotFound)
		return
	}
	directory, projectID := run.desktop.Directory, run.desktop.ProjectID
	env := map[string]string{"SECTILE_PROJECT_ID": projectID}
	for key, value := range run.conversation.env {
		env[key] = value
	}
	d.queue.mu.Unlock()
	if d.terminal.manager == nil {
		http.Error(w, "Terminal manager unavailable", http.StatusServiceUnavailable)
		return
	}
	sessionID := conversationShellID(input.RunID)
	if _, err := d.terminal.manager.GetOrCreateSession(sessionID, directory, env); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	terminal := d.resolveTerminalForProject(r.Context(), projectID, input.Terminal)
	if err := d.launchExternalTerminal(terminal, sessionID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"opened": true, "terminal": terminal})
}
