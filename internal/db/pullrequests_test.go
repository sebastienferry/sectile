package db

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"tasks/internal/agentprotocol"
	"tasks/internal/models"
	"tasks/internal/trackerapi"
)

func TestPullRequestSetOrderAndDeduplication(t *testing.T) {
	links := models.AppendPullRequestLink(nil, "https://forge/pull/1", "ticket")
	links = models.AppendPullRequestLink(links, "https://forge/pull/2", "ticket")
	links = models.AppendPullRequestLink(links, "https://forge/pull/1", "ticket")
	if len(links) != 2 || links[0].URL != "https://forge/pull/1" || links[1].URL != "https://forge/pull/2" {
		t.Fatalf("append order or deduplication broken: %+v", links)
	}
	if got := models.CurrentPullRequest(links); got != "https://forge/pull/2" {
		t.Fatalf("current PR is not the last link: %s", got)
	}
	if pullRequestURLValue(nil) != nil {
		t.Fatal("an empty set must carry no current PR")
	}
	if got := decodePullRequestLinks(encodePullRequestLinks(links)); len(got) != 2 || got[1].Branch != "ticket" {
		t.Fatalf("round trip lost the set: %+v", got)
	}
	if got := decodePullRequestLinks("not json"); got != nil {
		t.Fatalf("unreadable links must read as an empty set, got %+v", got)
	}
}

// A task whose recorded PR was merged must accept the follow-up pushed on the
// same branch. That is the blockage the whole capability exists for.
func TestFollowUpPullRequestOnTheSameBranchIsAccepted(t *testing.T) {
	d, task := taskWithMergedPullRequest(t)
	followUp := trackerapi.PullRequest{URL: "https://forge/pull/2", Branch: "ticket", SHA: "agent-commit", Open: true}
	d.prEvidenceLookup = func(string, string, string) (trackerapi.PullRequest, error) { return followUp, nil }

	got, _, err := d.TransitionTaskStage(task.ID, "implemented", "follow-up work", followUp.URL, "ticket")
	if err != nil {
		t.Fatalf("follow-up PR refused: %v", err)
	}
	if len(got.PrLinks) != 2 || got.PrLinks[0].URL != "https://forge/pull/1" || got.PrLinks[1].URL != followUp.URL {
		t.Fatalf("set did not grow in order: %+v", got.PrLinks)
	}
	if got.PrURL == nil || *got.PrURL != followUp.URL {
		t.Fatalf("current PR is not the follow-up: %+v", got.PrURL)
	}

	reviewed, _, err := d.TransitionTaskStage(task.ID, "reviewed", "review of the follow-up", followUp.URL, "ticket")
	if err != nil || d.StageOfTask(reviewed) != "reviewed" {
		t.Fatalf("reviewed refused after the follow-up: %+v %v", reviewed, err)
	}
	// Recording the same PR twice must not grow the set.
	if len(reviewed.PrLinks) != 2 {
		t.Fatalf("re-recording grew the set: %+v", reviewed.PrLinks)
	}
	stored, err := d.GetTaskByID(task.ID)
	if err != nil || len(stored.PrLinks) != 2 || stored.PrLinks[1].Branch != "ticket" {
		t.Fatalf("set not persisted: %+v %v", stored, err)
	}
}

// The guard the ticket asks to keep: a PR on a branch no recorded link mentions
// is a substitution, not a follow-up.
func TestPullRequestOnAnUnrelatedBranchIsRefused(t *testing.T) {
	d, task := taskWithMergedPullRequest(t)
	unrelated := trackerapi.PullRequest{URL: "https://forge/pull/9", Branch: "other-ticket", SHA: "agent-commit", Open: true}
	d.prEvidenceLookup = func(string, string, string) (trackerapi.PullRequest, error) { return unrelated, nil }

	_, _, err := d.TransitionTaskStage(task.ID, "implemented", "swapped PR", unrelated.URL, "other-ticket")
	if err == nil {
		t.Fatal("a PR on an unrelated branch was accepted")
	}
	// The refusal must name the branch the task actually recorded, or it reads as
	// an unexplained wall like the one this capability removes.
	if !strings.Contains(err.Error(), "ticket") || !strings.Contains(err.Error(), unrelated.URL) {
		t.Fatalf("refusal does not say what it expected: %v", err)
	}
	stored, err := d.GetTaskByID(task.ID)
	if err != nil || len(stored.PrLinks) != 1 || *stored.PrURL != "https://forge/pull/1" {
		t.Fatalf("a refused PR must leave the set untouched: %+v %v", stored, err)
	}
}

