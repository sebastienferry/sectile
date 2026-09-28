package db

import (
	"fmt"
	"strings"

	"tasks/internal/models"
)

// A batch (#522, ADR 0034) is the ordered list of tickets one pickup_issues run
// works through. The run sits on the first ticket, the lead; batch_members says
// which other tickets it covers and where each one stands. Membership is only
// ever read through the batch run's status, so a batch that ended shows nothing
// and its rows need no cleanup.

// batchMemberSelect reads a member's place in a running batch: the batch run,
// its lead, the member's position and state, and the size of the batch.
func batchMemberSelect() string {
	return `SELECT m.task_id, m.run_id, a.task_id, COALESCE(l.key, ''), m.position, m.state,
			(SELECT COUNT(*) FROM batch_members s WHERE s.run_id = m.run_id)
		FROM batch_members m
		JOIN task_activities a ON a.id = m.run_id
		LEFT JOIN tasks l ON l.id = a.task_id
		WHERE a.skill_id = 'remote_run' AND a.status IN (` + sqlStringList(activeRunStatuses) + `)`
}

// RecordBatch records the members of the batch run runID, in launch order. The
// first one is the lead and starts processing; the others wait their turn. Every
// member is announced to the listeners, so each board picks the batch up.
func (d *DB) RecordBatch(runID string, taskIDs []string) error {
	runID = strings.TrimSpace(runID)
	if runID == "" || len(taskIDs) < 2 {
		return fmt.Errorf("a batch needs a run and at least two tickets")
	}
	d.mu.Lock()
	err := func() error {
		tx, err := d.conn.Begin()
		if err != nil {
			return err
		}
		defer func() { _ = tx.Rollback() }()
		for i, taskID := range taskIDs {
			state := models.BatchMemberWaiting
			if i == 0 {
				state = models.BatchMemberProcessing
			}
			if _, err := tx.Exec(`INSERT INTO batch_members (run_id, task_id, position, state) VALUES (?, ?, ?, ?)`,
				runID, taskID, i+1, state); err != nil {
				return fmt.Errorf("recording batch member %s: %w", taskID, err)
			}
		}
		return tx.Commit()
	}()
	d.mu.Unlock()
	if err != nil {
		return err
	}
	d.notifyBatchMembers(runID, taskIDs...)
	return nil
}

// MarkBatchMemberProcessing records that the agent of the batch run runID has
// started working on taskID: that member becomes processing and the one that
// was processing becomes done. Reporting the member already processing changes
// nothing, and reporting a done one makes it processing again.
//
// member is false when taskID is not in that batch, and nothing is changed.
// previous is the member that became done, empty when none did. The run's
// status is the caller's to check: this only moves the mark.
func (d *DB) MarkBatchMemberProcessing(runID, taskID string) (member bool, previous string, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	tx, err := d.conn.Begin()
	if err != nil {
		return false, "", err
	}
	defer func() { _ = tx.Rollback() }()
	// Every row of the batch is locked, so two reports crossing on two
	// instances leave exactly one member processing.
	rows, err := tx.Query(`SELECT task_id, state FROM batch_members WHERE run_id = ? ORDER BY position`+d.forUpdate(), runID)
	if err != nil {
		return false, "", err
	}
	var processing []string
	for rows.Next() {
		var id, state string
		if err := rows.Scan(&id, &state); err != nil {
			rows.Close()
			return false, "", err
		}
		if id == taskID {
			member = true
		}
		if state == models.BatchMemberProcessing {
			processing = append(processing, id)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return false, "", err
	}
	if !member {
		return false, "", nil
	}
	if len(processing) == 1 && processing[0] == taskID {
		return true, "", nil
	}
	if _, err := tx.Exec(`UPDATE batch_members SET state = ? WHERE run_id = ? AND state = ? AND task_id != ?`,
		models.BatchMemberDone, runID, models.BatchMemberProcessing, taskID); err != nil {
		return false, "", err
	}
	if _, err := tx.Exec(`UPDATE batch_members SET state = ? WHERE run_id = ? AND task_id = ?`,
		models.BatchMemberProcessing, runID, taskID); err != nil {
		return false, "", err
	}
	if err := tx.Commit(); err != nil {
		return false, "", err
	}
	for _, id := range processing {
		if id != taskID {
			previous = id
		}
	}
	return true, previous, nil
}

// ActiveBatchOf is the place of taskID in a running batch, or nil when it is in
// none. Should a race have put it in two, the older batch is reported.
func (d *DB) ActiveBatchOf(taskID string) (*models.TaskBatch, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.activeBatchOfUnsafe(taskID)
}

func (d *DB) activeBatchOfUnsafe(taskID string) (*models.TaskBatch, error) {
	batches, err := d.scanActiveBatchesUnsafe(" AND m.task_id = ?", taskID)
	if err != nil {
		return nil, err
	}
	if batch, ok := batches[taskID]; ok {
		return &batch, nil
	}
	return nil, nil
}

// activeBatchesUnsafe is the place of every task that is in a running batch,
// by task id. It reads the members of the running batches rather than the
// tasks of a list, since there are few of the first and many of the second.
func (d *DB) activeBatchesUnsafe() (map[string]models.TaskBatch, error) {
	return d.scanActiveBatchesUnsafe("")
}

func (d *DB) scanActiveBatchesUnsafe(filter string, args ...any) (map[string]models.TaskBatch, error) {
	rows, err := d.conn.Query(batchMemberSelect()+filter+" ORDER BY a.created_at ASC", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	batches := map[string]models.TaskBatch{}
	for rows.Next() {
		var taskID string
		var b models.TaskBatch
		if err := rows.Scan(&taskID, &b.RunID, &b.LeadTaskID, &b.LeadKey, &b.Position, &b.State, &b.Size); err != nil {
			return nil, err
		}
		if _, seen := batches[taskID]; !seen {
			batches[taskID] = b
		}
	}
	return batches, rows.Err()
}

// attachBatchesUnsafe fills Task.Batch on a task list with one query.
func (d *DB) attachBatchesUnsafe(tasks []models.Task) error {
	if len(tasks) == 0 {
		return nil
	}
	batches, err := d.activeBatchesUnsafe()
	if err != nil || len(batches) == 0 {
		return err
	}
	for i := range tasks {
		if batch, ok := batches[tasks[i].ID]; ok {
			tasks[i].Batch = &batch
		}
	}
	return nil
}

// batchMemberIDs lists the members of the batch run runID in launch order,
// whatever the run's status. Empty for a run that is no batch.
func (d *DB) batchMemberIDs(runID string) []string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	rows, err := d.conn.Query(`SELECT task_id FROM batch_members WHERE run_id = ? ORDER BY position`, runID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	return ids
}

// notifyBatchMembers announces the given members of a batch, or every member
// when none is given, each read afresh so its batch is the current one. It is
// how the other tickets of a batch hear that it started, moved or ended: the run
// itself only names its lead.
func (d *DB) notifyBatchMembers(runID string, taskIDs ...string) {
	if len(taskIDs) == 0 {
		taskIDs = d.batchMemberIDs(runID)
	}
	if len(taskIDs) == 0 {
		return
	}
	activity, _ := d.GetActivityByID(runID)
	for _, id := range taskIDs {
		task, err := d.GetTaskByID(id)
		if err != nil || task == nil {
			continue
		}
		d.notifyPostBackListeners(task, activity, nil)
	}
}
