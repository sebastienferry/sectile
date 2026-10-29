package agent

import (
	"encoding/json"
	"fmt"
	"sort"
)

type mcpFormField struct {
	Type  string   `json:"type"`
	Title string   `json:"title"`
	Enum  []string `json:"enum"`
}

// MCP form consent is a user decision, never a blanket tool authorization.
// Unsupported nested schemas and URL flows remain refused rather than guessed.
func codexMCPApproval(frame codexFrame) (conversationApproval, bool) {
	var p struct {
		Server  string `json:"serverName"`
		Mode    string `json:"mode"`
		Message string `json:"message"`
		Meta    struct {
			ToolParams json.RawMessage `json:"tool_params"`
		} `json:"_meta"`
		Schema struct {
			Type       string                  `json:"type"`
			Properties map[string]mcpFormField `json:"properties"`
		} `json:"requestedSchema"`
	}
	if json.Unmarshal(frame.Params, &p) != nil || p.Server == "" || p.Message == "" || p.Schema.Type != "object" || (p.Mode != "form" && p.Mode != "openai/form" && p.Mode != "openaiForm") || len(p.Schema.Properties) > 16 {
		return conversationApproval{}, false
	}
	a := conversationApproval{ID: "codex-" + string(frame.ID), RequestID: frame.ID, Method: frame.Method, Tool: "MCP · " + p.Server, Description: p.Message, Input: p.Meta.ToolParams, MCPFields: p.Schema.Properties}
	if len(p.Schema.Properties) == 0 {
		a.Choices = []conversationDecisionOption{{"allow", "Allow"}, {"deny", "Deny"}}
		return a, true
	}
	a.Tool = askUserQuestion
	a.QuestionIDs = map[string]string{}
	keys := make([]string, 0, len(p.Schema.Properties))
	for key := range p.Schema.Properties {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	questions := []map[string]any{}
	for _, key := range keys {
		field := p.Schema.Properties[key]
		if field.Type != "boolean" && (field.Type != "string" || len(field.Enum) == 0) {
			return conversationApproval{}, false
		}
		question := p.Message + "\n" + key
		options := []map[string]string{}
		values := field.Enum
		if field.Type == "boolean" {
			values = []string{"true", "false"}
		}
		for _, v := range values {
			options = append(options, map[string]string{"label": v})
		}
		a.QuestionIDs[question] = key
		questions = append(questions, map[string]any{"header": "MCP · " + p.Server, "question": question, "options": options})
	}
	a.Input, _ = json.Marshal(map[string]any{"questions": questions})
	return a, true
}

func codexMCPAnswer(a conversationApproval, decision string, answers map[string]string) (map[string]any, error) {
	if decision == "deny" {
		return map[string]any{"action": "decline"}, nil
	}
	if a.Tool != askUserQuestion {
		if decision != "allow" {
			return nil, fmt.Errorf("MCP consent requires an explicit decision")
		}
		return map[string]any{"action": "accept", "content": map[string]any{}}, nil
	}
	if decision != "answer" {
		return nil, fmt.Errorf("MCP form requires answers")
	}
	content := map[string]any{}
	for question, key := range a.QuestionIDs {
		field := a.MCPFields[key]
		value := answers[question]
		if field.Type == "boolean" {
			if value != "true" && value != "false" {
				return nil, fmt.Errorf("MCP boolean answer must be true or false")
			}
			content[key] = value == "true"
		} else {
			if len(field.Enum) > 0 {
				valid := false
				for _, allowed := range field.Enum {
					valid = valid || allowed == value
				}
				if !valid {
					return nil, fmt.Errorf("MCP answer is not an offered choice")
				}
			}
			content[key] = value
		}
	}
	return map[string]any{"action": "accept", "content": content}, nil
}
