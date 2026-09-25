package handlers

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"tasks/internal/agentprotocol"
	"tasks/internal/db"
	"tasks/internal/models"
)

// ownedAgentRun gives carol a live interactive run and an agent credential.
func ownedAgentRun(t *testing.T, h *Handler, database *db.DB) (*httptest.Server, string, *models.TaskActivity) {
	t.Helper()
	server := guardedServer(t, h)
	carolID, _ := account(t, database, "carol@example.com")
	task, err := database.CreateTask(models.CreateTaskRequest{ProjectID: "default", Title: "Carol's question", Status: models.StatusToClarify})
	if err != nil {
		t.Fatal(err)
	}
	run, err := database.StartAgentRun(task.ID, "clarify", db.RunLaunch{UserID: carolID, Mode: models.SkillModeInteractive})
	if err != nil {
		t.Fatal(err)
	}
	key, _, err := database.CreateAPIKey(carolID, "carol laptop", 0)
	if err != nil {
		t.Fatal(err)
	}
	return server, key, run
}

func awaitAgent(t *testing.T, h *Handler, userID string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for h.agentDispatcher.Lookup(userID, "default") == nil {
		if time.Now().After(deadline) {
			t.Fatal("the agent never registered")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// answerPull reads agent messages until the server's pull, and answers it with
// the given run as running.
func answerPull(t *testing.T, agent *websocket.Conn, run *models.TaskActivity) {
	t.Helper()
	_ = agent.SetReadDeadline(time.Now().Add(3 * time.Second))
	for {
		_, raw, err := agent.ReadMessage()
		if err != nil {
			t.Fatalf("the server never pulled the running tasks: %v", err)
		}
		var message AgentMessage
		if err := json.Unmarshal(raw, &message); err != nil || message.Type != "pull_tasks" {
			continue
		}
		payload, _ := json.Marshal([]agentprotocol.RunningTask{{ID: run.ID, TaskID: run.TaskID, Skill: "clarify", Status: "running"}})
		if err := agent.WriteJSON(AgentMessage{MsgID: message.MsgID, Type: "running_tasks", Payload: payload}); err != nil {
			t.Fatal(err)
		}
		return
	}
}

// A clear reaches the agent even though this process never sent it the mark:
// the mark may have been set before a restart, or by another instance (#475).
func TestAClearIsPushedWithoutThePushedMark(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	server, key, run := ownedAgentRun(t, h, database)
	if err := database.SetRemoteRunWaiting(run.ID, true); err != nil {
		t.Fatal(err)
	}
	agent := connectAgentAs(t, server, key, "default")
	awaitAgent(t, h, run.UserID)

	if err := database.SetRemoteRunWaiting(run.ID, false); err != nil {
		t.Fatal(err)
	}
	if cleared := readRunWaiting(t, agent); cleared.RunID != run.ID || cleared.WaitingSince != nil {
		t.Fatalf("the agent received %+v, want the mark cleared", cleared)
	}
}

// An agent that reconnects is sent the waiting state of the runs it reports,
// set or cleared, so a change made while it was away reaches the desktop.
func TestReconnectResendsTheWaitingState(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	h.SetPullOnConnect(true)
	server, key, run := ownedAgentRun(t, h, database)
	if err := database.SetRemoteRunWaiting(run.ID, true); err != nil {
		t.Fatal(err)
	}

	agent := connectAgentAs(t, server, key, "default")
	answerPull(t, agent, run)
	if marked := readRunWaiting(t, agent); marked.RunID != run.ID || marked.WaitingSince == nil {
		t.Fatalf("the reconnected agent received %+v, want the run marked", marked)
	}
	_ = agent.Close()
	deadline := time.Now().Add(3 * time.Second)
	for h.agentDispatcher.Lookup(run.UserID, "default") != nil {
		if time.Now().After(deadline) {
			t.Fatal("the agent never unregistered")
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Cleared while the agent is away: nobody receives it.
	if err := database.SetRemoteRunWaiting(run.ID, false); err != nil {
		t.Fatal(err)
	}
	again := connectAgentAs(t, server, key, "default")
	answerPull(t, again, run)
	if cleared := readRunWaiting(t, again); cleared.RunID != run.ID || cleared.WaitingSince != nil {
		t.Fatalf("the reconnected agent received %+v, want the mark cleared", cleared)
	}
}

// The owner's agent reporting an answer ends the wait, and is told so; another
// user's agent cannot end it.
func TestRunAnsweredEndsTheOwnersWait(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	server, key, run := ownedAgentRun(t, h, database)
	daveID, _ := account(t, database, "dave@example.com")
	daveKey, _, err := database.CreateAPIKey(daveID, "dave laptop", 0)
	if err != nil {
		t.Fatal(err)
	}
	agent := connectAgentAs(t, server, key, "default")
	awaitAgent(t, h, run.UserID)
	intruder := connectAgentAs(t, server, daveKey, "default")
	awaitAgent(t, h, daveID)

	if err := database.SetRemoteRunWaiting(run.ID, true); err != nil {
		t.Fatal(err)
	}
	marked := readRunWaiting(t, agent)
	if marked.WaitingSince == nil {
		t.Fatal("the mark was not pushed")
	}
	answer, _ := json.Marshal(agentprotocol.RunAnswered{RunID: run.ID, WaitingSince: *marked.WaitingSince})

	if err := intruder.WriteJSON(AgentMessage{Type: agentprotocol.RunAnsweredType, Payload: answer}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	if waitingSinceOf(t, database, run.ID) == nil {
		t.Fatal("another user's agent ended carol's wait")
	}

	if err := agent.WriteJSON(AgentMessage{Type: agentprotocol.RunAnsweredType, Payload: answer}); err != nil {
		t.Fatal(err)
	}
	if cleared := readRunWaiting(t, agent); cleared.RunID != run.ID || cleared.WaitingSince != nil {
		t.Fatalf("the agent received %+v, want the mark cleared", cleared)
	}
	if waitingSinceOf(t, database, run.ID) != nil {
		t.Fatal("the answered wait is still recorded")
	}
}
