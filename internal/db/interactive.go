package db

import (
	"context"
	"fmt"
	"strings"

	"tasks/internal/models"
	"tasks/internal/tracker"
)

// CompleteInteractiveStep closes a workflow step that ran in a TTY session.
//
// A headless skill is moved by the worker, which is the only place that posts
// the stage label and transitions the ticket. An interactive step has no
// worker: the agent answers in the terminal and the repo skill only produces
// text, it never touches the tracker. Without this call the ticket stayed where
// it was, whatever happened in the session. So the user confirms the session is
// over, and Taskacao applies the same move the worker would have applied.
//
// note, when not empty, is published as a comment on the ticket: the summary of
// what the session concluded.
func (d *DB) CompleteInteractiveStep(taskID, skillID, note string) (*models.Task, *models.TaskActivity, error) {
	skillID = models.NormalizeSkillID(skillID)
	stageLabel := skillStageLabel[skillID]
	if stageLabel == "" {
		return nil, nil, fmt.Errorf("skill %q sans étape de workflow", skillID)
	}

	// A non-interactive run of the same skill has already posted this
	// transition. The user confirming afterwards is confirming something that
	// happened, so it succeeds and changes nothing rather than failing on a
	// board that is already where it should be.
	if task, err := d.GetTaskByID(taskID); err == nil && task != nil && d.StageOfTask(task) == stageLabel {
		return task, nil, nil
	}

	return d.TransitionTaskStage(taskID, stageLabel, note, "", "")
}

func (d *DB) pushStageToTracker(task *models.Task, stageLabel, statusTarget, trackerURL, note string) {
	if task == nil {
		return
	}
	ts, err := d.TrackerForTask(task)
	if err != nil || ts == nil || ts.Name() == "local" {
		return
	}

	stale := StaleWorkflowLabels(stageLabel)
	body := ""
	if strings.TrimSpace(note) != "" {
		body = "### 💬 [Sectile] Rapport de session interactive\n\n" + note
	}

	proj, _ := d.GetProjectByID(task.ProjectID)
	go func(t models.Task, p *models.Project, staleLbls []string, comment string) {
		ctx := context.Background()
		_ = ts.UpdateIssue(ctx, tracker.UpdateIssueRequest{
			Project:       p,
			Task:          &t,
			Key:           t.Key,
			Status:        &t.Status,
			Labels:        t.Labels,
			RemovedLabels: staleLbls,
		})
		if comment != "" && ts.Supports(tracker.CapComment) {
			_ = ts.AddComment(ctx, tracker.AddCommentRequest{
				Project: p,
				Key:     t.Key,
				Body:    comment,
			})
		}
	}(*task, proj, stale, body)
}
