package db

import (
	"fmt"
	"log"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"tasks/internal/models"
	"tasks/internal/trackerapi"
)

// Boucle de synchronisation de fond.
//
// Une passe complète d'un projet coûte une requête par tranche de cent tickets:
// quatorze pour un projet de mille quatre cents. La répéter toutes les minutes
// serait à la fois inutile et grossier envers l'instance. La boucle met donc en
// file une relecture unitaire de chaque ticket non terminé du projet, par le
// GetIssue de son tracker; la lecture incrémentale par JQL bornée sur `updated`
// que la fenêtre ci-dessous calcule n'est pas encore branchée sur l'adaptateur.
//
// Trois garde-fous complètent cela: une seule passe à la fois par projet, un
// recul progressif quand l'instance répond qu'elle en a assez (429), et une
// passe complète espacée qui rattrape ce qu'une lecture incrémentale ne peut pas
// voir, à savoir un ticket sorti du périmètre.

const (
	// autoSyncMinInterval borne l'intervalle réglable. Sous trente secondes, on
	// interroge le tracker plus vite qu'il ne change.
	autoSyncMinInterval = 30 * time.Second
	// autoSyncFullEvery espace les passes complètes. Elles rattrapent les
	// disparitions, qu'aucune lecture par date de mise à jour ne signale.
	autoSyncFullEvery = 30 * time.Minute
	// autoSyncOverlap élargit la fenêtre incrémentale. La JQL raisonne à la
	// minute et les horloges dérivent: sans marge, un ticket modifié pile entre
	// deux passes passerait au travers.
	autoSyncOverlap = 3
	// autoSyncMaxWindow borne la fenêtre quand la boucle a dormi longtemps
	// (machine en veille): au delà, la passe complète est plus honnête.
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
				// La passe précédente n'a pas fini: en lancer une seconde ne
				// ferait qu'empiler des requêtes sur une instance déjà lente.
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

// runAutoSyncPass queues a re-read of every unfinished work item of every
// project that opted in, through the project's own tracker.
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

	imported := 0
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

		d.auto.mu.Lock()
		d.auto.lastPassAt[proj.ID] = time.Now()
		d.auto.mu.Unlock()

		// Retrieve all non-finished tasks for this project
		tasks, taskErr := d.GetTasks("", "", "", "", proj.ID, "", "", "", "", nil, nil, false)
		if taskErr != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", proj.Name, taskErr))
			continue
		}

		// The pass runs under the project's owner. It is nobody's request, so
		// there is no acting user to carry, and a tracker whose credential is
		// personal has none of its own to fall back on: the owner is the person
		// who turned this loop on, and it is their token it reads with. An
		// ownerless project keeps the historical behaviour, the server
		// credential, which is what SECTILE_JIRA_TOKEN is for.
		owner := strings.TrimSpace(proj.OwnerUserID)

		for _, t := range tasks {
			// Skip finished tasks
			if t.Status == models.StatusFinished || strings.EqualFold(t.TrackerStatus, "Done") {
				continue
			}

			// Enqueue single task sync activity in the queue
			if _, syncErr := d.EnqueueSingleTaskSyncAs(owner, &t); syncErr != nil {
				failures = append(failures, fmt.Sprintf("%s (%s): %v", proj.Name, t.Key, syncErr))
			} else {
				imported++
			}
		}
	}

	d.auto.mu.Lock()
	d.auto.lastImported = imported
	d.auto.imported += imported
	d.auto.lastError = strings.Join(failures, " | ")
	d.auto.mu.Unlock()

	if imported > 0 {
		log.Printf("[autosync] %d tâche(s) de synchronisation en file d'attente", imported)
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
