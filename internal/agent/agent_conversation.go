package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"tasks/internal/agentconfig"
	"tasks/internal/agentexec"
	"tasks/internal/runner"
	"time"

	"github.com/google/uuid"
)

// Experimental conversations are local free consoles. Each turn runs Claude
// over pipes and resumes its own session; no shell or PTY interprets the prompt.
// The queue lock guards session and busy, including admission of the next turn.
type claudeConversation struct {
	session string
	busy    bool
}

type conversationEvent struct {
	Kind   string `json:"kind"`
	Text   string `json:"text"`
	Detail string `json:"detail,omitempty"`
}

func conversationWrite(trace *runTrace, kind, text, detail string) {
	data, _ := json.Marshal(conversationEvent{kind, text, detail})
	trace.write(string(data))
}

func (d *agentDaemon) desktopConversation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	var input struct {
		SourceRunID string `json:"sourceRunId"`
		Message     string `json:"message"`
	}
	if r.Method == http.MethodPost {
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024)).Decode(&input) != nil {
			http.Error(w, "Invalid conversation request", http.StatusBadRequest)
			return
		}
	}
	d.queue.mu.Lock()
	defer d.queue.mu.Unlock()
	if id == "" && r.Method == http.MethodPost {
		source := d.queue.runs[input.SourceRunID]
		if source == nil || source.desktop.Directory == "" {
			http.Error(w, "Select an execution with a local directory", http.StatusNotFound)
			return
		}
		id = uuid.NewString()
		run, err := d.enqueueRunLocked("", agentconfig.Dispatch{RunID: id}, source.desktop.ProjectID, source.desktop.Directory, 1, false)
		if err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		run.desktop.Kind = consoleRunKind
		run.desktop.Provider = "claude"
		if source.desktop.Provider == "claude" {
			run.desktop.Model = source.desktop.Model
		}
		run.desktop.Branch = source.desktop.Branch
		run.desktop.Conversation, run.desktop.Headless = true, true
		run.desktop.Status = "running"
		run.desktop.StartedAt = time.Now().UTC()
		run.trace = newRunTrace()
		run.conversation = &claudeConversation{}
		conversationWrite(run.trace, "notice", "Claude Code conversation · experimental", "Edits are accepted. Commands requiring approval are denied. This conversation is independent of the selected execution and uses the same directory.")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(run.desktop)
		return
	}
	run := d.queue.runs[id]
	if run == nil || !run.desktop.Conversation {
		http.Error(w, "Conversation not found", http.StatusNotFound)
		return
	}
	if r.Method == http.MethodGet {
		lines, version := run.trace.snapshot()
		busy := run.conversation != nil && run.conversation.busy
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": id, "events": lines, "version": version, "busy": busy, "readOnly": run.restored || run.canceled || run.conversation == nil})
		return
	}
	if strings.TrimSpace(input.Message) == "" {
		http.Error(w, "Message required", http.StatusBadRequest)
		return
	}
	if run.conversation == nil || run.restored || run.canceled || d.queue.shuttingDown || run.conversation.busy {
		http.Error(w, "Conversation is unavailable or already answering", http.StatusConflict)
		return
	}
	run.conversation.busy = true
	conversationWrite(run.trace, "user", input.Message, "")
	go d.conversationTurn(run, input.Message)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]bool{"accepted": true})
}

func claudeConversationCommand(directory, model, session, prompt string) *exec.Cmd {
	args := []string{"-p", "--output-format", "stream-json", "--verbose", "--permission-mode", "acceptEdits"}
	if model != "" {
		args = append(args, "--model", model)
	}
	if session != "" {
		args = append(args, "--resume", session)
	}
	cmd := agentexec.Hidden(exec.Command("claude", args...))
	cmd.Dir = directory
	cmd.Env = commandEnv(nil)
	cmd.Stdin = strings.NewReader(prompt)
	// A descendant retaining stdout must not prevent stopping the conversation.
	cmd.WaitDelay = headlessStopGrace
	return cmd
}

// conversationOutput bounds each frame, while preserving frames larger than a
// scanner's default 64 KiB limit. Unknown JSON stays out of the rendered chat.
type conversationOutput struct {
	buffer     []byte
	discarding bool
	line       func(string)
}

func (o *conversationOutput) Write(data []byte) (int, error) {
	for _, b := range data {
		if b == '\n' {
			if !o.discarding {
				o.line(strings.TrimSuffix(string(o.buffer), "\r"))
			}
			o.buffer = nil
			o.discarding = false
		} else if !o.discarding {
			o.buffer = append(o.buffer, b)
			if len(o.buffer) > 2*1024*1024 {
				o.buffer = nil
				o.discarding = true
				o.line("Claude emitted a frame too large to display.")
			}
		}
	}
	return len(data), nil
}

