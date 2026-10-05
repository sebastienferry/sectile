package handlers

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"tasks/internal/agentprotocol"
	"tasks/internal/models"
)

// answerRefresh reads the refresh_skills request an agent receives and answers it.
func answerRefresh(t *testing.T, conn *websocket.Conn, projectID string) {
	t.Helper()
	msg := readAgentMessage(t, conn)
	var op agentprotocol.Operation
	if msg.Type != "workspace_request" || json.Unmarshal(msg.Payload, &op) != nil || op.Action != "refresh_skills" || op.ProjectID != projectID {
		t.Fatalf("the agent was sent %s %s", msg.Type, msg.Payload)
	}
	payload, _ := json.Marshal(agentprotocol.Result{Value: json.RawMessage(`{"written":3}`)})
	if err := conn.WriteJSON(AgentMessage{Type: "workspace_result", MsgID: msg.MsgID, Payload: payload}); err != nil {
		t.Fatal(err)
	}
}

func expectNothing(t *testing.T, conn *websocket.Conn, who string) {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	var msg AgentMessage
	if conn.ReadJSON(&msg) == nil {
		t.Fatalf("%s was sent %s %s", who, msg.Type, msg.Payload)
	}
}

// A skill save, and a reset, ask the connected workstations serving the project
// to rewrite the direct copies they manage (#732). An agent too old for the
// operation is not sent it, and one connected for another project is not asked.
func TestSkillSaveRefreshesConnectedAgents(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Refresh"})
	if err != nil {
		t.Fatal(err)
	}
	current := connectClusterAgentBuild(t, h.agentDispatcher, "u1", project.ID, "laptop", announcedBuild("v1.0.0", ""))
	outdated := connectClusterAgentBuild(t, h.agentDispatcher, "u2", "all", "desktop", announcedBuild("v0.9.0", "", "refresh_skills"))
	elsewhere := connectClusterAgentBuild(t, h.agentDispatcher, "u3", "other-project", "tower", announcedBuild("v1.0.0", ""))
	base := "/api/projects/" + project.ID + "/skill-editor"

	rec := skillEditorRequest(h, http.MethodPut, base+"/clarify", `{"content":"## Steps\nProject steps."}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	answerRefresh(t, current, project.ID)

	rec = skillEditorRequest(h, http.MethodPost, base+"/clarify/reset", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("reset: %d %s", rec.Code, rec.Body.String())
	}
	answerRefresh(t, current, project.ID)

	expectNothing(t, outdated, "an agent without refresh_skills")
	expectNothing(t, elsewhere, "an agent of another project")
}
