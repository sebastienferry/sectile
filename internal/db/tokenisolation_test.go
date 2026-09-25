package db

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"tasks/internal/models"
	"tasks/internal/tracker"
	"tasks/internal/trackerapi"
)

// githubRecorder is a GitHub instance that answers every call the store makes
// and remembers which token each one carried. It is what tells a write signed
// by a person from one signed by the server account (#482).
type githubRecorder struct {
	mu       sync.Mutex
	requests []string // "METHOD path token"
	next     int
}

func (g *githubRecorder) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	g.requests = append(g.requests, r.Method+" "+r.URL.Path+" "+token)
	g.next++
	number := 100 + g.next
	g.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/milestones"):
		fmt.Fprint(w, `[{"number":3,"title":"Macro"}]`)
	case strings.HasSuffix(r.URL.Path, "/milestones"):
		fmt.Fprintf(w, `{"number":%d,"title":"Macro"}`, number)
	case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/comments"):
		fmt.Fprint(w, `[]`)
	case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/issues"):
		fmt.Fprint(w, `[]`)
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/issues"):
		fmt.Fprintf(w, `{"number":%d,"title":"T","state":"open","labels":[],"html_url":"https://github.com/acme/app/issues/%d"}`, number, number)
	default:
		fmt.Fprint(w, `{"number":7,"title":"T","state":"open","labels":[]}`)
	}
}

// since returns the requests recorded after the first n.
func (g *githubRecorder) since(n int) []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]string{}, g.requests[n:]...)
}

func (g *githubRecorder) count() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.requests)
}

// isolationFixture is a GitHub project with a server token, a person with a
// token of their own (ada) and a person without one (grace).
type isolationFixture struct {
	d       *DB
	github  *githubRecorder
	project *models.Project
	ada     context.Context
	grace   context.Context
}

func newIsolationFixture(t *testing.T) *isolationFixture {
	t.Helper()
	recorder := &githubRecorder{}
	server := httptest.NewServer(recorder)
	t.Cleanup(server.Close)
	t.Setenv("SECTILE_GITHUB_API_URL", server.URL)
	t.Setenv("SECTILE_GITHUB_TOKEN", "server-token")
	d, err := NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	project, err := d.CreateProject(models.CreateProjectRequest{Name: "Isolation", IssueTracker: "github", GithubRepo: "acme/app"})
	if err != nil {
		t.Fatal(err)
	}
	for _, user := range []string{"usr_ada", "usr_grace"} {
		if err := d.EnsureUser(user); err != nil {
			t.Fatal(err)
		}
	}
	if err := d.SetUserTrackerCredential("usr_ada", "github", "", "", "ada-token", ""); err != nil {
		t.Fatal(err)
	}
	return &isolationFixture{
		d: d, github: recorder, project: project,
		ada:   tracker.WithActingUser(context.Background(), "usr_ada"),
		grace: tracker.WithActingUser(context.Background(), "usr_grace"),
	}
}

// remoteTask creates an issue as ada, the only person who may.
func (f *isolationFixture) remoteTask(t *testing.T, title string) *models.Task {
	t.Helper()
	task, err := f.d.CreateTaskAs(f.ada, models.CreateTaskRequest{ProjectID: f.project.ID, Title: title})
	if err != nil {
		t.Fatal(err)
	}
	f.settle(t)
	return task
}

// settle waits for everything queued to have run, so a request counted after
// it belongs to what the test did next.
func (f *isolationFixture) settle(t *testing.T) {
	t.Helper()
	if !f.d.jobs.drain(10 * time.Second) {
		t.Fatal("the queue did not settle")
	}
}

// finished waits for a queued activity to end and returns it.
func (f *isolationFixture) finished(t *testing.T, id string) *models.TaskActivity {
	t.Helper()
	f.settle(t)
	act, err := f.d.GetActivityByID(id)
	if err != nil || act == nil {
		t.Fatalf("activity %s: %v", id, err)
	}
	return act
}

func signedOnlyBy(t *testing.T, requests []string, token string) {
	t.Helper()
	if len(requests) == 0 {
		t.Fatal("no request reached GitHub")
	}
	for _, request := range requests {
		if !strings.HasSuffix(request, " "+token) {
			t.Fatalf("every request must carry %s, got %v", token, requests)
		}
	}
}

func hasStep(act *models.TaskActivity, fragment string) bool {
	for _, step := range act.Steps {
		if strings.Contains(step, fragment) {
			return true
		}
	}
	return false
}

