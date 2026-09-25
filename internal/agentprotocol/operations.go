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
	Model     string `json:"model,omitempty"`
	RunID     string `json:"runId,omitempty"`
	ProjectID string `json:"projectId"`
	TaskID    string `json:"taskId,omitempty"`
	Action    string `json:"action"`
	Branch    string `json:"branch,omitempty"`
	// Repository names a repository other than the project checkout's, as its
	// host/path identity, for evidence of a pull request that lives there.
	// Empty keeps the operation on the project checkout.
	Repository   string `json:"repository,omitempty"`
	Create       bool   `json:"create,omitempty"`
	DeleteRemote bool   `json:"deleteRemote,omitempty"`
	Editor       string `json:"editor,omitempty"`
	Prompt       string `json:"prompt,omitempty"`
	// Mode is the execution mode the server resolved for this launch:
	// "interactive" or "autonomous". Empty is read as interactive by the agent,
	// which keeps an older server working.
	Mode string `json:"mode,omitempty"`
	// MacroKey and MacroTitle name the macro of a macro_worktree operation,
	// which prepares that macro's specification checkout.
	MacroKey   string `json:"macroKey,omitempty"`
	MacroTitle string `json:"macroTitle,omitempty"`
	// SpecFile names the file a macro_spec_file operation reads in the
	// macro's specification folder: "tasks.md" or "spec.md".
	SpecFile string `json:"specFile,omitempty"`
}

// MacroSpecFile answers a macro_spec_file operation: the file's content and
// where it was read, a workstation path or "<branch>:<path>".
type MacroSpecFile struct {
	Content string `json:"content"`
	Origin  string `json:"origin"`
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
