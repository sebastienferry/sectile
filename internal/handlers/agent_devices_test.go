package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"tasks/internal/agentconfig"
	"tasks/internal/agentprotocol"
	"tasks/internal/models"
)

func TestDistinctDevicesAndUserIsolation(t *testing.T) {
	d := NewAgentDispatcher()
	a := connectClusterAgent(t, d, "u1", "all", "laptop")
	b := connectClusterAgent(t, d, "u1", "all", "desktop")
	connectClusterAgent(t, d, "u2", "all", "laptop")
	if len(d.ActiveConnections()) != 3 || len(d.ConnectedAgents()) != 3 {
		t.Fatal("a distinct user/device was displaced")
	}
	if _, err := d.SelectRoute("u1", "project", ""); !errors.Is(err, ErrDeviceRequired) {
		t.Fatalf("selection: %v", err)
	}
	if err := d.Dispatch("u1", "project", "pty_input", "t1", map[string]string{"data": "hello"}, "desktop"); err != nil {
		t.Fatal(err)
	}
	if msg := readAgentMessage(t, b); msg.Type != "pty_input" {
		t.Fatalf("message: %+v", msg)
	}
	old := d.Lookup("u1", "project", "laptop")
	replacement := connectClusterAgent(t, d, "u1", "specific", "laptop")
	_ = a.SetReadDeadline(time.Now().Add(time.Second))
	_, _, err := a.ReadMessage()
	var closeErr *websocket.CloseError
	if !errors.As(err, &closeErr) || closeErr.Code != 4001 {
		t.Fatalf("rebound: %v", err)
	}
	d.Unregister(old.UserID, old.DeviceID, old.Conn)
	if got := d.Lookup("u1", "specific", "laptop"); got == nil || got == old {
		t.Fatal("late unregister removed replacement")
	}
	if err := d.Dispatch("u1", "project", "pty_input", "t1", nil, "desktop"); err != nil {
		t.Fatal(err)
	}
	readAgentMessage(t, b)
	if err := d.Dispatch("u1", "specific", "pty_input", "t1", nil, "laptop"); err != nil {
		t.Fatal(err)
	}
	readAgentMessage(t, replacement)
}

func TestDeviceSelectionEligibilityAndDiagnostics(t *testing.T) {
	for _, tc := range []struct {
		name, project, device string
		extra                 bool
		want                  error
	}{
		{"single wildcard", "project", "", false, nil},
		{"single explicit", "project", "laptop", false, nil},
		{"invalid", "project", "missing", false, ErrInvalidDevice},
		{"ambiguous before eligibility", "project", "", true, ErrDeviceRequired},
		{"ineligible", "project", "scoped", true, ErrInvalidDevice},
		{"exact scoped", "another", "scoped", true, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := NewAgentDispatcher()
			connectClusterAgent(t, d, "u", "all", "laptop")
			connectClusterAgent(t, d, "other", "all", "secret-other-user")
			if tc.extra {
				connectClusterAgent(t, d, "u", "another", "scoped")
			}
			route, err := d.SelectRoute("u", tc.project, tc.device)
			if !errors.Is(err, tc.want) {
				t.Fatalf("route=%+v err=%v want=%v", route, err, tc.want)
			}
			if err != nil && strings.Contains(err.Error(), "secret-other-user") {
				t.Fatal("leaked another user's device")
			}
		})
	}
	if _, err := NewAgentDispatcher().SelectRoute("u", "p", ""); !errors.Is(err, ErrNoAgentConnected) {
		t.Fatal(err)
	}
}

func TestReplicaDeviceSelectionAndPinnedGeneration(t *testing.T) {
	c := newCluster(t, "A", "B", "C")
	a := connectClusterAgent(t, c[0].dispatcher, "u", "all", "laptop")
	b := connectClusterAgent(t, c[1].dispatcher, "u", "all", "desktop")
	for _, instance := range c {
		if _, err := instance.dispatcher.SelectRoute("u", "p", ""); !errors.Is(err, ErrDeviceRequired) {
			t.Fatalf("instance %s: %v", instance.view.id, err)
		}
	}
	if err := c[2].dispatcher.Dispatch("u", "p", "pty_input", "t", nil, "desktop"); err != nil {
		t.Fatal(err)
	}
	readAgentMessage(t, b)
	old := c[0].dispatcher.Lookup("u", "p", "laptop")
	replacement := connectClusterAgent(t, c[0].dispatcher, "u", "all", "laptop")
	_, err := c[0].dispatcher.serveInternal(context.Background(), "close", internalRequest{UserID: "u", DeviceID: "laptop", ConnectedAt: old.ConnectedAt})
	if err != nil {
		t.Fatal(err)
	}
	if err := c[2].dispatcher.Dispatch("u", "p", "pty_input", "t", nil, "laptop"); err != nil {
		t.Fatal(err)
	}
	readAgentMessage(t, replacement)
	_ = a.Close()
}

