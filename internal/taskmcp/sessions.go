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
	// The note is shared with internal/db, which recognizes it to let the run's
	// owner rewrite an outcome a disconnection decided for them.
	disconnectNote = models.RunDisconnectNote
)

// defaultSilenceBound is how long a session may say nothing before the registry
// remarks on it. It bounds an observation, never a life: the deployment
// overrides it through SECTILE_MCP_SESSION_TIMEOUT.
const defaultSilenceBound = 4 * time.Hour

// silenceSweepDivisor sets how often the sweeper looks, as a fraction of the
// bound, so a silence is noticed shortly after it crosses rather than a whole
// bound later.
const silenceSweepDivisor = 10

// RunCloser finishes a run whose client can no longer report on it. The
// registry depends on this narrow contract rather than on the database so that
// session ownership can be exercised without one.
type RunCloser interface {
	FinishRemoteRun(taskKey, runID, status, note string) (*models.TaskActivity, error)
}

// RunNoter appends one sentence to a run that is still running. Marking is the
// only thing a silence may do, so the registry asks for exactly that verb and
// nothing that could end a run.
type RunNoter interface {
	NoteRemoteRun(runID, note string) error
}

// RunWaiter clears the mark a session put on a run it declared blocked on the
// user. The registry only ever clears: declaring a wait is the tool's business,
// under the caller's identity, while ending one is an observation the registry
// makes on its own, that the session spoke again or went away.
type RunWaiter interface {
	SetRemoteRunWaiting(runID string, waiting bool) error
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
	// waiting names the runs this session declared blocked on the user. It is
	// kept apart from runs because a session may report on a run it did not
	// adopt, such as one a launcher created and handed over.
	waiting map[string]bool
	// lastSeen is the last client-to-server message, whatever it invoked.
	lastSeen time.Time
	// silentSince marks the stretch of silence already remarked upon, and is
	// nil the rest of the time, so one stretch costs exactly one sentence.
	silentSince *time.Time
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
	mu    sync.Mutex
	live  map[string]*liveSession
	runs  RunCloser
	notes RunNoter
	waits RunWaiter
	bound time.Duration
	now   func() time.Time
	stop  chan struct{}
	// stopOnce keeps Stop idempotent: a server shut down twice, as tests do,
	// must not panic on a closed channel.
	stopOnce sync.Once
}

// NewSessionRegistry builds a registry that observes silences but has nothing to
// note them on, which is the shape a host without a database gets.
func NewSessionRegistry(runs RunCloser) *SessionRegistry {
	return NewSessionRegistryWith(runs, nil, defaultSilenceBound)
}

// NewSessionRegistryWith is NewSessionRegistry with the deployment's silence
// bound and the sink that records an observed silence. The sweeper starts here
// and stops with Stop, so a caller owns the goroutine it created.
func NewSessionRegistryWith(runs RunCloser, notes RunNoter, bound time.Duration) *SessionRegistry {
	if bound <= 0 {
		bound = defaultSilenceBound
	}
	r := &SessionRegistry{live: make(map[string]*liveSession), runs: runs, notes: notes,
		bound: bound, now: time.Now, stop: make(chan struct{})}
	go r.sweep(bound / silenceSweepDivisor)
	return r
}

// SetWaiter gives the registry the sink that clears waiting marks. Without one
// the registry still tracks the marks, and a wait lasts until the run ends.
func (r *SessionRegistry) SetWaiter(waits RunWaiter) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.waits = waits
	r.mu.Unlock()
}

// Stop ends the sweeper. The registry keeps serving every other method, since a
// session's ownership does not depend on anyone watching it fall silent.
func (r *SessionRegistry) Stop() {
	if r == nil {
		return
	}
	r.stopOnce.Do(func() { close(r.stop) })
}

// sweep remarks on the sessions that crossed the bound. It cancels nothing:
// silence says a client is slow, not that it is gone, and only a real ending
// closes a run.
func (r *SessionRegistry) sweep(every time.Duration) {
	if every <= 0 {
		every = time.Second
	}
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-r.stop:
			return
		case <-ticker.C:
			r.markSilentSessions()
		}
	}
}

// silentRun pairs a run to note with the sentence it earns, so the notes go out
// once the lock is released.
type silentRun struct {
	runID string
	note  string
}

