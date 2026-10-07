package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"tasks/internal/db"
	"tasks/internal/models"
	"tasks/internal/testsqlite"
	"tasks/internal/tracker"
)

// candidateTracker serves one synthetic select field on every epic's edit
// screen (#680).
type candidateTracker struct {
	tracker.BaseTicketingSystem
	reads []string
}

func (c *candidateTracker) EpicAxisFieldCandidates(ctx context.Context, trk *models.Tracker, epicKey string) ([]models.EpicFieldCandidate, error) {
	c.reads = append(c.reads, epicKey)
	return []models.EpicFieldCandidate{{ID: "cf-epic-rank", Name: "Epic rank", Kind: models.EpicFieldSelect, Options: []models.EpicFieldOption{{ID: "o1", Value: "P1"}}}}, nil
}

func (c *candidateTracker) SetEpicAxisField(ctx context.Context, trk *models.Tracker, epicKey string, field models.EpicAxisField, optionPath string) error {
	return nil
}

var _ tracker.EpicAxisFieldManager = (*candidateTracker)(nil)

// TestEpicAxisFieldsOverHTTP covers what the Roadmap tab relies on: the
// candidates answer "no epic" before any epic exists, then the fields of an
// epic with their deduced maps; a mapping saves with the project, and an
// invalid one answers 400.
func TestEpicAxisFieldsOverHTTP(t *testing.T) {
	database, err := testsqlite.New(t, filepath.Join(t.TempDir(), "tasks.db"), db.NewDB)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	fake := &candidateTracker{BaseTicketingSystem: tracker.BaseTicketingSystem{TrackerName: "jira"}}
	database.TrackerRegistry().Register("jira", fake)
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Ops", IssueTracker: "jira", JiraProject: "OPS"})
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(database)
	send := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		rec := httptest.NewRecorder()
		h.HandleProjectDetail(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
		return rec
	}

	rec := send(http.MethodGet, "/api/projects/"+project.ID+"/epic-axis-fields", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"noEpic":true`) || len(fake.reads) != 0 {
		t.Fatalf("no epic: %d %s, reads %v", rec.Code, rec.Body.String(), fake.reads)
	}

	horizon := "now"
	if _, err := database.SaveMacroMeta(project.ID, "OPS-3", &horizon, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	rec = send(http.MethodGet, "/api/projects/"+project.ID+"/epic-axis-fields", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("candidates returned %d: %s", rec.Code, rec.Body.String())
	}
	var discovery db.EpicAxisFieldDiscovery
	if err := json.Unmarshal(rec.Body.Bytes(), &discovery); err != nil {
		t.Fatal(err)
	}
	if discovery.EpicKey != "OPS-3" || len(discovery.Candidates) != 1 || discovery.Candidates[0].Deduced[models.EpicAxisPriority]["p1"] != "o1" {
		t.Fatalf("discovery = %+v", discovery)
	}

	rec = send(http.MethodPut, "/api/projects/"+project.ID, `{"epicAxisFields":{"priority":{"id":"cf-epic-rank","name":"Epic rank","kind":"select","options":{"p1":"o1"}}}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("save returned %d: %s", rec.Code, rec.Body.String())
	}
	var saved models.Project
	if err := json.Unmarshal(rec.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.EpicAxisFields.Priority == nil || saved.EpicAxisFields.Priority.Options["p1"] != "o1" {
		t.Fatalf("saved = %+v", saved.EpicAxisFields)
	}

	rec = send(http.MethodPut, "/api/projects/"+project.ID, `{"epicAxisFields":{"priority":{"id":"cf-epic-rank","kind":"text"}}}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("an invalid kind returned %d: %s", rec.Code, rec.Body.String())
	}
}
