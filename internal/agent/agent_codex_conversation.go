package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"tasks/internal/agentexec"
	"tasks/internal/runner"
	"tasks/internal/version"
)

// Codex conversations use one app-server process until stopped. The queue lock
// owns conversation state; RPC responses use a separate lock so a caller never
// waits for a response while holding the queue lock.
type codexConversation struct {
	rpc           *codexRPC
	turnID        string
	partialID     string
	model         string
	defaultEffort string
	skills        map[string]string
}

type conversationModelOption struct {
	Model         string   `json:"model"`
	Efforts       []string `json:"efforts"`
	DefaultEffort string   `json:"-"`
}

type conversationDecisionOption struct {
	Decision string `json:"decision"`
	Label    string `json:"label"`
}

func providerConversationModes(provider string) map[string]bool {
	if provider == "codex" {
		return map[string]bool{"read-only": true, "workspace-write": true, "plan": true}
	}
	return conversationModes
}

func providerConversationMode(provider, mode string) string {
	if provider != "codex" {
		return conversationMode(mode)
	}
	if providerConversationModes(provider)[mode] {
		return mode
	}
	return "workspace-write"
}

func providerConversationEfforts(run *controlledRun, chosen ...string) map[string]bool {
	if run.desktop.Provider != "codex" {
		return conversationEfforts
	}
	result := map[string]bool{}
	modelName := run.desktop.Model
	if len(chosen) > 0 && strings.TrimSpace(chosen[0]) != "" {
		modelName = strings.TrimSpace(chosen[0])
	}
	for _, model := range run.conversation.models {
		if model.Model == modelName {
			for _, effort := range model.Efforts {
				result[effort] = true
			}
			return result
		}
	}
	// Before model/list returns, use the documented CLI effort values.
	for _, effort := range []string{"none", "minimal", "low", "medium", "high", "xhigh", "max", "ultra"} {
		result[effort] = true
	}
	return result
}

func codexConversationCommand(directory string, env map[string]string) *exec.Cmd {
	cmd := agentexec.Hidden(exec.Command("codex", "app-server", "--listen", "stdio://"))
	cmd.Dir, cmd.Env = directory, commandEnv(env)
	cmd.WaitDelay = headlessStopGrace
	return cmd
}

type codexFrame struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type codexRequestError struct {
	Method, Message string
	Code            int
}

func (e *codexRequestError) Error() string { return e.Method + ": " + e.Message }

type codexRPC struct {
	input    *conversationInput
	mu       sync.Mutex
	sequence uint64
	pending  map[string]chan codexFrame
	done     chan struct{}
	notify   func(codexFrame)
}

func (rpc *codexRPC) read(line string) {
	var frame codexFrame
	if json.Unmarshal([]byte(line), &frame) != nil {
		return
	}
	if frame.Method != "" {
		rpc.notify(frame)
		return
	}
	rpc.mu.Lock()
	reply := rpc.pending[string(frame.ID)]
	delete(rpc.pending, string(frame.ID))
	rpc.mu.Unlock()
	if reply != nil {
		reply <- frame
	}
}

func (rpc *codexRPC) call(ctx context.Context, method string, params any, result any) error {
	rpc.mu.Lock()
	rpc.sequence++
	id := fmt.Sprintf("sectile-%d", rpc.sequence)
	key, _ := json.Marshal(id)
	reply := make(chan codexFrame, 1)
	rpc.pending[string(key)] = reply
	rpc.mu.Unlock()
	defer func() { rpc.mu.Lock(); delete(rpc.pending, string(key)); rpc.mu.Unlock() }()
	if err := rpc.input.send(map[string]any{"id": id, "method": method, "params": params}); err != nil {
		return err
	}
	select {
	case frame := <-reply:
		if frame.Error != nil {
			return &codexRequestError{Method: method, Message: frame.Error.Message, Code: frame.Error.Code}
		}
		if result != nil {
			return json.Unmarshal(frame.Result, result)
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("%s: %w", method, ctx.Err())
	case <-rpc.done:
		return fmt.Errorf("Codex app-server exited during %s", method)
	}
}

func (rpc *codexRPC) request(method string, params any, result any) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return rpc.call(ctx, method, params, result)
}

