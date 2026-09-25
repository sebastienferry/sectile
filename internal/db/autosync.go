package db

import (
	"database/sql"
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
	// Passes and Imported count every pass the deployment has run, whichever
	// server ran it, to make the loop's cost visible rather than a matter of
	// trust.
	Passes   int `json:"passes"`
	Imported int `json:"imported"`
	// BackoffUntil is set when the tracker asked to be left alone.
	BackoffUntil string `json:"backoffUntil,omitempty"`
}

// autoSync is what stays in the process: whether this process is in the middle
// of a pass. Everything else the loop knows, its pacing, its backoff and what it
// reports, is in the database, because several server instances run the loop
// against one database and must agree on it. See docs/clarifications/404.md.
type autoSync struct {
	mu      sync.Mutex
	running bool
}

// autoSyncPacing is when a project was last claimed by a pass, and when it was
// last read in full. A zero time means never.
type autoSyncPacing struct {
	lastPass time.Time
	lastFull time.Time
}

// autoSyncBackoff is how long the loop steps back when a tracker says it has
// had enough.
const autoSyncBackoff = 10 * time.Minute

// StartAutoSync runs the loop until the process stops. It reads its settings on
// every tick, so switching it on or changing the interval takes effect without a
// restart.
func (d *DB) StartAutoSync() {
	if d.auto == nil {
		d.auto = &autoSync{}
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

			// A backoff asked for by any instance holds for all of them: the
			// tracker that answered 429 is the same one for everybody.
			if time.Now().UTC().Before(d.autoSyncBackoffUntil()) {
				continue
			}

			d.auto.mu.Lock()
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
			d.setAutoSyncRunning(false)
		}
	}()
	d.runAutoSyncPass(settings)
}

func (d *DB) setAutoSyncRunning(running bool) {
	if d.auto == nil {
		return
	}
	d.auto.mu.Lock()
	d.auto.running = running
	d.auto.mu.Unlock()
}

// runAutoSyncPass queues one synchronisation per project that opted in, bounded
// on what the tracker has touched since the previous pass.
func (d *DB) runAutoSyncPass(settings *models.Settings) {
	defer func() {
		d.setAutoSyncRunning(false)
		if _, err := d.conn.Exec(`UPDATE auto_sync_state SET last_run_at = ?, passes = passes + 1 WHERE id = 1`, time.Now().UTC()); err != nil {
			log.Printf("[autosync] état de la passe non enregistré: %v", err)
		}
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

		ts, tsErr := d.TrackerForProject(&proj)
		if tsErr != nil || ts == nil || ts.Name() == "local" || !ts.Supports(tracker.CapSync) {
			continue
		}

		// The claim is what makes one pass, among the instances sharing the
		// database, the one that queues this project. It also dates the pass,
		// and returns the pacing as it stood before, which is what the window
		// is computed from.
		intervalMin := models.NormalizeAutoSyncIntervalMin(proj.AutoSyncIntervalMin)
		now := time.Now().UTC()
		pacing, claimed, claimErr := d.claimAutoSyncPass(proj.ID, time.Duration(intervalMin)*time.Minute, now)
		if claimErr != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", proj.Name, claimErr))
			continue
		}
		if !claimed {
			continue
		}

		// A tracker that cannot narrow a search reads the whole project, and
		// that read counts as the full pass it is.
		window := 0
		if ts.Supports(tracker.CapIncrementalSync) {
			window = autoSyncWindowFrom(pacing, now)
		}

		// Nobody asked for this pass, and it reads with the server credential
		// of the project's provider, as every synchronisation does (#464). It
		// used to borrow the owner's personal token, which failed the day that
		// token was locked, missing, or its owner gone.
		if _, syncErr := d.EnqueueSyncWith("", ts.Name(), "", proj.ID, SyncOptions{WindowMin: window, Background: true}); syncErr != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", proj.Name, syncErr))
			continue
		}
		queued++
	}

	if len(failures) > 0 {
		d.recordAutoSyncError(fmt.Errorf("%s", strings.Join(failures, " | ")))
	}

	if queued > 0 {
		log.Printf("[autosync] %d synchronisation(s) de projet en file d'attente", queued)
	}
}

