package db

import (
	"fmt"
	"strings"

	"tasks/internal/models"
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

	return d.TransitionTaskStage(taskID, stageLabel, note, "", "")
}

func (d *DB) pushStageToTracker(task *models.Task, stageLabel, statusTarget, trackerURL, note string) {
	isTracked := task.Source == "linear" || task.Source == "github" || task.Source == "jira" ||
		strings.HasPrefix(task.Key, "FRE-") || strings.HasPrefix(task.Key, "#") ||
		strings.HasPrefix(task.Key, "gh-") || strings.HasPrefix(task.Key, "GH-#")
	if !isTracked {
		return
	}

	settings, _ := d.GetSettings()
	if settings == nil {
		return
	}
	repoPath := d.ResolveTaskRepoPath(task)
	if repoPath == "" {
		repoPath = settings.RepoPath
	}
	stale := StaleWorkflowLabels(stageLabel)
	body := ""
	if strings.TrimSpace(note) != "" {
		body = "### 💬 [TaskFlow] Rapport de session interactive\n\n" + note
	}

	go func(src, repo, rPath, key string, st models.Status, lbls, staleLbls []string, target, url, comment string) {
		switch {
		case src == "linear" || strings.HasPrefix(key, "FRE-"):
			_ = d.runner.UpdateLinearIssueState(key, st)
			_ = d.runner.UpdateLinearIssue(key, nil, nil, nil, &st, lbls)

		default:
			_ = d.runner.UpdateGithubIssueState(repo, rPath, key, st)
			_ = d.runner.UpdateGithubIssue(repo, rPath, key, nil, nil, &st, lbls, staleLbls)
		}
		if comment != "" {
			_ = d.runner.AddIssueComment(src, repo, rPath, key, comment)
		}
	}(task.Source, settings.GithubRepo, repoPath, task.Key, task.Status, task.Labels, stale, statusTarget, trackerURL, body)
}
