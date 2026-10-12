package db

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"tasks/internal/models"
)

func checkWorkstationSnapshot(t *testing.T, d *DB) {
	t.Helper()
	user, err := d.SignInLocal("snapshot@example.com")
	if err != nil {
		t.Fatal(err)
	}
	_, device, err := d.CreateAPIKey(user.ID, "  My Mac  ", 0)
	if err != nil {
		t.Fatal(err)
	}
	if label := d.WorkstationLabel(user.ID, device.ID); label != "My Mac" {
		t.Fatal(label)
	}
	if label := d.WorkstationLabel("foreign", device.ID); label != device.ID {
		t.Fatal("foreign label disclosed")
	}
	task, err := d.CreateTask(models.CreateTaskRequest{Title: "Snapshot", ProjectID: "default"})
	if err != nil {
		t.Fatal(err)
	}
	run, err := d.StartAgentRun(task.ID, "implement", RunLaunch{UserID: user.ID, DeviceID: device.ID, DeviceLabel: d.WorkstationLabel(user.ID, device.ID)})
	if err != nil {
		t.Fatal(err)
	}
	if err := d.FillRunWorkstation(run.ID, user.ID, "other"); err != nil {
		t.Fatal(err)
	}
	if err := d.RevokeDeviceCredential(user.ID, device.ID); err != nil {
		t.Fatal(err)
	}
	for _, read := range []func() (*models.TaskActivity, error){
		func() (*models.TaskActivity, error) { return d.GetActivityByID(run.ID) },
		func() (*models.TaskActivity, error) {
			a, e := d.GetTaskActivities(task.ID)
			if e != nil || len(a) != 1 {
				return nil, e
			}
			return &a[0], nil
		},
		func() (*models.TaskActivity, error) {
			a, e := d.GetActivities("", "", "", task.ID, "", 100)
			if e != nil || len(a) != 1 {
				return nil, e
			}
			return &a[0], nil
		},
	} {
		actual, err := read()
		if err != nil || actual == nil || actual.DeviceLabel != "My Mac" || actual.DeviceID != device.ID {
			t.Fatalf("snapshot lost: %+v %v", actual, err)
		}
	}
}
func TestWorkstationSnapshot(t *testing.T)         { checkWorkstationSnapshot(t, testDB(t)) }
func TestPostgresWorkstationSnapshot(t *testing.T) { checkWorkstationSnapshot(t, openPostgres(t)) }

func TestWorkstationSnapshotSurvivesRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "restart.db")
	d, err := NewDB(path)
	if err != nil {
		t.Fatal(err)
	}
	checkWorkstationSnapshot(t, d)
	d.Close()
	d, err = NewDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	rows, err := d.GetActivities("", "", "", "", "", 100)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range rows {
		if row.DeviceLabel == "My Mac" {
			found = true
		}
	}
	if !found {
		t.Fatal("restart erased label snapshot")
	}
}

func TestCancelAndDispatchHaveOneWinner(t *testing.T) {
	path := filepath.Join(t.TempDir(), "race.db")
	d, err := NewDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	other, err := NewDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	task, err := d.CreateTask(models.CreateTaskRequest{Title: "Race", ProjectID: "default"})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		run, err := d.StartAgentRun(task.ID, "implement", RunLaunch{UserID: "u", DeviceID: "mac"})
		if err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		wg.Add(2)
		var claimed, canceled bool
		go func() { defer wg.Done(); claimed = d.ClaimRunDispatch(run.ID) == nil }()
		go func() { defer wg.Done(); _, canceled, err = other.CancelUndispatchedRun(run.ID) }()
		wg.Wait()
		if err != nil || claimed == canceled {
			t.Fatalf("claim=%v cancel=%v err=%v", claimed, canceled, err)
		}
		if canceled {
			if e := d.ClaimRunDispatch(run.ID); e == nil {
				t.Fatal("canceled run dispatched")
			}
		}
		if claimed {
			if e := d.CompleteRunDispatch(run.ID); e != nil {
				t.Fatal(e)
			}
			if _, e := d.FinishRemoteRun(task.ID, run.ID, "canceled", "test stopped"); e != nil {
				t.Fatal(e)
			}
		}
	}
}

func TestLegacyAttributionIsAuthenticatedAndFilledOnce(t *testing.T) {
	d := testDB(t)
	if err := d.AddTaskActivity(models.TaskActivity{ID: "legacy", TaskID: "task", UserID: "owner", SkillID: "remote_run", Status: "running", CreatedAt: time.Now(), Concurrent: true}); err != nil {
		t.Fatal(err)
	}
	if err := d.FillRunWorkstation("legacy", "foreign", "fake"); err != nil {
		t.Fatal(err)
	}
	run, _ := d.GetActivityByID("legacy")
	if run.DeviceID != "" {
		t.Fatal("foreign assignment accepted")
	}
	for _, device := range []string{"first", "second"} {
		if err := d.FillRunWorkstation("legacy", "owner", device); err != nil {
			t.Fatal(err)
		}
	}
	run, _ = d.GetActivityByID("legacy")
	if run.DeviceID != "first" || run.DeviceLabel != "first" {
		t.Fatal("legacy assignment replaced")
	}
}

func TestRetryKeepsLaunchOptionsAndBatchMembers(t *testing.T) {
	d := testDB(t)
	lead, err := d.CreateTask(models.CreateTaskRequest{Title: "Retry lead", ProjectID: "default"})
	if err != nil {
		t.Fatal(err)
	}
	member, err := d.CreateTask(models.CreateTaskRequest{Title: "Retry member", ProjectID: "default"})
	if err != nil {
		t.Fatal(err)
	}
	run, err := d.StartAgentRun(lead.ID, "pickup_issues", RunLaunch{Prompt: "accepted batch prompt", Mode: "autonomous", Model: "chosen", UserID: "owner", DeviceID: "old"})
	if err != nil {
		t.Fatal(err)
	}
	if err := d.RecordBatch(run.ID, []string{lead.ID, member.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.FinishRemoteRun(lead.ID, run.ID, "failed", "test retry"); err != nil {
		t.Fatal(err)
	}
	request, err := d.RetryTaskRequest(run, "default", "new")
	if err != nil {
		t.Fatal(err)
	}
	if request.Prompt != "accepted batch prompt" || request.Mode != "autonomous" || request.Model != "chosen" || request.DeviceID != "new" || len(request.BatchTaskIDs) != 2 || request.BatchTaskIDs[1] != member.ID {
		t.Fatalf("retry options lost: %+v", request)
	}
}
