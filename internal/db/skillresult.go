package db

import (
	"database/sql"

	"tasks/internal/models"
)

func (d *DB) managedStageRunningUnsafe(taskID string) (bool, error) {
	var running bool
	err := d.conn.QueryRow(`SELECT EXISTS (
		SELECT 1 FROM task_activities WHERE task_id = ? AND status = 'running'
		AND action != ('Exécution de ' || skill_id || ' sur l''agent local')
		AND skill_id IN ('clarify', 'specify', 'implement', 'adjust', 'create_pr', 'review', 'pickup', 'pick', 'handoff')
	)`, taskID).Scan(&running)
	return running, err
}

// ActiveRunOnTask reports the run that makes a task busy for a new launch, or
// nil when none does. Every active run counts, ordinary or concurrent: the
// database only refuses a second ordinary one, but a launch must still see a
// forced run it would sit next to. A queued run is active, since it will start
// an agent on the task; so is a run waiting for user input, whose row stays
// 'running'. An agent_launch record is a launch, not a run.
//
// A running run is reported before a queued one, the oldest first.
//
// This is deliberately not managedStageRunningUnsafe: that helper guards stage
// transitions and postbacks, and its skill list omits 'remote_run' on purpose.
func (d *DB) ActiveRunOnTask(taskID string) (*models.TaskActivity, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var a models.TaskActivity
	var startedAt, waitingSince sql.NullTime
	err := d.conn.QueryRow(`
		SELECT id, task_id, skill_id, skill_name, action, status, started_at, waiting_since, user_id, concurrent
		FROM task_activities
		WHERE task_id = ? AND `+activeRunPredicate()+`
		ORDER BY started_at IS NULL, started_at ASC, created_at ASC
		LIMIT 1
	`, taskID).Scan(
		&a.ID,
		&a.TaskID,
		&a.SkillID,
		&a.SkillName,
		&a.Action,
		&a.Status,
		&startedAt,
		&waitingSince,
		&a.UserID,
		&a.Concurrent,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if startedAt.Valid {
		a.StartedAt = &startedAt.Time
	}
	if waitingSince.Valid {
		a.WaitingSince = &waitingSince.Time
	}
	return &a, nil
}