// A human corrects a wrong link, or detaches every link, without any direct
// database access.
func TestUpdateTaskEditsThePullRequestSet(t *testing.T) {
	d, task := taskWithMergedPullRequest(t)
	corrected := []models.TaskPullRequest{{URL: "https://forge/pull/7", Branch: "ticket"}}
	got, err := d.UpdateTask(task.ID, models.UpdateTaskRequest{PrLinks: &corrected})
	if err != nil || len(got.PrLinks) != 1 || got.PrURL == nil || *got.PrURL != "https://forge/pull/7" {
		t.Fatalf("correction did not apply: %+v %v", got, err)
	}
	empty := []models.TaskPullRequest{}
	got, err = d.UpdateTask(task.ID, models.UpdateTaskRequest{PrLinks: &empty})
	if err != nil || len(got.PrLinks) != 0 || got.PrURL != nil {
		t.Fatalf("detaching every link did not clear the current PR: %+v %v", got, err)
	}
	stored, err := d.GetTaskByID(task.ID)
	if err != nil || len(stored.PrLinks) != 0 || stored.PrURL != nil {
		t.Fatalf("detachment not persisted: %+v %v", stored, err)
	}
	// The legacy single URL still records a link, so the two never diverge.
	single := "https://forge/pull/8"
	got, err = d.UpdateTask(task.ID, models.UpdateTaskRequest{PrURL: &single})
	if err != nil || len(got.PrLinks) != 1 || got.PrLinks[0].URL != single || *got.PrURL != single {
		t.Fatalf("legacy single URL not recorded in the set: %+v %v", got, err)
	}
}

// A database written before the column existed keeps its single PR, once.
func TestExistingPullRequestURLIsMigratedOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	d, err := NewDB(path)
	if err != nil {
		t.Fatal(err)
	}
	p, err := d.CreateProject(models.CreateProjectRequest{Name: "Migrate", IssueTracker: "local"})
	if err != nil {
		t.Fatal(err)
	}
	task, err := d.CreateTask(models.CreateTaskRequest{ProjectID: p.ID, Title: "legacy"})
	if err != nil {
		t.Fatal(err)
	}
	// The shape a row had before this capability: a single pr_url, no links.
	if _, err = d.conn.Exec("UPDATE tasks SET branch_name='ticket', pr_url='https://forge/pull/1', pr_links='[]' WHERE id=?", task.ID); err != nil {
		t.Fatal(err)
	}
	forgetSchemaVersion(t, d)
	if err = d.Close(); err != nil {
		t.Fatal(err)
	}

	for pass := 1; pass <= 2; pass++ {
		reopened, err := NewDB(path)
		if err != nil {
			t.Fatal(err)
		}
		stored, err := reopened.GetTaskByID(task.ID)
		if err != nil || stored == nil {
			t.Fatalf("pass %d: %v", pass, err)
		}
		if len(stored.PrLinks) != 1 || stored.PrLinks[0].URL != "https://forge/pull/1" || stored.PrLinks[0].Branch != "ticket" {
			t.Fatalf("pass %d: backfill wrong: %+v", pass, stored.PrLinks)
		}
		if stored.PrURL == nil || *stored.PrURL != "https://forge/pull/1" {
			t.Fatalf("pass %d: current PR changed: %+v", pass, stored.PrURL)
		}
		if err = reopened.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

// taskWithMergedPullRequest is an implemented task whose branch PR was merged by
// the human and recorded on the task.
func taskWithMergedPullRequest(t *testing.T) (*DB, *models.Task) {
	t.Helper()
	d, err := NewDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	p, err := d.CreateProject(models.CreateProjectRequest{Name: "Follow-up", IssueTracker: "local"})
	if err != nil {
		t.Fatal(err)
	}
	task, err := d.CreateTask(models.CreateTaskRequest{ProjectID: p.ID, Title: "follow-up", Labels: []string{"#implemented"}})
	if err != nil {
		t.Fatal(err)
	}
	links := encodePullRequestLinks([]models.TaskPullRequest{{URL: "https://forge/pull/1", Branch: "ticket"}})
	if _, err = d.conn.Exec("UPDATE tasks SET branch_name='ticket', status='to_test', labels='[\"#implemented\"]', pr_url='https://forge/pull/1', pr_links=? WHERE id=?", links, task.ID); err != nil {
		t.Fatal(err)
	}
	d.SetAgentOperations(func(ctx context.Context, op agentprotocol.Operation) (json.RawMessage, error) {
		return json.RawMessage(`{"sha":"agent-commit","branch":"ticket","clean":true}`), nil
	})
	stored, err := d.GetTaskByID(task.ID)
	if err != nil || stored == nil {
		t.Fatal(err)
	}
	return d, stored
}
