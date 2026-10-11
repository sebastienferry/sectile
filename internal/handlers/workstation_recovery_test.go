package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"tasks/internal/agentprotocol"
	"tasks/internal/db"
	"tasks/internal/models"
)

func TestDisconnectedExecutionRecoveryBothSurfaces(t *testing.T) {
	for _, surface := range []string{"task", "activity"} {
		for _, state := range []string{"queued", "pending", "running"} {
			t.Run(surface+"/"+state, func(t *testing.T) {
				h, d, cleanup := setupTestHandler(t)
				defer cleanup()
				server := clientRunServer(t, h)
				_, admin := account(t, d, "admin@example.com")
				ownerID, owner := account(t, d, "owner@example.com")
				_, foreign := account(t, d, "foreign@example.com")
				task, err := d.CreateTask(models.CreateTaskRequest{Title: "Recovery", ProjectID: "default", Status: models.StatusToImplement, Labels: []string{"#specified"}})
				if err != nil {
					t.Fatal(err)
				}
				initialStage := d.StageOfTask(task)
				run := models.TaskActivity{DispatchState: "admitted", ID: "disconnected", TaskID: task.ID, RunProjectID: "default", SkillID: "remote_run", SkillName: "implement", Action: db.RunActionAgent, Status: state, UserID: ownerID, DeviceID: "offline", DeviceLabel: "Offline Mac", CreatedAt: time.Now()}
				if err := d.AddTaskActivity(run); err != nil {
					t.Fatal(err)
				}
				if err := d.ClaimRunDispatch(run.ID); err != nil {
					t.Fatal(err)
				}
				if err := d.CompleteRunDispatch(run.ID); err != nil {
					t.Fatal(err)
				}
				path := "/api/activities/" + run.ID + "/cancel"
				body := ""
				forced := `{"force":true}`
				if surface == "task" {
					path = "/api/tasks/" + task.ID + "/cancel-run"
					body = `{"runId":"` + run.ID + `"}`
					forced = `{"runId":"` + run.ID + `","force":true}`
				}
				if status, text := call(t, server, foreign, http.MethodPost, path, forced); status != 403 {
					t.Fatalf("foreign cancel: %d %s", status, text)
				}
				if status, text := call(t, server, owner, http.MethodPost, path, body); status != 502 {
					t.Fatalf("unconfirmed stop: %d %s", status, text)
				}
				actual, _ := d.GetActivityByID(run.ID)
				if actual.Status != state {
					t.Fatal("disconnect closed an unconfirmed execution")
				}
				if status, text := call(t, server, admin, http.MethodPost, path, forced); status != 200 {
					t.Fatalf("admin force: %d %s", status, text)
				}
				actual, _ = d.GetActivityByID(run.ID)
				if actual.Status != "canceled" || !strings.Contains(actual.Summary, "process was left running") {
					t.Fatalf("untruthful closure: %+v", actual)
				}
				next, err := d.StartAgentRun(task.ID, "implement", db.RunLaunch{UserID: ownerID, DeviceID: "new"})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := d.SyncRemoteRunStatusFor(ownerID, run.ID, task.ID, "default", task.Key, "implement", "running", "late reconnect", nil); err != nil {
					t.Fatal(err)
				}
				actual, _ = d.GetActivityByID(run.ID)
				if actual.Status != "canceled" {
					t.Fatal("reconnect resurrected forced execution")
				}
				if _, err := d.SyncRemoteRunStatusFor(ownerID, run.ID, task.ID, "default", task.Key, "implement", "completed", "late supervisor report", nil); err != nil {
					t.Fatal(err)
				}
				actual, _ = d.GetActivityByID(run.ID)
				if actual.Status != "canceled" {
					t.Fatal("late supervisor report overwrote forced closure")
				}

				if _, err := d.FinishRemoteRunAs(db.Actor{ID: ownerID}, false, task.ID, run.ID, "completed", "late completion"); err == nil {
					t.Fatal("late report bypassed terminal guard")
				}
				newer, _ := d.GetActivityByID(next.ID)
				if newer.Status != "running" {
					t.Fatal("late report changed newer execution")
				}
				current, _ := d.GetTaskByID(task.ID)
				if d.StageOfTask(current) != initialStage {
					t.Fatalf("force advanced workflow: %s", d.StageOfTask(current))
				}
			})
		}
	}
}

