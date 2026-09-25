package db

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tasks/internal/models"
	"tasks/internal/tracker"
)

// discoveryTestDB is a project whose pull requests are created at the
// implemented stage, which is the default, plus one task at a chosen stage. Its
// tracker is the local board so that creating the task stays local; the
// discovery read itself is the injected hook, as the forge one is elsewhere.
func discoveryTestDB(t *testing.T, stageLabel string) (*DB, *models.Project, *models.Task) {
	t.Helper()
	d, err := NewDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	no := false
	p, err := d.CreateProject(models.CreateProjectRequest{Name: "Rediscovery", IssueTracker: "local", GithubRepo: "acme/app", UseWorktrees: &no})
	if err != nil {
		t.Fatal(err)
	}
	task, err := d.CreateTask(models.CreateTaskRequest{ProjectID: p.ID, Title: "PR not synched"})
	if err != nil {
		t.Fatal(err)
	}
	labels := []string{stageLabel}
	if _, err = d.UpdateTask(task.ID, models.UpdateTaskRequest{Labels: &labels}); err != nil {
		t.Fatal(err)
	}
	stored, err := d.GetTaskByID(task.ID)
	if err != nil || stored == nil {
		t.Fatal(err)
	}
	return d, p, stored
}

func discovered(urls ...string) []models.TaskPullRequest {
	var links []models.TaskPullRequest
	for _, url := range urls {
		links = append(links, models.TaskPullRequest{URL: url, Branch: "feat/42"})
	}
	return links
}

// US1: an instance that holds no link at all is seeded by discovery, oldest
// first, and the current pull request is the newest one.
func TestDiscoverySeedsATaskWithNoLink(t *testing.T) {
	d, proj, task := discoveryTestDB(t, "#implemented")
	d.prDiscoveryLookup = func(projectID, key string) ([]models.TaskPullRequest, error) {
		if projectID != proj.ID || key != task.Key {
			t.Errorf("discovery asked for the wrong work item: %s %s", projectID, key)
		}
		return discovered("https://forge/pull/1", "https://forge/pull/2"), nil
	}

	steps, halt := d.rediscoverPullRequests(context.Background(), proj, nil, task, false)
	if halt {
		t.Fatal("a nominal discovery must not halt the pass")
	}
	stored, err := d.GetTaskByID(task.ID)
	if err != nil || len(stored.PrLinks) != 2 {
		t.Fatalf("the set was not seeded: %+v %v", stored, err)
	}
	if stored.PrLinks[0].URL != "https://forge/pull/1" || stored.PrLinks[1].URL != "https://forge/pull/2" {
		t.Fatalf("seeding lost the oldest-first order: %+v", stored.PrLinks)
	}
	if stored.PrURL == nil || *stored.PrURL != "https://forge/pull/2" {
		t.Fatalf("the current pull request is not the newest: %+v", stored.PrURL)
	}
	// OPEN-4: the branch seeds an empty branch_name so the branch lookup works
	// from the next synchronisation onwards.
	if stored.BranchName == nil || *stored.BranchName != "feat/42" {
		t.Fatalf("an empty branch was not seeded: %+v", stored.BranchName)
	}
	// US4: what was attached is said, by task and by URL.
	if len(steps) != 1 || !strings.Contains(steps[0], task.Key) || !strings.Contains(steps[0], "https://forge/pull/2") {
		t.Fatalf("the attachment was not reported: %+v", steps)
	}
}

// An issue with no pull request leaves the task exactly as it was, and says
// nothing rather than reporting an empty attachment.
func TestDiscoveryOfNothingWritesNothing(t *testing.T) {
	d, proj, task := discoveryTestDB(t, "#implemented")
	d.prDiscoveryLookup = func(string, string) ([]models.TaskPullRequest, error) { return nil, nil }

	steps, _ := d.rediscoverPullRequests(context.Background(), proj, nil, task, false)
	stored, err := d.GetTaskByID(task.ID)
	if err != nil || len(stored.PrLinks) != 0 || stored.PrURL != nil || len(steps) != 0 {
		t.Fatalf("an issue with no pull request must change nothing: %+v %+v %v", stored, steps, err)
	}
}

// US2: an already recorded URL produces no write, no duplicate and no noise.
func TestDiscoveryOfAKnownPullRequestChangesNothing(t *testing.T) {
	d, proj, task := discoveryTestDB(t, "#implemented")
	known := []models.TaskPullRequest{{URL: "https://forge/pull/1", Branch: "feat/42"}}
	if _, err := d.UpdateTask(task.ID, models.UpdateTaskRequest{PrLinks: &known}); err != nil {
		t.Fatal(err)
	}
	task, _ = d.GetTaskByID(task.ID)
	d.prDiscoveryLookup = func(string, string) ([]models.TaskPullRequest, error) {
		return discovered("https://forge/pull/1"), nil
	}

	// Forced, because the gate alone would already skip a task that holds links.
	steps, _ := d.rediscoverPullRequests(context.Background(), proj, nil, task, true)
	stored, err := d.GetTaskByID(task.ID)
	if err != nil || len(stored.PrLinks) != 1 || *stored.PrURL != "https://forge/pull/1" || len(steps) != 0 {
		t.Fatalf("a known pull request must leave the set untouched: %+v %+v %v", stored, steps, err)
	}
}

