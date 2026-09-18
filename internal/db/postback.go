package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"tasks/internal/models"
)

// PostBackListener defines a callback function invoked whenever a post-back occurs.
type PostBackListener func(task *models.Task, activity *models.TaskActivity, err error)

// RegisterPostBackListener registers a listener to be notified when a task post-back completes.
func (d *DB) RegisterPostBackListener(listener PostBackListener) {
	d.postBackMu.Lock()
	defer d.postBackMu.Unlock()
	d.postBackListeners = append(d.postBackListeners, listener)
}

func (d *DB) notifyPostBackListeners(task *models.Task, activity *models.TaskActivity, err error) {
	d.postBackMu.RLock()
	defer d.postBackMu.RUnlock()
	for _, l := range d.postBackListeners {
		fn := l
		go fn(task, activity, err)
	}
}

// PostBackTask performs an idempotent local update of task attributes resulting from
// local actions or background tracker operations, records activity auditing logs,
// and notifies listeners.
func (d *DB) PostBackTask(payload models.TaskPostBackPayload) (*models.Task, *models.TaskActivity, error) {
	if payload.Stage != nil {
		stage := strings.TrimPrefix(strings.TrimSpace(*payload.Stage), "#")
		skill := map[string]string{"specified": "specify", "implemented": "implement", "reviewed": "adjust"}[stage]
		if skill != "" {
			id := strings.TrimSpace(payload.TaskID)
			if id == "" {
				id = payload.TaskKey
			}
			task, err := d.GetTaskByID(id)
			if err != nil || task == nil {
				return nil, nil, fmt.Errorf("postback task not found")
			}
			branch, url := "", ""
			if task.BranchName != nil {
				branch = *task.BranchName
			}
			if payload.BranchName != nil {
				branch = *payload.BranchName
			}
			if payload.PrURL != nil {
				url = *payload.PrURL
			}
			verified, err := d.validateStagePR(task, skill, d.adjustmentCheckout(task), branch, url)
			if err != nil {
				return nil, nil, err
			}
			if verified != "" {
				payload.PrURL = &verified
			}
			if d.StageOfTask(task) == "implemented" && stage == "specified" {
				stage = "implemented"
				payload.Stage = &stage
				payload.Status = nil
				payload.Labels = nil
				payload.TrackerStatus = nil
			}
		}
	}

	d.mu.Lock()

	targetIdentifier := strings.TrimSpace(payload.TaskID)
	if targetIdentifier == "" {
		targetIdentifier = strings.TrimSpace(payload.TaskKey)
	}

	if targetIdentifier == "" {
		d.mu.Unlock()
		err := fmt.Errorf("task identifier (taskId or taskKey) required for post-back")
		d.notifyPostBackListeners(nil, nil, err)
		return nil, nil, err
	}

	existing, err := d.getTaskByIDUnsafe(targetIdentifier)
	if (err != nil || existing == nil) && payload.TaskKey != "" && payload.TaskKey != targetIdentifier {
		existing, _ = d.getTaskByIDUnsafe(payload.TaskKey)
	}

	if existing == nil {
		d.mu.Unlock()
		err := fmt.Errorf("task not found for post-back: %s", targetIdentifier)
		d.notifyPostBackListeners(nil, nil, err)
		return nil, nil, err
	}
	if payload.Stage != nil || payload.Status != nil || payload.Labels != nil || payload.TrackerStatus != nil {
		running, runErr := d.managedStageRunningUnsafe(existing.ID)
		if runErr != nil || running {
			d.mu.Unlock()
			if runErr == nil {
				runErr = fmt.Errorf("une étape Sectile est en cours : son résultat doit être vérifié avant le post-back d'état")
			}
			d.notifyPostBackListeners(existing, nil, runErr)
			return nil, nil, runErr
		}
	}

	// 1. Mutate local task attributes idempotently
	if payload.Title != nil {
		existing.Title = *payload.Title
	}
	if payload.Description != nil {
		existing.Description = *payload.Description
	}
	if payload.Status != nil {
		existing.Status = *payload.Status
	}
	if payload.Stage != nil {
		cleanStage := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(*payload.Stage), "#"))
		if cleanStage != "" {
			existing.Labels = SetWorkflowLabel(existing.Labels, "#"+cleanStage)
			if internalSt, ok := InternalStatusForStage(cleanStage); ok && payload.Status == nil {
				existing.Status = internalSt
			}
		}
	}
	if payload.Assignee != nil {
		existing.Assignee = *payload.Assignee
	}
	if payload.AssigneeAvatar != nil {
		existing.AssigneeAvatar = *payload.AssigneeAvatar
	}
	if payload.BranchName != nil {
		existing.BranchName = payload.BranchName
	}
	if payload.PrURL != nil {
		branch := ""
		if existing.BranchName != nil {
			branch = *existing.BranchName
		}
		existing.PrLinks = models.AppendPullRequestLink(existing.PrLinks, *payload.PrURL, branch)
		existing.PrURL = pullRequestURLValue(existing.PrLinks)
	}
	if payload.Labels != nil {
		existing.Labels = *payload.Labels
	}
	if payload.TrackerStatus != nil {
		existing.TrackerStatus = *payload.TrackerStatus
	}
	if payload.Sprint != nil {
		existing.Sprint = *payload.Sprint
	}
	if payload.Team != nil {
		existing.Team = *payload.Team
	}
	if payload.TeamID != nil {
		existing.TeamID = *payload.TeamID
	}
	if payload.ParentKey != nil {
		existing.ParentKey = *payload.ParentKey
	}
	if payload.ParentTitle != nil {
		existing.ParentTitle = *payload.ParentTitle
	}
	if payload.ParentType != nil {
		existing.ParentType = *payload.ParentType
	}
	if payload.TrackerUpdatedAt != nil {
		existing.TrackerUpdatedAt = payload.TrackerUpdatedAt
	}

	existing.UpdatedAt = time.Now()

	// Update SQLite DB table `tasks`
	labelsJSON, _ := json.Marshal(existing.Labels)
	pinnedVal := 0
	if existing.Pinned {
		pinnedVal = 1
	}

	_, updateErr := d.conn.Exec(`
		UPDATE tasks
		SET project_id = ?, title = ?, description = ?, status = ?, priority = ?, labels = ?, pinned = ?, assignee = ?, assignee_avatar = ?, position = ?, due_date = ?, branch_name = ?, pr_url = ?, pr_links = ?, repo_path = ?, tracker_status = ?, source = ?, external_url = ?, issue_type = ?, sprint = ?, team = ?, team_id = ?, parent_key = ?, parent_title = ?, parent_type = ?, tracker_updated_at = ?, updated_at = ?
		WHERE id = ?
	`, existing.ProjectID, existing.Title, existing.Description, string(existing.Status), string(existing.Priority), string(labelsJSON), pinnedVal, existing.Assignee, existing.AssigneeAvatar, existing.Position, existing.DueDate, existing.BranchName, existing.PrURL, encodePullRequestLinks(existing.PrLinks), repoPathValue(existing.RepoPath), existing.TrackerStatus, existing.Source, existing.ExternalURL, existing.IssueType, existing.Sprint, existing.Team, existing.TeamID, existing.ParentKey, existing.ParentTitle, existing.ParentType, existing.TrackerUpdatedAt, existing.UpdatedAt, existing.ID)

	if updateErr != nil {
		log.Printf("[PostBackTask] error updating DB for task %s: %v", existing.Key, updateErr)
	}

	// 2. Activity failure auditing & status updating
	var activity *models.TaskActivity
	if payload.ActivityID != "" {
		if payload.Error != nil && *payload.Error != "" {
			_, _ = d.conn.Exec(`
				UPDATE task_activities
				SET status = 'failed', error = ?, summary = ?, completed_at = ?
				WHERE id = ?
			`, *payload.Error, "Échec post-back tracker : "+*payload.Error, time.Now(), payload.ActivityID)
		} else {
			_, _ = d.conn.Exec(`
				UPDATE task_activities
				SET status = 'completed', completed_at = ?
				WHERE id = ?
			`, time.Now(), payload.ActivityID)
		}
		activity = d.getActivityByIDUnsafe(payload.ActivityID)
	} else if payload.Error != nil && *payload.Error != "" {
		// Log failure audit entry without blocking local UX
		actID := fmt.Sprintf("postback-err-%d", time.Now().UnixNano())
		now := time.Now()
		act := models.TaskActivity{
			ID:          actID,
			TaskID:      existing.ID,
			TaskKey:     existing.Key,
			SkillID:     "postback",
			SkillName:   "Post-back Sync",
			Action:      "Post-back local",
			Status:      string(models.ActivityStatusFailed),
			Summary:     "Échec post-back tracker: " + *payload.Error,
			Error:       *payload.Error,
			Steps:       []string{fmt.Sprintf("❌ Erreur post-back: %s", *payload.Error)},
			CreatedAt:   now,
			CompletedAt: &now,
		}
		_ = d.addTaskActivityDirect(act)
		activity = &act
	}

	d.mu.Unlock()

	var pbErr error
	if payload.Error != nil && *payload.Error != "" {
		pbErr = fmt.Errorf("%s", *payload.Error)
	}

	// 3. Notify post-back listeners
	d.notifyPostBackListeners(existing, activity, pbErr)

	return existing, activity, pbErr
}

