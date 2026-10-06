package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"tasks/internal/agentprotocol"
	"tasks/internal/db"
	"tasks/internal/handlers"
	"tasks/internal/models"
	"tasks/internal/testsqlite"
)

func TestHandleArchiveWorkspace(t *testing.T) {
	database, err := testsqlite.New(t, filepath.Join(t.TempDir(), "test.db"), db.NewDB)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	user, err := database.SignInLocal("alice@example.com")
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := database.CreateWebSession(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	task, err := database.CreateTask(models.CreateTaskRequest{ProjectID: "default", Title: "archive me"})
	if err != nil {
		t.Fatal(err)
	}
	h := handlers.NewHandler(database)
	var seen agentprotocol.Operation
	answer := models.WorkspaceArchive{Repositories: []models.WorkspaceArchiveEntry{{Role: "code", Outcome: models.ArchiveFailed, Error: "contains untracked files"}}}
	var refusal error
	database.SetAgentOperations(func(_ context.Context, op agentprotocol.Operation) (json.RawMessage, error) {
		seen = op
		if refusal != nil {
			return nil, refusal
		}
		return json.Marshal(answer)
	})
	post := func(id string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/tasks/"+id+"/archive-workspace", nil)
		req.AddCookie(&http.Cookie{Name: "sectile_session", Value: token})
		rr := httptest.NewRecorder()
		h.HandleTaskDetail(rr, req)
		return rr
	}

	rr := post(task.ID)
	var body struct {
		Archivable   bool                           `json:"archivable"`
		Repositories []models.WorkspaceArchiveEntry `json:"repositories"`
	}
	if rr.Code != http.StatusOK || json.Unmarshal(rr.Body.Bytes(), &body) != nil || body.Archivable || len(body.Repositories) != 1 || body.Repositories[0].Error != "contains untracked files" {
		t.Fatalf("refused archive = %d %s", rr.Code, rr.Body.String())
	}
	if seen.Action != "archive_workspace" || seen.UserID != user.ID || seen.TaskID != task.ID {
		t.Errorf("operation = %+v", seen)
	}

	answer.Repositories[0] = models.WorkspaceArchiveEntry{Role: "code", Outcome: models.ArchiveRemoved}
	if rr := post(task.ID); rr.Code != http.StatusOK || json.Unmarshal(rr.Body.Bytes(), &body) != nil || !body.Archivable {
		t.Errorf("clean archive = %d %s", rr.Code, rr.Body.String())
	}

	refusal = errors.New("local operation requires a connected agent")
	if rr := post(task.ID); rr.Code != http.StatusBadGateway {
		t.Errorf("no agent = %d %s", rr.Code, rr.Body.String())
	}
	if rr := post("missing"); rr.Code != http.StatusNotFound || rr.Body.String() == "" {
		t.Errorf("unknown task = %d %s", rr.Code, rr.Body.String())
	}
}