func (d *agentDaemon) conversationTurn(run *controlledRun, prompt string) {
	d.queue.mu.Lock()
	cmd := claudeConversationCommand(run.desktop.Directory, run.desktop.Model, run.conversation.session, prompt)
	d.queue.mu.Unlock()
	resultSeen, resultFailed, assistantSeen := false, false, false
	output := &conversationOutput{line: func(line string) {
		var frame struct {
			Type    string            `json:"type"`
			Session string            `json:"session_id"`
			IsError bool              `json:"is_error"`
			Denials []json.RawMessage `json:"permission_denials"`
		}
		if json.Unmarshal([]byte(line), &frame) == nil && frame.Type != "" {
			if frame.Session != "" && (frame.Type == "system" || frame.Type == "result") {
				if _, err := uuid.Parse(frame.Session); err == nil {
					d.queue.mu.Lock()
					run.conversation.session = frame.Session
					d.queue.mu.Unlock()
				}
			}
			events, result, done := runner.ParseReasoningLine(line)
			for _, event := range events {
				kind := event.Kind
				if kind == runner.ReasoningText {
					kind = "assistant"
					assistantSeen = true
				}
				conversationWrite(run.trace, kind, event.Text, event.Detail)
			}
			if done {
				resultSeen, resultFailed = true, frame.IsError
				if frame.IsError {
					conversationWrite(run.trace, "error", result, "")
				} else if !assistantSeen && result != "" {
					conversationWrite(run.trace, "assistant", result, "")
				}
				if len(frame.Denials) > 0 {
					conversationWrite(run.trace, "notice", "Some tools required approval and were denied.", "This prototype does not yet offer interactive tool approvals.")
				}
			}
			return
		}
		if strings.TrimSpace(line) != "" {
			conversationWrite(run.trace, "error", line, "")
		}
	}}
	cmd.Stdout = output
	// Keep diagnostics separate so stderr cannot corrupt JSON on stdout.
	var diagnostics limitedConversationBuffer
	cmd.Stderr = &diagnostics
	release, err := agentexec.StartDetached(cmd)
	if err == nil {
		waited := make(chan error, 1)
		go func() { waited <- cmd.Wait() }()
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		stopping := false
		for waiting := true; waiting; {
			select {
			case err = <-waited:
				waiting = false
			case <-ticker.C:
				if d.queue.canceled(run) {
					agentexec.StopControlled(cmd, stopping)
					stopping = true
				}
			}
		}
	}
	release()
	if len(output.buffer) > 0 {
		output.line(string(output.buffer))
	}
	if diagnostics.Len() > 0 {
		conversationWrite(run.trace, "notice", diagnostics.String(), "")
	}
	d.queue.mu.Lock()
	defer d.queue.mu.Unlock()
	if run.canceled {
		run.desktop.Status = "canceled"
		run.once.Do(func() { close(run.exited) })
		conversationWrite(run.trace, "notice", "Conversation stopped", "")
		run.trace.close()
	} else if err != nil {
		conversationWrite(run.trace, "error", fmt.Sprintf("Claude Code failed: %v", err), "")
	} else if !resultSeen {
		conversationWrite(run.trace, "error", "Claude Code exited without a final result.", "")
	} else if !resultFailed {
		conversationWrite(run.trace, "notice", "Ready for your next message", "")
	}
	run.conversation.busy = false
}

type limitedConversationBuffer struct{ bytes.Buffer }

func (b *limitedConversationBuffer) Write(data []byte) (int, error) {
	n := len(data)
	if remaining := 64*1024 - b.Len(); remaining > 0 {
		if len(data) > remaining {
			data = data[:remaining]
		}
		_, _ = b.Buffer.Write(data)
	}
	return n, nil
}

func (d *agentDaemon) stopConversations() {
	d.queue.mu.Lock()
	var exits []<-chan struct{}
	for _, run := range d.queue.runs {
		if run.conversation == nil {
			continue
		}
		run.canceled = true
		if !run.conversation.busy {
			run.desktop.Status = "canceled"
			run.once.Do(func() { close(run.exited) })
			run.trace.close()
		}
		exits = append(exits, run.exited)
	}
	d.queue.mu.Unlock()
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	for _, exit := range exits {
		select {
		case <-exit:
		case <-deadline.C:
			return
		}
	}
}
