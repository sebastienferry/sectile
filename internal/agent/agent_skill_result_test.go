package agent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"tasks/internal/models"
	"testing"
	"time"
)

func TestDesktopSkillResultMatchesOwnedExecution(t *testing.T) {
	projectID := "project"
	fail := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail {
			http.Error(w, "unavailable", 503)
			return
		}
		switch r.URL.Path {
		case "/api/tasks/task":
			_ = json.NewEncoder(w).Encode(models.Task{ID: "task", ProjectID: projectID, Status: "to_specify"})
		case "/api/tasks/task/activities":
			_ = json.NewEncoder(w).Encode([]models.TaskActivity{
				{ID: "old", TaskID: "task", SkillID: "remote_run", SkillName: "clarify", Status: "completed"},
				{ID: "run", TaskID: "task", SkillID: "remote_run", SkillName: "clarify", Status: "running"},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	d := &agentDaemon{link: serverLink{serverURL: server.URL}, loopback: loopbackServer{desktopToken: "private"}, queue: runQueue{runs: map[string]*controlledRun{
		"run": {taskID: "task", desktop: desktopRun{ProjectID: "project", Status: "running"}},
	}}}
	request := httptest.NewRequest("GET", "/desktop/run-result?id=run", nil)
	response := httptest.NewRecorder()
	d.desktopHandler(response, request)
	if response.Code != 401 {
		t.Fatal("unauthenticated result lookup accepted")
	}
	response = disconnectRequest(d, "GET", "/desktop/run-result?id=run", "")
	var result struct {
		Activity models.TaskActivity `json:"activity"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if response.Code != 200 || result.Activity.ID != "run" || result.Activity.Status != "running" || result.Activity.SkillID != "clarify" {
		t.Fatalf("wrong result: %s", response.Body.String())
	}
	if response = disconnectRequest(d, "GET", "/desktop/run-result?id=unknown", ""); response.Code != 404 {
		t.Fatal("unknown run accepted")
	}
	projectID = "other"
	if response = disconnectRequest(d, "GET", "/desktop/run-result?id=run", ""); response.Code != 409 {
		t.Fatal("mismatched project accepted")
	}
	fail = true
	if response = disconnectRequest(d, "GET", "/desktop/run-result?id=run", ""); response.Code != 502 {
		t.Fatal("unavailable result accepted")
	}
}

func TestSkillSuccessorFollowsTheNextSkillOfTheConsole(t *testing.T) {
	at := func(minute int) time.Time { return time.Date(2026, 10, 6, 9, minute, 0, 0, time.UTC) }
	own := models.TaskActivity{ID: "run", TaskID: "task", SkillID: "remote_run", SkillName: "clarify", Status: "completed", CreatedAt: at(0)}
	next := func(id string, minute int) models.TaskActivity {
		return models.TaskActivity{ID: id, TaskID: "task", SkillID: "remote_run", SkillName: "specify-issue", Status: "running", CreatedAt: at(minute)}
	}
	held := map[string]bool{"run": true, "launched": true}
	cases := []struct {
		name       string
		own        models.TaskActivity
		activities []models.TaskActivity
		exited     bool
		exitedAt   time.Time
		want       string
	}{
		{name: "none", own: own, activities: []models.TaskActivity{own}},
		{name: "newest of two", own: own, activities: []models.TaskActivity{own, next("first", 5), next("second", 9)}, want: "second"},
		{name: "older than the console", own: own, activities: []models.TaskActivity{next("before", 0), own}},
		{name: "another execution of the agent", own: own, activities: []models.TaskActivity{own, next("launched", 5)}},
		{name: "own skill still running", own: models.TaskActivity{ID: "run", TaskID: "task", Status: "running", CreatedAt: at(0)}, activities: []models.TaskActivity{next("first", 5)}},
		{name: "another task", own: own, activities: []models.TaskActivity{own, {ID: "other", TaskID: "elsewhere", SkillID: "remote_run", CreatedAt: at(5)}}},
		{name: "not a remote run", own: own, activities: []models.TaskActivity{own, {ID: "sync", TaskID: "task", SkillID: "tracker_op", CreatedAt: at(5)}}},
		{name: "before the exit", own: own, activities: []models.TaskActivity{own, next("inside", 5), next("after", 20)}, exited: true, exitedAt: at(10), want: "inside"},
		{name: "exit time unknown", own: own, activities: []models.TaskActivity{own, next("first", 5)}, exited: true},
	}
	for _, tc := range cases {
		got := skillSuccessor(tc.own, tc.activities, held, tc.exited, tc.exitedAt)
		if (got == nil) != (tc.want == "") || got != nil && got.ID != tc.want {
			t.Errorf("%s: got %+v, want %q", tc.name, got, tc.want)
		}
	}
}

func TestDesktopSkillResultReportsTheSuccessor(t *testing.T) {
	waiting := time.Date(2026, 10, 6, 9, 6, 0, 0, time.UTC)
	activities := []models.TaskActivity{
		{ID: "run", TaskID: "task", SkillID: "remote_run", SkillName: "clarify", Status: "completed", CreatedAt: waiting.Add(-6 * time.Minute)},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tasks/task":
			_ = json.NewEncoder(w).Encode(models.Task{ID: "task", ProjectID: "project", Status: "to_specify"})
		case "/api/tasks/task/activities":
			_ = json.NewEncoder(w).Encode(activities)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	d := &agentDaemon{link: serverLink{serverURL: server.URL}, loopback: loopbackServer{desktopToken: "private"}, queue: runQueue{runs: map[string]*controlledRun{
		"run": {taskID: "task", desktop: desktopRun{ProjectID: "project", Status: "running"}},
	}}}
	read := func() (result struct {
		Successor *struct {
			ID           string     `json:"id"`
			SkillID      string     `json:"skillId"`
			Status       string     `json:"status"`
			WaitingSince *time.Time `json:"waitingSince"`
		} `json:"successor"`
	}) {
		response := disconnectRequest(d, "GET", "/desktop/run-result?id=run", "")
		if response.Code != 200 {
			t.Fatalf("result refused: %d %s", response.Code, response.Body.String())
		}
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	if result := read(); result.Successor != nil {
		t.Fatalf("successor without a next skill: %+v", result.Successor)
	}
	activities = append(activities, models.TaskActivity{ID: "next", TaskID: "task", SkillID: "remote_run", SkillName: "specify-issue", Status: "running", CreatedAt: waiting.Add(-time.Minute), WaitingSince: &waiting})
	result := read()
	if result.Successor == nil || result.Successor.ID != "next" || result.Successor.SkillID != "specify-issue" || result.Successor.Status != "running" || result.Successor.WaitingSince == nil || !result.Successor.WaitingSince.Equal(waiting) {
		t.Fatalf("wrong successor: %+v", result.Successor)
	}
}

func TestDesktopSkillRecordCarriesTheRecordedReason(t *testing.T) {
	completed := time.Date(2026, 10, 6, 9, 30, 0, 0, time.FixedZone("CEST", 2*3600))
	long := strings.Repeat("é", desktopReasonLimit+10)
	own := desktopSkillRecord(models.TaskActivity{ID: "run", TaskID: "task", SkillID: "remote_run", SkillName: "clarify", Status: "canceled", CompletedAt: &completed,
		Summary: "Execution reported by a local agent or native client \u2014 " + models.RunDisconnectNote, Error: long})
	if own["id"] != "run" || own["taskId"] != "task" || own["skillId"] != "clarify" || own["status"] != "canceled" {
		t.Fatalf("wrong identity: %+v", own)
	}
	if own["summary"] != "Execution reported by a local agent or native client \u2014 "+models.RunDisconnectNote || own["disconnected"] != true || own["completedAt"] != "2026-10-06T07:30:00Z" {
		t.Fatalf("wrong reason fields: %+v", own)
	}
	if got := own["error"].(string); got != strings.Repeat("é", desktopReasonLimit) {
		t.Fatalf("error not trimmed to %d characters: %d", desktopReasonLimit, len([]rune(got)))
	}
	if _, ok := own["waitingReason"]; ok {
		t.Fatalf("waiting reason sent for a run that does not wait: %+v", own)
	}
	parked := desktopSkillRecord(models.TaskActivity{ID: "next", TaskID: "task", SkillName: "specify-issue", Status: "running", WaitingReason: "repository"})
	// summary and error are sent even empty: they tell this agent from an older one.
	if parked["disconnected"] != false || parked["waitingReason"] != "repository" || parked["error"] != "" || parked["summary"] != "" {
		t.Fatalf("wrong parked record: %+v", parked)
	}
	if _, ok := parked["completedAt"]; ok {
		t.Fatalf("completion time sent for a running skill: %+v", parked)
	}
	// A note quoting the disconnection elsewhere than in its summary is no disconnection.
	if quoted := desktopSkillRecord(models.TaskActivity{Status: "failed", Error: models.RunDisconnectNote}); quoted["disconnected"] != false {
		t.Fatalf("disconnection read from the error: %+v", quoted)
	}
}
