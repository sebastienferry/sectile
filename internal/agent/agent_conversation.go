package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"strconv"
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
	// mode is the permission mode turns run in, one of conversationModes;
	// empty means acceptEdits.
	mode string
	// contextUsed is the size of the latest main-thread request, and
	// contextWindow the limit Claude reported for its model; zero when unknown.
	contextUsed   int
	contextWindow int
	// env is the environment a ticket discussion's turns carry, the one its
	// terminal would have had; nil for a free conversation.
	env map[string]string
	// partial is the reply Claude is still writing, from its streamed text
	// deltas. It is never stored: the complete message replaces it in the
	// trace as soon as Claude emits it.
	partial string
	// extraDirs are folders a skill launch adds to the project's, such as a
	// custom skill's own folder; every turn is given them.
	extraDirs []string
	// interrupted asks the turn in progress to stop. Unlike a stop, the
	// conversation stays open and the next message resumes its session.
	interrupted bool
	// input is the stdin of the turn in progress, nil between turns, and
	// approvals the tool calls of that turn waiting for the owner.
	input     *conversationInput
	approvals []conversationApproval
	// requestAt is when Claude last started a main-thread request, and
	// sentAt when a message last reached a turn already running: a message
	// sent after Claude's last request may not have been read yet, so the
	// turn's stdin stays open a moment past its result.
	requestAt, sentAt time.Time
	// next holds messages that arrived after the turn's stdin closed; they
	// start the next turn as soon as this one ends.
	next []string
	// shellContext is what the commands typed with "!" since the last
	// message printed; it goes to Claude with the next message.
	shellContext string
	// commands are the slash commands Claude offers here, for the composer's
	// completion; loadingCommands says a probe is reading them.
	commands        []conversationSlash
	loadingCommands bool
	// sectileMCP is whether Claude reaches Sectile's MCP server here, nil
	// until checked; checkingMCP says a check runs.
	sectileMCP  *conversationMCP
	checkingMCP bool
	// waitingForApproval says the run's WaitingSince mark is the agent's own,
	// raised while a tool call or a question waits for the owner, rather than
	// one the session declared.
	waitingForApproval bool
}

// markApprovalWaitLocked raises the run's wait while a tool call or question
// waits for the owner, and lowers it once none does, so Desktop notifies and
// marks the run as it does for a session waiting in a terminal. A wait the
// session declared is left alone. The queue lock is held.
func markApprovalWaitLocked(run *controlledRun) {
	c := run.conversation
	if len(c.approvals) > 0 && run.desktop.WaitingSince.IsZero() {
		run.desktop.WaitingSince, c.waitingForApproval = time.Now().UTC(), true
	} else if len(c.approvals) == 0 && c.waitingForApproval {
		run.desktop.WaitingSince, c.waitingForApproval = time.Time{}, false
	}
}

// conversationSentGrace is how long a turn's stdin stays open past its result
// for a message sent after Claude's last request.
const conversationSentGrace = 3 * time.Second

// endTurnInput closes a turn's stdin on its result, unless a message sent
// since Claude's last request may still be read: then Claude has a moment to
// start the request that answers it, and the next result closes stdin.
func (d *agentDaemon) endTurnInput(run *controlledRun, input *conversationInput) {
	d.queue.mu.Lock()
	request, waiting := run.conversation.requestAt, run.conversation.sentAt.After(run.conversation.requestAt)
	d.queue.mu.Unlock()
	if !waiting {
		input.close()
		return
	}
	go func() {
		time.Sleep(conversationSentGrace)
		d.queue.mu.Lock()
		answered := run.conversation.requestAt.After(request)
		d.queue.mu.Unlock()
		if !answered {
			input.close()
		}
	}()
}

// conversationPartialLimit bounds a reply in progress; past it the draft stops
// growing and the complete message, which arrives anyway, shows the rest.
const conversationPartialLimit = 256 * 1024

