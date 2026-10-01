package db

import "testing"

func waitingOf(t *testing.T, d *DB, runID string) bool {
	t.Helper()
	run, err := d.GetActivityByID(runID)
	if err != nil || run == nil {
		t.Fatalf("reading run %s: %v", runID, err)
	}
	return run.WaitingSince != nil
}

// ResumeRunWait ends the wait of the run a launched console names, from a
// session other than the declaring one, and nothing else (#498).
func TestResumeRunWaitEndsOnlyTheNamedSessionWait(t *testing.T) {
	batchEngines(t, func(t *testing.T, d *DB) {
		seedProjectAndUser(t, d)
		seedTask(t, d)
		run, err := d.StartAgentRun("t1", "clarify-issue", RunLaunch{UserID: "u1"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := d.conn.Exec(`INSERT INTO tasks (id, project_id, key, title, status, priority) VALUES ('t2', 'p1', '#2', 'T', 'backlog', 'medium')`); err != nil {
			t.Fatal(err)
		}
		other, err := d.StartAgentRun("t2", "specify-issue", RunLaunch{UserID: "u1"})
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range []string{run.ID, other.ID} {
			if err := d.setRemoteRunWaiting(id, "dead.session-1", true); err != nil {
				t.Fatal(err)
			}
		}

		for _, refused := range []struct{ why, run, session, user string }{
			{"no run named", "", "new.session-2", "u1"},
			{"another user", run.ID, "new.session-2", "u2"},
			{"no user", run.ID, "new.session-2", ""},
			{"the declaring session itself", run.ID, "dead.session-1", "u1"},
		} {
			cleared, err := d.ResumeRunWait(refused.run, refused.session, refused.user)
			if err != nil || cleared {
				t.Errorf("%s: cleared=%v err=%v, want the wait kept", refused.why, cleared, err)
			}
		}
		if !waitingOf(t, d, run.ID) {
			t.Fatal("a refused call ended the wait")
		}

		cleared, err := d.ResumeRunWait(run.ID, "new.session-2", "u1")
		if err != nil || !cleared {
			t.Fatalf("the console's call: cleared=%v err=%v, want the wait ended", cleared, err)
		}
		if waitingOf(t, d, run.ID) {
			t.Error("the named run is still waiting")
		}
		if !waitingOf(t, d, other.ID) {
			t.Error("another waiting run lost its wait")
		}
	})
}

// A wait set by hand belongs to no session, so no console call ends it.
func TestResumeRunWaitLeavesAHandSetWait(t *testing.T) {
	batchEngines(t, func(t *testing.T, d *DB) {
		seedProjectAndUser(t, d)
		seedTask(t, d)
		run, err := d.StartAgentRun("t1", "clarify-issue", RunLaunch{UserID: "u1"})
		if err != nil {
			t.Fatal(err)
		}
		if err := d.SetRemoteRunWaiting(run.ID, true); err != nil {
			t.Fatal(err)
		}
		if cleared, err := d.ResumeRunWait(run.ID, "new.session-2", "u1"); err != nil || cleared {
			t.Fatalf("cleared=%v err=%v, want the hand-set wait kept", cleared, err)
		}
		if !waitingOf(t, d, run.ID) {
			t.Error("the hand-set wait ended")
		}
	})
}
