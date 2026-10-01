package db

import (
	"strings"
	"testing"
)

// A macro run, which has no task, reports its wait by its project and macro
// key, and its refusals tell a wrong id, a run of something else and a
// finished run apart (#648).
func TestMacroRunReportsItsWait(t *testing.T) {
	batchEngines(t, func(t *testing.T, d *DB) {
		shapedMacro(t, d)
		seedTask(t, d)
		run, err := d.StartMacroRunBy("u1", "p1", "M-1", "refine-macro", "")
		if err != nil {
			t.Fatalf("starting the macro run: %v", err)
		}
		owner := Actor{ID: "u1"}

		activity, applied, err := d.ReportSessionMacroRunWaitingAs(owner, false, "instance.session-1", "p1", "m-1", run.ID, true)
		if err != nil || !applied || activity == nil || activity.WaitingSince == nil || activity.MacroKey != "M-1" {
			t.Fatalf("report_waiting on the macro run = %+v, %v, %v; want the wait applied", activity, applied, err)
		}
		runs, err := d.MacroRuns("p1", "M-1", 5)
		if err != nil || len(runs) != 1 || runs[0].WaitingSince == nil {
			t.Fatalf("the macro's runs = %+v, %v; want the run with its wait", runs, err)
		}
		if cleared, err := d.ResumeWaits("instance.session-1"); err != nil || len(cleared) != 1 {
			t.Fatalf("the session's next call cleared %v, %v; want the macro run", cleared, err)
		}
		if waitingOf(t, d, run.ID) {
			t.Fatal("the macro run is still waiting after the session's next call")
		}

		if _, _, err := d.ReportSessionMacroRunWaitingAs(Actor{ID: "u2"}, false, "s", "p1", "M-1", run.ID, true); err != ErrRunNotYours {
			t.Errorf("another user's report = %v, want ErrRunNotYours", err)
		}
		task, err := d.StartAgentRun("t1", "clarify-issue", RunLaunch{UserID: "u1"})
		if err != nil {
			t.Fatal(err)
		}
		for _, refused := range []struct {
			why  string
			call func() error
			want string
		}{
			{"an unknown run", func() error {
				_, _, err := d.ReportSessionMacroRunWaitingAs(owner, false, "s", "p1", "M-1", "no-such-run", true)
				return err
			}, "remote run no-such-run not found"},
			{"a task run named as a macro run", func() error {
				_, _, err := d.ReportSessionMacroRunWaitingAs(owner, false, "s", "p1", "M-1", task.ID, true)
				return err
			}, "is not attached to macro M-1"},
			{"a macro run named by a task", func() error {
				_, _, err := d.ReportSessionRunWaitingAs(owner, false, "s", "t1", run.ID, true)
				return err
			}, "is not attached to task #1"},
		} {
			if err := refused.call(); err == nil || !strings.Contains(err.Error(), refused.want) {
				t.Errorf("%s: err = %v, want %q", refused.why, err, refused.want)
			}
		}

		if _, err := d.FinishMacroRunAs(owner, false, "p1", "M-1", run.ID, "completed", "done"); err != nil {
			t.Fatal(err)
		}
		_, _, err = d.ReportSessionMacroRunWaitingAs(owner, false, "s", "p1", "M-1", run.ID, true)
		if err == nil || !strings.Contains(err.Error(), "is no longer running (completed)") {
			t.Errorf("a finished macro run: err = %v, want it named as no longer running", err)
		}
	})
}