// conversationStream is the part of a streamed frame a draft is built from.
type conversationStream struct {
	Type  string `json:"type"`
	Delta struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"delta"`
	ContentBlock struct {
		Type string `json:"type"`
	} `json:"content_block"`
}

// streamPartial applies one streamed event of the main thread to the draft.
func streamPartial(c *claudeConversation, event conversationStream) {
	switch event.Type {
	case "content_block_start", "content_block_stop", "message_stop":
		c.partial = ""
	case "content_block_delta":
		if event.Delta.Type == "text_delta" && len(c.partial)+len(event.Delta.Text) <= conversationPartialLimit {
			c.partial += event.Delta.Text
		}
	}
}

// conversationAllowedTools lets a conversation call Sectile's own MCP tools
// without an approval it has no way to ask for: a skill reports its run and
// moves its task through them. The rule names the server, so it covers every
// tool it has; the "=" form keeps it from swallowing the next argument.
const conversationAllowedTools = "--allowedTools=mcp__sectile"

// conversationModes are the permission modes the owner can pick, as Claude
// Code's own mode switch offers them. bypassPermissions is not one of them.
var conversationModes = map[string]bool{"default": true, "acceptEdits": true, "plan": true}

// conversationMode is the permission mode a turn runs in.
func conversationMode(mode string) string {
	if conversationModes[mode] {
		return mode
	}
	return "acceptEdits"
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
	// Tool and Input are a tool call's full name and arguments, which
	// Desktop draws as a card per tool; Detail stays for a reader without.
	Tool  string          `json:"tool,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
	// ToolID ties a tool call to its result; a "tool_result" event carries
	// the answer in Text, with Truncated and Error saying how it went.
	ToolID    string `json:"toolId,omitempty"`
	Truncated bool   `json:"truncated,omitempty"`
	Error     bool   `json:"error,omitempty"`
}

func conversationWrite(trace *runTrace, kind, text, detail string) {
	conversationWriteEvent(trace, conversationEvent{Kind: kind, Text: text, Detail: detail})
}

