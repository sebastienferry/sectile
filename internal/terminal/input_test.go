package terminal

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// inputRecorder collects what an input listener was shown.
type inputRecorder struct {
	mu   sync.Mutex
	seen []string
}

func (r *inputRecorder) record(data []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seen = append(r.seen, string(data))
}

func (r *inputRecorder) waitFor(t *testing.T, want string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		r.mu.Lock()
		for _, got := range r.seen {
			if got == want {
				r.mu.Unlock()
				return
			}
		}
		seen := append([]string(nil), r.seen...)
		r.mu.Unlock()
		if time.Now().After(deadline) {
			t.Fatalf("the listener never saw %q; it saw %q", want, seen)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (r *inputRecorder) contains(fragment string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, got := range r.seen {
		if strings.Contains(got, fragment) {
			return true
		}
	}
	return false
}

// Whatever a viewer types reaches the input listeners, whichever way it came:
// the console WebSocket in binary, raw text or JSON, or relayed from the web
// terminal. The lines the agent types itself are not a viewer's (#475).
func TestInputListenersSeeWhatAViewerTypes(t *testing.T) {
	m := NewManager()
	defer func() { _ = m.CloseSession("viewer-input") }()
	if _, err := m.GetOrCreateSession("viewer-input", t.TempDir(), nil); err != nil {
		t.Fatalf("session: %v", err)
	}
	recorder := &inputRecorder{}
	if !m.AddInputListener("viewer-input", recorder.record) {
		t.Fatal("the listener was not registered on an existing session")
	}
	if m.AddInputListener("no-such-session", recorder.record) {
		t.Fatal("a listener was reported registered on a missing session")
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.HandleWebSocket(w, r, "viewer-input", "", nil)
	}))
	defer server.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	if err := conn.WriteMessage(websocket.BinaryMessage, []byte("binary\r")); err != nil {
		t.Fatal(err)
	}
	recorder.waitFor(t, "binary\r")
	if err := conn.WriteMessage(websocket.TextMessage, []byte("raw text\r")); err != nil {
		t.Fatal(err)
	}
	recorder.waitFor(t, "raw text\r")
	if err := conn.WriteJSON(WsMessage{Type: "input", Data: "structured\r"}); err != nil {
		t.Fatal(err)
	}
	recorder.waitFor(t, "structured\r")
	if err := conn.WriteJSON(WsMessage{Type: "resize", Cols: 80, Rows: 24}); err != nil {
		t.Fatal(err)
	}

	if err := m.SendViewerInput("viewer-input", "relayed\r"); err != nil {
		t.Fatal(err)
	}
	recorder.waitFor(t, "relayed\r")

	if err := m.SendInput("viewer-input", "agent line\n"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if recorder.contains("agent line") || recorder.contains("resize") {
		t.Fatal("the listener was shown input no viewer typed")
	}
}
