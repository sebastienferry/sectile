package agent

import (
	"net/http"
	"strings"
	"sync"

	"tasks/internal/runner"

	"github.com/gorilla/websocket"
)

// The trace of a run nobody is watching.
//
// An autonomous run has no terminal on purpose, so the desktop had nothing to
// show for the ten or forty minutes it lasts. Asked for its reasoning stream,
// the engine says what it is doing as it does it; this is where those events are
// rendered, held, and handed to whoever attaches.
//
// Two rules govern everything below, and they are the same rule twice: the trace
// is a courtesy, and the run is the work. A line that cannot be rendered is
// dropped, a watcher that cannot keep up is disconnected, and neither is ever
// allowed to slow down or fail the run being traced.

// traceRetained is how many rendered lines a run keeps. A run reading a large
// file prints it, and the daemon holds every trace of every run it remembers;
// past this the oldest go, which is what a terminal does with its scrollback.
const traceRetained = 2000

// traceBacklog is how far a watcher may fall behind before it is dropped. It is
// generous — a desktop writing to xterm drains far faster than an engine
// produces — and bounded, because the alternative is blocking the supervisor.
const traceBacklog = 256

// runTrace holds what a run has shown and feeds the watchers attached to it.
type runTrace struct {
	mu      sync.Mutex
	lines   []string
	clients map[chan string]struct{}
	closed  bool
}

func newRunTrace() *runTrace {
	return &runTrace{clients: make(map[chan string]struct{})}
}

// publish renders one event and hands it to the buffer and to every watcher.
// An event that renders to nothing is not published.
func (t *runTrace) publish(event runner.ReasoningEvent) {
	if t == nil {
		return
	}
	t.write(renderTraceEvent(event))
}

// write appends one already-rendered line. Sending to a watcher never blocks:
// one that is not draining its channel is dropped and closed, so a stalled
// desktop cannot hold up the process reading the engine's output.
func (t *runTrace) write(line string) {
	if t == nil || line == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return
	}
	t.lines = append(t.lines, line)
	if len(t.lines) > traceRetained {
		t.lines = append([]string(nil), t.lines[len(t.lines)-traceRetained:]...)
	}
	for client := range t.clients {
		select {
		case client <- line:
		default:
			delete(t.clients, client)
			close(client)
		}
	}
}

// attach returns everything the run has shown so far and a channel carrying what
// it shows next. The replay and the subscription are taken under the same lock,
// so a line produced between them is delivered once rather than lost or doubled.
// A trace already closed replays its lines and hands back a closed channel: the
// watcher reads the run's history and sees the stream end.
func (t *runTrace) attach() ([]string, chan string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	replay := append([]string(nil), t.lines...)
	client := make(chan string, traceBacklog)
	if t.closed {
		close(client)
		return replay, client
	}
	t.clients[client] = struct{}{}
	return replay, client
}

// detach drops one watcher, whether it left or was dropped for falling behind.
func (t *runTrace) detach(client chan string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, live := t.clients[client]; live {
		delete(t.clients, client)
		close(client)
	}
}

// close ends the trace when its run ends: every watcher sees the stream finish
// rather than a connection that stays open on a process that exited. What the
// run showed stays readable for as long as the agent remembers the run.
func (t *runTrace) close() {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return
	}
	t.closed = true
	for client := range t.clients {
		delete(t.clients, client)
		close(client)
	}
}

// Terminal styling. The pane on the other side is xterm, so a line arrives
// rendered: the desktop's only job with an attached socket is to write the bytes
// it receives, and the trace must not become a second place where the look of a
// tool call is decided.
const (
	traceDim   = "\x1b[2m"
	traceReset = "\x1b[0m"
)

// renderTraceEvent turns one event into one terminal line. The prose is the
// run talking and is left alone; a tool call is one dim line naming the tool and
// what the call was about, because a trace is read at a glance.
func renderTraceEvent(event runner.ReasoningEvent) string {
	switch event.Kind {
	case runner.ReasoningText:
		return traceLine(event.Text)
	case runner.ReasoningThinking:
		return traceLine(traceDim + event.Text + traceReset)
	case runner.ReasoningTool:
		line := traceDim + "▸ " + event.Text
		if strings.TrimSpace(event.Detail) != "" {
			line += " · " + event.Detail
		}
		return traceLine(line + traceReset)
	}
	return ""
}

// traceLine ends a line the way a terminal expects one, and refuses an empty
// one: a blank line in the pane says the run did something, and it did not.
func traceLine(text string) string {
	if strings.TrimSpace(stripTraceStyle(text)) == "" {
		return ""
	}
	// The engine writes prose with plain newlines; xterm needs the carriage
	// return, or the second line starts where the first one ended.
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\n", "\r\n")
	return text + "\r\n"
}

func stripTraceStyle(text string) string {
	return strings.NewReplacer(traceDim, "", traceReset, "").Replace(text)
}

// traceUpgrader mirrors the terminal manager's: the same local-origin rule, on
// the same loopback, for a connection the same desktop opens.
var traceUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// serveRunTrace streams a run's trace to one watcher: everything it has shown,
// then everything it shows next, as the bytes a terminal displays.
//
// The connection is read only to notice the watcher leaving. A frame it sends is
// discarded rather than forwarded: nobody is answering an autonomous run, and
// the renderer's keyboard is wired to the socket globally, so refusing the input
// here is the only place it cannot be re-enabled by accident.
func serveRunTrace(w http.ResponseWriter, r *http.Request, trace *runTrace) {
	conn, err := traceUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	replay, lines := trace.attach()
	defer trace.detach(lines)

	for _, line := range replay {
		if err := conn.WriteMessage(websocket.BinaryMessage, []byte(line)); err != nil {
			return
		}
	}

	// Reading in its own goroutine: a websocket connection only learns that its
	// peer is gone by reading, and the writing side has to keep serving the
	// lines meanwhile.
	gone := make(chan struct{})
	go func() {
		defer close(gone)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	for {
		select {
		case line, open := <-lines:
			if !open {
				// The run ended, or this watcher fell too far behind to be
				// worth catching up. Either way there is nothing more to send.
				return
			}
			if err := conn.WriteMessage(websocket.BinaryMessage, []byte(line)); err != nil {
				return
			}
		case <-gone:
			return
		}
	}
}
