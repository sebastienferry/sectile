package db

import (
	"context"
	"testing"
	"time"

	"tasks/internal/models"
)

// A project is owned by whoever created it. On a tracker whose credential is
// personal that is what the background synchronisation reads with: it is
// nobody's request, so it has no acting user of its own.
func TestAProjectIsOwnedByItsCreator(t *testing.T) {
	database := testDB(t)

	owned, err := database.CreateProjectAs("u-ada", models.CreateProjectRequest{Name: "Owned", IssueTracker: "jira", JiraProject: "PE"})
	if err != nil {
		t.Fatal(err)
	}
	if owned.OwnerUserID != "u-ada" {
		t.Fatalf("the creator owns the project, got %q", owned.OwnerUserID)
	}

	// Saving somebody else's project does not hand them its synchronisation.
	saved, err := database.UpdateProjectAs("u-grace", owned.ID, models.UpdateProjectRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if saved.OwnerUserID != "u-ada" {
		t.Fatalf("an owner is never replaced, got %q", saved.OwnerUserID)
	}
}

// Every project created before the column has no owner, and would keep reading
// under the server credential for good. Whoever saves it adopts it.
func TestAnOwnerlessProjectAdoptsWhoeverSavesIt(t *testing.T) {
	database := testDB(t)

	orphan, err := database.CreateProject(models.CreateProjectRequest{Name: "Orphan", IssueTracker: "jira", JiraProject: "PE"})
	if err != nil {
		t.Fatal(err)
	}
	if orphan.OwnerUserID != "" {
		t.Fatalf("a project nobody signed for has no owner, got %q", orphan.OwnerUserID)
	}

	adopted, err := database.UpdateProjectAs("u-grace", orphan.ID, models.UpdateProjectRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if adopted.OwnerUserID != "u-grace" {
		t.Fatalf("the person saving it adopts it, got %q", adopted.OwnerUserID)
	}
}

// The queue outlives the pass that filled it, so the account travels on the
// job. Without it the worker resolved the server credential, and a deployment
// whose only Jira credential is personal failed every one of these reads with
// "configure the Jira account e-mail", one per unfinished work item, per pass.
func TestTheBackgroundReadOfOneTaskRunsAsTheJobsUser(t *testing.T) {
	fake := newFakeTracker()
	database, project := jiraTestDB(t, fake)

	task, err := database.CreateTask(models.CreateTaskRequest{Title: "Seed", Source: "local", ProjectID: project.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.conn.Exec("UPDATE tasks SET source='jira', key='PE-1' WHERE id=?", task.ID); err != nil {
		t.Fatal(err)
	}

	activity := models.TaskActivity{ID: "sync-owner", TaskID: task.ID, SkillID: "sync_task", Status: "running", CreatedAt: time.Now()}
	if err := database.AddTaskActivity(activity); err != nil {
		t.Fatal(err)
	}

	// Run the job here rather than queue it: the worker would run it in
	// parallel and overwrite what this test is watching.
	database.processSyncTaskJob(context.Background(), SkillJob{SkillID: "sync_task", ActivityID: activity.ID, TaskID: task.ID, ProjectID: project.ID, ActingUser: "u-ada"})
	if fake.readAs != "u-ada" {
		t.Fatalf("the background read must run as the job's user, got %q", fake.readAs)
	}

	// A job queued for nobody keeps the server credential.
	fake.readAs = "sentinel"
	database.processSyncTaskJob(context.Background(), SkillJob{SkillID: "sync_task", ActivityID: activity.ID, TaskID: task.ID, ProjectID: project.ID})
	if fake.readAs != "" {
		t.Fatalf("an unattended read must name nobody, got %q", fake.readAs)
	}
}

// And the pass itself queues under the project's owner, which is the whole
// point: the loop has nobody to ask, so it borrows the account that turned it
// on. The activity carries it, so a refusal says whose credential was refused.
func TestTheAutoSyncPassQueuesUnderTheProjectOwner(t *testing.T) {
	fake := newFakeTracker()
	database, project := jiraTestDB(t, fake)
	database.auto = &autoSync{lastFullSync: map[string]time.Time{}, lastPassAt: map[string]time.Time{}}

	enabled := true
	if _, err := database.UpdateProjectAs("u-ada", project.ID, models.UpdateProjectRequest{AutoSyncEnabled: &enabled}); err != nil {
		t.Fatal(err)
	}
	task, err := database.CreateTask(models.CreateTaskRequest{Title: "Unfinished", Source: "local", ProjectID: project.ID})
	if err != nil {
		t.Fatal(err)
	}

	settings, _ := database.GetSettings()
	database.runAutoSyncPass(settings)

	activities, err := database.GetTaskActivities(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	queued := 0
	for _, act := range activities {
		if act.SkillID != "sync_task" {
			continue
		}
		queued++
		if act.UserID != "u-ada" {
			t.Fatalf("the pass must queue under the owner, got %q", act.UserID)
		}
	}
	if queued == 0 {
		t.Fatal("the pass queued no synchronisation for the unfinished work item")
	}
}
