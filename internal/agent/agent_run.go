package agent

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"tasks/internal/agentconfig"
	"tasks/internal/agenthttp"
	"tasks/internal/agentprotocol"
	"tasks/internal/runner"
)

// runQueue owns every execution the agent supervises, from admission to exit,
// together with the lifecycle flags that decide whether new work is admitted at
// all. It is the sole owner of mu: nothing outside this type's own state may be
// read or written under that lock.
type runQueue struct {
	mu   sync.Mutex
	runs map[string]*controlledRun
	// sequence orders queued runs so admission is first-come, first-served.
	sequence uint64
	// shuttingDown stops admitting work; restartRequested distinguishes a
	// restart from a plain stop once the process is on its way out.
	shuttingDown     bool
	restartRequested bool
}

// read runs fn against the execution registered under id, holding the queue
// lock for exactly that call, and reports whether the execution existed. It
// spares every caller that only needs to copy or amend a couple of fields from
// spelling the locking out.
func (q *runQueue) read(id string, fn func(*controlledRun)) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	run := q.runs[id]
	if run == nil {
		return false
	}
	fn(run)
	return true
}

// canceled reports whether a stop was requested on a run the caller already
// holds. Supervisors poll it from outside the lock while the run is live.
func (q *runQueue) canceled(run *controlledRun) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return run.canceled
}

type controlledRun struct {
	sequence uint64
	limit    int
	root     string
	isolated bool
	desktop  desktopRun
	taskID   string
	token    string
	canceled bool
	exited   chan struct{}
	// transcript is the output of a headless run, kept so the desktop can show
	// it: that run has no PTY session to read from. Bounded by
	// headlessTranscriptLimit, head dropped first, guarded by queue.mu.
	transcript          string
	transcriptTruncated bool
	once                sync.Once
}

func (d *agentDaemon) wrapRun(taskID, runID, command string) (string, error) {
	binary, err := os.Executable()
	if err != nil {
		return "", err
	}
	d.queue.mu.Lock()
	defer d.queue.mu.Unlock()
	if d.queue.shuttingDown {
		return "", fmt.Errorf("agent is restarting")
	}
	if d.queue.runs == nil {
		d.queue.runs = make(map[string]*controlledRun)
	}
	existing := d.queue.runs[runID]
	if existing != nil && (existing.token != "" || existing.taskID != taskID || existing.desktop.Status != "preparing") {
		return "", fmt.Errorf("execution already registered")
	}
	run := &controlledRun{taskID: taskID, token: uuid.NewString(), exited: make(chan struct{})}
	if existing != nil {
		existing.token = run.token
		run = existing
	}
	d.queue.runs[runID] = run
	endpoint := d.loopback.url + "/control/runs/" + runID
	if shell := runner.HostShell(); shell != runner.ShellPosix {
		// The line is read by the user's own shell, so it is quoted for that shell. The command
		// itself is encoded because no Windows shell line can carry a multi-line prompt.
		quote := func(value string) string { return runner.QuoteArg(shell, value) }
		prefix := ""
		if shell == runner.ShellPowerShell {
			// PowerShell treats a quoted first token as a string unless it is invoked.
			prefix = "& "
		}
		return prefix + quote(binary) + " agent-exec --url " + quote(endpoint) +
			" --token " + quote(run.token) +
			" --command-base64 " + base64.StdEncoding.EncodeToString([]byte(command)), nil
	}
	return quoteShell(binary) + " agent-exec --url " + quoteShell(endpoint) + " --token " + quoteShell(run.token) + " --command " + quoteShell(command), nil
}

func (d *agentDaemon) handleRunControl(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Origin") != "" {
		http.Error(w, "Forbidden", 403)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/control/runs/")
	d.queue.mu.Lock()
	run := d.queue.runs[id]
	if run == nil || subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")), []byte(run.token)) != 1 {
		d.queue.mu.Unlock()
		http.Error(w, "Unauthorized", 401)
		return
	}
	canceled := run.canceled
	if r.Method == http.MethodPost {
		var result struct {
			Status string `json:"status"`
		}
		_ = json.NewDecoder(r.Body).Decode(&result)
		if result.Status != "completed" && result.Status != "failed" {
			result.Status = "failed"
		}
		if run.canceled {
			result.Status = "canceled"
		}
		run.desktop.Status = result.Status
		go func() { _ = d.finishDesktopRun(context.Background(), run.taskID, id, result.Status, "") }()
		run.once.Do(func() { close(run.exited) })
	}
	d.queue.mu.Unlock()
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"canceled": canceled})
}

// postRunEngine tells the server which engine this run was actually launched
// with. The launcher recorded its own resolution, but the workstation override
// lives here, so this is the report that makes the run record true.
func (d *agentDaemon) postRunEngine(runID, provider, model string) {
	if strings.TrimSpace(runID) == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	body := mustJSON(map[string]string{"provider": provider, "model": model})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		d.link.serverURL+"/api/activities/"+url.PathEscape(runID)+"/engine", strings.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := agenthttp.Client(d.link.token).Do(req)
	if err != nil {
		return
	}
	_ = resp.Body.Close()
}

