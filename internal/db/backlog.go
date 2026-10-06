package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"tasks/internal/models"
	"tasks/internal/tracker"
)

// TrackerOpTaskLabels writes labels on one ticket of a tracker, the way a
// backlog ticket joins a project (#741).
const TrackerOpTaskLabels TrackerOpKind = "task_labels"

// ErrProjectWithoutLabel refuses to label a ticket into a project that has no
// label: such a project already shows every ticket of its trackers.
var ErrProjectWithoutLabel = errors.New("ce projet n'a pas de label : il montre déjà tous les tickets de ses trackers")

// ErrProjectNotOnTracker refuses to label a ticket into a project that does not
// select the ticket's tracker.
var ErrProjectNotOnTracker = errors.New("ce projet ne sélectionne pas le tracker de ce ticket")

// backlogScopeUnsafe selects a tracker's tickets no project shows: those of
// the tracker that no linked project's label selects. A tracker an unlabelled
// project selects has an empty backlog.
func (d *DB) backlogScopeUnsafe(trackerID string) (string, []interface{}) {
	index := d.membershipUnsafe()
	cond := "tracker_id = ?"
	args := []interface{}{trackerID}
	var labels []string
	for _, p := range index.members(trackerID, nil) {
		if strings.TrimSpace(p.Label) == "" {
			return "1 = 0", nil
		}
	}
	if index != nil {
		for _, i := range index.byTracker[trackerID] {
			labels = append(labels, strings.TrimSpace(index.projects[i].Label))
		}
	}
	if labelCond, labelArgs := viewLabelScope(labels, d.lowerASCII("labels")); labelCond != "" {
		cond += " AND NOT " + labelCond
		args = append(args, labelArgs...)
	}
	return "(" + cond + ")", args
}

// GetTrackerBacklog lists a tracker's tickets that no project shows (#741).
func (d *DB) GetTrackerBacklog(trackerID string) ([]models.Task, error) {
	return d.GetTasksInScope(TaskScope{BacklogOf: trackerID}, "", "", "", "", "", "", "", "", nil, nil, false)
}

// TrackerHasProjects says whether at least one project selects the tracker:
// who may read its backlog and label from it.
func (d *DB) TrackerHasProjects(trackerID string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	ids, _ := d.trackerLinkedProjectsUnsafe(trackerID)
	return len(ids) > 0
}

// AddTaskToProjectAs gives a ticket the label of a project selecting its
// tracker, so it joins the project at once, and queues the label write on the
// tracker with the acting user's credential (#741). A ticket already carrying
// the label queues no write: the write would add nothing, and its failure
// would withdraw a label the ticket held before.
func (d *DB) AddTaskToProjectAs(ctx context.Context, taskID, projectID string) (*models.Task, *models.TaskActivity, error) {
	userID := tracker.ActingUser(ctx)
	d.mu.Lock()
	project, err := d.getProjectByIDUnsafe(projectID)
	if err != nil || project == nil {
		d.mu.Unlock()
		return nil, nil, fmt.Errorf("projet non trouvé")
	}
	label := strings.TrimSpace(project.Label)
	var task *models.Task
	added := false
	err = d.conn.WithTx(func(tx *sqlTx) error {
		var err error
		task, err = d.lockTaskUnsafe(tx, taskID)
		if err != nil {
			return err
		}
		if task == nil {
			return fmt.Errorf("task not found")
		}
		linked := false
		for _, t := range project.Trackers {
			linked = linked || (task.TrackerID != "" && t.TrackerID == task.TrackerID)
		}
		switch {
		case !linked:
			return ErrProjectNotOnTracker
		case label == "":
			return ErrProjectWithoutLabel
		case labelCarried(task.Labels, label):
			return nil
		}
		task.Labels = append(task.Labels, label)
		labelsJSON, _ := json.Marshal(task.Labels)
		task.UpdatedAt = time.Now()
		_, err = tx.Exec(`UPDATE tasks SET labels = ?, updated_at = ? WHERE id = ?`, string(labelsJSON), task.UpdatedAt, task.ID)
		added = err == nil
		return err
	})
	if err != nil {
		d.mu.Unlock()
		return nil, nil, err
	}
	d.fillTaskProjectsUnsafe(task, project.ID)
	var activity *models.TaskActivity
	if added && task.Source != "local" {
		activity, err = d.enqueueTrackerOpUnsafe(ctx, TrackerOp{Kind: TrackerOpTaskLabels, TaskID: task.ID, TaskKey: task.Key, TrackerID: task.TrackerID, Labels: []string{label}, UserID: userID})
	}
	d.mu.Unlock()
	return task, activity, err
}

