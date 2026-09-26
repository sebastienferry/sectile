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
	TrackerURL         string   `json:"trackerUrl,omitempty"`
	JiraProject        string   `json:"jiraProject,omitempty"`
	GithubRepo         string   `json:"githubRepo,omitempty"`
	IssueTracker       string   `json:"issueTracker,omitempty"`
	PRCreationStage    string   `json:"prCreationStage"`
	DefaultSkillMode   string   `json:"defaultSkillMode,omitempty"`
	FullChainStopStage string   `json:"fullChainStopStage,omitempty"`
	SchemaVersion      int      `json:"schemaVersion"`
	ProjectID          string   `json:"projectId"`
	ProjectName        string   `json:"projectName"`
	Description        string   `json:"description"`
	GitRemoteURL       string   `json:"gitRemoteUrl"`
	SpecFramework      string   `json:"specFramework"`
	UseWorktrees       bool     `json:"useWorktrees"`
	AIProvider         string   `json:"aiProvider"`
	SetupProviders     []string `json:"setupProviders,omitempty"`
	AICommandTemplate  string   `json:"aiCommandTemplate"`
	// AICommandTemplateAutonomous is additive: an agent that predates it resolves
	// nothing and keeps building the headless line it built before, from the
	// interactive template's {mode:...} marker or the provider's own command.
	AICommandTemplateAutonomous string `json:"aiCommandTemplateAutonomous,omitempty"`
	// AIModel and AISkillModels are additive: an agent that predates them resolves
	// no model and builds exactly the command lines it built before.
	AIModel                 string            `json:"aiModel,omitempty"`
	AISkillModels           map[string]string `json:"aiSkillModels,omitempty"`
	ExternalTerminalCommand string            `json:"externalTerminalCommand"`
	Skills                  []Skill           `json:"skills"`
	// Repositories are the remotes the project's tickets work in, the code
	// remote first (#456). Remotes only: the workstation maps them to folders.
	// Absent (an older server) means the code remote alone, as before.
	Repositories []string `json:"repositories,omitempty"`
	// SpecArtifacts is "drop" when the project keeps its tasks' clarification
	// and specification files out of the repository (#487), else empty or
	// "keep". After ApplyOverrides it holds this workstation's effective value.
	SpecArtifacts string `json:"specArtifacts,omitempty"`
	// EngineID and EngineName name the catalogue engine Resolve picked (#510).
	// OffProjectDefaultEngine is true when a task runs another engine than its
	// project default one, which is when a one-off launch model is ignored; its
	// zero value keeps a configuration built by hand on today's behaviour.
	// None of them is part of the contract.
	EngineID                string `json:"-"`
	EngineName              string `json:"-"`
	OffProjectDefaultEngine bool   `json:"-"`
}

// DropsSpecArtifacts reads SpecArtifacts with its default: keep.
func (c Config) DropsSpecArtifacts() bool { return c.SpecArtifacts == "drop" }

// Dispatch carries launch intent only. Execution settings are fetched separately.
// A zero version is accepted for legacy senders; new senders always emit Version.
type Dispatch struct {
	RunID         string `json:"runId,omitempty"`
	SchemaVersion int    `json:"schemaVersion,omitempty"`
	TaskID        string `json:"taskId"`
	TaskKey       string `json:"taskKey"`
	ProjectID     string `json:"projectId,omitempty"`
	SkillID       string `json:"skillId,omitempty"`
	Action        string `json:"action"`
	Prompt        string `json:"prompt,omitempty"`
	Command       string `json:"command,omitempty"`
	// Mode is the execution mode resolved by the server: "interactive" opens a
	// terminal the user answers, "autonomous" runs the CLI headless. Empty is
	// read as interactive, which keeps an older server working.
	Mode string `json:"mode,omitempty"`
	// Model is the one-off model this launch runs against. It outranks every
	// configured level of the workstation settings, for this run only.
	// Empty means no override; an agent that predates the field ignores it and
	// runs the configured model.
	Model string `json:"model,omitempty"`
	// MacroKey names the macro a macro-scoped skill runs for. A dispatch that
	// carries it has no task: TaskID and TaskKey are empty, ProjectID is set.
	// An agent that predates the field reads it as a task dispatch without a
	// task and refuses it, which is the failure a server wants to see.
	MacroKey string `json:"macroKey,omitempty"`
	// MacroTitle is the macro's title, used to name a new macro branch.
	MacroTitle string `json:"macroTitle,omitempty"`
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
