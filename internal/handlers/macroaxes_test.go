package handlers_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"tasks/internal/db"
	"tasks/internal/handlers"
	"tasks/internal/models"
	"tasks/internal/testsqlite"
	"tasks/internal/tracker"
)

// labelledEpics is a Jira-shaped tracker whose epics can carry labels. The
// handler tests only need its capabilities: the writes are queued, not run.
func labelledEpics() tracker.TicketingSystem {
	return &tracker.BaseTicketingSystem{
		TrackerName:  "jira",
		Capabilities: []tracker.Capability{tracker.CapUpdate, tracker.CapLabels, tracker.CapEpic},
	}
}

func macroAxesFixture(t *testing.T, issueTracker string) (*db.DB, *httptest.Server, *models.Project) {
	t.Helper()
	database, err := testsqlite.New(t, filepath.Join(t.TempDir(), "test.db"), db.NewDB)
	if err != nil {
		t.Fatalf("db error: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	req := models.CreateProjectRequest{Name: "Platform", Slug: "platform", IssueTracker: issueTracker}
	if issueTracker == "jira" {
		req.JiraProject = "PE"
		database.TrackerRegistry().Register("jira", labelledEpics())
	}
	project, err := database.CreateProject(req)
	if err != nil {
		t.Fatalf("project: %v", err)
	}
	h := handlers.NewHandler(database)
	server := httptest.NewServer(http.HandlerFunc(h.HandleProjectDetail))
	t.Cleanup(server.Close)
	return database, server, project
}

func postMacro(t *testing.T, database *db.DB, server *httptest.Server, projectID, key, body string) (int, map[string]any) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/projects/"+projectID+"/macros/"+key, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(defaultSession(t, database))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	out := map[string]any{}
	_ = json.Unmarshal(raw, &out)
	if resp.StatusCode != http.StatusOK {
		out["raw"] = string(raw)
	}
	return resp.StatusCode, out
}

// axisOps lists the actions of the queued epic axis writes of a project.
func axisOps(t *testing.T, database *db.DB, projectID string) []string {
	t.Helper()
	acts, err := database.GetProjectActivities(projectID)
	if err != nil {
		t.Fatal(err)
	}
	out := []string{}
	for _, a := range acts {
		if a.SkillID == "tracker_op" && (strings.HasPrefix(a.Action, "Priorité de") || strings.HasPrefix(a.Action, "Trimestre de") || strings.HasPrefix(a.Action, "Readiness de")) {
			out = append(out, a.Action)
		}
	}
	return out
}

func TestMacroAxesAreSavedAndQueuedOnAJiraEpic(t *testing.T) {
	database, server, project := macroAxesFixture(t, "jira")

	status, body := postMacro(t, database, server, project.ID, "PE-1", `{"priority":"P1","quarter":"2026.q4"}`)
	if status != http.StatusOK {
		t.Fatalf("status %d: %v", status, body)
	}
	macro, _ := body["macro"].(map[string]any)
	if macro["priority"] != "p1" || macro["quarter"] != "2026-Q4" || macro["labelsWritable"] != true {
		t.Errorf("macro = %v", macro)
	}
	if body["labelNote"] != "labels en file d'attente" {
		t.Errorf("labelNote = %v", body["labelNote"])
	}
	ops := axisOps(t, database, project.ID)
	if len(ops) != 2 {
		t.Fatalf("queued = %v, want one write per axis", ops)
	}

	// Changing one axis queues one write.
	if status, body := postMacro(t, database, server, project.ID, "PE-1", `{"priority":""}`); status != http.StatusOK {
		t.Fatalf("status %d: %v", status, body)
	}
	ops = axisOps(t, database, project.ID)
	if len(ops) != 3 || !containsString(ops, "Priorité de PE-1 ➔ aucune") {
		t.Errorf("queued = %v", ops)
	}
}

func TestMacroAxesRefuseAnInvalidValueAndSaveNothing(t *testing.T) {
	database, server, project := macroAxesFixture(t, "jira")

	status, body := postMacro(t, database, server, project.ID, "PE-1", `{"horizon":"now","priority":"p1","quarter":"2026-Q5"}`)
	if status != http.StatusBadRequest || !strings.Contains(body["raw"].(string), "n'est pas un trimestre") {
		t.Fatalf("status %d: %v", status, body)
	}
	macros, _ := database.GetProjectMacros(project.ID)
	for _, m := range macros {
		if m.Key == "PE-1" && (m.Horizon != "" || m.Priority != "") {
			t.Errorf("a refused edit was partly saved: %+v", m)
		}
	}
	if ops := axisOps(t, database, project.ID); len(ops) != 0 {
		t.Errorf("queued = %v", ops)
	}
}

func TestMacroAxesStayInSectileOnAMilestoneOrALocalProject(t *testing.T) {
	for _, tc := range []struct{ tracker, key string }{{"jira", "M-3"}, {"jira", "OTHER-9"}, {"local", "EPIC-1"}} {
		database, server, project := macroAxesFixture(t, tc.tracker)
		status, body := postMacro(t, database, server, project.ID, tc.key, `{"priority":"p0","quarter":"2027-Q1"}`)
		if status != http.StatusOK {
			t.Fatalf("%s %s: status %d: %v", tc.tracker, tc.key, status, body)
		}
		macro, _ := body["macro"].(map[string]any)
		if macro["priority"] != "p0" || macro["labelsWritable"] != false {
			t.Errorf("%s %s: macro = %v", tc.tracker, tc.key, macro)
		}
		if body["labelNote"] != "conservé dans Sectile, non écrit sur le tracker" {
			t.Errorf("%s %s: labelNote = %v", tc.tracker, tc.key, body["labelNote"])
		}
		if ops := axisOps(t, database, project.ID); len(ops) != 0 {
			t.Errorf("%s %s: queued = %v, want none", tc.tracker, tc.key, ops)
		}
	}
}

func containsString(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

func TestMacroReadinessIsSavedAndQueuedOnAJiraEpic(t *testing.T) {
	database, server, project := macroAxesFixture(t, "jira")

	status, body := postMacro(t, database, server, project.ID, "PE-1", `{"readiness":"Shaping"}`)
	if status != http.StatusOK {
		t.Fatalf("status %d: %v", status, body)
	}
	macro, _ := body["macro"].(map[string]any)
	if macro["readiness"] != "shaping" || macro["labelsWritable"] != true {
		t.Errorf("macro = %v", macro)
	}
	if ops := axisOps(t, database, project.ID); len(ops) != 1 || ops[0] != "Readiness de PE-1 ➔ En cadrage" {
		t.Fatalf("queued = %v, want the readiness write only", ops)
	}

	if status, body := postMacro(t, database, server, project.ID, "PE-1", `{"readiness":""}`); status != http.StatusOK {
		t.Fatalf("status %d: %v", status, body)
	}
	if ops := axisOps(t, database, project.ID); len(ops) != 2 || !containsString(ops, "Readiness de PE-1 ➔ aucune") {
		t.Errorf("queued = %v", ops)
	}
}

func TestMacroReadinessRefusesAnInvalidLevelAndSavesNothing(t *testing.T) {
	database, server, project := macroAxesFixture(t, "jira")

	status, body := postMacro(t, database, server, project.ID, "PE-1", `{"priority":"p1","readiness":"done"}`)
	if status != http.StatusBadRequest || !strings.Contains(body["raw"].(string), "n'est pas une readiness") {
		t.Fatalf("status %d: %v", status, body)
	}
	macros, _ := database.GetProjectMacros(project.ID)
	for _, m := range macros {
		if m.Key == "PE-1" && (m.Priority != "" || m.Readiness != "") {
			t.Errorf("a refused edit was partly saved: %+v", m)
		}
	}
	if ops := axisOps(t, database, project.ID); len(ops) != 0 {
		t.Errorf("queued = %v", ops)
	}
}

func TestMacroReadinessStaysInSectileOnAMilestone(t *testing.T) {
	for _, tc := range []struct{ tracker, key string }{{"github", "M-3"}, {"jira", "M-3"}, {"local", "EPIC-1"}} {
		database, server, project := macroAxesFixture(t, tc.tracker)
		status, body := postMacro(t, database, server, project.ID, tc.key, `{"readiness":"ready"}`)
		if status != http.StatusOK {
			t.Fatalf("%s %s: status %d: %v", tc.tracker, tc.key, status, body)
		}
		macro, _ := body["macro"].(map[string]any)
		if macro["readiness"] != "ready" || macro["labelsWritable"] != false {
			t.Errorf("%s %s: macro = %v", tc.tracker, tc.key, macro)
		}
		if body["labelNote"] != "conservé dans Sectile, non écrit sur le tracker" {
			t.Errorf("%s %s: labelNote = %v", tc.tracker, tc.key, body["labelNote"])
		}
		if ops := axisOps(t, database, project.ID); len(ops) != 0 {
			t.Errorf("%s %s: queued = %v, want none", tc.tracker, tc.key, ops)
		}
	}
}