func (d *DB) getActivityByIDUnsafe(activityID string) *models.TaskActivity {
	var a models.TaskActivity
	var stepsJSON string
	var prompt, errStr, runProvider, runModel sql.NullString
	var startedAt, completedAt, waitingSince sql.NullTime

	err := d.conn.QueryRow(`
		SELECT id, task_id, skill_id, skill_name, action, status, summary, output, steps, prompt, started_at, completed_at, error, created_at, waiting_since, run_provider, run_model
		FROM task_activities WHERE id = ?
	`, activityID).Scan(&a.ID, &a.TaskID, &a.SkillID, &a.SkillName, &a.Action, &a.Status, &a.Summary, &a.Output, &stepsJSON, &prompt, &startedAt, &completedAt, &errStr, &a.CreatedAt, &waitingSince, &runProvider, &runModel)

	if err != nil {
		return nil
	}

	_ = json.Unmarshal([]byte(stepsJSON), &a.Steps)
	if a.Steps == nil {
		a.Steps = []string{}
	}
	if prompt.Valid {
		a.Prompt = prompt.String
	}
	if errStr.Valid {
		a.Error = errStr.String
	}
	if startedAt.Valid {
		a.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		a.CompletedAt = &completedAt.Time
	}
	if waitingSince.Valid {
		a.WaitingSince = &waitingSince.Time
	}
	a.Provider, a.Model = runProvider.String, runModel.String

	return &a
}
