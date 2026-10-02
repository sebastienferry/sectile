package db

import (
	"context"
	"slices"
	"strings"
	"testing"

	"tasks/internal/models"
	"tasks/internal/tracker"
	"tasks/internal/trackerapi"
)

// withPrimaryPR gives the two-repository task of stageprs_test.go its primary
// repository's merge request, at the implemented stage.
func withPrimaryPR(t *testing.T, d *DB, task *models.Task) *models.Task {
	t.Helper()
	links := []models.TaskPullRequest{{URL: mrA, Branch: "feat/12"}}
	if _, err := d.conn.Exec(`UPDATE tasks SET pr_links = ?, pr_url = ?, status = 'to_test', labels = '["#implemented"]' WHERE id = ?`, encodePullRequestLinks(links), mrA, task.ID); err != nil {
		t.Fatal(err)
	}
	got, err := d.GetTaskByID(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func linkURLs(links []models.TaskPullRequest) []string {
	var out []string
	for _, link := range links {
		out = append(out, link.URL)
	}
	return out
}

func TestRecordPullRequestAddsASecondaryRepositoryAndKeepsThePrimaryCurrent(t *testing.T) {
	d, task, _ := twoRepoTask(t)
	task = withPrimaryPR(t, d, task)
	got, err := d.RecordPullRequest(context.Background(), "", task.ID, mrB)
	if err != nil {
		t.Fatal(err)
	}
	if urls := linkURLs(got.PrLinks); !slices.Equal(urls, []string{mrB, mrA}) {
		t.Fatalf("links = %v, want the secondary one before the primary one", urls)
	}
	if got.PrURL == nil || *got.PrURL != mrA {
		t.Fatalf("prUrl = %v, want the primary repository's", got.PrURL)
	}
	if got.PrLinks[0].Repository != "gitlab.com/g/b" || got.PrLinks[1].Repository != "gitlab.com/g/a" {
		t.Fatalf("links name no repository: %+v", got.PrLinks)
	}
	stored, _ := d.GetTaskByID(task.ID)
	if !slices.Equal(stored.Labels, task.Labels) || stored.Status != task.Status {
		t.Fatalf("recording a pull request moved the task: %v %s", stored.Labels, stored.Status)
	}
	var raw string
	if err := d.conn.QueryRow("SELECT pr_links FROM tasks WHERE id = ?", task.ID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(raw, "repository") {
		t.Fatalf("the repository is derived, never stored: %s", raw)
	}
}

func TestRecordPullRequestRefusals(t *testing.T) {
	d, task, _ := twoRepoTask(t)
	task = withPrimaryPR(t, d, task)
	for url, want := range map[string]string{
		"https://example.org/x":                     "not a GitHub pull request",
		"https://gitlab.com/g/c/-/merge_requests/3": "not one of the task's repositories",
	} {
		if _, err := d.RecordPullRequest(context.Background(), "", task.ID, url); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v, want %q", url, err, want)
		}
	}
	if got, _ := d.GetTaskByID(task.ID); !slices.Equal(linkURLs(got.PrLinks), []string{mrA}) {
		t.Fatalf("a refused link was written: %v", linkURLs(got.PrLinks))
	}
}

func TestDiscoveryAcceptsASecondaryRepositoryOnAnotherBranch(t *testing.T) {
	d, task, _ := twoRepoTask(t)
	task = withPrimaryPR(t, d, task)
	attached, warnings, err := d.applyDiscoveredPullRequests(task, []models.TaskPullRequest{
		{URL: mrB, Branch: "other"},
		{URL: "https://gitlab.com/g/c/-/merge_requests/3", Branch: "feat/12"},
		{URL: "https://gitlab.com/g/a/-/merge_requests/9", Branch: "unrelated"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(attached, []string{mrB}) || len(warnings) != 2 {
		t.Fatalf("attached = %v, warnings = %v", attached, warnings)
	}
	if got, _ := d.GetTaskByID(task.ID); got.PrURL == nil || *got.PrURL != mrA {
		t.Fatalf("prUrl = %v, want the primary repository's", got.PrURL)
	}
}

func TestSyncFindsTheMissingPullRequestOfAChangedRepository(t *testing.T) {
	d, task, agent := twoRepoTask(t)
	task = withPrimaryPR(t, d, task)
	project, _ := d.GetProjectByID(task.ProjectID)
	ctx := tracker.WithActingUser(context.Background(), "u1")

	steps := d.discoverSecondaryPullRequests(ctx, project, task)
	if len(steps) != 1 || !strings.Contains(steps[0], mrB) {
		t.Fatalf("steps = %v", steps)
	}
	got, _ := d.GetTaskByID(task.ID)
	if !slices.Equal(linkURLs(got.PrLinks), []string{mrB, mrA}) {
		t.Fatalf("links = %v", linkURLs(got.PrLinks))
	}
	agent.set(func(f *fakeRepoAgent) { f.lookups = nil })
	if steps := d.discoverSecondaryPullRequests(ctx, project, got); len(steps) != 0 || len(agent.lookups) != 0 {
		t.Fatalf("a repository with a recorded pull request was looked up again: %v %v", steps, agent.lookups)
	}
}

func TestSyncLeavesSecondaryPullRequestsOfAFinishedOrAnonymousTaskAlone(t *testing.T) {
	d, task, agent := twoRepoTask(t)
	task = withPrimaryPR(t, d, task)
	project, _ := d.GetProjectByID(task.ProjectID)
	if steps := d.discoverSecondaryPullRequests(context.Background(), project, task); steps != nil {
		t.Fatalf("a sync with no acting user looked up: %v", steps)
	}
	if _, err := d.conn.Exec(`UPDATE tasks SET status = 'done', labels = '["#finished"]' WHERE id = ?`, task.ID); err != nil {
		t.Fatal(err)
	}
	finished, _ := d.GetTaskByID(task.ID)
	if steps := d.discoverSecondaryPullRequests(tracker.WithActingUser(context.Background(), "u1"), project, finished); steps != nil {
		t.Fatalf("a finished task was looked up: %v", steps)
	}
	if len(agent.lookups) != 0 {
		t.Fatalf("lookups = %v", agent.lookups)
	}
}

func TestPostBackOfASecondaryPullRequestKeepsThePrimaryCurrent(t *testing.T) {
	d, task, _ := twoRepoTask(t)
	task = withPrimaryPR(t, d, task)
	url := mrB
	got, _, err := d.PostBackTask(models.TaskPostBackPayload{TaskID: task.ID, PrURL: &url})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(linkURLs(got.PrLinks), []string{mrB, mrA}) || got.PrURL == nil || *got.PrURL != mrA {
		t.Fatalf("links = %v, prUrl = %v", linkURLs(got.PrLinks), got.PrURL)
	}
}

func TestStateRefreshWithoutATokenMarksTheLinkInsteadOfWarning(t *testing.T) {
	d, task, _ := twoRepoTask(t)
	task = withPrimaryPR(t, d, task)
	d.trackers = &trackerapi.Client{GitlabURL: "https://gitlab.com/api/v4"}
	if warnings := d.refreshPullRequestStates(context.Background(), task.ProjectID, []models.Task{*task}); len(warnings) != 0 {
		t.Fatalf("warnings = %v", warnings)
	}
	got, _ := d.GetTaskByID(task.ID)
	if got.PrLinks[0].MissingToken != "gitlab" || got.PrLinks[0].State != "" {
		t.Fatalf("link = %+v, want the missing GitLab token named", got.PrLinks[0])
	}
	if err := d.applyPullRequestStates(task.ID, map[string]string{mrA: "open"}, map[string]string{mrA: ""}); err != nil {
		t.Fatal(err)
	}
	if got, _ = d.GetTaskByID(task.ID); got.PrLinks[0].MissingToken != "" || got.PrLinks[0].State != "open" {
		t.Fatalf("a read state must clear the mark: %+v", got.PrLinks[0])
	}
}
