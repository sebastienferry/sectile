package taskmcp

import (
	"context"
	"errors"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
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

// defaultAbandonAfter is how long a session may say nothing before the registry
// gives up on it (#319). A client that died without closing its connection is
// indistinguishable from a silent one, so past this bound the session is closed
// and its runs are canceled with the disconnect note, which leaves their owner
// free to report the real outcome afterwards. The deployment overrides it
// through SECTILE_MCP_SESSION_ABANDON_AFTER.
const defaultAbandonAfter = 8 * time.Hour

// DefaultKeepaliveInterval is how often the registry pings every live session
// (#517). A client's standalone GET stream otherwise carries nothing, and a
// proxy in front of the server cuts an idle stream (HAProxy after 50s by
// default): the client then gives up on its session and opens a new one,
// leaving the old one registered until the abandon bound. 25s keeps the stream
// busy under that default. The deployment overrides it through
// SECTILE_MCP_KEEPALIVE_INTERVAL.
const DefaultKeepaliveInterval = 25 * time.Second

// DefaultKeepaliveFailures is how many consecutive pings a session may fail,
// with no message from its client in between, before a session that owns no
// run is closed. The deployment overrides it through
// SECTILE_MCP_KEEPALIVE_FAILURES.
const DefaultKeepaliveFailures = 3

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

// RunWaiter clears the marks a session put on the runs it declared blocked on
// the user. The registry only ever clears: declaring a wait is the tool's
// business, under the caller's identity, while ending one is an observation the
// registry makes on its own, that the session spoke again or went away. The
// marks are found by session in the store rather than in this registry's
// memory, so a session this instance never saw, or saw before a restart, has
// its wait ended all the same (#475).
type RunWaiter interface {
	ResumeWaits(sessionID string) ([]string, error)
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
	// lastSeen is the last client-to-server message, whatever it invoked.
	lastSeen time.Time
	// silentSince marks the stretch of silence already remarked upon, and is
	// nil the rest of the time, so one stretch costs exactly one sentence.
	silentSince *time.Time
	// pingFailures counts the keepalive pings failed in a row since the last
	// answered ping or the last client message.
	pingFailures int
	// session is the transport's session, kept so that abandoning it closes the
	// connection as well: a client that comes back is then told its session is
	// gone rather than served by a registry that forgot it. Nil for a session
	// opened without a transport, as tests do.
	session *mcp.ServerSession
}

// SessionOwner returns the server instance a session id names, or "" when it
// names none: an id from a version that did not record its instance, or one a
// client made up. A session lives in the memory of the instance that created
// it, so its id is the only way another instance can find it (#408).
func SessionOwner(sessionID string) string {
	owner, _, found := strings.Cut(sessionID, ".")
	if !found {
		return ""
	}
	return owner
}

// SessionView projects a live session for the status API. It carries the
// client's own description of itself, which is declarative and therefore
// advisory: sessions are told apart by their identifier, not by their name.
// Instance is the server instance holding the session.
type SessionView struct {
	ID          string    `json:"id"`
	Instance    string    `json:"instance"`
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
	// abandon is how long a silence lasts before the session is closed. It is
	// never shorter than bound, so no session is closed before it was noticed.
	abandon time.Duration
	now     func() time.Time
	stop    chan struct{}
	// instance names the server instance this registry belongs to, for the
	// sessions view of a deployment several instances serve.
	instance string
	// stopOnce keeps Stop idempotent: a server shut down twice, as tests do,
	// must not panic on a closed channel.
	stopOnce sync.Once
	// keepaliveOnce starts the keepalive loop at most once.
	keepaliveOnce sync.Once
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
	return NewSessionRegistryBounded(runs, notes, bound, defaultAbandonAfter)
}

// NewSessionRegistryBounded is NewSessionRegistryWith with the deployment's
// abandon bound as well. A non-positive bound takes its default, and an abandon
// bound shorter than the silence bound is raised to it.
func NewSessionRegistryBounded(runs RunCloser, notes RunNoter, bound, abandon time.Duration) *SessionRegistry {
	if bound <= 0 {
		bound = defaultSilenceBound
	}
	if abandon <= 0 {
		abandon = defaultAbandonAfter
	}
	if abandon < bound {
		abandon = bound
	}
	r := &SessionRegistry{live: make(map[string]*liveSession), runs: runs, notes: notes,
		bound: bound, abandon: abandon, now: time.Now, stop: make(chan struct{})}
	go r.sweep(bound / silenceSweepDivisor)
	return r
}

// SetKeepalive starts pinging every live session each interval. A session that
// owns no run is closed once failures consecutive pings fail with no message
// from its client in between; a session that owns one is left to the silence
// and abandon bounds, which exist for exactly that case. Non-positive values
// take their defaults. The loop stops with Stop, and only the first call starts
// one: a registry without it never pings, as a host without HTTP needs.
//
// The pinging lives here rather than in go-sdk's ServerOptions.KeepAlive,
// because a session the SDK gives up on ends in Close, which cancels the runs
// the session adopted: that is exactly what #319 keeps a silence from doing.
func (r *SessionRegistry) SetKeepalive(interval time.Duration, failures int) {
	if r == nil {
		return
	}
	if interval <= 0 {
		interval = DefaultKeepaliveInterval
	}
	if failures <= 0 {
		failures = DefaultKeepaliveFailures
	}
	r.keepaliveOnce.Do(func() { go r.keepalive(interval, failures) })
}

func (r *SessionRegistry) keepalive(interval time.Duration, failures int) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-r.stop:
			return
		case <-ticker.C:
			// Half the interval, as go-sdk's own keepalive does, so one round
			// always ends before the next one starts.
			r.pingSessions(interval/2, failures)
		}
	}
}

