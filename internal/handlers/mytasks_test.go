package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"tasks/internal/db"
	"tasks/internal/models"
)

// fakeTracker answers GitHub's /user and Jira's /myself for the tokens it
// knows, and counts every request it receives.
func fakeTracker(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	calls := &atomic.Int32{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		switch {
		case r.URL.Path == "/user" && r.Header.Get("Authorization") == "Bearer ghp-ada":
			_, _ = w.Write([]byte(`{"login":"ada"}`))
		case r.URL.Path == "/rest/api/3/myself" && strings.HasPrefix(r.Header.Get("Authorization"), "Basic "):
			_, _ = w.Write([]byte(`{"accountId":"acc-1","displayName":"Ada Lovelace"}`))
		default:
			w.WriteHeader(http.StatusUnauthorized)
		}
	}))
	t.Cleanup(server.Close)
	return server, calls
}

func serve(h http.HandlerFunc, cookie *http.Cookie, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rr := httptest.NewRecorder()
	h(rr, req)
	return rr
}

func credentialAccounts(t *testing.T, rr *httptest.ResponseRecorder) map[string]string {
	t.Helper()
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
	}
	var body struct {
		Credentials []db.UserCredential `json:"credentials"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	accounts := map[string]string{}
	for _, credential := range body.Credentials {
		accounts[credential.Tracker] = credential.Account
	}
	return accounts
}

// Saving a personal credential asks the tracker whose it is (#468); a failed
// answer still saves it, with no account.
func TestSavingAPersonalCredentialLearnsItsAccount(t *testing.T) {
	database, err := db.NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	h := NewHandler(database)
	userID, cookie := account(t, database, "ada@example.com")
	tracker, _ := fakeTracker(t)

	put := func(body string) map[string]string {
		return credentialAccounts(t, serve(h.HandleUserTrackerCredentials, cookie, http.MethodPut, "/api/me/tracker-credentials", body))
	}
	if got := put(`{"tracker":"github","siteUrl":"` + tracker.URL + `","token":"ghp-ada"}`); got["github"] != "ada" {
		t.Fatalf("GitHub account: %v", got)
	}
	if got := put(`{"tracker":"jira","siteUrl":"` + tracker.URL + `","email":"ada@example.com","token":"ATATT"}`); got["jira"] != "Ada Lovelace" {
		t.Fatalf("Jira account: %v", got)
	}
	got := put(`{"tracker":"github","siteUrl":"` + tracker.URL + `","token":"ghp-unknown"}`)
	if _, saved := got["github"]; !saved || got["github"] != "" {
		t.Fatalf("a refused probe keeps the save with no account: %v", got)
	}

	// Verifying the stored credential from the profile learns it again.
	if err := database.SetUserTrackerCredential(userID, "github", tracker.URL, "", "ghp-ada", ""); err != nil {
		t.Fatal(err)
	}
	rr := serve(h.HandleTrackerSetup, cookie, http.MethodPost, "/api/setup/tracker/check", `{"tracker":"github","siteUrl":"`+tracker.URL+`"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("check: %d %s", rr.Code, rr.Body.String())
	}
	if got := credentialAccounts(t, serve(h.HandleUserTrackerCredentials, cookie, http.MethodGet, "/api/me/tracker-credentials", "")); got["github"] != "ada" {
		t.Fatalf("after verifying: %v", got)
	}
}

