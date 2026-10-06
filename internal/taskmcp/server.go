// Package taskmcp exposes Sectile's existing workflow services as typed MCP tools.
package taskmcp

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"tasks/internal/agentconfig"
	"tasks/internal/agentprotocol"
	"tasks/internal/db"
	"tasks/internal/models"
	"tasks/internal/tracker"
)

type taskInput struct {
	TaskKey string `json:"taskKey" jsonschema:"Task key or ID"`
}
type transitionInput struct {
	TaskKey string `json:"taskKey"`
	Stage   string `json:"stage"`
	Note    string `json:"note"`
	Branch  string `json:"branch,omitempty"`
	PRURL   string `json:"prUrl,omitempty"`
	// PRURLs are the pull requests of the other repositories the task changed.
	PRURLs []string `json:"prUrls,omitempty"`
	// NoRepositoryChange states that the task's work changed no repository,
	// in place of a pull request (#584).
	NoRepositoryChange bool `json:"noRepositoryChange,omitempty"`
}
type commentInput struct {
	TaskKey string `json:"taskKey"`
	Body    string `json:"body"`
}
type listInput struct {
	ProjectID string `json:"projectId,omitempty"`
	Status    string `json:"status,omitempty"`
	Sprint    string `json:"sprint,omitempty"`
}
type contextInput struct {
	ProjectID string `json:"projectId,omitempty"`
	TaskKey   string `json:"taskKey,omitempty"`
}

// createTaskInput mirrors the descriptive half of models.CreateTaskRequest. The
// fields a caller could use to contradict the board's own invariants (status,
// source, external URL) are deliberately absent: a task created here enters the
// workflow where every other new task enters it.
type createTaskInput struct {
	ProjectID string `json:"projectId"`
	// Tracker names one of the project's trackers, by id or identity (#741).
	Tracker     string   `json:"tracker,omitempty"`
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	IssueType   string   `json:"issueType,omitempty"`
	Priority    string   `json:"priority,omitempty"`
	Labels      []string `json:"labels,omitempty"`
	ParentKey   string   `json:"parentKey,omitempty"`
}

type updateTaskInput struct {
	TaskKey     string    `json:"taskKey"`
	Title       *string   `json:"title,omitempty"`
	Description *string   `json:"description,omitempty"`
	Priority    *string   `json:"priority,omitempty"`
	IssueType   *string   `json:"issueType,omitempty"`
	Labels      *[]string `json:"labels,omitempty"`
}

type startRunInput struct {
	TaskKey string `json:"taskKey,omitempty" jsonschema:"task key or ID; omit for a macro run"`
	Skill   string `json:"skill"`
	RunID   string `json:"runId,omitempty"`
	// ProjectID and MacroKey name a macro skill run instead of a task run. With
	// a taskKey, ProjectID names the project the task run works for (#741).
	ProjectID string `json:"projectId,omitempty" jsonschema:"project primary key of a macro run, or of the project a task run works for when the task belongs to several"`
	MacroKey  string `json:"macroKey,omitempty" jsonschema:"macro key of a macro run, with projectId instead of taskKey"`
}
type finishRunInput struct {
	TaskKey   string `json:"taskKey,omitempty" jsonschema:"task key or ID; omit for a macro run"`
	RunID     string `json:"runId"`
	Status    string `json:"status" jsonschema:"completed, failed or canceled"`
	Note      string `json:"note"`
	ProjectID string `json:"projectId,omitempty" jsonschema:"project primary key of a macro run"`
	MacroKey  string `json:"macroKey,omitempty" jsonschema:"macro key of a macro run, with projectId instead of taskKey"`
}
type reportWaitingInput struct {
	TaskKey string `json:"taskKey,omitempty" jsonschema:"task key or ID of the run; omit for a macro run"`
	RunID   string `json:"runId" jsonschema:"the runId start_run returned"`
	Waiting bool   `json:"waiting" jsonschema:"true before asking the user a blocking question, false once answered"`
	// ProjectID and MacroKey name a macro run, as start_run and finish_run do
	// (#648).
	ProjectID string `json:"projectId,omitempty" jsonschema:"project primary key of a macro run"`
	MacroKey  string `json:"macroKey,omitempty" jsonschema:"macro key of a macro run, with projectId instead of taskKey"`
}

// reportWaitingTool is the one call that must not end a wait: reporting the same
// wait twice, or clearing it, is not a sign that the session moved on.
const reportWaitingTool = "report_waiting"

// resumesWaits says whether a message proves its session is no longer blocked on
// the user. Only a tool call does: a keepalive ping arrives every minute from
// the stdio bridge whatever the model is doing.
func resumesWaits(method string, req mcp.Request) bool {
	if method != "tools/call" {
		return false
	}
	call, ok := req.(*mcp.CallToolRequest)
	return ok && call.Params != nil && call.Params.Name != reportWaitingTool
}

type repositoryWorktreeInput struct {
	TaskKey    string `json:"taskKey" jsonschema:"task key or ID"`
	Repository string `json:"repository" jsonschema:"one of the project's repositories or the remote of a Git folder attached to the project on the caller's workstation, as a remote URL or a host/path identity; with the project's Any repository option on, any repository's remote URL or host/path"`
	Path       string `json:"path,omitempty" jsonschema:"absolute path of a local checkout of the repository, whose origin is that repository; used only when the caller's workstation has the project's Any repository option on and no mapped or attached folder already holds it"`
}

