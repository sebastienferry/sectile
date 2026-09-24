package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"tasks/internal/agentprotocol"
	"tasks/internal/db"
)

// memoryDirectory is a presence table shared by the instances of a test.
type memoryDirectory struct {
	mu   sync.Mutex
	rows map[agentKey]db.AgentLocation
}

// instanceView is one instance's handle on the shared directory.
type instanceView struct {
	shared  *memoryDirectory
	id      string
	address string
}

func (v *instanceView) InstanceID() string { return v.id }

func (v *instanceView) AgentConnected(userID, projectID, deviceID string) (db.AgentLocation, bool, error) {
	v.shared.mu.Lock()
	defer v.shared.mu.Unlock()
	key := agentKey{UserID: userID, ProjectID: projectID}
	previous, had := v.shared.rows[key]
	v.shared.rows[key] = db.AgentLocation{UserID: userID, ProjectID: projectID, InstanceID: v.id, Address: v.address, DeviceID: deviceID, ConnectedAt: time.Now()}
	return previous, had && previous.InstanceID != v.id, nil
}

func (v *instanceView) AgentDisconnected(userID, projectID, deviceID string) {
	v.shared.mu.Lock()
	defer v.shared.mu.Unlock()
	key := agentKey{UserID: userID, ProjectID: projectID}
	if row, ok := v.shared.rows[key]; ok && row.InstanceID == v.id && row.DeviceID == deviceID {
		delete(v.shared.rows, key)
	}
}

func (v *instanceView) AgentOwner(userID, projectID string) (db.AgentLocation, bool) {
	v.shared.mu.Lock()
	defer v.shared.mu.Unlock()
	row, ok := v.shared.rows[agentKey{UserID: userID, ProjectID: projectID}]
	return row, ok
}

func (v *instanceView) AgentRecentlyConnected(userID, projectID string, since time.Time) bool {
	_, ok := v.AgentOwner(userID, projectID)
	return ok
}

func (v *instanceView) ConnectedAgentLocations() []db.AgentLocation {
	v.shared.mu.Lock()
	defer v.shared.mu.Unlock()
	var out []db.AgentLocation
	for _, row := range v.shared.rows {
		out = append(out, row)
	}
	return out
}

const clusterToken = "shared-token"

// clusterInstance is one server instance of a test: a dispatcher, its internal
// endpoints, and its view of the shared directory.
type clusterInstance struct {
	dispatcher *AgentDispatcher
	view       *instanceView
}

func newCluster(t *testing.T, ids ...string) []*clusterInstance {
	t.Helper()
	shared := &memoryDirectory{rows: map[agentKey]db.AgentLocation{}}
	var out []*clusterInstance
	for _, id := range ids {
		d := NewAgentDispatcher()
		internal := httptest.NewServer(d.InternalHandler())
		t.Cleanup(internal.Close)
		view := &instanceView{shared: shared, id: id, address: internal.URL}
		d.SetCluster(view, clusterToken, nil)
		out = append(out, &clusterInstance{dispatcher: d, view: view})
	}
	return out
}

// connectClusterAgent connects a fake agent to an instance and returns the agent's
// side of the WebSocket. The instance side feeds every answer back to its
// dispatcher, as /ws/agent-connect does.
func connectClusterAgent(t *testing.T, d *AgentDispatcher, userID, projectID, deviceID string) *websocket.Conn {
	t.Helper()
	registered := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		ac := d.Register(userID, projectID, deviceID, conn)
		close(registered)
		defer conn.Close()
		defer d.Unregister(ac.UserID, ac.ProjectID, conn)
		for {
			var msg AgentMessage
			if conn.ReadJSON(&msg) != nil {
				return
			}
			switch msg.Type {
			case "workspace_result":
				d.ReportOperation(ac, msg)
			case "step_status":
				d.ReportLaunchStatus(ac, msg)
			case "running_tasks":
				d.ReportRunningTasks(ac, msg)
			}
		}
	}))
	t.Cleanup(srv.Close)
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	<-registered
	return conn
}

func readAgentMessage(t *testing.T, conn *websocket.Conn) AgentMessage {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var msg AgentMessage
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("the agent received nothing: %v", err)
	}
	return msg
}

