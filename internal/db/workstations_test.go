package db

import (
	"testing"
	"time"
)

func TestWorkstationDirectoryRetainsDevicesAndGuardsSessions(t *testing.T) {
	a, b := twoServingStores(t)
	now := time.Now().UTC()
	first := AgentLocation{ConnectionID: "first", UserID: "u", ProjectID: "all", DeviceID: "a", ConnectedAt: now}
	second := AgentLocation{ConnectionID: "second", UserID: "u", ProjectID: "all", DeviceID: "b", ConnectedAt: now.Add(time.Millisecond)}
	if _, had, err := a.WorkstationConnected(first); err != nil || had {
		t.Fatalf("first registration: %v %v", had, err)
	}
	if _, had, err := b.WorkstationConnected(second); err != nil || had {
		t.Fatalf("distinct device replaced another: %v %v", had, err)
	}
	if err := a.WorkstationInventory(first.ConnectionID, []string{"p"}, now); err != nil {
		t.Fatal(err)
	}
	if err := b.WorkstationInventory(second.ConnectionID, []string{"q"}, now); err != nil {
		t.Fatal(err)
	}
	if got := a.ConnectedWorkstations(); len(got) != 2 {
		t.Fatalf("sessions = %+v", got)
	}
	replacement := first
	replacement.ConnectionID = "replacement"
	replacement.ProjectID = "new-scope"
	replacement.ConnectedAt = now.Add(2 * time.Millisecond)
	old, had, err := b.WorkstationConnected(replacement)
	if err != nil || !had || old.ConnectionID != "first" || old.InstanceID != a.InstanceID() {
		t.Fatalf("replacement = %+v %v %v", old, had, err)
	}
	for _, l := range a.ConnectedWorkstations() {
		if l.DeviceID == "a" && (l.ProjectID != "new-scope" || l.InventoryState != "unknown" || len(l.ProjectIDs) != 0) {
			t.Fatalf("replacement retained old scope or inventory: %+v", l)
		}
	}
	if err := a.WorkstationInventory("first", []string{"leaked"}, now); err == nil {
		t.Fatal("accepted replaced inventory")
	}
	if err := a.WorkstationDisconnected("first"); err != nil {
		t.Fatal(err)
	}
	if got := a.ConnectedWorkstations(); len(got) != 2 {
		t.Fatalf("late teardown removed replacement: %+v", got)
	}
	if err := b.WorkstationInventory("replacement", []string{}, now); err != nil {
		t.Fatal(err)
	}
	if err := b.WorkstationDisconnected("replacement"); err != nil {
		t.Fatal(err)
	}
	owner, ok := a.AgentOwner("u", "p")
	if !ok || owner.DeviceID != "b" {
		t.Fatalf("route did not fall back: %+v %v", owner, ok)
	}
	if _, err := a.conn.Exec(`UPDATE server_instances SET last_seen=? WHERE id=?`, now.Add(-2*instanceDeadAfter), b.InstanceID()); err != nil {
		t.Fatal(err)
	}
	if got := a.ConnectedWorkstations(); len(got) != 0 {
		t.Fatalf("stale instance remained visible: %+v", got)
	}
}

func TestPostgresWorkstationSessions(t *testing.T) {
	a := openPostgres(t)
	if _, err := a.conn.Exec(`DELETE FROM agent_workstation_presence`); err != nil {
		t.Fatal(err)
	}
	b, err := Open(Config{Driver: DriverPostgres, DSN: postgresDSN(t)})
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	stopA, err := a.StartInstance()
	if err != nil {
		t.Fatal(err)
	}
	defer stopA()
	stopB, err := b.StartInstance()
	if err != nil {
		t.Fatal(err)
	}
	defer stopB()
	now := time.Now().UTC()
	first := AgentLocation{ConnectionID: "pg-first", UserID: "pg-user", DeviceID: "a", ProjectID: "all", ConnectedAt: now}
	second := first
	second.ConnectionID = "pg-second"
	second.DeviceID = "b"
	second.ConnectedAt = now.Add(time.Millisecond)
	if _, had, err := a.WorkstationConnected(first); err != nil || had {
		t.Fatalf("first: %v %v", had, err)
	}
	if _, had, err := b.WorkstationConnected(second); err != nil || had {
		t.Fatalf("distinct: %v %v", had, err)
	}
	if err := a.WorkstationInventory(first.ConnectionID, []string{"p"}, now); err != nil {
		t.Fatal(err)
	}
	if err := b.WorkstationInventory(second.ConnectionID, []string{"p"}, now); err != nil {
		t.Fatal(err)
	}
	if got := a.ConnectedWorkstations(); len(got) != 2 {
		t.Fatalf("sessions=%+v", got)
	}
	replacement := first
	replacement.ConnectionID = "pg-replacement"
	replacement.ProjectID = "new-scope"
	replacement.ConnectedAt = now.Add(2 * time.Millisecond)
	old, had, err := b.WorkstationConnected(replacement)
	if err != nil || !had || old.ConnectionID != first.ConnectionID {
		t.Fatalf("replacement=%+v %v %v", old, had, err)
	}
	if err := a.WorkstationDisconnected(first.ConnectionID); err != nil {
		t.Fatal(err)
	}
	if err := b.WorkstationDisconnected(replacement.ConnectionID); err != nil {
		t.Fatal(err)
	}
	if owner, ok := a.AgentOwner(first.UserID, "p"); !ok || owner.DeviceID != second.DeviceID {
		t.Fatalf("fallback=%+v %v", owner, ok)
	}
	if err := b.WorkstationDisconnected(second.ConnectionID); err != nil {
		t.Fatal(err)
	}
}
