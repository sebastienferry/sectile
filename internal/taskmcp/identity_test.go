package taskmcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"tasks/internal/db"
)

// A session id is the only thing a request carries that can lead another
// instance to the one holding the session (#408).
func TestSessionOwnerReadsTheInstanceOffTheID(t *testing.T) {
	for id, want := range map[string]string{
		"8b1c2d3e-0000-4000-8000-000000000001.ABCDEF234567": "8b1c2d3e-0000-4000-8000-000000000001",
		"instance.random.with.dots":                         "instance",
		"ABCDEF234567":                                      "",
		"":                                                  "",
		".random":                                           "",
	} {
		if got := SessionOwner(id); got != want {
			t.Errorf("SessionOwner(%q) = %q, want %q", id, got, want)
		}
	}
}

// Every session this server creates names the instance it lives on, whatever
// the storage engine, and the sessions view says which instance that is.
func TestSessionsNameTheirInstance(t *testing.T) {
	database, err := db.NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	registry := NewSessionRegistry(database)
	defer registry.Stop()
	registry.SetInstance(database.InstanceID())

	server := NewServerWithCallers(database, registry, nil)
	srv := httptest.NewServer(mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{JSONResponse: true},
	))
	defer srv.Close()

	ctx := context.Background()
	session, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil).Connect(ctx, &mcp.StreamableClientTransport{Endpoint: srv.URL}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	id := session.ID()
	if !strings.HasPrefix(id, database.InstanceID()+".") || len(id) <= len(database.InstanceID())+1 {
		t.Fatalf("session id = %q, want %q followed by a random part", id, database.InstanceID()+".")
	}
	if owner := SessionOwner(id); owner != database.InstanceID() {
		t.Fatalf("SessionOwner(%q) = %q, want this instance", id, owner)
	}
	// The registry learns of the session once initialization completes, which
	// the client's first call after Connect guarantees.
	if _, err := session.ListTools(ctx, nil); err != nil {
		t.Fatal(err)
	}
	snapshot := registry.Snapshot()
	if len(snapshot) != 1 || snapshot[0].ID != id || snapshot[0].Instance != database.InstanceID() {
		t.Fatalf("snapshot = %+v, want the session on this instance", snapshot)
	}
}
