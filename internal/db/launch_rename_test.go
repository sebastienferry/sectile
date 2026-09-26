package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"

	"tasks/internal/agentprotocol"
)

// A launch whose rename to agent_launch failed ends failed under its stage
// skill, and the task accepts a new launch at once instead of answering 409
// until the row is reclaimed (#439).
func TestAFailedLaunchRenameFreesTheTask(t *testing.T) {
	d, _ := activeRunDB(t)
	if _, err := d.conn.Exec(`CREATE TRIGGER refuse_agent_launch BEFORE UPDATE OF skill_id ON task_activities
		WHEN NEW.skill_id = 'agent_launch'
		BEGIN SELECT RAISE(ABORT, 'forced rename failure'); END`); err != nil {
		t.Fatal(err)
	}
	d.SetAgentOperations(func(ctx context.Context, op agentprotocol.Operation) (json.RawMessage, error) {
		return json.RawMessage(`null`), nil
	})
	if err := addRun(d, "launch", "clarify", "queued", false); err != nil {
		t.Fatal(err)
	}
	d.processSkillJob(SkillJob{ActivityID: "launch", SkillID: "clarify", TaskID: "t1"})

	var skillID, status, summary string
	var errorText sql.NullString
	var completedAt sql.NullTime
	if err := d.conn.QueryRow(`SELECT skill_id, status, summary, error, completed_at FROM task_activities WHERE id = 'launch'`).
		Scan(&skillID, &status, &summary, &errorText, &completedAt); err != nil {
		t.Fatal(err)
	}
	if skillID != "clarify" || status != "failed" || summary != "Agent launch failed" || !completedAt.Valid {
		t.Fatalf("launch row = %q %q %q completed=%v", skillID, status, summary, completedAt.Valid)
	}
	if !strings.Contains(errorText.String, "forced rename failure") {
		t.Fatalf("error = %q, want the database error", errorText.String)
	}
	var runs int
	if err := d.conn.QueryRow(`SELECT COUNT(*) FROM task_activities WHERE task_id = 't1' AND skill_id = 'remote_run'`).Scan(&runs); err != nil || runs != 0 {
		t.Fatalf("a failed launch started %d remote runs (%v)", runs, err)
	}

	if _, err := d.conn.Exec(`DROP TRIGGER refuse_agent_launch`); err != nil {
		t.Fatal(err)
	}
	if _, _, err := d.EnqueueSkillOnTask("t1", "clarify", ""); err != nil {
		t.Fatalf("a new launch after the failed one: %v", err)
	}
	waitForIdleConnections(t, d)
}

// The status-only close never overwrites a cancellation (FR3).
func TestAFailedLaunchCloseKeepsACancellation(t *testing.T) {
	d, _ := activeRunDB(t)
	if err := addRun(d, "launch", "clarify", "canceled", false); err != nil {
		t.Fatal(err)
	}
	d.closeLaunch("launch", false, "failed", "Agent launch failed", "forced rename failure")
	if got := activityStatus(t, d, "launch"); got != "canceled" {
		t.Fatalf("status = %q, want canceled", got)
	}
}
