package db

import (
	"fmt"
	"log"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"tasks/internal/models"
	"tasks/internal/tracker"
	"tasks/internal/trackerapi"
)

// The background synchronisation loop.
//
// Reading a whole project costs one request per hundred work items: fourteen
// for a project of fourteen hundred. Repeating that every minute is both
// pointless and rude to the instance. The loop therefore queues one single
// synchronisation per project, bounded on the update date — `updated >= -Nm` in
// JQL, `since` on GitHub — which brings back only what the tracker has touched
// since the previous pass. A tracker that cannot narrow a search is asked for
// all of it, which is still one request per hundred work items where the unit
// re-read it replaces cost one per work item.
//
// Three guards complete it: one pass at a time per project, a step back when
// the instance answers that it has had enough (429), and a spaced full pass
// that catches what a read by update date cannot see, namely a work item that
// left the perimeter.

const (
	// autoSyncMinInterval bounds the configurable interval. Under thirty
	// seconds, the tracker is polled faster than it changes.
	autoSyncMinInterval = 30 * time.Second
	// autoSyncFullEvery spaces the full passes. They catch the disappearances,
	// which no read by update date ever reports.
	autoSyncFullEvery = 30 * time.Minute
	// autoSyncOverlap widens the incremental window. JQL reasons to the minute
	// and clocks drift: with no margin, a work item changed exactly between two
	// passes would fall through.
	autoSyncOverlap = 3
	// autoSyncMaxWindow bounds the window when the loop has slept for a long
	// time (a machine suspended): beyond it, the full pass is the honest read.
	autoSyncMaxWindow = 24 * 60
)

// AutoSyncState is what the interface shows about the loop.
type AutoSyncState struct {
	Enabled     bool   `json:"enabled"`
	IntervalSec int    `json:"intervalSec"`
	Running     bool   `json:"running"`
	LastRunAt   string `json:"lastRunAt,omitempty"`
	LastError   string `json:"lastError,omitempty"`
	// LastImported is how many work items the last pass actually wrote.
	LastImported int `json:"lastImported"`
	// Passes and Imported count the whole session, to make the loop's cost
	// visible rather than a matter of trust.
	Passes   int `json:"passes"`
	Imported int `json:"imported"`
	// BackoffUntil is set when the tracker asked to be left alone.
	BackoffUntil string `json:"backoffUntil,omitempty"`
}

type autoSync struct {
	mu           sync.Mutex
	running      bool
	lastRunAt    time.Time
	lastError    string
	lastImported int
	passes       int
	imported     int
	backoffUntil time.Time
	lastFullSync map[string]time.Time
	lastPassAt   map[string]time.Time
}

// StartAutoSync runs the loop until the process stops. It reads its settings on
// every tick, so switching it on or changing the interval takes effect without a
// restart.
func (d *DB) StartAutoSync() {
	if d.auto == nil {
		d.auto = &autoSync{lastFullSync: map[string]time.Time{}, lastPassAt: map[string]time.Time{}}
	}

	go func() {
		for {
			time.Sleep(30 * time.Second)

			projects, err := d.GetProjects()
			if err != nil {
				continue
			}

			hasActiveAutoSync := false
			for _, p := range projects {
				if p.AutoSyncEnabled {
					hasActiveAutoSync = true
					break
				}
			}

			settings, _ := d.GetSettings()
			if !hasActiveAutoSync && (settings == nil || !settings.AutoSyncEnabled) {
				continue
			}

			d.auto.mu.Lock()
			backoff := d.auto.backoffUntil
			busy := d.auto.running
			if !busy {
				d.auto.running = true
			}
			d.auto.mu.Unlock()

			if busy {
				// The previous pass has not finished: starting a second one
				// would only pile requests onto an already slow instance.
				continue
			}
			if time.Now().Before(backoff) {
				d.auto.mu.Lock()
				d.auto.running = false
				d.auto.mu.Unlock()
				continue
			}

			d.runAutoSyncPassGuarded(settings)
		}
	}()
}