func (d *agentDaemon) cancelRun(ctx context.Context, conn *websocket.Conn, msg agentprotocol.Message, payload agentconfig.Dispatch) {
	d.queue.mu.Lock()
	run := d.queue.runs[payload.RunID]
	if run == nil || run.taskID != msg.TaskID {
		d.queue.mu.Unlock()
		// The marker lets the server close a run this agent cannot own, which
		// happens whenever the agent restarted while a run was recorded.
		d.sendStatus(conn, msg.MsgID, msg.TaskID, "failed", agentprotocol.RunNotOwned+": this agent does not own the execution")
		return
	}
	run.canceled = true
	d.queue.mu.Unlock()
	timer := time.NewTimer(12 * time.Second)
	defer timer.Stop()
	select {
	case <-run.exited:
		d.sendStatus(conn, msg.MsgID, msg.TaskID, "completed", "Execution stopped")
	case <-timer.C:
		d.sendStatus(conn, msg.MsgID, msg.TaskID, "failed", "Stop requested but process exit is not confirmed")
	case <-ctx.Done():
	}
}

// admitProjectRun serializes repository resolution and registration with disconnection.
func (d *agentDaemon) admitProjectRun(ctx context.Context, taskID string, payload agentconfig.Dispatch, config agentconfig.Config) (*controlledRun, error) {
	d.prepareMu.Lock()
	defer d.prepareMu.Unlock()
	root, overrides, err := d.localProjectRoot(ctx, config)
	if err != nil {
		return nil, err
	}
	config = agentconfig.ApplyOverrides(config, overrides)
	return d.enqueueRun(taskID, payload, config.ProjectID, root, agentconfig.ExecutionLimit(config.ProjectID, config.UseWorktrees, overrides), config.UseWorktrees)
}

func (d *agentDaemon) enqueueRun(taskID string, payload agentconfig.Dispatch, projectID, root string, limit int, isolated bool) (*controlledRun, error) {
	d.queue.mu.Lock()
	defer d.queue.mu.Unlock()
	return d.enqueueRunLocked(taskID, payload, projectID, root, limit, isolated)
}

// enqueueRunLocked lets local admission publish complete metadata atomically.
func (d *agentDaemon) enqueueRunLocked(taskID string, payload agentconfig.Dispatch, projectID, root string, limit int, isolated bool) (*controlledRun, error) {
	if d.queue.shuttingDown {
		return nil, fmt.Errorf("agent is stopping")
	}
	if d.queue.runs == nil {
		d.queue.runs = map[string]*controlledRun{}
	}
	if d.queue.runs[payload.RunID] != nil {
		return nil, fmt.Errorf("execution already registered")
	}
	d.queue.sequence++
	run := &controlledRun{taskID: taskID, exited: make(chan struct{}), sequence: d.queue.sequence, limit: limit, root: root, isolated: isolated,
		desktop: desktopRun{CreatedAt: time.Now().UTC(), Prompt: payload.Prompt, ID: payload.RunID, TaskID: taskID, TaskKey: payload.TaskKey, ProjectID: projectID, Skill: payload.SkillID, Directory: root, Status: "queued"}}
	d.queue.runs[payload.RunID] = run
	return run, nil
}

func (r *controlledRun) isConsole() bool { return r.desktop.Kind == consoleRunKind }

// sharesCheckout reports whether two live runs would work in the same directory.
// A console runs in the mapped checkout, so it collides with executions that also
// use it, but not with worktree-isolated ones. Two consoles are opened deliberately
// by the user and are left alone.
func sharesCheckout(other, run *controlledRun) bool {
	if other.root != run.root {
		return false
	}
	if other.isConsole() && run.isConsole() {
		return false
	}
	if other.isConsole() {
		return !run.isolated
	}
	if run.isConsole() {
		return !other.isolated
	}
	return !other.isolated || !run.isolated || other.taskID == run.taskID
}

func (d *agentDaemon) awaitRunSlot(ctx context.Context, run *controlledRun) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		d.queue.mu.Lock()
		if run.canceled || d.queue.shuttingDown {
			d.queue.mu.Unlock()
			return fmt.Errorf("execution canceled")
		}
		active, blocked := 0, false
		for _, other := range d.queue.runs {
			if other == run {
				continue
			}
			select {
			case <-other.exited:
				continue
			default:
			}
			sameProject := other.desktop.ProjectID == run.desktop.ProjectID
			shared := sharesCheckout(other, run)
			// A free console is human-initiated and holds no background worker
			// capacity: it neither counts nor queues behind background work.
			capacity := sameProject && !other.isConsole() && !run.isConsole()
			if !capacity && !shared {
				continue
			}
			if other.desktop.Status == "queued" {
				if other.canceled {
					continue
				}
				if other.sequence < run.sequence {
					blocked = true
				}
			} else {
				if capacity {
					active++
				}
				if shared {
					blocked = true
				}
			}
		}
		if !blocked && (active < run.limit || run.isConsole()) {
			run.desktop.Status = "preparing"
			d.queue.mu.Unlock()
			return nil
		}
		d.queue.mu.Unlock()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
