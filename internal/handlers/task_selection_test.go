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

	"tasks/internal/agentconfig"
	"tasks/internal/agentprotocol"
	"tasks/internal/models"
)

func reportTaskInventory(t *testing.T, d *AgentDispatcher, user, device string, projects ...string) {
	t.Helper()
	ac := d.Lookup(user, "all", device)
	if ac == nil {
		for _, candidate := range d.ActiveConnections() {
			if candidate.UserID == user && candidate.DeviceID == device {
				ac = candidate
			}
		}
	}
	if ac == nil {
		t.Fatal("missing inventory session")
	}
	if projects == nil {
		projects = []string{}
	}
	if err := d.acceptInventory(ac, agentprotocol.ProjectInventory{SchemaVersion: 1, ProjectIDs: projects}); err != nil {
		t.Fatal(err)
	}
}

func TestTaskRoutingInventoryAffinityAndRanking(t *testing.T) {
	d := NewAgentDispatcher()
	connectClusterAgent(t, d, "u", "all", "a")
	connectClusterAgent(t, d, "u", "all", "b")
	connectClusterAgent(t, d, "u", "all", "unknown")
	connectClusterAgent(t, d, "u", "all", "empty")
	connectClusterAgent(t, d, "u", "all", "wrong")
	connectClusterAgent(t, d, "other", "all", "secret")
	reportTaskInventory(t, d, "u", "a", "p")
	reportTaskInventory(t, d, "u", "b", "p")
	reportTaskInventory(t, d, "u", "empty")
	reportTaskInventory(t, d, "u", "wrong", "q")
	reportTaskInventory(t, d, "other", "secret", "p")
	for _, tc := range []struct {
		target, affinity, want string
		err                    error
	}{
		{"", "", "b", nil}, {"a", "b", "a", nil}, {"", "a", "a", nil},
		{"", "offline", "", ErrDeviceRequired}, {"b", "offline", "b", nil},
		{"secret", "", "", ErrInvalidDevice}, {"unknown", "", "", ErrInvalidDevice},
		{"empty", "", "", ErrInvalidDevice}, {"wrong", "", "", ErrInvalidDevice},
	} {
		route, err := d.selectTaskRoute(context.Background(), "u", "p", tc.target, tc.affinity, 0)
		if !errors.Is(err, tc.err) || tc.want != "" && (route == nil || route.DeviceID != tc.want) {
			t.Fatalf("%+v: route=%+v err=%v", tc, route, err)
		}
		if err != nil && strings.Contains(err.Error(), "secret") {
			t.Fatal("foreign device leaked")
		}
	}
	d.mu.Lock()
	d.agents[agentKey{UserID: "u", DeviceID: "a"}].ConnectedAt = d.agents[agentKey{UserID: "u", DeviceID: "b"}].ConnectedAt
	d.mu.Unlock()
	route, err := d.selectTaskRoute(context.Background(), "u", "p", "", "", 0)
	if err != nil || route.DeviceID != "a" {
		t.Fatalf("stable tie: %+v %v", route, err)
	}
	reportTaskInventory(t, d, "u", "a", "q")
	route, err = d.selectTaskRoute(context.Background(), "u", "p", "", "", 0)
	if err != nil || route.DeviceID != "b" {
		t.Fatalf("filter before ranking: %+v %v", route, err)
	}
	if _, err := d.selectTaskRoute(context.Background(), "u", "p", "", "a", 0); !errors.Is(err, ErrDeviceRequired) {
		t.Fatalf("ineligible affinity: %v", err)
	}
	if _, err := d.selectTaskRoute(context.Background(), "u", "missing-project", "", "", 0); !errors.Is(err, ErrDeviceRequired) {
		t.Fatalf("no inventory: %v", err)
	}
}

