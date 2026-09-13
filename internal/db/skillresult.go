package db

func (d *DB) managedStageRunningUnsafe(taskID string) (bool, error) {
	var running bool
	err := d.conn.QueryRow(`SELECT EXISTS (
		SELECT 1 FROM task_activities WHERE task_id = ? AND status = 'running'
		AND action != ('Exécution de ' || skill_id || ' sur l''agent local')
		AND skill_id IN ('clarify', 'specify', 'implement', 'adjust', 'create_pr', 'review', 'pickup', 'pick', 'handoff')
	)`, taskID).Scan(&running)
	return running, err
}
