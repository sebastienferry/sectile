package trackerapi

import (
	"context"
	"fmt"
	"net/http"
	"tasks/internal/models"
	"tasks/internal/tracker"
	"testing"
)

func TestAdaptersSatisfyTicketingSystem(t *testing.T) {
	c := &Client{}
	var _ tracker.TicketingSystem = NewGithubAdapter(c)
	var _ tracker.TicketingSystem = NewLinearAdapter(c)
}

func TestNewDefaultRegistryResolvesTrackers(t *testing.T) {
	c := &Client{}
	reg := NewDefaultRegistry(c)

	gh, ok := reg.Get("github")
	if !ok || gh.Name() != "github" {
		t.Errorf("expected github tracker, got %v", gh)
	}

	lin, ok := reg.Get("linear")
	if !ok || lin.Name() != "linear" {
		t.Errorf("expected linear tracker, got %v", lin)
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

	pLin := &models.Project{IssueTracker: "linear", LinearTeam: "ENG"}
	resolved, err = reg.ForProject(pLin)
	if err != nil || resolved.Name() != "linear" {
		t.Errorf("ForProject(linear) failed: %v", err)
	}

	// Resolution for task
	tGH := &models.Task{Source: "github", Key: "#42"}
	resolved, err = reg.ForTask(tGH, nil)
	if err != nil || resolved.Name() != "github" {
		t.Errorf("ForTask(github) failed: %v", err)
	}

	tLin := &models.Task{Source: "linear", Key: "ENG-10"}
	resolved, err = reg.ForTask(tLin, nil)
	if err != nil || resolved.Name() != "linear" {
		t.Errorf("ForTask(linear) failed: %v", err)
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
	lin := NewLinearAdapter(c)
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

	// Linear
	if id := lin.FormatTaskID("default", "ENG-10", "c7b508f7-6467-422f-a99f-e60d251d234a"); id != "c7b508f7-6467-422f-a99f-e60d251d234a" {
		t.Errorf("expected linear rawID preserved, got %s", id)
	}
	if id := lin.FormatTaskID("default", "ENG-10", ""); id != "ENG-10" {
		t.Errorf("expected linear key fallback, got %s", id)
	}

	// Local
	if id := loc.FormatTaskID("default", "TASK-1", "custom-id-123"); id != "custom-id-123" {
		t.Errorf("expected local rawID preserved, got %s", id)
	}
	if id := loc.FormatTaskID("default", "TASK-1", ""); id == "" {
		t.Errorf("expected local generated UUID, got empty")
	}
}
