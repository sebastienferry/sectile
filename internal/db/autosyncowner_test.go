package db

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"tasks/internal/models"
)

// A project is owned by whoever created it. The background synchronisation no
// longer reads with the owner's token (#464), but the ownership stays.
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

// Every project created before the column has no owner. Whoever saves it
// adopts it.
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

// A synchronisation reads with the server credential, whoever asked for it
// (#464): somebody clicking Sync is recorded on the activity, never put on the
// tracker call, where it would substitute their personal token.
func TestASynchronisationAskedForBySomebodyRecordsThemAndReadsAsNobody(t *testing.T) {
	fake := newFakeTracker()
	database, project := jiraTestDB(t, fake)

	queued, err := database.EnqueueSyncAs("u-ada", "jira", "", project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if queued.UserID != "u-ada" {
		t.Fatalf("the activity must say who asked, got %q", queued.UserID)
	}

	// Run a job here rather than wait for the worker, which would overwrite
	// what this test is watching. Even a job that names somebody reads as
	// nobody: the worker puts no acting user on a synchronisation.
	activity := models.TaskActivity{ID: "sync-owner", ProjectID: project.ID, SkillID: "sync_jira", Status: "running", CreatedAt: time.Now()}
	if err := database.AddTaskActivity(activity); err != nil {
		t.Fatal(err)
	}
	settings, _ := database.GetSettings()
	fake.syncedAs = "sentinel"
	database.processSyncJob(context.Background(), SkillJob{SkillID: "sync_jira", ActivityID: activity.ID, ProjectID: project.ID, ActingUser: "u-ada"}, settings)
	if fake.syncedAs != "" {
		t.Fatalf("a synchronisation must read as nobody, got %q", fake.syncedAs)
	}
	// And it says so: a write it makes on its own keeps the server credential
	// rather than being refused as one that lost its author (#482).
	if !fake.syncedUnattended {
		t.Fatal("a synchronisation must run marked as unattended work")
	}
}

// The timer's pass is nobody's request, and no longer borrows the owner's
// account: its activity names nobody.
func TestTheAutoSyncPassQueuesUnderNobody(t *testing.T) {
	fake := newFakeTracker()
	// A refused read is kept, which is what lets this assert on the activity.
	fake.syncErr = errors.New("no Jira server credential")
	database, project := autoSyncTestDB(t, fake)
	seedTrackerTask(t, database, project.ID, "PE-1", "Open")

	settings, _ := database.GetSettings()
	database.runAutoSyncPass(settings)

	activities := syncActivities(t, database, project.ID, true)
	if len(activities) == 0 {
		t.Fatal("the pass queued no synchronisation for the project")
	}
	for _, act := range activities {
		if act.UserID != "" {
			t.Fatalf("the pass must queue under nobody, got %q", act.UserID)
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

// The owner's personal token, locked here, used to be what the background pass
// read with, so the pass failed until the owner came back. It now reads with the
// server credential whatever the owner stored (#464).
func TestABackgroundGithubPassReadsWithTheServerTokenWhateverTheOwnerStored(t *testing.T) {
	var mu sync.Mutex
	var authorizations []string
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		authorizations = append(authorizations, r.Header.Get("Authorization"))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	}))
	defer site.Close()

	database := testDB(t)
	database.trackers.HTTP = site.Client()
	database.trackers.GithubURL = site.URL
	database.trackers.GithubToken = "server-token"
	database.auto = &autoSync{}
	enabled := true
	project, err := database.CreateProjectAs("u-ada", models.CreateProjectRequest{Name: "App", IssueTracker: "github", GithubRepo: "acme/app"})
	if err != nil {
		t.Fatal(err)
	}
	if project, err = database.UpdateProjectAs("u-ada", project.ID, models.UpdateProjectRequest{AutoSyncEnabled: &enabled}); err != nil {
		t.Fatal(err)
	}
	task, err := database.CreateTask(models.CreateTaskRequest{Title: "#7", Source: "local", ProjectID: project.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.conn.Exec("UPDATE tasks SET source='github', key='#7', tracker_status='open' WHERE id=?", task.ID); err != nil {
		t.Fatal(err)
	}
	if err := database.SetUserTrackerCredential("u-ada", "github", "", "", "ada-token", "phrase"); err != nil {
		t.Fatal(err)
	}
	database.LockUserTrackerCredential("u-ada", "github")

	settings, _ := database.GetSettings()
	database.runAutoSyncPass(settings)

	// A pass that imported nothing leaves no activity behind, so what shows
	// it finished is GitHub having been asked and nothing left queued.
	for i := 0; i < 100; i++ {
		mu.Lock()
		asked := len(authorizations) > 0
		mu.Unlock()
		pending := false
		activities, err := database.GetProjectActivities(project.ID)
		if err != nil {
			t.Fatal(err)
		}
		for _, act := range activities {
			if act.SkillID == "sync_github" && (act.Status == string(models.ActivityStatusQueued) || act.Status == string(models.ActivityStatusRunning)) {
				pending = true
			}
		}
		if asked && !pending {
			break
		}
		time.Sleep(30 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(authorizations) == 0 {
		t.Fatal("the pass never reached GitHub")
	}
	for _, auth := range authorizations {
		if auth != "Bearer server-token" {
			t.Fatalf("the pass must read with the server token, GitHub saw %v", authorizations)
		}
	}
	activities, err := database.GetProjectActivities(project.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, act := range activities {
		if act.SkillID == "sync_github" && act.Status == string(models.ActivityStatusFailed) {
			t.Fatalf("a locked owner token must not fail the pass: %+v", act)
		}
	}
}