// A stage report used to be posted with context.Background(), under the server
// account, whoever recorded the stage. It now goes out, with the stage label,
// under the person's own token.
func TestStageReportIsPostedAsThePersonWhoRecordedIt(t *testing.T) {
	f := newIsolationFixture(t)
	task := f.remoteTask(t, "Report")
	before := f.github.count()

	_, act, err := f.d.TransitionTaskStageBy("usr_ada", task.ID, "clarified", "Scope settled", "", "")
	if err != nil {
		t.Fatal(err)
	}
	done := f.finished(t, act.ID)
	if done.Status != string(models.ActivityStatusCompleted) {
		t.Fatalf("the stage must complete: %+v", done)
	}
	requests := f.github.since(before)
	signedOnlyBy(t, requests, "ada-token")
	commented := false
	for _, request := range requests {
		commented = commented || strings.HasPrefix(request, "POST /repos/acme/app/issues/") && strings.Contains(request, "/comments ")
	}
	if !commented {
		t.Fatalf("the report must be posted as a comment: %v", requests)
	}
}

// Without a personal token, the stage is recorded on the board and nothing
// reaches the tracker: no label, no report. The activity fails and says why,
// rather than claiming a report nobody can find.
func TestStageWithoutAPersonalTokenIsKeptLocallyAndFails(t *testing.T) {
	f := newIsolationFixture(t)
	task := f.remoteTask(t, "No token")
	before := f.github.count()

	_, act, err := f.d.TransitionTaskStageBy("usr_grace", task.ID, "clarified", "Scope settled", "", "")
	if err != nil {
		t.Fatal(err)
	}
	done := f.finished(t, act.ID)
	if done.Status != string(models.ActivityStatusFailed) {
		t.Fatalf("the activity must fail: %+v", done)
	}
	if !hasStep(done, "Rapport d'étape non consigné") || !hasStep(done, "Profile → Tracker credentials") {
		t.Fatalf("the failure step must name the missing credential: %v", done.Steps)
	}
	if hasStep(done, "Rapport d'étape consigné sur") {
		t.Fatalf("no step may claim the report was posted: %v", done.Steps)
	}
	if requests := f.github.since(before); len(requests) != 0 {
		t.Fatalf("nothing may reach GitHub, got %v", requests)
	}
	stored, err := f.d.GetTaskByID(task.ID)
	if err != nil || f.d.StageOfTask(stored) != "clarified" {
		t.Fatalf("the stage must be kept locally: %+v %v", stored, err)
	}
}

