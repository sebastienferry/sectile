package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"

	"tasks/internal/agentexec"
	"tasks/internal/runner"

	"github.com/google/uuid"
)

// agyConversationModes are the permission modes the owner can pick in an
// Antigravity conversation:
//   - "accept-edits": passes --mode=accept-edits (default)
//   - "default": passes no mode flag so agy follows the user's rules (request-review)
//   - "auto": passes --dangerously-skip-permissions to auto-approve all tools
//   - "plan": passes --mode=plan for read-only planning
var agyConversationModes = map[string]bool{
	"default":      true,
	"accept-edits": true,
	"auto":         true,
	"plan":         true,
}

// agyLocalCommands are the slash commands Antigravity answers in the CLI itself
// rather than forwarding over --input-format stream-json. They are invoked via
// -p=<prompt> with --output-format stream-json.
var agyLocalCommands = map[string]bool{
	"/agents":      true,
	"/changelog":   true,
	"/config":      true,
	"/settings":    true,
	"/credits":     true,
	"/effort":      true,
	"/help":        true,
	"/hooks":       true,
	"/model":       true,
	"/permissions": true,
	"/plugin":      true,
	"/skills":      true,
	"/usage":       true,
	"/quota":       true,
}

// agyBuiltinSlashCommands are offered in the composer's "/" completion even
// before or without a dynamic skill probe.
var agyBuiltinSlashCommands = []conversationSlash{
	{Name: "help", Description: "Show available commands and keybindings"},
	{Name: "usage", Description: "View model quota usage"},
	{Name: "credits", Description: "Show remaining G1 credits and purchase link"},
	{Name: "skills", Description: "List available skills"},
	{Name: "agents", Description: "List available custom agents"},
	{Name: "model", Description: "Show or set the active model"},
	{Name: "effort", Description: "Show or set the reasoning effort"},
	{Name: "permissions", Description: "Show or manage tool permissions"},
	{Name: "plugin", Description: "List and manage plugins"},
	{Name: "hooks", Description: "List hook configurations"},
	{Name: "changelog", Description: "Show release notes and changes"},
	{Name: "mcp", Description: "Check MCP servers"},
}

func isAgyLocalSlashCommand(prompt string) bool {
	fields := strings.Fields(strings.TrimSpace(prompt))
	if len(fields) == 0 {
		return false
	}
	return agyLocalCommands[strings.ToLower(fields[0])]
}

// agyConversationCommand builds one turn's Antigravity CLI invocation.
// Regular turns read the user prompt as an NDJSON frame on stdin via
// --input-format stream-json --output-format stream-json -p=. Local CLI slash
// commands (/help, /usage, /skills, ...) are passed via -p=<slashPrompt>
// because agy rejects CLI-handled slash commands on stream-json stdin.
func agyConversationCommand(directory, model, effort, mode, session, slashPrompt string, dirs []string, env map[string]string) *exec.Cmd {
	var args []string
	if slash := strings.TrimSpace(slashPrompt); slash != "" {
		args = []string{"--output-format", "stream-json", "-p=" + slash}
	} else {
		args = []string{"--input-format", "stream-json", "--output-format", "stream-json", "-p="}
	}
	if session != "" && uuid.Validate(session) == nil {
		args = append(args, "--conversation="+session)
	}
	if model = strings.TrimSpace(model); model != "" {
		args = append(args, "--model="+model)
	}
	if conversationEfforts[effort] {
		args = append(args, "--effort="+effort)
	}
	switch providerConversationMode("agy", mode) {
	case "accept-edits":
		args = append(args, "--mode=accept-edits")
	case "plan":
		args = append(args, "--mode=plan")
	case "auto":
		args = append(args, "--dangerously-skip-permissions")
	}
	for _, dir := range dirs {
		if dir = strings.TrimSpace(dir); dir != "" {
			args = append(args, "--add-dir="+dir)
		}
	}
	cmd := agentexec.Hidden(exec.Command("agy", args...))
	cmd.Dir = directory
	cmd.Env = commandEnv(env)
	cmd.WaitDelay = headlessStopGrace
	return cmd
}

func agyUserMessage(prompt string) map[string]any {
	return map[string]any{
		"event": "user",
		"message": map[string]any{
			"role":    "user",
			"content": prompt,
		},
	}
}

