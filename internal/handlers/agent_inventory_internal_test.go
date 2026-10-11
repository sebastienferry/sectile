package handlers

import (
	"context"
	"errors"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"encoding/json"
	"tasks/internal/agentconfig"
	"tasks/internal/agentprotocol"
	"tasks/internal/db"

	"github.com/gorilla/websocket"
)

func TestSharedWorkstationDirectoryAndExactForwarding(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cluster.db")
	stores := make([]*db.DB, 2)
	for i := range stores {
		var err error
		stores[i], err = db.NewDB(path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { stores[i].Close() })
	}
	dispatchers := []*AgentDispatcher{NewAgentDispatcher(), NewAgentDispatcher()}
	// Internal forwarding uses real servers, with the same database on both sides.
	for i := range stores {
		server := httptest.NewServer(dispatchers[i].InternalHandler())
		t.Cleanup(server.Close)
		stores[i].SetInstanceAddress(server.URL)
		stop, err := stores[i].StartInstance()
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(stop)
		dispatchers[i].SetCluster(stores[i], clusterToken, nil)
	}
	a := connectClusterAgent(t, dispatchers[0], "u", "all", "a")
	// A concrete legacy registration must not override the reported inventory.
	b := connectClusterAgent(t, dispatchers[1], "u", "q", "b")
	first := dispatchers[0].Lookup("u", "p", "a")
	second := dispatchers[1].Lookup("u", "q", "b")
	p := agentprotocol.ProjectInventory{SchemaVersion: 1, ProjectIDs: []string{"p"}}
	if err := dispatchers[0].acceptInventory(first, p); err != nil {
		t.Fatal(err)
	}
	if err := dispatchers[1].acceptInventory(second, p); err != nil {
		t.Fatal(err)
	}
	for _, d := range dispatchers {
		if got := d.AvailableWorkstations("u", "p"); len(got) != 2 {
			t.Fatalf("cross-instance availability=%+v", got)
		}
		if got := d.AvailableWorkstations("foreign", "p"); len(got) != 0 {
			t.Fatal("foreign inventory leaked")
		}
	}
	// Task routing sees the same strict inventory on both replicas and sends
	// only to the selected remote session, retaining that device on failure.
	for _, dispatcher := range dispatchers {
		route, err := dispatcher.selectTaskRoute(context.Background(), "u", "p", "", "a", 0)
		if err != nil || route.DeviceID != "a" {
			t.Fatalf("replica affinity: %+v %v", route, err)
		}
	}
	route, err := dispatchers[0].selectTaskRoute(context.Background(), "u", "p", "b", "a", 0)
	if err != nil || route.remote == nil {
		t.Fatalf("explicit remote route: %+v %v", route, err)
	}
	result := make(chan error, 1)
	go func() {
		result <- dispatchers[0].dispatchAndWaitRoute(context.Background(), route, "u", "p", "task", agentconfig.Dispatch{Action: "implement", RunID: "remote-task-run"})
	}()
	message := readAgentMessage(t, b)
	ack, _ := json.Marshal(map[string]string{"status": "completed"})
	if err := b.WriteJSON(AgentMessage{MsgID: message.MsgID, Type: "step_status", Payload: ack}); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("remote task launch not confirmed")
	}
	locations := stores[0].ConnectedWorkstations()
	var old db.AgentLocation
	for _, l := range locations {
		if l.DeviceID == "a" {
			old = l
		}
	}
	replacement := connectClusterAgent(t, dispatchers[1], "u", "p", "a")
	replacementConn := dispatchers[1].Lookup("u", "p", "a")
	if err := dispatchers[1].acceptInventory(replacementConn, p); err != nil {
		t.Fatal(err)
	}
	_ = a.SetReadDeadline(time.Now().Add(time.Second))
	var closed AgentMessage
	if err := a.ReadJSON(&closed); !websocket.IsCloseError(err, 4001) {
		t.Fatalf("old replica session not closed: %v", err)
	}
	if err := dispatchers[0].acceptInventory(first, p); err == nil {
		t.Fatal("stale snapshot accepted")
	}
	_, err = dispatchers[1].cluster.forward(context.Background(), old, "pull", internalRequest{UserID: "u", ProjectID: "all"})
	if !errors.Is(err, ErrNoAgentConnected) {
		t.Fatalf("stale forward retargeted: %v", err)
	}
	if got := dispatchers[0].AvailableWorkstations("u", "p"); len(got) != 2 {
		t.Fatalf("replacement lost another device: %+v", got)
	}
	a.Close()
	b.Close()
	replacement.Close()
}