func conversationWriteEvent(trace *runTrace, event conversationEvent) {
	data, _ := json.Marshal(event)
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
		// Model and Mode, when set, apply from the next turn on.
		Model string `json:"model"`
		Mode  string `json:"mode"`
		// Interrupt stops the turn in progress and keeps the conversation.
		Interrupt bool `json:"interrupt"`
		// CheckMCP checks again whether Claude reaches Sectile's MCP server.
		CheckMCP bool `json:"checkMcp"`
		// Approval answers a tool call the turn is waiting on.
		Approval *struct {
			ID       string            `json:"id"`
			Decision string            `json:"decision"`
			Answers  map[string]string `json:"answers"`
		} `json:"approval"`
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
		response := map[string]any{"id": id, "version": version, "busy": false, "readOnly": run.restored || run.canceled || run.conversation == nil}
		// Desktop polls a conversation several times a second and sends the
		// version it already shows: the events go only when they changed.
		if since := r.URL.Query().Get("since"); since == "" || since != strconv.FormatUint(version, 10) {
			response["events"] = lines
		}
		if c := run.conversation; c != nil {
			response["busy"], response["effort"], response["model"], response["mode"] = c.busy, c.effort, run.desktop.Model, conversationMode(c.mode)
			response["approvals"] = append([]conversationApproval{}, c.approvals...)
			response["commands"] = c.commands
			response["sectileMcp"] = c.sectileMCP
			if !run.restored && !run.canceled {
				if c.commands == nil {
					d.loadConversationCommandsLocked(run)
				}
				if c.sectileMCP == nil {
					d.checkSectileMCPLocked(run)
				}
			}
			if c.busy && c.partial != "" {
				response["partial"] = c.partial
			}
			if c.contextWindow > 0 {
				response["context"] = map[string]int{"used": c.contextUsed, "window": c.contextWindow}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
		return
	}
	if input.CheckMCP {
		if run.conversation == nil || run.restored || run.canceled {
			http.Error(w, "Conversation is unavailable", http.StatusConflict)
			return
		}
		d.checkSectileMCPLocked(run)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]bool{"accepted": true})
		return
	}
	if input.Approval != nil {
		if run.conversation == nil || !conversationDecisions[input.Approval.Decision] {
			http.Error(w, "Unknown decision", http.StatusBadRequest)
			return
		}
		if err := d.decideApprovalLocked(run, input.Approval.ID, input.Approval.Decision, input.Approval.Answers); err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]bool{"accepted": true})
		return
	}
	if input.Interrupt {
		if run.conversation == nil || !run.conversation.busy {
			http.Error(w, "Claude Code is not answering", http.StatusConflict)
			return
		}
		run.conversation.interrupted = true
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]bool{"accepted": true})
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
	if input.Mode != "" && !conversationModes[input.Mode] {
		http.Error(w, "Unknown permission mode", http.StatusBadRequest)
		return
	}
	if err := agentconfig.ValidModel(input.Model); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if run.conversation == nil || run.restored || run.canceled || d.queue.shuttingDown {
		http.Error(w, "Conversation is unavailable", http.StatusConflict)
		return
	}
	if model := strings.TrimSpace(input.Model); model != "" {
		run.desktop.Model = model
	}
	// /mcp is answered by the CLI's own health check, not by print mode.
	if strings.TrimSpace(input.Message) == "/mcp" {
		conversationWrite(run.trace, "user", "/mcp", "")
		d.startConversationMCPLocked(run)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]bool{"accepted": true, "local": true})
		return
	}
	// A message starting with "!" runs in the shell, not through Claude.
	if line, ok := strings.CutPrefix(strings.TrimSpace(input.Message), "!"); ok {
		if strings.TrimSpace(line) == "" {
			http.Error(w, "Command required", http.StatusBadRequest)
			return
		}
		d.startConversationShellLocked(run, strings.TrimSpace(line))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]bool{"accepted": true, "shell": true})
		return
	}
	// A message answers a wait the session declared, as Enter does in a
	// terminal: the mark goes, and the server is told.
	if !run.desktop.WaitingSince.IsZero() && !run.conversation.waitingForApproval {
		since := run.desktop.WaitingSince
		run.desktop.WaitingSince, run.answeredAt = time.Time{}, since
		go d.sendAnswered(id, since)
	}
	if input.Mode != "" {
		run.conversation.mode = input.Mode
	}
	// A message sent while Claude works joins the turn, as in Claude Code;
	// one sent as the turn closes starts the next turn.
	if run.conversation.busy {
		conversationWrite(run.trace, "user", input.Message, "")
		message := withShellContext(run.conversation, input.Message)
		queued := run.conversation.input == nil || run.conversation.input.send(conversationUserMessage(message)) != nil
		if queued {
			run.conversation.next = append(run.conversation.next, message)
		} else {
			run.conversation.sentAt = time.Now()
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]bool{"accepted": true, "joined": !queued})
		return
	}
	run.conversation.busy = true
	run.conversation.effort = input.Effort
	conversationWrite(run.trace, "user", input.Message, "")
	go d.conversationTurn(run, withShellContext(run.conversation, input.Message))
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
	startConversationLocked(run, model, origin)
	return run, nil
}

// startConversationLocked turns an admitted run into a conversation waiting
// for its first message. The queue lock is held.
func startConversationLocked(run *controlledRun, model, origin string) {
	run.desktop.Provider = "claude"
	run.desktop.Model = model
	run.desktop.Conversation, run.desktop.Headless = true, true
	run.desktop.Status = "running"
	run.desktop.StartedAt = time.Now().UTC()
	run.trace = newRunTrace()
	run.conversation = &claudeConversation{}
	conversationWrite(run.trace, "notice", "Claude Code conversation · experimental", "Sectile's tools are accepted; what your rules and the chosen mode do not allow asks for your approval. "+origin)
}

