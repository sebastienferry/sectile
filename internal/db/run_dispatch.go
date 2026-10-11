package db

import (
	"fmt"
	"time"

	"tasks/internal/models"
)

// ClaimRunDispatch and CancelUndispatchedRun arbitrate admission against cancel
// in the shared database, including requests handled by different replicas.
func (d *DB) ClaimRunDispatch(runID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	result, err := d.conn.Exec(`UPDATE task_activities SET dispatch_state='dispatching'
 WHERE id=? AND dispatch_state='admitted' AND status IN ('queued','pending','running')`, runID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("L’exécution n’attend plus son envoi au poste")
	}
	return nil
}

func (d *DB) RunDispatchState(runID string) (string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var state string
	err := d.conn.QueryRow("SELECT dispatch_state FROM task_activities WHERE id=?", runID).Scan(&state)
	return state, err
}

func (d *DB) CompleteRunDispatch(runID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	_, err := d.conn.Exec("UPDATE task_activities SET dispatch_state='dispatched' WHERE id=? AND dispatch_state='dispatching'", runID)
	return err
}

func (d *DB) CancelUndispatchedRun(runID string) (*models.TaskActivity, bool, error) {
	d.mu.Lock()
	result, err := d.conn.Exec(`UPDATE task_activities SET status='canceled', completed_at=?,
 summary='Exécution annulée avant l’envoi au poste', waiting_since=NULL, waiting_session=''
 WHERE id=? AND dispatch_state='admitted' AND status IN ('queued','pending','running')`, time.Now(), runID)
	d.mu.Unlock()
	if err != nil {
		return nil, false, err
	}
	count, err := result.RowsAffected()
	if err != nil || count == 0 {
		return nil, false, err
	}
	activityBefore, _ := d.GetActivityByID(runID)
	if activityBefore != nil && activityBefore.TaskID != "" {
		d.handBackRun(activityBefore.TaskID, runID, "canceled")
	}
	d.notifyBatchMembers(runID, d.batchMemberIDs(runID)...)
	activity, err := d.GetActivityByID(runID)
	return activity, true, err
}

// addLaunchedRemoteRun transfers a claimed queue entry to its remote execution
// only while the launcher is still active. Cancellation of the old row wins
// before this transaction, or sees the recorded remote run after it.
func (d *DB) addLaunchedRemoteRun(activity models.TaskActivity, launchID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	tx, err := d.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var status string
	if err := tx.QueryRow("SELECT status FROM task_activities WHERE id=?"+d.dialect.ForUpdate(), launchID).Scan(&status); err != nil {
		return err
	}
	if status != "running" {
		return fmt.Errorf("Le lancement n’est plus actif")
	}
	if _, err := tx.Exec("UPDATE task_activities SET remote_run_id=? WHERE id=?", activity.ID, launchID); err != nil {
		return err
	}
	if err := insertTaskActivity(tx, d.instanceID, activity); err != nil {
		return err
	}
	return tx.Commit()
}

// CancelQueuedActivity wins only before a worker claims the queue record.
func (d *DB) CancelQueuedActivity(activityID string) (bool, error) {
	d.mu.Lock()
	result, err := d.conn.Exec(`UPDATE task_activities SET status='canceled', summary='Annulée par l’utilisateur',
 completed_at=?, waiting_since=NULL, waiting_session='' WHERE id=? AND status IN ('queued','pending')`, time.Now(), activityID)
	d.mu.Unlock()
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	if err != nil || count == 0 {
		return false, err
	}
	d.cancelLocal(activityID)
	d.publish(BusMessage{Kind: busKindCancel, ActivityID: activityID})
	return true, nil
}

// LaunchRemoteRun reads the launch's exact child, never the task's newest run.
func (d *DB) LaunchRemoteRun(launchID string) (string, bool, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var runID, status string
	err := d.conn.QueryRow("SELECT remote_run_id, status FROM task_activities WHERE id=?", launchID).Scan(&runID, &status)
	return runID, status == "queued" || status == "pending" || status == "running", err
}
