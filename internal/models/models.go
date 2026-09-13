package models

import (
	"strings"
	"time"
)

type Priority string

const (
	PriorityUrgent Priority = "urgent"
	PriorityHigh   Priority = "high"
	PriorityMedium Priority = "medium"
	PriorityLow    Priority = "low"
)

type Status string

const (
	StatusToClarify   Status = "to_clarify"   // A clarifier (Label: #new)
	StatusClarified   Status = "clarified"    // Cadré (Label: #clarified)
	StatusToImplement Status = "to_implement" // A implémenter (Label: #specified)
	StatusToTest      Status = "to_test"      // A tester (Label: #implemented)
	StatusToClose     Status = "to_close"     // En revue / PR (Label: #reviewed)
	StatusFinished    Status = "finished"     // Terminé (Label: #finished)

	// Compatibility aliases
	StatusBacklog    Status = "backlog"
	StatusSpecified  Status = "specified"
	StatusInProgress Status = "in_progress"
	StatusToValidate Status = "to_validate"
	StatusDone       Status = "done"
)

type ActivityStatus string

const (
	ActivityStatusQueued    ActivityStatus = "queued"
	ActivityStatusPending   ActivityStatus = "pending"
	ActivityStatusRunning   ActivityStatus = "running"
	ActivityStatusCompleted ActivityStatus = "completed"
	ActivityStatusFailed    ActivityStatus = "failed"
	ActivityStatusCanceled  ActivityStatus = "canceled"
)

type TaskActivity struct {
	ID          string     `json:"id"`
	TaskID      string     `json:"taskId"`
	ProjectID   string     `json:"projectId,omitempty"`
	TaskKey     string     `json:"taskKey,omitempty"`
	TaskTitle   string     `json:"taskTitle,omitempty"`
	SkillID     string     `json:"skillId"`
	SkillName   string     `json:"skillName"`
	Action      string     `json:"action"`
	Status      string     `json:"status"` // "queued", "pending", "running", "completed", "failed", "canceled"
	Summary     string     `json:"summary"`
	Output      string     `json:"output"` // Real AI or CLI Markdown / logs
	Steps       []string   `json:"steps"`
	Prompt      string     `json:"prompt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	StartedAt   *time.Time `json:"startedAt,omitempty"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
	Error       string     `json:"error,omitempty"`
	Duration    string     `json:"duration,omitempty"`
}

type ActivityStats struct {
	Total     int `json:"total"`
	Queued    int `json:"queued"`
	Running   int `json:"running"`
	Completed int `json:"completed"`
	Failed    int `json:"failed"`
	Canceled  int `json:"canceled"`
}

type Project struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	Icon        string `json:"icon"`     // "Folder", "Terminal", "Flame", "Zap", "Layers", "Code2", "Box", "Cpu"
	Color       string `json:"color"`    // "indigo", "emerald", "purple", "amber", "blue", "rose", "cyan", "orange", "neon-cyan", etc.
	RepoPath    string `json:"repoPath"` // CWD for AI skills
	// RepoPaths is the list of working directories known for this project.
	// It is fed automatically: whenever a ticket pins a new CWD, that path is
	// registered here so the next ticket can pick it instead of retyping it.
	RepoPaths []string `json:"repoPaths,omitempty"`
	// UseWorktrees decides whether each task gets its own isolated Git worktree
	// under .tasks/worktrees, or whether the agent simply runs in the clone. A
	// solo project rarely needs that isolation and pays the setup cost for
	// nothing. Default true, which is the historical behaviour.
	UseWorktrees    bool   `json:"useWorktrees"`
	PRCreationStage string `json:"prCreationStage"`
	// BoardID / TrackerColumns mirror the tracker's board: its columns in order,
	// with the statuses each one groups. Imported from the tracker, not typed by
	// hand.
	BoardID        string          `json:"boardId,omitempty"`
	TrackerColumns []TrackerColumn `json:"trackerColumns,omitempty"`
	// MonoRepo says the project lives in a single repository. The current branch,
	// the branch switcher and the branch shown on a card only mean something
	// there: on a project whose tickets span several repositories, they display
	// the branch of whichever repository happens to be configured.
	MonoRepo bool `json:"monoRepo"`
	// IssueTypes names the tracker work item types this project imports as cards.
	// Empty means the default (Task and Story). A project whose tracker exposes
	// its own type imports nothing without it: a feedback project may carry a
	// single custom type, which the default list would match on no ticket.
	IssueTypes []string `json:"issueTypes,omitempty"`
	// Sprints mirrors the board's sprints with their state, refreshed by the sync.
	Sprints []TrackerSprint `json:"sprints,omitempty"`
	// StageColumns assigns each agentic workflow stage to one or several of those
	// columns, which is what decides the skill proposed on a card.
	StageColumns map[string][]string `json:"stageColumns,omitempty"`
	GitRemoteUrl string              `json:"gitRemoteUrl"` // e.g. "git@github.com:owner/repo.git"
	LinearTeam   string              `json:"linearTeam"`   // Key / prefix (optionnel)
	GithubRepo   string              `json:"githubRepo"`   // e.g. "owner/repo"
	JiraProject  string              `json:"jiraProject"`  // Jira project key, e.g. "PE"
	IssueTracker string              `json:"issueTracker"` // "linear", "github", "jira", "local"
	TrackerUrl   string              `json:"trackerUrl"`   // e.g. "https://linear.app/my-team/project/xxx" or "https://acme.atlassian.net"
	// ProjectType is "standard" (a delivery project) or "personal" (a personal
	// board). The daily digest is only meaningful on a personal project, so it
	// is served for that type only.
	ProjectType             string            `json:"projectType"`
	IsDefault               bool              `json:"isDefault"`
	StageMapping            map[string]string `json:"stageMapping,omitempty"`            // mapping AI workflow labels to tracker statuses
	SkillOverrides          map[string]string `json:"skillOverrides,omitempty"`          // skillId -> custom skill name override
	AIProvider              string            `json:"aiProvider,omitempty"`              // "agy", "claude", "vibe", "gemini", "cursor", "custom"
	AICommandTemplate       string            `json:"aiCommandTemplate,omitempty"`       // e.g. 'agy -p "{prompt}"'
	SpecFramework           string            `json:"specFramework,omitempty"`           // "speckit", "openspec"
	Parallelism             int               `json:"parallelism"`                       // 1 to 3 concurrent AI background workers
	AutoSyncEnabled         bool              `json:"autoSyncEnabled"`                   // Enable background sync for non-finished tickets
	AutoSyncIntervalMin     int               `json:"autoSyncIntervalMin"`               // Period in minutes (1 to 30)
	TtyMode                 string            `json:"ttyMode,omitempty"`                 // "integrated" or "external"
	ExternalTerminalCommand string            `json:"externalTerminalCommand,omitempty"` // e.g. "Ghostty", "Terminal", "iTerm", "alacritty", "kitty"
	TaskCount               int               `json:"taskCount"`
	CreatedAt               time.Time         `json:"createdAt"`
	UpdatedAt               time.Time         `json:"updatedAt"`
}

