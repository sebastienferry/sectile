package db

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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
// "configure the Jira account e-mail", pass after pass.
func TestTheBackgroundSynchronisationRunsAsTheJobsUser(t *testing.T) {
	fake := newFakeTracker()
	database, project := jiraTestDB(t, fake)

	activity := models.TaskActivity{ID: "sync-owner", ProjectID: project.ID, SkillID: "sync_jira", Status: "running", CreatedAt: time.Now()}
	if err := database.AddTaskActivity(activity); err != nil {
		t.Fatal(err)
	}

	// Run the job here rather than queue it: the worker would run it in
	// parallel and overwrite what this test is watching.
	settings, _ := database.GetSettings()
	database.processSyncJob(context.Background(), SkillJob{SkillID: "sync_jira", ActivityID: activity.ID, ProjectID: project.ID, ActingUser: "u-ada"}, settings)
	if fake.syncedAs != "u-ada" {
		t.Fatalf("the background read must run as the job's user, got %q", fake.syncedAs)
	}

	// A job queued for nobody keeps the server credential.
	fake.syncedAs = "sentinel"
	database.processSyncJob(context.Background(), SkillJob{SkillID: "sync_jira", ActivityID: activity.ID, ProjectID: project.ID}, settings)
	if fake.syncedAs != "" {
		t.Fatalf("an unattended read must name nobody, got %q", fake.syncedAs)
	}
}

// And the pass itself queues under the project's owner, which is the whole
// point: the loop has nobody to ask, so it borrows the account that turned it
// on. The activity carries it, so a refusal says whose credential was refused.
func TestTheAutoSyncPassQueuesUnderTheProjectOwner(t *testing.T) {
	fake := newFakeTracker()
	// A refused read is kept, which is what lets this assert on the activity.
	fake.syncErr = errors.New("configure the Jira account e-mail")
	database, project := autoSyncTestDB(t, fake)
	seedTrackerTask(t, database, project.ID, "PE-1", "Open")

	settings, _ := database.GetSettings()
	database.runAutoSyncPass(settings)

	activities := syncActivities(t, database, project.ID, true)
	if len(activities) == 0 {
		t.Fatal("the pass queued no synchronisation for the project")
	}
	for _, act := range activities {
		if act.UserID != "u-ada" {
			t.Fatalf("the pass must queue under the owner, got %q", act.UserID)
		}
		if act.Status != string(models.ActivityStatusFailed) {
			t.Fatalf("a refused read is kept as a failure, got %q", act.Status)
		}
	}
}

// A pass nobody asked for that finds nothing leaves nothing: it runs every few
// minutes on every project that opted in, and a row per pass would bury the
// entries the feed exists for.
func TestABackgroundPassThatFoundNothingLeavesNoTrace(t *testing.T) {
	fake := newFakeTracker()
	database, project := autoSyncTestDB(t, fake)
	seedTrackerTask(t, database, project.ID, "PE-1", "Open")

	settings, _ := database.GetSettings()
	database.runAutoSyncPass(settings)

	if _, ran := fake.syncedWithin(t); !ran {
		t.Fatal("the pass never reached the tracker")
	}
	if left := syncActivities(t, database, project.ID, false); len(left) != 0 {
		t.Fatalf("a pass that imported nothing must leave nothing behind, got %d activity(ies)", len(left))
	}
}

// On GitHub a sealed owner token nobody unlocked used to fall back on the
// server token: the pass read as the service account while its activity named
// the owner, whose credential was never touched. The read is refused instead,
// and the failure still says whose credential it was (ADR 0018).
func TestABackgroundGithubPassRefusesALockedOwnerToken(t *testing.T) {
	var authorizations []string
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorizations = append(authorizations, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	}))
	defer site.Close()

	database := testDB(t)
	database.trackers.HTTP = site.Client()
	database.trackers.GithubURL = site.URL
	database.trackers.GithubToken = "server-token"
	project, err := database.CreateProjectAs("u-ada", models.CreateProjectRequest{Name: "App", IssueTracker: "github", GithubRepo: "acme/app"})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.SetUserTrackerCredential("u-ada", "github", "", "", "ada-token", "phrase"); err != nil {
		t.Fatal(err)
	}
	database.LockUserTrackerCredential("u-ada", "github")

	activity := models.TaskActivity{ID: "sync-locked", ProjectID: project.ID, SkillID: "sync_github", Status: "running", UserID: "u-ada", CreatedAt: time.Now()}
	if err := database.AddTaskActivity(activity); err != nil {
		t.Fatal(err)
	}
	settings, _ := database.GetSettings()
	database.processSyncJob(context.Background(), SkillJob{SkillID: "sync_github", ActivityID: activity.ID, ProjectID: project.ID, ActingUser: "u-ada", Sync: SyncOptions{Background: true}}, settings)

	for _, auth := range authorizations {
		if strings.Contains(auth, "server-token") {
			t.Fatalf("a locked owner token must not fall back on the server token, GitHub saw %v", authorizations)
		}
	}
	got, err := database.GetActivityByID(activity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != string(models.ActivityStatusFailed) {
		t.Fatalf("a refused read is a failure, got %q", got.Status)
	}
	if got.UserID != "u-ada" {
		t.Fatalf("the failure names the owner whose credential was refused, got %q", got.UserID)
	}
}