func launchReply(t *testing.T, agent *websocket.Conn, summary string) {
	t.Helper()
	request := readAgentMessage(t, agent)
	status := "completed"
	if summary != "" {
		status = "failed"
	}
	raw, _ := json.Marshal(agentLaunchStatus{Status: status, Summary: summary})
	if err := agent.WriteJSON(AgentMessage{MsgID: request.MsgID, Type: "step_status", Payload: raw}); err != nil {
		t.Fatal(err)
	}
}

func TestCancellationAcrossDevices(t *testing.T) {
	for _, tc := range []struct {
		name                                           string
		known, disconnect, silent, allNotOwned, remote bool
		want                                           error
	}{
		{name: "unknown owner"},
		{name: "known owner", known: true},
		{name: "non-owner first"},
		{name: "silent non-owner", silent: true},
		{name: "all non-owners", allNotOwned: true, want: ErrRunNotOwned},
		{name: "known owner disconnected", known: true, disconnect: true, allNotOwned: true, want: ErrNoAgentConnected},
		{name: "cross replica", remote: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := NewAgentDispatcher()
			holder := d
			if tc.remote {
				c := newCluster(t, "A", "B")
				d = c[0].dispatcher
				holder = c[1].dispatcher
			}
			a := connectClusterAgent(t, d, "u", "all", "non-owner")
			b := connectClusterAgent(t, holder, "u", "all", "owner")
			if tc.known {
				d.rememberRun("run", "u", "owner")
			}
			if tc.disconnect {
				b.Close()
				deadline := time.Now().Add(time.Second)
				for holder.Lookup("u", "p", "owner") != nil && time.Now().Before(deadline) {
					time.Sleep(time.Millisecond)
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- d.CancelRun(ctx, "u", "p", "task", "run") }()
			if tc.known && !tc.disconnect {
				launchReply(t, b, "")
			} else {
				if !tc.silent {
					launchReply(t, a, agentprotocol.RunNotOwned+" run")
				}
				if !tc.disconnect {
					summary := ""
					if tc.allNotOwned {
						summary = agentprotocol.RunNotOwned + " run"
					}
					launchReply(t, b, summary)
				}
			}
			err := <-done
			if !errors.Is(err, tc.want) {
				t.Fatalf("cancel=%v want=%v", err, tc.want)
			}
			if tc.want == nil && d.runDevice("run", "u") != "" {
				t.Fatal("completed cancellation retained ownership")
			}
		})
	}
}

func TestIncompleteCancellationAndDeadline(t *testing.T) {
	d := NewAgentDispatcher()
	connectClusterAgent(t, d, "u", "all", "silent")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	err := d.CancelRun(ctx, "u", "p", "task", "run")
	if err == nil || errors.Is(err, ErrRunNotOwned) || time.Since(started) > time.Second {
		t.Fatalf("incomplete result: %v after %s", err, time.Since(started))
	}
	d.mu.RLock()
	pending := len(d.pending)
	d.mu.RUnlock()
	// Cancellation return cancels child waits; cleanup can finish just afterwards.
	deadline := time.Now().Add(time.Second)
	for pending > 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
		d.mu.RLock()
		pending = len(d.pending)
		d.mu.RUnlock()
	}
	if pending != 0 {
		t.Fatalf("pending acknowledgments: %d", pending)
	}
}

func TestLaunchRecordsPinnedOwnerBeforeAcknowledgment(t *testing.T) {
	d := NewAgentDispatcher()
	a := connectClusterAgent(t, d, "u", "all", "owner")
	route, err := d.SelectRoute("u", "p", "")
	if err != nil {
		t.Fatal(err)
	}
	connectClusterAgent(t, d, "u", "all", "new-device")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- d.dispatchAndWaitRoute(ctx, route, "u", "p", "task", agentconfig.Dispatch{Action: "implement", RunID: "run"})
	}()
	readAgentMessage(t, a)
	if got := d.runDevice("run", "u"); got != "owner" {
		t.Fatalf("owner=%q", got)
	}
	if err := <-done; !errors.Is(err, ErrLaunchUnconfirmed) {
		t.Fatalf("unconfirmed=%v", err)
	}
	if got := d.runDevice("run", "u"); got != "owner" {
		t.Fatal("unconfirmed launch lost owner")
	}
}