// TrackerColumn is one column of the tracker's own board, with the tracker
// statuses it groups. A column can group several statuses, as "TO
// MERGE/DEPLOY" groups "To Merge" and "To Deploy" on the PE board.
// TaskComment is a comment on a work item. On a tracker-backed task they are
// read from and written to the tracker, which is the source of truth; a local
// task keeps them in the local table instead.
type TaskComment struct {
	ID        string     `json:"id"`
	TaskID    string     `json:"taskId,omitempty"`
	Author    string     `json:"author"`
	Body      string     `json:"body"`
	CreatedAt *time.Time `json:"createdAt,omitempty"`
	Source    string     `json:"source"` // "jira", "github", "linear", "local"
}

type TrackerColumn struct {
	Name     string   `json:"name"`
	Statuses []string `json:"statuses"`
	// Hidden keeps a column out of the board without losing its status
	// assignment: a team rarely wants Done or Blocked in the way every day.
	Hidden bool `json:"hidden,omitempty"`
}

// TrackerSprint is a sprint of the project's board with its state, which is what
// separates the operational horizons: a ticket belongs to NOW when its sprint is
// active, and to NEXT when its sprint is still in the future.
type TrackerSprint struct {
	// ID is the tracker's own sprint id. Moving a work item into a sprint goes
	// through the Agile API, which addresses sprints by id, never by name.
	ID    string `json:"id,omitempty"`
	Name  string `json:"name"`
	State string `json:"state"` // "active", "future", "closed"
	// StartDate et EndDate viennent du board. Sans elles, l'ordre chronologique
	// se devine par le nom, ce qui casse dès qu'une équipe nomme autrement.
	StartDate string `json:"startDate,omitempty"`
	EndDate   string `json:"endDate,omitempty"`
}

// MacroMeta is the macro-level data TaskFlow owns. Macros are containers referenced by their children
// — so their horizon, their framing notes and their todo list have nowhere else to live.
type MacroMeta struct {
	ProjectID      string      `json:"projectId"`
	Key            string      `json:"key"`
	Horizon        string      `json:"horizon"` // "now", "next", "later", "hidden", "" = non classé
	Description    string      `json:"description"`
	FramingComment string      `json:"framingComment,omitempty"`
	Todos          []MacroTodo `json:"todos"`
	// Title et Status viennent du ticket macro lui-même, que la synchro n'importe
	// pas comme carte. Closed permet de sortir de la roadmap ce qui est terminé
	// sans avoir à deviner depuis l'état des enfants.
	Title     string    `json:"title,omitempty"`
	Status    string    `json:"status,omitempty"`
	Closed    bool      `json:"closed"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// MacroTodo is one shaping item on a macro, before it becomes a story.
type MacroTodo struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	Done     bool   `json:"done"`
	StoryKey string `json:"storyKey,omitempty"`
}

// Backwards compatibility aliases
type EpicMeta = MacroMeta
type EpicTodo = MacroTodo

// TrackerBoard is a board of the tracker, for the project picker.
type TrackerBoard struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// TrackerTeam is a team of the tracker (an Atlassian team on Jira Cloud) as it
// appears on the work items of a project, with the people it carries. The team
// is optional on a work item: a project may well have tickets with no team at
// all, and those keep working exactly as before.
type TrackerTeam struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// MemberCount is served even when Members is trimmed from the payload.
	MemberCount int          `json:"memberCount"`
	Members     []TeamMember `json:"members,omitempty"`
	// SyncedAt is when the members were last read from the tracker, empty when
	// they have never been read.
	SyncedAt string `json:"syncedAt,omitempty"`
	// TaskCount is the number of synced work items carrying this team.
	TaskCount int `json:"taskCount"`
}

// TeamMember is one person of a tracker team. AccountID is what the tracker
// assigns work items by; DisplayName is what it shows on the card.
type TeamMember struct {
	TeamID      string `json:"teamId"`
	TeamName    string `json:"teamName,omitempty"`
	AccountID   string `json:"accountId"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email,omitempty"`
	AvatarURL   string `json:"avatarUrl,omitempty"`
	Active      bool   `json:"active"`
}

