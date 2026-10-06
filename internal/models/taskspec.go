package models

// TaskSpecWorkspace is where a task's clarification report and specification
// are written on a workstation (#736): a worktree of the project's Issue
// specifications folder on the task's branch when that folder is not the code
// checkout, else the task's own worktree. Repository is the Issue folder the
// workstation resolved; Distinct says whether it is another folder than the
// code checkout. An empty Branch marks a plain folder, written in place with
// nothing to commit or push.
type TaskSpecWorkspace struct {
	Repository string `json:"repository"`
	Path       string `json:"path"`
	Branch     string `json:"branch"`
	Worktree   bool   `json:"worktree"`
	Distinct   bool   `json:"distinct"`
	Warning    string `json:"warning,omitempty"`
}
