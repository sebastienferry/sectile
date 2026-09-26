package db

import (
	"path/filepath"
	"testing"
	"time"
)

// twoServingStores opens two stores on one database file and registers both as
// serving instances, as two server processes sharing a database would.
func twoServingStores(t *testing.T) (*DB, *DB) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "presence.db")
	stores := make([]*DB, 2)
	// Both are opened before either registers: opening a SQLite store forgets
	// every instance, since SQLite assumes a single process.
	for i := range stores {
		d, err := NewDB(path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { d.Close() })
		stores[i] = d
	}
	for i, address := range []string{"http://10.0.0.1:8092", "http://10.0.0.2:8092"} {
		stores[i].SetInstanceAddress(address)
		stop, err := stores[i].StartInstance()
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(stop)
	}
	return stores[0], stores[1]
}

// An agent connected to one instance is found from the other, with the address
// to reach it; once it leaves, it is no longer found but still reads as a
// recent presence.
func TestAnAgentIsFoundFromAnyInstance(t *testing.T) {
	a, b := twoServingStores(t)

	if _, had, err := a.AgentConnected("u1", "p1", "laptop"); err != nil || had {
		t.Fatalf("first connection: had=%v err=%v", had, err)
	}
	owner, ok := b.AgentOwner("u1", "p1")
	if !ok || owner.InstanceID != a.InstanceID() || owner.Address != "http://10.0.0.1:8092" || owner.DeviceID != "laptop" {
		t.Fatalf("owner seen from the other instance = %+v (found %v)", owner, ok)
	}
	if got := b.ConnectedAgentLocations(); len(got) != 1 || got[0].InstanceID != a.InstanceID() {
		t.Errorf("connected agents = %+v", got)
	}

	a.AgentDisconnected("u1", "p1", "laptop")
	if _, ok := b.AgentOwner("u1", "p1"); ok {
		t.Fatal("a disconnected agent is still found")
	}
	if !b.AgentRecentlyConnected("u1", "p1", time.Now().UTC().Add(-time.Minute)) {
		t.Error("an agent that just left no longer reads as recent")
	}
	if b.AgentRecentlyConnected("u1", "p1", time.Now().UTC().Add(time.Minute)) {
		t.Error("an old departure reads as recent")
	}
}

// An agent registered for every project answers for any project.
func TestAnAgentForEveryProjectAnswersForAnyProject(t *testing.T) {
	a, b := twoServingStores(t)
	if _, _, err := a.AgentConnected("u1", "default", "laptop"); err != nil {
		t.Fatal(err)
	}
	if owner, ok := b.AgentOwner("u1", "p9"); !ok || owner.ProjectID != "default" {
		t.Fatalf("fallback owner = %+v (found %v)", owner, ok)
	}
}

// The same slot connecting through the other instance takes it over, and says
// which instance held it before; a late departure from the old holder does not
// release the new one.
func TestReconnectingThroughAnotherInstanceTakesTheSlotOver(t *testing.T) {
	a, b := twoServingStores(t)
	if _, _, err := a.AgentConnected("u1", "p1", "laptop"); err != nil {
		t.Fatal(err)
	}
	previous, had, err := b.AgentConnected("u1", "p1", "laptop")
	if err != nil || !had || previous.InstanceID != a.InstanceID() {
		t.Fatalf("previous holder = %+v had=%v err=%v", previous, had, err)
	}

	a.AgentDisconnected("u1", "p1", "laptop")
	if owner, ok := a.AgentOwner("u1", "p1"); !ok || owner.InstanceID != b.InstanceID() {
		t.Fatalf("the old holder's departure released the new holder: %+v %v", owner, ok)
	}
}

// The agents of an instance that stopped answering are not found, and the
// reaper removes them.
func TestTheAgentsOfADeadInstanceAreForgotten(t *testing.T) {
	a, b := twoServingStores(t)
	if _, _, err := a.AgentConnected("u1", "p1", "laptop"); err != nil {
		t.Fatal(err)
	}
	stale := time.Now().UTC().Add(-2 * instanceDeadAfter)
	if _, err := b.conn.Exec(`UPDATE server_instances SET last_seen = ? WHERE id = ?`, stale, a.InstanceID()); err != nil {
		t.Fatal(err)
	}
	if _, ok := b.AgentOwner("u1", "p1"); ok {
		t.Fatal("an agent of a dead instance is still found")
	}
	if _, err := b.reclaimDeadInstances(time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	var rows int
	if err := b.conn.QueryRow(`SELECT COUNT(*) FROM agent_presence`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 0 {
		t.Errorf("%d presence rows left after the reaper", rows)
	}
}

// Every instance of one database derives the same internal bearer.
func TestTheInternalTokenIsSharedByTheInstances(t *testing.T) {
	a, b := twoServingStores(t)
	ta, errA := a.InternalToken()
	tb, errB := b.InternalToken()
	if errA != nil || errB != nil {
		t.Fatalf("tokens: %v %v", errA, errB)
	}
	if ta == "" || ta != tb {
		t.Fatalf("tokens differ or are empty: %q %q", ta, tb)
	}
}