// TeamMemberLoad is one member of a team with the work items assigned to them,
// for the workload view.
type TeamMemberLoad struct {
	Member TeamMember `json:"member"`
	// Tasks are the member's work items, most recently updated first.
	Tasks []Task `json:"tasks"`
	// ByStatus counts the member's work items per internal status.
	ByStatus map[string]int `json:"byStatus"`
	Total    int            `json:"total"`
}

// TeamWorkload is a team, its members and their load, plus the work items of the
// team that nobody in it owns.
type TeamWorkload struct {
	Team    TrackerTeam      `json:"team"`
	Members []TeamMemberLoad `json:"members"`
	// Unassigned are the team's work items with no assignee at all.
	Unassigned []Task `json:"unassigned"`
	// Outside are the team's work items assigned to someone who is not a member
	// of the team, which is worth seeing rather than hiding.
	Outside []TeamMemberLoad `json:"outside"`
}

type CreateProjectRequest struct {
	// IssueTypes names the tracker work item types to import. Empty means the
	// default list.
	IssueTypes []string `json:"issueTypes,omitempty"`
	// MonoRepo defaults to true when absent: a single repository is the common
	// case, and it is what the tool did before the setting existed.
	MonoRepo                *bool             `json:"monoRepo,omitempty"`
	Name                    string            `json:"name"`
	Slug                    string            `json:"slug,omitempty"`
	Description             string            `json:"description,omitempty"`
	Icon                    string            `json:"icon,omitempty"`
	Color                   string            `json:"color,omitempty"`
	RepoPath                string            `json:"repoPath,omitempty"`
	RepoPaths               []string          `json:"repoPaths,omitempty"`
	PRCreationStage         string            `json:"prCreationStage,omitempty"`
	UseWorktrees            *bool             `json:"useWorktrees,omitempty"`
	BoardID                 string            `json:"boardId,omitempty"`
	GitRemoteUrl            string            `json:"gitRemoteUrl,omitempty"`
	LinearTeam              string            `json:"linearTeam,omitempty"`
	GithubRepo              string            `json:"githubRepo,omitempty"`
	JiraProject             string            `json:"jiraProject,omitempty"`
	IssueTracker            string            `json:"issueTracker,omitempty"`
	TrackerUrl              string            `json:"trackerUrl,omitempty"`
	ProjectType             string            `json:"projectType,omitempty"`
	IsDefault               bool              `json:"isDefault,omitempty"`
	StageMapping            map[string]string `json:"stageMapping,omitempty"`
	SkillOverrides          map[string]string `json:"skillOverrides,omitempty"`
	AIProvider              string            `json:"aiProvider,omitempty"`
	AICommandTemplate       string            `json:"aiCommandTemplate,omitempty"`
	SpecFramework           string            `json:"specFramework,omitempty"`
	Parallelism             int               `json:"parallelism,omitempty"`
	AutoSyncEnabled         *bool             `json:"autoSyncEnabled,omitempty"`
	AutoSyncIntervalMin     *int              `json:"autoSyncIntervalMin,omitempty"`
	TtyMode                 string            `json:"ttyMode,omitempty"`
	ExternalTerminalCommand string            `json:"externalTerminalCommand,omitempty"`
}

type UpdateProjectRequest struct {
	Name                    *string              `json:"name,omitempty"`
	Slug                    *string              `json:"slug,omitempty"`
	Description             *string              `json:"description,omitempty"`
	Icon                    *string              `json:"icon,omitempty"`
	Color                   *string              `json:"color,omitempty"`
	RepoPath                *string              `json:"repoPath,omitempty"`
	RepoPaths               *[]string            `json:"repoPaths,omitempty"`
	PRCreationStage         *string              `json:"prCreationStage,omitempty"`
	UseWorktrees            *bool                `json:"useWorktrees,omitempty"`
	BoardID                 *string              `json:"boardId,omitempty"`
	TrackerColumns          *[]TrackerColumn     `json:"trackerColumns,omitempty"`
	Sprints                 *[]TrackerSprint     `json:"sprints,omitempty"`
	IssueTypes              *[]string            `json:"issueTypes,omitempty"`
	MonoRepo                *bool                `json:"monoRepo,omitempty"`
	StageColumns            *map[string][]string `json:"stageColumns,omitempty"`
	GitRemoteUrl            *string              `json:"gitRemoteUrl,omitempty"`
	LinearTeam              *string              `json:"linearTeam,omitempty"`
	GithubRepo              *string              `json:"githubRepo,omitempty"`
	JiraProject             *string              `json:"jiraProject,omitempty"`
	IssueTracker            *string              `json:"issueTracker,omitempty"`
	TrackerUrl              *string              `json:"trackerUrl,omitempty"`
	ProjectType             *string              `json:"projectType,omitempty"`
	IsDefault               *bool                `json:"isDefault,omitempty"`
	StageMapping            *map[string]string   `json:"stageMapping,omitempty"`
	SkillOverrides          *map[string]string   `json:"skillOverrides,omitempty"`
	AIProvider              *string              `json:"aiProvider,omitempty"`
	AICommandTemplate       *string              `json:"aiCommandTemplate,omitempty"`
	SpecFramework           *string              `json:"specFramework,omitempty"`
	Parallelism             *int                 `json:"parallelism,omitempty"`
	AutoSyncEnabled         *bool                `json:"autoSyncEnabled,omitempty"`
	AutoSyncIntervalMin     *int                 `json:"autoSyncIntervalMin,omitempty"`
	TtyMode                 *string              `json:"ttyMode,omitempty"`
	ExternalTerminalCommand *string              `json:"externalTerminalCommand,omitempty"`
}

