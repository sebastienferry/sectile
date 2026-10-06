package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"slices"
	"sort"
	"strings"
	"time"

	"tasks/internal/models"

	"github.com/google/uuid"
)

// tasksTrackerKeyIndex makes one record per remote ticket a constraint (#741).
// It is created by the adoption, after the duplicates are merged, never by a
// numbered migration: before the merge, the rows it would refuse exist.
const tasksTrackerKeyIndex = "ux_tasks_tracker_key"

// adoptTrackers moves a database whose projects each held their own tracker to
// one where trackers are shared sources (#741). Migration 49 only adds the
// tables; this step fills them, because it needs what SQL cannot express: the
// tracker a project's fields name once resolved against the settings, and the
// stage order that picks the survivor of a duplicate.
//
// For each project, oldest first, it derives the tracker its fields name and
// inserts it unless one with that identity exists, so two projects on one Jira
// space share one tracker while two spaces of one site stay two. The first
// project of an identity gives the tracker its name and its board mirror; a
// later one that disagrees is logged. Auto-sync is on if either project had
// it, with the shorter interval. Each local project gets a tracker of its own.
//
// It then tags every ticket with its project's tracker, merges the tickets two
// projects imported from one tracker into one record (the most advanced stage
// wins, the pull requests, labels and changed repositories are united, the
// activities, comments, pins and batch places follow the survivor, and the
// loser's id becomes an alias), and creates the unique index that keeps it so.
//
// It is idempotent and holds the migration lock, like
// adoptLegacyServerTrackerTokens: two replicas may start together, and once
// everything is adopted it does nothing.
func (d *DB) adoptTrackers() error {
	unlock, err := d.dialect.LockForMigration(d.conn)
	if err != nil {
		return fmt.Errorf("taking the migration lock: %w", err)
	}
	defer unlock()

	done, err := d.trackerAdoptionDone()
	if err != nil || done {
		return err
	}
	projects, err := d.scanProjectsUnsafe(false)
	if err != nil {
		return fmt.Errorf("reading the projects: %w", err)
	}
	sort.SliceStable(projects, func(i, j int) bool { return projects[i].CreatedAt.Before(projects[j].CreatedAt) })
	settings, _ := d.getSettingsUnsafe()
	indexed, err := d.indexExists(tasksTrackerKeyIndex)
	if err != nil {
		return err
	}
	err = d.conn.WithTx(func(tx *sqlTx) error {
		members, err := adoptProjectTrackers(tx, projects, settings)
		if err != nil {
			return err
		}
		if err := tagTasksWithTrackers(tx, members); err != nil {
			return err
		}
		if err := mergeDuplicateTasks(tx); err != nil {
			return err
		}
		if err := tagJiraEpicsWithTrackers(tx, members); err != nil {
			return err
		}
		if err := copyAutoSyncBookkeeping(tx, members); err != nil {
			return err
		}
		if !indexed {
			if _, err := tx.Exec("CREATE UNIQUE INDEX IF NOT EXISTS " + tasksTrackerKeyIndex + " ON tasks (tracker_id, key)"); err != nil {
				return fmt.Errorf("creating %s: %w", tasksTrackerKeyIndex, err)
			}
		}
		return nil
	})
	d.trackerCache.clear()
	return err
}

