package handlers_test

import (
	"net/http"
	"testing"

	"tasks/internal/db"
	"tasks/internal/models"
)

// declaredFixture is macroAxesFixture's Jira project, declaring DATA as a
// roadmap project (#632).
func declaredFixture(t *testing.T) (*db.DB, string, func(key, body string) (int, map[string]any), func(open bool)) {
	t.Helper()
	database, server, project := macroAxesFixture(t, "jira")
	declared := []string{"DATA"}
	if _, err := database.UpdateProject(project.ID, models.UpdateProjectRequest{RoadmapProjects: &declared}); err != nil {
		t.Fatal(err)
	}
	post := func(key, body string) (int, map[string]any) {
		return postMacro(t, database, server, project.ID, key, body)
	}
	setOptIn := func(open bool) {
		if _, err := database.UpdateProject(project.ID, models.UpdateProjectRequest{RoadmapAxisWrites: &open}); err != nil {
			t.Fatal(err)
		}
	}
	return database, project.ID, post, setOptIn
}

// trackerOps counts every queued tracker write of a project.
func trackerOps(t *testing.T, database *db.DB, projectID string) int {
	t.Helper()
	acts, err := database.GetProjectActivities(projectID)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, a := range acts {
		if a.SkillID == "tracker_op" {
			n++
		}
	}
	return n
}

func TestTheHorizonOfAForeignEpicIsKeptInSectile(t *testing.T) {
	database, projectID, post, setOptIn := declaredFixture(t)
	setOptIn(true)

	status, body := post("DATA-12", `{"horizon":"now"}`)
	if status != http.StatusOK {
		t.Fatalf("status %d: %v", status, body)
	}
	macro, _ := body["macro"].(map[string]any)
	if macro["horizon"] != "now" || macro["foreign"] != true || macro["origin"] != "DATA" {
		t.Errorf("macro = %v", macro)
	}
	if body["labelNote"] != "conservé dans Sectile, non écrit sur le tracker" {
		t.Errorf("labelNote = %v", body["labelNote"])
	}
	if n := trackerOps(t, database, projectID); n != 0 {
		t.Errorf("queued = %d, the horizon of a foreign epic queues nothing", n)
	}
}

func TestForeignEpicAxesStayLocalUntilTheOptInOpens(t *testing.T) {
	database, projectID, post, setOptIn := declaredFixture(t)

	status, body := post("DATA-12", `{"priority":"P1"}`)
	if status != http.StatusOK {
		t.Fatalf("status %d: %v", status, body)
	}
	macro, _ := body["macro"].(map[string]any)
	if macro["priority"] != "p1" || macro["axesWritable"] != false {
		t.Errorf("macro = %v", macro)
	}
	if n := trackerOps(t, database, projectID); n != 0 {
		t.Fatalf("queued = %d, a closed opt-in queues nothing", n)
	}

	setOptIn(true)
	if status, body := post("DATA-12", `{"priority":"P2","quarter":"2026-Q4"}`); status != http.StatusOK {
		t.Fatalf("status %d: %v", status, body)
	}
	if ops := axisOps(t, database, projectID); len(ops) != 2 {
		t.Fatalf("queued = %v, want one write per axis once opted in", ops)
	}

	// The seeding is a bulk edit: it never reaches a foreign epic.
	status, body = post("DATA-12", `{"priority":"P3","bulk":true}`)
	if status != http.StatusOK {
		t.Fatalf("status %d: %v", status, body)
	}
	if ops := axisOps(t, database, projectID); len(ops) != 2 {
		t.Errorf("queued = %v, a bulk edit must queue nothing on a foreign epic", ops)
	}
	macro, _ = body["macro"].(map[string]any)
	if macro["priority"] != "p3" {
		t.Errorf("the bulk value must still be stored, macro = %v", macro)
	}

	// An own epic is unchanged by bulk.
	if status, body := post("PE-1", `{"priority":"P1","bulk":true}`); status != http.StatusOK {
		t.Fatalf("status %d: %v", status, body)
	}
	if ops := axisOps(t, database, projectID); len(ops) != 3 {
		t.Errorf("queued = %v, an own epic's bulk edit is still written", ops)
	}
}