func TestUndispatchedCancellationPreventsDispatch(t *testing.T) {
	h, d, cleanup := setupTestHandler(t)
	defer cleanup()
	server := clientRunServer(t, h)
	ownerID, owner := account(t, d, "owner@example.com")
	task, err := d.CreateTask(models.CreateTaskRequest{Title: "Before dispatch", ProjectID: "default"})
	if err != nil {
		t.Fatal(err)
	}
	run, err := d.StartAgentRun(task.ID, "clarify", db.RunLaunch{UserID: ownerID, DeviceID: "mac"})
	if err != nil {
		t.Fatal(err)
	}
	if status, text := call(t, server, owner, http.MethodPost, "/api/activities/"+run.ID+"/cancel", ""); status != 200 {
		t.Fatalf("cancel: %d %s", status, text)
	}
	if err := d.ClaimRunDispatch(run.ID); err == nil {
		t.Fatal("canceled execution can dispatch")
	}
	if _, err := d.StartAgentRun(task.ID, "clarify", db.RunLaunch{UserID: ownerID, DeviceID: "mac"}); err != nil {
		t.Fatal("canceled execution retains lock")
	}
}

func TestReconnectClosesOnlyProvenPinnedOrphans(t *testing.T) {
	h, d, cleanup := setupTestHandler(t)
	defer cleanup()
	task, err := d.CreateTask(models.CreateTaskRequest{Title: "Reconnect evidence", ProjectID: "default"})
	if err != nil {
		t.Fatal(err)
	}
	run, err := d.StartAgentRun(task.ID, "clarify", db.RunLaunch{UserID: "owner", DeviceID: "mac"})
	if err != nil {
		t.Fatal(err)
	}
	if err := d.ClaimRunDispatch(run.ID); err != nil {
		t.Fatal(err)
	}
	if err := d.CompleteRunDispatch(run.ID); err != nil {
		t.Fatal(err)
	}
	connected := time.Now().Add(time.Second)
	for _, identity := range [][2]string{{"foreign", "mac"}, {"owner", "other"}} {
		if candidates := h.reconnectCandidates(identity[0], identity[1], connected); len(candidates) != 0 {
			t.Fatal("foreign reconciliation exposed run")
		}
	}
	candidates := h.reconnectCandidates("owner", "mac", connected)
	if len(candidates) != 1 {
		t.Fatalf("candidate count: %d", len(candidates))
	}
	h.reconcileAssignedRuns(candidates, []agentprotocol.RunningTask{{ID: run.ID, Status: "running"}})
	actual, _ := d.GetActivityByID(run.ID)
	if actual.Status != "running" {
		t.Fatal("reported process closed")
	}
	h.reconcileAssignedRuns(candidates, nil)
	actual, _ = d.GetActivityByID(run.ID)
	if actual.Status != "canceled" {
		t.Fatal("proven orphan stayed active")
	}
	next, err := d.StartAgentRun(task.ID, "clarify", db.RunLaunch{UserID: "owner", DeviceID: "mac"})
	if err != nil {
		t.Fatal(err)
	}
	if candidates := h.reconnectCandidates("owner", "mac", connected); len(candidates) != 0 {
		t.Fatal("undispatched admission considered orphan")
	}
	if err := d.ClaimRunDispatch(next.ID); err != nil {
		t.Fatal(err)
	}
	if err := d.CompleteRunDispatch(next.ID); err != nil {
		t.Fatal(err)
	}
	if candidates := h.reconnectCandidates("owner", "mac", next.CreatedAt.Add(-time.Second)); len(candidates) != 0 {
		t.Fatal("new launch considered reconnect orphan")
	}
}