func (d *DB) autoSyncInterval() time.Duration {
	settings, _ := d.GetSettings()
	if settings == nil || settings.AutoSyncIntervalSec <= 0 {
		return time.Minute
	}
	interval := time.Duration(settings.AutoSyncIntervalSec) * time.Second
	if interval < autoSyncMinInterval {
		return autoSyncMinInterval
	}
	return interval
}

// runAutoSyncPassGuarded isolates one pass from the loop. A panic in a background
// pass used to take the whole process down, and a process that dies is a window
// that reopens: the interface is served by this binary, and starting it again
// reloads the tab. The jobs queue has had this guard from the start; the loop
// deserved the same.
func (d *DB) runAutoSyncPassGuarded(settings *models.Settings) {
	defer func() {
		if rec := recover(); rec != nil {
			log.Printf("[autosync] panique pendant une passe: %v\n%s", rec, debug.Stack())
			d.recordAutoSyncError(fmt.Errorf("panique pendant une passe: %v", rec))

			d.auto.mu.Lock()
			d.auto.running = false
			d.auto.mu.Unlock()
		}
	}()
	d.runAutoSyncPass(settings)
}

// runAutoSyncPass queues one synchronisation per project that opted in, bounded
// on what the tracker has touched since the previous pass.
func (d *DB) runAutoSyncPass(settings *models.Settings) {
	defer func() {
		d.auto.mu.Lock()
		d.auto.running = false
		d.auto.lastRunAt = time.Now()
		d.auto.passes++
		d.auto.mu.Unlock()
	}()

	projects, err := d.GetProjects()
	if err != nil {
		d.recordAutoSyncError(err)
		return
	}

	queued := 0
	var failures []string

	for _, proj := range projects {
		if !proj.AutoSyncEnabled {
			continue
		}

		intervalMin := models.NormalizeAutoSyncIntervalMin(proj.AutoSyncIntervalMin)

		d.auto.mu.Lock()
		lastRun, hasRun := d.auto.lastPassAt[proj.ID]
		d.auto.mu.Unlock()

		if hasRun && time.Since(lastRun) < time.Duration(intervalMin)*time.Minute {
			continue
		}

		ts, tsErr := d.TrackerForProject(&proj)
		if tsErr != nil || ts == nil || ts.Name() == "local" || !ts.Supports(tracker.CapSync) {
			continue
		}

		// The window is read before the pass is dated, since it is computed
		// from the previous one. A tracker that cannot narrow a search reads
		// the whole project, and that read counts as the full pass it is.
		window := 0
		if ts.Supports(tracker.CapIncrementalSync) {
			window = d.autoSyncWindow(proj.ID)
		}

		d.auto.mu.Lock()
		d.auto.lastPassAt[proj.ID] = time.Now()
		d.auto.mu.Unlock()

		// The pass runs under the project's owner. It is nobody's request, so
		// there is no acting user to carry, and a tracker whose credential is
		// personal has none of its own to fall back on: the owner is the person
		// who turned this loop on, and it is their token it reads with. An
		// ownerless project keeps the historical behaviour, the server
		// credential, which is what SECTILE_JIRA_TOKEN is for.
		owner := strings.TrimSpace(proj.OwnerUserID)

		if _, syncErr := d.EnqueueSyncWith(owner, ts.Name(), "", proj.ID, SyncOptions{WindowMin: window, Background: true}); syncErr != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", proj.Name, syncErr))
			continue
		}
		queued++
	}

	if len(failures) > 0 {
		d.auto.mu.Lock()
		d.auto.lastError = strings.Join(failures, " | ")
		d.auto.mu.Unlock()
	}

	if queued > 0 {
		log.Printf("[autosync] %d synchronisation(s) de projet en file d'attente", queued)
	}
}

