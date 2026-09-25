package trackerapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"tasks/internal/models"
	"tasks/internal/tracker"
	"testing"
)

func TestAdaptersSatisfyTicketingSystem(t *testing.T) {
	c := &Client{}
	var _ tracker.TicketingSystem = NewGithubAdapter(c)
}

func TestNewDefaultRegistryResolvesTrackers(t *testing.T) {
	c := &Client{}
	reg := NewDefaultRegistry(c)

	gh, ok := reg.Get("github")
	if !ok || gh.Name() != "github" {
		t.Errorf("expected github tracker, got %v", gh)
	}

	loc, ok := reg.Get("local")
	if !ok || loc.Name() != "local" {
		t.Errorf("expected local tracker, got %v", loc)
	}

	// Resolution for project
	pGH := &models.Project{IssueTracker: "github", GithubRepo: "owner/repo"}
	resolved, err := reg.ForProject(pGH)
	if err != nil || resolved.Name() != "github" {
		t.Errorf("ForProject(github) failed: %v", err)
	}

	// Resolution for task
	tGH := &models.Task{Source: "github", Key: "#42"}
	resolved, err = reg.ForTask(tGH, nil)
	if err != nil || resolved.Name() != "github" {
		t.Errorf("ForTask(github) failed: %v", err)
	}

}

func TestGithubAdapterCreateAndSync(t *testing.T) {
	calls := 0
	c := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "POST" && r.URL.Path == "/repos/org/repo/issues" {
			fmt.Fprint(w, `{"number":12,"title":"New Feature","body":"Description","state":"open"}`)
			return
		}
		if r.Method == "GET" && r.URL.Path == "/repos/org/repo/issues" {
			fmt.Fprint(w, `[{"number":12,"title":"New Feature","state":"open"}]`)
			return
		}
	})

	adapter := NewGithubAdapter(c)
	proj := &models.Project{IssueTracker: "github", GithubRepo: "org/repo"}

	// Create
	task, err := adapter.CreateIssue(context.Background(), tracker.CreateIssueRequest{
		Project:     proj,
		Title:       "New Feature",
		Description: "Description",
	})
	if err != nil {
		t.Fatalf("CreateIssue failed: %v", err)
	}
	if task.Key != "#12" || task.Title != "New Feature" {
		t.Errorf("unexpected created task: %#v", task)
	}

	// Sync
	tasks, err := adapter.SyncIssues(context.Background(), tracker.SyncRequest{
		Project: proj,
	})
	if err != nil {
		t.Fatalf("SyncIssues failed: %v", err)
	}
	if len(tasks) != 1 || tasks[0].Key != "#12" {
		t.Errorf("unexpected synced tasks: %#v", tasks)
	}
}

func TestFormatTaskID(t *testing.T) {
	c := &Client{}
	gh := NewGithubAdapter(c)
	loc := tracker.NewLocalAdapter()

	// GitHub default project
	if id := gh.FormatTaskID("default", "#42", ""); id != "gh-42" {
		t.Errorf("expected gh-42, got %s", id)
	}
	if id := gh.FormatTaskID("", "#42", "gh-42"); id != "gh-42" {
		t.Errorf("expected gh-42, got %s", id)
	}

	// GitHub custom project
	if id := gh.FormatTaskID("myproj", "#42", ""); id != "gh-myproj-42" {
		t.Errorf("expected gh-myproj-42, got %s", id)
	}
	if id := gh.FormatTaskID("myproj", "42", ""); id != "gh-myproj-42" {
		t.Errorf("expected gh-myproj-42, got %s", id)
	}
	if id := gh.FormatTaskID("myproj", "", "gh-myproj-42"); id != "gh-myproj-42" {
		t.Errorf("expected gh-myproj-42, got %s", id)
	}

	// Local
	if id := loc.FormatTaskID("default", "TASK-1", "custom-id-123"); id != "custom-id-123" {
		t.Errorf("expected local rawID preserved, got %s", id)
	}
	if id := loc.FormatTaskID("default", "TASK-1", ""); id == "" {
		t.Errorf("expected local generated UUID, got empty")
	}
}