type agyUsage struct {
	InputTokens     int `json:"input_tokens"`
	OutputTokens    int `json:"output_tokens"`
	ThinkingTokens  int `json:"thinking_tokens"`
	CacheReadTokens int `json:"cache_read_tokens"`
	TotalTokens     int `json:"total_tokens"`
}

func (u agyUsage) contextSize() int {
	if u.InputTokens > 0 || u.CacheReadTokens > 0 || u.OutputTokens > 0 {
		return u.InputTokens + u.CacheReadTokens + u.OutputTokens + u.ThinkingTokens
	}
	return u.TotalTokens
}

type agyToolError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type agyToolInfo struct {
	Name       string          `json:"name"`
	Parameters json.RawMessage `json:"parameters"`
	Output     string          `json:"output"`
	Error      *agyToolError   `json:"error"`
}

type agyStepUpdate struct {
	ConversationID  string       `json:"conversation_id"`
	StepIndex       int          `json:"step_index"`
	State           string       `json:"state"`
	StepType        string       `json:"step_type"`
	TextDelta       string       `json:"text_delta"`
	ThinkingDelta   string       `json:"thinking_delta"`
	ToolName        string       `json:"tool_name"`
	DurationSeconds float64      `json:"duration_seconds"`
	ToolInfo        *agyToolInfo `json:"tool_info"`
	Usage           *agyUsage    `json:"usage"`
}

type agyDeniedAction struct {
	Tool  string `json:"tool"`
	Scope string `json:"scope"`
	Rule  string `json:"rule"`
}

type agyResult struct {
	ConversationID  string            `json:"conversation_id"`
	Status          string            `json:"status"`
	Response        string            `json:"response"`
	Error           string            `json:"error"`
	DurationSeconds float64           `json:"duration_seconds"`
	NumTurns        int               `json:"num_turns"`
	Usage           *agyUsage         `json:"usage"`
	Command         json.RawMessage   `json:"command"`
	DeniedActions   []agyDeniedAction `json:"denied_actions"`
}

type agyFrame struct {
	Event          string          `json:"event"`
	ConversationID string          `json:"conversation_id"`
	StepUpdate     *agyStepUpdate  `json:"step_update"`
	Command        json.RawMessage `json:"command"`
	Result         *agyResult      `json:"result"`
}

type agyStreamState struct {
	sessionID          string
	localSlash         bool
	thinkingByStep     map[int]string
	textByStep         map[int]string
	thinkingFlushed    map[int]bool
	textFlushed        map[int]bool
	toolEmitted        map[int]bool
	toolResultEmitted  map[int]bool
	stepOrder          []int
	stepSeen           map[int]bool
	wroteAssistant     bool
	wroteCommandOutput bool
	contextUsed        int
	resultStatus       string
	resultError        string
	deniedActions      []agyDeniedAction
	discoveredCommands []conversationSlash
}

func newAgyStreamState(localSlash bool) *agyStreamState {
	return &agyStreamState{
		localSlash:        localSlash,
		thinkingByStep:    map[int]string{},
		textByStep:        map[int]string{},
		thinkingFlushed:   map[int]bool{},
		textFlushed:       map[int]bool{},
		toolEmitted:       map[int]bool{},
		toolResultEmitted: map[int]bool{},
		stepSeen:          map[int]bool{},
	}
}

func (s *agyStreamState) noteStep(idx int) {
	if !s.stepSeen[idx] {
		s.stepSeen[idx] = true
		s.stepOrder = append(s.stepOrder, idx)
		sort.Ints(s.stepOrder)
	}
}

func (s *agyStreamState) flushThinking(trace *runTrace, idx int) {
	if s.thinkingFlushed[idx] {
		return
	}
	if text := strings.TrimSpace(s.thinkingByStep[idx]); text != "" {
		conversationWrite(trace, "thinking", text, "")
	}
	s.thinkingFlushed[idx] = true
}

func (s *agyStreamState) flushText(trace *runTrace, idx int) {
	s.flushThinking(trace, idx)
	if s.textFlushed[idx] {
		return
	}
	if text := strings.TrimSpace(s.textByStep[idx]); text != "" {
		conversationWrite(trace, "assistant", text, "")
		s.wroteAssistant = true
	}
	s.textFlushed[idx] = true
}