// recordAutoSyncPass is how a queued pass reports back. The job outlives the
// pass that filed it, so what a pass actually imported is only known here — and
// so is whether it succeeded, which is what dates the full read.
//
// A full pass that failed is not one: dating it would narrow every pass that
// follows for half an hour, on a project whose copy the failure just left
// incomplete. It stays undated, so the loop keeps asking for the whole project
// until one read comes back.
func (d *DB) recordAutoSyncPass(projectID string, window int, imported int, failed bool, message string) {
	if d.auto == nil {
		return
	}
	d.auto.mu.Lock()
	defer d.auto.mu.Unlock()
	d.auto.lastImported = imported
	d.auto.imported += imported
	if failed {
		d.auto.lastError = message
		return
	}
	d.auto.lastError = ""
	if window == 0 && projectID != "" {
		if d.auto.lastFullSync == nil {
			d.auto.lastFullSync = map[string]time.Time{}
		}
		d.auto.lastFullSync[projectID] = time.Now()
	}
}

// autoSyncWindow returns the number of minutes to read back for a project: zero
// for a full pass, which happens on the first pass and at a slow cadence
// afterwards.
func (d *DB) autoSyncWindow(projectID string) int {
	d.auto.mu.Lock()
	defer d.auto.mu.Unlock()

	lastFull, hadFull := d.auto.lastFullSync[projectID]
	if !hadFull || time.Since(lastFull) > autoSyncFullEvery {
		return 0
	}

	lastPass, hadPass := d.auto.lastPassAt[projectID]
	if !hadPass {
		return 0
	}

	minutes := int(time.Since(lastPass).Minutes()) + autoSyncOverlap
	if minutes > autoSyncMaxWindow {
		return 0
	}
	return minutes
}

// isRateLimited reports whether the tracker asked to be left alone.
func isRateLimited(err error) bool {
	if err == nil {
		return false
	}
	if trackerapi.IsRateLimited(err) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "429") || strings.Contains(strings.ToLower(msg), "rate limit")
}

// enterAutoSyncBackoff steps back for a while. A tracker that says it has had
// enough is answered by waiting, not by trying again a minute later.
func (d *DB) enterAutoSyncBackoff() {
	// The backoff is asked for by any rate-limited tracker call, including one
	// made before the loop was ever started; there is then nothing to step
	// back from.
	if d.auto == nil {
		return
	}
	d.auto.mu.Lock()
	defer d.auto.mu.Unlock()
	d.auto.backoffUntil = time.Now().Add(10 * time.Minute)
	log.Printf("[autosync] limite de débit atteinte, pause jusqu'à %s", d.auto.backoffUntil.Format(time.Kitchen))
}

func (d *DB) recordAutoSyncError(err error) {
	d.auto.mu.Lock()
	defer d.auto.mu.Unlock()
	d.auto.lastError = err.Error()
}

// AutoSyncStatus reports what the loop has been doing, for the interface.
func (d *DB) AutoSyncStatus() AutoSyncState {
	settings, _ := d.GetSettings()
	state := AutoSyncState{IntervalSec: 60}
	if settings != nil {
		state.Enabled = settings.AutoSyncEnabled
		if settings.AutoSyncIntervalSec > 0 {
			state.IntervalSec = settings.AutoSyncIntervalSec
		}
	}
	if d.auto == nil {
		return state
	}

	d.auto.mu.Lock()
	defer d.auto.mu.Unlock()
	state.Running = d.auto.running
	state.LastImported = d.auto.lastImported
	state.Passes = d.auto.passes
	state.Imported = d.auto.imported
	state.LastError = d.auto.lastError
	if !d.auto.lastRunAt.IsZero() {
		state.LastRunAt = d.auto.lastRunAt.Format(time.RFC3339)
	}
	if time.Now().Before(d.auto.backoffUntil) {
		state.BackoffUntil = d.auto.backoffUntil.Format(time.RFC3339)
	}
	return state
}