// GET /api/tasks?mine=1 resolves "me" on the server, from what is stored:
// loading the board never reaches a tracker.
func TestMyTasksFilterAndIdentities(t *testing.T) {
	database, err := db.NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	h := NewHandler(database)
	userID, cookie := account(t, database, "ada@example.com")
	tracker, calls := fakeTracker(t)

	hub, err := database.CreateProject(models.CreateProjectRequest{Name: "Hub", Slug: "hub", IssueTracker: "github"})
	if err != nil {
		t.Fatal(err)
	}
	jay, err := database.CreateProject(models.CreateProjectRequest{Name: "Jay", Slug: "jay", IssueTracker: "jira"})
	if err != nil {
		t.Fatal(err)
	}
	tasks := []models.Task{
		{ProjectID: hub.ID, Key: "#1", Title: "mine on GitHub", Source: "github", Assignee: "Ada"},
		{ProjectID: hub.ID, Key: "#2", Title: "bob's", Source: "github", Assignee: "bob"},
		{ProjectID: jay.ID, Key: "PE-1", Title: "mine by mail", Source: "jira", Assignee: "ada@example.com"},
		{ProjectID: jay.ID, Key: "PE-2", Title: "bob's on Jira", Source: "jira", Assignee: "Bob"},
	}
	for i := range tasks {
		tasks[i].Status, tasks[i].Priority = models.StatusToClarify, models.PriorityMedium
	}
	if err := database.ImportOrUpdateTasks(tasks); err != nil {
		t.Fatal(err)
	}
	if err := database.SetUserTrackerCredential(userID, "github", tracker.URL, "", "ghp-ada", ""); err != nil {
		t.Fatal(err)
	}
	if err := database.SetUserTrackerCredentialAccount(userID, "github", "ada"); err != nil {
		t.Fatal(err)
	}
	before := calls.Load()

	titles := func(path string) []string {
		rr := serve(h.HandleTasks, cookie, http.MethodGet, path, "")
		if rr.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", path, rr.Code, rr.Body.String())
		}
		var got []models.Task
		if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		out := []string{}
		for _, task := range got {
			out = append(out, task.Title)
		}
		return out
	}
	if got := titles("/api/tasks?mine=1&projectId=" + hub.ID); len(got) != 1 || got[0] != "mine on GitHub" {
		t.Fatalf("GitHub project: %q", got)
	}
	if got := titles("/api/tasks?mine=1&projectId=" + jay.ID); len(got) != 1 || got[0] != "mine by mail" {
		t.Fatalf("Jira project falls back on the e-mail: %q", got)
	}
	if got := titles("/api/tasks?projectId=" + hub.ID); len(got) != 2 {
		t.Fatalf("without mine=1 nothing is filtered: %q", got)
	}
	if calls.Load() != before {
		t.Fatalf("loading the board reached the tracker %d time(s)", calls.Load()-before)
	}

	var identities struct {
		SignedIn bool     `json:"signedIn"`
		Fallback []string `json:"fallback"`
		Trackers []struct {
			Tracker  string `json:"tracker"`
			Identity string `json:"identity"`
			Known    bool   `json:"known"`
		} `json:"trackers"`
	}
	read := func(cookie *http.Cookie, path string) int {
		rr := serve(h.HandleAssigneeIdentities, cookie, http.MethodGet, path, "")
		if rr.Code == http.StatusOK {
			identities.Trackers = nil
			if err := json.Unmarshal(rr.Body.Bytes(), &identities); err != nil {
				t.Fatal(err)
			}
		}
		return rr.Code
	}
	name := "Hub and Jay"
	projects := []string{hub.ID, jay.ID}
	view, err := database.CreateBoardView(userID, models.BoardViewRequest{Name: &name, ProjectIDs: &projects})
	if err != nil {
		t.Fatal(err)
	}
	if code := read(cookie, "/api/me/assignee-identities?viewId="+view.ID); code != http.StatusOK {
		t.Fatalf("identities: %d", code)
	}
	if !identities.SignedIn || len(identities.Trackers) != 2 ||
		identities.Trackers[0].Tracker != "github" || !identities.Trackers[0].Known || identities.Trackers[0].Identity != "ada" ||
		identities.Trackers[1].Tracker != "jira" || identities.Trackers[1].Known {
		t.Fatalf("identities: %+v", identities)
	}
	// The local sign-in names the account by its address: one identity.
	if len(identities.Fallback) != 1 || identities.Fallback[0] != "ada@example.com" {
		t.Fatalf("the fallback is the account's name and e-mail, once each: %q", identities.Fallback)
	}

	if code := read(cookie, "/api/me/assignee-identities?projectId="+hub.ID); code != http.StatusOK || len(identities.Trackers) != 1 {
		t.Fatalf("one project's trackers: %d %+v", code, identities.Trackers)
	}
	if code := read(nil, "/api/me/assignee-identities?projectId="+hub.ID); code != http.StatusOK || identities.SignedIn || identities.Trackers[0].Known {
		t.Fatalf("signed out: %d %+v", code, identities)
	}
	if code := read(cookie, "/api/me/assignee-identities?viewId=someone-elses"); code != http.StatusNotFound {
		t.Fatalf("a foreign view: %d", code)
	}
	if calls.Load() != before {
		t.Fatal("the identities never reach the tracker")
	}
}