// An operation asked of the instance without the agent runs on the agent held
// by the other, and its result comes back.
func TestAnOperationReachesAnAgentHeldByAnotherInstance(t *testing.T) {
	c := newCluster(t, "A", "B")
	agent := connectClusterAgent(t, c[0].dispatcher, "default", "project", "laptop")

	done := make(chan error, 1)
	var result json.RawMessage
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var err error
		result, err = c[1].dispatcher.CallOperation(ctx, agentprotocol.Operation{ProjectID: "project", Action: "git_status"})
		done <- err
	}()
	request := readAgentMessage(t, agent)
	if request.Type != "workspace_request" {
		t.Fatalf("agent received %q", request.Type)
	}
	if err := agent.WriteJSON(AgentMessage{Type: "workspace_result", MsgID: request.MsgID, Payload: json.RawMessage(`{"value":{"branch":"feat/406"}}`)}); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatalf("forwarded operation failed: %v", err)
	}
	if string(result) != `{"branch":"feat/406"}` {
		t.Errorf("result = %s", result)
	}
}

// A confirmed launch through the other instance waits for the agent's answer,
// and a refusal keeps its meaning across instances.
func TestALaunchIsConfirmedThroughAnotherInstance(t *testing.T) {
	c := newCluster(t, "A", "B")
	agent := connectClusterAgent(t, c[0].dispatcher, "u1", "project", "laptop")

	for _, tc := range []struct {
		reply   string
		wantErr error
	}{
		{`{"status":"completed","summary":"ok"}`, nil},
		{`{"status":"failed","summary":"` + agentprotocol.RunNotOwned + ` run-1"}`, ErrRunNotOwned},
	} {
		done := make(chan error, 1)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			done <- c[1].dispatcher.DispatchAndWait(ctx, "u1", "project", "t1", map[string]string{"action": "cancel_run"})
		}()
		request := readAgentMessage(t, agent)
		if request.Type != "dispatch_step" {
			t.Fatalf("agent received %q", request.Type)
		}
		if err := agent.WriteJSON(AgentMessage{Type: "step_status", MsgID: request.MsgID, Payload: json.RawMessage(tc.reply)}); err != nil {
			t.Fatal(err)
		}
		err := <-done
		if tc.wantErr == nil && err != nil {
			t.Fatalf("launch failed: %v", err)
		}
		if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
			t.Fatalf("error = %v, want %v", err, tc.wantErr)
		}
	}
}

// A plain dispatch and a task pull reach the agent through the other instance.
func TestADispatchAndAPullReachAnAgentHeldByAnotherInstance(t *testing.T) {
	c := newCluster(t, "A", "B")
	agent := connectClusterAgent(t, c[0].dispatcher, "u1", "project", "laptop")

	if route := c[1].dispatcher.Route("u1", "project"); route == nil || route.DeviceID != "laptop" || route.remote == nil {
		t.Fatalf("route from the other instance = %+v", route)
	}
	if err := c[1].dispatcher.Dispatch("u1", "project", "pty_input", "t1", map[string]string{"data": "ls"}); err != nil {
		t.Fatal(err)
	}
	if msg := readAgentMessage(t, agent); msg.Type != "pty_input" || msg.TaskID != "t1" {
		t.Fatalf("agent received %+v", msg)
	}

	remote := c[1].dispatcher.RemoteAgents()
	if len(remote) != 1 || remote[0].InstanceID != "A" {
		t.Fatalf("remote agents = %+v", remote)
	}
	done := make(chan []agentprotocol.RunningTask, 1)
	go func() {
		tasks, err := c[1].dispatcher.PullRemoteTasks(context.Background(), remote[0])
		if err != nil {
			t.Errorf("pull: %v", err)
		}
		done <- tasks
	}()
	request := readAgentMessage(t, agent)
	if request.Type != "pull_tasks" {
		t.Fatalf("agent received %q", request.Type)
	}
	if err := agent.WriteJSON(AgentMessage{Type: "running_tasks", MsgID: request.MsgID, Payload: json.RawMessage(`[{"taskId":"t1"}]`)}); err != nil {
		t.Fatal(err)
	}
	if tasks := <-done; len(tasks) != 1 {
		t.Fatalf("pulled tasks = %+v", tasks)
	}
	if agents := c[1].dispatcher.ConnectedAgents(); len(agents) != 1 || agents[0].InstanceID != "A" {
		t.Errorf("status from the other instance = %+v", agents)
	}
}

