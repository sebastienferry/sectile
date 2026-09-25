package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"tasks/internal/models"
	"tasks/internal/taskmcp"
)

// Over REST, a write the tracker client refuses for want of the caller's own
// credential is a 403 that says what to add, never a 500 and never a write
// under the server account (#482).
func TestRESTTrackerWriteRefusalsAreForbiddenAndNamed(t *testing.T) {
	var requests atomic.Int32
	github := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"number":42,"title":"T","state":"open"}`))
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
	if err := database.EnsureUser("usr_grace"); err != nil {
		t.Fatal(err)
	}
	session, _, err := database.CreateWebSession("usr_grace")
	if err != nil {
		t.Fatal(err)
	}
	create := func(authorize func(*http.Request)) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/tasks", strings.NewReader(`{"projectId":"`+project.ID+`","title":"Follow-up"}`))
		authorize(req)
		rr := httptest.NewRecorder()
		h.HandleTasks(rr, req)
		return rr
	}

	// A person signed in on the web, with no GitHub token of their own.
	rr := create(func(r *http.Request) { r.AddCookie(&http.Cookie{Name: sessionCookie, Value: session}) })
	if rr.Code != http.StatusForbidden || !strings.Contains(rr.Body.String(), "Profile → Tracker credentials") {
		t.Fatalf("a web session without a token must get a named 403, got %d %s", rr.Code, rr.Body.String())
	}

	// The shared server key names nobody.
	rr = create(func(r *http.Request) { r.Header.Set("Authorization", "Bearer shared-secret") })
	if rr.Code != http.StatusForbidden || !strings.Contains(rr.Body.String(), taskmcp.AnonymousWriteRefusal) {
		t.Fatalf("a key tied to no user must get a named 403, got %d %s", rr.Code, rr.Body.String())
	}

	if n := requests.Load(); n != 0 {
		t.Fatalf("a refused write must reach nothing, GitHub received %d request(s)", n)
	}
	if tasks, _ := database.GetTasks("", "", "", "", project.ID, "", "", "", "", nil, nil, false); len(tasks) != 0 {
		t.Fatalf("a refused creation filed %d task(s)", len(tasks))
	}
}
