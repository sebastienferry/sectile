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
// The transition is recorded without an actor; callers that know who asked for
// it use TransitionTaskStageBy.
func (d *DB) TransitionTaskStage(taskIDOrKey string, targetStage string, note string, prURL string, branch string) (*models.Task, *models.TaskActivity, error) {
	return d.TransitionTaskStageBy("", taskIDOrKey, targetStage, note, prURL, branch)
}

// TransitionTaskStageBy is TransitionTaskStage attributed to a user, recorded
// on the transition's activity.
func (d *DB) TransitionTaskStageBy(actorID string, taskIDOrKey string, targetStage string, note string, prURL string, branch string) (*models.Task, *models.TaskActivity, error) {
	return d.TransitionTaskStageWithPRs(actorID, taskIDOrKey, targetStage, note, []string{prURL}, branch)
}

// TransitionTaskStageWithPRs is TransitionTaskStageBy for a ticket that may
// carry one pull request per repository it changed (#456). The first link is
// the one prUrl names; the others follow in any order.
func (d *DB) TransitionTaskStageWithPRs(actorID string, taskIDOrKey string, targetStage string, note string, prURLs []string, branch string) (*models.Task, *models.TaskActivity, error) {
	prURL := ""
	if len(prURLs) > 0 {
		prURL = prURLs[0]
	}
	var otherPRs []string
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
		set, err := d.validateStagePRs(task, actorID, skillForStage, d.adjustmentCheckout(task), branchForPR, prURLs)
		if err != nil {
			return nil, nil, err
		}
		prURL = set.primary()
		notice := set.notice
		if len(set.urls) > 1 {
			otherPRs = set.urls[:len(set.urls)-1]
		}
		if notice != "" {
			note = strings.TrimSpace(note + "\n\n" + notice)
		}
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

	// The workflow stage label becomes #<stage>. The labels, the branch and the
	// tracker status are derived inside the transaction, from the locked row.
	targetLabel := "#" + cleanStage
	var newLabels []string
	var branchName *string
	var trackerStatus string

	// Merge Request / PR URL
	mrURL := strings.TrimSpace(prURL)

	nowT := time.Now()
	now := nowT.Format("2006-01-02 15:04:05")

	activity, job, err := buildTrackerOpJob(TrackerOp{
		Kind: TrackerOpStage, ProjectID: task.ProjectID, TaskID: task.ID,
		TaskKey: task.Key, Stage: cleanStage, TargetStatus: trackerStatusTarget,
		Note: note, PrURL: mrURL, BranchName: branch, UserID: actorID,
	})
	if err != nil {
		return nil, nil, err
	}
	// Commit state and its tracker activity together. A failed activity insert
	// must not leave the task advanced without a report or synchronization job.
	//
	// The task row is locked and read again inside the transaction: a
	// transition running at the same time on another server instance waits,
	// then derives its labels, links and branch from what this one committed,
	// so neither loses the other's pull request. The running-stage rule is
	// checked on the same locked state.
	err = func() error {
		d.mu.Lock()
		defer d.mu.Unlock()
		return d.conn.WithTx(func(tx *sqlTx) error {
			locked, err := d.lockTaskUnsafe(tx, task.ID)
			if err != nil {
				return err
			}
			if locked == nil {
				return fmt.Errorf("tâche %q non trouvée", taskIDOrKey)
			}
			running, err := managedStageRunningOn(tx, task.ID)
			if err != nil {
				return err
			}
			if running {
				return fmt.Errorf("a managed Sectile stage is still running")
			}
			newLabels = SetWorkflowLabel(locked.Labels, targetLabel)
			labelsJSON, _ := json.Marshal(newLabels)
			branchName = locked.BranchName
			if strings.TrimSpace(branch) != "" {
				b := strings.TrimSpace(branch)
				branchName = &b
			}
			trackerStatus = locked.TrackerStatus
			if trackerStatusTarget != "" {
				trackerStatus = trackerStatusTarget
			}
			// The set is the authority and pr_url is its last link, so both are
			// written by the same statement: a follow-up PR on the task branch is
			// appended rather than replacing the PR the task already carries.
			links := locked.PrLinks
			if mrURL != "" {
				linkBranch := strings.TrimSpace(branch)
				if linkBranch == "" && branchName != nil {
					linkBranch = *branchName
				}
				// The other repositories' pull requests first, so that the
				// primary repository's stays the current one.
				for _, other := range otherPRs {
					links = models.AppendPullRequestLink(links, other, linkBranch)
				}
				links = pullRequestLinkLast(models.AppendPullRequestLink(links, mrURL, linkBranch), mrURL)
			}
			// A stage that records a link undoes a past detachment: the workflow
			// attached a pull request again, so rediscovery may speak once more. A
			// stage that records none leaves the flag alone — most transitions
			// carry no pull request, and raising it there would silence discovery
			// on every task.
			attached := 0
			if len(links) > 0 {
				attached = 1
			}
			if _, err := tx.Exec(`UPDATE tasks SET status = ?, labels = ?, tracker_status = ?, pr_url = ?, pr_links = ?, pr_links_detached = CASE WHEN ? = 1 THEN 0 ELSE pr_links_detached END, branch_name = ?, updated_at = ? WHERE id = ?`,
				string(newStatus), string(labelsJSON), trackerStatus, pullRequestURLValue(links), encodePullRequestLinks(links), attached, branchName, now, task.ID); err != nil {
				return err
			}
			task.PrLinks = links
			return insertTaskActivity(tx, d.instanceID, *activity)
		})
	}()
	if err != nil {
		return nil, nil, err
	}
	d.pushTrackerOpJob(job)

	task.Status = newStatus
	task.Labels = newLabels
	task.TrackerStatus = trackerStatus
	if mrURL != "" {
		task.PrURL = pullRequestURLValue(task.PrLinks)
	}
	if branchName != nil {
		task.BranchName = branchName
	}
	task.UpdatedAt = nowT

	return task, activity, nil
}
