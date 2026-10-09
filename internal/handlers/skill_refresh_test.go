package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
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

// An import takes the file on disk through the agent, then asks the connected
// workstations to refresh their copies, as a save does (#732).
func TestSkillImportRefreshesConnectedAgents(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Refresh"})
	if err != nil {
		t.Fatal(err)
	}
	database.SetAgentOperations(func(_ context.Context, op agentprotocol.Operation) (json.RawMessage, error) {
		if op.Action != "read_skill" {
			return nil, fmt.Errorf("unexpected operation %s", op.Action)
		}
		return json.RawMessage(`{"content":"# Clarify\n\nThe file on disk."}`), nil
	})
	agent := connectClusterAgentBuild(t, h.agentDispatcher, "u1", project.ID, "laptop", announcedBuild("v1.0.0", ""))

	rec := skillEditorRequest(h, http.MethodPost, "/api/projects/"+project.ID+"/skill-editor/clarify/import", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("import: %d %s", rec.Code, rec.Body.String())
	}
	answerRefresh(t, agent, project.ID)
}

// A user with two agents serving the project, one for it and one for every
// project, is asked once: the operation is routed by user and project.
func TestSkillSaveRefreshesOneAgentPerUser(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Refresh"})
	if err != nil {
		t.Fatal(err)
	}
	agents := []*websocket.Conn{
		connectClusterAgentBuild(t, h.agentDispatcher, "u1", project.ID, "laptop", announcedBuild("v1.0.0", "")),
		connectClusterAgentBuild(t, h.agentDispatcher, "u1", "all", "desktop", announcedBuild("v1.0.0", "")),
	}

	rec := skillEditorRequest(h, http.MethodPut, "/api/projects/"+project.ID+"/skill-editor/clarify", `{"content":"## Steps\nProject steps."}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	var received []string
	for _, conn := range agents {
		wg.Add(1)
		go func(conn *websocket.Conn) {
			defer wg.Done()
			deadline := time.Now().Add(2 * time.Second)
			for {
				_ = conn.SetReadDeadline(deadline)
				var msg AgentMessage
				if conn.ReadJSON(&msg) != nil {
					return
				}
				mu.Lock()
				received = append(received, msg.Type+" "+string(msg.Payload))
				mu.Unlock()
				payload, _ := json.Marshal(agentprotocol.Result{Value: json.RawMessage(`{"written":3}`)})
				_ = conn.WriteJSON(AgentMessage{Type: "workspace_result", MsgID: msg.MsgID, Payload: payload})
			}
		}(conn)
	}
	wg.Wait()
	if len(received) != 1 {
		t.Fatalf("the user's agents were sent %d messages, want one refresh_skills: %v", len(received), received)
	}
	var msg struct {
		Action string `json:"action"`
	}
	if !strings.HasPrefix(received[0], "workspace_request ") || json.Unmarshal([]byte(strings.TrimPrefix(received[0], "workspace_request ")), &msg) != nil || msg.Action != "refresh_skills" {
		t.Fatalf("the user's agents were sent %v", received)
	}
}
