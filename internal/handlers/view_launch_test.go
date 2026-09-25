package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tasks/internal/db"
	"tasks/internal/handlers"
	"tasks/internal/models"

	"github.com/gorilla/websocket"
)

// viewLaunchFixture is a task of one project, a view of the implicit user that
// selects it with a repository, and a server a workstation agent can connect to.
func viewLaunchFixture(t *testing.T) (*db.DB, *handlers.Handler, *models.Task, *models.BoardView, string) {
	t.Helper()
	database, err := db.NewDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	h := handlers.NewHandler(database)
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Views", IssueTracker: "local"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := database.CreateProject(models.CreateProjectRequest{Name: "Elsewhere", IssueTracker: "local"})
	if err != nil {
		t.Fatal(err)
	}
	task, err := database.CreateTask(models.CreateTaskRequest{Title: "From a view", ProjectID: project.ID})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.EnsureUser(db.ImplicitUserID); err != nil {
		t.Fatal(err)
	}
	name, repository := "Platform", "git@gitlab.com:group/platform.git"
	view, err := database.CreateBoardView(db.ImplicitUserID, models.BoardViewRequest{Name: &name, ProjectIDs: &[]string{project.ID}, Repository: &repository})
	if err != nil {
		t.Fatal(err)
	}
	return database, h, task, view, other.ID
}

func postViewLaunch(t *testing.T, h *handlers.Handler, cookie *http.Cookie, taskID, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/tasks/"+taskID+"/run-skill", strings.NewReader(body))
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	h.HandleTaskDetail(rec, req)
	return rec
}

// A launch from a view the user does not have, or from a view that does not
// show the ticket, is refused before anything is recorded (#429 US4).
func TestRunSkillRefusesAViewThatDoesNotApply(t *testing.T) {
	database, h, task, view, otherProject := viewLaunchFixture(t)
	cookie := defaultSession(t, database)

	if rec := postViewLaunch(t, h, cookie, task.ID, `{"skillId":"clarify","viewId":"no-such-view"}`); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown view: status %d: %s", rec.Code, rec.Body.String())
	}
	// Without a session the view is nobody's, which answers like a missing one.
	if rec := postViewLaunch(t, h, nil, task.ID, `{"skillId":"clarify","viewId":"`+view.ID+`"}`); rec.Code != http.StatusNotFound {
		t.Fatalf("view without a session: status %d: %s", rec.Code, rec.Body.String())
	}
	if _, err := database.UpdateBoardView(db.ImplicitUserID, view.ID, models.BoardViewRequest{ProjectIDs: &[]string{otherProject}}); err != nil {
		t.Fatal(err)
	}
	if rec := postViewLaunch(t, h, cookie, task.ID, `{"skillId":"clarify","viewId":"`+view.ID+`"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("view without the project: status %d: %s", rec.Code, rec.Body.String())
	}
	activities, err := database.GetTaskActivities(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	stored, _ := database.GetTaskByID(task.ID)
	if len(activities) != 0 || stored.ViewRepository != "" {
		t.Fatalf("a refused launch left a trace: %d activities, view repository %q", len(activities), stored.ViewRepository)
	}
}

// A launch admitted from a view with a repository records it on the ticket.
func TestRunSkillFromAViewRecordsItsRepository(t *testing.T) {
	database, h, task, view, _ := viewLaunchFixture(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ws/agent-connect" {
			h.HandleAgentConnect(w, r)
			return
		}
		h.HandleTaskDetail(w, r)
	}))
	defer server.Close()
	u, _ := url.Parse(server.URL)
	u.Scheme, u.Path = "ws", "/ws/agent-connect"
	u.RawQuery = "token=" + defaultAgentKey(t, database) + "&deviceId=my-laptop&projectId=default"
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/tasks/"+task.ID+"/run-skill", strings.NewReader(`{"skillId":"clarify","viewId":"`+view.ID+`","viewFolder":true}`))
	req.AddCookie(defaultSession(t, database))
	responses := make(chan *http.Response, 1)
	go func() {
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Error(err)
			close(responses)
			return
		}
		responses <- resp
	}()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, raw, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("agent did not receive the dispatch: %v", err)
	}
	var message handlers.AgentMessage
	if err := json.Unmarshal(raw, &message); err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(map[string]string{"status": "completed", "summary": "opened"})
	if err := conn.WriteJSON(handlers.AgentMessage{MsgID: message.MsgID, TaskID: task.ID, Type: "step_status", Payload: payload}); err != nil {
		t.Fatal(err)
	}
	select {
	case resp := <-responses:
		if resp == nil {
			t.FailNow()
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("run-skill status %d", resp.StatusCode)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("launch acknowledgement not handled")
	}
	stored, err := database.GetTaskByID(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.ViewRepository != "gitlab.com/group/platform" {
		t.Fatalf("view repository = %q, want the view's identity", stored.ViewRepository)
	}
}