// US2: a follow-up on a branch the task already used is appended and becomes
// the current pull request; a candidate on an unrelated branch is refused and
// reported as a warning rather than written.
func TestDiscoveryAppendsAFollowUpAndRefusesASubstitution(t *testing.T) {
	d, proj, task := discoveryTestDB(t, "#implemented")
	known := []models.TaskPullRequest{{URL: "https://forge/pull/1", Branch: "feat/42"}}
	if _, err := d.UpdateTask(task.ID, models.UpdateTaskRequest{PrLinks: &known}); err != nil {
		t.Fatal(err)
	}
	task, _ = d.GetTaskByID(task.ID)
	d.prDiscoveryLookup = func(string, string) ([]models.TaskPullRequest, error) {
		return []models.TaskPullRequest{
			{URL: "https://forge/pull/2", Branch: "feat/42"},
			{URL: "https://forge/pull/9", Branch: "feat/other"},
		}, nil
	}

	steps, _ := d.rediscoverPullRequests(context.Background(), proj, nil, task, true)
	stored, err := d.GetTaskByID(task.ID)
	if err != nil || len(stored.PrLinks) != 2 || *stored.PrURL != "https://forge/pull/2" {
		t.Fatalf("the follow-up was not appended: %+v %v", stored, err)
	}
	var refusal, attachment bool
	for _, step := range steps {
		if strings.Contains(step, "https://forge/pull/9") {
			refusal = true
		}
		if strings.Contains(step, "https://forge/pull/2") && strings.Contains(step, "🔗") {
			attachment = true
		}
	}
	if !refusal || !attachment {
		t.Fatalf("the refusal and the attachment must both be reported: %+v", steps)
	}
}

// US3: a discovery failure is a warning, never a write and never a sync error.
func TestDiscoveryFailureLeavesTheTaskUntouched(t *testing.T) {
	d, proj, task := discoveryTestDB(t, "#implemented")
	d.auto = &autoSync{}
	d.prDiscoveryLookup = func(string, string) ([]models.TaskPullRequest, error) {
		return nil, fmt.Errorf("API rate limit exceeded (429)")
	}

	steps, halt := d.rediscoverPullRequests(context.Background(), proj, nil, task, false)
	if !halt {
		t.Fatal("a rate limit must stop the discoveries of the pass")
	}
	stored, err := d.GetTaskByID(task.ID)
	if err != nil || len(stored.PrLinks) != 0 || stored.PrURL != nil {
		t.Fatalf("a failed discovery must write nothing: %+v %v", stored, err)
	}
	if len(steps) != 1 || !strings.Contains(steps[0], "⚠️") {
		t.Fatalf("the failure must be reported as a warning: %+v", steps)
	}
	// And the loop steps back, as it does for any rate-limited tracker call.
	if status := d.AutoSyncStatus(); status.BackoffUntil == "" {
		t.Fatal("a rate limit must enter the auto-sync backoff")
	}
}

// A token the tracker refuses halts the pass too: the same refusal repeated
// once per ticket is the noise this avoids.
func TestDiscoveryHaltsOnARefusedCredential(t *testing.T) {
	d, proj, task := discoveryTestDB(t, "#implemented")
	d.prDiscoveryLookup = func(string, string) ([]models.TaskPullRequest, error) {
		return nil, fmt.Errorf("tracker request failed: 403 Forbidden")
	}
	if _, halt := d.rediscoverPullRequests(context.Background(), proj, nil, task, false); !halt {
		t.Fatal("a refused credential must stop the discoveries of the pass")
	}
}

// US3: a tracker that cannot answer is skipped before any call, and the local
// board is exactly that tracker.
func TestDiscoveryIsSkippedWithoutTheCapability(t *testing.T) {
	d, proj, task := discoveryTestDB(t, "#implemented")
	local := tracker.NewLocalAdapter()
	if d.pullRequestDiscoverer(local) != nil {
		t.Fatal("a tracker without the capability must not be asked")
	}
	steps, halt := d.rediscoverPullRequests(context.Background(), proj, local, task, false)
	if len(steps) != 0 || halt {
		t.Fatalf("a tracker without the capability must be silent: %+v", steps)
	}
}

