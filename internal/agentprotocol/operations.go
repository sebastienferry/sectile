package agentprotocol

import "encoding/json"

// Operation names a local capability; it never supplies an executable or an arbitrary directory.
type Operation struct {
	Framework    string `json:"framework,omitempty"`
	Provider     string `json:"provider,omitempty"`
	Force        bool   `json:"force,omitempty"`
	SkillID      string `json:"skillId,omitempty"`
	RunID        string `json:"runId,omitempty"`
	ProjectID    string `json:"projectId"`
	TaskID       string `json:"taskId,omitempty"`
	Action       string `json:"action"`
	Branch       string `json:"branch,omitempty"`
	Create       bool   `json:"create,omitempty"`
	DeleteRemote bool   `json:"deleteRemote,omitempty"`
	Editor       string `json:"editor,omitempty"`
	Prompt       string `json:"prompt,omitempty"`
}
type Result struct {
	Value json.RawMessage `json:"value,omitempty"`
	Error string          `json:"error,omitempty"`
}