type recordPullRequestInput struct {
	TaskKey string `json:"taskKey" jsonschema:"task key or ID"`
	URL     string `json:"url" jsonschema:"the pull request or merge request URL"`
}

type macroWorktreeInput struct {
	ProjectID string `json:"projectId" jsonschema:"project primary key"`
	MacroKey  string `json:"macroKey" jsonschema:"macro key, for example M-7"`
}

// macroInput names one macro of one project: a macro key alone can match
// another project's macro.
type macroInput struct {
	ProjectID string `json:"projectId" jsonschema:"project primary key"`
	MacroKey  string `json:"macroKey" jsonschema:"macro key, for example PE-12 or M-7"`
}

// macroTodosInput is the full ordered list update_macro_todos saves.
type macroTodosInput struct {
	ProjectID string           `json:"projectId" jsonschema:"project primary key"`
	MacroKey  string           `json:"macroKey" jsonschema:"macro key, for example PE-12 or M-7"`
	Todos     []macroTodoInput `json:"todos" jsonschema:"the full list in the order of execution, top first"`
}

// macroTodoInput is one line of it. The story key and the origin a todo of
// get_macro carries are accepted, so an agent can send a list back as it read
// it, and ignored: only story creation attaches a line to a story, and only an
// import says where a line comes from.
type macroTodoInput struct {
	ID                   string `json:"id,omitempty" jsonschema:"id of an existing todo, from get_macro; omit for a new todo"`
	Text                 string `json:"text" jsonschema:"one-line wording of the todo"`
	Done                 bool   `json:"done,omitempty"`
	TargetProjectID      string `json:"targetProjectId,omitempty" jsonschema:"Sectile project the todo's story is created in, empty for the macro's own"`
	TargetTrackerProject string `json:"targetTrackerProject,omitempty" jsonschema:"roadmap Jira project key the todo's story is created in"`
	StoryKey             string `json:"storyKey,omitempty" jsonschema:"ignored: an existing todo keeps its story key"`
	SourceKind           string `json:"sourceKind,omitempty" jsonschema:"ignored: an existing todo keeps its origin"`
	SourceEntry          string `json:"sourceEntry,omitempty" jsonschema:"ignored: an existing todo keeps its origin"`
}

// runTarget says whether a run input names a task or a macro, and refuses one
// that names both or neither: a macro key alone can match another project's
// macro, and a task key with a macro key is ambiguous.
func runTarget(taskKey, projectID, macroKey string) (macro bool, err error) {
	task := strings.TrimSpace(taskKey) != ""
	hasMacro := strings.TrimSpace(macroKey) != ""
	switch {
	case task && hasMacro:
		return false, fmt.Errorf("name either taskKey or projectId with macroKey, not both")
	case hasMacro && strings.TrimSpace(projectID) == "":
		return false, fmt.Errorf("a macro run needs projectId as well as macroKey")
	case !task && !hasMacro:
		return false, fmt.Errorf("taskKey, or projectId with macroKey, is required")
	}
	return hasMacro, nil
}

// skillReference names a skill without carrying its body. A launched session
// already holds the skill it is running; it opens the file when it needs another.
type skillReference struct {
	ID                     string `json:"id"`
	Directory              string `json:"directory"`
	Command                string `json:"command,omitempty"`
	RequiresReconciliation bool   `json:"requiresReconciliation,omitempty"`
}

// sessionContext projects the execution contract down to what a skill session
// consumes. It names no worktree setting, provider or model: those are the
// workstation's (#305), and the server cannot know what a session runs with. Inlining every skill and command body made this payload exceed what
// a session can read, which left the documented interface unusable.
func sessionContext(config *agentconfig.Config) map[string]any {
	if config == nil {
		return nil
	}
	skills := make([]skillReference, 0, len(config.Skills))
	for _, skill := range config.Skills {
		skills = append(skills, skillReference{ID: skill.ID, Directory: skill.Directory, Command: skill.Command, RequiresReconciliation: skill.RequiresReconciliation})
	}
	trackers := config.Trackers
	if trackers == nil {
		trackers = []agentconfig.TrackerRef{}
	}
	result := map[string]any{
		"schemaVersion": config.SchemaVersion, "projectId": config.ProjectID, "projectName": config.ProjectName,
		"description": config.Description, "gitRemoteUrl": config.GitRemoteURL, "issueTracker": config.IssueTracker,
		"trackerUrl": config.TrackerURL, "githubRepo": config.GithubRepo,
		"jiraProject": config.JiraProject, "specFramework": config.SpecFramework, "prCreationStage": config.PRCreationStage,
		"defaultSkillMode": config.DefaultSkillMode, "fullChainStopStage": config.FullChainStopStage, "pushStageCommits": config.PushStageCommits, "branchNameFormat": config.BranchNameFormat,
		"skills": skills, "skillDirectories": []string{".agents/skills", ".claude/skills", ".gemini/skills", ".agy/skills", ".skills"},
		// The trackers the project selects its tickets from and its label
		// (#741), and the tracker of the task the context was read for.
		"trackers": trackers, "label": config.Label,
	}
	if config.Tracker != nil {
		result["tracker"] = config.Tracker
	}
	return result
}

