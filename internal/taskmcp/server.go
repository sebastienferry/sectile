// Package taskmcp exposes Sectile's existing workflow services as typed MCP tools.
package taskmcp

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"tasks/internal/agentconfig"
	"tasks/internal/db"
	"tasks/internal/models"
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
// fields a caller could use to contradict the board's own invariants — status,
// source, external URL — are deliberately absent: a task created here enters the
// workflow where every other new task enters it.
type createTaskInput struct {
	ProjectID   string   `json:"projectId"`
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
	// ProjectID and MacroKey name a macro skill run instead of a task run.
	ProjectID string `json:"projectId,omitempty" jsonschema:"project primary key of a macro run"`
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
type macroWorktreeInput struct {
	ProjectID string `json:"projectId" jsonschema:"project primary key"`
	MacroKey  string `json:"macroKey" jsonschema:"macro key, for example M-7"`
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
// consumes. Inlining every skill and command body made this payload exceed what
// a session can read, which left the documented interface unusable.
func sessionContext(config *agentconfig.Config) map[string]any {
	if config == nil {
		return nil
	}
	skills := make([]skillReference, 0, len(config.Skills))
	for _, skill := range config.Skills {
		skills = append(skills, skillReference{ID: skill.ID, Directory: skill.Directory, Command: skill.Command, RequiresReconciliation: skill.RequiresReconciliation})
	}
	return map[string]any{
		"schemaVersion": config.SchemaVersion, "projectId": config.ProjectID, "projectName": config.ProjectName,
		"description": config.Description, "gitRemoteUrl": config.GitRemoteURL, "issueTracker": config.IssueTracker,
		"trackerUrl": config.TrackerURL, "githubRepo": config.GithubRepo,
		"jiraProject": config.JiraProject, "specFramework": config.SpecFramework, "prCreationStage": config.PRCreationStage,
		"defaultSkillMode": config.DefaultSkillMode, "fullChainStopStage": config.FullChainStopStage,
		"useWorktrees": config.UseWorktrees, "aiProvider": config.AIProvider, "aiModel": config.AIModel,
		"skills": skills, "skillDirectories": []string{".agents/skills", ".claude/skills", ".gemini/skills", ".agy/skills", ".skills"},
	}
}

// Caller is the user behind an MCP call, resolved by the host from the bearer
// credential of the HTTP request that carried it.
type Caller struct {
	UserID string
	Name   string
	Role   string
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

// NewServerWithCallers is NewServer with the host's caller resolver, so a run,
// a transition or a comment records the user whose key made the call.
func NewServerWithCallers(database *db.DB, sessions *SessionRegistry, resolve CallerResolver) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "sectile", Version: "1.0.0"}, &mcp.ServerOptions{
		// A session begins when its client finishes initializing, which is the
		// first moment the server knows who connected.
		InitializedHandler: func(_ context.Context, req *mcp.InitializedRequest) {
			sessions.Watch(req.Session)
		},
	})
	// Any message proves the client is alive, whichever tool or protocol method
	// it invoked, so activity is observed in the one place they all pass
	// through rather than tool by tool.
	s.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			if session := req.GetSession(); session != nil {
				sessions.Touch(session.ID())
			}
			return next(ctx, method, req)
		}
	})
	mcp.AddTool(s, &mcp.Tool{Name: "get_task", Description: "Read task details, workflow labels, branch metadata and live comments. Comments come from the tracker; when they cannot be retrieved the task is still returned and the failure is reported in commentsError."},
		func(ctx context.Context, req *mcp.CallToolRequest, in taskInput) (*mcp.CallToolResult, any, error) {
			if strings.TrimSpace(in.TaskKey) == "" {
				return nil, nil, fmt.Errorf("taskKey is required")
			}
			task, err := database.GetTaskByID(in.TaskKey)
			if err != nil {
				return nil, nil, err
			}
			if task == nil {
				return nil, nil, fmt.Errorf("task not found: %s", in.TaskKey)
			}
			// The task is already read. A tracker that cannot be reached costs the
			// session its comments, not its ticket, so the failure is reported as a
			// field and never as an empty discussion.
			result := map[string]any{"task": task}
			comments, err := database.GetTaskComments(task.ID)
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
		},
	}}, func(ctx context.Context, req *mcp.CallToolRequest, in transitionInput) (*mcp.CallToolResult, any, error) {
		if strings.TrimSpace(in.Note) == "" {
			return nil, nil, fmt.Errorf("note is required")
		}
		task, activity, err := database.TransitionTaskStageBy(callerOf(resolve, req).UserID, in.TaskKey, in.Stage, in.Note, in.PRURL, in.Branch)
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
			comments, err := database.PostTaskCommentBy(db.Actor{ID: caller.UserID, Name: caller.Name}, in.TaskKey, in.Body)
			return nil, map[string]any{"comments": comments}, err
		})
	mcp.AddTool(s, &mcp.Tool{Name: "list_tasks", Description: "List board tasks, optionally filtered by project, status and sprint."},
		func(ctx context.Context, req *mcp.CallToolRequest, in listInput) (*mcp.CallToolResult, any, error) {
			tasks, err := database.GetTasks("", in.Status, "", "", in.ProjectID, in.Sprint, "", "", "", nil, nil, false)
			return nil, map[string]any{"tasks": tasks}, err
		})
	mcp.AddTool(s, &mcp.Tool{Name: "get_project_context", Description: "Read project description, repository identity, execution settings, specification framework, pull-request creation stage, default skill execution mode, full chain stop stage and skill references. Supply projectId or taskKey. Skill bodies are not inlined: open <skillDirectory>/<skill directory>/SKILL.md in the checkout, and read repository AGENTS.md for additional conventions."},
		func(ctx context.Context, req *mcp.CallToolRequest, in contextInput) (*mcp.CallToolResult, any, error) {
			config, err := database.AgentConfig(in.ProjectID, in.TaskKey)
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
	mcp.AddTool(s, &mcp.Tool{Name: "start_run", Description: "Report the start of a remote skill execution so the task displays an active indicator. Save the returned activity ID as runId. Supply SECTILE_RUN_ID when provided by a launcher to reuse its run. Reads and transitions do not implicitly start or finish runs. A run this session creates is owned by it: if this client disconnects without finishing it, the server closes the run as canceled. A long silence does not: a quiet run stays open and is only remarked upon. A run reused from a launcher keeps the ownership of that launcher."},
		func(ctx context.Context, req *mcp.CallToolRequest, in startRunInput) (*mcp.CallToolResult, any, error) {
			macro, err := runTarget(in.TaskKey, in.ProjectID, in.MacroKey)
			if err != nil {
				return nil, nil, err
			}
			var activity *models.TaskActivity
			if macro {
				activity, err = database.StartMacroRunBy(callerOf(resolve, req).UserID, in.ProjectID, in.MacroKey, in.Skill, in.RunID)
			} else {
				activity, err = database.StartRemoteRunBy(callerOf(resolve, req).UserID, in.TaskKey, in.Skill, in.RunID)
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
				activity, err = database.FinishRemoteRunAs(actor, caller.Role == db.RoleAdmin, in.TaskKey, in.RunID, in.Status, in.Note)
			}
			if err != nil {
				return nil, nil, err
			}
			sessions.Release(sessionID(req.Session), in.RunID)
			return nil, activity, nil
		})
	mcp.AddTool(s, &mcp.Tool{Name: "prepare_macro_worktree", Description: "Prepare the checkout a macro's specification is written in, on the caller's local agent: the macro's own worktree in the project's specifications repository, on the macro branch, created from the up-to-date default branch or reused as is. Returns path, branch, whether it is a dedicated worktree, and any warning. Call it before a macro skill writes, unless SECTILE_SPEC_REPO already names the checkout."},
		func(ctx context.Context, req *mcp.CallToolRequest, in macroWorktreeInput) (*mcp.CallToolResult, any, error) {
			workspace, err := database.PrepareMacroWorktree(ctx, callerOf(resolve, req).UserID, in.ProjectID, in.MacroKey)
			if err != nil {
				return nil, nil, err
			}
			return nil, workspace, nil
		})
	mcp.AddTool(s, &mcp.Tool{Name: "create_task", Description: "Create a task on an explicitly named project and return it with its key and external URL. Creation is remote whenever the project's tracker supports it, and fails rather than leaving a ticket that exists only on the local board. The new task enters the workflow at its first stage; it cannot be created at a later one.", InputSchema: map[string]any{
		"type": "object", "additionalProperties": false, "required": []string{"projectId", "title"},
		"properties": map[string]any{
			"projectId":   map[string]any{"type": "string", "minLength": 1, "description": "Project primary key from list_projects. Required and never inferred: a bare task key can name another project's ticket."},
			"title":       map[string]any{"type": "string", "minLength": 1},
			"description": map[string]any{"type": "string"},
			"issueType":   map[string]any{"type": "string", "description": "Project issue type, for example Task or Story."},
			"priority":    map[string]any{"type": "string", "enum": []string{"low", "medium", "high", "urgent"}},
			"labels":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Custom labels. The workflow label is assigned by the board."},
			"parentKey":   map[string]any{"type": "string", "description": "Key of the parent macro or epic."},
		},
	}}, func(ctx context.Context, req *mcp.CallToolRequest, in createTaskInput) (*mcp.CallToolResult, any, error) {
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
		task, err := database.CreateTask(models.CreateTaskRequest{
			// Remote creation is required, not preferred: a ticket an agent files
			// has to exist where a human will see it.
			RequireRemoteCreation: true,
			ProjectID:             project.ID,
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
		existing, err := database.GetTaskByID(taskKey)
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

		caller := callerOf(resolve, req)
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
