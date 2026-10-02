package db

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"tasks/internal/models"
)

// importLinklessTickets imports n Jira and n GitLab tickets, keys from+1 to from+n, with no stored external_url, as a
// sync that recorded no link leaves them.
func importLinklessTickets(t *testing.T, d *DB, projectID string, from, n int) {
	t.Helper()
	var batch []models.Task
	for i := from + 1; i <= from+n; i++ {
		batch = append(batch,
			models.Task{ID: fmt.Sprintf("jira-%d", i), ProjectID: projectID, Key: fmt.Sprintf("PE-%d", i), Title: "Jira", Source: "jira", Status: models.StatusBacklog, Labels: []string{}},
			models.Task{ID: fmt.Sprintf("gitlab-%d", i), ProjectID: projectID, Key: fmt.Sprintf("#%d", i), Title: "GitLab", Source: "gitlab", Status: models.StatusBacklog, Labels: []string{}})
	}
	if err := d.ImportOrUpdateTasks(batch); err != nil {
		t.Fatal(err)
	}
}

// jiraLinkOf lists the project's board and returns the link of its first Jira ticket.
func jiraLinkOf(t *testing.T, d *DB, projectID string) string {
	t.Helper()
	tasks, err := d.GetTasksInScope(TaskScope{ProjectID: projectID}, "", "", "", "", "", "", "", "", nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, task := range tasks {
		if task.Source == "jira" && task.ExternalURL != nil {
			return *task.ExternalURL
		}
	}
	t.Fatalf("no Jira ticket with a link in %+v", tasks)
	return ""
}

// A board's tracker links cost the same statements whatever the number of tickets without a stored link: one project
// read and at most one settings read per list, never one per row (#486).
func TestTaskListQueryCountIsTheSameForOneAndTwentyTicketsWithoutAStoredLink(t *testing.T) {
	d := testDB(t)
	project, err := d.CreateProject(models.CreateProjectRequest{
		Name:          "Links",
		IssueTracker:  "jira",
		JiraProject:   "PE",
		TrackerUrl:    "https://acme.atlassian.net",
		GitlabUrl:     "https://gitlab.example.org/api/v4",
		GitlabProject: "acme/app",
	})
	if err != nil {
		t.Fatal(err)
	}
	listCost := func() int64 {
		d.projectURLs.clear()
		before := d.conn.queries.Load()
		tasks, err := d.GetTasksInScope(TaskScope{ProjectID: project.ID}, "", "", "", "", "", "", "", "", nil, nil, false)
		cost := d.conn.queries.Load() - before
		if err != nil {
			t.Fatal(err)
		}
		for _, task := range tasks {
			want := "https://acme.atlassian.net/browse/" + task.Key
			if task.Source == "gitlab" {
				want = "https://gitlab.example.org/acme/app/-/issues/" + task.Key[1:]
			}
			if task.ExternalURL == nil || *task.ExternalURL != want {
				t.Fatalf("%s: link %v, want %s", task.Key, task.ExternalURL, want)
			}
		}
		return cost
	}
	importLinklessTickets(t, d, project.ID, 0, 1)
	one := listCost()
	importLinklessTickets(t, d, project.ID, 1, 19)
	twenty := listCost()
	if one != twenty {
		t.Fatalf("listing 1 ticket of each tracker cost %d statements, 20 cost %d", one, twenty)
	}
}

// A project saved on this instance changes its tickets' links on the very next read: the write clears the cache, it
// does not wait for the entry to expire.
func TestAProjectSavedOnThisInstanceChangesItsTicketLinksOnTheNextRead(t *testing.T) {
	d := testDB(t)
	project, err := d.CreateProject(models.CreateProjectRequest{Name: "Links", IssueTracker: "jira", JiraProject: "PE", TrackerUrl: "https://old.atlassian.net"})
	if err != nil {
		t.Fatal(err)
	}
	importLinklessTickets(t, d, project.ID, 0, 1)
	if got, want := jiraLinkOf(t, d, project.ID), "https://old.atlassian.net/browse/PE-1"; got != want {
		t.Fatalf("link before the save %s, want %s", got, want)
	}
	newURL := "https://new.atlassian.net"
	if _, err := d.UpdateProject(project.ID, models.UpdateProjectRequest{TrackerUrl: &newURL}); err != nil {
		t.Fatal(err)
	}
	if got, want := jiraLinkOf(t, d, project.ID), "https://new.atlassian.net/browse/PE-1"; got != want {
		t.Fatalf("link right after the save %s, want %s", got, want)
	}
}

// A project changed by another instance reaches this one's links once the cached entry expires, never later than
// projectURLCacheTTL: the staleness accepted in #486, since nothing tells this instance about the change.
func TestAProjectChangedByAnotherInstanceShowsInItsLinksOnceTheCacheEntryExpires(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.db")
	writer, err := NewDB(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { writer.Close() })
	project, err := writer.CreateProject(models.CreateProjectRequest{Name: "Links", IssueTracker: "jira", JiraProject: "PE", TrackerUrl: "https://old.atlassian.net"})
	if err != nil {
		t.Fatal(err)
	}
	importLinklessTickets(t, writer, project.ID, 0, 1)
	reader, err := NewDB(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reader.Close() })
	now := time.Now()
	reader.projectURLs.now = func() time.Time { return now }
	oldLink, newLink := "https://old.atlassian.net/browse/PE-1", "https://new.atlassian.net/browse/PE-1"
	if got := jiraLinkOf(t, reader, project.ID); got != oldLink {
		t.Fatalf("link before the change %s, want %s", got, oldLink)
	}
	newURL := "https://new.atlassian.net"
	if _, err := writer.UpdateProject(project.ID, models.UpdateProjectRequest{TrackerUrl: &newURL}); err != nil {
		t.Fatal(err)
	}
	now = now.Add(projectURLCacheTTL - time.Second)
	if got := jiraLinkOf(t, reader, project.ID); got != oldLink {
		t.Fatalf("the reader serves its cached fields until they expire: link %s, want %s", got, oldLink)
	}
	now = now.Add(2 * time.Second)
	if got := jiraLinkOf(t, reader, project.ID); got != newLink {
		t.Fatalf("expired fields are read again: link %s, want %s", got, newLink)
	}
}

// A fill that read the database before a clear is not stored after it: it may carry the fields the clearing write replaced.
func TestAProjectURLFillThatStartedBeforeAClearIsNotStored(t *testing.T) {
	var c projectURLCache
	_, gen, _ := c.get("p1")
	c.clear()
	c.put("p1", projectURLFields{TrackerUrl: "https://old.atlassian.net"}, gen)
	if _, _, ok := c.get("p1"); ok {
		t.Fatal("a fill older than the last clear must not be stored")
	}
}