// claimAutoSyncPass claims a project for this pass when it is due, and reports
// whether it did along with the pacing read before the claim.
//
// The claim is one conditional UPDATE: it dates the pass only if nobody dated
// one within the interval. Two instances claiming at the same moment both read
// the same pacing, and exactly one of them sees its UPDATE touch the row.
func (d *DB) claimAutoSyncPass(projectID string, interval time.Duration, now time.Time) (autoSyncPacing, bool, error) {
	var pacing autoSyncPacing
	if _, err := d.conn.Exec(`INSERT INTO auto_sync_projects (project_id) VALUES (?) ON CONFLICT DO NOTHING`, projectID); err != nil {
		return pacing, false, fmt.Errorf("pacing of project %s: %w", projectID, err)
	}
	var lastPass, lastFull sql.NullTime
	if err := d.conn.QueryRow(`SELECT last_pass_at, last_full_sync_at FROM auto_sync_projects WHERE project_id = ?`, projectID).Scan(&lastPass, &lastFull); err != nil {
		return pacing, false, fmt.Errorf("pacing of project %s: %w", projectID, err)
	}
	if lastPass.Valid {
		pacing.lastPass = lastPass.Time
	}
	if lastFull.Valid {
		pacing.lastFull = lastFull.Time
	}
	result, err := d.conn.Exec(`UPDATE auto_sync_projects SET last_pass_at = ?
		WHERE project_id = ? AND (last_pass_at IS NULL OR last_pass_at < ?)`,
		now, projectID, now.Add(-interval))
	if err != nil {
		return pacing, false, fmt.Errorf("claiming project %s: %w", projectID, err)
	}
	touched, _ := result.RowsAffected()
	return pacing, touched == 1, nil
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
	lastError := ""
	if failed {
		lastError = message
	}
	if _, err := d.conn.Exec(`UPDATE auto_sync_state SET last_imported = ?, imported = imported + ?, last_error = ? WHERE id = 1`,
		imported, imported, lastError); err != nil {
		log.Printf("[autosync] résultat de la passe non enregistré: %v", err)
	}
	if failed || window != 0 || projectID == "" {
		return
	}
	if _, err := d.conn.Exec(`INSERT INTO auto_sync_projects (project_id, last_full_sync_at) VALUES (?, ?)
		ON CONFLICT (project_id) DO UPDATE SET last_full_sync_at = excluded.last_full_sync_at`,
		projectID, time.Now().UTC()); err != nil {
		log.Printf("[autosync] lecture complète de %s non datée: %v", projectID, err)
	}
}

// autoSyncWindowFrom returns the number of minutes to read back for a project:
// zero for a full pass, which happens on the first pass and at a slow cadence
// afterwards.
func autoSyncWindowFrom(p autoSyncPacing, now time.Time) int {
	if p.lastFull.IsZero() || now.Sub(p.lastFull) > autoSyncFullEvery {
		return 0
	}
	if p.lastPass.IsZero() {
		return 0
	}
	minutes := int(now.Sub(p.lastPass).Minutes()) + autoSyncOverlap
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
// enough is answered by waiting, not by trying again a minute later, and by
// every instance, since it is the same tracker for all of them.
func (d *DB) enterAutoSyncBackoff() {
	until := time.Now().UTC().Add(autoSyncBackoff)
	if _, err := d.conn.Exec(`UPDATE auto_sync_state SET backoff_until = ? WHERE id = 1`, until); err != nil {
		log.Printf("[autosync] pause non enregistrée: %v", err)
		return
	}
	log.Printf("[autosync] limite de débit atteinte, pause jusqu'à %s", until.Local().Format(time.Kitchen))
}

// autoSyncBackoffUntil is when the current backoff ends, or the zero time.
func (d *DB) autoSyncBackoffUntil() time.Time {
	var until sql.NullTime
	if err := d.conn.QueryRow(`SELECT backoff_until FROM auto_sync_state WHERE id = 1`).Scan(&until); err != nil || !until.Valid {
		return time.Time{}
	}
	return until.Time
}

func (d *DB) recordAutoSyncError(err error) {
	if _, execErr := d.conn.Exec(`UPDATE auto_sync_state SET last_error = ? WHERE id = 1`, err.Error()); execErr != nil {
		log.Printf("[autosync] erreur non enregistrée (%v): %v", err, execErr)
	}
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
	if d.auto != nil {
		d.auto.mu.Lock()
		state.Running = d.auto.running
		d.auto.mu.Unlock()
	}

	var lastRun, backoff sql.NullTime
	if err := d.conn.QueryRow(`SELECT last_run_at, last_error, last_imported, passes, imported, backoff_until FROM auto_sync_state WHERE id = 1`).
		Scan(&lastRun, &state.LastError, &state.LastImported, &state.Passes, &state.Imported, &backoff); err != nil {
		return state
	}
	if lastRun.Valid {
		state.LastRunAt = lastRun.Time.Format(time.RFC3339)
	}
	if backoff.Valid && time.Now().UTC().Before(backoff.Time) {
		state.BackoffUntil = backoff.Time.Format(time.RFC3339)
	}
	return state
}