// claudeConversationCommand is one turn's Claude. Each of dirs is one
// --add-dir=<path> argument: no shell reads it, so it needs no quoting, and the
// "=" form keeps a value from swallowing the arguments that follow it. The
// message is not an argument either: it goes in on stdin, in the streaming
// input format that also carries the tool approvals.
//
// settings is the file generated from the project's sandbox values (#700),
// "" when it has none, which leaves the command as it was.
func claudeConversationCommand(directory, model, effort, mode, session, settings string, dirs []string, env map[string]string) *exec.Cmd {
	// Partial messages stream each reply as Claude writes it; the complete
	// messages still arrive, so the trace is built from them alone.
	args := []string{"-p", "--input-format", "stream-json", "--output-format", "stream-json", "--verbose", "--include-partial-messages", "--permission-mode", conversationMode(mode), "--permission-prompt-tool", "stdio", conversationAllowedTools}
	if model != "" {
		args = append(args, "--model", model)
	}
	if effort != "" {
		args = append(args, "--effort", effort)
	}
	if session != "" {
		args = append(args, "--resume", session)
	}
	for _, dir := range dirs {
		args = append(args, "--add-dir="+dir)
	}
	if settings != "" {
		args = append(args, "--settings="+settings)
	}
	cmd := agentexec.Hidden(exec.Command("claude", args...))
	cmd.Dir = directory
	cmd.Env = commandEnv(env)
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

// conversationFolders reads the project's folders when a turn starts, so a
// folder attached since the previous turn, from the conversation or from the
// project settings, is given to this one (#676). env carries the same folder
// map as a skill run of the project.
func (d *agentDaemon) conversationFolders(projectID, directory string) ([]string, map[string]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), conversationFoldersTimeout)
	defer cancel()
	env := map[string]string{"SECTILE_PROJECT_ID": projectID}
	config, err := d.fetchConfig(ctx, projectID, "")
	if err != nil {
		return nil, env, err
	}
	folders, err := d.projectFolderMap(ctx, config, directory)
	if err != nil {
		return nil, env, err
	}
	if raw, err := json.Marshal(folders); err == nil && len(folders) > 0 {
		env["SECTILE_REPOSITORIES"] = string(raw)
	}
	return folderMapDirs(folders), env, nil
}

// conversationFoldersTimeout bounds the folder read of a turn: a server that
// does not answer delays the message, it never holds it.
const conversationFoldersTimeout = 10 * time.Second

// conversationInterruptGrace is how long an interrupted turn has to end on
// its own before it is stopped.
const conversationInterruptGrace = 5 * time.Second

