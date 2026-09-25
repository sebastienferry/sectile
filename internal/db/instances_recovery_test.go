package db

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tasks/internal/models"
)

// The lookups the MCP router and the sessions view rely on (#408): a session
// id names an instance, and only a live one is somewhere to forward to.
func TestLiveInstanceLookupsExcludeTheDead(t *testing.T) {
	recoveryEngines(t, func(t *testing.T, d *DB, _ *models.Project) {
		if _, err := d.conn.Exec(`DELETE FROM server_instances`); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _, _ = d.conn.Exec(`DELETE FROM server_instances`) })
		seedInstances(t, d, time.Now().UTC())
		if _, err := d.conn.Exec(`UPDATE server_instances SET address = 'http://10.0.0.1:8092' WHERE id = 'live'`); err != nil {
			t.Fatal(err)
		}

		if got, ok, err := d.LiveInstance("live"); err != nil || !ok || got.ID != "live" || got.Address != "http://10.0.0.1:8092" {
			t.Errorf("LiveInstance(live) = %+v, %v, %v", got, ok, err)
		}
		for _, id := range []string{"stale", "ghost", ""} {
			if got, ok, err := d.LiveInstance(id); err != nil || ok {
				t.Errorf("LiveInstance(%q) = %+v, %v, %v, want no live instance and no error", id, got, ok, err)
			}
		}
		live := d.LiveInstances()
		if len(live) != 1 || live[0].ID != "live" || live[0].Address != "http://10.0.0.1:8092" {
			t.Errorf("LiveInstances() = %+v, want the live instance alone", live)
		}
	})
}

// A client run whose session died with its instance was canceled in the
// client's absence, like one whose client disconnected: its owner may still
// report the real outcome (#408). The server's own closure path may not.
func TestOwnerRecoversARunReclaimedFromADeadInstance(t *testing.T) {
	recoveryEngines(t, func(t *testing.T, d *DB, project *models.Project) {
		if _, err := d.conn.Exec(`DELETE FROM server_instances`); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _, _ = d.conn.Exec(`DELETE FROM server_instances`) })
		owner, err := d.SignInLocal("alice@example.com")
		if err != nil {
			t.Fatal(err)
		}
		task, err := d.CreateTask(models.CreateTaskRequest{ProjectID: project.ID, Title: "reclaimed"})
		if err != nil {
			t.Fatal(err)
		}
		run, err := d.StartRemoteRunBy(owner.ID, task.ID, "implement", "")
		if err != nil {
			t.Fatal(err)
		}
		now := time.Now().UTC()
		seedInstances(t, d, now)
		if _, err := d.conn.Exec(`UPDATE task_activities SET instance_id = 'stale' WHERE id = ?`, run.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := d.reclaimDeadInstances(now); err != nil {
			t.Fatal(err)
		}
		reclaimed, _ := d.GetActivityByID(run.ID)
		if reclaimed.Status != "canceled" || reclaimed.Summary != interruptedClientRun {
			t.Fatalf("reclaimed run = %q/%q, want canceled with %q", reclaimed.Status, reclaimed.Summary, interruptedClientRun)
		}

		if _, err := d.FinishRemoteRun(task.ID, run.ID, "completed", "the server itself"); err == nil {
			t.Fatal("the server's own closure path rewrote a reclaimed run")
		}
		recovered, err := d.FinishRemoteRunAs(Actor{ID: owner.ID}, false, task.ID, run.ID, "completed", "done, really")
		if err != nil {
			t.Fatalf("the owner could not report on a reclaimed run: %v", err)
		}
		if recovered.Status != "completed" || recovered.Summary != "done, really" {
			t.Fatalf("recovered run = %q/%q, want the reported outcome", recovered.Status, recovered.Summary)
		}
	})
}

// On an engine that serves one process, a restart reclaims every client run.
// The owner may report on it afterwards, as after a disconnection.
func TestOwnerRecoversARunReclaimedByASingleProcessRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "restart.db")
	d, err := NewDB(path)
	if err != nil {
		t.Fatal(err)
	}
	no := false
	project, err := d.CreateProject(models.CreateProjectRequest{Name: "Restart", RepoPath: "/not-mounted",
		IssueTracker: "local", UseWorktrees: &no, AIProvider: "claude"})
	if err != nil {
		t.Fatal(err)
	}
	owner, err := d.SignInLocal("alice@example.com")
	if err != nil {
		t.Fatal(err)
	}
	task, err := d.CreateTask(models.CreateTaskRequest{ProjectID: project.ID, Title: "restarted"})
	if err != nil {
		t.Fatal(err)
	}
	run, err := d.StartRemoteRunBy(owner.ID, task.ID, "implement", "")
	if err != nil {
		t.Fatal(err)
	}
	d.Close()

	restarted, err := NewDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	reclaimed, _ := restarted.GetActivityByID(run.ID)
	if reclaimed.Status != "canceled" || !strings.Contains(reclaimed.Summary, models.RunDisconnectNote) {
		t.Fatalf("reclaimed run = %q/%q, want canceled with the disconnect note", reclaimed.Status, reclaimed.Summary)
	}
	recovered, err := restarted.FinishRemoteRunAs(Actor{ID: owner.ID}, false, task.ID, run.ID, "failed", "tests red")
	if err != nil {
		t.Fatalf("the owner could not report on a run lost with the restart: %v", err)
	}
	if recovered.Status != "failed" || recovered.Summary != "tests red" {
		t.Fatalf("recovered run = %q/%q, want the reported outcome", recovered.Status, recovered.Summary)
	}
}
