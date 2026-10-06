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
	// The tracker now holds the labels: the local ticket follows at once,
	// whatever took them away since the operation was queued, the revert of an
	// earlier refusal or a sync that read the tracker before the write.
	d.mu.Lock()
	err = d.applyTaskLabelsUnsafe(op)
	d.mu.Unlock()
	if err != nil {
		*steps = append(*steps, fmt.Sprintf("⚠️ Labels locaux de %s non mis à jour : %v", task.Key, err))
	}
	return note, nil
}

// applyTaskLabelsUnsafe writes on the local ticket what a task_labels
// operation wrote on the tracker: the labels it added are carried, those it
// removed are not. The caller holds d.mu.
func (d *DB) applyTaskLabelsUnsafe(op TrackerOp) error {
	return d.conn.WithTx(func(tx *sqlTx) error {
		task, err := d.lockTaskUnsafe(tx, op.TaskID)
		if err != nil || task == nil {
			return err
		}
		labels := []string{}
		changed := false
		for _, l := range task.Labels {
			if namesOneOf(l, op.RemovedLabels) {
				changed = true
				continue
			}
			labels = append(labels, l)
		}
		for _, l := range op.Labels {
			if l = strings.TrimSpace(l); l != "" && !labelCarried(labels, l) {
				labels = append(labels, l)
				changed = true
			}
		}
		if !changed {
			return nil
		}
		labelsJSON, _ := json.Marshal(labels)
		_, err = tx.Exec(`UPDATE tasks SET labels = ?, updated_at = ? WHERE id = ?`, string(labelsJSON), time.Now(), task.ID)
		return err
	})
}

// namesOneOf says whether the ticket label l is one of labels, A-Z folded as
// membership compares them: l as stored, each of labels trimmed.
func namesOneOf(l string, labels []string) bool {
	for _, label := range labels {
		if strings.TrimSpace(label) != "" && labelCarried([]string{l}, label) {
			return true
		}
	}
	return false
}

// remoteLabelsTimeout bounds the read of a ticket's labels on its tracker
// before a refused write is reverted.
const remoteLabelsTimeout = 10 * time.Second

// remoteTaskLabels reads the labels the operation's ticket carries on its
// tracker, as whoever the context names, within remoteLabelsTimeout. known is
// false when the tracker cannot read a ticket or the read fails.
func (d *DB) remoteTaskLabels(ctx context.Context, op TrackerOp) (labels []string, known bool) {
	task, err := d.GetTaskByID(op.TaskID)
	if err != nil || task == nil {
		return nil, false
	}
	ts, err := d.TrackerForTask(task)
	if err != nil || ts == nil || !ts.Supports(tracker.CapGet) {
		return nil, false
	}
	d.mu.RLock()
	trk := d.trackerOfOp(op)
	d.mu.RUnlock()
	readCtx, cancel := context.WithTimeout(ctx, remoteLabelsTimeout)
	defer cancel()
	remote, err := ts.GetIssue(readCtx, tracker.GetIssueRequest{Tracker: trk, Key: task.Key})
	if err != nil || remote == nil {
		return nil, false
	}
	return remote.Labels, true
}

// revertTaskLabelsUnsafe undoes on the local ticket what a failed task_labels
// operation wrote ahead of the tracker: the labels it added are withdrawn,
// A-Z folded as membership compares them, and those it removed come back.
// Otherwise the next sync would rewrite the labels from the tracker and the
// ticket would leave its project with nothing saying why. When the tracker's
// own labels are known (remoteKnown), what they show is kept: an added label
// the tracker carries stays, a removed one it lacks stays away, since a sync
// may have brought that state meanwhile, or the write landed before its answer
// was lost. It returns the labels actually withdrawn and restored. The caller
// holds d.mu.
func (d *DB) revertTaskLabelsUnsafe(op TrackerOp, remote []string, remoteKnown bool) (withdrawn, restored []string, err error) {
	err = d.conn.WithTx(func(tx *sqlTx) error {
		task, err := d.lockTaskUnsafe(tx, op.TaskID)
		if err != nil || task == nil {
			return err
		}
		labels := []string{}
		for _, l := range task.Labels {
			if namesOneOf(l, op.Labels) && !(remoteKnown && namesOneOf(l, remote)) {
				withdrawn = append(withdrawn, l)
				continue
			}
			labels = append(labels, l)
		}
		for _, l := range op.RemovedLabels {
			if remoteKnown && !labelCarried(remote, l) {
				continue
			}
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
// operation and says so in a step of its activity. The ticket is read on its
// tracker first, outside the lock, so a label the tracker carries is not
// withdrawn; a tracker that cannot answer leaves the revert whole.
func (d *DB) revertFailedTaskLabelsOp(ctx context.Context, activityID string, op TrackerOp) {
	remote, known := d.remoteTaskLabels(ctx, op)
	d.mu.Lock()
	withdrawn, restored, err := d.revertTaskLabelsUnsafe(op, remote, known)
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
