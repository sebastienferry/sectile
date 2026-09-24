package db

import (
	"errors"
	"fmt"
	"strings"

	"tasks/internal/models"
)

// activeRunIndex is the partial unique index that allows one ordinary active
// run per task (migration 9, recreated by 13). The database enforces it, so it holds across
// server processes and on every path that inserts a run, where a check made in
// Go before the insert only holds on the path that makes it.
const activeRunIndex = "idx_activities_one_active_run"

// activeRunStatuses are the statuses of a run that is not over. A queued run
// counts: it waits for the project's worker and will start an agent on the task.
var activeRunStatuses = []string{
	string(models.ActivityStatusQueued),
	string(models.ActivityStatusPending),
	string(models.ActivityStatusRunning),
}

// activeRunSkillIDs are the activity kinds that are runs: a remote run, and every
// skill the catalog can queue on a task. 'review' and 'pick' are no longer
// written but survive in older rows. Everything else recorded on a task, a
// launch record, a tracker write, a synchronisation, is not a run.
//
// Migration 9 carries its own frozen copy of this list, as a migration must.
// A skill added to the catalog needs a later migration that recreates the index
// with it; TestActiveRunSkillsCoverTheCatalog fails until then.
var activeRunSkillIDs = []string{
	"remote_run", "clarify", "specify", "implement", "adjust", "handoff",
	"create_pr", "pickup", "rewrite_story", "refine_macro", "pickup_issues",
	"review", "pick", "realign_macro",
}

// activeRunPredicate is the SQL condition selecting the active runs, ordinary or
// not. The index adds "concurrent = 0" to it.
func activeRunPredicate() string {
	return "status IN (" + sqlStringList(activeRunStatuses) + ") AND skill_id IN (" + sqlStringList(activeRunSkillIDs) + ")"
}

func sqlStringList(values []string) string {
	quoted := make([]string, len(values))
	for i, v := range values {
		quoted[i] = "'" + strings.ReplaceAll(v, "'", "''") + "'"
	}
	return strings.Join(quoted, ", ")
}

// ErrTaskBusy says a task already carries an active ordinary run, so another
// one was not recorded.
var ErrTaskBusy = errors.New("task already has an active run")

// TaskBusyError is ErrTaskBusy with the run that makes the task busy, for an
// answer that names it. Active is nil when the run ended between the refused
// insert and the read that looked for it.
type TaskBusyError struct {
	Active *models.TaskActivity
}

func (e *TaskBusyError) Error() string {
	if e.Active == nil {
		return ErrTaskBusy.Error()
	}
	return fmt.Sprintf("%s: %s", ErrTaskBusy.Error(), e.Active.ID)
}

func (e *TaskBusyError) Is(target error) bool { return target == ErrTaskBusy }

// taskBusy turns a refused insert into the error its caller answers with, and
// leaves any other error as it is. It reads the active run with the plain
// connection, so the caller must not hold DB.mu.
func (d *DB) taskBusy(taskID string, err error) error {
	if !isUniqueViolation(err, activeRunIndex) {
		return err
	}
	active, _ := d.ActiveRunOnTask(taskID)
	return &TaskBusyError{Active: active}
}

// boolToInt writes a flag into an INTEGER column the same way on both engines.
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
