package db

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tasks/internal/models"
	"tasks/internal/tracker"
	"tasks/internal/trackerapi"
)

func TestPullRequestStateRefreshBatchesWithoutStoryReads(t *testing.T) {
	d, p, first := discoveryTestDB(t, "#implemented")
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/graphql" {
			t.Errorf("unexpected issue read: %s", r.URL)
		}
		var req struct{ Query string }
		json.NewDecoder(r.Body).Decode(&req)
		n := strings.Count(req.Query, "pullRequest(number:")
		if n != 2 {
			t.Errorf("expected one batch of two links, got %d", n)
		}
		fmt.Fprint(w, `{"data":{"p0":{"pullRequest":{"state":"MERGED","mergeable":"UNKNOWN"}},"p1":{"pullRequest":{"state":"OPEN","mergeable":"CONFLICTING"}}}}`)
	}))
	defer server.Close()
	d.trackers = &trackerapi.Client{GithubURL: server.URL, GithubToken: "test", HTTP: server.Client()}
	second, err := d.CreateTask(models.CreateTaskRequest{ProjectID: p.ID, Title: "second"})
	if err != nil {
		t.Fatal(err)
	}
	for i, task := range []*models.Task{first, second} {
		links := []models.TaskPullRequest{{URL: fmt.Sprintf("%s/a/b/pull/%d", server.URL, i+1), Branch: "feat/233"}}
		d.conn.Exec("UPDATE tasks SET pr_links = ?, pr_url = ? WHERE id = ?", encodePullRequestLinks(links), links[0].URL, task.ID)
	}
	warnings := d.refreshProjectPullRequestStates(context.Background(), p.ID)
	if len(warnings) != 0 || calls != 1 {
		t.Fatalf("calls=%d warnings=%v", calls, warnings)
	}
	for _, task := range []*models.Task{first, second} {
		got, _ := d.GetTaskByID(task.ID)
		if len(got.PrLinks) != 1 || got.PrLinks[0].State == "" || got.Status != task.Status {
			t.Fatalf("unexpected task: %+v", got)
		}
	}
}

func TestPullRequestStateWritePreservesOrderAndDetachment(t *testing.T) {
	d, _, task := discoveryTestDB(t, "#implemented")
	links := []models.TaskPullRequest{{URL: "https://forge/pull/1", Branch: "old", State: "open"}, {URL: "https://forge/pull/2", Branch: "new", State: "conflicting"}}
	d.conn.Exec("UPDATE tasks SET pr_links = ?, pr_url = ? WHERE id = ?", encodePullRequestLinks(links), links[1].URL, task.ID)
	if err := d.applyPullRequestStates(task.ID, map[string]string{links[0].URL: "merged", links[1].URL: "", "https://forge/pull/3": "open"}); err != nil {
		t.Fatal(err)
	}
	got, _ := d.GetTaskByID(task.ID)
	if len(got.PrLinks) != 2 || got.PrLinks[0].State != "merged" || got.PrLinks[1].State != "conflicting" || *got.PrURL != links[1].URL || got.PrLinks[0].Branch != "old" {
		t.Fatalf("links changed: %+v", got.PrLinks)
	}
	d.conn.Exec("UPDATE tasks SET pr_links = '[]', pr_url = NULL WHERE id = ?", task.ID)
	d.applyPullRequestStates(task.ID, map[string]string{links[0].URL: "merged"})
	got, _ = d.GetTaskByID(task.ID)
	if len(got.PrLinks) != 0 || got.PrURL != nil {
		t.Fatal("refresh resurrected detached PR")
	}
}

