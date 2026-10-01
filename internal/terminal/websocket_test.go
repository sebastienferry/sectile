package terminal

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gorilla/websocket"
)

// A connection has a single writer at a time: a viewer joining or pinging a
// session that is printing must not write to its socket while the read loop
// broadcasts to it, or gorilla/websocket panics on the concurrent write.
func TestViewersJoinAndPingWhileTheSessionPrints(t *testing.T) {
	m := NewManager()
	defer func() { _ = m.CloseSession("busy") }()
	if _, err := m.GetOrCreateSession("busy", t.TempDir(), nil); err != nil {
		t.Fatalf("session: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.HandleWebSocket(w, r, "busy", "", nil)
	}))
	defer server.Close()
	url := "ws" + strings.TrimPrefix(server.URL, "http")

	// The PTY echoes what is typed, so typing keeps the session printing on
	// every platform without depending on the shell.
	printing := make(chan struct{})
	go func() {
		defer close(printing)
		for i := 0; i < 300; i++ {
			if err := m.SendInput("busy", "echo"); err != nil {
				return
			}
		}
	}()

	var viewers sync.WaitGroup
	for v := 0; v < 4; v++ {
		viewers.Add(1)
		go func() {
			defer viewers.Done()
			conn, _, err := websocket.DefaultDialer.Dial(url, nil)
			if err != nil {
				t.Error(err)
				return
			}
			defer conn.Close()
			go func() {
				for {
					if _, _, err := conn.ReadMessage(); err != nil {
						return
					}
				}
			}()
			for i := 0; i < 50; i++ {
				if err := conn.WriteJSON(WsMessage{Type: "ping"}); err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	viewers.Wait()
	<-printing
}
