package agentprotocol

import (
	"encoding/json"
	"errors"
	"fmt"
)

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
	Repository string `json:"repository,omitempty"`
	// Repositories names, as identities, every repository a task has a
	// worktree in, for a remove_workspace operation that must clean them all.
	// Empty keeps the operation on the project checkout.
	Repositories []string `json:"repositories,omitempty"`
	Create       bool     `json:"create,omitempty"`
	DeleteRemote bool     `json:"deleteRemote,omitempty"`
	Editor       string   `json:"editor,omitempty"`
	Prompt       string   `json:"prompt,omitempty"`
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

// Operations lists every workspace operation an agent of this build runs. The
// agent dispatches on this list and announces it when it connects, so what it
// says it can do and what it does cannot differ; the server compares it with
// its own copy to name an agent that is older than the operation it needs.
var Operations = []string{
	"macro_worktree", "macro_spec_file",
	"git_status", "git_branches", "git_checkout", "git_clean", "git_delete",
	"open_editor", "cli_status", "prepare_workspace", "remove_workspace",
	"repository_worktree", "workspace_info", "git_diff", "git_evidence",
	"pr_evidence", "run_prompt", "skills_status", "skill_files", "sync_config",
	"read_skill", "spec_status", "spec_install", "init_git",
}

// ErrUnsupportedOperation is what every "the local agent cannot run this
// operation" failure unwraps to, whichever operation and whichever caller, so
// callers recognise it without reading its text.
var ErrUnsupportedOperation = errors.New("local agent does not support the operation")

// UnsupportedOperationError names the agent that lacks an operation and the
// fix. Build is empty for an agent that predates the announcement.
type UnsupportedOperationError struct {
	Device    string
	Build     string
	Operation string
}

func (e *UnsupportedOperationError) Error() string {
	build := e.Build
	if build == "" {
		build = "unknown build"
	}
	return fmt.Sprintf("the local agent on %s (%s) does not support the %q operation; restart or update the Sectile desktop app, then retry", e.Device, build, e.Operation)
}

func (e *UnsupportedOperationError) Unwrap() error { return ErrUnsupportedOperation }

// DescribeBuild renders an agent build for UnsupportedOperationError: the
// version, and the abbreviated commit when one is known.
func DescribeBuild(version, commit string) string {
	if len(commit) > 12 {
		commit = commit[:12]
	}
	switch {
	case version == "":
		return ""
	case commit == "":
		return version
	default:
		return version + ", commit " + commit
	}
}

// IsUnknownOperationReply reports whether reply is exactly the refusal an agent
// sends for an operation it does not dispatch, for that operation. Any other
// text is a genuine failure of the operation and must be left as it is.
func IsUnknownOperationReply(reply, operation string) bool {
	return reply == UnknownOperationReply(operation)
}

// UnknownOperationReply is the agent's refusal of an operation it does not
// dispatch. Agents that predate the announcement send the same text, which is
// what lets the server recognise them.
func UnknownOperationReply(operation string) string {
	return fmt.Sprintf("unknown local operation %q", operation)
}
