package handlers

import (
	"errors"
	"net/http/httptest"
	"testing"

	"tasks/internal/db"
)

// startPostgresCredentialNode makes a store a serving instance whose internal
// listener serves the held keys, found by the others through server_instances.
func startPostgresCredentialNode(t *testing.T, d *db.DB) *credentialNode {
	t.Helper()
	n := &credentialNode{h: NewHandler(d), db: d}
	t.Cleanup(n.h.mcpSessions.Stop)
	n.internal = httptest.NewServer(n.h.InternalCredentialsHandler())
	t.Cleanup(n.internal.Close)
	d.SetInstanceAddress(n.internal.URL)
	stop, err := d.StartInstance()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(stop)
	n.h.setCredentialCluster(d, mcpClusterToken, nil)
	return n
}

// Two instances on one PostgreSQL database: a credential unlocked through A
// serves B, a lock through A holds on B, and an instance started afterwards
// takes the key over (#409).
func TestPostgresUnlockedCredentialAcrossTwoInstances(t *testing.T) {
	t.Setenv("SECTILE_SECRET_KEY", "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f")
	a := startPostgresCredentialNode(t, openPostgresInstance(t))
	b := startPostgresCredentialNode(t, openPostgresInstance(t))
	user := "u-" + a.id()[:8]

	if err := a.db.SetUserTrackerCredential(user, "jira", "https://acme.atlassian.net", "ada@example.com", "sealed-token", "open sesame"); err != nil {
		t.Fatal(err)
	}
	if err := a.db.LockUserTrackerCredential(user, "jira"); err != nil {
		t.Fatal(err)
	}
	a.settle()
	if err := a.db.UnlockUserTrackerCredential(user, "jira", "open sesame"); err != nil {
		t.Fatal(err)
	}
	a.settle()
	if _, _, token, err := b.db.UserTrackerCredentialsFor(user, "jira"); err != nil || token != "sealed-token" {
		t.Fatalf("B after the unlock through A: %q %v", token, err)
	}

	c := startPostgresCredentialNode(t, openPostgresInstance(t))
	if adopted := c.h.PullUnlockedKeys(); adopted < 1 {
		t.Fatalf("C adopted %d key(s), want the one A holds", adopted)
	}
	if _, _, token, err := c.db.UserTrackerCredentialsFor(user, "jira"); err != nil || token != "sealed-token" {
		t.Fatalf("C after pulling: %q %v", token, err)
	}

	// B stops hearing anything, and the lock still holds there.
	b.internal.Close()
	if err := a.db.LockUserTrackerCredential(user, "jira"); err != nil {
		t.Fatal(err)
	}
	a.settle()
	for name, node := range map[string]*credentialNode{"B": b, "C": c} {
		if _, _, _, err := node.db.UserTrackerCredentialsFor(user, "jira"); !errors.Is(err, db.ErrCredentialLocked) {
			t.Fatalf("%s after the lock through A: %v", name, err)
		}
	}
}
