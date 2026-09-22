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
// nil when none does. A run waiting for user input is active: the session that
// owns it is still there, and waiting keeps the row in status 'running'. An
// agent_launch record is a launch, not a run, and is excluded the same way
// managedStageRunningUnsafe excludes it.
//
// This is deliberately not managedStageRunningUnsafe: that helper guards stage
// transitions and postbacks, and its skill list omits 'remote_run' on purpose.
func (d *DB) ActiveRunOnTask(taskID string) (*models.TaskActivity, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var a models.TaskActivity
	var startedAt, waitingSince sql.NullTime
	err := d.conn.QueryRow(`
		SELECT id, task_id, skill_id, skill_name, action, status, started_at, waiting_since, user_id
		FROM task_activities
		WHERE task_id = ? AND status = 'running'
		AND action != ('Exécution de ' || skill_id || ' sur l''agent local')
		AND skill_id IN ('remote_run', 'clarify', 'specify', 'implement', 'adjust', 'create_pr', 'review', 'pickup', 'pick', 'handoff')
		ORDER BY started_at ASC, created_at ASC
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
