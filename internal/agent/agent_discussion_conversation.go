package agent

import (
	"encoding/json"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"

	"tasks/internal/agentconfig"
)

// conversationControlsCapability tells Desktop this agent interrupts a
// conversation's turn and opens a terminal beside it.
const conversationControlsCapability = "conversation-controls"

// conversationQueueCapability tells Desktop this agent takes a message while
// Claude works.
const conversationQueueCapability = "conversation-queue"

// conversationDiscussionTTL bounds how long a ticket discussion launched from
// Desktop waits for its dispatch to come back from the server. A preference
// older than this belongs to a launch the server refused or never sent.
const conversationDiscussionTTL = 2 * time.Minute

// taskLaunchPreference carries the per-launch view and engine options Desktop
// selected when dispatching a task skill.
type taskLaunchPreference struct {
	at               time.Time
	view             string
	provider         string
	engineID         string
	model            string
	conversationMode string
}

// pendingDiscussionViews remembers which task launches Desktop explicitly asked
// to open in the conversation or terminal view, along with any one-off engine
// or permission mode overrides. The request goes through the server, which
// knows nothing of the view, so the agent keeps the preference until the
// dispatch of that task's launch arrives and consumes it. A mark wins; a
// dispatch without one, such as a launch from the web app, follows the
// workstation console view (#711).
type pendingDiscussionViews struct {
	mu       sync.Mutex
	at       map[string]time.Time
	launches map[string]taskLaunchPreference
}

func (p *pendingDiscussionViews) mark(taskID string) {
	p.markLaunch(taskID, taskLaunchPreference{view: agentconfig.ConsoleViewConversation})
}

func (p *pendingDiscussionViews) markLaunch(taskID string, pref taskLaunchPreference) {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := time.Now()
	pref.at = now
	if p.launches == nil {
		p.launches = map[string]taskLaunchPreference{}
	}
	p.launches[taskID] = pref
	if pref.view == agentconfig.ConsoleViewConversation {
		if p.at == nil {
			p.at = map[string]time.Time{}
		}
		p.at[taskID] = now
	} else if p.at != nil {
		delete(p.at, taskID)
	}
}

// takeLaunch consumes the launch preference recorded for any of ids, reporting
// whether a fresh one was found. Expired entries are dropped on the way.
func (p *pendingDiscussionViews) takeLaunch(ids ...string) (taskLaunchPreference, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	var chosen taskLaunchPreference
	found := false
	for _, id := range ids {
		if id == "" {
			continue
		}
		if pref, ok := p.launches[id]; ok {
			delete(p.launches, id)
			delete(p.at, id)
			if time.Since(pref.at) < conversationDiscussionTTL && !found {
				chosen = pref
				found = true
			}
		} else if at, ok := p.at[id]; ok {
			delete(p.at, id)
			if time.Since(at) < conversationDiscussionTTL && !found {
				chosen = taskLaunchPreference{at: at, view: agentconfig.ConsoleViewConversation}
				found = true
			}
		}
	}
	for id, pref := range p.launches {
		if time.Since(pref.at) >= conversationDiscussionTTL {
			delete(p.launches, id)
		}
	}
	for id, at := range p.at {
		if time.Since(at) >= conversationDiscussionTTL {
			delete(p.at, id)
		}
	}
	return chosen, found
}

// take consumes the preference recorded for any of ids, reporting whether a
// fresh conversation view preference was found. Expired entries are dropped on the way.
func (p *pendingDiscussionViews) take(ids ...string) bool {
	pref, ok := p.takeLaunch(ids...)
	return ok && pref.view == agentconfig.ConsoleViewConversation
}

// conversationDiscussionEngine reports whether a project's discussion can run
// as a conversation: Claude, Codex and Antigravity engines can. A launch
// template is not run by the conversation, which keeps only its model; the
// conversation says so in its first notice.
func conversationDiscussionEngine(config agentconfig.Config) bool {
	provider := liveProvider(config)
	return provider == "claude" || provider == "codex" || provider == "agy"
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
	return where + " Le modèle de lancement du moteur ne s’applique pas ici : seul son modèle IA est conservé."
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

// desktopConversationTerminal opens a plain terminal window on a
// conversation's directory, running the user's own shell, in the terminal the
// project is set to use.
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
	d.queue.mu.Unlock()
	terminal := d.resolveTerminalForProject(r.Context(), projectID, input.Terminal)
	if err := d.openDirectoryTerminal(runtime.GOOS, terminal, directory); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"opened": true, "terminal": terminal})
}