func TestActivityStopUsesPersistedDeviceAndInventoryNames(t *testing.T) {
	h, d, cleanup := setupTestHandler(t)
	defer cleanup()
	ownerID, owner := account(t, d, "owner@example.com")
	token, device, err := d.CreateAPIKey(ownerID, "My workstation", 0)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/ws/agent-connect", h.HandleAgentConnect)
	mux.HandleFunc("/api/activities/", h.HandleActivityDetail)
	mux.HandleFunc("/api/agent/status", h.HandleAgentStatus)
	server := httptest.NewServer(h.RequireSession(mux))
	defer server.Close()
	endpoint := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/agent-connect?token=" + url.QueryEscape(token) + "&projectId=default&deviceId=" + device.ID
	agent, _, err := websocket.DefaultDialer.Dial(endpoint, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer agent.Close()
	// Wait until the exact session is registered, then report its available project.
	deadline := time.Now().Add(time.Second)
	for len(h.agentDispatcher.ActiveConnections()) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("registration timed out")
		}
		time.Sleep(time.Millisecond)
	}
	reportTaskInventory(t, h.agentDispatcher, ownerID, device.ID, "default")
	status, body := call(t, server, owner, http.MethodGet, "/api/agent/status?projectId=default", "")
	if status != 200 || !strings.Contains(body, `"label":"My workstation"`) || strings.Contains(body, token) {
		t.Fatalf("inventory label: %d %s", status, body)
	}
	task, err := d.CreateTask(models.CreateTaskRequest{Title: "Persisted stop", ProjectID: "default"})
	if err != nil {
		t.Fatal(err)
	}
	run, err := d.StartAgentRun(task.ID, "implement", db.RunLaunch{UserID: ownerID, DeviceID: device.ID, DeviceLabel: "My workstation"})
	if err != nil {
		t.Fatal(err)
	}
	if err := d.ClaimRunDispatch(run.ID); err != nil {
		t.Fatal(err)
	}
	if err := d.CompleteRunDispatch(run.ID); err != nil {
		t.Fatal(err)
	}
	// There is no in-memory ownership entry: cancellation must restore it from storage.
	result := make(chan int, 1)
	go func() {
		status, _ := call(t, server, owner, http.MethodPost, "/api/activities/"+run.ID+"/cancel", "")
		result <- status
	}()
	agent.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, raw, err := agent.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	var message AgentMessage
	if err := json.Unmarshal(raw, &message); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(message.Payload), run.ID) || !strings.Contains(string(message.Payload), "cancel_run") {
		t.Fatalf("wrong process stop: %+v", message)
	}
	payload, _ := json.Marshal(map[string]string{"status": "completed", "summary": "stopped"})
	if err := agent.WriteJSON(AgentMessage{MsgID: message.MsgID, TaskID: task.ID, Type: "step_status", Payload: payload}); err != nil {
		t.Fatal(err)
	}
	if status := <-result; status != 200 {
		t.Fatalf("confirmed stop=%d", status)
	}
	actual, _ := d.GetActivityByID(run.ID)
	if actual.Status != "canceled" || !strings.Contains(actual.Summary, "stopped by") {
		t.Fatalf("confirmed outcome: %+v", actual)
	}
}

func TestStaleLauncherCannotCancelANewerRun(t *testing.T) {
	h, d, cleanup := setupTestHandler(t)
	defer cleanup()
	server := clientRunServer(t, h)
	ownerID, owner := account(t, d, "owner@example.com")
	task, err := d.CreateTask(models.CreateTaskRequest{Title: "Stale launch", ProjectID: "default"})
	if err != nil {
		t.Fatal(err)
	}
	old, err := d.StartAgentRun(task.ID, "implement", db.RunLaunch{UserID: ownerID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.FinishRemoteRun(task.ID, old.ID, "canceled", "old ended"); err != nil {
		t.Fatal(err)
	}
	next, err := d.StartAgentRun(task.ID, "implement", db.RunLaunch{UserID: ownerID})
	if err != nil {
		t.Fatal(err)
	}
	if err := d.AddTaskActivity(models.TaskActivity{ID: "stale-launch", TaskID: task.ID, UserID: ownerID, RemoteRunID: old.ID, SkillID: "agent_launch", Status: "running", CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	status, body := call(t, server, owner, http.MethodPost, "/api/activities/stale-launch/cancel", `{"force":true}`)
	if status != 409 {
		t.Fatalf("stale cancellation: %d %s", status, body)
	}
	newer, _ := d.GetActivityByID(next.ID)
	if newer.Status != "running" {
		t.Fatal("stale launch canceled newer execution")
	}
}

func TestQueuedTaskCancellationBothEndpoints(t *testing.T) {
	for _, state := range []string{"queued", "pending"} {
		for _, surface := range []string{"task", "activity"} {
			t.Run(surface+"/"+state, func(t *testing.T) {
				h, d, cleanup := setupTestHandler(t)
				defer cleanup()
				server := clientRunServer(t, h)
				ownerID, owner := account(t, d, "owner@example.com")
				task, err := d.CreateTask(models.CreateTaskRequest{Title: "Queued recovery", ProjectID: "default"})
				if err != nil {
					t.Fatal(err)
				}
				if err := d.AddTaskActivity(models.TaskActivity{ID: "queued-launch", TaskID: task.ID, UserID: ownerID, SkillID: "implement", Status: state, CreatedAt: time.Now()}); err != nil {
					t.Fatal(err)
				}
				path := "/api/activities/queued-launch/cancel"
				body := ""
				if surface == "task" {
					path = "/api/tasks/" + task.ID + "/cancel-run"
					body = `{"runId":"queued-launch"}`
				}
				if status, text := call(t, server, owner, http.MethodPost, path, body); status != 200 {
					t.Fatalf("queued cancel: %d %s", status, text)
				}
				actual, _ := d.GetActivityByID("queued-launch")
				if actual.Status != "canceled" {
					t.Fatal("queue entry remained active")
				}
				if _, err := d.StartAgentRun(task.ID, "implement", db.RunLaunch{UserID: ownerID}); err != nil {
					t.Fatal("canceled queue entry blocks relaunch")
				}
			})
		}
	}
}