// trackerAdoptionDone says whether adoptTrackers has nothing left to do: every
// project links a tracker, every ticket of an existing project names its
// tracker, and the unique index exists. A ticket whose project is gone cannot
// be adopted and does not keep the step running at every start: it is only
// reported.
func (d *DB) trackerAdoptionDone() (bool, error) {
	var unlinked, untagged, orphans int
	if err := d.conn.QueryRow(`SELECT COUNT(*) FROM projects p WHERE NOT EXISTS (SELECT 1 FROM project_trackers pt WHERE pt.project_id = p.id)`).Scan(&unlinked); err != nil {
		return false, fmt.Errorf("counting the projects without a tracker: %w", err)
	}
	if err := d.conn.QueryRow(`SELECT COUNT(*) FROM tasks t WHERE t.tracker_id IS NULL
		AND EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id OR p.slug = t.project_id)`).Scan(&untagged); err != nil {
		return false, fmt.Errorf("counting the tickets without a tracker: %w", err)
	}
	if err := d.conn.QueryRow(`SELECT COUNT(*) FROM tasks t WHERE t.tracker_id IS NULL
		AND NOT EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id OR p.slug = t.project_id)`).Scan(&orphans); err != nil {
		return false, fmt.Errorf("counting the orphan tickets: %w", err)
	}
	if orphans > 0 {
		log.Printf("[Trackers] %d ticket(s) sans projet existant ne peuvent être rattachés à aucun tracker", orphans)
	}
	indexed, err := d.indexExists(tasksTrackerKeyIndex)
	if err != nil {
		return false, err
	}
	return unlinked == 0 && untagged == 0 && indexed, nil
}

// indexExists asks the engine's catalog whether an index of that name exists.
func (d *DB) indexExists(name string) (bool, error) {
	query := `SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = ?`
	if d.dialect.Engine() == DriverPostgres {
		query = `SELECT COUNT(*) FROM pg_indexes WHERE schemaname = current_schema() AND indexname = ?`
	}
	var count int
	if err := d.conn.QueryRow(query, name).Scan(&count); err != nil {
		return false, fmt.Errorf("looking up the index %s: %w", name, err)
	}
	return count > 0, nil
}

// trackerMembers lists, per tracker id, the references its projects' tickets
// and macros carry: every project id and slug.
type trackerMembers struct {
	order    []string
	refs     map[string][]string
	projects map[string][]string
	provider map[string]string
}

func (m *trackerMembers) add(trackerID, provider string, p models.Project) {
	if _, ok := m.refs[trackerID]; !ok {
		m.order = append(m.order, trackerID)
	}
	m.provider[trackerID] = provider
	m.projects[trackerID] = append(m.projects[trackerID], p.ID)
	m.refs[trackerID] = append(m.refs[trackerID], p.ID)
	if slug := strings.TrimSpace(p.Slug); slug != "" && slug != p.ID {
		m.refs[trackerID] = append(m.refs[trackerID], slug)
	}
}

// adoptProjectTrackers inserts the tracker each project names, links it, and
// makes it the project's default.
func adoptProjectTrackers(tx *sqlTx, projects []models.Project, settings *models.Settings) (*trackerMembers, error) {
	members := &trackerMembers{refs: map[string][]string{}, projects: map[string][]string{}, provider: map[string]string{}}
	now := time.Now().UTC()
	for _, p := range projects {
		var linked string
		err := tx.QueryRow(`SELECT tracker_id FROM project_trackers WHERE project_id = ? ORDER BY position LIMIT 1`, p.ID).Scan(&linked)
		if err != nil && err != sql.ErrNoRows {
			return nil, fmt.Errorf("reading the tracker of %s: %w", p.ID, err)
		}
		if linked != "" {
			var provider string
			if err := tx.QueryRow(`SELECT provider FROM trackers WHERE id = ?`, linked).Scan(&provider); err == nil {
				members.add(linked, provider, p)
				continue
			}
		}
		wanted := legacyTrackerOf(&p, settings)
		existing, err := trackerByIdentityOn(tx, wanted.Identity)
		if err != nil {
			return nil, err
		}
		trackerID := ""
		if existing == nil {
			wanted.ID = uuid.New().String()
			wanted.CreatedAt, wanted.UpdatedAt = now, now
			if err := insertTrackerOn(tx, &wanted); err != nil {
				return nil, err
			}
			if existing, err = trackerByIdentityOn(tx, wanted.Identity); err != nil || existing == nil {
				return nil, fmt.Errorf("reading back the tracker %s: %v", wanted.Identity, err)
			}
			trackerID = existing.ID
		} else {
			trackerID = existing.ID
			if err := settleTrackerConflict(tx, existing, &p, &wanted); err != nil {
				return nil, err
			}
		}
		if _, err := tx.Exec(`INSERT INTO project_trackers (project_id, tracker_id, position) VALUES (?, ?, 0) ON CONFLICT DO NOTHING`, p.ID, trackerID); err != nil {
			return nil, fmt.Errorf("linking %s to its tracker: %w", p.ID, err)
		}
		if _, err := tx.Exec(`UPDATE projects SET default_tracker_id = ? WHERE id = ? AND default_tracker_id = ''`, trackerID, p.ID); err != nil {
			return nil, fmt.Errorf("setting the default tracker of %s: %w", p.ID, err)
		}
		members.add(trackerID, existing.Provider, p)
	}
	return members, nil
}

