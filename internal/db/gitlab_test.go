package db

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"tasks/internal/models"
	"tasks/internal/tracker"
)

// gitlabInstance is a self-managed GitLab the store reaches through the real
// adapter: two issues, one board, and every write recorded with its token.
type gitlabInstance struct {
	mu     sync.Mutex
	writes []gitlabWrite
}

type gitlabWrite struct {
	method, path, token string
	body                map[string]any
}

func (g *gitlabInstance) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.EscapedPath(), "/api/v4")
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		g.mu.Lock()
		g.writes = append(g.writes, gitlabWrite{r.Method, path, token, body})
		g.mu.Unlock()
	}
	switch {
	case r.Method == http.MethodGet && path == "/projects/acme%2Fapp/issues":
		fmt.Fprint(w, `[
			{"iid":1,"title":"Specified","state":"opened","labels":["#specified","team::platform","macro:Core features","parent:M-8","Doing"],"web_url":"https://gitlab.example.org/acme/app/-/issues/1","author":{"username":"ada"}},
			{"iid":2,"title":"Closed","state":"closed","labels":["#clarified"],"web_url":"https://gitlab.example.org/acme/app/-/issues/2"}]`)
	case r.Method == http.MethodGet && path == "/projects/acme%2Fapp/boards":
		fmt.Fprint(w, `[{"id":1,"name":"Dev","lists":[{"id":1,"position":0,"label":{"name":"Doing"}}]}]`)
	case r.Method == http.MethodGet && strings.HasSuffix(path, "/related_merge_requests"):
		fmt.Fprint(w, `[]`)
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/projects/acme%2Fapp/issues/"):
		fmt.Fprint(w, `{"iid":1,"state":"opened","labels":["team::platform"]}`)
	case r.Method == http.MethodGet && path == "/projects/acme%2Fapp/milestones":
		fmt.Fprint(w, `[]`)
	case r.URL.Path == "/api/graphql":
		fmt.Fprint(w, `{"errors":[{"message":"Field 'iterationCadences' doesn't exist on type 'Group'"}]}`)
	case r.Method == http.MethodGet:
		fmt.Fprint(w, `[]`)
	default:
		fmt.Fprint(w, `{}`)
	}
}

func (g *gitlabInstance) writesTo(path string) []gitlabWrite {
	g.mu.Lock()
	defer g.mu.Unlock()
	var out []gitlabWrite
	for _, w := range g.writes {
		if w.path == path {
			out = append(out, w)
		}
	}
	return out
}

type gitlabFixture struct {
	d       *DB
	gitlab  *gitlabInstance
	project *models.Project
	ada     context.Context
}

func newGitlabFixture(t *testing.T) *gitlabFixture {
	t.Helper()
	instance := &gitlabInstance{}
	server := httptest.NewServer(instance)
	t.Cleanup(server.Close)
	t.Setenv("SECTILE_GITLAB_API_URL", server.URL+"/api/v4")
	t.Setenv("SECTILE_GITLAB_TOKEN", "server-token")
	d, err := NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	project, err := d.CreateProject(models.CreateProjectRequest{Name: "App", IssueTracker: "gitlab", GitlabProject: "acme/app"})
	if err != nil {
		t.Fatal(err)
	}
	if err := d.EnsureUser("usr_ada"); err != nil {
		t.Fatal(err)
	}
	if err := d.SetUserTrackerCredential("usr_ada", "gitlab", "", "", "ada-token", ""); err != nil {
		t.Fatal(err)
	}
	return &gitlabFixture{d: d, gitlab: instance, project: project, ada: tracker.WithActingUser(context.Background(), "usr_ada")}
}

func (f *gitlabFixture) finished(t *testing.T, id string) *models.TaskActivity {
	t.Helper()
	if !f.d.jobs.drain(10 * time.Second) {
		t.Fatal("the queue did not settle")
	}
	act, err := f.d.GetActivityByID(id)
	if err != nil || act == nil {
		t.Fatalf("activity %s: %v", id, err)
	}
	return act
}

