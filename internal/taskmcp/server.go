// Package taskmcp exposes Sectile's existing workflow services as typed MCP tools.
package taskmcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"tasks/internal/db"
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

type startRunInput struct {
	TaskKey string `json:"taskKey"`
	Skill   string `json:"skill"`
	RunID   string `json:"runId,omitempty"`
}
type finishRunInput struct {
	TaskKey string `json:"taskKey"`
	RunID   string `json:"runId"`
	Status  string `json:"status" jsonschema:"completed, failed or canceled"`
	Note    string `json:"note"`
}

func NewServer(database *db.DB) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "sectile", Version: "1.0.0"}, nil)
	mcp.AddTool(s, &mcp.Tool{Name: "get_task", Description: "Read task details, workflow labels, branch metadata and live comments."},
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
			comments, err := database.GetTaskComments(task.ID)
			if err != nil {
				return nil, nil, err
			}
			return nil, map[string]any{"task": task, "comments": comments}, nil
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
		task, activity, err := database.TransitionTaskStage(in.TaskKey, in.Stage, in.Note, in.PRURL, in.Branch)
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
			comments, err := database.PostTaskComment(in.TaskKey, in.Body)
			return nil, map[string]any{"comments": comments}, err
		})
	mcp.AddTool(s, &mcp.Tool{Name: "list_tasks", Description: "List board tasks, optionally filtered by project, status and sprint."},
		func(ctx context.Context, req *mcp.CallToolRequest, in listInput) (*mcp.CallToolResult, any, error) {
			tasks, err := database.GetTasks("", in.Status, "", "", in.ProjectID, in.Sprint, "", "", "", nil, nil, false)
			return nil, map[string]any{"tasks": tasks}, err
		})
	mcp.AddTool(s, &mcp.Tool{Name: "get_project_context", Description: "Read project description, repository identity, execution settings, specification framework and skill instructions. Supply projectId or taskKey; read repository AGENTS.md locally for additional conventions."},
		func(ctx context.Context, req *mcp.CallToolRequest, in contextInput) (*mcp.CallToolResult, any, error) {
			config, err := database.AgentConfig(in.ProjectID, in.TaskKey)
			return nil, config, err
		})
	mcp.AddTool(s, &mcp.Tool{Name: "list_projects", Description: "Discover projects and their primary keys, names and Git remote URLs. Use a project ID with get_project_context or list_tasks."},
		func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
			projects, err := database.AgentProjects()
			return nil, projects, err
		})
	mcp.AddTool(s, &mcp.Tool{Name: "start_run", Description: "Report the start of a remote skill execution so the task displays an active indicator. Save the returned activity ID as runId. Supply TASKFLOW_RUN_ID when provided by a launcher to reuse its run. Reads and transitions do not implicitly start or finish runs."},
		func(ctx context.Context, req *mcp.CallToolRequest, in startRunInput) (*mcp.CallToolResult, any, error) {
			activity, err := database.StartRemoteRun(in.TaskKey, in.Skill, in.RunID)
			return nil, activity, err
		})
	mcp.AddTool(s, &mcp.Tool{Name: "finish_run", Description: "Finish the specified remote execution with completed, failed or canceled status. Call when the entire invoked skill ends, including when stopping for user input. Does not transition the task."},
		func(ctx context.Context, req *mcp.CallToolRequest, in finishRunInput) (*mcp.CallToolResult, any, error) {
			activity, err := database.FinishRemoteRun(in.TaskKey, in.RunID, in.Status, in.Note)
			return nil, activity, err
		})
	return s
}
