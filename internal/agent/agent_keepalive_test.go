package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// keepaliveFixture wires a client connection to a server that can be told to
// stop reading. A server that stops reading backs the client's socket up, and a
// frame the client is writing then occupies the connection's write mutex until
// the socket drains — which is exactly what a large operation result does over
// a slow link, and the window in which a pong used to be discarded.
type keepaliveFixture struct {
	server *websocket.Conn
	client *websocket.Conn
	pongs  <-chan string
	resume chan struct{}
}

func newKeepaliveFixture(t *testing.T) *keepaliveFixture {
	t.Helper()
	pongs := make(chan string, 16)
	resume := make(chan struct{})
	serverConns := make(chan *websocket.Conn, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{ReadBufferSize: 1024, WriteBufferSize: 1024}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		conn.SetPongHandler(func(payload string) error {
			pongs <- payload
			return nil
		})
		serverConns <- conn
		// Read nothing until released: the client's writes back up.
		<-resume
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	t.Cleanup(srv.Close)

	// A write buffer large enough for the whole payload keeps it in a single
	// frame flush, so the connection is held for one uninterrupted write
	// instead of being released between chunks.
	dialer := &websocket.Dialer{WriteBufferSize: 20 << 20, ReadBufferSize: 4096}
	client, _, err := dialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })

	f := &keepaliveFixture{server: <-serverConns, client: client, pongs: pongs, resume: resume}
	// The client keeps processing control frames, as the agent's message loop
	// does.
	go func() {
		for {
			if _, _, err := client.ReadMessage(); err != nil {
				return
			}
		}
	}()
	return f
}

// blockWrite starts a payload far larger than the socket buffers, so the client
// sits inside a single frame write, holding the connection, until the server
// starts reading again.
func (f *keepaliveFixture) blockWrite(t *testing.T) <-chan error {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		_ = f.client.SetWriteDeadline(time.Now().Add(30 * time.Second))
		done <- f.client.WriteMessage(websocket.BinaryMessage, make([]byte, 16<<20))
	}()
	// Give the write time to fill the buffers and settle on the socket.
	time.Sleep(300 * time.Millisecond)
	return done
}

// The regression. gorilla's default ping handler writes the pong itself with a
// one-second budget and, because the write timeout it gets back is declared
// temporary, discards the reply and reports success. Nothing is logged, the
// server sees a silent agent and drops it.
func TestKeepaliveSurvivesAWriteThatHoldsTheConnection(t *testing.T) {
	f := newKeepaliveFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	installKeepalive(ctx, f.client)

	written := f.blockWrite(t)
	if err := f.server.WriteControl(websocket.PingMessage, []byte("probe"), time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	// Well past the one second the default handler would have waited before
	// throwing the reply away.
	time.Sleep(1500 * time.Millisecond)
	select {
	case <-f.pongs:
		t.Fatal("the write did not hold the connection: the test proves nothing")
	default:
	}

	close(f.resume)
	if err := <-written; err != nil {
		t.Fatalf("blocked write failed: %v", err)
	}
	select {
	case payload := <-f.pongs:
		if payload != "probe" {
			t.Fatalf("answered with %q instead of the ping payload", payload)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the keepalive probe was never answered: the pong was dropped")
	}
}

// Pongs are not cumulative: probes arriving while a reply is still pending must
// not queue one reply each.
func TestQueuedPongsDoNotAccumulate(t *testing.T) {
	f := newKeepaliveFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	installKeepalive(ctx, f.client)

	written := f.blockWrite(t)
	for i := 0; i < 5; i++ {
		if err := f.server.WriteControl(websocket.PingMessage, []byte("probe"), time.Now().Add(time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(500 * time.Millisecond)
	close(f.resume)
	if err := <-written; err != nil {
		t.Fatalf("blocked write failed: %v", err)
	}

	select {
	case <-f.pongs:
	case <-time.After(10 * time.Second):
		t.Fatal("no reply at all once the connection was free")
	}
	// No further probe is sent, so every extra reply is an accumulated one. At
	// most one more is expected: the sender takes a reply off the queue before
	// waiting on the write, which frees the single slot once.
	extra := 0
	deadline := time.After(time.Second)
	for done := false; !done; {
		select {
		case <-f.pongs:
			extra++
		case <-deadline:
			done = true
		}
	}
	if extra > 1 {
		t.Fatalf("five probes produced %d extra replies: they accumulate", extra)
	}
}

// The heartbeat is the keepalive that survives a broken pong path, so it must
// not land on the server read timeout the way the old 30s value did.
func TestHeartbeatKeepsAMarginBelowTheServerReadTimeout(t *testing.T) {
	const serverReadTimeout = 45 * time.Second // handlers.defaultAgentReadTimeout
	if agentHeartbeatInterval >= serverReadTimeout {
		t.Fatalf("heartbeat %s does not sit below the server read timeout", agentHeartbeatInterval)
	}
	if serverReadTimeout/agentHeartbeatInterval < 4 {
		t.Fatalf("heartbeat %s leaves fewer than four attempts before the %s timeout", agentHeartbeatInterval, serverReadTimeout)
	}
}