func TestTaskSelectedSessionCannotDispatchAfterReplacement(t *testing.T) {
	d := NewAgentDispatcher()
	connectClusterAgent(t, d, "u", "all", "a")
	reportTaskInventory(t, d, "u", "a", "p")
	route, err := d.selectTaskRoute(context.Background(), "u", "p", "a", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	connectClusterAgent(t, d, "u", "all", "a")
	reportTaskInventory(t, d, "u", "a", "p")
	if err := d.dispatchAndWaitRoute(context.Background(), route, "u", "p", "t", agentconfig.Dispatch{Action: "implement"}); !errors.Is(err, ErrNoAgentConnected) {
		t.Fatalf("displaced route dispatched: %v", err)
	}
}

func TestTaskLaunchAliasesPersistTheChosenDevice(t *testing.T) {
	for _, body := range []string{`{"skillId":"implement","targetDeviceId":"a"}`, `{"skillId":"implement","deviceId":"a"}`, `{"skillId":"implement","deviceId":" a ","targetDeviceId":"a"}`} {
		t.Run(body, func(t *testing.T) {
			h, database, cleanup := setupTestHandler(t)
			defer cleanup()
			task := guardTask(t, database, "targeted launch")
			a := connectClusterAgent(t, h.agentDispatcher, ImplicitUser, "all", "a")
			connectClusterAgent(t, h.agentDispatcher, ImplicitUser, "all", "b")
			reportTaskInventory(t, h.agentDispatcher, ImplicitUser, "a", "default")
			reportTaskInventory(t, h.agentDispatcher, ImplicitUser, "b", "default")
			if err := database.AddTaskActivity(models.TaskActivity{ID: "previous", TaskID: task.ID, RunProjectID: "default", UserID: ImplicitUser, DeviceID: "b", SkillID: "agent_launch", Status: "completed", CreatedAt: time.Now().Add(-time.Hour)}); err != nil {
				t.Fatal(err)
			}
			key, _, err := database.CreateAPIKey(ImplicitUser, "test", 0)
			if err != nil {
				t.Fatal(err)
			}
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/tasks/"+task.ID+"/run-skill", strings.NewReader(body))
			req.Header.Set("Authorization", "Bearer "+key)
			done := make(chan struct{})
			go func() { h.HandleTaskDetail(rr, req); close(done) }()
			msg := readAgentMessage(t, a)
			payload, _ := json.Marshal(map[string]string{"status": "completed"})
			if err := a.WriteJSON(AgentMessage{Type: "step_status", MsgID: msg.MsgID, Payload: payload}); err != nil {
				t.Fatal(err)
			}
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("launch not confirmed")
			}
			if rr.Code != http.StatusOK {
				t.Fatalf("launch: %d %s", rr.Code, rr.Body.String())
			}
			var dispatch agentconfig.Dispatch
			if err := json.Unmarshal(msg.Payload, &dispatch); err != nil {
				t.Fatal(err)
			}
			run, err := database.GetActivityByID(dispatch.RunID)
			if err != nil || run == nil || run.DeviceID != "a" {
				t.Fatalf("persisted run: %+v %v", run, err)
			}
			device, err := database.TaskLaunchDevice(task.ID, "default", ImplicitUser)
			if err != nil || device != "a" {
				t.Fatalf("launch affinity: %s %v", device, err)
			}
		})
	}
}

func TestConflictingTaskTargetsHaveNoSideEffects(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	task := guardTask(t, database, "alias conflict")
	key, _, err := database.CreateAPIKey(ImplicitUser, "test", 0)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/tasks/"+task.ID+"/run-skill", strings.NewReader(`{"skillId":"implement","deviceId":"a","targetDeviceId":"b"}`))
	req.Header.Set("Authorization", "Bearer "+key)
	rr := httptest.NewRecorder()
	h.HandleTaskDetail(rr, req)
	if rr.Code != 400 || activityCount(t, database, task.ID) != 0 {
		t.Fatalf("conflict side effects: %d %s", rr.Code, rr.Body.String())
	}
}

func TestTaskTargetReconnectRequiresFreshInventory(t *testing.T) {
	d := NewAgentDispatcher()
	connectClusterAgent(t, d, "u", "all", "a")
	reportTaskInventory(t, d, "u", "a", "p")
	route, err := d.selectTaskRoute(context.Background(), "u", "p", "a", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	d.Unregister("u", "a", route.local.Conn)
	connectClusterAgent(t, d, "u", "all", "b")
	reportTaskInventory(t, d, "u", "b", "p")
	if _, err := d.selectTaskRoute(context.Background(), "u", "p", "", "", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := d.selectTaskRoute(context.Background(), "u", "p", "", "a", 0); !errors.Is(err, ErrDeviceRequired) {
		t.Fatalf("unavailable affinity fell back: %v", err)
	}
	answers := make(chan *AgentRoute, 1)
	go func() {
		selected, _ := d.selectTaskRoute(context.Background(), "u", "p", "a", "", time.Second)
		answers <- selected
	}()
	connectClusterAgent(t, d, "u", "all", "a")
	select {
	case <-answers:
		t.Fatal("replacement qualified without inventory")
	case <-time.After(75 * time.Millisecond):
	}
	reportTaskInventory(t, d, "u", "a", "p")
	select {
	case selected := <-answers:
		if selected == nil || selected.DeviceID != "a" {
			t.Fatalf("reconnect: %+v", selected)
		}
	case <-time.After(time.Second):
		t.Fatal("reconnect not selected")
	}
}
