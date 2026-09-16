// Package agentconfig defines the versioned, secret-free execution contract.
package agentconfig

const Version = 1

type Skill struct {
	RequiresReconciliation bool   `json:"requiresReconciliation,omitempty"`
	ID                     string `json:"id"`
	Directory              string `json:"directory"`
	Command                string `json:"command"`
	Content                string `json:"content"`
	CommandContent         string `json:"commandContent"`
}

type Config struct {
	TrackerURL              string   `json:"trackerUrl,omitempty"`
	LinearTeam              string   `json:"linearTeam,omitempty"`
	JiraProject             string   `json:"jiraProject,omitempty"`
	GithubRepo              string   `json:"githubRepo,omitempty"`
	IssueTracker            string   `json:"issueTracker,omitempty"`
	PRCreationStage         string   `json:"prCreationStage"`
	SchemaVersion           int      `json:"schemaVersion"`
	ProjectID               string   `json:"projectId"`
	ProjectName             string   `json:"projectName"`
	Description             string   `json:"description"`
	GitRemoteURL            string   `json:"gitRemoteUrl"`
	SpecFramework           string   `json:"specFramework"`
	UseWorktrees            bool     `json:"useWorktrees"`
	DefaultSkillMode        string   `json:"defaultSkillMode,omitempty"`
	AutonomousStopStage     string   `json:"autonomousStopStage,omitempty"`
	AIProvider              string   `json:"aiProvider"`
	SetupProviders          []string `json:"setupProviders,omitempty"`
	AICommandTemplate       string   `json:"aiCommandTemplate"`
	ExternalTerminalCommand string   `json:"externalTerminalCommand"`
	Skills                  []Skill  `json:"skills"`
}

// Dispatch carries launch intent only. Execution settings are fetched separately.
// A zero version is accepted for legacy senders; new senders always emit Version.
type Dispatch struct {
	RunID            string `json:"runId,omitempty"`
	SchemaVersion    int    `json:"schemaVersion,omitempty"`
	TaskID           string `json:"taskId"`
	TaskKey          string `json:"taskKey"`
	ProjectID        string `json:"projectId,omitempty"`
	SkillID          string `json:"skillId,omitempty"`
	Action           string `json:"action"`
	Prompt           string `json:"prompt,omitempty"`
	Command          string `json:"command,omitempty"`
	TerminalOverride string `json:"terminalOverride,omitempty"`
	// Mode is the execution mode resolved by the server: "interactive" opens a
	// terminal session, "non_interactive" runs the provider CLI headless. Empty
	// reads as interactive, which is what legacy senders mean.
	Mode string `json:"mode,omitempty"`
}

// Project is a discovery record. ID is the server primary key, not a display name.
type Project struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	GitRemoteURL string `json:"gitRemoteUrl"`
}
type Projects struct {
	SchemaVersion int       `json:"schemaVersion"`
	Projects      []Project `json:"projects"`
}
