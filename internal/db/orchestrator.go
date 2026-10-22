package db

import (
	"context"
	"fmt"
	"strings"

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

	query := `
		SELECT COUNT(DISTINCT a.task_id)
		FROM task_activities a
		WHERE (a.run_project_id IN (?, ?) OR (COALESCE(a.run_project_id, '') = '' AND a.task_id IN (SELECT id FROM tasks WHERE project_id IN (?, ?))))
		  AND a.status IN ('queued', 'pending', 'running')
		  AND a.skill_id IN ('remote_run', 'agent_launch', 'clarify', 'specify', 'implement', 'adjust', 'handoff', 'create_pr', 'pickup', 'rewrite_story', 'refine_macro', 'pickup_issues', 'review', 'pick', 'realign_macro')`

	var count int
	if err := d.conn.QueryRow(query, project.ID, project.Slug, project.ID, project.Slug).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// EvaluateTaskEligibility evaluates whether a task is suitable for autonomous agentic chaining.
func (d *DB) EvaluateTaskEligibility(task *models.Task, projectPrompt string) (bool, string) {
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
		norm := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(label, "#")))
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

	stopStage := d.FullChainStopStage(project.ID)

	for _, task := range tasks {
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

		eligible, reason := d.EvaluateTaskEligibility(&task, project.OrchestratorPrompt)
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
		_, act, launchErr := d.EnqueueFullChainRunFor(task.ID, project.ID)
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
