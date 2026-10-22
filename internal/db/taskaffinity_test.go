package db

import (
	"path/filepath"
	"testing"
	"time"

	"tasks/internal/models"
)

func checkTaskLaunchAffinity(t *testing.T, d *DB) {
	t.Helper()
	project, err := d.CreateProject(models.CreateProjectRequest{Name: "Affinity project"})
	if err != nil {
		t.Fatal(err)
	}
	task, err := d.CreateTask(models.CreateTaskRequest{Title: "affinity", ProjectID: project.ID})
	if err != nil {
		t.Fatal(err)
	}
	otherTask, err := d.CreateTask(models.CreateTaskRequest{Title: "other affinity", ProjectID: project.ID})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	for i, row := range []struct{ id, task, project, user, device, skill, status string }{
		{"a", task.ID, project.ID, "u", "a", "agent_launch", "completed"},
		{"z", task.ID, project.ID, "u", "z", "agent_launch", "completed"},
		{"failed", task.ID, project.ID, "u", "failed", "agent_launch", "failed"},
		{"unconfirmed", task.ID, project.ID, "u", "unconfirmed", "agent_launch", "running"},
		{"legacy", task.ID, project.ID, "u", "", "agent_launch", "completed"},
		{"other-user", task.ID, project.ID, "other", "other", "agent_launch", "completed"},
		{"other-project", task.ID, "other", "u", "other", "agent_launch", "completed"},
		{"execution", task.ID, project.ID, "u", "other", "remote_run", "completed"},
		{"other-task", otherTask.ID, project.ID, "u", "other", "agent_launch", "completed"},
	} {
		at := now
		if i > 1 {
			at = now.Add(time.Duration(i) * time.Second)
		}
		err := d.AddTaskActivity(models.TaskActivity{ID: row.id, TaskID: row.task, RunProjectID: row.project, UserID: row.user, DeviceID: row.device, SkillID: row.skill, Status: row.status, CreatedAt: at})
		if err != nil {
			t.Fatal(err)
		}
	}
	device, err := d.TaskLaunchDevice(task.ID, project.ID, "u")
	if err != nil || device != "z" {
		t.Fatalf("scoped stable affinity: %s %v", device, err)
	}
	if device, err := d.TaskLaunchDevice(task.ID, project.ID, "missing"); err != nil || device != "" {
		t.Fatalf("unknown owner: %s %v", device, err)
	}
	run, err := d.StartAgentRun(task.ID, "implement", RunLaunch{UserID: "u", DeviceID: "z", ProjectID: project.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.StartRemoteRunBy("u", task.ID, "implement", run.ID); err != nil {
		t.Fatal(err)
	}
	if err := d.SetRemoteRunEngine(run.ID, "codex", "model"); err != nil {
		t.Fatal(err)
	}
	if err := d.SetRemoteRunWaiting(run.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := d.FinishRemoteRun(task.ID, run.ID, "failed", "execution failed after successful launch"); err != nil {
		t.Fatal(err)
	}
	persisted, err := d.GetActivityByID(run.ID)
	if err != nil || persisted.DeviceID != "z" {
		t.Fatalf("identity after adoption and completion: %+v %v", persisted, err)
	}
	for _, read := range []func() ([]models.TaskActivity, error){func() ([]models.TaskActivity, error) { return d.GetTaskActivities(task.ID) }, func() ([]models.TaskActivity, error) { return d.GetActivities("", "", "", task.ID, "", 0) }} {
		records, err := read()
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, record := range records {
			if record.ID == run.ID {
				found = record.DeviceID == "z"
			}
		}
		if !found {
			t.Fatal("history lost device identity")
		}
	}
	if device, err := d.TaskLaunchDevice(task.ID, project.ID, "u"); err != nil || device != "z" {
		t.Fatalf("failed execution erased launch affinity: %s %v", device, err)
	}
}

func TestTaskLaunchAffinityAndIdentitySQLite(t *testing.T) { checkTaskLaunchAffinity(t, testDB(t)) }
func TestPostgresTaskLaunchAffinityAndIdentity(t *testing.T) {
	checkTaskLaunchAffinity(t, openPostgres(t))
}

func checkActivityDeviceMigration(t *testing.T, d *DB) {
	t.Helper()
	task, err := d.CreateTask(models.CreateTaskRequest{Title: "legacy device", ProjectID: "default"})
	if err != nil {
		t.Fatal(err)
	}
	if err := d.AddTaskActivity(models.TaskActivity{ID: "legacy-device", TaskID: task.ID, SkillID: "agent_launch", Status: "completed", Summary: "Old summary (invented-device)", CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{"DROP INDEX task_launch_affinity", "ALTER TABLE task_activities DROP COLUMN device_id", "ALTER TABLE task_activities DROP COLUMN device_label", "ALTER TABLE task_activities DROP COLUMN dispatch_state", "ALTER TABLE task_activities DROP COLUMN remote_run_id", "ALTER TABLE projects DROP COLUMN orchestrator_prompt", "ALTER TABLE projects DROP COLUMN orchestrator_max_concurrent", "ALTER TABLE projects DROP COLUMN orchestrator_enabled", "ALTER TABLE jira_oauth_flows DROP COLUMN tracker", "DELETE FROM schema_migrations WHERE version>=53"} {
		if _, err := d.conn.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
	if err := d.applyMigrations(migrations, 52); err != nil {
		t.Fatal(err)
	}
	legacy, err := d.GetActivityByID("legacy-device")
	if err != nil || legacy == nil || legacy.DeviceID != "" {
		t.Fatalf("legacy identity guessed: %+v %v", legacy, err)
	}
	if err := d.applyMigrations(migrations, migrations[len(migrations)-1].version); err != nil {
		t.Fatal(err)
	}
}
func TestActivityDeviceMigrationSQLite(t *testing.T) { checkActivityDeviceMigration(t, testDB(t)) }
func TestPostgresActivityDeviceMigration(t *testing.T) {
	checkActivityDeviceMigration(t, openPostgres(t))
}

func TestActivityDeviceSurvivesRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "restart.db")
	d, err := NewDB(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.AddTaskActivity(models.TaskActivity{ID: "recorded", TaskID: "task", SkillID: "agent_launch", Status: "completed", DeviceID: "workstation", CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	d.Close()
	d, err = NewDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	activity, err := d.GetActivityByID("recorded")
	if err != nil || activity.DeviceID != "workstation" {
		t.Fatalf("restart identity: %+v %v", activity, err)
	}
}

func TestStatusSyncCannotConfirmAnUnconfirmedLaunch(t *testing.T) {
	d := testDB(t)
	task, err := d.CreateTask(models.CreateTaskRequest{Title: "unconfirmed launch", ProjectID: "default"})
	if err != nil {
		t.Fatal(err)
	}
	if err := d.AddTaskActivity(models.TaskActivity{ID: "unconfirmed", TaskID: task.ID, RunProjectID: "default", UserID: "u", DeviceID: "a", SkillID: "agent_launch", Status: "failed", CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SyncRemoteRunStatusFor("u", "unconfirmed", task.ID, "default", task.Key, "implement", "completed", "late completion", nil); err == nil {
		t.Fatal("status sync confirmed a launch without dispatch acknowledgement")
	}
	if device, err := d.TaskLaunchDevice(task.ID, "default", "u"); err != nil || device != "" {
		t.Fatalf("unconfirmed affinity: %s %v", device, err)
	}
}

func TestTaskAffinityLookupReportsStorageFailure(t *testing.T) {
	d := testDB(t)
	d.Close()
	if _, err := d.TaskLaunchDevice("task", "project", "owner"); err == nil {
		t.Fatal("storage failure treated as no affinity")
	}
}