// NormalizeAutoSyncIntervalMin clamps the project background sync interval between 1 and 30 minutes (default 5).
func NormalizeAutoSyncIntervalMin(min int) int {
	if min < 1 {
		return 5
	}
	if min > 30 {
		return 30
	}
	return min
}

// NormalizeParallelism keeps the concurrent background agent workers count between 1 and 3.
func NormalizeParallelism(p int) int {
	if p < 1 {
		return 1
	}
	if p > 3 {
		return 3
	}
	return p
}

type InstalledSkillInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Installed   bool   `json:"installed"`
	Path        string `json:"path"`
	Description string `json:"description"`
}

// SkillDirNames maps a workflow skill id to the directory its SKILL.md lives
// in, which is also its slash command. It sits in models because both the
// database (which renders the files) and the runner (which invokes the agent)
// need it, and a second list would drift.
var SkillDirNames = map[string]string{
	"clarify":       "clarify-issue",
	"specify":       "specify-issue",
	"implement":     "code-issue",
	"adjust":        "adjust-issue",
	"adjust-issue":  "adjust-issue",
	"create-pr":     "create-pr",
	"create_pr":     "create-pr",
	"review":        "adjust-issue",
	"handoff":       "handoff-issue",
	"pickup":        "pickup-issue",
	"pick":          "pickup-issue",
	"pickup_issues": "pickup-issues",
	"pickup-issues": "pickup-issues",
	"pick_issues":   "pickup-issues",
	"pick-issues":   "pickup-issues",
	"rewrite_story": "rewrite-story",
	"rewrite-story": "rewrite-story",
	"rewrite":       "rewrite-story",
	"refine_macro":  "refine-macro",
	"refine-macro":  "refine-macro",
	"refine":        "refine-macro",
}

// SkillAgentDirs are the per-repository directories the agent CLIs read their
// skills from. ".skills" at the root is the CLI-agnostic convention.
var SkillAgentDirs = []string{".claude", ".agents", ".gemini", ".agy", ""}

// SkillEditorEntry is one workflow skill as the in-app editor sees it: the
// content Taskacao holds, the built-in default it may override, and whether the
// file on disk still matches. Divergence is surfaced rather than overwritten:
// the repository file may carry hand edits worth keeping.
type SkillEditorEntry struct {
	LegacyContents         map[string]string `json:"legacyContents,omitempty"`
	OverrideOrigin         string            `json:"overrideOrigin,omitempty"`
	LegacyConflicts        []string          `json:"legacyConflicts,omitempty"`
	RequiresReconciliation bool              `json:"requiresReconciliation"`
	ID                     string            `json:"id"`
	Name                   string            `json:"name"`
	DirName                string            `json:"dirName"`
	Command                string            `json:"command"`
	Description            string            `json:"description"`
	FromStage              string            `json:"fromStage"`
	ToStage                string            `json:"toStage"`
	Scope                  string            `json:"scope,omitempty"`
	Interactive            bool              `json:"interactive"`
	Content                string            `json:"content"`
	DefaultContent         string            `json:"defaultContent"`
	IsCustom               bool              `json:"isCustom"`
	UpdatedAt              string            `json:"updatedAt,omitempty"`
	Installed              bool              `json:"installed"`
	Paths                  []string          `json:"paths"`
	Diverged               bool              `json:"diverged"`
	RepoContent            string            `json:"repoContent,omitempty"`
	RepoPath               string            `json:"repoPath,omitempty"`
}

type ProjectSkillsStatus struct {
	ProjectID      string               `json:"projectId"`
	ProjectName    string               `json:"projectName"`
	RepoPath       string               `json:"repoPath"`
	PathExists     bool                 `json:"pathExists"`
	IsGitRepo      bool                 `json:"isGitRepo"`
	GitBranch      string               `json:"gitBranch,omitempty"`
	InstalledAll   bool                 `json:"installedAll"`
	SpecFramework  string               `json:"specFramework,omitempty"`
	WorktreesCount int                  `json:"worktreesCount,omitempty"`
	WorktreePaths  []string             `json:"worktreePaths,omitempty"`
	Skills         []InstalledSkillInfo `json:"skills"`
}

type ProjectGitInitResult struct {
	RepoPath    string `json:"repoPath"`
	IsGitRepo   bool   `json:"isGitRepo"`
	Branch      string `json:"branch"`
	Message     string `json:"message"`
	Initialized bool   `json:"initialized"`
}

type DetectedStatus struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Type   string `json:"type,omitempty"` // "backlog", "unstarted", "started", "completed", "canceled", "triage", "custom"
	Color  string `json:"color,omitempty"`
	Source string `json:"source,omitempty"` // "linear", "github", "db", "preset"
}

type GitBranchItem struct {
	Name      string `json:"name"`
	IsCurrent bool   `json:"isCurrent"`
	IsRemote  bool   `json:"isRemote"`
	Commit    string `json:"commit,omitempty"`
	Message   string `json:"message,omitempty"`
}

type GitBranchesInfo struct {
	RepoPath      string          `json:"repoPath"`
	CurrentBranch string          `json:"currentBranch"`
	Branches      []GitBranchItem `json:"branches"`
}

type CleanBranchesResult struct {
	RepoPath        string   `json:"repoPath"`
	DefaultBranch   string   `json:"defaultBranch"`
	DeletedBranches []string `json:"deletedBranches"`
	Message         string   `json:"message"`
}

type GitDiffFile struct {
	Path      string `json:"path"`
	OldPath   string `json:"oldPath,omitempty"`
	Status    string `json:"status"` // "modified", "added", "deleted", "renamed"
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Diff      string `json:"diff"`
}