func TestPullRequestRefreshPreservesStateOnLockedCredential(t *testing.T) {
	d, p, task := discoveryTestDB(t, "#implemented")
	link := models.TaskPullRequest{URL: "https://github.com/a/b/pull/1", State: "merged"}
	d.conn.Exec("UPDATE tasks SET pr_links = ?, pr_url = ? WHERE id = ?", encodePullRequestLinks([]models.TaskPullRequest{link}), link.URL, task.ID)
	d.trackers = &trackerapi.Client{GithubURL: trackerapi.DefaultGithubURL, GithubToken: "must-not-fallback", ResolveUser: func(string, string) (string, string, string, error) {
		return "", "", "", fmt.Errorf("locked credential")
	}}
	warnings := d.refreshProjectPullRequestStates(tracker.WithActingUser(context.Background(), "owner"), p.ID)
	got, _ := d.GetTaskByID(task.ID)
	if len(warnings) == 0 || got.PrLinks[0].State != "merged" {
		t.Fatalf("warnings=%v links=%+v", warnings, got.PrLinks)
	}
}

func TestEditingOneStoryRefreshesOnlyItsPRMetadata(t *testing.T) {
	d, p, task := discoveryTestDB(t, "#implemented")
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "POST" || r.URL.Path != "/graphql" {
			t.Errorf("unexpected story synchronization: %s %s", r.Method, r.URL)
		}
		var req struct{ Query string }
		json.NewDecoder(r.Body).Decode(&req)
		if strings.Count(req.Query, "pullRequest(number:") != 1 || !strings.Contains(req.Query, "number:1") {
			t.Errorf("refreshed unrelated links: %s", req.Query)
		}
		fmt.Fprint(w, `{"data":{"p0":{"pullRequest":{"state":"CLOSED","mergeable":"UNKNOWN"}}}}`)
	}))
	defer server.Close()
	d.trackers = &trackerapi.Client{GithubURL: server.URL, GithubToken: "test", HTTP: server.Client()}
	other, err := d.CreateTask(models.CreateTaskRequest{ProjectID: p.ID, Title: "unrelated"})
	if err != nil {
		t.Fatal(err)
	}
	for i, item := range []*models.Task{task, other} {
		link := models.TaskPullRequest{URL: fmt.Sprintf("%s/a/b/pull/%d", server.URL, i+1), State: "open"}
		d.conn.Exec("UPDATE tasks SET pr_links = ?, pr_url = ? WHERE id = ?", encodePullRequestLinks([]models.TaskPullRequest{link}), link.URL, item.ID)
	}
	branch := "feat/233"
	updated, err := d.UpdateTaskBy(Actor{}, task.ID, models.UpdateTaskRequest{BranchName: &branch})
	if err != nil || calls != 1 || updated.PrLinks[0].State != "closed" {
		t.Fatalf("calls=%d task=%+v err=%v", calls, updated, err)
	}
	untouched, _ := d.GetTaskByID(other.ID)
	if untouched.PrLinks[0].State != "open" {
		t.Fatal("changed unrelated story")
	}
	var count int
	d.conn.QueryRow("SELECT COUNT(*) FROM task_activities WHERE skill_id = 'sync_task'").Scan(&count)
	if count != 0 {
		t.Fatalf("created %d unitary story syncs", count)
	}
}

func TestEditingLinksCannotForgeOrEraseObservedState(t *testing.T) {
	d, _, task := discoveryTestDB(t, "#implemented")
	saved := []models.TaskPullRequest{{URL: "https://unsupported.test/a/b/pull/1", State: "merged"}}
	d.conn.Exec("UPDATE tasks SET pr_links = ?, pr_url = ? WHERE id = ?", encodePullRequestLinks(saved), saved[0].URL, task.ID)
	edited := []models.TaskPullRequest{{URL: saved[0].URL}, {URL: "https://unsupported.test/a/b/pull/2", State: "merged"}}
	got, err := d.UpdateTask(task.ID, models.UpdateTaskRequest{PrLinks: &edited})
	if err != nil {
		t.Fatal(err)
	}
	if got.PrLinks[0].State != "merged" || got.PrLinks[1].State != "" {
		t.Fatalf("untrusted states applied: %+v", got.PrLinks)
	}
}
