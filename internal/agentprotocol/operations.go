package agentprotocol

import "encoding/json"

// Operation names a local capability; directories are resolved by the agent from project identity.
type Operation struct {
	// UserID names the agent owner to reach. Empty means the deployment's
	// single implicit user, which is what a server without an identity
	// provider has.
	UserID            string `json:"userId,omitempty"`
	AICommandTemplate string `json:"aiCommandTemplate,omitempty"`
	Framework         string `json:"framework,omitempty"`
	Provider          string `json:"provider,omitempty"`
	Force             bool   `json:"force,omitempty"`
	SkillID           string `json:"skillId,omitempty"`
	RunID             string `json:"runId,omitempty"`
	ProjectID         string `json:"projectId"`
	TaskID            string `json:"taskId,omitempty"`
	Action            string `json:"action"`
	Branch            string `json:"branch,omitempty"`
	Create            bool   `json:"create,omitempty"`
	DeleteRemote      bool   `json:"deleteRemote,omitempty"`
	Editor            string `json:"editor,omitempty"`
	Prompt            string `json:"prompt,omitempty"`
}
type Result struct {
	Value json.RawMessage `json:"value,omitempty"`
	Error string          `json:"error,omitempty"`
}

// SkillFile is workstation evidence for the server's skill editor.
type SkillFile struct {
	Content string   `json:"content"`
	Paths   []string `json:"paths"`
}