// englishError renders, in English, the store refusals this surface speaks
// about (#741): the store writes them in French for the web.
func englishError(err error) error {
	var ambiguous *db.ErrRunProjectAmbiguous
	switch {
	case err == nil:
		return nil
	case errors.As(err, &ambiguous):
		names := make([]string, 0, len(ambiguous.Candidates))
		for _, c := range ambiguous.Candidates {
			names = append(names, fmt.Sprintf("%s (projectId %s)", c.Name, c.ID))
		}
		if ambiguous.Unattended {
			return fmt.Errorf("unattended run refused: the task belongs to several projects: %s; launch it from one project", strings.Join(names, ", "))
		}
		return fmt.Errorf("the task belongs to several projects: %s; ask the user which one and pass its projectId", strings.Join(names, ", "))
	case errors.Is(err, db.ErrTaskKeyAmbiguous):
		return fmt.Errorf("this key names tasks of several trackers: give the task ID, or the projectId the task belongs to")
	case errors.Is(err, db.ErrTaskInNoProject):
		return fmt.Errorf("the task belongs to no project: give it a project's label before running it")
	case errors.Is(err, db.ErrRunProjectNotMember):
		return fmt.Errorf("the task does not belong to that project")
	case errors.Is(err, db.ErrTrackerNotInProject):
		return fmt.Errorf("unknown tracker: name one of the project's trackers by id or identity")
	}
	return err
}

// callProject is the project a tool call works in when it names none (#741):
// the project of the run the launched console names in its header, else of a
// run the session started. Empty when none recorded one.
func callProject(database *db.DB, sessions *SessionRegistry, req *mcp.CallToolRequest) string {
	if database == nil || req == nil {
		return ""
	}
	var runs []string
	if req.Extra != nil {
		if runID := strings.TrimSpace(req.Extra.Header.Get(agentprotocol.RunIDHeader)); runID != "" {
			runs = append(runs, runID)
		}
	}
	runs = append(runs, sessions.AdoptedRuns(sessionID(req.Session))...)
	for _, runID := range runs {
		if run, err := database.GetActivityByID(runID); err == nil && run != nil && run.RunProjectID != "" {
			return run.RunProjectID
		}
	}
	return ""
}

// Caller is the user behind an MCP call, resolved by the host from the bearer
// credential of the HTTP request that carried it.
type Caller struct {
	UserID string
	Name   string
	Role   string
	// Anonymous says the credential names no person, like the shared server
	// key: the call may read and report runs, never write to a tracker.
	Anonymous bool
}

// AnonymousWriteRefusal refuses a tracker write from a caller Sectile cannot
// tie to a person: signing it with the server account would make a user action
// anonymous on the tracker (#482). The REST handlers answer with it too.
const AnonymousWriteRefusal = "tracker write refused: this key is not tied to a user; pair the desktop app or use a personal API key"

// requireCaller refuses a write tool called by nobody in particular, before
// anything is written.
func requireCaller(caller Caller) error {
	if caller.Anonymous || strings.TrimSpace(caller.UserID) == "" {
		return fmt.Errorf("%s", AnonymousWriteRefusal)
	}
	return nil
}

// CallerResolver maps the headers of an MCP request to its caller. It returns
// false when the request names nobody, as over a transport without headers.
type CallerResolver func(header http.Header) (Caller, bool)

// callerOf resolves the caller of one tool call, or nobody when the host gave
// no resolver or the transport carried no headers.
func callerOf(resolve CallerResolver, req *mcp.CallToolRequest) Caller {
	if resolve == nil || req == nil || req.Extra == nil {
		return Caller{}
	}
	caller, _ := resolve(req.Extra.Header)
	return caller
}

// NewServer builds the tool catalog. The registry may be nil: session
// ownership is an addition to the catalog, never a precondition for serving it.
// Calls served this way name no caller; a host that can resolve one uses
// NewServerWithCallers.
func NewServer(database *db.DB, sessions *SessionRegistry) *mcp.Server {
	return NewServerWithCallers(database, sessions, nil)
}

// sessionIDFor makes the session ids of an instance: the instance id, a dot,
// then the SDK's own random part. Any instance that receives a request can then
// tell which one holds the session (see SessionOwner). Instance ids are UUIDs,
// so the id stays a header-safe token.
func sessionIDFor(instanceID string) func() string {
	return func() string { return instanceID + "." + rand.Text() }
}