func (s *agyStreamState) flushPriorSteps(trace *runTrace, beforeIdx int) {
	for _, idx := range s.stepOrder {
		if idx < beforeIdx {
			s.flushText(trace, idx)
		}
	}
}

func (s *agyStreamState) flushAll(trace *runTrace) {
	for _, idx := range s.stepOrder {
		s.flushText(trace, idx)
	}
}

func (s *agyStreamState) handleLine(line string, trace *runTrace, onPartial func(delta string, clear bool)) (done bool) {
	var frame agyFrame
	if json.Unmarshal([]byte(line), &frame) != nil || frame.Event == "" {
		return false
	}
	if uuid.Validate(frame.ConversationID) == nil {
		s.sessionID = frame.ConversationID
	}
	switch frame.Event {
	case "init":
		return false
	case "command_result":
		if cmds := parseAgyCommandResultSlash(frame.Command); len(cmds) > 0 {
			s.discoveredCommands = cmds
		}
		return false
	case "step_update":
		step := frame.StepUpdate
		if step == nil {
			return false
		}
		if uuid.Validate(step.ConversationID) == nil {
			s.sessionID = step.ConversationID
		}
		if step.Usage != nil {
			if used := step.Usage.contextSize(); used > 0 {
				s.contextUsed = used
			}
		}
		if step.StepType == "user_input" {
			return false
		}
		idx := step.StepIndex
		s.noteStep(idx)
		isTool := step.StepType == "tool" || step.ToolName != "" || step.ToolInfo != nil
		if isTool {
			s.flushPriorSteps(trace, idx)
			if onPartial != nil {
				onPartial("", true)
			}
			if !s.toolEmitted[idx] {
				s.toolEmitted[idx] = true
				conversationWriteEvent(trace, agyToolEvent(idx, step.ToolName, step.ToolInfo))
			}
			if step.State == "DONE" || step.State == "ERROR" {
				if !s.toolResultEmitted[idx] {
					s.toolResultEmitted[idx] = true
					conversationWriteEvent(trace, agyToolResultEvent(idx, step.State, step.ToolInfo))
				}
			}
			return false
		}
		if step.ThinkingDelta != "" {
			s.thinkingByStep[idx] += step.ThinkingDelta
		}
		if step.TextDelta != "" {
			s.flushThinking(trace, idx)
			s.textByStep[idx] += step.TextDelta
			if onPartial != nil {
				onPartial(step.TextDelta, false)
			}
		}
		if step.State == "DONE" || step.State == "ERROR" {
			if onPartial != nil {
				onPartial("", true)
			}
			s.flushText(trace, idx)
		}
		return false
	case "result":
		if onPartial != nil {
			onPartial("", true)
		}
		s.flushAll(trace)
		res := frame.Result
		if res == nil {
			return true
		}
		if uuid.Validate(res.ConversationID) == nil {
			s.sessionID = res.ConversationID
		}
		if res.Usage != nil && s.contextUsed == 0 {
			if used := res.Usage.contextSize(); used > 0 {
				s.contextUsed = used
			}
		}
		if cmds := parseAgyCommandResultSlash(res.Command); len(cmds) > 0 && len(s.discoveredCommands) == 0 {
			s.discoveredCommands = cmds
		}
		s.resultStatus = res.Status
		s.resultError = strings.TrimSpace(res.Error)
		s.deniedActions = res.DeniedActions
		isCommand := s.localSlash || (len(res.Command) > 0 && string(res.Command) != "null")
		if isCommand {
			if text := strings.TrimSpace(res.Response); text != "" && !s.wroteCommandOutput {
				conversationWrite(trace, "command_output", text, "")
				s.wroteCommandOutput = true
			}
		} else if !s.wroteAssistant && res.Status == "SUCCESS" {
			if text := strings.TrimSpace(res.Response); text != "" {
				conversationWrite(trace, "assistant", text, "")
				s.wroteAssistant = true
			}
		}
		if len(res.DeniedActions) > 0 {
			conversationWrite(trace, "notice", "Antigravity skipped actions requiring approval", formatAgyDeniedActions(res.DeniedActions))
		}
		return true
	}
	return false
}

func agyToolID(stepIndex int) string {
	return fmt.Sprintf("agy-step-%d", stepIndex)
}