func (r *SessionRegistry) markSilentSessions() {
	if r == nil {
		return
	}
	now := r.now()
	r.mu.Lock()
	var pending []silentRun
	for _, entry := range r.live {
		silence := now.Sub(entry.lastSeen)
		if silence < r.bound || entry.silentSince != nil {
			continue
		}
		since := entry.lastSeen
		entry.silentSince = &since
		note := models.RunSilenceNote(silence)
		for runID := range entry.runs {
			pending = append(pending, silentRun{runID: runID, note: note})
		}
		log.Printf("[MCP] session %s has been silent for %s: its runs stay open", entry.id, silence.Round(time.Minute))
	}
	r.mu.Unlock()
	if r.notes == nil {
		return
	}
	// Outside the lock, as with Close: annotating runs must not hold up the
	// sessions still being served.
	for _, run := range pending {
		if err := r.notes.NoteRemoteRun(run.runID, run.note); err != nil {
			log.Printf("[MCP] cannot note the silence on run %s: %v", run.runID, err)
		}
	}
}

// Touch records that a client spoke. Any message counts, whichever tool or
// protocol method it invoked, and it rearms the observation so the next silence
// is remarked upon in its turn.
func (r *SessionRegistry) Touch(sessionID string) {
	if r == nil || sessionID == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if entry := r.live[sessionID]; entry != nil {
		entry.lastSeen = r.now()
		entry.silentSince = nil
	}
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
	entry := &liveSession{id: id, client: "unknown", connectedAt: r.now(), lastSeen: r.now(),
		runs: make(map[string]adoptedRun), waiting: make(map[string]bool)}
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

// MarkWaiting records that a session declared one of its runs blocked on the
// user, so that the session's next call can end the wait.
func (r *SessionRegistry) MarkWaiting(sessionID, runID string) {
	if r == nil || sessionID == "" || runID == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if entry := r.live[sessionID]; entry != nil {
		entry.waiting[runID] = true
	}
}

// ForgetWaiting drops a wait the session cleared itself.
func (r *SessionRegistry) ForgetWaiting(sessionID, runID string) {
	if r == nil || sessionID == "" || runID == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if entry := r.live[sessionID]; entry != nil {
		delete(entry.waiting, runID)
	}
}

// Resume ends every wait a session declared. A session that makes a call is no
// longer blocked on its owner, whatever it forgot to report, so a wait never
// depends on the model remembering to clear it. It reports how many waits it
// cleared.
func (r *SessionRegistry) Resume(sessionID string) int {
	if r == nil || sessionID == "" {
		return 0
	}
	r.mu.Lock()
	entry := r.live[sessionID]
	if entry == nil || len(entry.waiting) == 0 {
		r.mu.Unlock()
		return 0
	}
	runIDs := make([]string, 0, len(entry.waiting))
	for runID := range entry.waiting {
		runIDs = append(runIDs, runID)
	}
	entry.waiting = make(map[string]bool)
	waits := r.waits
	r.mu.Unlock()
	return clearWaits(waits, sessionID, runIDs)
}

// clearWaits clears the marks outside the registry lock, as Close does for the
// runs it finishes. A run that already ended has no mark left to clear, which is
// an ordinary race rather than a failure.
func clearWaits(waits RunWaiter, sessionID string, runIDs []string) int {
	if waits == nil {
		return 0
	}
	cleared := 0
	for _, runID := range runIDs {
		if err := waits.SetRemoteRunWaiting(runID, false); err != nil {
			log.Printf("[MCP] session %s: cannot clear the wait on run %s: %v", sessionID, runID, err)
			continue
		}
		cleared++
	}
	return cleared
}

// ReleaseRun forgets a run in whichever session holds it, for a run closed from
// outside MCP, such as from the board. The session no longer owns it, so its
// ending must not close the run a second time.
func (r *SessionRegistry) ReleaseRun(runID string) {
	if r == nil || runID == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, entry := range r.live {
		delete(entry.runs, runID)
		delete(entry.waiting, runID)
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
	waits := r.waits
	r.mu.Unlock()
	if entry == nil {
		return 0
	}
	// A run this session waited on but does not own survives the session, and
	// nobody is left to be waiting for. A run it owns is closed below, and its
	// terminal status clears the mark.
	var orphanedWaits []string
	for runID := range entry.waiting {
		if _, owned := entry.runs[runID]; !owned {
			orphanedWaits = append(orphanedWaits, runID)
		}
	}
	clearWaits(waits, sessionID, orphanedWaits)
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