func (f *gitlabFixture) sync(t *testing.T) []models.Task {
	t.Helper()
	act, err := f.d.EnqueueSync("gitlab", "", f.project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if act.SkillName != "Sync GitLab" {
		t.Errorf("the activity spells GitLab: %q", act.SkillName)
	}
	if done := f.finished(t, act.ID); done.Status != "completed" {
		t.Fatalf("sync: %s %s %v", done.Status, done.Summary, done.Steps)
	}
	tasks, err := f.d.GetTasks("", "", "", "", f.project.ID, "", "", "", "", nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	return tasks
}

func TestGitlabSyncJobImportsThroughTheAdapter(t *testing.T) {
	f := newGitlabFixture(t)
	tasks := f.sync(t)
	byKey := map[string]models.Task{}
	for _, task := range tasks {
		byKey[task.Key] = task
	}
	first, second := byKey["#1"], byKey["#2"]
	if first.ID != "gl-"+f.project.ID+"-1" || first.Source != "gitlab" {
		t.Fatalf("identity: %q %q", first.ID, first.Source)
	}
	if first.Status != models.StatusToImplement || second.Status != models.StatusFinished {
		t.Fatalf("stages: %v %v", first.Status, second.Status)
	}
	if first.Team != "platform" || first.ParentKey != "M-8" || first.ParentTitle != "Core features" || first.TrackerStatus != "Doing" {
		t.Fatalf("mapping: %+v", first)
	}
	if first.ExternalURL == nil || *first.ExternalURL != "https://gitlab.example.org/acme/app/-/issues/1" {
		t.Fatalf("web link: %v", first.ExternalURL)
	}
}

func TestGitlabTeamWriteIsAcceptedAndGithubRefused(t *testing.T) {
	f := newGitlabFixture(t)
	f.sync(t)
	task, _ := f.d.GetTaskByID("gl-" + f.project.ID + "-1")
	if task == nil {
		t.Fatal("task #1 imported")
	}
	act, err := f.d.SetTasksTeam(f.ada, f.project.ID, []string{task.ID}, "data", "data")
	if err != nil {
		t.Fatalf("a GitLab task has a team: %v", err)
	}
	if done := f.finished(t, act.ID); done.Status != "completed" {
		t.Fatalf("team write: %s %v", done.Status, done.Steps)
	}
	writes := f.gitlab.writesTo("/projects/acme%2Fapp/issues/1")
	if len(writes) != 1 || writes[0].body["add_labels"] != "team::data" || writes[0].body["remove_labels"] != "team::platform" || writes[0].token != "ada-token" {
		t.Fatalf("the team label is swapped as the person: %+v", writes)
	}

	github, err := f.d.CreateProject(models.CreateProjectRequest{Name: "Hub", IssueTracker: "github", GithubRepo: "acme/hub"})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.d.ImportOrUpdateTasks([]models.Task{{ID: "gh-" + github.ID + "-5", ProjectID: github.ID, Key: "#5", Title: "Hub", Source: "github", Status: models.StatusToClarify, Priority: models.PriorityMedium}}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.d.SetTasksTeam(f.ada, github.ID, []string{"gh-" + github.ID + "-5"}, "data", "data"); err == nil {
		t.Fatal("a GitHub task still has no team")
	}
}

func TestGitlabMacroIsWrittenAsLabels(t *testing.T) {
	f := newGitlabFixture(t)
	f.sync(t)
	title := "Payments"
	if _, err := f.d.saveMacroMetaFull(f.project.ID, "M-9", nil, nil, nil, nil, &title, nil, nil); err != nil {
		t.Fatal(err)
	}
	var steps []string
	if _, err := f.d.applyTaskMacro(f.ada, "gl-"+f.project.ID+"-1", "M-9", &steps); err != nil {
		t.Fatalf("attach: %v %v", err, steps)
	}
	writes := f.gitlab.writesTo("/projects/acme%2Fapp/issues/1")
	if len(writes) != 1 {
		t.Fatalf("one label write: %+v", writes)
	}
	body := writes[0].body
	if body["add_labels"] != "macro:Payments,parent:M-9" || body["remove_labels"] != "macro:Core features,parent:M-8" || writes[0].token != "ada-token" {
		t.Fatalf("macro labels replace the previous ones as the person: %v", body)
	}
	task, _ := f.d.GetTaskByID("gl-" + f.project.ID + "-1")
	if task.ParentKey != "M-9" {
		t.Fatalf("the attachment is recorded locally: %q", task.ParentKey)
	}
}

func TestGitlabMacroCreatesNoGithubMilestone(t *testing.T) {
	f := newGitlabFixture(t)
	repo := "acme/stale"
	if _, err := f.d.UpdateProject(f.project.ID, models.UpdateProjectRequest{GithubRepo: &repo}); err != nil {
		t.Fatal(err)
	}
	project, _ := f.d.GetProjectByID(f.project.ID)
	if githubMilestoneMacros(project) {
		t.Fatal("a GitLab project's macros are never GitHub milestones, even with a stale repository")
	}
	created, err := f.d.CreateMacro(f.ada, f.project.ID, "Local macro", "", nil)
	if err != nil || created == nil || !strings.HasPrefix(created.Key, "M-") {
		t.Fatalf("a GitLab macro is local: %+v %v", created, err)
	}
}

func TestGitlabExternalURLFallback(t *testing.T) {
	f := newGitlabFixture(t)
	url := "https://gitlab.example.org/api/v4"
	if _, err := f.d.UpdateProject(f.project.ID, models.UpdateProjectRequest{GitlabUrl: &url}); err != nil {
		t.Fatal(err)
	}
	f.d.mu.RLock()
	got := f.d.computeExternalURLUnsafe(&models.Task{ProjectID: f.project.ID, Key: "#12", Source: "gitlab"})
	f.d.mu.RUnlock()
	if got == nil || *got != "https://gitlab.example.org/acme/app/-/issues/12" {
		t.Fatalf("fallback link: %v", got)
	}
}