// A queued write that names neither a person nor unattended work is a write
// that lost its author: it fails instead of going out under the server
// account. Work marked unattended keeps the server token.
func TestQueuedWriteNeedsAPersonOrTheUnattendedMarker(t *testing.T) {
	f := newIsolationFixture(t)
	task := f.remoteTask(t, "Queue")

	before := f.github.count()
	_, act, err := f.d.TransitionTaskStage(task.ID, "clarified", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	done := f.finished(t, act.ID)
	if done.Status != string(models.ActivityStatusFailed) || !strings.Contains(done.Error, trackerapi.ErrNoActingUser.Error()) {
		t.Fatalf("an operation naming nobody must fail: %+v", done)
	}
	if requests := f.github.since(before); len(requests) != 0 {
		t.Fatalf("nothing may reach GitHub, got %v", requests)
	}

	before = f.github.count()
	queued, err := f.d.EnqueueTrackerOp(tracker.WithUnattended(context.Background()), TrackerOp{
		Kind: TrackerOpStage, ProjectID: f.project.ID, TaskID: task.ID, TaskKey: task.Key, Stage: "specified",
	})
	if err != nil {
		t.Fatal(err)
	}
	if done = f.finished(t, queued.ID); done.Status != string(models.ActivityStatusCompleted) {
		t.Fatalf("unattended work must complete: %+v", done)
	}
	signedOnlyBy(t, f.github.since(before), "server-token")
}

// Converting a local ticket creates its issue as the person who asked. Without
// a token of their own, nothing is created and the ticket stays local.
func TestConversionCreatesTheIssueAsThePerson(t *testing.T) {
	f := newIsolationFixture(t)
	local, err := f.d.CreateTask(models.CreateTaskRequest{Title: "Local", Source: "local", ProjectID: f.project.ID})
	if err != nil {
		t.Fatal(err)
	}

	before := f.github.count()
	_, err = f.d.ConvertTaskToRemote(f.grace, local.ID, "github")
	var missing *trackerapi.MissingPersonalCredentialError
	if !errors.As(err, &missing) {
		t.Fatalf("a conversion without a personal token must be refused: %v", err)
	}
	if requests := f.github.since(before); len(requests) != 0 {
		t.Fatalf("nothing may reach GitHub, got %v", requests)
	}
	if stayed, _ := f.d.GetTaskByID(local.ID); stayed == nil || stayed.Source != "local" {
		t.Fatalf("a refused conversion leaves the task local: %+v", stayed)
	}

	converted, err := f.d.ConvertTaskToRemote(f.ada, local.ID, "github")
	if err != nil {
		t.Fatal(err)
	}
	if converted.Source != "github" {
		t.Fatalf("the task must be converted: %+v", converted)
	}
	f.settle(t)
	signedOnlyBy(t, f.github.since(before), "ada-token")
}

// A story created under a macro is created, and put on the macro's milestone,
// as the person who asked.
func TestStoryUnderAMacroIsCreatedAsThePerson(t *testing.T) {
	f := newIsolationFixture(t)
	title := "Macro"
	if _, err := f.d.saveMacroMetaFull(f.project.ID, "M-3", nil, nil, nil, nil, &title, nil, nil); err != nil {
		t.Fatal(err)
	}

	before := f.github.count()
	if _, _, err := f.d.CreateStoryUnderMacro(f.grace, f.project.ID, "M-3", "Story"); err == nil {
		t.Fatal("a story without a personal token must be refused")
	}
	if requests := f.github.since(before); len(requests) != 0 {
		t.Fatalf("nothing may reach GitHub, got %v", requests)
	}

	task, notice, err := f.d.CreateStoryUnderMacro(f.ada, f.project.ID, "M-3", "Story")
	if err != nil || notice != "" {
		t.Fatalf("story: %+v %q %v", task, notice, err)
	}
	requests := f.github.since(before)
	signedOnlyBy(t, requests, "ada-token")
	milestoned := false
	for _, request := range requests {
		milestoned = milestoned || strings.HasPrefix(request, "PATCH /repos/acme/app/issues/")
	}
	if !milestoned {
		t.Fatalf("the story must be put on the milestone: %v", requests)
	}
}

// The GitHub milestones backing macros were written with the server client.
// They are now written as the person, and a refusal keeps the local change
// while saying GitHub was not updated. Listing them stays a read.
func TestMacroMilestonesAreWrittenAsThePerson(t *testing.T) {
	f := newIsolationFixture(t)

	before := f.github.count()
	created, err := f.d.CreateMacro(f.ada, f.project.ID, "Macro", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	title := "Renamed"
	if _, err := f.d.UpdateMacro(f.ada, f.project.ID, created.Key, &title, nil, nil, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := f.d.DeleteMacro(f.ada, f.project.ID, created.Key); err != nil {
		t.Fatal(err)
	}
	signedOnlyBy(t, f.github.since(before), "ada-token")

	before = f.github.count()
	kept, err := f.d.CreateMacro(f.grace, f.project.ID, "Local macro", "", nil)
	var missing *trackerapi.MissingPersonalCredentialError
	if !errors.As(err, &missing) || kept == nil {
		t.Fatalf("a creation without a token is kept locally and refused on GitHub: %+v %v", kept, err)
	}
	renamed := "Still local"
	saved, err := f.d.UpdateMacro(f.grace, f.project.ID, "M-3", &renamed, nil, nil, nil, nil, nil)
	if !errors.As(err, &missing) || saved == nil || saved.Title != renamed {
		t.Fatalf("an edit without a token is saved locally and refused on GitHub: %+v %v", saved, err)
	}
	if err := f.d.DeleteMacro(f.grace, f.project.ID, "M-3"); !errors.As(err, &missing) {
		t.Fatalf("a deletion without a token is refused on GitHub: %v", err)
	}
	for _, request := range f.github.since(before) {
		if !strings.HasPrefix(request, "GET ") {
			t.Fatalf("a refused person may cause no write, got %v", request)
		}
	}

	before = f.github.count()
	if _, err := f.d.GetProjectMacros(f.project.ID); err != nil {
		t.Fatal(err)
	}
	signedOnlyBy(t, f.github.since(before), "server-token")
}

// Migrating a macro to another GitHub repository writes its milestone there:
// without a token of their own, the person moves nothing.
func TestMacroMigrationIsRefusedBeforeAnythingMoves(t *testing.T) {
	f := newIsolationFixture(t)
	target, err := f.d.CreateProject(models.CreateProjectRequest{Name: "Target", IssueTracker: "github", GithubRepo: "acme/other"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.d.SaveMacroMeta(f.project.ID, "M-9", nil, nil, nil, nil); err != nil {
		t.Fatal(err)
	}

	before := f.github.count()
	_, _, err = f.d.MigrateMacro(f.grace, f.project.ID, "M-9", target.ID, true)
	var missing *trackerapi.MissingPersonalCredentialError
	if !errors.As(err, &missing) {
		t.Fatalf("the migration must be refused: %v", err)
	}
	if requests := f.github.since(before); len(requests) != 0 {
		t.Fatalf("nothing may reach GitHub, got %v", requests)
	}
	metas, _ := f.d.GetProjectMacros(f.project.ID)
	found := false
	for _, meta := range metas {
		found = found || meta.Key == "M-9"
	}
	if !found {
		t.Fatal("a refused migration leaves the macro where it was")
	}

	if _, _, err := f.d.MigrateMacro(f.ada, f.project.ID, "M-9", target.ID, false); err != nil {
		t.Fatal(err)
	}
	for _, request := range f.github.since(before) {
		if !strings.HasPrefix(request, "GET ") && !strings.HasSuffix(request, " ada-token") {
			t.Fatalf("a migration write must carry the person's token, got %v", request)
		}
	}
}