type GitDiffResult struct {
	TaskKey      string        `json:"taskKey"`
	Branch       string        `json:"branch"`
	BaseBranch   string        `json:"baseBranch"`
	RepoPath     string        `json:"repoPath"`
	WorktreePath string        `json:"worktreePath,omitempty"`
	IsClean      bool          `json:"isClean"`
	FilesChanged int           `json:"filesChanged"`
	Insertions   int           `json:"insertions"`
	Deletions    int           `json:"deletions"`
	Files        []GitDiffFile `json:"files"`
	RawDiff      string        `json:"rawDiff"`
	PrURL        *string       `json:"prUrl,omitempty"`
	Error        string        `json:"error,omitempty"`
}

type GitStatusInfo struct {
	RepoPath       string `json:"repoPath"`
	IsGitRepo      bool   `json:"isGitRepo"`
	Branch         string `json:"branch"`
	BaseBranch     string `json:"baseBranch,omitempty"`
	IsClean        bool   `json:"isClean"`
	ModifiedCount  int    `json:"modifiedCount"`
	UntrackedCount int    `json:"untrackedCount"`
	Ahead          int    `json:"ahead"`
	Behind         int    `json:"behind"`
	RemoteName     string `json:"remoteName,omitempty"`
	RemoteURL      string `json:"remoteUrl,omitempty"`
	LatestCommit   string `json:"latestCommit,omitempty"`
	Error          string `json:"error,omitempty"`
}

type WorktreeInfo struct {
	TaskKey      string `json:"taskKey"`
	Branch       string `json:"branch"`
	WorktreePath string `json:"worktreePath"`
	Exists       bool   `json:"exists"`
	MainRepoPath string `json:"mainRepoPath"`
}

type Task struct {
	ID             string   `json:"id"`
	ProjectID      string   `json:"projectId"`
	Key            string   `json:"key"`
	Title          string   `json:"title"`
	Description    string   `json:"description"`
	Status         Status   `json:"status"`
	Priority       Priority `json:"priority"`
	Labels         []string `json:"labels"`
	Assignee       string   `json:"assignee"`
	AssigneeAvatar string   `json:"assigneeAvatar"`
	Position       int      `json:"position"`
	DueDate        *string  `json:"dueDate"`
	BranchName     *string  `json:"branchName,omitempty"`
	PrURL          *string  `json:"prUrl,omitempty"`
	WorktreePath   *string  `json:"worktreePath,omitempty"`
	// RepoPath pins the repository this single ticket works in. It overrides the
	// project's repoPath, for trackers where one epic spans several codebases.
	// Empty means "inherit the project, then the global setting".
	RepoPath *string `json:"repoPath,omitempty"`
	// TrackerStatus is the status name as the tracker spells it ("Dev Test", "To
	// Merge"…). The internal Status folds those onto six values, which is too
	// lossy to place a card in the tracker's own board columns.
	TrackerStatus string `json:"trackerStatus,omitempty"`
	// Sprint and Team come from the tracker: the Jira Sprint and Team fields,
	// imported by the REST enrichment pass of the sync.
	Sprint string `json:"sprint,omitempty"`
	Team   string `json:"team,omitempty"`
	// TeamID is the Atlassian team id behind Team. The label alone cannot be
	// used to read a team's members: the members endpoint is keyed by id, and
	// two teams may carry the same name.
	TeamID      string  `json:"teamId,omitempty"`
	Source      string  `json:"source"` // "linear", "github", "jira", "local"
	ExternalURL *string `json:"externalUrl,omitempty"`
	// IssueType is the tracker's own work item type. Only "Task" and "Story"
	// are imported; epics and other types stay out of the board.
	IssueType string `json:"issueType,omitempty"`
	// ParentKey / ParentTitle / ParentType describe the work item this task
	// hangs under — an epic, or a parent story for a sub-task. The parent is a
	// property of the task rather than a card of its own.
	ParentKey   string `json:"parentKey,omitempty"`
	ParentTitle string `json:"parentTitle,omitempty"`
	ParentType  string `json:"parentType,omitempty"` // "Epic", "Story", …
	// TrackerCreatedAt / TrackerUpdatedAt are the tracker's own dates, when it
	// gave them. CreatedAt and UpdatedAt below are local: on a synced ticket they
	// hold the import time, which is why they cannot answer "open for how long".
	// Nil means the tracker did not provide them, and the digest then says so
	// rather than counting days from an import.
	TrackerCreatedAt *time.Time `json:"trackerCreatedAt,omitempty"`
	TrackerUpdatedAt *time.Time `json:"trackerUpdatedAt,omitempty"`
	// StatusChangedAt is when the work item entered its current status category,
	// which is what "in progress for four days" counts from. It is the category
	// and not the precise status: two statuses of the same category, such as a
	// review column and a merge column, do not move it.
	StatusChangedAt *time.Time     `json:"statusChangedAt,omitempty"`
	Activities      []TaskActivity `json:"activities,omitempty"`
	Pinned          bool           `json:"pinned,omitempty"`
	CreatedAt       time.Time      `json:"createdAt"`
	UpdatedAt       time.Time      `json:"updatedAt"`
}