func TestRejectedSelectionsHaveNoLaunchSideEffects(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	task, err := database.CreateTask(models.CreateTaskRequest{Title: "device selection", ProjectID: "default"})
	if err != nil {
		t.Fatal(err)
	}
	connectClusterAgent(t, h.agentDispatcher, ImplicitUser, "all", "laptop")
	connectClusterAgent(t, h.agentDispatcher, ImplicitUser, "all", "desktop")
	key, _, err := database.CreateAPIKey(ImplicitUser, "selection-tests", 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		path, body string
		status     int
	}{
		{"/api/tasks/" + task.ID + "/run-skill", `{"skillId":"implement"}`, 428},
		{"/api/tasks/" + task.ID + "/run-skill", `{"skillId":"implement","deviceId":"unknown"}`, 400},
		{"/api/tasks/" + task.ID + "/tty-external", `{}`, 428},
		{"/api/tasks/" + task.ID + "/tty-external", `{"deviceId":"unknown"}`, 400},
	} {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
		req.Header.Set("Authorization", "Bearer "+key)
		h.HandleTaskDetail(rr, req)
		if rr.Code != tc.status {
			t.Fatalf("%s %s: %d %s", tc.path, tc.body, rr.Code, rr.Body.String())
		}
	}
	active, err := database.ActiveRunOnTask(task.ID)
	if err != nil || active != nil {
		t.Fatalf("rejected request created run: %+v %v", active, err)
	}
}

func TestReconnectDoesNotSwitchDevices(t *testing.T) {
	d := NewAgentDispatcher()
	a := connectClusterAgent(t, d, "u", "all", "original")
	a.Close()
	deadline := time.Now().Add(time.Second)
	for d.Lookup("u", "p", "original") != nil && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := d.waitForSelectedRoute(ctx, "u", "p", "", time.Second); done <- err }()
	time.Sleep(30 * time.Millisecond)
	connectClusterAgent(t, d, "u", "all", "different")
	if err := <-done; err == nil {
		t.Fatal("reconnect wait migrated to different device")
	}
}

// Verify a request explicitly targeting an old remote generation cannot reach
// a replacement that may announce a different operation set.
func TestInternalDispatchRejectsReplacedGeneration(t *testing.T) {
	c := newCluster(t, "A", "B")
	connectClusterAgent(t, c[0].dispatcher, "u", "all", "laptop")
	old := c[0].dispatcher.Lookup("u", "p", "laptop")
	connectClusterAgent(t, c[0].dispatcher, "u", "all", "laptop")
	_, err := c[0].dispatcher.serveInternal(context.Background(), "dispatch", internalRequest{UserID: "u", ProjectID: "p", DeviceID: "laptop", ConnectedAt: old.ConnectedAt, Type: "pty_input"})
	if !errors.Is(err, ErrNoAgentConnected) {
		t.Fatalf("old generation: %v", err)
	}
}

func TestSlowRecordedOwnerCanStillConfirmDuringFallback(t *testing.T) {
	d := NewAgentDispatcher()
	owner := connectClusterAgent(t, d, "u", "all", "owner")
	other := connectClusterAgent(t, d, "u", "all", "other")
	d.rememberRun("run", "u", "owner")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- d.CancelRun(ctx, "u", "p", "task", "run") }()
	request := readAgentMessage(t, owner)
	launchReply(t, other, agentprotocol.RunNotOwned+" run")
	if err := owner.WriteJSON(AgentMessage{MsgID: request.MsgID, Type: "step_status", Payload: json.RawMessage(`{"status":"completed"}`)}); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestGenericDispatchExplicitDeviceAndNoSideEffectsOnRefusal(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	key, _, err := database.CreateAPIKey(ImplicitUser, "dispatch-tests", 0)
	if err != nil {
		t.Fatal(err)
	}
	connectClusterAgent(t, h.agentDispatcher, ImplicitUser, "all", "a")
	b := connectClusterAgent(t, h.agentDispatcher, ImplicitUser, "all", "b")
	for _, tc := range []struct {
		device string
		status int
	}{{"", 428}, {"missing", 400}, {"b", 200}} {
		body, _ := json.Marshal(map[string]any{"projectId": "default", "action": "pty_input", "deviceId": tc.device})
		rr := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/agent/dispatch", strings.NewReader(string(body)))
		r.Header.Set("Authorization", "Bearer "+key)
		h.HandleAgentDispatch(rr, r)
		if rr.Code != tc.status {
			t.Fatalf("device=%s: %d %s", tc.device, rr.Code, rr.Body.String())
		}
	}
	if msg := readAgentMessage(t, b); msg.Type != "pty_input" {
		t.Fatalf("dispatch=%+v", msg)
	}
}