// pingTarget is one session a round pings, with what the ping answered.
type pingTarget struct {
	id      string
	session *mcp.ServerSession
	err     error
}

// pingSessions pings every session that has a transport, concurrently and
// outside the lock, then records the answers.
//
// A ping reply is a response, not a method, so it never reaches the receiving
// middleware and never counts as the client speaking: silences keep their
// meaning.
func (r *SessionRegistry) pingSessions(timeout time.Duration, failures int) {
	r.mu.Lock()
	targets := make([]pingTarget, 0, len(r.live))
	for _, entry := range r.live {
		if entry.session != nil {
			targets = append(targets, pingTarget{id: entry.id, session: entry.session})
		}
	}
	r.mu.Unlock()

	var wg sync.WaitGroup
	for i := range targets {
		wg.Add(1)
		go func(target *pingTarget) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			target.err = target.session.Ping(ctx, nil)
			// A client without ping support answered all the same. Any other
			// error, the transport's own refusals included, is no answer.
			var answered *jsonrpc.Error
			if errors.As(target.err, &answered) && answered.Code == jsonrpc.CodeMethodNotFound {
				target.err = nil
			}
		}(&targets[i])
	}
	wg.Wait()

	r.mu.Lock()
	var orphans []pingTarget
	for _, target := range targets {
		entry := r.live[target.id]
		if entry == nil || entry.session != target.session {
			continue
		}
		if target.err == nil {
			entry.pingFailures = 0
			continue
		}
		entry.pingFailures++
		if entry.pingFailures < failures {
			continue
		}
		if len(entry.runs) > 0 {
			if entry.pingFailures == failures {
				log.Printf("[MCP] session %s missed %d pings (%v): it owns %d run(s), so it stays open", entry.id, failures, target.err, len(entry.runs))
			}
			continue
		}
		// Forgotten under the same lock that saw it own nothing, so a run
		// adopted in the meantime is never canceled by this closure.
		delete(r.live, target.id)
		orphans = append(orphans, target)
	}
	waits := r.waits
	r.mu.Unlock()

	for _, orphan := range orphans {
		clearWaits(waits, orphan.id)
		log.Printf("[MCP] session %s missed %d pings and owns no run: closed (%v)", orphan.id, failures, orphan.err)
		// Its watcher then finds the session already forgotten.
		_ = orphan.session.Close()
	}
}

// SetWaiter gives the registry the sink that clears waiting marks. Without one
// a wait lasts until the run ends.
func (r *SessionRegistry) SetWaiter(waits RunWaiter) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.waits = waits
	r.mu.Unlock()
}

