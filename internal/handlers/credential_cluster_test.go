package handlers

import (
	"bytes"
	"encoding/base64"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"tasks/internal/db"
)

// These tests run #409 on server instances in one process. Their stores open
// one database file, so the credential rows are common and the held keys are
// not, which is the situation of replicas sharing PostgreSQL; each instance has
// its own internal listener, found through a directory the test controls.

type credentialNode struct {
	h        *Handler
	db       *db.DB
	internal *httptest.Server
}

func (n *credentialNode) id() string { return n.db.InstanceID() }

// settle waits for the pushes the node started.
func (n *credentialNode) settle() { n.h.credentialCluster.pending.Wait() }

func newCredentialNode(t *testing.T, shared *mcpInstances, path string) *credentialNode {
	t.Helper()
	database, err := db.NewDB(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	n := &credentialNode{h: NewHandler(database), db: database}
	t.Cleanup(n.h.mcpSessions.Stop)
	n.internal = httptest.NewServer(n.h.InternalCredentialsHandler())
	t.Cleanup(n.internal.Close)
	shared.set(n.id(), n.internal.URL)
	n.h.setCredentialCluster(&mcpInstanceView{shared: shared, id: n.id()}, mcpClusterToken, nil)
	return n
}

func newCredentialCluster(t *testing.T, count int) ([]*credentialNode, *mcpInstances, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tasks.db")
	shared := &mcpInstances{live: map[string]string{}}
	var nodes []*credentialNode
	for range count {
		nodes = append(nodes, newCredentialNode(t, shared, path))
	}
	return nodes, shared, path
}

// captureLog collects what the standard logger prints during a test.
func captureLog(t *testing.T) *lockedBuffer {
	t.Helper()
	buf := &lockedBuffer{}
	previous := log.Writer()
	log.SetOutput(buf)
	t.Cleanup(func() { log.SetOutput(previous) })
	return buf
}

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// The acceptance criterion: unlocked through A, it serves the tracker calls B
// makes as that person.
func TestACredentialUnlockedThroughOneInstanceServesAnother(t *testing.T) {
	logs := captureLog(t)
	nodes, _, _ := newCredentialCluster(t, 2)
	a, b := nodes[0], nodes[1]
	if err := a.db.SetUserTrackerCredential("u1", "jira", "https://acme.atlassian.net", "ada@example.com", "sealed-token", "open sesame"); err != nil {
		t.Fatal(err)
	}
	a.settle()
	if _, _, token, err := b.db.UserTrackerCredentialsFor("u1", "jira"); err != nil || token != "sealed-token" {
		t.Fatalf("B after the store through A: %q %v", token, err)
	}

	if err := a.db.LockUserTrackerCredential("u1", "jira"); err != nil {
		t.Fatal(err)
	}
	a.settle()
	if _, _, _, err := b.db.UserTrackerCredentialsFor("u1", "jira"); !errors.Is(err, db.ErrCredentialLocked) {
		t.Fatalf("B after the lock through A: %v", err)
	}

	if err := a.db.UnlockUserTrackerCredential("u1", "jira", "open sesame"); err != nil {
		t.Fatal(err)
	}
	a.settle()
	if _, _, token, err := b.db.UserTrackerCredentialsFor("u1", "jira"); err != nil || token != "sealed-token" {
		t.Fatalf("B after the unlock through A: %q %v", token, err)
	}
	listed, err := b.db.UserTrackerCredentials("u1")
	if err != nil || len(listed) != 1 || !listed[0].Unlocked {
		t.Fatalf("B must list it unlocked: %+v %v", listed, err)
	}

	keys, err := a.db.HeldKeys()
	if err != nil || len(keys) != 1 {
		t.Fatalf("held keys on A: %v %v", keys, err)
	}
	for _, secret := range []string{"open sesame", base64.StdEncoding.EncodeToString(keys[0].Wrapped)} {
		if strings.Contains(logs.String(), secret) {
			t.Fatalf("the logs carry key material:\n%s", logs.String())
		}
	}
}

// An instance that starts after the unlock takes the key over from its peers.
func TestAStartingInstancePullsTheKeysItsPeersHold(t *testing.T) {
	nodes, shared, path := newCredentialCluster(t, 1)
	a := nodes[0]
	if err := a.db.SetUserTrackerCredential("u1", "jira", "", "", "sealed-token", "open sesame"); err != nil {
		t.Fatal(err)
	}
	a.settle()

	c := newCredentialNode(t, shared, path)
	if _, _, _, err := c.db.UserTrackerCredentialsFor("u1", "jira"); !errors.Is(err, db.ErrCredentialLocked) {
		t.Fatalf("before pulling: %v", err)
	}
	if adopted := c.h.PullUnlockedKeys(); adopted != 1 {
		t.Fatalf("adopted %d key(s), want 1", adopted)
	}
	if _, _, token, err := c.db.UserTrackerCredentialsFor("u1", "jira"); err != nil || token != "sealed-token" {
		t.Fatalf("after pulling: %q %v", token, err)
	}
}

// A peer that cannot be reached misses the lock and still cannot use the key;
// the owner's lock succeeded all the same.
func TestALockHoldsOnAPeerTheMessageNeverReached(t *testing.T) {
	logs := captureLog(t)
	nodes, shared, _ := newCredentialCluster(t, 2)
	a, b := nodes[0], nodes[1]
	if err := a.db.SetUserTrackerCredential("u1", "jira", "", "", "sealed-token", "open sesame"); err != nil {
		t.Fatal(err)
	}
	a.settle()

	// B is listed at an address that answers nothing.
	shared.set(b.id(), "http://127.0.0.1:1")
	if err := a.db.LockUserTrackerCredential("u1", "jira"); err != nil {
		t.Fatalf("the lock must succeed with a peer down: %v", err)
	}
	a.settle()
	if !strings.Contains(logs.String(), b.id()) {
		t.Fatalf("the failed push must be logged with the peer:\n%s", logs.String())
	}
	if _, _, _, err := b.db.UserTrackerCredentialsFor("u1", "jira"); !errors.Is(err, db.ErrCredentialLocked) {
		t.Fatalf("B, which missed the lock, opened the credential: %v", err)
	}
}

// Storing the credential again and deleting it reach the peers too.
func TestANewRecordAndADeletionReachThePeers(t *testing.T) {
	nodes, _, _ := newCredentialCluster(t, 2)
	a, b := nodes[0], nodes[1]
	if err := a.db.SetUserTrackerCredential("u1", "jira", "", "", "old-token", "old phrase"); err != nil {
		t.Fatal(err)
	}
	a.settle()
	if err := a.db.SetUserTrackerCredential("u1", "jira", "", "", "new-token", "new phrase"); err != nil {
		t.Fatal(err)
	}
	a.settle()
	if _, _, token, err := b.db.UserTrackerCredentialsFor("u1", "jira"); err != nil || token != "new-token" {
		t.Fatalf("B after the new record: %q %v", token, err)
	}
	if err := a.db.ClearUserTrackerCredential("u1", "jira"); err != nil {
		t.Fatal(err)
	}
	a.settle()
	if keys, err := b.db.HeldKeys(); err != nil || len(keys) != 0 {
		t.Fatalf("B still holds keys after the deletion: %v %v", keys, err)
	}
}

// Only the other instances may read or push keys.
func TestTheInternalKeysEndpointRefusesAStranger(t *testing.T) {
	nodes, _, _ := newCredentialCluster(t, 1)
	a := nodes[0]
	if err := a.db.SetUserTrackerCredential("u1", "jira", "", "", "sealed-token", "open sesame"); err != nil {
		t.Fatal(err)
	}
	for _, bearer := range []string{"", "Bearer wrong", "Bearer " + mcpClientKey} {
		req, _ := http.NewRequest(http.MethodGet, a.internal.URL+internalCredentialKeysPath, nil)
		if bearer != "" {
			req.Header.Set("Authorization", bearer)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("bearer %q: %s, want 401", bearer, resp.Status)
		}
	}
	req, _ := http.NewRequest(http.MethodGet, a.internal.URL+internalCredentialKeysPath, nil)
	req.Header.Set("Authorization", "Bearer "+mcpClusterToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the internal credential: %s, want 200", resp.Status)
	}
}

// Without an internal credential nothing is shared, and the endpoint refuses.
func TestNoInternalCredentialSharesNothing(t *testing.T) {
	nodes, shared, _ := newCredentialCluster(t, 2)
	a, b := nodes[0], nodes[1]
	a.h.setCredentialCluster(&mcpInstanceView{shared: shared, id: a.id()}, "", errors.New("no server key"))
	if err := a.db.SetUserTrackerCredential("u1", "jira", "", "", "sealed-token", "open sesame"); err != nil {
		t.Fatal(err)
	}
	b.settle()
	if _, _, _, err := b.db.UserTrackerCredentialsFor("u1", "jira"); !errors.Is(err, db.ErrCredentialLocked) {
		t.Fatalf("B opened a credential nobody shared: %v", err)
	}
	if a.h.PullUnlockedKeys() != 0 {
		t.Fatal("an instance without the internal credential pulled keys")
	}
}