func TestLaunchAcknowledgmentCannotCrossDevices(t *testing.T) {
	d := NewAgentDispatcher()
	a := connectClusterAgent(t, d, "u", "all", "a")
	connectClusterAgent(t, d, "u", "all", "b")
	route, err := d.SelectRoute("u", "p", "a")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- d.dispatchAndWaitRoute(ctx, route, "u", "p", "task", agentconfig.Dispatch{RunID: "run", Action: "implement"})
	}()
	request := readAgentMessage(t, a)
	reply := AgentMessage{MsgID: request.MsgID, Type: "step_status", Payload: json.RawMessage(`{"status":"completed"}`)}
	d.ReportLaunchStatus(d.Lookup("u", "p", "b"), reply)
	select {
	case err := <-done:
		t.Fatalf("other device confirmed launch: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	if err := a.WriteJSON(reply); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestConnectionDeviceIdentityFallbacks(t *testing.T) {
	for _, tc := range []struct {
		name, query, want string
		shared            bool
	}{{"query", "explicit", "explicit", false}, {"credential", "", "", false}, {"legacy unknown", "", "unknown", true}} {
		t.Run(tc.name, func(t *testing.T) {
			h, database, cleanup := setupTestHandler(t)
			defer cleanup()
			h.pullOnConnect = false
			key, credential, err := database.CreateAPIKey(ImplicitUser, "identity-test", 0)
			if err != nil {
				t.Fatal(err)
			}
			want := tc.want
			if want == "" {
				want = credential.ID
			}
			if tc.shared {
				key = "shared-test-key"
				t.Setenv("SECTILE_SERVER_TOKEN", key)
			}
			srv := httptest.NewServer(http.HandlerFunc(h.HandleAgentConnect))
			defer srv.Close()
			agent, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"?token="+key+"&deviceId="+tc.query, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer agent.Close()
			deadline := time.Now().Add(time.Second)
			for time.Now().Before(deadline) {
				if got := h.agentDispatcher.Lookup(ImplicitUser, "default", want); got != nil {
					return
				}
				time.Sleep(time.Millisecond)
			}
			t.Fatalf("identity=%q not registered: %+v", want, h.agentDispatcher.ConnectedAgents())
		})
	}
}

func TestUnconfirmedRemoteDispatchRetainsLaunchOwnership(t *testing.T) {
	c := newCluster(t, "A", "B")
	connectClusterAgent(t, c[0].dispatcher, "u", "all", "owner")
	// Simulate a holding instance that accepted a request but cannot return its
	// confirmation. The caller cannot infer that the launch failed.
	silent := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-time.After(100 * time.Millisecond) }))
	defer silent.Close()
	route, err := c[1].dispatcher.SelectRoute("u", "p", "owner")
	if err != nil {
		t.Fatal(err)
	}
	route.remote.Address = silent.URL
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err = c[1].dispatcher.dispatchAndWaitRoute(ctx, route, "u", "p", "task", agentconfig.Dispatch{Action: "implement", RunID: "run"})
	if !errors.Is(err, ErrLaunchUnconfirmed) || c[1].dispatcher.runDevice("run", "u") != "owner" {
		t.Fatalf("unconfirmed=%v owner=%s", err, c[1].dispatcher.runDevice("run", "u"))
	}
}

func TestUnreadableRemoteLaunchConfirmationRetainsOwnership(t *testing.T) {
	c := newCluster(t, "A", "B")
	connectClusterAgent(t, c[0].dispatcher, "u", "all", "owner")
	route, err := c[1].dispatcher.SelectRoute("u", "p", "owner")
	if err != nil {
		t.Fatal(err)
	}
	// A proxy may truncate the response after the holding instance launched.
	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"value":`))
	}))
	defer broken.Close()
	route.remote.Address = broken.URL
	err = c[1].dispatcher.dispatchAndWaitRoute(context.Background(), route, "u", "p", "task", agentconfig.Dispatch{Action: "implement", RunID: "run"})
	if !errors.Is(err, ErrLaunchUnconfirmed) || c[1].dispatcher.runDevice("run", "u") != "owner" {
		t.Fatalf("unconfirmed=%v owner=%s", err, c[1].dispatcher.runDevice("run", "u"))
	}
}

func TestRemoteOperationLaunchTransportFailuresAreUnconfirmed(t *testing.T) {
	c := newCluster(t, "A", "B")
	connectClusterAgent(t, c[0].dispatcher, "u", "all", "owner")
	route, err := c[1].dispatcher.SelectRoute("u", "p", "owner")
	if err != nil {
		t.Fatal(err)
	}
	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"value":`))
	}))
	defer broken.Close()
	for _, action := range []string{"execute_skill", "open_terminal", "git_status"} {
		for _, unreachable := range []bool{false, true} {
			location := *route.remote
			location.Address = broken.URL
			if unreachable {
				location.Address = "http://127.0.0.1:0"
			}
			_, err := c[1].dispatcher.cluster.operation(context.Background(), location, agentprotocol.Operation{UserID: "u", ProjectID: "p", Action: action})
			wantUnconfirmed := action != "git_status"
			if err == nil || errors.Is(err, ErrLaunchUnconfirmed) != wantUnconfirmed {
				t.Fatalf("action=%s unreachable=%v: %v", action, unreachable, err)
			}
		}
	}
}