// SetInstance names the server instance the sessions live on.
func (r *SessionRegistry) SetInstance(id string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.instance = id
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

// sweep remarks on the sessions that crossed the silence bound, and gives up on
// those that crossed the abandon bound. A silence alone cancels nothing: it says
// a client is slow, not that it is gone. Only a silence long enough to stand for
// a client that died without a word closes its session.
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
	var abandoned []*liveSession
	for _, entry := range r.live {
		silence := now.Sub(entry.lastSeen)
		if silence >= r.abandon {
			abandoned = append(abandoned, entry)
		}
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
	// Outside the lock, as with Close: annotating runs must not hold up the
	// sessions still being served. The silence is noted before an abandonment
	// closes the run, so the board reads the observation, then the verdict.
	if r.notes != nil {
		for _, run := range pending {
			if err := r.notes.NoteRemoteRun(run.runID, run.note); err != nil {
				log.Printf("[MCP] cannot note the silence on run %s: %v", run.runID, err)
			}
		}
	}
	for _, entry := range abandoned {
		// The notes above ran outside the lock, and a client may have spoken in
		// the meantime: a session is only given up on while it is still silent.
		if !r.stillSilentFor(entry.id, r.abandon) {
			continue
		}
		closed := r.Close(entry.id)
		log.Printf("[MCP] session %s said nothing for %s: abandoned, %d run(s) closed", entry.id, r.abandon, closed)
		if entry.session != nil {
			// Its watcher then finds the session already forgotten.
			_ = entry.session.Close()
		}
	}
}

// stillSilentFor says whether a session is still live and has said nothing for
// at least the given duration.
func (r *SessionRegistry) stillSilentFor(sessionID string, silence time.Duration) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry := r.live[sessionID]
	return entry != nil && r.now().Sub(entry.lastSeen) >= silence
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
		// A client that speaks is alive, whatever its pings say: one that
		// never opens the standalone stream cannot receive them (#517).
		entry.pingFailures = 0
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
	r.mu.Lock()
	if entry := r.live[session.ID()]; entry != nil {
		entry.session = session
	}
	r.mu.Unlock()
	go func() {
		_ = session.Wait()
		r.Close(session.ID())
	}()
}

func (r *SessionRegistry) open(id string, info *mcp.Implementation) {
	entry := &liveSession{id: id, client: "unknown", connectedAt: r.now(), lastSeen: r.now(),
		runs: make(map[string]adoptedRun)}
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

// Resume ends every wait a session declared. A session that makes a call is no
// longer blocked on its owner, whatever it forgot to report, so a wait never
// depends on the model remembering to clear it. It reports how many waits it
// cleared. The session need not be one this registry holds: the store knows
// which session declared each wait.
func (r *SessionRegistry) Resume(sessionID string) int {
	if r == nil || sessionID == "" {
		return 0
	}
	r.mu.Lock()
	waits := r.waits
	r.mu.Unlock()
	return clearWaits(waits, sessionID)
}

// clearWaits clears the marks outside the registry lock, as Close does for the
// runs it finishes.
func clearWaits(waits RunWaiter, sessionID string) int {
	if waits == nil {
		return 0
	}
	cleared, err := waits.ResumeWaits(sessionID)
	if err != nil {
		log.Printf("[MCP] session %s: cannot clear its waits: %v", sessionID, err)
	}
	return len(cleared)
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
	// nobody is left to be waiting for. A run it owns is closed below, which
	// would clear its mark anyway.
	clearWaits(waits, sessionID)
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
		views = append(views, SessionView{ID: entry.id, Instance: r.instance, Client: entry.client, Title: entry.title,
			Version: entry.version, ConnectedAt: entry.connectedAt, Runs: runs})
	}
	r.mu.Unlock()
	SortSessions(views)
	return views
}

// SortSessions orders sessions most recently connected first, then by id, so a
// view merged from several instances reads as one instance's would.
func SortSessions(views []SessionView) {
	sort.Slice(views, func(i, j int) bool {
		if views[i].ConnectedAt.Equal(views[j].ConnectedAt) {
			return views[i].ID < views[j].ID
		}
		return views[i].ConnectedAt.After(views[j].ConnectedAt)
	})
}

// sessionID names the session a tool call belongs to, or an empty string when
// the transport does not provide one.
func sessionID(session *mcp.ServerSession) string {
	if session == nil {
		return ""
	}
	return session.ID()
}
