package db

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"tasks/internal/agentprotocol"
	"tasks/internal/models"
)

// ActiveRunsCountForProject returns the number of tasks with active runs or in-flight launches for a given project.
func (d *DB) ActiveRunsCountForProject(projectID string) (int, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	project, err := d.getProjectByIDUnsafe(projectID)
	if err != nil || project == nil {
		return 0, fmt.Errorf("project not found: %s", projectID)
	}

	return activeRunsCountForProjectOn(d.conn, project)
}

func activeRunsCountForProjectOn(q rowQuerier, project *models.Project) (int, error) {
	query := `
  SELECT COUNT(DISTINCT a.task_id)
  FROM task_activities a
  WHERE (a.run_project_id IN (?, ?) OR (COALESCE(a.run_project_id, '') = '' AND a.task_id IN (SELECT id FROM tasks WHERE project_id IN (?, ?))))
    AND a.status IN ('queued', 'pending', 'running')
    AND a.skill_id IN ('remote_run', 'agent_launch', 'clarify', 'specify', 'implement', 'adjust', 'handoff', 'create_pr', 'pickup', 'rewrite_story', 'refine_macro', 'pickup_issues', 'review', 'pick', 'realign_macro')`
	var count int
	err := q.QueryRow(query, project.ID, project.Slug, project.ID, project.Slug).Scan(&count)
	return count, err
}

var errOrchestratorCapacity = errors.New("max concurrency limit reached")

// admitOrchestratedActivity checks capacity and inserts the queued reservation in
// one transaction. The project row lock serializes PostgreSQL server instances;
// SQLite takes its write lock at Begin. The caller holds d.mu, and only queues
// the job after this transaction commits.
func (d *DB) admitOrchestratedActivity(act models.TaskActivity, maxConcurrent int) error {
	return d.conn.WithTx(func(tx *sqlTx) error {
		project := &models.Project{}
		if err := tx.QueryRow("SELECT id, slug FROM projects WHERE id = ?"+d.dialect.ForUpdate(), act.RunProjectID).Scan(&project.ID, &project.Slug); err != nil {
			return err
		}
		count, err := activeRunsCountForProjectOn(tx, project)
		if err != nil {
			return err
		}
		if count >= maxConcurrent {
			return errOrchestratorCapacity
		}
		return insertTaskActivity(tx, d.instanceID, act)
	})
}

// orchestratorLaunchPrompt makes the agent assess natural-language guidelines
// before it changes a ticket. The server's deterministic checks cannot decide
// whether a ticket satisfies arbitrary project instructions. Keeping this prompt
// on every step also prevents a later step from dropping the restrictions.
func orchestratorLaunchPrompt(guidelines string) string {
	prompt := "Backlog orchestration: before making changes or advancing the workflow, assess whether this ticket has clear actionable scope, sufficient technical context, and no blocking dependencies or unresolved critical decisions. If it is not eligible, explain why and end the run without changing the ticket or advancing its stage. The chain must stop at implemented; review and merge require a human."
	if guidelines = strings.TrimSpace(guidelines); guidelines != "" {
		prompt += "\nProject eligibility guidelines (mandatory):\n" + guidelines + "\nIf this ticket does not satisfy these guidelines, explain the unmet criteria and end the run without making changes or advancing its stage."
	}
	return prompt
}

// EvaluateTaskEligibility applies the deterministic backlog filters. The agent
// assesses technical readiness and project guidelines before doing any work.
func (d *DB) EvaluateTaskEligibility(task *models.Task) (bool, string) {
	if task == nil {
		return false, "task is nil"
	}
	if strings.TrimSpace(task.Title) == "" || len(strings.TrimSpace(task.Title)) < 3 {
		return false, "title is too short or empty"
	}
	desc := strings.TrimSpace(task.Description)
	if desc == "" || len(desc) < 10 {
		return false, "description is too short or lacks actionable technical context"
	}
	for _, label := range task.Labels {
		norm := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(label), "#")))
		if norm == "blocked" || norm == "hold" || norm == "on-hold" || norm == "needs-info" || norm == "manual" {
			return false, fmt.Sprintf("task carries blocker label %q", label)
		}
	}
	return true, ""
}