// A personal GitHub token was stored, listed as active in the profile, and
// never used: the adapter resolved the server's client and every issue the
// person created went out under the service account. GitHub attributes a
// comment to the account behind the token exactly as Jira does.
func TestGithubWritesUseTheActingPersonsToken(t *testing.T) {
	var seen []string
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"number":7,"title":"T","state":"open","labels":[],"html_url":"https://github.com/acme/app/issues/7"}`)
	}))
	defer site.Close()

	client := NewClient()
	client.HTTP = site.Client()
	client.GithubURL = site.URL
	client.GithubToken = "server-token"
	client.ResolveUser = func(userID, trackerName string) (string, string, string, error) {
		if userID == "u-ada" && trackerName == "github" {
			return "", "", "ada-token", nil
		}
		return "", "", "", nil
	}
	adapter := NewGithubAdapter(client)
	project := &models.Project{ID: "p1", GithubRepo: "acme/app"}

	if _, err := adapter.CreateIssue(tracker.WithActingUser(context.Background(), "u-ada"),
		tracker.CreateIssueRequest{Project: project, Title: "T"}); err != nil {
		t.Fatal(err)
	}
	if len(seen) == 0 || !strings.Contains(seen[0], "ada-token") {
		t.Fatalf("the write must carry the acting person's token, got %v", seen)
	}

	// Work nobody asked for keeps the server's token rather than failing.
	seen = nil
	if _, err := adapter.CreateIssue(context.Background(), tracker.CreateIssueRequest{Project: project, Title: "T"}); err != nil {
		t.Fatal(err)
	}
	if len(seen) == 0 || !strings.Contains(seen[0], "server-token") {
		t.Fatalf("unattended work keeps the server token, got %v", seen)
	}
}

// A sealed GitHub token nobody unlocked fell back on the server token: a
// background pass read as the service account while its activity named the
// owner, and a person's write went out under an account they did not choose.
// A locked credential refuses every call, before anything reaches GitHub.
func TestGithubRefusesALockedPersonalToken(t *testing.T) {
	requests := 0
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	}))
	defer site.Close()

	locked := errors.New("credential sealed and locked")
	client := NewClient()
	client.HTTP = site.Client()
	client.GithubURL = site.URL
	client.GithubToken = "server-token"
	client.ResolveUser = func(userID, trackerName string) (string, string, string, error) {
		return "", "", "", locked
	}
	adapter := NewGithubAdapter(client)
	project := &models.Project{ID: "p1", GithubRepo: "acme/app"}
	ctx := tracker.WithProject(tracker.WithActingUser(context.Background(), "u-ada"), project.ID)

	calls := map[string]func() error{
		"IssuePullRequests": func() error {
			_, err := adapter.IssuePullRequests(ctx, tracker.IssuePullRequestsRequest{Project: project, Key: "#7"})
			return err
		},
		"CreateIssue": func() error {
			_, err := adapter.CreateIssue(ctx, tracker.CreateIssueRequest{Project: project, Title: "T"})
			return err
		},
		"GetIssue": func() error {
			_, err := adapter.GetIssue(ctx, tracker.GetIssueRequest{Project: project, Key: "#7"})
			return err
		},
		"UpdateIssue": func() error {
			return adapter.UpdateIssue(ctx, tracker.UpdateIssueRequest{Project: project, Key: "#7"})
		},
		"DeleteIssue": func() error {
			return adapter.DeleteIssue(ctx, tracker.DeleteIssueRequest{Project: project, Key: "#7"})
		},
		"SyncIssues": func() error {
			_, err := adapter.SyncIssues(ctx, tracker.SyncRequest{Project: project})
			return err
		},
		"AddComment": func() error {
			return adapter.AddComment(ctx, tracker.AddCommentRequest{Project: project, Key: "#7", Body: "b"})
		},
		"GetComments": func() error {
			_, err := adapter.GetComments(ctx, tracker.GetCommentsRequest{Project: project, Key: "#7"})
			return err
		},
		"UpdateLabels": func() error {
			return adapter.UpdateLabels(ctx, "#7", []string{"a"}, nil)
		},
	}
	for name, call := range calls {
		if err := call(); !errors.Is(err, locked) {
			t.Errorf("%s must refuse a locked credential, got %v", name, err)
		}
	}
	if requests != 0 {
		t.Fatalf("a refused call must reach nothing, GitHub received %d request(s)", requests)
	}
}

// Somebody who stored no GitHub token of their own keeps the project's token,
// or the server's when the project has none: that fallback is decided (ADR
// 0018), because a shared token is how GitHub deployments run.
func TestGithubWithoutAPersonalTokenKeepsTheProjectToken(t *testing.T) {
	var seen []string
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"number":7,"title":"T","state":"open","labels":[],"html_url":"https://github.com/acme/app/issues/7"}`)
	}))
	defer site.Close()

	client := NewClient()
	client.HTTP = site.Client()
	client.GithubURL = site.URL
	client.GithubToken = "server-token"
	client.Resolve = func(projectID string) Credentials {
		if projectID == "p-own" {
			return Credentials{GithubToken: "project-token"}
		}
		return Credentials{}
	}
	client.ResolveUser = func(userID, trackerName string) (string, string, string, error) {
		return "", "", "", nil
	}
	adapter := NewGithubAdapter(client)
	ctx := tracker.WithActingUser(context.Background(), "u-grace")

	for project, want := range map[string]string{"p-own": "project-token", "p-shared": "server-token"} {
		seen = nil
		if _, err := adapter.GetIssue(ctx, tracker.GetIssueRequest{Project: &models.Project{ID: project, GithubRepo: "acme/app"}, Key: "#7"}); err != nil {
			t.Fatalf("%s: %v", project, err)
		}
		if len(seen) == 0 || !strings.Contains(seen[0], want) {
			t.Fatalf("%s: a person without a GitHub token keeps %s, got %v", project, want, seen)
		}
	}
}
