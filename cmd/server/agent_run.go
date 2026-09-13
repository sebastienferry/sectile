package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"tasks/internal/agentconfig"
	"tasks/internal/handlers"
)

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
	once     sync.Once
}

func (d *agentDaemon) wrapRun(taskID, runID, command string) (string, error) {
	binary, err := os.Executable()
	if err != nil {
		return "", err
	}
	d.runsMu.Lock()
	defer d.runsMu.Unlock()
	if d.shuttingDown {
		return "", fmt.Errorf("agent is restarting")
	}
	if d.runs == nil {
		d.runs = make(map[string]*controlledRun)
	}
	existing := d.runs[runID]
	if existing != nil && (existing.token != "" || existing.taskID != taskID || existing.desktop.Status != "preparing") {
		return "", fmt.Errorf("execution already registered")
	}
	run := &controlledRun{taskID: taskID, token: uuid.NewString(), exited: make(chan struct{})}
	if existing != nil {
		existing.token = run.token
		run = existing
	}
	d.runs[runID] = run
	return quoteShell(binary) + " agent-exec --url " + quoteShell(d.agentURL+"/control/runs/"+runID) + " --token " + quoteShell(run.token) + " --command " + quoteShell(command), nil
}

func (d *agentDaemon) handleRunControl(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Origin") != "" {
		http.Error(w, "Forbidden", 403)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/control/runs/")
	d.runsMu.Lock()
	run := d.runs[id]
	if run == nil || subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")), []byte(run.token)) != 1 {
		d.runsMu.Unlock()
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
		go func() { _ = d.finishDesktopRun(context.Background(), run.taskID, id, result.Status) }()
		run.once.Do(func() { close(run.exited) })
	}
	d.runsMu.Unlock()
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"canceled": canceled})
}

func (d *agentDaemon) cancelRun(ctx context.Context, conn *websocket.Conn, msg handlers.AgentMessage, payload agentconfig.Dispatch) {
	d.runsMu.Lock()
	run := d.runs[payload.RunID]
	if run == nil || run.taskID != msg.TaskID {
		d.runsMu.Unlock()
		d.sendStatus(conn, msg.MsgID, msg.TaskID, "failed", "This agent does not own the execution")
		return
	}
	run.canceled = true
	d.runsMu.Unlock()
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

// runAgentExec is the terminal-side supervisor for an agent-owned command.
func runAgentExec(args []string) error {
	flags := flag.NewFlagSet("agent-exec", flag.ContinueOnError)
	endpoint := flags.String("url", "", "Local run control endpoint")
	token := flags.String("token", "", "Run control token")
	command := flags.String("command", "", "Command")
	if err := flags.Parse(args); err != nil {
		return err
	}
	client := &http.Client{Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	exitStatus := "failed"
	control := func(method string) (bool, error) {
		req, err := http.NewRequest(method, *endpoint, strings.NewReader(mustJSON(map[string]string{"status": exitStatus})))
		if err != nil {
			return false, err
		}
		req.Header.Set("Authorization", "Bearer "+*token)
		resp, err := client.Do(req)
		if err != nil {
			return false, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return false, fmt.Errorf("run control returned %d", resp.StatusCode)
		}
		var state struct {
			Canceled bool `json:"canceled"`
		}
		err = json.NewDecoder(resp.Body).Decode(&state)
		return state.Canceled, err
	}
	canceled, err := control(http.MethodGet)
	if err != nil {
		return err
	}
	if canceled {
		_, _ = control(http.MethodPost)
		return fmt.Errorf("execution canceled before launch")
	}
	cmd := exec.Command("bash", "-lc", *command)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	restore, err := startControlledCommand(cmd)
	if err != nil {
		_, _ = control(http.MethodPost)
		return err
	}
	defer restore()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case err := <-done:
			if err == nil {
				exitStatus = "completed"
			}
			_, _ = control(http.MethodPost)
			return err
		case <-ticker.C:
			canceled, err := control(http.MethodGet)
			if err != nil {
				continue
			}
			if !canceled {
				continue
			}
			stopControlledCommand(cmd, false)
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				stopControlledCommand(cmd, true)
				<-done
			}
			// Clean up remaining processes in the owned process group.
			stopControlledCommand(cmd, true)
			_, err = control(http.MethodPost)
			if err != nil {
				return err
			}
			return fmt.Errorf("execution canceled")
		}
	}
}

func (d *agentDaemon) enqueueRun(taskID string, payload agentconfig.Dispatch, projectID, root string, limit int, isolated bool) (*controlledRun, error) {
	d.runsMu.Lock()
	defer d.runsMu.Unlock()
	if d.shuttingDown {
		return nil, fmt.Errorf("agent is stopping")
	}
	if d.runs == nil {
		d.runs = map[string]*controlledRun{}
	}
	if d.runs[payload.RunID] != nil {
		return nil, fmt.Errorf("execution already registered")
	}
	d.queueSequence++
	run := &controlledRun{taskID: taskID, exited: make(chan struct{}), sequence: d.queueSequence, limit: limit, root: root, isolated: isolated,
		desktop: desktopRun{CreatedAt: time.Now().UTC(), Prompt: payload.Prompt, ID: payload.RunID, TaskID: taskID, TaskKey: payload.TaskKey, ProjectID: projectID, Skill: payload.SkillID, Directory: root, Status: "queued"}}
	d.runs[payload.RunID] = run
	return run, nil
}

func (d *agentDaemon) awaitRunSlot(ctx context.Context, run *controlledRun) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		d.runsMu.Lock()
		if run.canceled || d.shuttingDown {
			d.runsMu.Unlock()
			return fmt.Errorf("execution canceled")
		}
		active, blocked := 0, false
		for _, other := range d.runs {
			if other == run {
				continue
			}
			select {
			case <-other.exited:
				continue
			default:
			}
			sameProject := other.desktop.ProjectID == run.desktop.ProjectID
			shared := other.root == run.root && (!other.isolated || !run.isolated || other.taskID == run.taskID)
			if !sameProject && !shared {
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
				if sameProject {
					active++
				}
				if shared {
					blocked = true
				}
			}
		}
		if !blocked && active < run.limit {
			run.desktop.Status = "preparing"
			d.runsMu.Unlock()
			return nil
		}
		d.runsMu.Unlock()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