// codexConversationTurn starts the persistent supervisor or hands an idle
// supervisor its next message. busy reserves startup before this goroutine runs.
func (d *agentDaemon) codexConversationTurn(run *controlledRun, prompt string) {
	d.queue.mu.Lock()
	c := run.conversation
	if run.canceled {
		c.busy = false
		if c.codex == nil {
			run.desktop.Status = conversationStoppedStatus(run)
			run.once.Do(func() { close(run.exited) })
			run.trace.close()
		}
		d.queue.mu.Unlock()
		return
	}
	if c.codex != nil {
		if prompt != "" {
			c.next = append(c.next, prompt)
		}
		d.queue.mu.Unlock()
		return
	}
	state := &codexConversation{skills: map[string]string{}}
	c.codex = state
	if prompt != "" {
		c.next = append(c.next, prompt)
	}
	directory, projectID := run.desktop.Directory, run.desktop.ProjectID
	env := map[string]string{"SECTILE_PROJECT_ID": projectID}
	for key, value := range c.env {
		env[key] = value
	}
	d.queue.mu.Unlock()

	cmd := codexConversationCommand(directory, env)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		d.endCodexConversation(run, state, err)
		return
	}
	rpc := &codexRPC{input: &conversationInput{w: stdin}, pending: map[string]chan codexFrame{}, done: make(chan struct{})}
	rpc.notify = func(frame codexFrame) { d.codexNotification(run, state, frame) }
	state.rpc = rpc
	output := &conversationOutput{line: rpc.read}
	cmd.Stdout = output
	var diagnostics limitedConversationBuffer
	cmd.Stderr = &diagnostics
	release, err := agentexec.StartDetached(cmd)
	if err != nil {
		rpc.input.close()
		d.endCodexConversation(run, state, err)
		return
	}
	waited := make(chan error, 1)
	go func() { err := cmd.Wait(); close(rpc.done); waited <- err }()
	watchDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		stopping := false
		for {
			select {
			case <-watchDone:
				return
			case <-rpc.done:
				return
			case <-ticker.C:
				if d.queue.canceled(run) {
					agentexec.StopControlled(cmd, stopping)
					stopping = true
				}
			}
		}
	}()
	defer close(watchDone)
	defer func() {
		rpc.input.close()
		agentexec.StopControlled(cmd, false)
		select {
		case <-waited:
		case <-time.After(headlessStopGrace):
			agentexec.StopControlled(cmd, true)
			<-waited
		}
		release()
		d.endCodexConversation(run, state, err)
	}()
	err = rpc.request("initialize", map[string]any{"clientInfo": map[string]string{"name": "sectile", "title": "Sectile", "version": version.Version}, "capabilities": map[string]bool{"experimentalApi": true}}, nil)
	if err == nil {
		err = rpc.input.send(map[string]any{"method": "initialized"})
	}
	if err == nil {
		err = d.initializeCodexConversation(run, state)
	}
	if err != nil {
		err = fmt.Errorf("Codex conversation could not start; check your Codex installation and sign-in: %w", err)
		return
	}
	d.queue.mu.Lock()
	c.input = rpc.input
	d.queue.mu.Unlock()
	d.loadCodexCatalog(run, state)
	d.refreshCodexMCP(run, state, false)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	var interruptAt time.Time
	for {
		select {
		case <-rpc.done:
			err = fmt.Errorf("Codex app-server exited. %s", strings.TrimSpace(diagnostics.String()))
			return
		case <-ticker.C:
			d.queue.mu.Lock()
			canceled, interrupted, turnID := run.canceled, c.interrupted, state.turnID
			messages := append([]string{}, c.next...)
			if len(messages) > 0 && !interrupted {
				c.next = nil
			}
			d.queue.mu.Unlock()
			if canceled {
				err = nil
				return
			}
			if interrupted {
				if turnID == "" {
					d.queue.mu.Lock()
					c.interrupted = false
					c.busy = len(c.next) > 0
					conversationWrite(run.trace, "notice", "Interrupted", "Codex stopped this answer. Send a message to continue.")
					d.queue.mu.Unlock()
					interruptAt = time.Time{}
				} else if interruptAt.IsZero() {
					interruptAt = time.Now()
					if interruptErr := rpc.request("turn/interrupt", map[string]string{"threadId": c.session, "turnId": turnID}, nil); interruptErr != nil {
						err = interruptErr
						return
					}
				} else if time.Since(interruptAt) > conversationInterruptGrace {
					err = fmt.Errorf("Codex did not confirm the interruption; its process was stopped. Send a message to resume.")
					return
				}
				continue
			}
			interruptAt = time.Time{}
			if len(messages) == 0 {
				continue
			}
			message := strings.Join(messages, "\n\n")
			if turnID != "" {
				err := rpc.request("turn/steer", map[string]any{"threadId": c.session, "expectedTurnId": turnID, "input": codexTextInput(message)}, nil)
				if err == nil {
					continue
				}
				// Only a confirmed end of this turn permits retrying as the next turn.
				// A transport failure is ambiguous and must never duplicate a message.
				d.queue.mu.Lock()
				var rejected *codexRequestError
				ended := errors.As(err, &rejected) && state.turnID != turnID
				if ended {
					c.next = append(messages, c.next...)
					c.busy = true
				}
				d.queue.mu.Unlock()
				if !ended {
					conversationWrite(run.trace, "error", "The message could not join Codex's answer", err.Error())
				}
				continue
			}
			if startErr := d.startCodexTurn(run, state, message); startErr != nil {
				var rejected *codexRequestError
				if !errors.As(startErr, &rejected) {
					err = startErr
					return
				}
				d.queue.mu.Lock()
				c.busy = false
				c.partial = ""
				d.queue.mu.Unlock()
				conversationWrite(run.trace, "error", "Codex could not start this answer", startErr.Error())
			}
		}
	}
}

