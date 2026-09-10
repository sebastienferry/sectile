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
		return nil, nil, fmt.Errorf("une étape TaskFlow est en cours : son résultat doit être vérifié avant la transition")
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

	proj, _ := d.GetProjectByID(task.ProjectID)

	// Determine internal status for the stage
	var newStatus models.Status
	if proj != nil && len(proj.StageMapping) > 0 {
		if mapped, ok := proj.StageMapping[cleanStage]; ok && mapped != "" {
			newStatus = models.Status(mapped)
		}
	}
	if newStatus == "" {
		if st, ok := InternalStatusForStage(cleanStage); ok {
			newStatus = st
		} else {
			newStatus = task.Status
		}
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
	if mrURL == "" && (task.PrURL == nil || strings.TrimSpace(*task.PrURL) == "") {
		branchForMR := ""
		if task.BranchName != nil {
			branchForMR = *task.BranchName
		}
		if branch != "" {
			branchForMR = branch
		}
		if detectedURL, _ := d.runner.MergeRequestForStep(d.ResolveTaskRepoPath(task), branchForMR, note); detectedURL != "" {
			mrURL = detectedURL
		}
	}

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

	d.mu.Lock()
	var execErr error
	if mrURL != "" {
		_, execErr = d.conn.Exec(`
			UPDATE tasks SET status = ?, labels = ?, tracker_status = ?, pr_url = ?, branch_name = ?, updated_at = ?
			WHERE id = ?
		`, string(newStatus), string(labelsJSON), trackerStatus, mrURL, branchName, now, task.ID)
	} else {
		_, execErr = d.conn.Exec(`
			UPDATE tasks SET status = ?, labels = ?, tracker_status = ?, branch_name = ?, updated_at = ?
			WHERE id = ?
		`, string(newStatus), string(labelsJSON), trackerStatus, branchName, now, task.ID)
	}
	d.mu.Unlock()
	if execErr != nil {
		return nil, nil, execErr
	}

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

	// Enqueue tracker synchronization operation in the activity queue
	activity, opErr := d.EnqueueTrackerOp(TrackerOp{
		Kind:         TrackerOpStage,
		ProjectID:    task.ProjectID,
		TaskID:       task.ID,
		TaskKey:      task.Key,
		Stage:        cleanStage,
		TargetStatus: trackerStatusTarget,
		Note:         note,
		PrURL:        mrURL,
		BranchName:   branch,
	})
	if opErr != nil {
		return task, nil, opErr
	}

	return task, activity, nil
}