// The same agent slot connecting through the other instance closes the first
// connection, as a rebind on one instance does.
func TestARebindThroughAnotherInstanceClosesTheOldConnection(t *testing.T) {
	c := newCluster(t, "A", "B")
	old := connectClusterAgent(t, c[0].dispatcher, "u1", "project", "laptop")
	connectClusterAgent(t, c[1].dispatcher, "u1", "project", "laptop")

	_ = old.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, _, err := old.ReadMessage()
	var closed *websocket.CloseError
	if !errors.As(err, &closed) || closed.Code != 4001 {
		t.Fatalf("the old connection was not rebound: %v", err)
	}
}

// An instance that does not answer fails the operation explicitly; nothing is
// retried.
func TestAnInstanceThatDoesNotAnswerFailsExplicitly(t *testing.T) {
	c := newCluster(t, "A", "B")
	gone := httptest.NewServer(http.NotFoundHandler())
	gone.Close()
	c[1].view.shared.rows[agentKey{UserID: "default", ProjectID: "project"}] = db.AgentLocation{
		UserID: "default", ProjectID: "project", InstanceID: "A", Address: gone.URL, DeviceID: "laptop",
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := c[1].dispatcher.CallOperation(ctx, agentprotocol.Operation{ProjectID: "project", Action: "git_status"})
	if err == nil || !strings.Contains(err.Error(), "(A) did not answer") {
		t.Fatalf("error = %v", err)
	}
}

// The internal endpoints refuse a call without the shared bearer, and an
// instance without a key refuses to forward, saying why.
func TestInternalCallsNeedTheSharedBearer(t *testing.T) {
	c := newCluster(t, "A", "B")
	connectClusterAgent(t, c[0].dispatcher, "u1", "project", "laptop")
	address := c[0].view.address

	resp, err := http.Post(address+internalAgentPath+"dispatch", "application/json", strings.NewReader(`{"userId":"u1","projectId":"project","type":"pty_input"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status without bearer = %d, want 401", resp.StatusCode)
	}

	c[1].dispatcher.SetCluster(c[1].view, "", errors.New("no server key"))
	err = c[1].dispatcher.Dispatch("u1", "project", "pty_input", "t1", nil)
	if err == nil || !strings.Contains(err.Error(), "no server key") {
		t.Fatalf("forwarding without a key = %v", err)
	}
}

// A skill launched through the other instance is confirmed by the agent held
// there, on the connection the operation was forwarded to.
func TestASkillLaunchReachesAnAgentHeldByAnotherInstance(t *testing.T) {
	c := newCluster(t, "A", "B")
	agent := connectClusterAgent(t, c[0].dispatcher, "u1", "project", "laptop")

	done := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := c[1].dispatcher.CallOperation(ctx, agentprotocol.Operation{UserID: "u1", ProjectID: "project", TaskID: "t1", Action: "execute_skill", SkillID: "clarify"})
		done <- err
	}()
	request := readAgentMessage(t, agent)
	if request.Type != "dispatch_step" || request.TaskID != "t1" {
		t.Fatalf("agent received %+v", request)
	}
	if err := agent.WriteJSON(AgentMessage{Type: "step_status", MsgID: request.MsgID, Payload: json.RawMessage(`{"status":"completed","summary":"ok"}`)}); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatalf("forwarded skill launch failed: %v", err)
	}
}

// Closing a rebound slot leaves alone an agent the instance holds for every
// project of the same user.
func TestClosingARemoteSlotKeepsTheAgentRegisteredForEveryProject(t *testing.T) {
	c := newCluster(t, "A", "B")
	agent := connectClusterAgent(t, c[0].dispatcher, "u1", "default", "laptop")

	req, err := http.NewRequest(http.MethodPost, c[0].view.address+internalAgentPath+"close", strings.NewReader(`{"userId":"u1","projectId":"project"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+clusterToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if err := c[0].dispatcher.Dispatch("u1", "default", "pty_input", "t1", map[string]string{"data": "ls"}); err != nil {
		t.Fatalf("the agent registered for every project was closed: %v", err)
	}
	if msg := readAgentMessage(t, agent); msg.Type != "pty_input" {
		t.Fatalf("agent received %+v", msg)
	}
}