// settleTrackerConflict applies what a second project of one identity brings:
// auto-sync on if it had it, with the shorter interval, and a log line when its
// board mirror differs from the one the first project gave the tracker.
func settleTrackerConflict(tx *sqlTx, existing *models.Tracker, p *models.Project, wanted *models.Tracker) error {
	if !sameBoardMirror(existing, wanted) {
		log.Printf("[Trackers] adoption : le projet %s (%s) a un miroir de board différent du tracker %s, celui du premier projet est conservé", p.ID, p.Name, existing.Identity)
	}
	if !p.AutoSyncEnabled {
		return nil
	}
	interval := wanted.AutoSyncIntervalMin
	if existing.AutoSyncEnabled && existing.AutoSyncIntervalMin < interval {
		interval = existing.AutoSyncIntervalMin
	}
	if existing.AutoSyncEnabled && interval == existing.AutoSyncIntervalMin {
		return nil
	}
	if _, err := tx.Exec(`UPDATE trackers SET auto_sync_enabled = 1, auto_sync_interval_min = ? WHERE id = ?`, interval, existing.ID); err != nil {
		return fmt.Errorf("merging the auto-sync of %s: %w", existing.Identity, err)
	}
	existing.AutoSyncEnabled, existing.AutoSyncIntervalMin = true, interval
	return nil
}

func sameBoardMirror(a, b *models.Tracker) bool {
	encode := func(t *models.Tracker) string {
		raw, _ := json.Marshal([]any{t.BoardID, t.TrackerColumns, t.StageColumns, t.Sprints, t.IssueTypes})
		return string(raw)
	}
	return encode(a) == encode(b)
}

// tagTasksWithTrackers sets the tracker of every ticket a project of that
// tracker holds, by id or by slug, as tasks.project_id may store either.
func tagTasksWithTrackers(tx *sqlTx, members *trackerMembers) error {
	for _, trackerID := range members.order {
		refs := members.refs[trackerID]
		args := append([]any{trackerID}, anySlice(refs)...)
		if _, err := tx.Exec(`UPDATE tasks SET tracker_id = ? WHERE tracker_id IS NULL AND project_id IN (`+placeholders(len(refs))+`)`, args...); err != nil {
			return fmt.Errorf("tagging the tickets of tracker %s: %w", trackerID, err)
		}
	}
	return nil
}

// adoptedTask is what a duplicate merge reads of one ticket.
type adoptedTask struct {
	id, status, labels, prLinks, changed string
	prURL                                sql.NullString
	pinned                               int
	updatedAt                            time.Time
}

