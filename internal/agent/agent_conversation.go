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
	// effort is the level the last turn ran with; empty leaves the CLI default.
	effort string
	// contextUsed is the size of the latest main-thread request, and
	// contextWindow the limit Claude reported for its model; zero when unknown.
	contextUsed   int
	contextWindow int
}

// conversationEfforts are the levels `claude --effort` accepts.
var conversationEfforts = map[string]bool{"low": true, "medium": true, "high": true, "xhigh": true, "max": true}

// conversationUsage is the token accounting of one Claude API request.
type conversationUsage struct {
	Input         int `json:"input_tokens"`
	CacheCreation int `json:"cache_creation_input_tokens"`
	CacheRead     int `json:"cache_read_input_tokens"`
	Output        int `json:"output_tokens"`
}

// contextSize is what the request occupies in the context window: everything
// sent, cached or not, plus what the model wrote back into the transcript.
func (u conversationUsage) contextSize() int {
	return u.Input + u.CacheCreation + u.CacheRead + u.Output
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
		Effort      string `json:"effort"`
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
		model := ""
		if source.desktop.Provider == "claude" {
			model = source.desktop.Model
		}
		run, err := d.newConversationLocked(source.desktop.ProjectID, source.desktop.Directory, model, "This conversation is independent of the selected execution and uses the same directory.")
		if err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		run.desktop.Branch = source.desktop.Branch
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
		response := map[string]any{"id": id, "events": lines, "version": version, "busy": false, "readOnly": run.restored || run.canceled || run.conversation == nil}
		if c := run.conversation; c != nil {
			response["busy"], response["effort"] = c.busy, c.effort
			if c.contextWindow > 0 {
				response["context"] = map[string]int{"used": c.contextUsed, "window": c.contextWindow}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
		return
	}
	if strings.TrimSpace(input.Message) == "" {
		http.Error(w, "Message required", http.StatusBadRequest)
		return
	}
	if input.Effort != "" && !conversationEfforts[input.Effort] {
		http.Error(w, "Unknown effort level", http.StatusBadRequest)
		return
	}
	if run.conversation == nil || run.restored || run.canceled || d.queue.shuttingDown || run.conversation.busy {
		http.Error(w, "Conversation is unavailable or already answering", http.StatusConflict)
		return
	}
	run.conversation.busy = true
	run.conversation.effort = input.Effort
	conversationWrite(run.trace, "user", input.Message, "")
	go d.conversationTurn(run, input.Message)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]bool{"accepted": true})
}

// newConversationLocked admits a conversation in directory. It needs no run
// slot: no process exists until a message arrives. The queue lock is held.
func (d *agentDaemon) newConversationLocked(projectID, directory, model, origin string) (*controlledRun, error) {
	run, err := d.enqueueRunLocked("", agentconfig.Dispatch{RunID: uuid.NewString()}, projectID, directory, 1, false)
	if err != nil {
		return nil, err
	}
	run.desktop.Kind = consoleRunKind
	run.desktop.Provider = "claude"
	run.desktop.Model = model
	run.desktop.Conversation, run.desktop.Headless = true, true
	run.desktop.Status = "running"
	run.desktop.StartedAt = time.Now().UTC()
	run.trace = newRunTrace()
	run.conversation = &claudeConversation{}
	conversationWrite(run.trace, "notice", "Claude Code conversation · experimental", "Edits are accepted. Commands requiring approval are denied. "+origin)
	return run, nil
}

func claudeConversationCommand(directory, model, effort, session, prompt string) *exec.Cmd {
	args := []string{"-p", "--output-format", "stream-json", "--verbose", "--permission-mode", "acceptEdits"}
	if model != "" {
		args = append(args, "--model", model)
	}
	if effort != "" {
		args = append(args, "--effort", effort)
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
	cmd := claudeConversationCommand(run.desktop.Directory, run.desktop.Model, run.conversation.effort, run.conversation.session, prompt)
	d.queue.mu.Unlock()
	resultSeen, resultFailed, assistantSeen, mainModel := false, false, false, ""
	output := &conversationOutput{line: func(line string) {
		var frame struct {
			Type    string            `json:"type"`
			Session string            `json:"session_id"`
			IsError bool              `json:"is_error"`
			Model   string            `json:"model"`
			Denials []json.RawMessage `json:"permission_denials"`
			// A subagent's requests carry their own context; only the main
			// thread's measure the conversation.
			Parent  *string `json:"parent_tool_use_id"`
			Message struct {
				Usage *conversationUsage `json:"usage"`
			} `json:"message"`
			ModelUsage map[string]struct {
				ContextWindow int `json:"contextWindow"`
			} `json:"modelUsage"`
		}
		if json.Unmarshal([]byte(line), &frame) == nil && frame.Type != "" {
			d.queue.mu.Lock()
			if frame.Type == "assistant" && frame.Parent == nil && frame.Message.Usage != nil {
				run.conversation.contextUsed = frame.Message.Usage.contextSize()
			}
			if frame.Type == "system" && frame.Model != "" {
				mainModel = frame.Model
			}
			// modelUsage also lists the models subagents used, so the main
			// model's entry wins; the largest window is only a fallback.
			if usage, ok := frame.ModelUsage[mainModel]; ok && usage.ContextWindow > 0 {
				run.conversation.contextWindow = usage.ContextWindow
			} else {
				for _, usage := range frame.ModelUsage {
					run.conversation.contextWindow = max(run.conversation.contextWindow, usage.ContextWindow)
				}
			}
			d.queue.mu.Unlock()
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
