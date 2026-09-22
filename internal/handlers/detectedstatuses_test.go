package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"tasks/internal/db"
	"tasks/internal/handlers"
	"tasks/internal/models"
	"tasks/internal/tracker"
)

// boardTracker stands in for a tracker that has boards. The detection endpoint
// must ask it rather than name a tracker, so a Jira project stops answering the
// GitHub shape.
type boardTracker struct {
	tracker.BaseTicketingSystem
}

func (b *boardTracker) ListBoards(ctx context.Context, req tracker.BoardsRequest) ([]models.TrackerBoard, error) {
	return []models.TrackerBoard{{ID: "5", Name: "PE", Type: "scrum"}}, nil
}

func (b *boardTracker) ListBoardColumns(ctx context.Context, req tracker.BoardRequest) ([]models.TrackerColumn, error) {
	return []models.TrackerColumn{
		{Name: "To Do", Statuses: []string{"To Do", "Backlog"}},
		{Name: "Done", Statuses: []string{"Done"}},
	}, nil
}

func (b *boardTracker) ListStatuses(ctx context.Context, req tracker.ProjectRequest) ([]tracker.TrackerStatus, error) {
	return []tracker.TrackerStatus{{ID: "1", Name: "To Do"}, {ID: "2", Name: "In Review"}}, nil
}

type detectedStatuses struct {
	Statuses []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"statuses"`
	Columns []models.TrackerColumn `json:"columns"`
	Error   string                 `json:"error"`
}

func detect(t *testing.T, h *handlers.Handler, query string) detectedStatuses {
	t.Helper()
	rr := httptest.NewRecorder()
	h.HandleProjectDetail(rr, httptest.NewRequest(http.MethodGet, "/api/projects/detected-statuses?"+query, nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("detection refused: %d %s", rr.Code, rr.Body.String())
	}
	var out detectedStatuses
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestDetectedStatusesMirrorsTheBoardColumns(t *testing.T) {
	database, err := db.NewDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	database.TrackerRegistry().Register("jira", &boardTracker{tracker.BaseTicketingSystem{
		TrackerName:  "jira",
		Capabilities: []tracker.Capability{tracker.CapBoard},
	}})
	h := handlers.NewHandler(database)

	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Platform", IssueTracker: "jira", JiraProject: "PE"})
	if err != nil {
		t.Fatal(err)
	}

	got := detect(t, h, "projectId="+project.ID)
	if len(got.Columns) != 2 || got.Columns[0].Name != "To Do" || len(got.Columns[0].Statuses) != 2 {
		t.Fatalf("the board columns must travel with the response: %+v", got.Columns)
	}
	if got.Error != "" {
		t.Fatalf("unexpected error field: %q", got.Error)
	}
	// The palette is the union: the project statuses plus what the board groups.
	names := map[string]bool{}
	for _, st := range got.Statuses {
		names[st.Name] = true
	}
	for _, want := range []string{"To Do", "In Review", "Backlog", "Done"} {
		if !names[want] {
			t.Fatalf("status %q missing from the palette: %+v", want, got.Statuses)
		}
	}
}

// A tracker without boards keeps the payload it had: no columns field, and the
// GitHub fallback untouched.
func TestDetectedStatusesKeepsTheGithubShape(t *testing.T) {
	database, err := db.NewDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	h := handlers.NewHandler(database)

	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Sectile", IssueTracker: "github"})
	if err != nil {
		t.Fatal(err)
	}

	saved := detect(t, h, "projectId="+project.ID)
	if saved.Columns != nil || saved.Error != "" {
		t.Fatalf("the GitHub payload gained a field: %+v", saved)
	}
	if len(saved.Statuses) != 2 || saved.Statuses[0].Name != "open" || saved.Statuses[1].Name != "closed" {
		t.Fatalf("the GitHub fallback changed: %+v", saved.Statuses)
	}

	draft := detect(t, h, "tracker=github")
	if draft.Columns != nil {
		t.Fatalf("a draft project has no board to mirror: %+v", draft.Columns)
	}
	if len(draft.Statuses) != 2 || draft.Statuses[0].Name != "open" {
		t.Fatalf("the draft path changed: %+v", draft.Statuses)
	}
}
