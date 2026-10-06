package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"tasks/internal/terminal"
)

// The runs the desktop lists, kept between two agent processes (#588).
//
// The agent is restarted often: after an update, after a settings change, when
// the machine reboots. Each run's record, and what its console showed, lived in
// this process only, so every restart emptied the sidebar. The run store keeps a
// private copy of each run on the workstation and the next process reads it
// back: the person finds the same consoles, readable as they ended. The process
// behind a console is not kept; a restored run is only ever replayed.
//
// The store is a courtesy, like the trace: a run is never failed, delayed or
// stopped because its copy could not be written.

// runStoreCapability tells the desktop that a restart keeps the consoles, so
// its confirmation does not warn that they will be lost.
const runStoreCapability = "run-store"

// runStoreVersion is the format of a stored run. A file of another version is
// left alone rather than guessed at.
const runStoreVersion = 1

// runStoreRetained is how many finished runs the store keeps. Past it the
// oldest finished run goes first; a live run never counts.
const runStoreRetained = 100

// runStoreFlush is how often a live run's changes are written, and so how much
// of its console a crash can lose.
const runStoreFlush = 5 * time.Second

// safeRunID is what a run id must look like to become a file name: the server
// hands out UUIDs, and anything with a separator or a dot is refused.
var safeRunID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`)

// storedRun is one file of the store.
type storedRun struct {
	Version int        `json:"version"`
	Run     desktopRun `json:"run"`
	// Console is what the run's terminal printed, bounded as the session
	// history is. Trace is a headless run's rendered reasoning stream.
	Console    []byte    `json:"console,omitempty"`
	Trace      []string  `json:"trace,omitempty"`
	FinishedAt time.Time `json:"finishedAt,omitzero"`
}

// runStore is the directory of stored runs. A nil store is a disabled one:
// every method is then a no-op, which is what a daemon built without a state
// directory, every test daemon included, gets.
type runStore struct {
	dir string
	// mu serializes the writes and guards the two maps.
	mu sync.Mutex
	// finished dates each finished run on disk, for retention.
	finished map[string]time.Time
	// removed are the runs the person cleared: a write racing the clear must
	// not bring the file back.
	removed map[string]bool
}

// openRunStore creates the store directory, private to the user, and returns
// nil when it cannot: the agent then runs as it did before the store existed.
func openRunStore(dir string) *runStore {
	if err := os.MkdirAll(dir, 0700); err != nil {
		log.Printf("[Agent] Run store unavailable, consoles will not survive a restart: %v", err)
		return nil
	}
	// MkdirAll leaves an existing directory's mode alone.
	if err := os.Chmod(dir, 0700); err != nil {
		log.Printf("[Agent] Run store unavailable, consoles will not survive a restart: %v", err)
		return nil
	}
	return &runStore{dir: dir, finished: map[string]time.Time{}, removed: map[string]bool{}}
}

func (s *runStore) path(id string) string { return filepath.Join(s.dir, id+".json") }

// save writes one run atomically: a reader sees the previous complete file or
// the new complete one, never half of either.
func (s *runStore) save(record storedRun) error {
	if s == nil {
		return nil
	}
	id := record.Run.ID
	if !safeRunID.MatchString(id) {
		return fmt.Errorf("run id %q cannot name a stored run", id)
	}
	record.Version = runStoreVersion
	raw, err := json.Marshal(record)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.removed[id] {
		return nil
	}
	file, err := os.CreateTemp(s.dir, ".run-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err := file.Chmod(0600); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(raw); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(file.Name(), s.path(id)); err != nil {
		return err
	}
	if record.FinishedAt.IsZero() {
		delete(s.finished, id)
	} else {
		s.finished[id] = record.FinishedAt
	}
	return nil
}

// remove deletes the runs the person cleared, and keeps them from coming back.
func (s *runStore) remove(ids ...string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, id := range ids {
		if !safeRunID.MatchString(id) {
			continue
		}
		s.removed[id] = true
		delete(s.finished, id)
		if err := os.Remove(s.path(id)); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Printf("[Agent] Stored run %s not deleted: %v", id, err)
		}
	}
}

// load reads the store back, oldest run first. A run stored while it was
// running died with the previous process and comes back canceled; one still
// queued or preparing had not started, and is dropped with its file. A file
// that cannot be read is skipped, and kept for whoever wants to look at it.
func (s *runStore) load() []storedRun {
	if s == nil {
		return nil
	}
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		log.Printf("[Agent] Run store not read: %v", err)
		return nil
	}
	records := []storedRun{}
	for _, entry := range entries {
		name := entry.Name()
		id, isRun := strings.CutSuffix(name, ".json")
		if entry.IsDir() || !isRun || strings.HasPrefix(name, ".") {
			continue
		}
		if !safeRunID.MatchString(id) {
			log.Printf("[Agent] Stored run %s skipped: not a run id", name)
			continue
		}
		raw, err := os.ReadFile(s.path(id))
		if err != nil {
			log.Printf("[Agent] Stored run %s skipped: %v", id, err)
			continue
		}
		var record storedRun
		if err := json.Unmarshal(raw, &record); err != nil {
			log.Printf("[Agent] Stored run %s skipped: %v", id, err)
			continue
		}
		if record.Version != runStoreVersion || record.Run.ID != id {
			log.Printf("[Agent] Stored run %s skipped: format %d is not %d, or the file names another run", id, record.Version, runStoreVersion)
			continue
		}
		switch record.Run.Status {
		case "completed", "failed", "canceled":
		case "running":
			record.Run.Status = "canceled"
			record.FinishedAt = time.Time{}
		default:
			_ = os.Remove(s.path(id))
			continue
		}
		if record.FinishedAt.IsZero() {
			record.FinishedAt = time.Now().UTC()
			if info, err := entry.Info(); err == nil {
				record.FinishedAt = info.ModTime().UTC()
			}
			// Written back so the run keeps the status it is shown with.
			if err := s.save(record); err != nil {
				log.Printf("[Agent] Stored run %s not updated: %v", id, err)
			}
		}
		s.mu.Lock()
		s.finished[id] = record.FinishedAt
		s.mu.Unlock()
		records = append(records, record)
	}
	sort.SliceStable(records, func(i, j int) bool { return records[i].Run.CreatedAt.Before(records[j].Run.CreatedAt) })
	return records
}

// prune keeps the keep most recently finished runs on disk and deletes the
// others. live names the runs still going, which are never deleted.
func (s *runStore) prune(keep int, live map[string]bool) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	type dated struct {
		id string
		at time.Time
	}
	finished := make([]dated, 0, len(s.finished))
	for id, at := range s.finished {
		if !live[id] {
			finished = append(finished, dated{id, at})
		}
	}
	if len(finished) <= keep {
		return
	}
	sort.Slice(finished, func(i, j int) bool {
		if finished[i].at.Equal(finished[j].at) {
			return finished[i].id > finished[j].id
		}
		return finished[i].at.After(finished[j].at)
	})
	for _, old := range finished[keep:] {
		delete(s.finished, old.id)
		if err := os.Remove(s.path(old.id)); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Printf("[Agent] Stored run %s not deleted: %v", old.id, err)
		}
	}
}

// consoleTap is a run's own copy of what its terminal printed, bounded as the
// session history is. The session's history goes with the session, which
// closes when its shell exits; the copy stays with the run.
type consoleTap struct {
	mu      sync.Mutex
	bytes   []byte
	version uint64
}

func (t *consoleTap) write(chunk []byte) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.bytes = append(t.bytes, chunk...)
	if len(t.bytes) > terminal.HistoryLimit {
		t.bytes = append([]byte(nil), t.bytes[len(t.bytes)-terminal.HistoryLimit:]...)
	}
	t.version++
}

// snapshot copies what the run has shown, with a version that changes on
// every write, so the flusher can tell a quiet run from a busy one.
func (t *consoleTap) snapshot() ([]byte, uint64) {
	if t == nil {
		return nil, 0
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]byte(nil), t.bytes...), t.version
}

// runSave is what the last write of a run held, to skip writing it again.
type runSave struct {
	status  string
	console uint64
	trace   uint64
	// folders counts the run's folders: one added while the console is quiet
	// is written all the same (#762).
	folders int
}

// tapConsole gives a run its own copy of its session's output, from the first
// byte the session printed. Only a daemon with a store needs one.
func (d *agentDaemon) tapConsole(runID string) {
	if d.store == nil || d.terminal.manager == nil {
		return
	}
	tap := &consoleTap{}
	fresh := false
	d.queue.read(runID, func(run *controlledRun) {
		if run.console == nil && !run.desktop.Headless {
			run.console, fresh = tap, true
		}
	})
	if !fresh {
		return
	}
	// Held across the registration: a chunk the session prints right after it
	// waits for the history to be in place, so the copy keeps the order.
	tap.mu.Lock()
	defer tap.mu.Unlock()
	history, _ := d.terminal.manager.TapOutput(runID, tap.write)
	if len(history) > terminal.HistoryLimit {
		history = history[len(history)-terminal.HistoryLimit:]
	}
	tap.bytes = history
	tap.version++
}

// trackRun writes a run as soon as it ends, rather than at the next flush. It
// is called where a run enters the index, under the queue lock.
func (d *agentDaemon) trackRun(id string, run *controlledRun) {
	if d.store == nil {
		return
	}
	go func() {
		<-run.exited
		d.persistRun(id, run)
		d.pruneRuns()
	}()
}

// persistRun writes one run when it changed since its last write. The run is
// copied under the queue lock and written outside it.
func (d *agentDaemon) persistRun(id string, run *controlledRun) {
	if d.store == nil {
		return
	}
	run.persistMu.Lock()
	defer run.persistMu.Unlock()
	d.queue.mu.Lock()
	if d.queue.runs[id] != run || run.restored {
		d.queue.mu.Unlock()
		return
	}
	record := storedRun{Run: run.desktop}
	status := run.desktop.Status
	exited := false
	select {
	case <-run.exited:
		exited = true
		if run.finishedAt.IsZero() {
			run.finishedAt = time.Now().UTC()
		}
		record.FinishedAt = run.finishedAt
	default:
	}
	tap, trace, saved := run.console, run.trace, run.saved
	d.queue.mu.Unlock()
	// A run that has not started has nothing to show, and would be dropped at
	// the next start anyway.
	if !exited && status != "running" {
		return
	}
	record.Run.ID = id
	record.Run.CancelRequested = false
	record.Run.WaitingSince = time.Time{}
	record.Run.QueueSequence = 0
	console, consoleVersion := tap.snapshot()
	lines, traceVersion := trace.snapshot()
	next := runSave{status: status, console: consoleVersion, trace: traceVersion, folders: len(record.Run.Folders)}
	if next == saved {
		return
	}
	record.Console, record.Trace = console, lines
	if err := d.store.save(record); err != nil {
		log.Printf("[Agent] Run %s not stored: %v", id, err)
		return
	}
	d.queue.read(id, func(run *controlledRun) { run.saved = next })
}

// persistRuns writes every run that changed since its last write.
func (d *agentDaemon) persistRuns() {
	if d.store == nil {
		return
	}
	type entry struct {
		id  string
		run *controlledRun
	}
	d.queue.mu.Lock()
	runs := make([]entry, 0, len(d.queue.runs))
	for id, run := range d.queue.runs {
		if !run.restored {
			runs = append(runs, entry{id, run})
		}
	}
	d.queue.mu.Unlock()
	for _, e := range runs {
		d.persistRun(e.id, e.run)
	}
}

// persistLoop writes the live runs that changed every runStoreFlush, until ctx
// ends.
func (d *agentDaemon) persistLoop(ctx context.Context) {
	if d.store == nil {
		return
	}
	ticker := time.NewTicker(runStoreFlush)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.persistRuns()
		}
	}
}

// pruneRuns applies the store's retention, sparing the runs still going.
func (d *agentDaemon) pruneRuns() {
	if d.store == nil {
		return
	}
	live := map[string]bool{}
	d.queue.mu.Lock()
	for id, run := range d.queue.runs {
		select {
		case <-run.exited:
		default:
			live[id] = true
		}
	}
	d.queue.mu.Unlock()
	d.store.prune(runStoreRetained, live)
}

// restoreRuns puts the stored runs back in the index, before the desktop can
// list anything. A restored run has exited, has no session and cannot be
// waited on or stopped; its terminal route replays what it showed, through the
// read-only path the headless trace uses, and ends.
func (d *agentDaemon) restoreRuns() {
	records := d.store.load()
	if len(records) == 0 {
		return
	}
	d.queue.mu.Lock()
	defer d.queue.mu.Unlock()
	if d.queue.runs == nil {
		d.queue.runs = map[string]*controlledRun{}
	}
	restored := 0
	for _, record := range records {
		id := record.Run.ID
		if d.queue.runs[id] != nil {
			continue
		}
		entry := record.Run
		entry.SessionID = ""
		entry.ExternalTerminal = ""
		entry.WaitingSince = time.Time{}
		entry.CancelRequested = false
		entry.QueueSequence = 0
		entry.Restored = true
		replay := newRunTrace()
		replay.lines = record.Trace
		if len(replay.lines) == 0 && len(record.Console) > 0 {
			replay.lines = []string{string(record.Console)}
		}
		replay.closed = true
		d.queue.sequence++
		run := &controlledRun{taskID: entry.TaskID, sequence: d.queue.sequence, root: entry.Directory, desktop: entry,
			exited: make(chan struct{}), trace: replay, restored: true, finishedAt: record.FinishedAt}
		run.once.Do(func() { close(run.exited) })
		d.queue.runs[id] = run
		restored++
	}
	log.Printf("[Agent] %d execution(s) restored from the run store", restored)
}