// NewServerWithCallers is NewServer with the host's caller resolver, so a run,
// a transition or a comment records the user whose key made the call.
func NewServerWithCallers(database *db.DB, sessions *SessionRegistry, resolve CallerResolver) *mcp.Server {
	options := &mcp.ServerOptions{
		// A session begins when its client finishes initializing, which is the
		// first moment the server knows who connected.
		InitializedHandler: func(_ context.Context, req *mcp.InitializedRequest) {
			sessions.Watch(req.Session)
		},
	}
	if database != nil {
		options.GetSessionID = sessionIDFor(database.InstanceID())
	}
	s := mcp.NewServer(&mcp.Implementation{Name: "sectile", Version: "1.0.0"}, options)
	addMacroResourceTools(s, database, resolve)
	// Any message proves the client is alive, whichever tool or protocol method
	// it invoked, so activity is observed in the one place they all pass
	// through rather than tool by tool.
	s.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			if session := req.GetSession(); session != nil {
				sessions.Touch(session.ID())
				if resumesWaits(method, req) {
					sessions.Resume(session.ID())
					// A launched console names its run, which ends that run's
					// wait even from a session that declared nothing (#498).
					if call, ok := req.(*mcp.CallToolRequest); ok && call.Extra != nil {
						if runID := strings.TrimSpace(call.Extra.Header.Get(agentprotocol.RunIDHeader)); runID != "" {
							sessions.ResumeRun(runID, session.ID(), callerOf(resolve, call).UserID)
						}
					}
				}
			}
			result, err := next(ctx, method, req)
			// The store's refusals reach a session in English (#741).
			if call, ok := result.(*mcp.CallToolResult); ok && call != nil && call.IsError {
				if original := call.GetError(); original != nil {
					if rendered := englishError(original); rendered != original {
						call.Content = []mcp.Content{&mcp.TextContent{Text: rendered.Error()}}
						call.SetError(rendered)
					}
				}
			}
			return result, englishError(err)
		}
	})
	// taskRef resolves a task key within the project the call works in (#741),
	// to the task's id, so the store reads the one ticket the caller means: a
	// key two projects each carry names the one of the call's project. A key
	// nobody carries is returned as it is, for the store's own refusal.
	taskRef := func(req *mcp.CallToolRequest, projectID, key string) (string, error) {
		key = strings.TrimSpace(key)
		if key == "" {
			return key, nil
		}
		if strings.TrimSpace(projectID) == "" {
			projectID = callProject(database, sessions, req)
		}
		task, err := database.GetTaskByIDIn(projectID, key)
		if err != nil {
			return "", err
		}
		if task == nil {
			return key, nil
		}
		return task.ID, nil
	}
	mcp.AddTool(s, &mcp.Tool{Name: "get_task", Description: "Read task details, workflow labels, branch metadata and live comments. Comments come from the tracker; when they cannot be retrieved the task is still returned and the failure is reported in commentsError."},
		func(ctx context.Context, req *mcp.CallToolRequest, in taskInput) (*mcp.CallToolResult, any, error) {
			if strings.TrimSpace(in.TaskKey) == "" {
				return nil, nil, fmt.Errorf("taskKey is required")
			}
			ref, err := taskRef(req, "", in.TaskKey)
			if err != nil {
				return nil, nil, err
			}
			task, err := database.GetTaskByIDIn(callProject(database, sessions, req), ref)
			if err != nil {
				return nil, nil, err
			}
			if task == nil {
				return nil, nil, fmt.Errorf("task not found: %s", in.TaskKey)
			}
			// The task is already read. A tracker that cannot be reached costs the
			// session its comments, not its ticket, so the failure is reported as a
			// field and never as an empty discussion. Comments are read as the
			// caller, with their own tracker account, as the web detail view reads
			// them for its viewer.
			result := map[string]any{"task": task}
			comments, err := database.GetTaskCommentsAs(tracker.WithActingUser(ctx, callerOf(resolve, req).UserID), task.ID)
			if err != nil {
				result["commentsError"] = err.Error()
			} else {
				result["comments"] = comments
			}
			return nil, result, nil
		})
	mcp.AddTool(s, &mcp.Tool{Name: "transition_stage", Description: "Record a verified standalone workflow stage, optionally attach the task pull request or merge request URL using prUrl, and queue tracker synchronization. Managed runs must use their result contract.", InputSchema: map[string]any{
		"type": "object", "additionalProperties": false, "required": []string{"taskKey", "stage", "note"},
		"properties": map[string]any{
			"taskKey": map[string]any{"type": "string", "minLength": 1},
			"stage":   map[string]any{"type": "string", "enum": []string{"clarified", "specified", "implemented", "reviewed", "finished"}},
			"note":    map[string]any{"type": "string", "minLength": 1}, "branch": map[string]any{"type": "string"}, "prUrl": map[string]any{"type": "string", "description": "Pull request or merge request URL to persist on the task. Omit to preserve its existing link."},
			"prUrls":             map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "On a task that changed several repositories: the pull requests of the other repositories, one per repository. prUrl names the primary repository's."},
			"noRepositoryChange": map[string]any{"type": "boolean", "description": "True when the task's work changed no repository (a configuration made through an API, a review, a follow-up), so there is no pull request to give. Say in the note what was done instead. Refused with prUrl or prUrls, and when the task records a pull request on its branch or a changed repository."},
		},
	}}, func(ctx context.Context, req *mcp.CallToolRequest, in transitionInput) (*mcp.CallToolResult, any, error) {
		if strings.TrimSpace(in.Note) == "" {
			return nil, nil, fmt.Errorf("note is required")
		}
		caller := callerOf(resolve, req)
		if err := requireCaller(caller); err != nil {
			return nil, nil, err
		}
		var task *models.Task
		var activity *models.TaskActivity
		ref, err := taskRef(req, "", in.TaskKey)
		if err != nil {
			return nil, nil, err
		}
		in.TaskKey = ref
		if in.NoRepositoryChange {
			if strings.TrimSpace(in.PRURL) != "" || len(in.PRURLs) > 0 {
				return nil, nil, fmt.Errorf("noRepositoryChange states there is no pull request: give either prUrl/prUrls or noRepositoryChange, not both")
			}
			task, activity, err = database.TransitionTaskStageWithoutRepositoryChange(caller.UserID, in.TaskKey, in.Stage, in.Note, in.Branch)
		} else {
			task, activity, err = database.TransitionTaskStageWithPRs(caller.UserID, in.TaskKey, in.Stage, in.Note, append([]string{in.PRURL}, in.PRURLs...), in.Branch)
		}
		if err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{"task": task, "activity": activity, "trackerSync": "queued"}, nil
	})
	mcp.AddTool(s, &mcp.Tool{Name: "add_comment", Description: "Post a comment to the task's discussion on its tracker or local board."},
		func(ctx context.Context, req *mcp.CallToolRequest, in commentInput) (*mcp.CallToolResult, any, error) {
			if strings.TrimSpace(in.TaskKey) == "" || strings.TrimSpace(in.Body) == "" {
				return nil, nil, fmt.Errorf("taskKey and body are required")
			}
			caller := callerOf(resolve, req)
			if err := requireCaller(caller); err != nil {
				return nil, nil, err
			}
			ref, err := taskRef(req, "", in.TaskKey)
			if err != nil {
				return nil, nil, err
			}
			comments, err := database.PostTaskCommentBy(db.Actor{ID: caller.UserID, Name: caller.Name}, ref, in.Body)
			return nil, map[string]any{"comments": comments}, err
		})
	mcp.AddTool(s, &mcp.Tool{Name: "list_tasks", Description: "List board tasks, optionally filtered by project, status and sprint. A project lists the tickets it selects: those of its trackers carrying its label, every ticket of its trackers when it has none."},
		func(ctx context.Context, req *mcp.CallToolRequest, in listInput) (*mcp.CallToolResult, any, error) {
			tasks, err := database.GetTasks("", in.Status, "", "", in.ProjectID, in.Sprint, "", "", "", nil, nil, false)
			return nil, map[string]any{"tasks": tasks}, err
		})
	mcp.AddTool(s, &mcp.Tool{Name: "get_project_context", Description: "Read project description, repository identity, execution settings, specification framework, pull-request creation stage, default skill execution mode, full chain stop stage and skill references. Supply projectId or taskKey. Skill bodies are not inlined: open <skillDirectory>/<skill directory>/SKILL.md in the checkout, and read repository AGENTS.md for additional conventions."},
		func(ctx context.Context, req *mcp.CallToolRequest, in contextInput) (*mcp.CallToolResult, any, error) {
			projectID := in.ProjectID
			if strings.TrimSpace(projectID) == "" && strings.TrimSpace(in.TaskKey) != "" {
				projectID = callProject(database, sessions, req)
			}
			config, err := database.AgentConfig(projectID, in.TaskKey)
			if err != nil {
				return nil, nil, err
			}
			return nil, sessionContext(config), nil
		})
	mcp.AddTool(s, &mcp.Tool{Name: "list_projects", Description: "Discover projects and their primary keys, names and Git remote URLs. Use a project ID with get_project_context or list_tasks."},
		func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
			projects, err := database.AgentProjects()
			return nil, projects, err
		})
	mcp.AddTool(s, &mcp.Tool{Name: "start_run", Description: "Report the start of a remote skill execution so the task displays an active indicator. Save the returned activity ID as runId. Supply SECTILE_RUN_ID when provided by a launcher to reuse its run. Reads and transitions do not implicitly start or finish runs. A run this session creates is owned by it: if this client disconnects without finishing it, the server closes the run as canceled. A long silence does not: a quiet run stays open and is only remarked upon. A run reused from a launcher keeps the ownership of that launcher. For a batch, call it with each ticket's key and the batch runId when work on that ticket begins."},
		func(ctx context.Context, req *mcp.CallToolRequest, in startRunInput) (*mcp.CallToolResult, any, error) {
			macro, err := runTarget(in.TaskKey, in.ProjectID, in.MacroKey)
			if err != nil {
				return nil, nil, err
			}
			var activity *models.TaskActivity
			if macro {
				activity, err = database.StartMacroRunBy(callerOf(resolve, req).UserID, in.ProjectID, in.MacroKey, in.Skill, in.RunID)
			} else {
				// A task run works for one project (#741): the one named,
				// else the task's only one; a task of several is refused with
				// the candidates, so the agent asks its user.
				var ref string
				if ref, err = taskRef(req, in.ProjectID, in.TaskKey); err == nil {
					activity, err = database.StartRemoteRunFor(callerOf(resolve, req).UserID, ref, in.Skill, in.RunID, in.ProjectID)
				}
			}
			if err != nil {
				return nil, nil, err
			}
			// A session owns the runs it creates, and only those. A run reused
			// from a launcher belongs to the agent that dispatched it, whose
			// supervisor already reports the real process exit; closing it here
			// on disconnection would end an execution that is still going.
			if strings.TrimSpace(in.RunID) == "" {
				sessions.Adopt(sessionID(req.Session), activity.ID, in.TaskKey, in.Skill)
			}
			return nil, activity, nil
		})
	mcp.AddTool(s, &mcp.Tool{Name: "finish_run", Description: "Finish the specified remote execution with completed, failed or canceled status. Call when the entire invoked skill ends, including when stopping for user input. This is how a run reports its own outcome; a run left open when the session ends is closed as canceled instead, and such a run may still be reported here afterwards by its owner. Does not transition the task."},
		func(ctx context.Context, req *mcp.CallToolRequest, in finishRunInput) (*mcp.CallToolResult, any, error) {
			caller := callerOf(resolve, req)
			macro, err := runTarget(in.TaskKey, in.ProjectID, in.MacroKey)
			if err != nil {
				return nil, nil, err
			}
			actor := db.Actor{ID: caller.UserID, Name: caller.Name}
			var activity *models.TaskActivity
			if macro {
				activity, err = database.FinishMacroRunAs(actor, caller.Role == db.RoleAdmin, in.ProjectID, in.MacroKey, in.RunID, in.Status, in.Note)
			} else {
				var ref string
				if ref, err = taskRef(req, "", in.TaskKey); err == nil {
					activity, err = database.FinishRemoteRunAs(actor, caller.Role == db.RoleAdmin, ref, in.RunID, in.Status, in.Note)
				}
			}
			if err != nil {
				return nil, nil, err
			}
			sessions.Release(sessionID(req.Session), in.RunID)
			return nil, activity, nil
		})
	mcp.AddTool(s, &mcp.Tool{Name: reportWaitingTool, Description: "Declare that a run is blocked on its user, so the board and the owner's desktop show it as waiting. Call it with waiting true right before asking the user a question you cannot continue without. The wait ends by itself on this session's next Sectile call, when the owner presses Enter in the run's console, when the run finishes, or with waiting false. A headless run has nobody to answer and is left unmarked. Tool permission prompts are not reported this way. Name a task run with taskKey, a macro run with projectId and macroKey, as for start_run."},
		func(ctx context.Context, req *mcp.CallToolRequest, in reportWaitingInput) (*mcp.CallToolResult, any, error) {
			caller := callerOf(resolve, req)
			macro, err := runTarget(in.TaskKey, in.ProjectID, in.MacroKey)
			if err != nil {
				return nil, nil, err
			}
			// The wait is recorded with the declaring session, whose next call
			// ends it on whichever instance serves that call.
			actor, admin, session := db.Actor{ID: caller.UserID, Name: caller.Name}, caller.Role == db.RoleAdmin, sessionID(req.Session)
			var activity *models.TaskActivity
			var applied bool
			if macro {
				activity, applied, err = database.ReportSessionMacroRunWaitingAs(actor, admin, session, in.ProjectID, in.MacroKey, in.RunID, in.Waiting)
			} else {
				var ref string
				if ref, err = taskRef(req, "", in.TaskKey); err == nil {
					activity, applied, err = database.ReportSessionRunWaitingAs(actor, admin, session, ref, in.RunID, in.Waiting)
				}
			}
			if err != nil {
				return nil, nil, err
			}
			result := map[string]any{"activity": activity, "applied": applied}
			if !applied {
				result["reason"] = "headless run: nobody can answer it, so it is not shown as waiting"
			}
			return nil, result, nil
		})
	mcp.AddTool(s, &mcp.Tool{Name: "prepare_macro_worktree", Description: "Prepare the checkout a macro's specification is written in, on the caller's local agent, in the project's Macro specifications folder (the desktop \"Macro specifications folder\" setting, else the code checkout): on a Git folder, the macro's own worktree on the macro branch, created from the up-to-date default branch or reused as is; on a plain folder, the folder itself with an empty branch, where nothing is committed or pushed. Returns path, branch, whether it is a dedicated worktree, any warning, and the macro's slicing lines (todos) to align on. Call it before a macro skill reads or writes; it reuses the worktree a launch already prepared."},
		func(ctx context.Context, req *mcp.CallToolRequest, in macroWorktreeInput) (*mcp.CallToolResult, any, error) {
			workspace, err := database.PrepareMacroWorktree(ctx, callerOf(resolve, req).UserID, in.ProjectID, in.MacroKey)
			if err != nil {
				return nil, nil, err
			}
			return nil, workspace, nil
		})
	mcp.AddTool(s, &mcp.Tool{Name: "get_macro", Description: "Read one macro (epic) of a project: title, description, framing comment, horizon, its todos in the order of execution (id, text, done, storyKey, target, origin) todosMirror, the status of their one-way copy on the tracker, and framingMirror, the status of the framing's one-way copy on a Jira epic. Writes nothing."},
		func(ctx context.Context, req *mcp.CallToolRequest, in macroInput) (*mcp.CallToolResult, any, error) {
			macro, err := database.GetMacro(in.ProjectID, in.MacroKey)
			if err != nil {
				return nil, nil, err
			}
			return nil, map[string]any{"macro": macro}, nil
		})
	mcp.AddTool(s, &mcp.Tool{Name: "update_macro_todos", Description: "Save the full ordered todo list of a macro, top first. A todo with the id of an existing one keeps its story key and origin and takes the given text, done and target; a todo without id is created; an existing todo the list omits is removed, except one linked to a story, which refuses the call: only the macro's panel removes it. A blank text, an unknown id or a repeated id refuses the whole call and saves nothing. Story keys cannot be set here. Save only a list the owner confirmed. Answers with the saved macro and todosMirror; the tracker copy is written shortly after."},
		func(ctx context.Context, req *mcp.CallToolRequest, in macroTodosInput) (*mcp.CallToolResult, any, error) {
			caller := callerOf(resolve, req)
			if err := requireCaller(caller); err != nil {
				return nil, nil, err
			}
			items := make([]db.MacroTodoInput, 0, len(in.Todos))
			for _, todo := range in.Todos {
				items = append(items, db.MacroTodoInput{ID: todo.ID, Text: todo.Text, Done: todo.Done,
					TargetProjectID: todo.TargetProjectID, TargetTrackerProject: todo.TargetTrackerProject})
			}
			macro, err := database.ReplaceMacroTodos(tracker.WithActingUser(ctx, caller.UserID), in.ProjectID, in.MacroKey, items)
			if err != nil {
				return nil, nil, err
			}
			return nil, map[string]any{"macro": macro, "todosMirror": macro.TodosMirror}, nil
		})
	mcp.AddTool(s, &mcp.Tool{Name: "prepare_repository_worktree", Description: "Prepare the task's worktree in another repository than its primary one, on the caller's local agent, on the task's branch: reused wherever that branch is already checked out, else created from the remote branch when it exists, else from the checkout's current HEAD. The repository is one of the project's repositories, or a Git folder attached to the project on the caller's workstation, given by its remote URL or host/path; it must have a folder on that workstation. Call it before changing a context folder (SECTILE_REPOSITORIES role \"context\"): context folders are read-only. A local folder (role \"local\", no remote) is changed in place without it, with no worktree and no pull request. The repository then needs its own pull request, given in transition_stage prUrls. The workstation's project settings are read at each call, so a folder attached after the session started is accepted; SECTILE_REPOSITORIES lists the folders known when the run was launched and is not refreshed. When the workstation has the project's Any repository option on, a repository no folder holds is accepted too: give path, the top level of a local checkout whose origin is that repository, or omit it and the agent clones the repository into the project's clones folder; either is then remembered on that workstation. A branch that exists neither locally nor on origin starts from the remote default branch, fetched first. Returns repository, path, branch, source (mapping, project, attached, path or clone), whether the checkout was remembered, whether the worktree was added to the running session (when false, run /add-dir with the path before writing there), and any warning."},
		func(ctx context.Context, req *mcp.CallToolRequest, in repositoryWorktreeInput) (*mcp.CallToolResult, any, error) {
			ref, err := taskRef(req, "", in.TaskKey)
			if err != nil {
				return nil, nil, err
			}
			worktree, err := database.PrepareRepositoryWorktree(ctx, callerOf(resolve, req).UserID, ref, in.Repository, in.Path)
			if err != nil {
				return nil, nil, err
			}
			return nil, worktree, nil
		})
	mcp.AddTool(s, &mcp.Tool{Name: "prepare_task_spec_worktree", Description: "Prepare where a task's clarification report and specification are written, on the caller's local agent, in the project's Issue specifications folder (the desktop \"Issue specifications folder\" setting, else the code checkout). When that folder is the code checkout, returns the task's own worktree and branch, with distinct false: write there as before. Otherwise, on a Git folder, the task's own worktree of it on a branch named like the task's branch, created from the up-to-date default branch (or the remote branch when it exists) or reused as is; on a plain folder, the folder itself with an empty branch, where nothing is committed or pushed. Returns repository (the Issue folder), path, branch, whether it is a dedicated worktree, distinct, and any warning. An issue skill calls it when SECTILE_SPEC_REPO is not set; it reuses the worktree a launch already prepared."},
		func(ctx context.Context, req *mcp.CallToolRequest, in taskInput) (*mcp.CallToolResult, any, error) {
			workspace, err := database.PrepareTaskSpecWorktree(ctx, callerOf(resolve, req).UserID, in.TaskKey)
			if err != nil {
				return nil, nil, err
			}
			return nil, workspace, nil
		})
	mcp.AddTool(s, &mcp.Tool{Name: "record_pull_request", Description: "Record a pull request or merge request on a task without changing its stage, for a pull request opened outside a stage transition (create-pr, or a secondary repository pushed after its stage). Call it once per repository. The URL must name a pull request in one of the task's repositories (its primary one, a project repository, or one changed through prepare_repository_worktree); a pull request on a branch unrelated to the ones already recorded for that repository is refused. The primary repository's pull request stays the task's prUrl. Returns the task and its prLinks, each naming its repository."},
		func(ctx context.Context, req *mcp.CallToolRequest, in recordPullRequestInput) (*mcp.CallToolResult, any, error) {
			if strings.TrimSpace(in.TaskKey) == "" || strings.TrimSpace(in.URL) == "" {
				return nil, nil, fmt.Errorf("taskKey and url are required")
			}
			caller := callerOf(resolve, req)
			if err := requireCaller(caller); err != nil {
				return nil, nil, err
			}
			ref, err := taskRef(req, "", in.TaskKey)
			if err != nil {
				return nil, nil, err
			}
			task, err := database.RecordPullRequest(ctx, caller.UserID, ref, in.URL)
			if err != nil {
				return nil, nil, err
			}
			return nil, map[string]any{"task": task, "prLinks": task.PrLinks}, nil
		})
	mcp.AddTool(s, &mcp.Tool{Name: "create_task", Description: "Create a task on an explicitly named project and return it with its key and external URL. Creation is remote whenever the project's tracker supports it, and fails rather than leaving a ticket that exists only on the local board. The new task enters the workflow at its first stage; it cannot be created at a later one.", InputSchema: map[string]any{
		"type": "object", "additionalProperties": false, "required": []string{"projectId", "title"},
		"properties": map[string]any{
			"projectId":   map[string]any{"type": "string", "minLength": 1, "description": "Project primary key from list_projects. Required and never inferred: a bare task key can name another project's ticket."},
			"tracker":     map[string]any{"type": "string", "description": "One of the project's trackers, by id or identity, from get_project_context trackers. Omit for the project's default tracker."},
			"title":       map[string]any{"type": "string", "minLength": 1},
			"description": map[string]any{"type": "string"},
			"issueType":   map[string]any{"type": "string", "description": "Project issue type, for example Task or Story."},
			"priority":    map[string]any{"type": "string", "enum": []string{"low", "medium", "high", "urgent"}},
			"labels":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Custom labels. The workflow label is assigned by the board."},
			"parentKey":   map[string]any{"type": "string", "description": "Key of the parent macro or epic."},
		},
	}}, func(ctx context.Context, req *mcp.CallToolRequest, in createTaskInput) (*mcp.CallToolResult, any, error) {
		caller := callerOf(resolve, req)
		if err := requireCaller(caller); err != nil {
			return nil, nil, err
		}
		projectID := strings.TrimSpace(in.ProjectID)
		if projectID == "" {
			return nil, nil, fmt.Errorf("projectId is required: name the project explicitly, list_projects reports the available primary keys")
		}
		if strings.TrimSpace(in.Title) == "" {
			return nil, nil, fmt.Errorf("title is required")
		}
		// CreateTask falls back to the first project when the identifier does not
		// resolve, which would file the ticket on someone else's board without
		// saying so. A session that names a project must get that project.
		project, err := database.GetProjectByID(projectID)
		if err != nil {
			return nil, nil, err
		}
		if project == nil {
			return nil, nil, fmt.Errorf("project not found: %s", projectID)
		}
		trackerID := ""
		if named := strings.TrimSpace(in.Tracker); named != "" {
			trk, err := database.ProjectTrackerNamed(project.ID, named)
			if err != nil {
				return nil, nil, fmt.Errorf("unknown tracker %s for project %s: name one of its trackers by id or identity", named, project.ID)
			}
			trackerID = trk.ID
		}
		// The issue is created as the caller, exactly as from the web: under
		// their own tracker credential, never the server's.
		task, err := database.CreateTaskAs(tracker.WithActingUser(ctx, caller.UserID), models.CreateTaskRequest{
			// Remote creation is required, not preferred: a ticket an agent files
			// has to exist where a human will see it.
			RequireRemoteCreation: true,
			ProjectID:             project.ID,
			TrackerID:             trackerID,
			Title:                 strings.TrimSpace(in.Title),
			Description:           in.Description,
			Priority:              models.Priority(strings.TrimSpace(in.Priority)),
			Labels:                in.Labels,
			IssueType:             strings.TrimSpace(in.IssueType),
			ParentKey:             strings.TrimSpace(in.ParentKey),
		})
		if err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{"task": task}, nil
	})
	mcp.AddTool(s, &mcp.Tool{Name: "update_task", Description: "Update mutable descriptive fields of an existing task (title, description, priority, issueType, labels). Disallows modifying workflow status, stage, branch or pull requests. Synchronizes updates to external trackers when supported.", InputSchema: map[string]any{
		"type": "object", "additionalProperties": false, "required": []string{"taskKey"},
		"properties": map[string]any{
			"taskKey":     map[string]any{"type": "string", "minLength": 1, "description": "Task key (e.g. #241) or full ID."},
			"title":       map[string]any{"type": "string", "minLength": 1, "description": "New task title."},
			"description": map[string]any{"type": "string", "description": "New task description. Provide empty string to clear."},
			"priority":    map[string]any{"type": "string", "enum": []string{"low", "medium", "high", "urgent"}, "description": "Task priority."},
			"issueType":   map[string]any{"type": "string", "description": "Project issue type, for example Task or Bug."},
			"labels":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Custom labels. The task's workflow stage label is preserved."},
		},
	}}, func(ctx context.Context, req *mcp.CallToolRequest, in updateTaskInput) (*mcp.CallToolResult, any, error) {
		caller := callerOf(resolve, req)
		if err := requireCaller(caller); err != nil {
			return nil, nil, err
		}
		taskKey := strings.TrimSpace(in.TaskKey)
		if taskKey == "" {
			return nil, nil, fmt.Errorf("taskKey is required")
		}
		if in.Title == nil && in.Description == nil && in.Priority == nil && in.IssueType == nil && in.Labels == nil {
			return nil, nil, fmt.Errorf("at least one mutable field (title, description, priority, issueType, labels) must be provided")
		}
		if in.Title != nil && strings.TrimSpace(*in.Title) == "" {
			return nil, nil, fmt.Errorf("title cannot be empty")
		}
		existing, err := database.GetTaskByIDIn(callProject(database, sessions, req), taskKey)
		if err != nil {
			return nil, nil, err
		}
		if existing == nil {
			return nil, nil, fmt.Errorf("task not found: %s", taskKey)
		}

		var title *string
		if in.Title != nil {
			t := strings.TrimSpace(*in.Title)
			title = &t
		}
		var desc *string
		if in.Description != nil {
			desc = in.Description
		}
		var prio *models.Priority
		if in.Priority != nil {
			p := models.Priority(strings.TrimSpace(*in.Priority))
			prio = &p
		}
		var issueType *string
		if in.IssueType != nil {
			it := strings.TrimSpace(*in.IssueType)
			issueType = &it
		}
		var labels *[]string
		if in.Labels != nil {
			currentStage := ""
			for _, l := range existing.Labels {
				clean := strings.ToLower(strings.TrimLeft(strings.TrimSpace(l), "#"))
				for _, wl := range []string{"untouched", "new", "clarified", "specified", "implemented", "reviewed", "finished", "closed"} {
					if clean == wl {
						currentStage = clean
						break
					}
				}
				if currentStage != "" {
					break
				}
			}
			if currentStage == "" {
				currentStage = db.GetStageLabelForStatus(existing.Status)
			}
			sanitized := db.SetWorkflowLabel(*in.Labels, "#"+currentStage)
			labels = &sanitized
		}

		actor := db.Actor{ID: caller.UserID, Name: caller.Name}
		task, err := database.UpdateTaskBy(actor, existing.ID, models.UpdateTaskRequest{
			Title:       title,
			Description: desc,
			Priority:    prio,
			IssueType:   issueType,
			Labels:      labels,
		})
		if err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{"task": task}, nil
	})
	return s
}