// mergeDuplicateTasks keeps one record per (tracker, key).
func mergeDuplicateTasks(tx *sqlTx) error {
	rows, err := tx.Query(`SELECT tracker_id, key FROM tasks WHERE tracker_id IS NOT NULL GROUP BY tracker_id, key HAVING COUNT(*) > 1`)
	if err != nil {
		return fmt.Errorf("finding the duplicate tickets: %w", err)
	}
	type pair struct{ tracker, key string }
	var duplicates []pair
	for rows.Next() {
		var p pair
		if err := rows.Scan(&p.tracker, &p.key); err != nil {
			rows.Close()
			return err
		}
		duplicates = append(duplicates, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, dup := range duplicates {
		if err := mergeDuplicate(tx, dup.tracker, dup.key); err != nil {
			return fmt.Errorf("merging %s on tracker %s: %w", dup.key, dup.tracker, err)
		}
	}
	return nil
}

func mergeDuplicate(tx *sqlTx, trackerID, key string) error {
	rows, err := tx.Query(`SELECT id, status, labels, pr_links, pr_url, COALESCE(changed_repositories, '[]'), pinned, updated_at FROM tasks WHERE tracker_id = ? AND key = ?`, trackerID, key)
	if err != nil {
		return err
	}
	var copies []adoptedTask
	for rows.Next() {
		var t adoptedTask
		if err := rows.Scan(&t.id, &t.status, &t.labels, &t.prLinks, &t.prURL, &t.changed, &t.pinned, &t.updatedAt); err != nil {
			rows.Close()
			return err
		}
		copies = append(copies, t)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if len(copies) < 2 {
		return nil
	}
	// The most advanced stage first, then the most recently updated, then the
	// id so that the outcome never depends on the order rows come back in.
	sort.SliceStable(copies, func(i, j int) bool {
		ri, rj := stageRank(copies[i].status), stageRank(copies[j].status)
		if ri != rj {
			return ri > rj
		}
		if !copies[i].updatedAt.Equal(copies[j].updatedAt) {
			return copies[i].updatedAt.After(copies[j].updatedAt)
		}
		return copies[i].id < copies[j].id
	})
	survivor, losers := copies[0], copies[1:]

	var links []models.TaskPullRequest
	_ = json.Unmarshal([]byte(survivor.prLinks), &links)
	links = models.NormalizePullRequestLinks(links)
	labels, changed := decodeStrings(survivor.labels), decodeStrings(survivor.changed)
	pinned := survivor.pinned
	prURL := strings.TrimSpace(survivor.prURL.String)
	for _, loser := range losers {
		var theirs []models.TaskPullRequest
		_ = json.Unmarshal([]byte(loser.prLinks), &theirs)
		for _, link := range models.NormalizePullRequestLinks(theirs) {
			if !slices.ContainsFunc(links, func(l models.TaskPullRequest) bool { return l.URL == link.URL }) {
				links = append(links, link)
			}
		}
		labels = unionStrings(labels, decodeStrings(loser.labels))
		changed = unionStrings(changed, decodeStrings(loser.changed))
		pinned = max(pinned, loser.pinned)
	}
	if prURL == "" {
		prURL = models.CurrentPullRequest(links)
	}
	if links == nil {
		links = []models.TaskPullRequest{}
	}
	linksJSON, _ := json.Marshal(links)
	labelsJSON, _ := json.Marshal(labels)
	changedJSON, _ := json.Marshal(changed)
	var prValue any
	if prURL != "" {
		prValue = prURL
	}
	if _, err := tx.Exec(`UPDATE tasks SET pr_links = ?, pr_url = ?, labels = ?, changed_repositories = ?, pinned = ? WHERE id = ?`,
		string(linksJSON), prValue, string(labelsJSON), string(changedJSON), pinned, survivor.id); err != nil {
		return err
	}
	for _, loser := range losers {
		if err := repointTask(tx, loser.id, survivor.id); err != nil {
			return err
		}
		log.Printf("[Trackers] adoption : doublon %s fusionné, %s conservé, %s supprimé (alias)", key, survivor.id, loser.id)
	}
	return nil
}

// repointTask moves everything that names loser to survivor, records loser as
// an alias of survivor, and deletes loser.
func repointTask(tx *sqlTx, loser, survivor string) error {
	// One ordinary active run per task: a second one arriving from the loser
	// is kept as a concurrent run rather than refused by the index.
	if _, err := tx.Exec(`UPDATE task_activities SET concurrent = 1
		WHERE task_id = ? AND concurrent = 0 AND status IN ('queued', 'pending', 'running')
		AND EXISTS (SELECT 1 FROM task_activities other WHERE other.task_id = ? AND other.concurrent = 0 AND other.status IN ('queued', 'pending', 'running'))`, loser, survivor); err != nil {
		return err
	}
	for _, statement := range []string{
		`UPDATE task_activities SET task_id = ? WHERE task_id = ?`,
		`UPDATE task_comments SET task_id = ? WHERE task_id = ?`,
		`UPDATE task_aliases SET task_id = ? WHERE task_id = ?`,
	} {
		if _, err := tx.Exec(statement, survivor, loser); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`INSERT INTO pinned_tasks (task_id, pinned_at) SELECT CAST(? AS TEXT), pinned_at FROM pinned_tasks WHERE task_id = ? ON CONFLICT DO NOTHING`, survivor, loser); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO batch_members (run_id, task_id, position, state)
		SELECT run_id, CAST(? AS TEXT), position, state FROM batch_members WHERE task_id = ? ON CONFLICT DO NOTHING`, survivor, loser); err != nil {
		return err
	}
	for _, statement := range []string{
		`DELETE FROM pinned_tasks WHERE task_id = ?`,
		`DELETE FROM batch_members WHERE task_id = ?`,
	} {
		if _, err := tx.Exec(statement, loser); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`INSERT INTO task_aliases (old_id, task_id) VALUES (?, ?) ON CONFLICT (old_id) DO UPDATE SET task_id = excluded.task_id`, loser, survivor); err != nil {
		return err
	}
	_, err := tx.Exec(`DELETE FROM tasks WHERE id = ?`, loser)
	return err
}

// tagJiraEpicsWithTrackers sets the tracker of the macros that are Jira epics.
// A local M-<n> macro stays its project's, and duplicate epic rows are left to
// a later step.
func tagJiraEpicsWithTrackers(tx *sqlTx, members *trackerMembers) error {
	for _, trackerID := range members.order {
		if members.provider[trackerID] != "jira" {
			continue
		}
		refs := members.refs[trackerID]
		rows, err := tx.Query(`SELECT project_id, key FROM macros WHERE tracker_id IS NULL AND project_id IN (`+placeholders(len(refs))+`)`, anySlice(refs)...)
		if err != nil {
			return fmt.Errorf("reading the macros of tracker %s: %w", trackerID, err)
		}
		type macroRef struct{ project, key string }
		var epics []macroRef
		for rows.Next() {
			var m macroRef
			if err := rows.Scan(&m.project, &m.key); err != nil {
				rows.Close()
				return err
			}
			if !isMilestoneKey(m.key) {
				epics = append(epics, m)
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		for _, epic := range epics {
			if _, err := tx.Exec(`UPDATE macros SET tracker_id = ? WHERE project_id = ? AND key = ? AND tracker_id IS NULL`, trackerID, epic.project, epic.key); err != nil {
				return err
			}
		}
	}
	return nil
}

// copyAutoSyncBookkeeping carries the background synchronisation's record of
// its passes to the trackers, the most recent pass of their projects.
func copyAutoSyncBookkeeping(tx *sqlTx, members *trackerMembers) error {
	for _, trackerID := range members.order {
		projects := members.projects[trackerID]
		args := append([]any{trackerID}, anySlice(projects)...)
		if _, err := tx.Exec(`INSERT INTO auto_sync_trackers (tracker_id, last_pass_at, last_full_sync_at)
			SELECT CAST(? AS TEXT), MAX(last_pass_at), MAX(last_full_sync_at) FROM auto_sync_projects WHERE project_id IN (`+placeholders(len(projects))+`)
			HAVING COUNT(*) > 0
			ON CONFLICT (tracker_id) DO NOTHING`, args...); err != nil {
			return fmt.Errorf("copying the auto-sync record of tracker %s: %w", trackerID, err)
		}
	}
	return nil
}

func anySlice(values []string) []any {
	out := make([]any, 0, len(values))
	for _, v := range values {
		out = append(out, v)
	}
	return out
}

func decodeStrings(raw string) []string {
	var values []string
	_ = json.Unmarshal([]byte(raw), &values)
	return values
}

func unionStrings(base, more []string) []string {
	for _, value := range more {
		if !slices.Contains(base, value) {
			base = append(base, value)
		}
	}
	return base
}
