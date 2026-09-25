package db

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"tasks/internal/models"
)

// Several server instances may share one PostgreSQL database. Each one is a row
// in server_instances that it keeps fresh, and each piece of work it starts
// carries its id in task_activities.instance_id. A start, and every live
// instance periodically, reclaims only the work whose owner is no longer seen:
// reclaiming everything, as a single process may, would fail the jobs and cancel
// the client runs of an instance that is still serving them.
//
// The bounds are variables so tests can shorten them.
var (
	// instanceHeartbeatEvery is how often a serving instance says it is alive.
	instanceHeartbeatEvery = 10 * time.Second
	// instanceDeadAfter is how long an instance may stay silent before its work
	// is reclaimed. Several heartbeats, so one slow write does not kill it.
	instanceDeadAfter = 45 * time.Second
	// instanceReclaimEvery is how often a serving instance looks for the work of
	// instances that went away without a word.
	instanceReclaimEvery = 15 * time.Second
)

// interruptedByRestart is the error a reclaimed server job carries. The words
// predate several instances and stay: to whoever reads the board, the process
// that ran the job did restart.
const interruptedByRestart = "Interrupted by server restart"

// interruptedClientRun is the summary of a reclaimed client-owned remote run.
// It carries the disconnect note on purpose: the session that owned the run
// died with its instance, which is a disconnection the client did not choose,
// so the owner may still report the real outcome through finish_run exactly as
// after any other disconnection (#408).
const interruptedClientRun = "Interrupted by server restart: " + models.RunDisconnectNote

// InstanceID names this process among the server instances sharing the
// database.
func (d *DB) InstanceID() string { return d.instanceID }

// InstanceLocation is a live server instance and where the others reach it.
type InstanceLocation struct {
	ID      string
	Address string
}

// LiveInstance returns an instance seen within the liveness bound. An instance
// that is not, or was never registered, is not somewhere to send anything.
func (d *DB) LiveInstance(id string) (InstanceLocation, bool) {
	cutoff := time.Now().UTC().Add(-instanceDeadAfter)
	location := InstanceLocation{ID: id}
	if err := d.conn.QueryRow(`SELECT address FROM server_instances WHERE id = ? AND last_seen >= ?`, id, cutoff).Scan(&location.Address); err != nil {
		return InstanceLocation{}, false
	}
	return location, true
}

// LiveInstances lists the instances seen within the liveness bound, this one
// included, ordered by id.
func (d *DB) LiveInstances() []InstanceLocation {
	cutoff := time.Now().UTC().Add(-instanceDeadAfter)
	rows, err := d.conn.Query(`SELECT id, address FROM server_instances WHERE last_seen >= ? ORDER BY id`, cutoff)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []InstanceLocation
	for rows.Next() {
		var l InstanceLocation
		if rows.Scan(&l.ID, &l.Address) == nil {
			out = append(out, l)
		}
	}
	return out
}

// recoverInterruptedRuns reclaims, at start, the work an earlier process left
// unfinished. It runs on every start, on every engine.
//
// It used to live inside applyLegacyMigrations, which only ever runs on SQLite.
// A PostgreSQL deployment therefore never recovered anything: a restart left
// its activities `running` forever, with nothing able to close them. Being a
// repair of data rather than of schema is what let it hide there; it is not a
// migration, and the numbered scheme has no place for something that must run
// every time. See docs/adrs/0021.
//
// An engine that serves one process at a time lost everything unfinished with
// that process, so all of it is reclaimed and every instance row is dropped.
// Otherwise only the work of instances that are gone is reclaimed.
func (d *DB) recoverInterruptedRuns() {
	if !d.dialect.ServesOneProcess() {
		if _, err := d.reclaimDeadInstances(time.Now().UTC()); err != nil {
			log.Printf("⚠️  Reprise des exécutions interrompues : %v", err)
		}
		return
	}
	// Work the server itself was running cannot survive its own restart.
	_, _ = d.conn.Exec("UPDATE task_activities SET status = 'failed', error = ? WHERE status IN ('running', 'queued', 'pending') AND skill_id != 'remote_run';",
		interruptedByRestart)
	// A remote run dispatched to an agent outlives the server: its supervisor
	// watches the real process and reports the outcome on reconnection. A run a
	// client started is owned by that client's MCP session, which the restart
	// destroyed along with every other, so nothing is left that could ever close
	// it. Canceled rather than failed: the work did not fail here, its outcome
	// merely became unknowable.
	_, _ = d.conn.Exec("UPDATE task_activities SET status = 'canceled', summary = ?, completed_at = ?, waiting_since = NULL WHERE status IN ('running', 'queued', 'pending') AND skill_id = 'remote_run' AND action != ?;",
		interruptedClientRun, time.Now(), RunActionAgent)
	_, _ = d.conn.Exec("DELETE FROM server_instances;")
	_, _ = d.conn.Exec("DELETE FROM agent_presence;")
}