type Settings struct {
	ID          int    `json:"id"`
	Theme       string `json:"theme"`       // "dark", "light", "system"
	AccentColor string `json:"accentColor"` // "indigo", "violet", "emerald", "amber", "rose", "cyan", "blue", "orange"
	Language    string `json:"language"`    // "fr", "en"
	Density     string `json:"density"`     // "compact", "standard", "comfortable"
	DefaultView string `json:"defaultView"` // "board", "list"
	// AutoSyncEnabled runs a background loop that reads from the tracker what
	// changed since the last pass. It is incremental by construction: a full
	// pass on a 1400 ticket project costs fourteen paginated requests, an
	// incremental one costs a single request that usually answers nothing.
	AutoSyncEnabled bool `json:"autoSyncEnabled"`
	// AutoSyncIntervalSec is that loop's period, in seconds. Floored at 30.
	AutoSyncIntervalSec int `json:"autoSyncIntervalSec"`
	// UIScale is the interface zoom in percent (90, 100, 110, 125). Density only
	// moves the root font size, which leaves every fixed pixel size untouched;
	// the scale zooms the whole interface, which is what a large or a small
	// screen actually needs.
	UIScale           int    `json:"uiScale"`
	DetailMode        string `json:"detailMode"` // "panel", "modal"
	UserName          string `json:"userName"`
	UserEmail         string `json:"userEmail"`
	UserAvatar        string `json:"userAvatar"`
	AIProvider        string `json:"aiProvider"`        // "agy", "vibe", "claude", "custom"
	AICommandTemplate string `json:"aiCommandTemplate"` // e.g. 'agy -p "{prompt}"'
	RepoPath          string `json:"repoPath"`          // e.g. '/path/to/project'
	IssueTracker      string `json:"issueTracker"`      // "linear", "github", "jira", "local"
	LinearTeam        string `json:"linearTeam"`        // e.g. "ENG"
	GithubRepo        string `json:"githubRepo"`        // e.g. "owner/repo"
	JiraProject       string `json:"jiraProject"`       // e.g. "PE"
	JiraUrl           string `json:"jiraUrl"`
	// JiraEmail / JiraAPIToken authenticate the Jira REST calls that fetch the
	// fields acli cannot return (Sprint and Team are custom fields, and acli's
	// --fields only accepts a fixed allow-list). Basic auth over HTTPS.
	JiraEmail string `json:"jiraEmail"`
	// JiraAPIToken never leaves the server: the API responses carry the two
	// flags below instead, so the token cannot be read back by anything that
	// can reach the settings endpoint.
	JiraAPIToken        string `json:"jiraApiToken,omitempty"`
	JiraAPITokenSet     bool   `json:"jiraApiTokenSet"`
	JiraAPITokenFromEnv bool   `json:"jiraApiTokenFromEnv"` // e.g. "https://acme.atlassian.net"
	// PromptDigestAgenda replaces the built-in agenda prompt of the daily digest.
	// Empty keeps the default. Placeholders: {project}, {date}.
	PromptDigestAgenda      string    `json:"promptDigestAgenda"`
	PromptClarify           string    `json:"promptClarify"`
	PromptSpecify           string    `json:"promptSpecify"`
	PromptImplement         string    `json:"promptImplement"`
	PromptCreatePR          string    `json:"promptCreatePr"`
	PromptPick              string    `json:"promptPick"`
	EditorCommand           string    `json:"editorCommand"`                     // "code", "cursor", "zed", "subl", etc.
	ExternalTerminalCommand string    `json:"externalTerminalCommand,omitempty"` // e.g. "Terminal", "iTerm", "Ghostty", "alacritty", "kitty"
	SpecFramework           string    `json:"specFramework"`                     // "speckit", "openspec"
	UpdatedAt               time.Time `json:"updatedAt"`
}

// SpecFrameworkInstallRequest asks Taskacao to bootstrap a Spec-Driven Design
// toolchain (GitHub Spec Kit or OpenSpec) inside a project working directory.
type SpecFrameworkInstallRequest struct {
	Framework string `json:"framework"` // "speckit" or "openspec"
	RepoPath  string `json:"repoPath,omitempty"`
	ProjectID string `json:"projectId,omitempty"`
	AIAgent   string `json:"aiAgent,omitempty"` // "claude", "gemini", "copilot", "cursor", "codex", ...
	Force     bool   `json:"force,omitempty"`   // re-run the initializer over an existing install
}

// SpecFrameworkStep records one shell command executed by the installer.
type SpecFrameworkStep struct {
	Label   string `json:"label"`
	Command string `json:"command"`
	Success bool   `json:"success"`
	Skipped bool   `json:"skipped"`
	Output  string `json:"output,omitempty"`
	Error   string `json:"error,omitempty"`
}

// SpecFrameworkInstallResult is the outcome of a Spec Kit / OpenSpec bootstrap.
type SpecFrameworkInstallResult struct {
	Framework      string              `json:"framework"`
	FrameworkLabel string              `json:"frameworkLabel"`
	RepoPath       string              `json:"repoPath"`
	Installed      bool                `json:"installed"`
	AlreadyInit    bool                `json:"alreadyInit"`
	Version        string              `json:"version,omitempty"`
	MarkerPaths    []string            `json:"markerPaths,omitempty"`
	Steps          []SpecFrameworkStep `json:"steps"`
	Message        string              `json:"message"`
	Error          string              `json:"error,omitempty"`
}

// SpecFrameworkStatus reports whether a SDD toolchain is present on the host
// (CLI reachable) and initialized in the project working directory.
type SpecFrameworkStatus struct {
	Framework      string   `json:"framework"`
	FrameworkLabel string   `json:"frameworkLabel"`
	RepoPath       string   `json:"repoPath"`
	CliAvailable   bool     `json:"cliAvailable"`
	CliCommand     string   `json:"cliCommand"`
	Initialized    bool     `json:"initialized"`
	MarkerPaths    []string `json:"markerPaths,omitempty"`
	InstallHint    string   `json:"installHint,omitempty"`
}

type MoveTaskRequest struct {
	Status   Status `json:"status"`
	Position int    `json:"position"`
}

