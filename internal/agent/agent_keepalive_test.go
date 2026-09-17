package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"tasks/internal/agentprotocol"

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
	// messages receives every data frame the server reads once resumed.
	messages <-chan []byte
	// clientDone is closed when the client read loop returns, which is how
	// the agent's message loop learns that its connection is gone.
	clientDone chan struct{}
}

func newKeepaliveFixture(t *testing.T) *keepaliveFixture {
	t.Helper()
	pongs := make(chan string, 16)
	messages := make(chan []byte, 16)
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
			_, raw, err := conn.ReadMessage()
			if err != nil {
				return
			}
			select {
			case messages <- raw:
			default:
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

	f := &keepaliveFixture{server: <-serverConns, client: client, pongs: pongs, resume: resume, messages: messages, clientDone: make(chan struct{})}
	// The client keeps processing control frames, as the agent's message loop
	// does.
	go func() {
		defer close(f.clientDone)
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

// The regression of 2026-09-17. gorilla applies the last SetWriteDeadline to
// every later data frame. An operation result set one five seconds ahead and
// the heartbeat, which set none, inherited it once it had passed: on a healthy
// loopback socket every heartbeat failed with "i/o timeout", the failure
// poisoned the connection, and the server dropped the agent for silence 45s
// later. A writer must never depend on the deadline another writer left.
func TestHeartbeatDoesNotInheritAStaleWriteDeadline(t *testing.T) {
	f := newKeepaliveFixture(t)
	close(f.resume)
	// What an earlier writer leaves behind once its own budget has passed.
	_ = f.client.SetWriteDeadline(time.Now().Add(-time.Second))

	d := &agentDaemon{}
	if err := d.link.write(f.client, agentprotocol.Message{Type: "heartbeat"}); err != nil {
		t.Fatalf("heartbeat failed on a healthy connection: %v", err)
	}
	select {
	case raw := <-f.messages:
		var msg agentprotocol.Message
		if err := json.Unmarshal(raw, &msg); err != nil || msg.Type != "heartbeat" {
			t.Fatalf("server read %q instead of a heartbeat", raw)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the heartbeat never reached the server")
	}
}

// Once a write has failed gorilla refuses every later write, so the connection
// cannot answer the server any more and would only be dropped for silence 45s
// later. The heartbeat loop must hang up itself, so the read loop returns and
// the reconnection starts at once.
func TestFailedHeartbeatHangsUpInsteadOfWaitingForTheServer(t *testing.T) {
	f := newKeepaliveFixture(t)
	close(f.resume)
	// Poison the connection the way the stale deadline did.
	_ = f.client.SetWriteDeadline(time.Now().Add(-time.Second))
	if err := f.client.WriteJSON(agentprotocol.Message{Type: "step_status"}); err == nil {
		t.Fatal("the poisoning write succeeded: the test proves nothing")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d := &agentDaemon{}
	go d.heartbeatLoop(ctx, f.client, 10*time.Millisecond)

	select {
	case <-f.clientDone:
	case <-time.After(5 * time.Second):
		t.Fatal("the read loop is still blocked on a connection that can no longer write")
	}
}
