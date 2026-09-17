package db

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"tasks/internal/models"
)

// TransitionTaskStage switches the agentic workflow stage and label on a story/task.
// It updates the local SQLite state immediately (workflow label replaced with #<stage>,
// status set to the mapped internal status, tracker status set to the mapped column)
// and enqueues tracker synchronization (labels, status, comments) in the activity queue.
func (d *DB) TransitionTaskStage(taskIDOrKey string, targetStage string, note string, prURL string, branch string) (*models.Task, *models.TaskActivity, error) {
	taskIDOrKey = strings.TrimSpace(taskIDOrKey)
	if taskIDOrKey == "" {
		return nil, nil, fmt.Errorf("identifiant ou clé de tâche manquant")
	}

	task, err := d.GetTaskByID(taskIDOrKey)
	if err != nil || task == nil {
		return nil, nil, fmt.Errorf("tâche %q non trouvée", taskIDOrKey)
	}
	d.mu.RLock()
	running, runErr := d.managedStageRunningUnsafe(task.ID)
	d.mu.RUnlock()
	if runErr != nil {
		return nil, nil, runErr
	}
	if running {
		return nil, nil, fmt.Errorf("une étape Sectile est en cours : son résultat doit être vérifié avant la transition")
	}

	cleanStage := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(targetStage), "#"))
	if cleanStage == "" {
		return nil, nil, fmt.Errorf("étape de workflow invalide")
	}

	// Normalize common stage aliases
	switch cleanStage {
	case "to_clarify", "open", "todo", "backlog":
		cleanStage = "new"
	case "to_implement":
		cleanStage = "specified"
	case "to_test":
		cleanStage = "implemented"
	case "to_close":
		cleanStage = "reviewed"
	case "done", "closed":
		cleanStage = "finished"
	}
	if _, ok := InternalStatusForStage(cleanStage); !ok {
		return nil, nil, fmt.Errorf("étape de workflow inconnue : %s", cleanStage)
	}

	branchForPR := strings.TrimSpace(branch)
	if branchForPR == "" && task.BranchName != nil {
		branchForPR = *task.BranchName
	}
	skillForStage := map[string]string{"specified": "specify", "implemented": "implement", "reviewed": "adjust"}[cleanStage]
	if skillForStage != "" {
		verified, err := d.validateStagePR(task, skillForStage, d.adjustmentCheckout(task), branchForPR, strings.TrimSpace(prURL), "")
		if err != nil {
			return nil, nil, err
		}
		prURL = verified
	}
	if d.StageOfTask(task) == "implemented" && cleanStage == "specified" {
		cleanStage = "implemented"
	}
	proj, _ := d.GetProjectByID(task.ProjectID)

	// The six stages are the six internal statuses, so the fold is fixed and
	// needs no per-project configuration.
	newStatus := task.Status
	if st, ok := InternalStatusForStage(cleanStage); ok {
		newStatus = st
	}

	// Determine tracker status target from project column mapping
	trackerStatusTarget := ""
	if proj != nil {
		trackerStatusTarget = TrackerStatusForStage(proj, cleanStage)
	}

	// Replace existing workflow stage label with #<stage>
	targetLabel := "#" + cleanStage
	newLabels := SetWorkflowLabel(task.Labels, targetLabel)
	labelsJSON, _ := json.Marshal(newLabels)

	// Merge Request / PR URL
	mrURL := strings.TrimSpace(prURL)

	branchName := task.BranchName
	if strings.TrimSpace(branch) != "" {
		b := strings.TrimSpace(branch)
		branchName = &b
	}

	trackerStatus := task.TrackerStatus
	if trackerStatusTarget != "" {
		trackerStatus = trackerStatusTarget
	}

	nowT := time.Now()
	now := nowT.Format("2006-01-02 15:04:05")

	activity, job, err := buildTrackerOpJob(TrackerOp{
		Kind: TrackerOpStage, ProjectID: task.ProjectID, TaskID: task.ID,
		TaskKey: task.Key, Stage: cleanStage, TargetStatus: trackerStatusTarget,
		Note: note, PrURL: mrURL, BranchName: branch,
	})
	if err != nil {
		return nil, nil, err
	}
	// Commit state and its tracker activity together. A failed activity insert
	// must not leave the task advanced without a report or synchronization job.
	err = func() error {
		d.mu.Lock()
		defer d.mu.Unlock()
		running, err := d.managedStageRunningUnsafe(task.ID)
		if err != nil {
			return err
		}
		if running {
			return fmt.Errorf("a managed Sectile stage is still running")
		}
		tx, err := d.conn.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback()
		pr := task.PrURL
		if mrURL != "" {
			pr = &mrURL
		}
		if _, err := tx.Exec(`UPDATE tasks SET status = ?, labels = ?, tracker_status = ?, pr_url = ?, branch_name = ?, updated_at = ? WHERE id = ?`,
			string(newStatus), string(labelsJSON), trackerStatus, pr, branchName, now, task.ID); err != nil {
			return err
		}
		if err := insertTaskActivity(tx, *activity); err != nil {
			return err
		}
		return tx.Commit()
	}()
	if err != nil {
		return nil, nil, err
	}
	d.pushTrackerOpJob(job)

	task.Status = newStatus
	task.Labels = newLabels
	task.TrackerStatus = trackerStatus
	if mrURL != "" {
		task.PrURL = &mrURL
	}
	if branchName != nil {
		task.BranchName = branchName
	}
	task.UpdatedAt = nowT

	return task, activity, nil
}
