package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"tasks/internal/db"
)

func readyAnswer(t *testing.T, h *Handler) (int, map[string]string) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.RequireSession(http.HandlerFunc(h.HandleReady)).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, ReadyPath, nil))
	var body map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return rec.Code, body
}

// The readiness probe answers without a session, says ready on a store that
// answers, and turns not ready as soon as the instance starts draining, while
// liveness is untouched (#410).
func TestTheReadinessProbeFollowsTheStoreAndTheDrain(t *testing.T) {
	database, err := db.NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(database)
	t.Cleanup(h.mcpSessions.Stop)

	if code, body := readyAnswer(t, h); code != http.StatusOK || body["status"] != "ready" || body["instance"] != database.InstanceID() {
		t.Fatalf("ready store: %d %v", code, body)
	}

	h.BeginDrain()
	if code, body := readyAnswer(t, h); code != http.StatusServiceUnavailable || body["reason"] == "" {
		t.Fatalf("draining: %d %v", code, body)
	}
	rec := httptest.NewRecorder()
	h.HandleHealth(rec, httptest.NewRequest(http.MethodGet, HealthPath, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("liveness while draining: %d", rec.Code)
	}
}

func TestTheReadinessProbeFailsOnAStoreThatDoesNotAnswer(t *testing.T) {
	database, err := db.NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(database)
	t.Cleanup(h.mcpSessions.Stop)
	database.Close()
	if code, body := readyAnswer(t, h); code != http.StatusServiceUnavailable || body["status"] != "not_ready" {
		t.Fatalf("closed store: %d %v", code, body)
	}
}