func TestDistinctDeviceKeepsPendingOperationOwner(t *testing.T) {
	d := NewAgentDispatcher()
	a := connectClusterAgent(t, d, "u", "all", "a")
	result := make(chan error, 1)
	go func() {
		_, err := d.CallOperation(context.Background(), agentprotocol.Operation{UserID: "u", ProjectID: "p", Action: "cli_status"})
		result <- err
	}()
	request := readAgentMessage(t, a)
	b := connectClusterAgent(t, d, "u", "all", "b")
	if err := d.acceptInventory(d.Lookup("u", "p", "a"), agentprotocol.ProjectInventory{SchemaVersion: 1, ProjectIDs: []string{"p"}}); err != nil {
		t.Fatal(err)
	}
	if len(d.AvailableWorkstations("u", "p")) != 1 {
		t.Fatal("expected only A to advertise P")
	}
	if _, err := d.SelectRoute("u", "p", ""); !errors.Is(err, ErrDeviceRequired) {
		t.Fatalf("ambiguous dispatch should require device selection: %v", err)
	}
	// A reply from B cannot complete A's pending request.
	if err := b.WriteJSON(AgentMessage{Type: "workspace_result", MsgID: request.MsgID, Payload: []byte(`{"value":null}`)}); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		t.Fatalf("wrong owner completed request: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	if err := a.WriteJSON(AgentMessage{Type: "workspace_result", MsgID: request.MsgID, Payload: []byte(`{"value":null}`)}); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("original owner did not complete request")
	}
	b.Close()
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); time.Sleep(time.Millisecond) {
		if ac := d.Lookup("u", "p"); ac != nil && ac.DeviceID == "a" {
			return
		}
	}
	t.Fatal("route did not fall back to remaining device")
}

type failingInventoryDirectory struct {
	*db.DB
	fail        bool
	failConnect bool
}

func (f *failingInventoryDirectory) WorkstationConnected(l db.AgentLocation) (db.AgentLocation, bool, error) {
	if f.failConnect {
		return db.AgentLocation{}, false, errors.New("registration unavailable")
	}
	return f.DB.WorkstationConnected(l)
}
func (f *failingInventoryDirectory) WorkstationInventory(id string, ids []string, now time.Time) error {
	if f.fail {
		return errors.New("directory unavailable")
	}
	return f.DB.WorkstationInventory(id, ids, now)
}
func TestInventoryDirectoryFailureKeepsPublishedSnapshot(t *testing.T) {
	store, err := db.NewDB(filepath.Join(t.TempDir(), "failed-directory.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	stop, err := store.StartInstance()
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	directory := &failingInventoryDirectory{DB: store}
	d := NewAgentDispatcher()
	d.SetCluster(directory, clusterToken, nil)
	_ = connectClusterAgent(t, d, "u", "all", "a")
	ac := d.Lookup("u", "p", "a")
	if err := d.acceptInventory(ac, agentprotocol.ProjectInventory{SchemaVersion: 1, ProjectIDs: []string{"p"}}); err != nil {
		t.Fatal(err)
	}
	directory.fail = true
	if err := d.acceptInventory(ac, agentprotocol.ProjectInventory{SchemaVersion: 1, ProjectIDs: []string{"q"}}); err == nil {
		t.Fatal("failed directory write accepted")
	}
	if len(d.AvailableWorkstations("u", "p")) != 1 || len(d.AvailableWorkstations("u", "q")) != 0 {
		t.Fatal("failed write changed published snapshot")
	}
	directory.fail = false
	if err := d.acceptInventory(ac, agentprotocol.ProjectInventory{SchemaVersion: 1, ProjectIDs: []string{"q"}}); err != nil {
		t.Fatal(err)
	}
	if len(d.AvailableWorkstations("u", "q")) != 1 {
		t.Fatal("snapshot did not recover")
	}
}

func TestWorkstationTimestampsExcludeForeignUsers(t *testing.T) {
	now := time.Now().UTC()
	agents := []AgentConnInfo{
		{UserID: "u", DeviceID: "a", InventoryState: "reported", AvailableProjectIDs: []string{"p"}, ConnectedAt: now, LastPingAt: now},
		{UserID: "foreign", DeviceID: "a", InventoryState: "reported", AvailableProjectIDs: []string{"p"}, ConnectedAt: now.Add(-2 * time.Hour), LastPingAt: now.Add(2 * time.Minute)},
	}
	got := workstations(agents, "u", "p")
	if len(got) != 1 || !got[0].ConnectedAt.Equal(agents[0].ConnectedAt) || !got[0].LastPingAt.Equal(agents[0].LastPingAt) {
		t.Fatalf("aggregate=%+v", got)
	}
}

func TestFailedReplacementRegistrationWithdrawsOldSession(t *testing.T) {
	store, err := db.NewDB(filepath.Join(t.TempDir(), "failed-registration.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	stop, err := store.StartInstance()
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	directory := &failingInventoryDirectory{DB: store}
	d := NewAgentDispatcher()
	d.SetCluster(directory, clusterToken, nil)
	old := connectClusterAgent(t, d, "u", "all", "a")
	if err := d.acceptInventory(d.Lookup("u", "p", "a"), agentprotocol.ProjectInventory{SchemaVersion: 1, ProjectIDs: []string{"p"}}); err != nil {
		t.Fatal(err)
	}
	directory.failConnect = true
	failed := connectClusterAgent(t, d, "u", "all", "a")
	_ = old.SetReadDeadline(time.Now().Add(time.Second))
	var msg AgentMessage
	if err := old.ReadJSON(&msg); !websocket.IsCloseError(err, 4001) {
		t.Fatalf("old close=%v", err)
	}
	_ = failed.SetReadDeadline(time.Now().Add(time.Second))
	if err := failed.ReadJSON(&msg); !websocket.IsCloseError(err, websocket.CloseTryAgainLater) {
		t.Fatalf("failed registration close=%v", err)
	}
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); time.Sleep(time.Millisecond) {
		if len(store.ConnectedWorkstations()) == 0 {
			return
		}
	}
	t.Fatal("closed session retained published availability after failed replacement")
}
