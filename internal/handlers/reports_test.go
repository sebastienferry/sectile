package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"tasks/internal/db"
	"tasks/internal/models"
)

func TestRetainedReportReadUsesTaskAndSessionBoundary(t *testing.T) {
	h, d, cleanup := setupTestHandler(t)
	defer cleanup()
	actor, cookie := account(t, d, "reader@example.com")
	server := guardedServer(t, h)
	task, err := d.CreateTask(models.CreateTaskRequest{ProjectID: "default", Title: "Reports"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := d.CreateTask(models.CreateTaskRequest{ProjectID: "default", Title: "Other"})
	if err != nil {
		t.Fatal(err)
	}
	meta := models.ReportMetadata{PublicationID: "round", Stage: "clarified", Round: 1, Summary: "Decision accepted."}
	if _, err = d.PublishTaskReportAs(context.Background(), db.Actor{ID: actor}, task.ID, "full **`report`**", meta, nil, ""); err != nil {
		t.Fatal(err)
	}
	path := "/api/tasks/" + url.PathEscape(task.ID) + "/reports"
	if status, _ := call(t, server, nil, http.MethodGet, path, ""); status != http.StatusUnauthorized {
		t.Fatalf("anonymous read: %d", status)
	}
	status, body := call(t, server, cookie, http.MethodGet, path, "")
	var reports []models.TaskReport
	if status != http.StatusOK || json.Unmarshal([]byte(body), &reports) != nil || len(reports) != 1 || reports[0].Body != "full **`report`**" {
		t.Fatalf("report read: %d %s", status, body)
	}
	status, body = call(t, server, cookie, http.MethodGet, "/api/tasks/"+url.PathEscape(other.ID)+"/reports", "")
	if status != http.StatusOK || body != "[]\n" {
		t.Fatalf("cross-task report exposed: %d %s", status, body)
	}
}
