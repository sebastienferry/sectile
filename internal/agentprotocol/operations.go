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
	// Model is the one-off model override carried by a queued launch, empty when
	// the launcher resolved none.
	Model        string `json:"model,omitempty"`
	RunID        string `json:"runId,omitempty"`
	ProjectID    string `json:"projectId"`
	TaskID       string `json:"taskId,omitempty"`
	Action       string `json:"action"`
	Branch       string `json:"branch,omitempty"`
	Create       bool   `json:"create,omitempty"`
	DeleteRemote bool   `json:"deleteRemote,omitempty"`
	Editor       string `json:"editor,omitempty"`
	Prompt       string `json:"prompt,omitempty"`
	// Mode is the execution mode the server resolved for this launch:
	// "interactive" or "autonomous". Empty is read as interactive by the agent,
	// which keeps an older server working.
	Mode string `json:"mode,omitempty"`
	// Marketplace coordinates of a skill pack operation. The server stores and
	// forwards them; it never dereferences them itself — cloning, fetching and
	// reading a marketplace all happen on the workstation.
	Marketplace string `json:"marketplace,omitempty"`
	Plugin      string `json:"plugin,omitempty"`
	Kind        string `json:"kind,omitempty"`    // github | git | path
	Locator     string `json:"locator,omitempty"` // owner/repo, git URL, or absolute path
	// Commit pins the revision to resolve. Empty resolves the marketplace head.
	Commit string `json:"commit,omitempty"`
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