type CreateTaskRequest struct {
	RequireRemoteCreation bool     `json:"requireRemoteCreation,omitempty"`
	ProjectID             string   `json:"projectId,omitempty"`
	Title                 string   `json:"title"`
	Description           string   `json:"description"`
	Status                Status   `json:"status"`
	Priority              Priority `json:"priority"`
	Labels                []string `json:"labels"`
	Assignee              string   `json:"assignee"`
	AssigneeAvatar        string   `json:"assigneeAvatar"`
	DueDate               *string  `json:"dueDate"`
	Source                string   `json:"source,omitempty"`
	ExternalURL           *string  `json:"externalUrl,omitempty"`
	Sprint                string   `json:"sprint,omitempty"`
	IssueType             string   `json:"issueType,omitempty"`
	ParentKey             string   `json:"parentKey,omitempty"`
	ParentTitle           string   `json:"parentTitle,omitempty"`
	ParentType            string   `json:"parentType,omitempty"`
}

type CloneTaskRequest struct {
	Title              string   `json:"title,omitempty"`
	ProjectID          string   `json:"projectId,omitempty"`
	Status             Status   `json:"status,omitempty"`
	Priority           Priority `json:"priority,omitempty"`
	Sprint             string   `json:"sprint,omitempty"`
	Assignee           string   `json:"assignee,omitempty"`
	AssigneeAvatar     string   `json:"assigneeAvatar,omitempty"`
	IncludeDescription *bool    `json:"includeDescription,omitempty"`
	IncludeLabels      *bool    `json:"includeLabels,omitempty"`
	IncludeParent      *bool    `json:"includeParent,omitempty"`
	IncludeSprint      *bool    `json:"includeSprint,omitempty"`
	IncludeAssignee    *bool    `json:"includeAssignee,omitempty"`
	Source             string   `json:"source,omitempty"`
	IssueType          string   `json:"issueType,omitempty"`
}

type UpdateTaskRequest struct {
	ProjectID         *string   `json:"projectId,omitempty"`
	Title             *string   `json:"title,omitempty"`
	Description       *string   `json:"description,omitempty"`
	Status            *Status   `json:"status,omitempty"`
	Priority          *Priority `json:"priority,omitempty"`
	Labels            *[]string `json:"labels,omitempty"`
	Assignee          *string   `json:"assignee,omitempty"`
	AssigneeAvatar    *string   `json:"assigneeAvatar,omitempty"`
	AssigneeAccountID *string   `json:"assigneeAccountId,omitempty"`
	Position          *int      `json:"position,omitempty"`
	DueDate           *string   `json:"dueDate,omitempty"`
	BranchName        *string   `json:"branchName,omitempty"`
	PrURL             *string   `json:"prUrl,omitempty"`
	RepoPath          *string   `json:"repoPath,omitempty"`
	TrackerStatus     *string   `json:"trackerStatus,omitempty"`
	Sprint            *string   `json:"sprint,omitempty"`
	Source            *string   `json:"source,omitempty"`
	ExternalURL       *string   `json:"externalUrl,omitempty"`
	IssueType         *string   `json:"issueType,omitempty"`
}

type Skill struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Command      string   `json:"command"`
	Description  string   `json:"description"`
	InputStatus  Status   `json:"inputStatus"`
	OutputStatus Status   `json:"outputStatus"`
	Icon         string   `json:"icon"`
	Color        string   `json:"color"`
	Steps        []string `json:"steps"`
}

type RunSkillRequest struct {
	SkillID      string `json:"skillId"`
	Prompt       string `json:"prompt,omitempty"`
	WithComments bool   `json:"withComments,omitempty"`
}

type RunSkillResponse struct {
	Task     Task         `json:"task"`
	Activity TaskActivity `json:"activity"`
	Message  string       `json:"message"`
}

type CliStatus struct {
	Tool       string `json:"tool"`
	Available  bool   `json:"available"`
	Path       string `json:"path"`
	AuthStatus string `json:"authStatus"`
	Details    string `json:"details"`
}

type ConvertTaskRequest struct {
	Target string `json:"target"` // "linear" or "github"
}

// -------------------------------------------------------------
// DAILY DIGEST
// -------------------------------------------------------------
//
// The digest is hybrid by design. Everything derived from the project's tasks is
// computed deterministically by Taskacao, so opening the view is instant and
// works offline. The agenda section is the only part that needs the project's AI
// agent, because meetings live in the user's calendar which Taskacao cannot see.

// DigestTaskRef is the compact view of a task inside a digest section.
type DigestTaskRef struct {
	Key         string   `json:"key"`
	Title       string   `json:"title"`
	Status      Status   `json:"status"`
	Priority    Priority `json:"priority"`
	IssueType   string   `json:"issueType,omitempty"`
	Assignee    string   `json:"assignee,omitempty"`
	ParentKey   string   `json:"parentKey,omitempty"`
	ParentTitle string   `json:"parentTitle,omitempty"`
	ExternalURL *string  `json:"externalUrl,omitempty"`
	BranchName  *string  `json:"branchName,omitempty"`
	PrURL       *string  `json:"prUrl,omitempty"`
	DueDate     *string  `json:"dueDate,omitempty"`
	// AgeDays is the number of days since creation; DaysToDue is negative when
	// the due date is already past.
	AgeDays int  `json:"ageDays"`
	IsStale bool `json:"isStale"`
	// DatesUnknown marks a task whose tracker dates were not available at sync
	// time, so age and closing date must not be presented as facts.
	DatesUnknown bool `json:"datesUnknown,omitempty"`
	DaysToDue    *int `json:"daysToDue,omitempty"`
}