// agyToolEvent converts an Antigravity tool step into the conversationEvent
// shape Desktop's tool-cards.mjs renders as rich diffs, command blocks, or
// file lines. Unrecognized tools keep their native name and JSON parameters.
func agyToolEvent(stepIndex int, toolName string, info *agyToolInfo) conversationEvent {
	name := strings.TrimSpace(toolName)
	var rawParams json.RawMessage
	if info != nil {
		if name == "" {
			name = strings.TrimSpace(info.Name)
		}
		rawParams = info.Parameters
	}
	if name == "" {
		name = "Tool"
	}
	event := conversationEvent{
		Kind:   "tool",
		ToolID: agyToolID(stepIndex),
		Tool:   name,
		Text:   name,
	}
	var params map[string]any
	if len(rawParams) > 0 {
		_ = json.Unmarshal(rawParams, &params)
	}
	switch name {
	case "view_file":
		path := firstString(params, "AbsolutePath", "file_path", "path")
		mapped := map[string]any{"file_path": path}
		if start, ok := intParam(params, "StartLine"); ok && start > 0 {
			mapped["offset"] = start
			if end, ok := intParam(params, "EndLine"); ok && end >= start {
				mapped["limit"] = end - start + 1
			}
		}
		event.Tool, event.Text, event.Detail = "Read", "Read", path
		event.Input, _ = json.Marshal(mapped)
	case "replace_file_content":
		path := firstString(params, "TargetFile", "file_path", "path")
		mapped := map[string]any{
			"file_path":   path,
			"old_string":  firstString(params, "TargetContent"),
			"new_string":  firstString(params, "ReplacementContent"),
			"replace_all": boolParam(params, "AllowMultiple"),
		}
		event.Tool, event.Text, event.Detail = "Edit", "Edit", path
		event.Input, _ = json.Marshal(mapped)
	case "multi_replace_file_content":
		path := firstString(params, "TargetFile", "file_path", "path")
		var edits []map[string]any
		if chunks, ok := params["ReplacementChunks"].([]any); ok {
			for _, item := range chunks {
				if chunk, ok := item.(map[string]any); ok {
					edits = append(edits, map[string]any{
						"old_string": firstString(chunk, "TargetContent"),
						"new_string": firstString(chunk, "ReplacementContent"),
					})
				}
			}
		}
		event.Tool, event.Text, event.Detail = "MultiEdit", "MultiEdit", path
		event.Input, _ = json.Marshal(map[string]any{"file_path": path, "edits": edits})
	case "write_to_file":
		path := firstString(params, "TargetFile", "file_path", "path")
		mapped := map[string]any{
			"file_path": path,
			"content":   firstString(params, "CodeContent", "content"),
		}
		event.Tool, event.Text, event.Detail = "Write", "Write", path
		event.Input, _ = json.Marshal(mapped)
	case "run_command":
		command := firstString(params, "CommandLine", "command")
		desc := firstString(params, "toolSummary", "toolAction")
		mapped := map[string]any{
			"command":           command,
			"description":       desc,
			"run_in_background": boolParam(params, "IsDaemon"),
		}
		event.Tool, event.Text, event.Detail = "Bash", "Bash", clampShellDetail(command)
		event.Input, _ = json.Marshal(mapped)
	case "grep_search":
		query := firstString(params, "Query", "pattern")
		searchPath := firstString(params, "SearchPath", "path")
		event.Tool, event.Text, event.Detail = "Grep", "Grep", query
		event.Input, _ = json.Marshal(map[string]any{"pattern": query, "path": searchPath})
	case "find_by_name":
		pattern := firstString(params, "Pattern", "pattern")
		searchDir := firstString(params, "SearchDirectory", "path")
		event.Tool, event.Text, event.Detail = "Glob", "Glob", pattern
		event.Input, _ = json.Marshal(map[string]any{"pattern": pattern, "path": searchDir})
	case "read_url_content":
		url := firstString(params, "Url", "url")
		event.Tool, event.Text, event.Detail = "WebFetch", "WebFetch", url
		event.Input, _ = json.Marshal(map[string]any{"url": url})
	case "search_web":
		query := firstString(params, "query", "Query")
		event.Tool, event.Text, event.Detail = "WebSearch", "WebSearch", query
		event.Input, _ = json.Marshal(map[string]any{"query": query})
	case "call_mcp_tool":
		server := firstString(params, "ServerName")
		tool := firstString(params, "ToolName")
		if server != "" && tool != "" {
			event.Tool = "mcp__" + server + "__" + tool
			event.Text = tool
		}
		if args, ok := params["Arguments"]; ok {
			event.Input, _ = json.Marshal(args)
		} else {
			event.Input = rawParams
		}
	default:
		event.Input = rawParams
	}
	if len(event.Input) > runner.ToolInputLimit {
		event.Input = nil
	}
	return event
}

