package db

import (
	"context"
	"encoding/json"
	"fmt"
	"tasks/internal/agentprotocol"
	"tasks/internal/models"
	"tasks/internal/tracker"
	"time"
)

type AgentOperations func(context.Context, agentprotocol.Operation) (json.RawMessage, error)

func (d *DB) SetAgentOperations(call AgentOperations) { d.agentOperations = call }
func (d *DB) callAgent(op agentprotocol.Operation, result any) error {
	return d.callAgentContext(context.Background(), op, result)
}

func (d *DB) callAgentContext(parent context.Context, op agentprotocol.Operation, result any) error {
	if d.agentOperations == nil {
		return fmt.Errorf("local operation requires a connected agent")
	}
	if op.ProjectID == "" || op.ProjectID == "all" {
		op.ProjectID = ""
		projects, err := d.GetProjects()
		if err != nil {
			return err
		}
		for _, p := range projects {
			if p.IsDefault {
				op.ProjectID = p.ID
				break
			}
		}
		if op.ProjectID == "" && len(projects) == 1 {
			op.ProjectID = projects[0].ID
		}
	}
	if op.ProjectID != "" {
		if p, _ := d.GetProjectByID(op.ProjectID); p == nil {
			projects, err := d.GetProjects()
			if err != nil {
				return err
			}
			found := ""
			for _, p := range projects {
				if p.RepoPath == op.ProjectID {
					if found != "" {
						return fmt.Errorf("repository path is ambiguous; use the project primary key")
					}
					found = p.ID
				}
			}
			if found == "" {
				return fmt.Errorf("unknown project; use its primary key")
			}
			op.ProjectID = found
		}
	}
	if op.ProjectID == "" {
		return fmt.Errorf("select a project for local execution")
	}
	ctx, cancel := context.WithTimeout(parent, operationTimeout(op.Action))
	defer cancel()
	raw, err := d.agentOperations(ctx, op)
	if err != nil {
		return err
	}
	if result != nil {
		return json.Unmarshal(raw, result)
	}
	return nil
}

// localInspections are read-only lookups the agent answers from git plumbing
// or the filesystem alone. They finish in milliseconds on a healthy agent, so
// they get a budget that leaves ample room for a slow repository while still
// failing fast when the agent is gone. Operations that may reach the network —
// cloning, fetching, deleting a remote branch — keep the longer default.
var localInspections = map[string]time.Duration{
	"git_evidence":   15 * time.Second,
	"git_status":     15 * time.Second,
	"git_branches":   15 * time.Second,
	"workspace_info": 15 * time.Second,
	"spec_status":    15 * time.Second,
	"skills_status":  15 * time.Second,
	"skill_files":    15 * time.Second,
	"read_skill":     15 * time.Second,
	"open_editor":    15 * time.Second,
	"cli_status":     30 * time.Second,
}

func operationTimeout(action string) time.Duration {
	if action == "spec_install" {
		return 7 * time.Minute
	}
	// Reading a marketplace clones or fetches a repository, so it gets the
	// network budget rather than the read-only one; three minutes is ample for
	// a blobless clone and still fails while someone is watching.
	if action == "marketplace_catalog" || action == "marketplace_pack" {
		return 3 * time.Minute
	}
	if action == "run_prompt" {
		return 12 * time.Minute
	}
	if d, ok := localInspections[action]; ok {
		return d
	}
	return 45 * time.Second
}

func (d *DB) EnsureTaskGitBranch(repoPath string, task *models.Task) (string, error) {
	_, branch, err := d.EnsureTaskWorktree(repoPath, task)
	return branch, err
}

func (d *DB) AgentOperation(op agentprotocol.Operation, result any) error {
	return d.callAgent(op, result)
}

// TrackerGraphQL runs one GitHub GraphQL query. It names no project on purpose:
// the caller is reading metadata for a project that does not exist yet. The
// context's acting user is therefore the only credential it can carry beyond
// the server's own, so it is read rather than dropped.
func (d *DB) TrackerGraphQL(ctx context.Context, query string) ([]byte, error) {
	return d.trackerAs(tracker.ActingUser(ctx), "github", "").GithubGraphQL(query)
}
