package runner

import (
	"encoding/json"
	"strings"
)

// parseCodexReasoningLine normalizes Codex exec --json without retaining raw
// protocol frames or full tool results in the server activity.
func parseCodexReasoningLine(line string) ([]ReasoningEvent, string, bool) {
	var frame struct {
		Type    string `json:"type"`
		Message string `json:"message"`
		Error   struct {
			Message string `json:"message"`
		} `json:"error"`
		Item struct {
			ID        string          `json:"id"`
			Type      string          `json:"type"`
			Text      string          `json:"text"`
			Command   string          `json:"command"`
			Server    string          `json:"server"`
			Tool      string          `json:"tool"`
			Arguments json.RawMessage `json:"arguments"`
			Status    string          `json:"status"`
			Changes   []struct {
				Path string `json:"path"`
			} `json:"changes"`
		} `json:"item"`
	}
	if json.Unmarshal([]byte(line), &frame) != nil {
		return nil, "", false
	}
	if frame.Type == "turn.failed" || frame.Type == "error" {
		message := frame.Error.Message
		if message == "" {
			message = frame.Message
		}
		return []ReasoningEvent{{Kind: ReasoningText, Text: message}}, message, true
	}
	item := frame.Item
	if frame.Type == "item.completed" && item.Type == "agent_message" && item.Text != "" {
		return []ReasoningEvent{{Kind: ReasoningText, Text: item.Text}}, item.Text, true
	}
	if frame.Type == "item.completed" && item.Type == "reasoning" && item.Text != "" {
		return []ReasoningEvent{{Kind: ReasoningThinking, Text: item.Text}}, "", false
	}
	if frame.Type != "item.started" && frame.Type != "item.completed" {
		return nil, "", false
	}
	event := ReasoningEvent{Kind: ReasoningTool, ToolID: item.ID}
	switch item.Type {
	case "command_execution":
		event.Tool, event.Detail = "Bash", item.Command
	case "mcp_tool_call":
		if item.Server == "" || item.Tool == "" {
			return nil, "", false
		}
		event.Tool = "mcp__" + item.Server + "__" + item.Tool
		if len(item.Arguments) <= ToolInputLimit && json.Valid(item.Arguments) {
			event.Input = item.Arguments
		}
		event.Detail = toolDetail(event.Tool, item.Arguments)
	case "file_change":
		event.Tool = "Edit"
		paths := []string{}
		for _, change := range item.Changes {
			paths = append(paths, change.Path)
		}
		event.Detail = strings.Join(paths, ", ")
	default:
		return nil, "", false
	}
	event.Text = shortToolName(event.Tool)
	if frame.Type == "item.completed" {
		if item.Status == "failed" {
			event.Text += " · erreur"
		} else {
			event.Text += " · terminé"
		}
	}
	return []ReasoningEvent{event}, "", false
}