func agyToolResultEvent(stepIndex int, state string, info *agyToolInfo) conversationEvent {
	var text string
	failed := state == "ERROR"
	if info != nil {
		text = info.Output
		if info.Error != nil {
			failed = true
			errText := strings.TrimSpace(info.Error.Message)
			if errText == "" {
				errText = strings.TrimSpace(info.Error.Type)
			}
			if errText != "" {
				if text != "" {
					text = text + "\n" + errText
				} else {
					text = errText
				}
			}
		}
	}
	text, truncated := codexToolText(text)
	return conversationEvent{
		Kind:      "tool_result",
		ToolID:    agyToolID(stepIndex),
		Text:      text,
		Error:     failed,
		Truncated: truncated,
	}
}

func intParam(params map[string]any, key string) (int, bool) {
	if params == nil {
		return 0, false
	}
	switch v := params[key].(type) {
	case float64:
		return int(v), true
	case int:
		return v, true
	}
	return 0, false
}

func boolParam(params map[string]any, key string) bool {
	if params == nil {
		return false
	}
	b, _ := params[key].(bool)
	return b
}

func firstString(entry map[string]any, fields ...string) string {
	for _, field := range fields {
		if value, ok := entry[field].(string); ok && value != "" {
			return value
		}
	}
	return ""
}

func formatAgyDeniedActions(actions []agyDeniedAction) string {
	seen := map[string]bool{}
	var parts []string
	for _, action := range actions {
		label := strings.TrimSpace(action.Tool)
		if label == "" {
			label = "tool"
		}
		var meta []string
		if s := strings.TrimSpace(action.Scope); s != "" {
			meta = append(meta, s)
		}
		if r := strings.TrimSpace(action.Rule); r != "" {
			meta = append(meta, r)
		}
		if len(meta) > 0 {
			label += " (" + strings.Join(meta, ", ") + ")"
		}
		if !seen[label] {
			seen[label] = true
			parts = append(parts, label)
		}
	}
	return "Denied in headless mode: " + strings.Join(parts, ", ") + ". Choose Auto-approve all tools or Accept edits in the permission mode selector and resend if you want Antigravity to run them."
}

