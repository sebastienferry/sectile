package db

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"tasks/internal/models"
	"tasks/internal/testsqlite"
)

// macroURLs reads the project's macros and returns their addresses by key.
func macroURLs(t *testing.T, d *DB, projectID string) map[string]string {
	t.Helper()
	metas, err := d.GetProjectMacros(projectID)
	if err != nil {
		t.Fatalf("GetProjectMacros: %v", err)
	}
	out := map[string]string{}
	for _, m := range metas {
		out[m.Key] = m.ExternalURL
	}
	return out
}

// On GitHub a macro is a milestone, and its page is the one GitHub gives. A
// macro kept locally because the milestone write failed carries an M-<n> key
// too, and gets no address: building one from its key would open another
// milestone, or none.
func TestGithubMacroAddressIsTheListedMilestone(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/milestones") {
			fmt.Fprint(w, `[
				{"number":3,"title":"Listed","state":"open","html_url":"https://github.example/acme/app/milestone/3"},
				{"number":5,"title":"Without page","state":"open"}
			]`)
			return
		}
		fmt.Fprint(w, `[]`)
	}))
	t.Cleanup(server.Close)
	t.Setenv("SECTILE_GITHUB_API_URL", server.URL)
	t.Setenv("SECTILE_GITHUB_TOKEN", "server-token")

	d, err := testsqlite.New(t, filepath.Join(t.TempDir(), "tasks.db"), NewDB)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	project, err := d.CreateProject(models.CreateProjectRequest{Name: "Milestones", IssueTracker: "github", GithubRepo: "acme/app"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.SaveMacroMeta(project.ID, "M-9", nil, nil, nil, nil); err != nil {
		t.Fatal(err)
	}

	urls := macroURLs(t, d, project.ID)
	if got := urls["M-3"]; got != "https://github.example/acme/app/milestone/3" {
		t.Errorf("M-3 address = %q, want the page GitHub gave", got)
	}
	if got := urls["M-5"]; got != "https://github.com/acme/app/milestone/5" {
		t.Errorf("M-5 address = %q, want the milestone page built from the repository", got)
	}
	if got, ok := urls["M-9"]; !ok || got != "" {
		t.Errorf("local M-9 address = %q (listed %v), want an empty address", got, ok)
	}
}

// Elsewhere the macro's page is the one of the work item carrying its key: a
// Jira epic is synced as a task, kept off the board. A macro no work item
// carries keeps no address.
func TestMacroAddressComesFromTheTaskCarryingItsKey(t *testing.T) {
	d, err := testsqlite.New(t, filepath.Join(t.TempDir(), "tasks.db"), NewDB)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	project, err := d.CreateProject(models.CreateProjectRequest{Name: "Epics", IssueTracker: "local"})
	if err != nil {
		t.Fatal(err)
	}
	page := "https://jira.example/browse/PROJ-7"
	if err := d.ImportOrUpdateTasks([]models.Task{{
		ProjectID: project.ID, Key: "PROJ-7", Title: "The epic", IssueType: "Epic", Source: "jira",
		ExternalURL: &page, Status: models.StatusToClarify, Priority: models.PriorityMedium,
	}}); err != nil {
		t.Fatalf("ImportOrUpdateTasks: %v", err)
	}
	for _, key := range []string{"PROJ-7", "PROJ-8"} {
		if _, err := d.SaveMacroMeta(project.ID, key, nil, nil, nil, nil); err != nil {
			t.Fatal(err)
		}
	}

	urls := macroURLs(t, d, project.ID)
	if got := urls["PROJ-7"]; got != page {
		t.Errorf("PROJ-7 address = %q, want %q", got, page)
	}
	if got := urls["PROJ-8"]; got != "" {
		t.Errorf("PROJ-8 address = %q, want none: no work item carries it", got)
	}
}