// The bounding rule: a task below the pull request creation stage, one that
// already holds links, and one whose links a human detached are all skipped —
// and a rediscovery a person asks for goes through anyway.
func TestDiscoveryGate(t *testing.T) {
	d, proj, task := discoveryTestDB(t, "#specified")
	calls := 0
	d.prDiscoveryLookup = func(string, string) ([]models.TaskPullRequest, error) {
		calls++
		return discovered("https://forge/pull/1"), nil
	}

	if reason := d.pullRequestDiscoveryGate(proj, task, false); reason != discoveryBeforeCreation {
		t.Fatalf("a specified task must not cost a discovery call: %q", reason)
	}
	if _, _ = d.rediscoverPullRequests(context.Background(), proj, nil, task, false); calls != 0 {
		t.Fatalf("the gate let a call through: %d", calls)
	}
	// US5: the person asked, so the gate steps aside.
	if _, _ = d.rediscoverPullRequests(context.Background(), proj, nil, task, true); calls != 1 {
		t.Fatalf("a forced rediscovery must run: %d", calls)
	}

	// Now the task holds a link, so the automatic pass has nothing to repair.
	task, _ = d.GetTaskByID(task.ID)
	if reason := d.pullRequestDiscoveryGate(proj, task, false); reason != discoveryAlreadyLinked {
		t.Fatalf("a task that holds links must be skipped: %q", reason)
	}

	// OPEN-2: a human detaches every link, and the automatic pass respects it.
	empty := []models.TaskPullRequest{}
	if _, err := d.UpdateTask(task.ID, models.UpdateTaskRequest{PrLinks: &empty}); err != nil {
		t.Fatal(err)
	}
	task, _ = d.GetTaskByID(task.ID)
	implemented := []string{"#implemented"}
	if _, err := d.UpdateTask(task.ID, models.UpdateTaskRequest{Labels: &implemented}); err != nil {
		t.Fatal(err)
	}
	task, _ = d.GetTaskByID(task.ID)
	if reason := d.pullRequestDiscoveryGate(proj, task, false); reason != discoveryDetached {
		t.Fatalf("a deliberate detachment must silence the automatic pass: %q", reason)
	}
	calls = 0
	if _, _ = d.rediscoverPullRequests(context.Background(), proj, nil, task, false); calls != 0 {
		t.Fatalf("a detached task was rediscovered: %d", calls)
	}
	// But the person can still ask, and attaching a link forgets the gesture.
	if _, _ = d.rediscoverPullRequests(context.Background(), proj, nil, task, true); calls != 1 {
		t.Fatalf("a forced rediscovery must ignore the detachment: %d", calls)
	}
	if d.pullRequestLinksDetached(task.ID) {
		t.Fatal("attaching a link again must clear the detachment")
	}
}

// A project whose pull requests are created at the specification stage
// discovers from that stage on.
func TestDiscoveryGateFollowsTheProjectCreationStage(t *testing.T) {
	d, proj, task := discoveryTestDB(t, "#specified")
	specified := "specified"
	if _, err := d.UpdateProject(proj.ID, models.UpdateProjectRequest{PRCreationStage: &specified}); err != nil {
		t.Fatal(err)
	}
	proj, _ = d.GetProjectByID(proj.ID)
	if reason := d.pullRequestDiscoveryGate(proj, task, false); reason != discoveryAllowed {
		t.Fatalf("a specified task must be discovered on such a project: %q", reason)
	}
}

// US1 end to end: a full synchronisation of a project whose tasks hold no link
// attaches them and lists them in the activity steps.
func TestFullSyncRediscoversPullRequests(t *testing.T) {
	fake := newFakeTracker()
	fake.tasks = []models.Task{{
		Key: "PE-1", Title: "Imported", Status: models.StatusToTest, Labels: []string{"#implemented"},
		Source: "jira", TrackerStatus: "In Review", CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}}
	database, project := jiraTestDB(t, fake)
	database.prDiscoveryLookup = func(projectID, key string) ([]models.TaskPullRequest, error) {
		if key != "PE-1" {
			return nil, nil
		}
		return discovered("https://forge/pull/1"), nil
	}

	activity := models.TaskActivity{ID: "sync-prs", TaskID: "sync-" + project.ID, SkillID: "sync_jira", Status: "running", CreatedAt: time.Now()}
	if err := database.AddTaskActivity(activity); err != nil {
		t.Fatal(err)
	}
	settings, _ := database.GetSettings()
	database.processSyncJob(context.Background(), SkillJob{SkillID: "sync_jira", ActivityID: activity.ID, ProjectID: project.ID}, settings)

	result, err := database.GetActivityByID(activity.ID)
	if err != nil || result.Status != "completed" {
		t.Fatalf("the sync did not complete: %+v %v", result, err)
	}
	task, err := database.GetTaskByID("jira-" + project.ID + "-PE-1")
	if err != nil || task == nil || task.PrURL == nil || *task.PrURL != "https://forge/pull/1" {
		t.Fatalf("the full sync did not rediscover the pull request: %+v %v", task, err)
	}
	var reported bool
	for _, step := range result.Steps {
		if strings.Contains(step, "PE-1") && strings.Contains(step, "https://forge/pull/1") {
			reported = true
		}
	}
	if !reported {
		t.Fatalf("the sync activity does not say what was attached: %+v", result.Steps)
	}
}
