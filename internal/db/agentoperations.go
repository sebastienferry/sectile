package db

import (
	"context"
	"encoding/json"
	"fmt"
	"tasks/internal/agentprotocol"
	"tasks/internal/models"
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

func operationTimeout(action string) time.Duration {
	if action == "run_prompt" {
		return 12 * time.Minute
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

func (d *DB) TrackerGraphQL(query string) ([]byte, error) { return d.trackers.GithubGraphQL(query) }
