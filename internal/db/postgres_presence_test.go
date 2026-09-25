package db

import "testing"

// The presence queries on the engine several instances actually share.
func TestPostgresAgentPresence(t *testing.T) {
	a := openPostgres(t)
	if _, err := a.conn.Exec(`DELETE FROM server_instances`); err != nil {
		t.Fatal(err)
	}
	if _, err := a.conn.Exec(`DELETE FROM agent_presence`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = a.conn.Exec(`DELETE FROM agent_presence`)
		_, _ = a.conn.Exec(`DELETE FROM server_instances`)
	})
	b, err := Open(Config{Driver: DriverPostgres, DSN: postgresDSN(t)})
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	a.SetInstanceAddress("http://10.0.0.1:8092")
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

	if _, _, err := a.AgentConnected("u1", "p1", "laptop"); err != nil {
		t.Fatal(err)
	}
	owner, ok := b.AgentOwner("u1", "p1")
	if !ok || owner.InstanceID != a.InstanceID() || owner.Address != "http://10.0.0.1:8092" {
		t.Fatalf("owner = %+v (found %v)", owner, ok)
	}
	previous, had, err := b.AgentConnected("u1", "p1", "laptop")
	if err != nil || !had || previous.InstanceID != a.InstanceID() {
		t.Fatalf("takeover: %+v %v %v", previous, had, err)
	}
	if got := a.ConnectedAgentLocations(); len(got) != 1 || got[0].InstanceID != b.InstanceID() {
		t.Errorf("connected agents = %+v", got)
	}
}
