package handlers_test

import (
	"context"
	"encoding/json"
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

// epicTracker serves one labelled epic and accepts every label write.
type epicTracker struct {
	tracker.BaseTicketingSystem
}

func (e *epicTracker) ListEpics(ctx context.Context, req tracker.ProjectRequest) ([]models.Task, error) {
	return []models.Task{{Key: "PE-1", Title: "Billing", Labels: []string{"domain-billing", "roadmap:now"}}}, nil
}

func (e *epicTracker) UpdateIssue(ctx context.Context, req tracker.UpdateIssueRequest) error {
	return nil
}

func epicLabelsFixture(t *testing.T) (*db.DB, *handlers.Handler, *models.Project) {
	t.Helper()
	database, err := testsqlite.New(t, filepath.Join(t.TempDir(), "test.db"), db.NewDB)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	database.TrackerRegistry().Register("jira", &epicTracker{tracker.BaseTicketingSystem{
		TrackerName:  "jira",
		Capabilities: []tracker.Capability{tracker.CapUpdate, tracker.CapLabels, tracker.CapEpic},
	}})
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Platform", Slug: "platform", IssueTracker: "jira", JiraProject: "PE"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.ImportMacroHorizons(t.Context(), project.ID); err != nil {
		t.Fatal(err)
	}
	return database, handlers.NewHandler(database), project
}

func postEpicLabels(t *testing.T, h *handlers.Handler, projectID string, body string) *httptest.ResponseRecorder {
	t.Helper()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/projects/"+projectID+"/macros/PE-1/labels", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	h.HandleProjectDetail(rr, req)
	return rr
}

func epicLabelActivities(t *testing.T, database *db.DB, projectID string) int {
	t.Helper()
	acts, err := database.GetActivities(projectID, "", "", "", "", 100)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, a := range acts {
		if strings.HasPrefix(a.Action, "Labels de PE-1") {
			n++
		}
	}
	return n
}

func TestEpicLabelEditIsQueued(t *testing.T) {
	database, h, project := epicLabelsFixture(t)

	rr := postEpicLabels(t, h, project.ID, `{"add":["client-acme"],"remove":["domain-billing"]}`)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d %s, want 202", rr.Code, rr.Body.String())
	}
	var out struct {
		Activity models.TaskActivity `json:"activity"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Activity.Action != "Labels de PE-1" {
		t.Errorf("activity action = %q", out.Activity.Action)
	}
	if got := epicLabelActivities(t, database, project.ID); got != 1 {
		t.Errorf("queued activities = %d, want 1", got)
	}
}

func TestEpicLabelEditRefusesAnAxisLabelWithoutQueuing(t *testing.T) {
	database, h, project := epicLabelsFixture(t)

	rr := postEpicLabels(t, h, project.ID, `{"add":["roadmap:later"]}`)
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "appartient à un axe de la roadmap") {
		t.Fatalf("got %d %s, want 400 naming the roadmap axis", rr.Code, rr.Body.String())
	}
	if got := epicLabelActivities(t, database, project.ID); got != 0 {
		t.Errorf("queued activities = %d, want none", got)
	}
}