func (d *agentDaemon) agyConversationTurn(run *controlledRun, prompt string) {
	d.queue.mu.Lock()
	projectID, directory := run.desktop.ProjectID, run.desktop.Directory
	d.queue.mu.Unlock()
	folders, env, err := d.conversationFolders(projectID, directory)
	if err != nil {
		conversationWrite(run.trace, "notice", "Attached folders could not be read for this message", err.Error())
	}
	dirs := folderMapDirs(folders)

	d.queue.mu.Lock()
	c := run.conversation
	session, effort, mode, model := c.session, c.effort, c.mode, run.desktop.Model
	dirs = append(dirs, c.extraDirs...)
	if err == nil {
		refreshConversationFolders(run, folders)
	}
	for key, value := range c.env {
		if _, fresh := env[key]; !fresh {
			env[key] = value
		}
	}
	localSlash := isAgyLocalSlashCommand(prompt)
	slashPrompt := ""
	if localSlash {
		slashPrompt = strings.TrimSpace(prompt)
	}
	cmd := agyConversationCommand(directory, model, effort, mode, session, slashPrompt, dirs, env)
	var input *conversationInput
	if !localSlash {
		stdin, err := cmd.StdinPipe()
		if err != nil {
			c.busy = false
			d.queue.mu.Unlock()
			conversationWrite(run.trace, "error", "Failed to open Antigravity input", err.Error())
			return
		}
		input = &conversationInput{w: stdin}
		c.input = input
	}
	c.interrupted = false
	c.sentAt = time.Now()
	d.queue.mu.Unlock()

	if input != nil {
		defer input.close()
	}

	state := newAgyStreamState(localSlash)
	cmd.Stdout = &conversationOutput{line: func(line string) {
		done := state.handleLine(line, run.trace, func(delta string, clear bool) {
			d.queue.mu.Lock()
			defer d.queue.mu.Unlock()
			if clear {
				c.partial = ""
				return
			}
			if len(c.partial)+len(delta) <= conversationPartialLimit {
				c.partial += delta
			}
		})
		if state.sessionID != "" {
			d.queue.mu.Lock()
			c.session = state.sessionID
			run.desktop.SessionID = state.sessionID
			d.queue.mu.Unlock()
		}
		if len(state.discoveredCommands) > 0 {
			d.queue.mu.Lock()
			c.commands = mergeAgySlashCommands(c.commands, state.discoveredCommands)
			d.queue.mu.Unlock()
		}
		if done && input != nil {
			input.close()
		}
	}}
	var diagnostics limitedConversationBuffer
	cmd.Stderr = &diagnostics
	release, err := agentexec.StartDetached(cmd)
	if err != nil {
		d.queue.mu.Lock()
		c.input = nil
		c.busy = false
		d.queue.mu.Unlock()
		conversationWrite(run.trace, "error", "Failed to start Antigravity", err.Error())
		return
	}
	defer release()

	if input != nil {
		if err := input.send(agyUserMessage(prompt)); err != nil {
			input.close()
			agentexec.StopControlled(cmd, true)
			_ = cmd.Wait()
			d.queue.mu.Lock()
			c.input = nil
			c.busy = false
			d.queue.mu.Unlock()
			conversationWrite(run.trace, "error", "Failed to send message to Antigravity", err.Error())
			return
		}
		// Close stdin once the prompt frame is written so agy exits cleanly at
		// the end of the turn without waiting for another line.
		input.close()
	}

	done := make(chan struct{})
	go func() {
		var interruptedAt time.Time
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				d.queue.mu.Lock()
				canceled := run.canceled || d.queue.shuttingDown
				interrupted := c.interrupted
				d.queue.mu.Unlock()
				if canceled {
					if input != nil {
						input.close()
					}
					agentexec.StopControlled(cmd, true)
					return
				}
				if interrupted {
					if interruptedAt.IsZero() {
						interruptedAt = time.Now()
						if input != nil {
							input.close()
						}
						agentexec.StopControlled(cmd, false)
					} else if time.Since(interruptedAt) >= conversationInterruptGrace {
						agentexec.StopControlled(cmd, true)
						return
					}
				}
			}
		}
	}()

	waitErr := cmd.Wait()
	close(done)

	d.queue.mu.Lock()
	c.input = nil
	c.partial = ""
	c.approvals = nil
	markApprovalWaitLocked(run)
	if state.sessionID != "" {
		c.session = state.sessionID
		run.desktop.SessionID = state.sessionID
	}
	if state.contextUsed > 0 {
		c.contextUsed = state.contextUsed
	}
	interrupted := c.interrupted
	c.interrupted = false
	var next string
	if len(c.next) > 0 && !interrupted && !run.canceled && !d.queue.shuttingDown {
		next, c.next = c.next[0], c.next[1:]
	} else {
		c.next = nil
		c.busy = false
	}
	canceled := run.canceled || d.queue.shuttingDown
	if canceled {
		if run.canceled && !d.queue.shuttingDown {
			run.desktop.Status = conversationStoppedStatus(run)
		} else {
			run.desktop.Status = "canceled"
		}
		run.once.Do(func() { close(run.exited) })
		run.trace.close()
	}
	d.queue.mu.Unlock()

	if next != "" {
		d.agyConversationTurn(run, next)
		return
	}
	if interrupted && !canceled {
		conversationWrite(run.trace, "notice", "Turn stopped", "")
		return
	}
	if (waitErr != nil || state.resultStatus == "ERROR") && !canceled {
		detail := strings.TrimSpace(diagnostics.String())
		if detail == "" {
			detail = state.resultError
		}
		if detail == "" && waitErr != nil {
			detail = waitErr.Error()
		}
		conversationWrite(run.trace, "notice", "Antigravity turn ended with an error", detail)
	}
}

