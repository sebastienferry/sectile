package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"tasks/internal/agentexec"
	"tasks/internal/runner"

	"github.com/google/uuid"
)

// A message starting with "!" is a shell command, as in Claude Code's bash
// mode: the agent runs it in the conversation's directory, without Claude,
// shows it as a Bash card, and hands the command and what it printed to
// Claude with the next message. Claude's print mode has no bash mode of its
// own: it reads "!" as text.

// conversationShellTimeout bounds a command typed in the conversation.
const conversationShellTimeout = 2 * time.Minute

// conversationShellContextLimit bounds what the commands run since the last
// message add to the next one; past it the oldest go.
const conversationShellContextLimit = 32 * 1024

// conversationShellCommand is the owner's own shell running line in directory.
func conversationShellCommand(ctx context.Context, directory, line string, env map[string]string) *exec.Cmd {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd.exe", "/c", line)
	} else {
		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/sh"
		}
		cmd = exec.CommandContext(ctx, shell, "-c", line)
	}
	cmd = agentexec.Hidden(cmd)
	cmd.Dir = directory
	cmd.Env = commandEnv(env)
	cmd.WaitDelay = headlessStopGrace
	return cmd
}

// startConversationShellLocked records the command as a Bash call and runs it
// in the background. The queue lock is held.
func (d *agentDaemon) startConversationShellLocked(run *controlledRun, line string) {
	toolID := "shell-" + uuid.NewString()
	input, _ := json.Marshal(map[string]string{"command": line, "description": "Run by you"})
	conversationWriteEvent(run.trace, conversationEvent{Kind: "tool", Text: "Bash", Tool: "Bash", ToolID: toolID, Input: input, Detail: clampShellDetail(line)})
	env := map[string]string{"SECTILE_PROJECT_ID": run.desktop.ProjectID}
	for key, value := range run.conversation.env {
		env[key] = value
	}
	directory := run.desktop.Directory
	go d.runConversationShell(run, toolID, directory, line, env)
}

func (d *agentDaemon) runConversationShell(run *controlledRun, toolID, directory, line string, env map[string]string) {
	ctx, cancel := context.WithTimeout(context.Background(), conversationShellTimeout)
	defer cancel()
	var output limitedConversationBuffer
	cmd := conversationShellCommand(ctx, directory, line, env)
	cmd.Stdout, cmd.Stderr = &output, &output
	err := cmd.Run()
	text := strings.TrimRight(output.String(), "\n")
	truncated := len(text) > runner.ToolResultLimit
	if truncated {
		text = text[:runner.ToolResultLimit]
	}
	failed := err != nil
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		text = strings.TrimSpace(text + "\nStopped after " + conversationShellTimeout.String() + ".")
	} else if exit := (*exec.ExitError)(nil); errors.As(err, &exit) {
		text = strings.TrimSpace(text + fmt.Sprintf("\nExit code %d.", exit.ExitCode()))
	} else if err != nil {
		text = strings.TrimSpace(text + "\n" + err.Error())
	}
	d.queue.mu.Lock()
	defer d.queue.mu.Unlock()
	conversationWriteEvent(run.trace, conversationEvent{Kind: "tool_result", Text: text, ToolID: toolID, Truncated: truncated, Error: failed})
	block := "<bash-input>" + line + "</bash-input>\n<bash-stdout>" + text + "</bash-stdout>"
	context := strings.TrimSpace(run.conversation.shellContext + "\n" + block)
	if len(context) > conversationShellContextLimit {
		context = context[len(context)-conversationShellContextLimit:]
	}
	run.conversation.shellContext = context
}

// withShellContext hands Claude the commands run since the last message, as
// Claude Code does, and forgets them. The queue lock is held.
func withShellContext(c *claudeConversation, message string) string {
	if c.shellContext == "" {
		return message
	}
	message = c.shellContext + "\n\n" + message
	c.shellContext = ""
	return message
}

func clampShellDetail(line string) string {
	var buffer bytes.Buffer
	for _, r := range strings.Join(strings.Fields(line), " ") {
		if buffer.Len() >= 160 {
			buffer.WriteString("…")
			break
		}
		buffer.WriteRune(r)
	}
	return buffer.String()
}

// conversationMCPTimeout bounds the health check behind /mcp.
const conversationMCPTimeout = time.Minute

// startConversationMCPLocked answers /mcp. Print mode only says how many
// servers are connected and sends the owner to a terminal for the rest, so the
// agent runs `claude mcp list` there instead, which checks each server, and
// shows what it printed. Claude is not told: it is for the owner. The queue
// lock is held.
func (d *agentDaemon) startConversationMCPLocked(run *controlledRun) {
	conversationWrite(run.trace, "notice", "Checking MCP server health…", "")
	env := map[string]string{"SECTILE_PROJECT_ID": run.desktop.ProjectID}
	for key, value := range run.conversation.env {
		env[key] = value
	}
	directory := run.desktop.Directory
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), conversationMCPTimeout)
		defer cancel()
		var output limitedConversationBuffer
		cmd := conversationShellCommand(ctx, directory, "claude mcp list", env)
		cmd.Stdout, cmd.Stderr = &output, &output
		err := cmd.Run()
		text := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(output.String()), "Checking MCP server health…"))
		kind := "command_output"
		if err != nil && text == "" {
			kind, text = "error", "claude mcp list failed: "+err.Error()
		}
		d.queue.mu.Lock()
		defer d.queue.mu.Unlock()
		conversationWrite(run.trace, kind, text, "")
	}()
}
