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
func withShellContext(c *providerConversation, message string) string {
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
	if run.desktop.Provider == "codex" {
		d.checkCodexMCPLocked(run, true)
		return
	}
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

// sectileMCPName is the name Sectile has its MCP server registered under
// (shared/mcpConfig.mjs), the one --allowedTools=mcp__sectile names.
const sectileMCPName = "sectile"

// conversationMCP is whether Claude reaches Sectile's MCP server from the
// conversation: connected, needs-auth, failed, pending, missing (not
// registered) or unknown, with what the check said.
type conversationMCP struct {
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

// mcpStatus maps what Claude Code says of a server to a conversationMCP
// status.
func mcpStatus(text string) string {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "connected") && !strings.Contains(lower, "not connected"):
		return "connected"
	case strings.Contains(lower, "auth"):
		return "needs-auth"
	case strings.Contains(lower, "fail"), strings.Contains(lower, "not connected"), strings.Contains(lower, "error"):
		return "failed"
	case strings.Contains(lower, "pending"), strings.Contains(lower, "connecting"):
		return "pending"
	}
	return "unknown"
}

// parseMCPGet reads `claude mcp get sectile`.
func parseMCPGet(output string) conversationMCP {
	if strings.Contains(output, "No MCP server named") {
		return conversationMCP{Status: "missing", Detail: "Sectile's MCP server is not registered in Claude Code."}
	}
	var details []string
	status := "unknown"
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if value, ok := strings.CutPrefix(line, "Status:"); ok {
			status = mcpStatus(value)
			details = append(details, strings.TrimSpace(value))
		} else if value, ok := strings.CutPrefix(line, "URL:"); ok {
			details = append(details, strings.TrimSpace(value))
		}
	}
	return conversationMCP{Status: status, Detail: strings.Join(details, " · ")}
}

// sectileMCPCheckTimeout bounds `claude mcp get sectile`.
const sectileMCPCheckTimeout = 30 * time.Second

func (d *agentDaemon) checkSectileMCP(directory string, env map[string]string) conversationMCP {
	ctx, cancel := context.WithTimeout(context.Background(), sectileMCPCheckTimeout)
	defer cancel()
	var output limitedConversationBuffer
	cmd := agentexec.Hidden(exec.CommandContext(ctx, "claude", "mcp", "get", sectileMCPName))
	cmd.Dir, cmd.Env = directory, commandEnv(env)
	cmd.Stdout, cmd.Stderr = &output, &output
	err := cmd.Run()
	result := parseMCPGet(output.String())
	if result.Status == "unknown" && err != nil {
		result.Detail = "claude mcp get " + sectileMCPName + " failed: " + err.Error()
	}
	return result
}

// checkSectileMCPLocked reads, in the background, whether Claude reaches
// Sectile's MCP server here. Each turn's init frame refreshes it. The queue
// lock is held.
func (d *agentDaemon) checkSectileMCPLocked(run *controlledRun) {
	if run.desktop.Provider == "codex" {
		d.checkCodexMCPLocked(run, false)
		return
	}
	c := run.conversation
	if c.checkingMCP || run.desktop.Directory == "" {
		return
	}
	c.checkingMCP = true
	env := map[string]string{"SECTILE_PROJECT_ID": run.desktop.ProjectID}
	for key, value := range c.env {
		env[key] = value
	}
	directory := run.desktop.Directory
	check := d.checkSectileMCP
	if d.checkMCPFn != nil {
		check = d.checkMCPFn
	}
	go func() {
		result := check(directory, env)
		d.queue.mu.Lock()
		defer d.queue.mu.Unlock()
		c.checkingMCP = false
		c.sectileMCP = &result
	}()
}

// initMCPServer reads Sectile's server out of an init frame's mcp_servers.
func initMCPServer(servers []struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}) conversationMCP {
	for _, server := range servers {
		if server.Name == sectileMCPName {
			return conversationMCP{Status: mcpStatus(server.Status), Detail: server.Status}
		}
	}
	return conversationMCP{Status: "missing", Detail: "Sectile's MCP server is not registered in Claude Code."}
}
