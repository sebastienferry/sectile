package db

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"tasks/internal/models"
)

// batchEngines runs a batch test on SQLite, and on PostgreSQL when a test
// server is configured.
func batchEngines(t *testing.T, test func(t *testing.T, d *DB)) {
	t.Run("sqlite", func(t *testing.T) {
		d, err := NewDB(filepath.Join(t.TempDir(), "batch.db"))
		if err != nil {
			t.Fatalf("opening: %v", err)
		}
		t.Cleanup(func() { d.Close() })
		test(t, d)
	})
	t.Run("postgres", func(t *testing.T) {
		test(t, openPostgres(t))
	})
}

// seedBatch creates tickets #1 to #4 and a batch run on #1 covering #1, #2 and
// #3, in that order. #4 is in no batch.
func seedBatch(t *testing.T, d *DB) *models.TaskActivity {
	t.Helper()
	seedProjectAndUser(t, d)
	for _, id := range []string{"1", "2", "3", "4"} {
		if _, err := d.conn.Exec(`INSERT INTO tasks (id, project_id, key, title, status, priority) VALUES (?, 'p1', ?, 'T', 'backlog', 'medium')`,
			"t"+id, "#"+id); err != nil {
			t.Fatalf("seeding task %s: %v", id, err)
		}
	}
	run, err := d.StartAgentRun("t1", "pickup_issues", RunLaunch{UserID: "u1"})
	if err != nil {
		t.Fatalf("starting the batch run: %v", err)
	}
	if err := d.RecordBatch(run.ID, []string{"t1", "t2", "t3"}); err != nil {
		t.Fatalf("recording the batch: %v", err)
	}
	return run
}

// batchStates reads the state of each ticket in its running batch, "" for a
// ticket in none.
func batchStates(t *testing.T, d *DB) map[string]string {
	t.Helper()
	states := map[string]string{}
	for _, id := range []string{"t1", "t2", "t3", "t4"} {
		batch, err := d.ActiveBatchOf(id)
		if err != nil {
			t.Fatalf("reading the batch of %s: %v", id, err)
		}
		if batch != nil {
			states[id] = batch.State
		} else {
			states[id] = ""
		}
	}
	return states
}

func wantStates(t *testing.T, d *DB, t1, t2, t3 string) {
	t.Helper()
	got := batchStates(t, d)
	want := map[string]string{"t1": t1, "t2": t2, "t3": t3, "t4": ""}
	for id, state := range want {
		if got[id] != state {
			t.Fatalf("states = %v, want %v", got, want)
		}
	}
}

// A new batch names its lead and each member's position; the lead processes and
// the others wait.
func TestRecordBatchStartsWithTheLead(t *testing.T) {
	batchEngines(t, func(t *testing.T, d *DB) {
		run := seedBatch(t, d)
		wantStates(t, d, models.BatchMemberProcessing, models.BatchMemberWaiting, models.BatchMemberWaiting)
		batch, err := d.ActiveBatchOf("t3")
		if err != nil || batch == nil {
			t.Fatalf("batch of #3 = %+v, %v", batch, err)
		}
		want := models.TaskBatch{RunID: run.ID, LeadTaskID: "t1", LeadKey: "#1", Position: 3, Size: 3, State: models.BatchMemberWaiting}
		if *batch != want {
			t.Fatalf("batch of #3 = %+v, want %+v", *batch, want)
		}
		if err := d.RecordBatch(run.ID, []string{"t4"}); err == nil {
			t.Fatal("a batch of one ticket was recorded")
		}
	})
}

// The processing mark moves only when the agent reports a member, and the
// member it leaves is done.
func TestMarkBatchMemberProcessingMovesTheMark(t *testing.T) {
	batchEngines(t, func(t *testing.T, d *DB) {
		run := seedBatch(t, d)

		member, previous, err := d.MarkBatchMemberProcessing(run.ID, "t2")
		if err != nil || !member || previous != "t1" {
			t.Fatalf("reporting #2 = %v, %q, %v", member, previous, err)
		}
		wantStates(t, d, models.BatchMemberDone, models.BatchMemberProcessing, models.BatchMemberWaiting)

		// Reporting the member already processing changes nothing.
		member, previous, err = d.MarkBatchMemberProcessing(run.ID, "t2")
		if err != nil || !member || previous != "" {
			t.Fatalf("reporting #2 again = %v, %q, %v", member, previous, err)
		}
		wantStates(t, d, models.BatchMemberDone, models.BatchMemberProcessing, models.BatchMemberWaiting)

		// A done member reported again processes again.
		if _, _, err := d.MarkBatchMemberProcessing(run.ID, "t1"); err != nil {
			t.Fatal(err)
		}
		wantStates(t, d, models.BatchMemberProcessing, models.BatchMemberDone, models.BatchMemberWaiting)

		// A ticket outside the batch is refused and changes nothing.
		member, _, err = d.MarkBatchMemberProcessing(run.ID, "t4")
		if err != nil || member {
			t.Fatalf("reporting #4 = %v, %v", member, err)
		}
		wantStates(t, d, models.BatchMemberProcessing, models.BatchMemberDone, models.BatchMemberWaiting)
	})
}