func parseAgyCommandResultSlash(raw json.RawMessage) []conversationSlash {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var payload struct {
		Name string `json:"name"`
		Data struct {
			Skills []struct {
				Name        string `json:"name"`
				Description string `json:"description"`
			} `json:"skills"`
			Commands []struct {
				Name        string `json:"name"`
				Description string `json:"description"`
			} `json:"commands"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &payload) != nil {
		return nil
	}
	var out []conversationSlash
	for _, skill := range payload.Data.Skills {
		if name := strings.TrimSpace(skill.Name); name != "" {
			out = append(out, conversationSlash{Name: name, Description: strings.TrimSpace(skill.Description)})
		}
	}
	for _, cmd := range payload.Data.Commands {
		if name := strings.TrimSpace(cmd.Name); name != "" {
			out = append(out, conversationSlash{Name: name, Description: strings.TrimSpace(cmd.Description)})
		}
	}
	return out
}

func mergeAgySlashCommands(base, extra []conversationSlash) []conversationSlash {
	seen := map[string]bool{}
	var out []conversationSlash
	for _, list := range [][]conversationSlash{base, agyBuiltinSlashCommands, extra} {
		for _, item := range list {
			name := strings.TrimPrefix(strings.TrimSpace(item.Name), "/")
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			item.Name = name
			out = append(out, item)
		}
	}
	return out
}

func (d *agentDaemon) probeAgyConversationCommands(cmd *exec.Cmd) []conversationSlash {
	var discovered []conversationSlash
	cmd.Stdout = &conversationOutput{line: func(line string) {
		var frame agyFrame
		if json.Unmarshal([]byte(line), &frame) != nil {
			return
		}
		if cmds := parseAgyCommandResultSlash(frame.Command); len(cmds) > 0 {
			discovered = append(discovered, cmds...)
		}
		if frame.Result != nil {
			if cmds := parseAgyCommandResultSlash(frame.Result.Command); len(cmds) > 0 && len(discovered) == 0 {
				discovered = append(discovered, cmds...)
			}
		}
	}}
	var diagnostics limitedConversationBuffer
	cmd.Stderr = &diagnostics
	release, err := agentexec.StartDetached(cmd)
	if err != nil {
		return mergeAgySlashCommands(nil, nil)
	}
	defer release()
	waited := make(chan error, 1)
	go func() { waited <- cmd.Wait() }()
	select {
	case <-waited:
	case <-time.After(conversationProbeTimeout):
		agentexec.StopControlled(cmd, true)
		<-waited
	}
	return mergeAgySlashCommands(nil, discovered)
}

// parseAgyMCPList reads `agy mcp list` tabular output:
// NAME     TYPE  STATUS   COMMAND/URL
// sectile  http  enabled  https://.../mcp
func parseAgyMCPList(output string) conversationMCP {
	if strings.Contains(output, "No MCP servers") {
		return conversationMCP{Status: "missing", Detail: "Sectile's MCP server is not registered in Antigravity."}
	}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[0] != sectileMCPName {
			continue
		}
		statusField := strings.ToLower(fields[2])
		detail := strings.TrimSpace(line)
		switch statusField {
		case "enabled", "connected":
			return conversationMCP{Status: "connected", Detail: detail}
		case "disabled":
			return conversationMCP{Status: "failed", Detail: detail}
		default:
			return conversationMCP{Status: mcpStatus(statusField), Detail: detail}
		}
	}
	if strings.Contains(output, "NAME") && strings.Contains(output, "STATUS") {
		return conversationMCP{Status: "missing", Detail: "Sectile's MCP server is not registered in Antigravity."}
	}
	return conversationMCP{Status: "unknown"}
}

func (d *agentDaemon) checkAgySectileMCP(directory string, env map[string]string) conversationMCP {
	ctx, cancel := context.WithTimeout(context.Background(), sectileMCPCheckTimeout)
	defer cancel()
	var output limitedConversationBuffer
	cmd := agentexec.Hidden(exec.CommandContext(ctx, "agy", "mcp", "list"))
	cmd.Dir, cmd.Env = directory, commandEnv(env)
	cmd.Stdout, cmd.Stderr = &output, &output
	err := cmd.Run()
	result := parseAgyMCPList(output.String())
	if result.Status == "unknown" && err != nil {
		result.Detail = "agy mcp list failed: " + err.Error()
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			result.Detail = "agy mcp list timed out"
		}
	}
	return result
}