func codexTextInput(text string) []map[string]string {
	return []map[string]string{{"type": "text", "text": text}}
}

func codexSkillInput(message string, skills map[string]string) []map[string]string {
	input := codexTextInput(message)
	if fields := strings.Fields(message); len(fields) > 0 {
		name := strings.TrimPrefix(fields[0], "$")
		if path := skills[name]; path != "" && strings.HasPrefix(fields[0], "$") {
			input = append(input, map[string]string{"type": "skill", "name": name, "path": path})
		}
	}
	return input
}

func (d *agentDaemon) initializeCodexConversation(run *controlledRun, state *codexConversation) error {
	reviewer := d.workstationCodexReviewer()
	d.queue.mu.Lock()
	c := run.conversation
	params := map[string]any{"cwd": run.desktop.Directory, "approvalPolicy": "on-request", "sandbox": "workspace-write", "approvalsReviewer": reviewer}
	if run.desktop.Model != "" {
		params["model"] = run.desktop.Model
	}
	method := "thread/start"
	if c.session != "" {
		method = "thread/resume"
		params["threadId"] = c.session
		params["excludeTurns"] = true
	}
	d.queue.mu.Unlock()
	var response struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
		Model  string `json:"model"`
		Effort string `json:"reasoningEffort"`
	}
	if err := state.rpc.request(method, params, &response); err != nil {
		return err
	}
	if response.Thread.ID == "" || response.Model == "" {
		return fmt.Errorf("Codex returned no thread or model")
	}
	d.queue.mu.Lock()
	c.session, state.model, state.defaultEffort = response.Thread.ID, response.Model, response.Effort
	if run.desktop.Model == "" {
		run.desktop.Model = response.Model
	}
	d.queue.mu.Unlock()
	return nil
}

