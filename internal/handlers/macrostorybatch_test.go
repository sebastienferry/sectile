package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tasks/internal/db"
	"tasks/internal/models"
	"tasks/internal/taskmcp"
)

func postMacroStories(h *Handler, projectID, body string, authorize func(*http.Request)) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/projects/"+projectID+"/macros/M-1/stories", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if authorize != nil {
		authorize(req)
	}
	rr := httptest.NewRecorder()
	h.HandleProjectDetail(rr, req)
	return rr
}

// The batch answers 200 with one outcome per line and the counts (#634).
func TestMacroStoriesBatchRoute(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Board", Slug: "board", IssueTracker: "local"})
	if err != nil {
		t.Fatal(err)
	}
	todos := []models.MacroTodo{{ID: "a", Text: "First"}, {ID: "b", Text: "Lost", TargetProjectID: "gone"}}
	if _, err := database.SaveMacroMeta(project.ID, "M-1", nil, nil, nil, &todos); err != nil {
		t.Fatal(err)
	}

	rr := postMacroStories(h, project.ID, `{"todoIds":["b","a"]}`, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("got %d %s", rr.Code, rr.Body.String())
	}
	var batch db.MacroStoryBatch
	if err := json.Unmarshal(rr.Body.Bytes(), &batch); err != nil {
		t.Fatal(err)
	}
	if batch.Created != 1 || batch.Failed != 1 || len(batch.Results) != 2 || batch.Results[0].TodoID != "a" || batch.Results[0].StoryKey == "" {
		t.Fatalf("unexpected batch %+v", batch)
	}
	if batch.Macro == nil || batch.Macro.Todos[0].StoryKey != batch.Results[0].StoryKey {
		t.Fatalf("the answer carries the saved macro, got %+v", batch.Macro)
	}

	// Every line failing is still a batch that ran.
	rr = postMacroStories(h, project.ID, `{"todoIds":["b"]}`, nil)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"failed":1`) {
		t.Fatalf("an all-failed batch answers 200, got %d %s", rr.Code, rr.Body.String())
	}

	for _, body := range []string{`{"todoIds":[]}`, `not json`} {
		if rr := postMacroStories(h, project.ID, body, nil); rr.Code != http.StatusBadRequest {
			t.Errorf("%s: want 400, got %d %s", body, rr.Code, rr.Body.String())
		}
	}
}

// A line whose target needs the caller's own token fails with its code and
// provider, the batch still answering 200; a key tied to nobody is refused
// whole, as the single-line action refuses it.
func TestMacroStoriesBatchCredentialRefusals(t *testing.T) {
	github := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Reads (the milestones behind the macros) are answered; a write
		// means a refusal was bypassed.
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		t.Errorf("a refused write must reach nothing, GitHub received %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer github.Close()
	t.Setenv("SECTILE_GITHUB_API_URL", github.URL)
	t.Setenv("SECTILE_GITHUB_TOKEN", "server-token")
	t.Setenv("SECTILE_SERVER_TOKEN", "shared-secret")
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "GitHub", IssueTracker: "github", GithubRepo: "acme/app"})
	if err != nil {
		t.Fatal(err)
	}
	todos := []models.MacroTodo{{ID: "a", Text: "First"}, {ID: "b", Text: "Second"}}
	if _, err := database.SaveMacroMeta(project.ID, "M-1", nil, nil, nil, &todos); err != nil {
		t.Fatal(err)
	}
	if err := database.EnsureUser("usr_grace"); err != nil {
		t.Fatal(err)
	}
	session, _, err := database.CreateWebSession("usr_grace")
	if err != nil {
		t.Fatal(err)
	}

	rr := postMacroStories(h, project.ID, `{"todoIds":["a","b"]}`, func(r *http.Request) {
		r.AddCookie(&http.Cookie{Name: sessionCookie, Value: session})
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("got %d %s", rr.Code, rr.Body.String())
	}
	var batch db.MacroStoryBatch
	if err := json.Unmarshal(rr.Body.Bytes(), &batch); err != nil {
		t.Fatal(err)
	}
	if batch.Failed != 2 {
		t.Fatalf("both lines must fail, got %+v", batch)
	}
	for _, r := range batch.Results {
		if r.Code != TrackerCredentialMissingCode || r.Tracker != "github" || !strings.Contains(r.Error, "Profile → Tracker credentials") {
			t.Errorf("line %s must name the missing token, got %+v", r.TodoID, r)
		}
	}

	rr = postMacroStories(h, project.ID, `{"todoIds":["a"]}`, func(r *http.Request) {
		r.Header.Set("Authorization", "Bearer shared-secret")
	})
	if rr.Code != http.StatusForbidden || !strings.Contains(rr.Body.String(), taskmcp.AnonymousWriteRefusal) {
		t.Fatalf("a key tied to nobody is refused whole, got %d %s", rr.Code, rr.Body.String())
	}
}