func (d *agentDaemon) conversationTurn(run *controlledRun, prompt string) {
	d.queue.mu.Lock()
	projectID, directory := run.desktop.ProjectID, run.desktop.Directory
	d.queue.mu.Unlock()
	// A turn without the project's folders still runs: the folders widen what
	// Claude may read, they never decide whether it answers.
	dirs, env, err := d.conversationFolders(projectID, directory)
	if err != nil {
		conversationWrite(run.trace, "notice", "Attached folders could not be read for this message", err.Error())
	}
	// Each turn is a new Claude: it reads the project's values as they are
	// now, rules an "Always allow" added during this conversation included.
	settings, settingsErr := d.projectClaudeSettings(projectID)
	d.queue.mu.Lock()
	dirs = append(dirs, run.conversation.extraDirs...)
	// The folder map read for this turn wins over the one recorded at launch.
	for key, value := range run.conversation.env {
		if _, fresh := env[key]; !fresh {
			env[key] = value
		}
	}
	cmd := claudeConversationCommand(directory, run.desktop.Model, run.conversation.effort, run.conversation.mode, run.conversation.session, settings, dirs, env)
	d.queue.mu.Unlock()
	initialize, initID := conversationInitialize()
	var input *conversationInput
	if stdin, pipeErr := cmd.StdinPipe(); pipeErr == nil {
		input = &conversationInput{w: stdin}
	}
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
			// Only a stream_event's event is an object; other frames, such
			// as ui_invalidate, carry a string there.
			Event      json.RawMessage `json:"event"`
			MCPServers []struct {
				Name   string `json:"name"`
				Status string `json:"status"`
			} `json:"mcp_servers"`
		}
		// A field of an unexpected type is skipped while the others are
		// still decoded, so such a frame is read rather than shown raw.
		err := json.Unmarshal([]byte(line), &frame)
		var mismatch *json.UnmarshalTypeError
		if (err == nil || errors.As(err, &mismatch)) && frame.Type != "" {
			// Claude asks before using a tool its rules do not allow; the
			// call waits for the owner. Other requests are refused at once.
			if frame.Type == "control_request" {
				var control conversationControlRequest
				_ = json.Unmarshal([]byte(line), &control)
				if control.Request.Subtype != "can_use_tool" {
					_ = input.send(controlError(control.RequestID, "unsupported control request"))
					return
				}
				d.queue.mu.Lock()
				run.conversation.approvals = append(run.conversation.approvals, conversationApproval{ID: control.RequestID, ToolUseID: control.Request.ToolUseID, Tool: control.Request.ToolName, Description: control.Request.Description, Reason: control.Request.Reason, Input: control.Request.Input, Suggestions: control.Request.Suggestions})
				markApprovalWaitLocked(run)
				d.queue.mu.Unlock()
				return
			}
			if frame.Type == "control_response" {
				if commands, ok := initializeCommands(line, initID); ok {
					d.queue.mu.Lock()
					run.conversation.commands = commands
					d.queue.mu.Unlock()
				}
				return
			}
			d.queue.mu.Lock()
			if frame.Type == "stream_event" {
				if frame.Parent == nil {
					// An event that is not an object leaves the draft as it is.
					var event conversationStream
					_ = json.Unmarshal(frame.Event, &event)
					streamPartial(run.conversation, event)
					if event.Type == "message_start" {
						run.conversation.requestAt = time.Now()
					}
				}
				d.queue.mu.Unlock()
				return
			}
			// A complete message replaces the draft it was streamed as.
			if frame.Type == "assistant" && frame.Parent == nil {
				run.conversation.partial = ""
			}
			if frame.Type == "assistant" && frame.Parent == nil && frame.Message.Usage != nil {
				run.conversation.contextUsed = frame.Message.Usage.contextSize()
			}
			if frame.Type == "system" && frame.Model != "" {
				mainModel = frame.Model
			}
			// The init frame says which MCP servers this turn reached.
			if frame.Type == "system" && frame.MCPServers != nil {
				status := initMCPServer(frame.MCPServers)
				run.conversation.sectileMCP = &status
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
				conversationWriteEvent(run.trace, conversationEvent{Kind: kind, Text: event.Text, Detail: event.Detail, Tool: event.Tool, Input: event.Input, ToolID: event.ToolID})
			}
			for _, result := range runner.ParseToolResults(line) {
				conversationWriteEvent(run.trace, conversationEvent{Kind: "tool_result", Text: result.Text, ToolID: result.ToolID, Truncated: result.Truncated, Error: result.IsError})
			}
			if done {
				// The result ends the turn: closing stdin lets Claude exit.
				d.endTurnInput(run, input)
				d.queue.mu.Lock()
				interrupted := run.conversation.interrupted
				d.queue.mu.Unlock()
				resultSeen, resultFailed = true, frame.IsError
				if frame.IsError && interrupted {
					resultFailed = false
				} else if frame.IsError {
					conversationWrite(run.trace, "error", result, "")
				} else if !assistantSeen && result != "" {
					// A result no message of Claude's came before is the
					// output of a local command such as /usage or /context:
					// text laid out for a terminal, which is not Markdown.
					conversationWrite(run.trace, "command_output", result, "")
				}
				if len(frame.Denials) > 0 {
					conversationWrite(run.trace, "notice", "Some tool calls were denied.", "")
				}
			}
			return
		}
		// JSON Sectile does not read stays out of the chat; only a line that
		// is not JSON at all is Claude Code reporting a problem.
		if strings.TrimSpace(line) != "" && !json.Valid([]byte(line)) {
			conversationWrite(run.trace, "error", line, "")
		}
	}}
	cmd.Stdout = output
	// Keep diagnostics separate so stderr cannot corrupt JSON on stdout.
	var diagnostics limitedConversationBuffer
	cmd.Stderr = &diagnostics
	// The folder read above may have failed; the turn runs regardless.
	err = nil
	if input == nil {
		err = fmt.Errorf("Claude Code's input could not be opened")
	} else if settingsErr != nil {
		err = settingsErr
	}
	release := func() {}
	if err == nil {
		release, err = agentexec.StartDetached(cmd)
	}
	if err == nil {
		d.queue.mu.Lock()
		run.conversation.input = input
		d.queue.mu.Unlock()
		_ = input.send(initialize)
		_ = input.send(conversationUserMessage(prompt))
		waited := make(chan error, 1)
		go func() { waited <- cmd.Wait() }()
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		stopping := false
		// An interrupt is asked of Claude first; a turn that has not ended
		// a few seconds later is stopped, as a stop always is.
		var interruptedAt time.Time
		for waiting := true; waiting; {
			select {
			case err = <-waited:
				waiting = false
			case <-ticker.C:
				d.queue.mu.Lock()
				canceled, interrupted := run.canceled, run.conversation.interrupted
				d.queue.mu.Unlock()
				if interrupted && interruptedAt.IsZero() {
					interruptedAt = time.Now()
					_ = input.send(conversationControl("interrupt"))
				}
				if canceled || interrupted && time.Since(interruptedAt) > conversationInterruptGrace {
					input.close()
					agentexec.StopControlled(cmd, stopping)
					stopping = true
				}
			}
		}
	}
	input.close()
	release()
	if len(output.buffer) > 0 {
		output.line(string(output.buffer))
	}
	if diagnostics.Len() > 0 {
		conversationWrite(run.trace, "notice", diagnostics.String(), "")
	}
	d.queue.mu.Lock()
	defer d.queue.mu.Unlock()
	interrupted := run.conversation.interrupted
	run.conversation.interrupted = false
	run.conversation.input = nil
	if pending := len(run.conversation.approvals); pending > 0 {
		run.conversation.approvals = nil
		conversationWrite(run.trace, "notice", "The turn ended before its tool calls were answered", "")
	}
	markApprovalWaitLocked(run)
	if interrupted && !run.canceled {
		conversationWrite(run.trace, "notice", "Interrupted", "Claude Code stopped this answer. Send a message to continue the conversation.")
	} else if run.canceled {
		run.desktop.Status = conversationStoppedStatus(run)
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
	run.conversation.partial = ""
	// Messages that missed this turn start the next one, unless the
	// conversation was stopped.
	if next := run.conversation.next; len(next) > 0 && !run.canceled {
		run.conversation.next = nil
		run.conversation.busy = true
		go d.conversationTurn(run, strings.Join(next, "\n\n"))
	} else {
		run.conversation.next = nil
	}
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

// conversationProbeTimeout bounds the Claude started only to list commands.
const conversationProbeTimeout = 20 * time.Second

// loadConversationCommandsLocked lists the slash commands of a conversation
// before its first turn: a Claude is started, sent the initialize request
// only, and stopped on its response, which calls no model. Each turn then
// refreshes the list from its own initialize. The queue lock is held.
func (d *agentDaemon) loadConversationCommandsLocked(run *controlledRun) {
	c := run.conversation
	if c.loadingCommands || c.commands != nil || run.desktop.Directory == "" {
		return
	}
	c.loadingCommands = true
	env := map[string]string{"SECTILE_PROJECT_ID": run.desktop.ProjectID}
	for key, value := range c.env {
		env[key] = value
	}
	cmd := claudeConversationCommand(run.desktop.Directory, run.desktop.Model, "", c.mode, "", "", c.extraDirs, env)
	probe := d.probeConversationCommands
	if d.probeCommandsFn != nil {
		probe = d.probeCommandsFn
	}
	go func() {
		commands := probe(cmd)
		d.queue.mu.Lock()
		defer d.queue.mu.Unlock()
		c.loadingCommands = false
		if c.commands == nil {
			// An empty list says the probe ran: it is not tried again.
			c.commands = append([]conversationSlash{}, commands...)
		}
	}()
}

func (d *agentDaemon) probeConversationCommands(cmd *exec.Cmd) []conversationSlash {
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil
	}
	input := &conversationInput{w: stdin}
	defer input.close()
	initialize, id := conversationInitialize()
	found := make(chan []conversationSlash, 1)
	cmd.Stdout = &conversationOutput{line: func(line string) {
		if commands, ok := initializeCommands(line, id); ok {
			select {
			case found <- commands:
			default:
			}
			input.close()
		}
	}}
	var diagnostics limitedConversationBuffer
	cmd.Stderr = &diagnostics
	release, err := agentexec.StartDetached(cmd)
	if err != nil {
		return nil
	}
	defer release()
	_ = input.send(initialize)
	waited := make(chan error, 1)
	go func() { waited <- cmd.Wait() }()
	select {
	case <-waited:
	case <-time.After(conversationProbeTimeout):
		input.close()
		agentexec.StopControlled(cmd, true)
		<-waited
	}
	select {
	case commands := <-found:
		return commands
	default:
		return nil
	}
}