func (d *agentDaemon) startCodexTurn(run *controlledRun, state *codexConversation, message string) error {
	reviewer := d.workstationCodexReviewer()
	folders, _, err := d.conversationFolders(run.desktop.ProjectID, run.desktop.Directory)
	if err != nil {
		conversationWrite(run.trace, "notice", "Attached folders could not be read for this message", err.Error())
	}
	d.queue.mu.Lock()
	c := run.conversation
	dirs := append(folderMapDirs(folders), c.extraDirs...)
	if err == nil {
		refreshConversationFolders(run, folders)
	}
	mode := providerConversationMode("codex", c.mode)
	sandbox := map[string]any{"type": "readOnly", "networkAccess": false}
	if mode == "workspace-write" {
		sandbox = map[string]any{"type": "workspaceWrite", "writableRoots": append([]string{run.desktop.Directory}, dirs...), "networkAccess": false}
	}
	effort := c.effort
	if effort == "" {
		effort = state.defaultEffort
		for _, model := range c.models {
			if model.Model == run.desktop.Model && model.DefaultEffort != "" {
				effort = model.DefaultEffort
				break
			}
		}
	}
	var wireEffort any
	if effort != "" {
		wireEffort = effort
	}
	params := map[string]any{"threadId": c.session, "input": codexSkillInput(message, state.skills), "cwd": run.desktop.Directory, "approvalPolicy": "on-request", "sandboxPolicy": sandbox, "approvalsReviewer": reviewer, "model": run.desktop.Model, "effort": wireEffort}
	collaboration := "default"
	if mode == "plan" {
		collaboration = "plan"
	}
	params["collaborationMode"] = map[string]any{"mode": collaboration, "settings": map[string]any{"model": run.desktop.Model, "reasoning_effort": wireEffort, "developer_instructions": nil}}
	c.busy = true
	d.queue.mu.Unlock()
	var result struct {
		Turn struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"turn"`
	}
	if err := state.rpc.request("turn/start", params, &result); err != nil {
		return err
	}
	if result.Turn.ID == "" {
		return fmt.Errorf("Codex returned no turn")
	}
	// turn/started and turn/completed are authoritative. A fast completed turn
	// may already have arrived before this response; never resurrect it here.
	return nil
}

func (d *agentDaemon) endCodexConversation(run *controlledRun, state *codexConversation, err error) {
	d.queue.mu.Lock()
	defer d.queue.mu.Unlock()
	c := run.conversation
	if c.codex != state {
		return
	}
	c.codex, c.input = nil, nil
	c.busy, c.interrupted = false, false
	c.partial = ""
	c.approvals = nil
	markApprovalWaitLocked(run)
	if len(c.next) > 0 {
		conversationWrite(run.trace, "notice", "Unsent messages were not retried", "Send them again to resume the conversation.")
		c.next = nil
	}
	if run.canceled {
		run.desktop.Status = conversationStoppedStatus(run)
		run.once.Do(func() { close(run.exited) })
		conversationWrite(run.trace, "notice", "Conversation stopped", "")
		run.trace.close()
	} else if err != nil {
		conversationWrite(run.trace, "error", err.Error(), "")
	}
}

// codexNotification normalizes Codex items into the existing bounded transcript.
// Requests retain their original wire id, including numeric ids.
func (d *agentDaemon) codexNotification(run *controlledRun, state *codexConversation, frame codexFrame) {
	d.queue.mu.Lock()
	defer d.queue.mu.Unlock()
	c := run.conversation
	if c.codex != state || run.canceled {
		return
	}
	var params struct {
		ThreadID  string          `json:"threadId"`
		TurnID    string          `json:"turnId"`
		ItemID    string          `json:"itemId"`
		RequestID json.RawMessage `json:"requestId"`
		Delta     string          `json:"delta"`
		Turn      struct {
			ID     string `json:"id"`
			Status string `json:"status"`
			Error  *struct {
				Message string `json:"message"`
			} `json:"error"`
		} `json:"turn"`
		Item       json.RawMessage `json:"item"`
		TokenUsage struct {
			Last struct {
				Input  int `json:"inputTokens"`
				Output int `json:"outputTokens"`
			} `json:"last"`
			Window int `json:"modelContextWindow"`
		} `json:"tokenUsage"`
	}
	if json.Unmarshal(frame.Params, &params) != nil {
		return
	}
	if len(frame.ID) > 0 && string(frame.ID) != "null" {
		approval, ok := codexApproval(frame)
		if !ok {
			_ = state.rpc.input.send(map[string]any{"id": frame.ID, "error": map[string]any{"code": -32601, "message": "Sectile does not support this Codex request"}})
			conversationWrite(run.trace, "notice", "Codex requested an unsupported interaction", frame.Method)
			return
		}
		c.approvals = append(c.approvals, approval)
		markApprovalWaitLocked(run)
		return
	}
	if params.ThreadID != "" && c.session != "" && params.ThreadID != c.session {
		return
	}
	switch frame.Method {
	case "turn/started":
		state.turnID = params.Turn.ID
		c.busy = true
	case "turn/completed":
		if state.turnID != "" && state.turnID != params.Turn.ID {
			return
		}
		state.turnID = ""
		state.partialID = ""
		c.partial = ""
		c.approvals = nil
		markApprovalWaitLocked(run)
		c.busy = len(c.next) > 0
		if params.Turn.Error != nil {
			conversationWrite(run.trace, "error", params.Turn.Error.Message, "")
		} else if params.Turn.Status == "interrupted" {
			conversationWrite(run.trace, "notice", "Interrupted", "Codex stopped this answer. Send a message to continue.")
		} else if params.Turn.Status == "completed" {
			conversationWrite(run.trace, "notice", "Ready for your next message", "")
		}
		c.interrupted = false
	case "item/agentMessage/delta", "item/plan/delta":
		if state.partialID != params.ItemID {
			c.partial = ""
			state.partialID = params.ItemID
		}
		if len(c.partial)+len(params.Delta) <= conversationPartialLimit {
			c.partial += params.Delta
		}
	case "item/started", "item/completed":
		events := codexItemEvents(params.Item, frame.Method == "item/completed")
		for _, event := range events {
			conversationWriteEvent(run.trace, event)
		}
		if frame.Method == "item/completed" && params.ItemID == state.partialID {
			c.partial = ""
			state.partialID = ""
		}
		// Item notifications carry the id inside item rather than itemId.
		var item struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(params.Item, &item)
		if frame.Method == "item/completed" && item.ID == state.partialID {
			c.partial = ""
			state.partialID = ""
		}
	case "thread/tokenUsage/updated":
		c.contextUsed = params.TokenUsage.Last.Input + params.TokenUsage.Last.Output
		c.contextWindow = params.TokenUsage.Window
	case "serverRequest/resolved":
		for i, approval := range c.approvals {
			if string(approval.RequestID) == string(params.RequestID) {
				c.approvals = append(c.approvals[:i:i], c.approvals[i+1:]...)
				break
			}
		}
		markApprovalWaitLocked(run)
	case "error":
		var detail struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(frame.Params, &detail)
		if detail.Error.Message != "" {
			conversationWrite(run.trace, "error", detail.Error.Message, "")
		}
	}
}

func codexItemEvents(raw json.RawMessage, completed bool) []conversationEvent {
	var item struct {
		ID        string          `json:"id"`
		Type      string          `json:"type"`
		Text      string          `json:"text"`
		Command   string          `json:"command"`
		CWD       string          `json:"cwd"`
		Output    string          `json:"aggregatedOutput"`
		Status    string          `json:"status"`
		ExitCode  *int            `json:"exitCode"`
		Summary   []string        `json:"summary"`
		Content   []string        `json:"content"`
		Server    string          `json:"server"`
		Tool      string          `json:"tool"`
		Arguments json.RawMessage `json:"arguments"`
		Result    json.RawMessage `json:"result"`
		Error     json.RawMessage `json:"error"`
		Changes   []struct {
			Path string `json:"path"`
			Diff string `json:"diff"`
		} `json:"changes"`
		Query string `json:"query"`
	}
	if json.Unmarshal(raw, &item) != nil {
		return nil
	}
	event := conversationEvent{Kind: "tool", ToolID: item.ID}
	switch item.Type {
	case "agentMessage", "plan":
		if completed && item.Text != "" {
			return []conversationEvent{{Kind: "assistant", Text: item.Text}}
		}
		return nil
	case "reasoning":
		if completed {
			text := strings.Join(item.Summary, "\n\n")
			if text == "" {
				text = strings.Join(item.Content, "\n\n")
			}
			if text != "" {
				return []conversationEvent{{Kind: "thinking", Text: text}}
			}
		}
		return nil
	case "commandExecution":
		event.Tool, event.Text, event.Detail = "Bash", "Bash", clampShellDetail(item.Command)
		event.Input, _ = json.Marshal(map[string]string{"command": item.Command, "description": item.CWD})
	case "fileChange":
		event.Tool, event.Text = "Codex file changes", "Codex file changes"
		event.Input = raw
		var diffs []string
		for _, change := range item.Changes {
			diffs = append(diffs, change.Path)
		}
		event.Detail = strings.Join(diffs, "\n\n")
	case "mcpToolCall":
		event.Tool = "mcp__" + item.Server + "__" + item.Tool
		event.Text, event.Input = item.Tool, item.Arguments
	case "webSearch":
		event.Tool, event.Text, event.Detail = "WebSearch", "WebSearch", item.Query
		event.Input, _ = json.Marshal(map[string]string{"query": item.Query})
	default:
		return nil
	}
	if !completed {
		if len(event.Input) > runner.ToolInputLimit {
			event.Input = nil
		}
		event.Detail, _ = codexToolText(event.Detail)
		return []conversationEvent{event}
	}
	text := item.Output
	if item.Type == "mcpToolCall" {
		text = codexMCPText(item.Result)
		if len(item.Error) > 0 && string(item.Error) != "null" {
			text = string(item.Error)
		}
	}
	if item.Type == "fileChange" {
		text = item.Status
	}
	failed := item.Status == "failed" || item.Status == "declined" || (item.ExitCode != nil && *item.ExitCode != 0)
	if len(item.Error) > 0 && string(item.Error) != "null" {
		failed = true
	}
	text, truncated := codexToolText(text)
	return []conversationEvent{{Kind: "tool_result", ToolID: item.ID, Text: text, Error: failed, Truncated: truncated}}
}

// Bound text on a character boundary, as Claude's tool-result parser does.
func codexToolText(text string) (string, bool) {
	if len(text) <= runner.ToolResultLimit {
		return text, false
	}
	cut := runner.ToolResultLimit
	for cut > 0 && !utf8.ValidString(text[:cut]) {
		cut--
	}
	return text[:cut], true
}

func codexMCPText(raw json.RawMessage) string {
	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if json.Unmarshal(raw, &result) == nil {
		texts := []string{}
		for _, block := range result.Content {
			if block.Type == "text" {
				texts = append(texts, block.Text)
			}
		}
		if len(texts) > 0 {
			return strings.Join(texts, "\n")
		}
	}
	return string(raw)
}

func codexApproval(frame codexFrame) (conversationApproval, bool) {
	var params struct {
		ItemID                string            `json:"itemId"`
		Command               string            `json:"command"`
		CWD                   string            `json:"cwd"`
		Reason                string            `json:"reason"`
		Available             []json.RawMessage `json:"availableDecisions"`
		Permissions           json.RawMessage   `json:"permissions"`
		AdditionalPermissions json.RawMessage   `json:"additionalPermissions"`
		Network               *struct {
			Host     string `json:"host"`
			Protocol string `json:"protocol"`
		} `json:"networkApprovalContext"`
		Questions []struct {
			ID       string          `json:"id"`
			Header   string          `json:"header"`
			Question string          `json:"question"`
			Secret   bool            `json:"isSecret"`
			Options  json.RawMessage `json:"options"`
		} `json:"questions"`
	}
	if json.Unmarshal(frame.Params, &params) != nil {
		return conversationApproval{}, false
	}
	approval := conversationApproval{ID: "codex-" + string(frame.ID), RequestID: frame.ID, Method: frame.Method, ToolUseID: params.ItemID, Reason: params.Reason, Input: frame.Params, WireDecisions: map[string]string{"allow": "accept", "always": "acceptForSession", "deny": "decline"}}
	choices := []conversationDecisionOption{{"allow", "Allow"}, {"always", "Allow for this conversation"}, {"deny", "Deny"}}
	switch frame.Method {
	case "item/commandExecution/requestApproval":
		approval.Tool = "Bash"
		approval.Input, _ = json.Marshal(map[string]string{"command": params.Command, "description": params.CWD})
		approval.Access = params.AdditionalPermissions
		if params.Network != nil {
			approval.Tool = "Codex network access"
			approval.Description = params.Network.Protocol + " access to " + params.Network.Host
			approval.Input = frame.Params
		}
		if len(params.Available) > 0 {
			choices = nil
			approval.WireDecisions = map[string]string{}
			for _, choice := range []struct{ Wire, Decision, Label string }{{"accept", "allow", "Allow"}, {"acceptForSession", "always", "Allow for this conversation"}, {"decline", "deny", "Deny"}} {
				for _, available := range params.Available {
					var value string
					_ = json.Unmarshal(available, &value)
					if value == choice.Wire {
						choices = append(choices, conversationDecisionOption{choice.Decision, choice.Label})
						approval.WireDecisions[choice.Decision] = choice.Wire
					}
				}
			}
			if approval.WireDecisions["deny"] == "" {
				for _, available := range params.Available {
					if string(available) == `"cancel"` {
						choices = append(choices, conversationDecisionOption{"deny", "Deny and stop answer"})
						approval.WireDecisions["deny"] = "cancel"
						break
					}
				}
			}
		}
	case "item/fileChange/requestApproval":
		approval.Tool = "Codex file changes"
	case "item/permissions/requestApproval":
		approval.Tool = "Codex permissions"
		approval.Input = params.Permissions
	case "item/tool/requestUserInput":
		approval.Tool = askUserQuestion
		approval.QuestionIDs = map[string]string{}
		approval.QuestionSecrets = map[string]bool{}
		questions := []map[string]any{}
		for _, q := range params.Questions {
			if q.Question == "" || approval.QuestionIDs[q.Question] != "" {
				return conversationApproval{}, false
			}
			approval.QuestionIDs[q.Question] = q.ID
			approval.QuestionSecrets[q.Question] = q.Secret
			questions = append(questions, map[string]any{"question": q.Question, "header": q.Header, "options": q.Options, "isSecret": q.Secret})
		}
		if len(questions) == 0 {
			return conversationApproval{}, false
		}
		approval.Input, _ = json.Marshal(map[string]any{"questions": questions})
		choices = nil
	default:
		return conversationApproval{}, false
	}
	approval.Choices = choices
	return approval, true
}

func (d *agentDaemon) decideCodexApprovalLocked(run *controlledRun, id, decision string, answers map[string]string) error {
	c := run.conversation
	for i, approval := range c.approvals {
		if approval.ID != id {
			continue
		}
		if c.input == nil {
			return fmt.Errorf("the turn has ended")
		}
		if err := checkAnswers(approval, decision, answers); err != nil {
			return err
		}
		result := map[string]any{}
		if approval.Tool == askUserQuestion {
			if decision != "answer" && decision != "deny" {
				return fmt.Errorf("a question takes an answer or skip")
			}
			values := map[string]any{}
			for question, key := range approval.QuestionIDs {
				values[key] = map[string]any{"answers": []string{answers[question]}}
				if decision == "deny" {
					values[key] = map[string]any{"answers": []string{}}
				}
			}
			result["answers"] = values
		} else {
			allowed := false
			for _, choice := range approval.Choices {
				if choice.Decision == decision {
					allowed = true
				}
			}
			if !allowed {
				return fmt.Errorf("Codex did not offer that decision")
			}
			wire := approval.WireDecisions[decision]
			result["decision"] = wire
			if approval.Method == "item/permissions/requestApproval" {
				permissions := json.RawMessage("{}")
				if decision != "deny" {
					permissions = approval.Input
				}
				scope := "turn"
				if decision == "always" {
					scope = "session"
				}
				result = map[string]any{"permissions": permissions, "scope": scope}
			}
		}
		if err := c.input.send(map[string]any{"id": approval.RequestID, "result": result}); err != nil {
			return err
		}
		c.approvals = append(c.approvals[:i:i], c.approvals[i+1:]...)
		markApprovalWaitLocked(run)
		event := conversationEvent{Kind: "approval", Text: decision, Tool: approval.Tool, ToolID: approval.ToolUseID}
		if decision == "answer" {
			safeAnswers := map[string]string{}
			for question, answer := range answers {
				if approval.QuestionSecrets[question] {
					safeAnswers[question] = "[hidden]"
				} else {
					safeAnswers[question] = answer
				}
			}
			event.Detail = answersDetail(safeAnswers)
		}
		conversationWriteEvent(run.trace, event)
		return nil
	}
	return fmt.Errorf("no tool call is waiting for that decision")
}

func (d *agentDaemon) loadCodexCatalog(run *controlledRun, state *codexConversation) {
	var models struct {
		Data []struct {
			Model         string `json:"model"`
			Hidden        bool   `json:"hidden"`
			DefaultEffort string `json:"defaultReasoningEffort"`
			Efforts       []struct {
				Effort string `json:"reasoningEffort"`
			} `json:"supportedReasoningEfforts"`
		} `json:"data"`
	}
	if err := state.rpc.request("model/list", map[string]any{}, &models); err == nil {
		options := []conversationModelOption{}
		for _, model := range models.Data {
			if model.Hidden {
				continue
			}
			option := conversationModelOption{Model: model.Model, Efforts: []string{}, DefaultEffort: model.DefaultEffort}
			for _, effort := range model.Efforts {
				option.Efforts = append(option.Efforts, effort.Effort)
			}
			options = append(options, option)
		}
		d.queue.mu.Lock()
		run.conversation.models = options
		d.queue.mu.Unlock()
	}
	var skills struct {
		Data []struct {
			Skills []struct {
				Name        string `json:"name"`
				Description string `json:"description"`
				Enabled     bool   `json:"enabled"`
				Path        string `json:"path"`
			} `json:"skills"`
		} `json:"data"`
	}
	if err := state.rpc.request("skills/list", map[string]any{"cwds": []string{run.desktop.Directory}}, &skills); err == nil {
		commands := []conversationSlash{{Name: "mcp", Description: "Check MCP servers"}}
		paths := map[string]string{}
		for _, entry := range skills.Data {
			for _, skill := range entry.Skills {
				if skill.Enabled {
					paths[skill.Name] = skill.Path
					commands = append(commands, conversationSlash{Name: "$" + skill.Name, Description: skill.Description})
				}
			}
		}
		d.queue.mu.Lock()
		run.conversation.commands = commands
		state.skills = paths
		d.queue.mu.Unlock()
	}
}

func (d *agentDaemon) checkCodexMCPLocked(run *controlledRun, show bool) {
	c := run.conversation
	if c.checkingMCP {
		return
	}
	if c.codex == nil || c.input == nil {
		c.sectileMCP = &conversationMCP{Status: "unknown", Detail: "Send a message to connect Codex and check its MCP servers."}
		if show {
			conversationWrite(run.trace, "command_output", c.sectileMCP.Detail, "")
		}
		return
	}
	c.checkingMCP = true
	state := c.codex
	go d.refreshCodexMCP(run, state, show)
}

func (d *agentDaemon) refreshCodexMCP(run *controlledRun, state *codexConversation, show bool) {
	d.queue.mu.Lock()
	session := run.conversation.session
	d.queue.mu.Unlock()
	var response struct {
		Data []struct {
			Name    string                     `json:"name"`
			Auth    string                     `json:"authStatus"`
			Runtime string                     `json:"runtimeStatus"`
			Error   string                     `json:"toolsError"`
			Tools   map[string]json.RawMessage `json:"tools"`
		} `json:"data"`
	}
	err := state.rpc.request("mcpServerStatus/list", map[string]any{"threadId": session}, &response)
	status := conversationMCP{Status: "missing", Detail: "Sectile's MCP server is not registered in Codex."}
	lines := []string{}
	for _, server := range response.Data {
		current := "unknown"
		switch {
		case server.Runtime == "connected":
			current = "connected"
		case server.Error != "" || server.Runtime == "failed":
			current = "failed"
		case server.Auth == "notLoggedIn" || server.Runtime == "authenticationRequired":
			current = "needs-auth"
		case server.Runtime == "starting" || server.Runtime == "notStarted":
			current = "pending"
		case server.Runtime == "" && server.Error == "" && len(server.Tools) > 0:
			current = "connected"
		}
		lines = append(lines, server.Name+": "+current)
		if server.Name == sectileMCPName {
			status = conversationMCP{Status: current, Detail: server.Error}
		}
	}
	if err != nil {
		status = conversationMCP{Status: "unknown", Detail: err.Error()}
	}
	d.queue.mu.Lock()
	defer d.queue.mu.Unlock()
	if run.conversation.codex != state {
		return
	}
	run.conversation.checkingMCP = false
	run.conversation.sectileMCP = &status
	if show {
		text := strings.Join(lines, "\n")
		if err != nil {
			text = err.Error()
		}
		if text == "" {
			text = "No MCP servers registered in Codex."
		}
		conversationWrite(run.trace, "command_output", text, "")
	}
}
