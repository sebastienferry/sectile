package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"tasks/internal/db"
)

// The desktop shows the server's board in a window of its own and signs every
// request of that window with the workstation API key, never with a session
// cookie (spike #396, ADR 0025). These tests pin what the board relies on: the
// key alone opens the routes the web interface calls, live updates included,
// and nothing about a key-signed request looks like a browser session.
func boardKeyServer(t *testing.T, h *Handler) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/me", h.HandleCurrentUser)
	mux.HandleFunc("/api/me/tracker-credentials", h.HandleUserTrackerCredentials)
	mux.HandleFunc("/api/events", h.HandleEventsSSE)
	mux.HandleFunc("/api/tasks", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"actor": h.webSessionUser(r)})
	})
	mux.HandleFunc("/auth/logout", h.HandleLogout)
	server := httptest.NewServer(h.EnableCORS(h.RequireSession(mux)))
	t.Cleanup(server.Close)
	return server
}

func keyed(t *testing.T, server *httptest.Server, key, method, path string) *http.Response {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	request, err := http.NewRequestWithContext(ctx, method, server.URL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if key != "" {
		request.Header.Set("Authorization", "Bearer "+key)
	}
	// The board page is served by the server itself: its requests carry the
	// server's own origin, which a web route must not hold against them.
	request.Header.Set("Origin", server.URL)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { response.Body.Close() })
	return response
}

func TestTheWorkstationKeyAloneSignsTheBoardIn(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	t.Setenv("SECTILE_SERVER_TOKEN", "")
	user, err := database.SignInLocal("ada@example.com")
	if err != nil {
		t.Fatal(err)
	}
	key, _, err := database.CreateAPIKey(user.ID, "laptop", db.DefaultAPIKeyTTL)
	if err != nil {
		t.Fatal(err)
	}
	server := boardKeyServer(t, h)

	var me struct {
		SignedIn bool   `json:"signedIn"`
		Email    string `json:"email"`
	}
	response := keyed(t, server, key, http.MethodGet, "/api/me")
	if err := json.NewDecoder(response.Body).Decode(&me); err != nil {
		t.Fatal(err)
	}
	if !me.SignedIn || me.Email != "ada@example.com" {
		t.Fatalf("/api/me with the key = %+v", me)
	}

	// Writes are attributed to the key's owner, as they are to a session's.
	var actor struct{ Actor string }
	if err := json.NewDecoder(keyed(t, server, key, http.MethodPost, "/api/tasks").Body).Decode(&actor); err != nil {
		t.Fatal(err)
	}
	if actor.Actor != user.ID {
		t.Fatalf("a keyed write is attributed to %q, want %q", actor.Actor, user.ID)
	}

	if response := keyed(t, server, key, http.MethodGet, "/api/me/tracker-credentials"); response.StatusCode != http.StatusOK {
		t.Fatalf("personal credentials with the key: %d", response.StatusCode)
	}

	// Live updates: EventSource cannot set a header, which is why the desktop
	// adds it from outside the page. With it, the stream opens and delivers.
	// The stream sends its headers with its first event, so keep publishing
	// until the subscription is in place.
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		for {
			select {
			case <-stop:
				return
			case <-time.After(20 * time.Millisecond):
				h.BroadcastEvent(Event{Type: "board-key-probe"})
			}
		}
	}()
	events := keyed(t, server, key, http.MethodGet, "/api/events")
	if events.StatusCode != http.StatusOK || !strings.HasPrefix(events.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("event stream with the key: %d %q", events.StatusCode, events.Header.Get("Content-Type"))
	}
	if response := keyed(t, server, "", http.MethodGet, "/api/events"); response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("event stream without the key: %d", response.StatusCode)
	}
}

// Signing out ends a browser session, and a key-signed board has none: the
// server answers that it signed out, and the key still answers for its owner.
// This is why the desktop refuses /auth/ from its board window rather than let
// "sign out" pretend to work.
func TestSigningOutDoesNotEndAKeySignedBoard(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	t.Setenv("SECTILE_SERVER_TOKEN", "")
	user, err := database.SignInLocal("grace@example.com")
	if err != nil {
		t.Fatal(err)
	}
	key, _, err := database.CreateAPIKey(user.ID, "laptop", db.DefaultAPIKeyTTL)
	if err != nil {
		t.Fatal(err)
	}
	server := boardKeyServer(t, h)
	if response := keyed(t, server, key, http.MethodPost, "/auth/logout"); response.StatusCode != http.StatusOK {
		t.Fatalf("logout: %d", response.StatusCode)
	}
	if response := keyed(t, server, key, http.MethodPost, "/api/tasks"); response.StatusCode != http.StatusOK {
		t.Fatalf("the key stopped working after a sign-out: %d", response.StatusCode)
	}

	// A key that expires, by contrast, leaves the board signed out.
	expireKey(t, h, key)
	if response := keyed(t, server, key, http.MethodPost, "/api/tasks"); response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expired key on a board route: %d", response.StatusCode)
	}
	var me struct {
		SignedIn bool `json:"signedIn"`
	}
	if err := json.NewDecoder(keyed(t, server, key, http.MethodGet, "/api/me").Body).Decode(&me); err != nil {
		t.Fatal(err)
	}
	if me.SignedIn {
		t.Fatal("an expired key still reads as signed in")
	}
}