// DigestMacroGroup summarises how much open work hangs under one parent macro.
type DigestMacroGroup struct {
	ParentKey   string `json:"parentKey"`
	ParentTitle string `json:"parentTitle,omitempty"`
	OpenCount   int    `json:"openCount"`
	DoneCount   int    `json:"doneCount"`
}

type DigestEpicGroup = DigestMacroGroup

// DigestStats are the headline counters of a digest.
type DigestStats struct {
	TotalOpen      int `json:"totalOpen"`
	Urgent         int `json:"urgent"`
	High           int `json:"high"`
	Stale          int `json:"stale"`
	Overdue        int `json:"overdue"`
	AwaitingReview int `json:"awaitingReview"`
	DoneLast7Days  int `json:"doneLast7Days"`
	// Dormant is the open work nobody has touched inside the watch window. It is
	// counted rather than listed: a brief that prints the whole backlog sorts
	// nothing, and this number is the honest way to say what was left out.
	Dormant int `json:"dormant"`
	// Counters for what could not be dated, so the UI never implies zero.
	OpenDateUnknown   int `json:"openDateUnknown"`
	ClosedDateUnknown int `json:"closedDateUnknown"`
}

// DailyDigest is one project's brief for one day.
type DailyDigest struct {
	ProjectID   string `json:"projectId"`
	ProjectName string `json:"projectName"`
	Date        string `json:"date"` // YYYY-MM-DD
	// Assignee narrows the digest to one person; empty means the whole project.
	Assignee string `json:"assignee"`
	// Assignees lists every assignee present in the project's tasks, so the UI
	// can offer the tracker's own spelling rather than guessing.
	Assignees []string `json:"assignees,omitempty"`

	// Deterministic, task-derived sections.
	Focus          []DigestTaskRef    `json:"focus"`          // urgent / high, act today
	Watch          []DigestTaskRef    `json:"watch"`          // medium, this week
	Stale          []DigestTaskRef    `json:"stale"`          // high+ open too long
	DueSoon        []DigestTaskRef    `json:"dueSoon"`        // overdue or due within a week
	AwaitingReview []DigestTaskRef    `json:"awaitingReview"` // has a PR or sits in review
	RecentlyDone   []DigestTaskRef    `json:"recentlyDone"`   // closed in the last 7 days
	ByMacro        []DigestMacroGroup `json:"byMacro"`
	ByEpic         []DigestMacroGroup `json:"byEpic,omitempty"`
	Stats          DigestStats        `json:"stats"`

	// AI enrichment (agenda / meetings).
	Agenda       string     `json:"agenda,omitempty"`
	AIStatus     string     `json:"aiStatus"` // "none" | "queued" | "running" | "completed" | "failed"
	AIError      string     `json:"aiError,omitempty"`
	AIActivityID string     `json:"aiActivityId,omitempty"`
	AIUpdatedAt  *time.Time `json:"aiUpdatedAt,omitempty"`

	Markdown    string    `json:"markdown"`
	GeneratedAt time.Time `json:"generatedAt"`
}

// DailyDigestRequest asks for a digest, optionally triggering the agenda pass.
type DailyDigestRequest struct {
	Date     string `json:"date,omitempty"`     // YYYY-MM-DD, defaults to today
	Assignee string `json:"assignee,omitempty"` // narrow to one person
	Enrich   bool   `json:"enrich,omitempty"`   // run the AI agenda pass
}

// TaskPostBackPayload represents an incoming update payload from local actions or background tracker operations.
type TaskPostBackPayload struct {
	TaskID           string     `json:"taskId"`
	TaskKey          string     `json:"taskKey,omitempty"`
	ProjectID        string     `json:"projectId,omitempty"`
	Title            *string    `json:"title,omitempty"`
	Description      *string    `json:"description,omitempty"`
	Status           *Status    `json:"status,omitempty"`
	Stage            *string    `json:"stage,omitempty"`
	Assignee         *string    `json:"assignee,omitempty"`
	AssigneeAvatar   *string    `json:"assigneeAvatar,omitempty"`
	BranchName       *string    `json:"branchName,omitempty"`
	PrURL            *string    `json:"prUrl,omitempty"`
	Labels           *[]string  `json:"labels,omitempty"`
	TrackerStatus    *string    `json:"trackerStatus,omitempty"`
	Sprint           *string    `json:"sprint,omitempty"`
	Team             *string    `json:"team,omitempty"`
	TeamID           *string    `json:"teamId,omitempty"`
	ParentKey        *string    `json:"parentKey,omitempty"`
	ParentTitle      *string    `json:"parentTitle,omitempty"`
	ParentType       *string    `json:"parentType,omitempty"`
	TrackerUpdatedAt *time.Time `json:"trackerUpdatedAt,omitempty"`
	Error            *string    `json:"error,omitempty"`
	ActivityID       string     `json:"activityId,omitempty"`
	OpKind           string     `json:"opKind,omitempty"`
}

// TaskPostBackResult represents the response returned after processing a post-back.
type TaskPostBackResult struct {
	Success  bool          `json:"success"`
	Task     *Task         `json:"task,omitempty"`
	Activity *TaskActivity `json:"activity,omitempty"`
	Error    string        `json:"error,omitempty"`
}

type ProposedMacroTask struct {
	Title       string `json:"title"`
	IssueType   string `json:"issueType"`
	Description string `json:"description"`
}

// NormalizeSkillID preserves legacy invocations without rewriting history.
func NormalizeSkillID(id string) string {
	switch strings.TrimSpace(id) {
	case "create_pr", "create-pr":
		return "create_pr"
	case "adjust", "adjust-issue", "review":
		return "adjust"
	default:
		return strings.TrimSpace(id)
	}
}