// EvaluateAndOrchestrateBacklog inspects candidate backlog tickets for a project,
// evaluates their eligibility for autonomous development, and initiates chain runs
// up to the configured maximum concurrent sessions.
func (d *DB) EvaluateAndOrchestrateBacklog(ctx context.Context, projectID string, actor Actor, req models.OrchestrateProjectRequest) (*models.OrchestrationResult, error) {
	if req.Guidelines != nil && len(*req.Guidelines) > models.OrchestratorGuidelinesMaxBytes {
		return nil, fmt.Errorf("orchestration guidelines exceed 32 KiB")
	}
	project, err := d.GetProjectByID(projectID)
	if err != nil || project == nil {
		return nil, fmt.Errorf("project not found: %s", projectID)
	}

	if !project.OrchestratorEnabled && !req.Force {
		return nil, fmt.Errorf("orchestrator is disabled for project %q (pass force to override)", project.Name)
	}

	maxConcurrent := project.OrchestratorMaxConcurrent
	if req.MaxConcurrent != nil && *req.MaxConcurrent > 0 {
		maxConcurrent = models.NormalizeOrchestratorMaxConcurrent(*req.MaxConcurrent)
	}
	if maxConcurrent < 1 {
		maxConcurrent = 1
	}

	activeBefore, err := d.ActiveRunsCountForProject(project.ID)
	if err != nil {
		return nil, fmt.Errorf("cannot check active runs count: %w", err)
	}

	availableSlots := maxConcurrent - activeBefore
	if availableSlots < 0 {
		availableSlots = 0
	}

	result := &models.OrchestrationResult{
		ProjectID:        project.ID,
		ActiveRunsBefore: activeBefore,
		ActiveRunsAfter:  activeBefore,
		MaxConcurrent:    maxConcurrent,
		AvailableSlots:   availableSlots,
		Launched:         []models.OrchestratedTaskInfo{},
		Skipped:          []models.SkippedTaskInfo{},
	}

	if availableSlots <= 0 {
		return result, nil
	}

	tasks, err := d.GetTasks("", "", "", "", project.ID, "", "", "", "", nil, nil, false)
	if err != nil {
		return nil, fmt.Errorf("cannot retrieve project tasks: %w", err)
	}

	stopStage := models.FullChainStopImplemented
	guidelines := project.OrchestratorPrompt
	if req.Guidelines != nil {
		guidelines = *req.Guidelines
	}
	prompt := orchestratorLaunchPrompt(guidelines)

	for _, task := range tasks {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		stage := d.StageOfTask(&task)
		// Only consider tasks before stopStage and not finished
		if stage == "finished" || stageAtOrPast(stage, stopStage) {
			continue
		}

		result.EvaluatedCount++

		active, batch, _ := d.ActiveBusyCause(task.ID)
		if active != nil || batch != nil {
			result.Skipped = append(result.Skipped, models.SkippedTaskInfo{
				ID:     task.ID,
				Key:    task.Key,
				Title:  task.Title,
				Reason: "task already has an active execution",
			})
			continue
		}

		eligible, reason := d.EvaluateTaskEligibility(&task)
		if !eligible {
			result.Skipped = append(result.Skipped, models.SkippedTaskInfo{
				ID:     task.ID,
				Key:    task.Key,
				Title:  task.Title,
				Reason: reason,
			})
			continue
		}

		if availableSlots <= 0 {
			result.Skipped = append(result.Skipped, models.SkippedTaskInfo{
				ID:     task.ID,
				Key:    task.Key,
				Title:  task.Title,
				Reason: "max concurrency limit reached",
			})
			continue
		}

		// Launch autonomous full chain run
		step, ok := NextStep(stage)
		if !ok {
			continue
		}
		_, act, launchErr := d.enqueueSkillWithOptions(task.ID, project.ID, step.SkillID, prompt, true, models.SkillModeAutonomous, "", queuedSkillOptions{
			UserID: actor.ID, DeviceID: agentprotocol.DeviceIDFromContext(ctx), ChainStop: stopStage, MaxConcurrent: maxConcurrent,
		})
		if errors.Is(launchErr, errOrchestratorCapacity) {
			availableSlots = 0
		}
		if launchErr != nil {
			result.Skipped = append(result.Skipped, models.SkippedTaskInfo{
				ID:     task.ID,
				Key:    task.Key,
				Title:  task.Title,
				Reason: fmt.Sprintf("chain launch failed: %v", launchErr),
			})
			continue
		}

		result.Launched = append(result.Launched, models.OrchestratedTaskInfo{
			ID:         task.ID,
			Key:        task.Key,
			Title:      task.Title,
			Stage:      stage,
			ActivityID: act.ID,
		})
		availableSlots--
	}

	activeAfter, err := d.ActiveRunsCountForProject(project.ID)
	if err == nil {
		result.ActiveRunsAfter = activeAfter
	} else {
		result.ActiveRunsAfter = activeBefore + len(result.Launched)
	}
	result.AvailableSlots = maxConcurrent - result.ActiveRunsAfter
	if result.AvailableSlots < 0 {
		result.AvailableSlots = 0
	}

	return result, nil
}
