package taskmcp

import (
	"log"
	"sort"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"tasks/internal/models"
)

// A run adopted by a session is closed when that session ends. The tool
// contract accepts completed, failed and canceled only, so the distinction
// between an end the client reported and a client that vanished lives in the
// note rather than in a new status value.
const (
	disconnectStatus = "canceled"
	disconnectNote   = "Client disconnected: the server closed this run when its MCP session ended"
)

// RunCloser finishes a run whose client can no longer report on it. The
// registry depends on this narrow contract rather than on the database so that
// session ownership can be exercised without one.
type RunCloser interface {
	FinishRemoteRun(taskKey, runID, status, note string) (*models.TaskActivity, error)
}

// adoptedRun remembers what a session would leave behind. The task key is kept
// because closing a run requires it, and the skill name because a disconnection
// is worth reporting in terms an operator recognizes.
type adoptedRun struct {
	taskKey string
	skill   string
}

type liveSession struct {
	id          string
	client      string
	title       string
	version     string
	connectedAt time.Time
	runs        map[string]adoptedRun
}

// SessionView projects a live session for the status API. It carries the
// client's own description of itself, which is declarative and therefore
// advisory: sessions are told apart by their identifier, not by their name.
type SessionView struct {
	ID          string    `json:"id"`
	Client      string    `json:"client"`
	Title       string    `json:"title,omitempty"`
	Version     string    `json:"version,omitempty"`
	ConnectedAt time.Time `json:"connectedAt"`
	Runs        []string  `json:"runs"`
}

// SessionRegistry holds the sessions the server currently serves and the runs
// each of them started. A session is an infrastructure fact the server
// observes; a run remains a domain fact its client declares. The registry is
// where the two meet: it never starts a run, and it only ever closes one whose
// client is gone.
//
// Every method tolerates a nil receiver so that a server built without session
// ownership keeps working, and tolerates an empty session identifier so that a
// transport without sessions degrades to the previous client-owned behaviour
// instead of failing.
type SessionRegistry struct {
	mu   sync.Mutex
	live map[string]*liveSession
	runs RunCloser
	now  func() time.Time
}

func NewSessionRegistry(runs RunCloser) *SessionRegistry {
	return &SessionRegistry{live: make(map[string]*liveSession), runs: runs, now: time.Now}
}

// Watch registers a session and closes it when its client goes away. The wait
// covers every ending the transport can report: an explicit termination, a
// dropped connection, and the idle timeout that bounds a client which never
// says goodbye.
func (r *SessionRegistry) Watch(session *mcp.ServerSession) {
	if r == nil || session == nil || session.ID() == "" {
		return
	}
	var info *mcp.Implementation
	if params := session.InitializeParams(); params != nil {
		info = params.ClientInfo
	}
	r.open(session.ID(), info)
	go func() {
		_ = session.Wait()
		r.Close(session.ID())
	}()
}

func (r *SessionRegistry) open(id string, info *mcp.Implementation) {
	entry := &liveSession{id: id, client: "unknown", connectedAt: r.now(), runs: make(map[string]adoptedRun)}
	if info != nil {
		if info.Name != "" {
			entry.client = info.Name
		}
		entry.title, entry.version = info.Title, info.Version
	}
	r.mu.Lock()
	r.live[id] = entry
	r.mu.Unlock()
}

// Adopt binds a run to the session that started it.
func (r *SessionRegistry) Adopt(sessionID, runID, taskKey, skill string) {
	if r == nil || sessionID == "" || runID == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if entry := r.live[sessionID]; entry != nil {
		entry.runs[runID] = adoptedRun{taskKey: taskKey, skill: skill}
	}
}

// Release drops a run its client finished itself. An explicit report stays the
// precise way to end a run; it simply stops being the only one.
func (r *SessionRegistry) Release(sessionID, runID string) {
	if r == nil || sessionID == "" || runID == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if entry := r.live[sessionID]; entry != nil {
		delete(entry.runs, runID)
	}
}

// Close forgets a session and closes the runs it still owns. It reports how
// many runs it closed, which is what a caller can act on; individual failures
// are logged because a run that someone else already finished is an ordinary
// race, not a reason to abandon the remaining ones.
func (r *SessionRegistry) Close(sessionID string) int {
	if r == nil || sessionID == "" {
		return 0
	}
	r.mu.Lock()
	entry := r.live[sessionID]
	delete(r.live, sessionID)
	r.mu.Unlock()
	if entry == nil {
		return 0
	}
	// The database call happens outside the lock: closing runs must not block
	// sessions that are still being served.
	closed := 0
	for runID, run := range entry.runs {
		if r.runs == nil {
			continue
		}
		if _, err := r.runs.FinishRemoteRun(run.taskKey, runID, disconnectStatus, disconnectNote); err != nil {
			log.Printf("[MCP] session %s: cannot close run %s on %s: %v", sessionID, runID, run.taskKey, err)
			continue
		}
		closed++
		log.Printf("[MCP] session %s ended: closed run %s (%s) on %s", sessionID, runID, run.skill, run.taskKey)
	}
	return closed
}

// Snapshot lists the live sessions, most recent first.
func (r *SessionRegistry) Snapshot() []SessionView {
	if r == nil {
		return []SessionView{}
	}
	r.mu.Lock()
	views := make([]SessionView, 0, len(r.live))
	for _, entry := range r.live {
		runs := make([]string, 0, len(entry.runs))
		for runID := range entry.runs {
			runs = append(runs, runID)
		}
		sort.Strings(runs)
		views = append(views, SessionView{ID: entry.id, Client: entry.client, Title: entry.title,
			Version: entry.version, ConnectedAt: entry.connectedAt, Runs: runs})
	}
	r.mu.Unlock()
	sort.Slice(views, func(i, j int) bool {
		if views[i].ConnectedAt.Equal(views[j].ConnectedAt) {
			return views[i].ID < views[j].ID
		}
		return views[i].ConnectedAt.After(views[j].ConnectedAt)
	})
	return views
}

// sessionID names the session a tool call belongs to, or an empty string when
// the transport does not provide one.
func sessionID(session *mcp.ServerSession) string {
	if session == nil {
		return ""
	}
	return session.ID()
}