// reclaimDeadInstances ends the unfinished work of instances not seen since the
// liveness bound, with the outcomes a restart has always given it, and forgets
// those instances. Work with no owner was written by a version that recorded
// none, and is reclaimed as a restart always did. It reports how many
// activities it ended.
//
// Every statement only touches rows that are still unfinished and still
// orphaned, so two instances reclaiming at the same moment end each row once.
func (d *DB) reclaimDeadInstances(now time.Time) (int64, error) {
	cutoff := now.Add(-instanceDeadAfter)
	const orphaned = ` AND (instance_id = '' OR instance_id NOT IN (SELECT id FROM server_instances WHERE last_seen >= ?))`

	d.mu.Lock()
	defer d.mu.Unlock()

	jobs, err := d.conn.Exec(`UPDATE task_activities SET status = 'failed', error = ?
		WHERE status IN ('running', 'queued', 'pending') AND skill_id != 'remote_run'`+orphaned,
		interruptedByRestart, cutoff)
	if err != nil {
		return 0, fmt.Errorf("reclaiming server jobs: %w", err)
	}
	// Agent-dispatched runs are left alone, as a restart leaves them: their
	// supervisor reports the real outcome whichever instance it reconnects to.
	runs, err := d.conn.Exec(`UPDATE task_activities SET status = 'canceled', summary = ?, completed_at = ?, waiting_since = NULL
		WHERE status IN ('running', 'queued', 'pending') AND skill_id = 'remote_run' AND action != ?`+orphaned,
		interruptedClientRun, now, RunActionAgent, cutoff)
	if err != nil {
		return 0, fmt.Errorf("reclaiming client runs: %w", err)
	}
	if _, err := d.conn.Exec(`DELETE FROM server_instances WHERE last_seen < ?`, cutoff); err != nil {
		return 0, fmt.Errorf("forgetting dead instances: %w", err)
	}
	// The agents those instances held reconnect elsewhere and register there;
	// until then nothing may be forwarded to an instance that is gone.
	if _, err := d.conn.Exec(`DELETE FROM agent_presence WHERE instance_id NOT IN (SELECT id FROM server_instances)`); err != nil {
		return 0, fmt.Errorf("forgetting the agents of dead instances: %w", err)
	}
	ended, _ := jobs.RowsAffected()
	canceled, _ := runs.RowsAffected()
	return ended + canceled, nil
}

// StartInstance registers this process as a serving instance, then keeps its
// row fresh and reclaims the work of instances that went away. Only a serving
// process calls it: a tool that opens the store, such as sectile-migrate, or a
// test, is not an instance and must not look like one.
//
// The returned stop ends the loop and removes the row. The server does not call
// it: a process that stops is noticed through its silence.
func (d *DB) StartInstance() (stop func(), err error) {
	if err := d.registerInstance(time.Now().UTC()); err != nil {
		return nil, err
	}
	done := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		heartbeat := time.NewTicker(instanceHeartbeatEvery)
		defer heartbeat.Stop()
		reclaim := time.NewTicker(instanceReclaimEvery)
		defer reclaim.Stop()
		for {
			select {
			case <-done:
				return
			case <-heartbeat.C:
				d.heartbeatInstance(time.Now().UTC())
			case <-reclaim.C:
				ended, err := d.reclaimDeadInstances(time.Now().UTC())
				if err != nil {
					log.Printf("⚠️  Reprise des exécutions d'instances disparues : %v", err)
				} else if ended > 0 {
					log.Printf("Reprise de %d exécution(s) laissée(s) par une instance disparue", ended)
				}
			}
		}
	}()
	var once sync.Once
	return func() {
		once.Do(func() {
			close(done)
			<-finished
			d.mu.Lock()
			defer d.mu.Unlock()
			_, _ = d.conn.Exec(`DELETE FROM server_instances WHERE id = ?`, d.instanceID)
		})
	}, nil
}

func (d *DB) registerInstance(now time.Time) error {
	hostname, _ := os.Hostname()
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, err := d.conn.Exec(`INSERT INTO server_instances (id, hostname, pid, started_at, last_seen, address) VALUES (?, ?, ?, ?, ?, ?)`,
		d.instanceID, hostname, os.Getpid(), now, now, d.instanceAddress); err != nil {
		return fmt.Errorf("registering server instance %s: %w", d.instanceID, err)
	}
	return nil
}

// heartbeatInstance refreshes this instance's row. A row that is gone means
// another instance took this one for dead, after a silence longer than the
// bound, and has already reclaimed its work: the instance says so and registers
// again, since it is plainly still serving.
func (d *DB) heartbeatInstance(now time.Time) {
	d.mu.Lock()
	result, err := d.conn.Exec(`UPDATE server_instances SET last_seen = ? WHERE id = ?`, now, d.instanceID)
	d.mu.Unlock()
	if err != nil {
		log.Printf("⚠️  Signal de vie de l'instance %s : %v", d.instanceID, err)
		return
	}
	if touched, _ := result.RowsAffected(); touched > 0 {
		return
	}
	log.Printf("⚠️  L'instance %s a été tenue pour disparue ; ses exécutions en cours ont pu être reprises. Réenregistrement.", d.instanceID)
	if err := d.registerInstance(now); err != nil {
		log.Printf("⚠️  %v", err)
	}
}
