package trackerapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"tasks/internal/models"
	"tasks/internal/tracker"
)

func TestJiraCreatesASprintOnTheProjectBoard(t *testing.T) {
	site := newJiraSite(t)
	var body map[string]any
	site.on("POST", "/rest/agile/1.0/sprint", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		fmt.Fprint(w, `{"id":42,"name":"Sprint 1","state":"future","startDate":"2026-10-05T09:00:00+02:00","endDate":"2026-10-19T08:59:59+02:00"}`)
	})
	project := jiraProject()
	project.BoardID = "5"
	start := time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)
	sprint, err := site.adapter().CreateSprint(unattended(), tracker.SprintCreateRequest{Project: project, Name: "Sprint 1", Start: start, End: start.AddDate(0, 0, 14).Add(-time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	if sprint.ID != "42" || sprint.State != "future" || sprint.Name != "Sprint 1" {
		t.Fatalf("unexpected sprint %+v", sprint)
	}
	if body["originBoardId"] != float64(5) || body["startDate"] != "2026-10-05T09:00:00Z" || body["endDate"] != "2026-10-19T08:59:59Z" {
		t.Fatalf("unexpected payload %v", body)
	}
}

func TestJiraSprintCreationNeedsABoard(t *testing.T) {
	site := newJiraSite(t)
	if _, err := site.adapter().CreateSprint(unattended(), tracker.SprintCreateRequest{Project: jiraProject(), Name: "Sprint 1"}); err == nil || !strings.Contains(err.Error(), "board") {
		t.Fatalf("a project without a board must be refused, got %v", err)
	}
}

func TestJiraSprintUpdateSendsOnlyThePatchedFields(t *testing.T) {
	site := newJiraSite(t)
	var body map[string]any
	site.on("POST", "/rest/agile/1.0/sprint/42", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		fmt.Fprint(w, `{"id":42,"name":"Renamed","state":"future","endDate":"2026-10-20T23:59:59Z"}`)
	})
	name, end := "Renamed", "2026-10-20"
	sprint, err := site.adapter().UpdateSprint(unattended(), jiraProject(), "42", models.SprintPatch{Name: &name, End: &end})
	if err != nil {
		t.Fatal(err)
	}
	if sprint.Name != "Renamed" || len(body) != 2 || body["name"] != "Renamed" {
		t.Fatalf("sprint %+v, payload %v", sprint, body)
	}
	// A bare end day ends one second before a 09:00 start, as a batch does.
	if got, _ := body["endDate"].(string); !strings.HasPrefix(got, "2026-10-20T08:59:59") {
		t.Fatalf("end date %q", got)
	}
	bogus := "paused"
	if _, err := site.adapter().UpdateSprint(unattended(), jiraProject(), "42", models.SprintPatch{State: &bogus}); err == nil {
		t.Fatal("an unknown state must be refused before any request")
	}
}

func TestJiraSprintRefusalIsQuoted(t *testing.T) {
	site := newJiraSite(t)
	site.on("POST", "/rest/agile/1.0/sprint/42", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"errorMessages":["Sprint cannot be closed: it has not been started."]}`)
	})
	closed := "closed"
	_, err := site.adapter().UpdateSprint(unattended(), jiraProject(), "42", models.SprintPatch{State: &closed})
	if err == nil || !strings.Contains(err.Error(), "has not been started") {
		t.Fatalf("Jira's reason must be quoted, got %v", err)
	}
}

func TestJiraSprintDeletionTreatsAMissingSprintAsDeleted(t *testing.T) {
	site := newJiraSite(t)
	site.on("DELETE", "/rest/agile/1.0/sprint/42", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	site.on("DELETE", "/rest/agile/1.0/sprint/43", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotFound) })
	site.on("DELETE", "/rest/agile/1.0/sprint/44", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"errorMessages":["An active sprint cannot be deleted."]}`)
	})
	if err := site.adapter().DeleteSprint(unattended(), jiraProject(), "42"); err != nil {
		t.Fatal(err)
	}
	if err := site.adapter().DeleteSprint(unattended(), jiraProject(), "43"); err != nil {
		t.Fatalf("a sprint Jira no longer knows is deleted: %v", err)
	}
	if err := site.adapter().DeleteSprint(unattended(), jiraProject(), "44"); err == nil || !strings.Contains(err.Error(), "active sprint") {
		t.Fatalf("a refusal must be quoted, got %v", err)
	}
}