// runTaskLabelsOp writes the labels of a task_labels operation on the ticket's
// tracker, through the tracker the context names.
func (d *DB) runTaskLabelsOp(ctx context.Context, op TrackerOp, steps *[]string) (string, error) {
	task, err := d.GetTaskByID(op.TaskID)
	if err != nil {
		return "", err
	}
	if task == nil {
		return "", fmt.Errorf("ticket %s introuvable", op.TaskKey)
	}
	ts, err := d.TrackerForTask(task)
	if err != nil {
		return "", err
	}
	if err := ts.UpdateLabels(ctx, task.Key, op.Labels, op.RemovedLabels); err != nil {
		return "", err
	}
	note := fmt.Sprintf("Labels de %s mis à jour sur %s : +%s", task.Key, trackerDisplayName(ts.Name()), strings.Join(op.Labels, ", +"))
	*steps = append(*steps, "✅ "+note)
	return note, nil
}

// revertTaskLabelsUnsafe undoes on the local ticket what a failed task_labels
// operation wrote ahead of the tracker: the labels it added are withdrawn,
// A-Z folded as membership compares them, and those it removed come back.
// Otherwise the next sync would rewrite the labels from the tracker and the
// ticket would leave its project with nothing saying why. It returns the
// labels actually withdrawn and restored. The caller holds d.mu.
func (d *DB) revertTaskLabelsUnsafe(op TrackerOp) (withdrawn, restored []string, err error) {
	err = d.conn.WithTx(func(tx *sqlTx) error {
		task, err := d.lockTaskUnsafe(tx, op.TaskID)
		if err != nil || task == nil {
			return err
		}
		labels := []string{}
		for _, l := range task.Labels {
			if labelCarried(op.Labels, l) {
				withdrawn = append(withdrawn, l)
				continue
			}
			labels = append(labels, l)
		}
		for _, l := range op.RemovedLabels {
			if l = strings.TrimSpace(l); l != "" && !labelCarried(labels, l) {
				labels = append(labels, l)
				restored = append(restored, l)
			}
		}
		if len(withdrawn) == 0 && len(restored) == 0 {
			return nil
		}
		labelsJSON, _ := json.Marshal(labels)
		_, err = tx.Exec(`UPDATE tasks SET labels = ?, updated_at = ? WHERE id = ?`, string(labelsJSON), time.Now(), task.ID)
		return err
	})
	if err != nil {
		return nil, nil, err
	}
	return withdrawn, restored, nil
}

// revertFailedTaskLabelsOp reverts the local labels of a failed task_labels
// operation and says so in a step of its activity.
func (d *DB) revertFailedTaskLabelsOp(activityID string, op TrackerOp) {
	d.mu.Lock()
	withdrawn, restored, err := d.revertTaskLabelsUnsafe(op)
	d.mu.Unlock()
	switch {
	case err != nil:
		d.appendActivityStep(activityID, fmt.Sprintf("⚠️ Labels locaux de %s non rétablis : %v", op.TaskKey, err))
	case len(withdrawn) > 0 || len(restored) > 0:
		var changes []string
		for _, l := range withdrawn {
			changes = append(changes, "-"+l)
		}
		for _, l := range restored {
			changes = append(changes, "+"+l)
		}
		d.appendActivityStep(activityID, fmt.Sprintf("↩️ Labels de %s rétablis localement : %s", op.TaskKey, strings.Join(changes, ", ")))
	}
}