// A batch shows on every member's task, in the list and alone, while its run is
// active, and on none once the run ended, whatever the outcome.
func TestBatchEndsWithItsRun(t *testing.T) {
	for _, status := range []string{"completed", "failed", "canceled"} {
		t.Run(status, func(t *testing.T) {
			batchEngines(t, func(t *testing.T, d *DB) {
				run := seedBatch(t, d)
				listed := func() map[string]*models.TaskBatch {
					t.Helper()
					tasks, err := d.GetTasks("", "", "", "", "p1", "", "", "", "", nil, nil, false)
					if err != nil {
						t.Fatal(err)
					}
					out := map[string]*models.TaskBatch{}
					for _, task := range tasks {
						out[task.ID] = task.Batch
					}
					return out
				}
				before := listed()
				for i, id := range []string{"t1", "t2", "t3"} {
					if before[id] == nil || before[id].Position != i+1 || before[id].LeadKey != "#1" {
						t.Fatalf("listed batch of %s = %+v", id, before[id])
					}
					task, _ := d.GetTaskByID(id)
					if task.Batch == nil || task.Batch.Position != i+1 {
						t.Fatalf("batch of %s alone = %+v", id, task.Batch)
					}
				}
				if before["t4"] != nil {
					t.Fatalf("#4 is in no batch, yet lists %+v", before["t4"])
				}

				if _, err := d.FinishRemoteRun("t1", run.ID, status, "over"); err != nil {
					t.Fatal(err)
				}
				for id, batch := range listed() {
					if batch != nil {
						t.Fatalf("%s still lists a batch once it %s: %+v", id, status, batch)
					}
				}
				wantStates(t, d, "", "", "")
				if active, err := d.ActiveRunOnTask("t2"); err != nil || active != nil {
					t.Fatalf("#2 is still busy once the batch %s: %+v %v", status, active, err)
				}
			})
		})
	}
}

// A member of a running batch is busy through the batch run, and a queued
// launch on it is refused with the batch named.
func TestBatchMembersAreBusy(t *testing.T) {
	batchEngines(t, func(t *testing.T, d *DB) {
		run := seedBatch(t, d)
		for _, id := range []string{"t1", "t2", "t3"} {
			active, batch, err := d.ActiveBusyCause(id)
			if err != nil || active == nil || active.ID != run.ID || batch == nil || batch.RunID != run.ID {
				t.Fatalf("busy cause of %s = %+v, %+v, %v", id, active, batch, err)
			}
		}
		if active, batch, err := d.ActiveBusyCause("t4"); err != nil || active != nil || batch != nil {
			t.Fatalf("#4 is in no batch, yet busy: %+v, %+v, %v", active, batch, err)
		}

		_, _, err := d.EnqueueSkillOnTaskWithOverrides("t2", "clarify", "", "", "")
		var busy *TaskBusyError
		if !errors.As(err, &busy) || busy.Batch == nil || busy.Batch.LeadKey != "#1" || busy.TaskKey != "#2" {
			t.Fatalf("a queued launch on #2 = %v (%+v)", err, busy)
		}
		activities, err := d.GetTaskActivities("t2")
		if err != nil || len(activities) != 0 {
			t.Fatalf("the refused launch left %d activities on #2 (%v)", len(activities), err)
		}
	})
}

// The agent reports the member it starts working on by starting the batch run
// on it: the batch run comes back, no run is created, and the mark moves. A
// ticket outside the batch and a batch that ended are refused.
func TestStartRunWithTheBatchRunReportsAMember(t *testing.T) {
	batchEngines(t, func(t *testing.T, d *DB) {
		run := seedBatch(t, d)

		reused, err := d.StartRemoteRunBy("u1", "#2", "pickup_issues", run.ID)
		if err != nil || reused.ID != run.ID || reused.TaskID != "t1" {
			t.Fatalf("start_run on #2 with the batch run = %+v, %v", reused, err)
		}
		if activities, _ := d.GetTaskActivities("t2"); len(activities) != 0 {
			t.Fatalf("reporting #2 recorded %d activities on it", len(activities))
		}
		wantStates(t, d, models.BatchMemberDone, models.BatchMemberProcessing, models.BatchMemberWaiting)

		// The lead reported with its own run takes the mark back.
		if _, err := d.StartRemoteRunBy("u1", "t1", "pickup_issues", run.ID); err != nil {
			t.Fatal(err)
		}
		wantStates(t, d, models.BatchMemberProcessing, models.BatchMemberDone, models.BatchMemberWaiting)

		if _, err := d.StartRemoteRunBy("u1", "t4", "pickup_issues", run.ID); err == nil {
			t.Fatal("start_run on a ticket outside the batch was accepted")
		}

		if _, err := d.FinishRemoteRun("t1", run.ID, "completed", "done"); err != nil {
			t.Fatal(err)
		}
		if _, err := d.StartRemoteRunBy("u1", "t3", "pickup_issues", run.ID); err == nil {
			t.Fatal("start_run with a finished batch run was accepted")
		}
	})
}

// An agent reporting the end of the batch run announces every ticket of the
// batch, each without its batch, as finish_run does.
func TestAgentReportedEndAnnouncesEveryMember(t *testing.T) {
	batchEngines(t, func(t *testing.T, d *DB) {
		run := seedBatch(t, d)
		cleared := make(chan string, 16)
		d.RegisterPostBackListener(func(task *models.Task, _ *models.TaskActivity, _ error) {
			if task != nil && task.Batch == nil {
				cleared <- task.ID
			}
		})
		if _, err := d.SyncRemoteRunStatus(run.ID, "t1", "p1", "#1", "pickup_issues", "completed", "done", nil); err != nil {
			t.Fatal(err)
		}
		want := map[string]bool{"t1": true, "t2": true, "t3": true}
		deadline := time.After(5 * time.Second)
		for len(want) > 0 {
			select {
			case id := <-cleared:
				delete(want, id)
			case <-deadline:
				t.Fatalf("no announcement without the batch for %v", want)
			}
		}
		wantStates(t, d, "", "", "")
	})
}
