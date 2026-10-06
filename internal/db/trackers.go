package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"tasks/internal/models"
	"tasks/internal/trackerapi"

	"github.com/google/uuid"
)

// trackerSelect lists the columns scanTracker reads, in its order.
const trackerSelect = `SELECT id, name, provider, site, scope, identity, board_id, tracker_columns, stage_columns, sprints, issue_types, auto_sync_enabled, auto_sync_interval_min, created_at,
	updated_at FROM trackers`

// ErrTrackerInUse refuses to delete a tracker a project still selects or a
// ticket still belongs to.
var ErrTrackerInUse = fmt.Errorf("ce tracker est encore utilisé par un projet ou par des tickets")

// ErrTrackerSourceInUse refuses to point a tracker holding tickets at another
// source: its tickets, read from the first one, would be left behind.
var ErrTrackerSourceInUse = errors.New("ce tracker a déjà des tickets : son fournisseur, son site et son périmètre ne changent plus")

// ErrTrackerNotInProject refuses a tracker a project does not select, named
// to create a ticket on it (#741).
var ErrTrackerNotInProject = errors.New("ce tracker n'est pas un tracker du projet")

func scanTracker(row rowScanner) (*models.Tracker, error) {
	var t models.Tracker
	var columnsJSON, stagesJSON, sprintsJSON, typesJSON string
	var autoSync int
	if err := row.Scan(
		&t.ID,
		&t.Name,
		&t.Provider,
		&t.Site,
		&t.Scope,
		&t.Identity,
		&t.BoardID,
		&columnsJSON,
		&stagesJSON,
		&sprintsJSON,
		&typesJSON,
		&autoSync,
		&t.AutoSyncIntervalMin,
		&t.CreatedAt,
		&t.UpdatedAt,
	); err != nil {
		return nil, err
	}
	t.TrackerColumns = parseTrackerColumns(columnsJSON)
	t.StageColumns = parseStageColumns(stagesJSON)
	t.Sprints = parseSprints(sprintsJSON)
	t.IssueTypes = parseIssueTypes(typesJSON)
	t.AutoSyncEnabled = autoSync == 1
	t.AutoSyncIntervalMin = models.NormalizeAutoSyncIntervalMin(t.AutoSyncIntervalMin)
	return &t, nil
}

// trackerByIDOn and trackerByIdentityOn read one tracker, nil when none.
func trackerByIDOn(q rowQuerier, id string) (*models.Tracker, error) {
	t, err := scanTracker(q.QueryRow(trackerSelect+` WHERE id = ?`, id))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return t, err
}

func trackerByIdentityOn(q rowQuerier, identity string) (*models.Tracker, error) {
	t, err := scanTracker(q.QueryRow(trackerSelect+` WHERE identity = ?`, identity))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return t, err
}

// trackerMirrorJSON encodes the board mirror the way the tracker row stores it.
func trackerMirrorJSON(t *models.Tracker) (columns, stages, sprints, types string) {
	if t.TrackerColumns == nil {
		t.TrackerColumns = []models.TrackerColumn{}
	}
	if t.StageColumns == nil {
		t.StageColumns = map[string][]string{}
	}
	if t.Sprints == nil {
		t.Sprints = []models.TrackerSprint{}
	}
	if t.IssueTypes == nil {
		t.IssueTypes = []string{}
	}
	c, _ := json.Marshal(t.TrackerColumns)
	s, _ := json.Marshal(t.StageColumns)
	sp, _ := json.Marshal(t.Sprints)
	ty, _ := json.Marshal(t.IssueTypes)
	return string(c), string(s), string(sp), string(ty)
}

// insertTrackerOn inserts a tracker, or nothing when its identity exists.
func insertTrackerOn(tx *sqlTx, t *models.Tracker) error {
	columns, stages, sprints, types := trackerMirrorJSON(t)
	_, err := tx.Exec(`INSERT INTO trackers (id, name, provider, site, scope, identity, board_id, tracker_columns, stage_columns, sprints, issue_types, auto_sync_enabled, auto_sync_interval_min,
		created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (identity) DO NOTHING`,
		t.ID,
		t.Name,
		t.Provider,
		t.Site,
		t.Scope,
		t.Identity,
		t.BoardID,
		columns,
		stages,
		sprints,
		types,
		boolInt(t.AutoSyncEnabled),
		models.NormalizeAutoSyncIntervalMin(t.AutoSyncIntervalMin),
		t.CreatedAt,
		t.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting the tracker %s: %w", t.Identity, err)
	}
	return nil
}

// updateTrackerOn rewrites every field of a tracker but its id and creation
// date.
func updateTrackerOn(tx *sqlTx, t *models.Tracker) error {
	columns, stages, sprints, types := trackerMirrorJSON(t)
	_, err := tx.Exec(`UPDATE trackers SET name = ?, provider = ?, site = ?, scope = ?, identity = ?, board_id = ?, tracker_columns = ?, stage_columns = ?, sprints = ?, issue_types = ?,
		auto_sync_enabled = ?, auto_sync_interval_min = ?, updated_at = ?
		WHERE id = ?`,
		t.Name,
		t.Provider,
		t.Site,
		t.Scope,
		t.Identity,
		t.BoardID,
		columns,
		stages,
		sprints,
		types,
		boolInt(t.AutoSyncEnabled),
		models.NormalizeAutoSyncIntervalMin(t.AutoSyncIntervalMin),
		t.UpdatedAt,
		t.ID,
	)
	return err
}

// resolvedTrackerSite is the address a tracker reaches: its own, else the
// deployment's for its provider, as trackerCredentials resolves it.
func resolvedTrackerSite(provider, site string, settings *models.Settings) string {
	if site = strings.TrimSpace(site); site != "" {
		return site
	}
	switch provider {
	case "jira":
		if settings != nil {
			return settings.JiraUrl
		}
	case "github":
		api := ""
		if settings != nil {
			api = strings.TrimSpace(settings.GithubApiUrl)
		}
		return orDefault(api, defaultGithubAPI)
	case "gitlab":
		api := ""
		if settings != nil {
			api = strings.TrimSpace(settings.GitlabUrl)
		}
		return orDefault(api, trackerapi.DefaultGitlabURL)
	}
	return ""
}

// trackerIdentityFor is the identity of a tracker, its site resolved against
// the deployment's: a tracker with no site of its own and one naming the
// deployment's site are the same source.
func trackerIdentityFor(t *models.Tracker, settings *models.Settings) string {
	return models.TrackerIdentity(t.Provider, resolvedTrackerSite(t.Provider, t.Site, settings), t.Scope)
}

// legacyTrackerOf is the tracker a project's own tracker fields name, the way
// Registry.ForProject reads them: the provider by trackerKindOf, the site the
// project overrides (empty for the deployment's), and the scope: the Jira key,
// the GitHub owner/repo or the GitLab path, the settings' when the project
// names none, and the project id for a local board, which is never shared.
// The board mirror and the auto-sync settings come along.
func legacyTrackerOf(p *models.Project, settings *models.Settings) models.Tracker {
	t := models.Tracker{
		Name:                p.Name,
		Provider:            trackerKindOf(p),
		BoardID:             strings.TrimSpace(p.BoardID),
		TrackerColumns:      p.TrackerColumns,
		StageColumns:        p.StageColumns,
		Sprints:             p.Sprints,
		IssueTypes:          p.IssueTypes,
		AutoSyncEnabled:     p.AutoSyncEnabled,
		AutoSyncIntervalMin: models.NormalizeAutoSyncIntervalMin(p.AutoSyncIntervalMin),
	}
	switch t.Provider {
	case "jira":
		t.Site, t.Scope = strings.TrimSpace(p.TrackerUrl), jiraProjectKeyFor(p)
	case "github":
		t.Site, t.Scope = strings.TrimSpace(p.GithubApiUrl), models.CleanGithubRepo(p.GithubRepo)
		if t.Scope == "" && settings != nil {
			t.Scope = models.CleanGithubRepo(settings.GithubRepo)
		}
	case "gitlab":
		t.Site, t.Scope = strings.TrimSpace(p.GitlabUrl), strings.Trim(strings.TrimSpace(p.GitlabProject), "/")
		if t.Scope == "" && settings != nil {
			t.Scope = strings.Trim(strings.TrimSpace(settings.GitlabProject), "/")
		}
	default:
		t.Scope = p.ID
	}
	t.Identity = trackerIdentityFor(&t, settings)
	return t
}

// legacyFieldsMatchTracker says whether the project's own tracker fields name
// the tracker t, without the settings: the read-through below leaves the
// fields of a project that already names its tracker exactly as stored.
func legacyFieldsMatchTracker(p *models.Project, t *models.Tracker) bool {
	if trackerKindOf(p) != t.Provider {
		return false
	}
	scope, site := "", ""
	switch t.Provider {
	case "jira":
		scope, site = jiraProjectKeyFor(p), p.TrackerUrl
	case "github":
		scope, site = models.CleanGithubRepo(p.GithubRepo), p.GithubApiUrl
	case "gitlab":
		scope, site = p.GitlabProject, p.GitlabUrl
	default:
		return true
	}
	if strings.TrimSpace(scope) != "" && models.TrackerScope(t.Provider, scope) != models.TrackerScope(t.Provider, t.Scope) {
		return false
	}
	return models.TrackerAddress(site) == models.TrackerAddress(t.Site)
}

// fillProjectTrackersUnsafe sets the trackers a project selects from, its
// default, and reads its tracker fields through from that default (#741):
// the board mirror and the auto-sync settings always, the provider, site and
// scope when the tracker no longer matches what the project stored. Every
// reader of those fields keeps working until the settings screen moves to
// the trackers.
func (d *DB) fillProjectTrackersUnsafe(p *models.Project) error {
	trackers, err := d.projectTrackersUnsafe(p.ID)
	if err != nil {
		return err
	}
	p.Trackers = []models.ProjectTracker{}
	for _, t := range trackers {
		p.Trackers = append(p.Trackers, models.ProjectTracker{TrackerID: t.ID, Identity: t.Identity})
	}
	def := defaultTrackerAmong(trackers, p.DefaultTrackerID)
	if def == nil {
		return nil
	}
	p.DefaultTrackerID = def.ID
	p.BoardID, p.TrackerColumns, p.StageColumns, p.Sprints, p.IssueTypes = def.BoardID, def.TrackerColumns, def.StageColumns, def.Sprints, def.IssueTypes
	p.AutoSyncEnabled, p.AutoSyncIntervalMin = def.AutoSyncEnabled, def.AutoSyncIntervalMin
	if legacyFieldsMatchTracker(p, def) {
		return nil
	}
	p.IssueTracker = def.Provider
	switch def.Provider {
	case "jira":
		p.JiraProject, p.TrackerUrl = def.Scope, def.Site
	case "github":
		p.GithubRepo, p.GithubApiUrl = def.Scope, def.Site
	case "gitlab":
		p.GitlabProject, p.GitlabUrl = def.Scope, def.Site
	}
	return nil
}

// defaultTrackerAmong is the tracker named by id among a project's trackers,
// or the first one when the id names none of them.
func defaultTrackerAmong(trackers []*models.Tracker, id string) *models.Tracker {
	for _, t := range trackers {
		if t.ID == id {
			return t
		}
	}
	if len(trackers) > 0 {
		return trackers[0]
	}
	return nil
}

// projectTrackersUnsafe lists the trackers a project selects from, by
// position. The project may be named by id or slug.
func (d *DB) projectTrackersUnsafe(projectID string) ([]*models.Tracker, error) {
	rows, err := d.conn.Query(`SELECT t.id, t.name, t.provider, t.site, t.scope, t.identity, t.board_id, t.tracker_columns, t.stage_columns, t.sprints, t.issue_types, t.auto_sync_enabled,
		t.auto_sync_interval_min, t.created_at, t.updated_at
		FROM project_trackers pt JOIN trackers t ON t.id = pt.tracker_id
		WHERE pt.project_id = ? OR pt.project_id IN (SELECT id FROM projects WHERE slug = ?)
		ORDER BY pt.position, t.created_at`, projectID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	trackers := []*models.Tracker{}
	for rows.Next() {
		t, err := scanTracker(rows)
		if err != nil {
			return nil, err
		}
		trackers = append(trackers, t)
	}
	return trackers, rows.Err()
}

// projectTrackerNamedUnsafe finds, among a project's trackers, the one named by
// id or identity; ErrTrackerNotInProject when none is.
func (d *DB) projectTrackerNamedUnsafe(projectID, ref string) (*models.Tracker, error) {
	trackers, err := d.projectTrackersUnsafe(projectID)
	if err != nil {
		return nil, err
	}
	ref = strings.TrimSpace(ref)
	for _, t := range trackers {
		if t.ID == ref || t.Identity == ref {
			return t, nil
		}
	}
	return nil, ErrTrackerNotInProject
}

// ProjectTrackerNamed finds, among a project's trackers, the one named by id
// or identity; ErrTrackerNotInProject when none is.
func (d *DB) ProjectTrackerNamed(projectID, ref string) (*models.Tracker, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.projectTrackerNamedUnsafe(projectID, ref)
}

// TrackerProjectIDs lists the ids of the projects selecting a tracker, oldest
// first.
func (d *DB) TrackerProjectIDs(trackerID string) []string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	ids, _ := d.trackerLinkedProjectsUnsafe(trackerID)
	return ids
}

// ProjectTrackers lists the trackers a project selects its tickets from, in
// order.
func (d *DB) ProjectTrackers(projectID string) ([]*models.Tracker, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.projectTrackersUnsafe(projectID)
}

// defaultTrackerOfUnsafe is the tracker a project's new tickets go to and its
// tracker fields read through from, nil when it has none.
func (d *DB) defaultTrackerOfUnsafe(projectID string) (*models.Tracker, error) {
	trackers, err := d.projectTrackersUnsafe(projectID)
	if err != nil {
		return nil, err
	}
	var defaultID string
	_ = d.conn.QueryRow(`SELECT default_tracker_id FROM projects WHERE id = ? OR slug = ?`, projectID, projectID).Scan(&defaultID)
	return defaultTrackerAmong(trackers, defaultID), nil
}

// ProjectDefaultTracker is the tracker a project's new tickets go to and its
// tracker settings read through from, nil when it has none (#741).
func (d *DB) ProjectDefaultTracker(projectID string) *models.Tracker {
	d.mu.RLock()
	defer d.mu.RUnlock()
	t, _ := d.defaultTrackerOfUnsafe(projectID)
	return t
}

// trackerLinkedProjectsUnsafe lists the ids of the projects selecting a
// tracker, oldest link first: the first one is the project the D1 shim of
// #741 writes on its tickets.
func (d *DB) trackerLinkedProjectsUnsafe(trackerID string) ([]string, error) {
	rows, err := d.conn.Query(`SELECT pt.project_id FROM project_trackers pt JOIN projects p ON p.id = pt.project_id WHERE pt.tracker_id = ? ORDER BY p.created_at, p.id`, trackerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// trackerUsageOn counts the tickets and the epics a tracker holds.
func trackerUsageOn(q rowQuerier, trackerID string) (int, error) {
	var tasks, macros int
	if err := q.QueryRow(`SELECT COUNT(*) FROM tasks WHERE tracker_id = ?`, trackerID).Scan(&tasks); err != nil {
		return 0, err
	}
	if err := q.QueryRow(`SELECT COUNT(DISTINCT key) FROM macros WHERE tracker_id = ?`, trackerID).Scan(&macros); err != nil {
		return 0, err
	}
	return tasks + macros, nil
}

// trackerWithUsageOn reads one tracker with its TicketCount, nil when there is
// none.
func trackerWithUsageOn(q rowQuerier, id string) (*models.Tracker, error) {
	t, err := trackerByIDOn(q, id)
	if err != nil || t == nil {
		return t, err
	}
	if t.TicketCount, err = trackerUsageOn(q, t.ID); err != nil {
		return nil, err
	}
	return t, nil
}

// resetTrackerSyncWindowOn forgets when a tracker was last read whole, so its
// next background pass reads it whole: a tracker pointed at another source
// has nothing of it yet.
func resetTrackerSyncWindowOn(tx *sqlTx, trackerID string) error {
	_, err := tx.Exec(`UPDATE auto_sync_trackers SET last_full_sync_at = NULL WHERE tracker_id = ?`, trackerID)
	return err
}

// trackerUsagesUnsafe counts the tickets and the epics of every tracker that
// holds some, by tracker id.
func (d *DB) trackerUsagesUnsafe() (map[string]int, error) {
	usages := map[string]int{}
	for _, query := range []string{
		`SELECT tracker_id, COUNT(*) FROM tasks WHERE tracker_id IS NOT NULL GROUP BY tracker_id`,
		`SELECT tracker_id, COUNT(DISTINCT key) FROM macros WHERE tracker_id IS NOT NULL GROUP BY tracker_id`,
	} {
		rows, err := d.conn.Query(query)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id string
			var n int
			if err := rows.Scan(&id, &n); err != nil {
				rows.Close()
				return nil, err
			}
			usages[id] += n
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	return usages, nil
}

// GetTrackers lists every tracker, with how many tickets each holds.
func (d *DB) GetTrackers() ([]*models.Tracker, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	rows, err := d.conn.Query(trackerSelect + ` ORDER BY provider, name, scope`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	trackers := []*models.Tracker{}
	for rows.Next() {
		t, err := scanTracker(rows)
		if err != nil {
			return nil, err
		}
		trackers = append(trackers, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	usages, err := d.trackerUsagesUnsafe()
	if err != nil {
		return nil, err
	}
	for _, t := range trackers {
		t.TicketCount = usages[t.ID]
	}
	return trackers, nil
}

// GetTrackerByID reads one tracker, with how many tickets it holds, nil when
// there is none.
func (d *DB) GetTrackerByID(id string) (*models.Tracker, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return trackerWithUsageOn(d.conn, id)
}

func (d *DB) getTrackerByIdentityUnsafe(identity string) (*models.Tracker, error) {
	return trackerByIdentityOn(d.conn, identity)
}

// normalizeTracker checks a tracker written through CreateTrackerAs or
// UpdateTrackerAs and recomputes its identity.
func normalizeTracker(t *models.Tracker, settings *models.Settings) error {
	t.Provider = strings.ToLower(strings.TrimSpace(t.Provider))
	t.Name, t.Site, t.Scope = strings.TrimSpace(t.Name), strings.TrimSpace(t.Site), strings.TrimSpace(t.Scope)
	switch t.Provider {
	case "jira":
		t.Scope = strings.ToUpper(t.Scope)
	case "github":
		t.Scope = models.CleanGithubRepo(t.Scope)
	case "gitlab":
		t.Scope = strings.Trim(t.Scope, "/")
	case "local":
	default:
		return fmt.Errorf("tracker %q inconnu", t.Provider)
	}
	if t.Scope == "" {
		return fmt.Errorf("le tracker doit nommer son projet, son dépôt ou son espace")
	}
	if t.Name == "" {
		t.Name = t.Scope
	}
	t.AutoSyncIntervalMin = models.NormalizeAutoSyncIntervalMin(t.AutoSyncIntervalMin)
	t.Identity = trackerIdentityFor(t, settings)
	return nil
}

// CreateTrackerAs records a tracker. One that names a source already recorded
// is refused.
func (d *DB) CreateTrackerAs(userID string, t models.Tracker) (*models.Tracker, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	settings, _ := d.getSettingsUnsafe()
	if err := normalizeTracker(&t, settings); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	t.ID, t.CreatedAt, t.UpdatedAt = uuid.New().String(), now, now
	err := d.conn.WithTx(func(tx *sqlTx) error {
		if existing, err := trackerByIdentityOn(tx, t.Identity); err != nil {
			return err
		} else if existing != nil {
			return fmt.Errorf("ce tracker existe déjà : %s", existing.Name)
		}
		return insertTrackerOn(tx, &t)
	})
	d.trackerCache.clear()
	if err != nil {
		return nil, err
	}
	return trackerWithUsageOn(d.conn, t.ID)
}

// UpdateTrackerAs rewrites a tracker. Its identity is recomputed, and may not
// become that of another tracker. A tracker holding tickets keeps its source,
// ErrTrackerSourceInUse otherwise; one holding none may change it, and its
// next background pass then reads the new source whole.
func (d *DB) UpdateTrackerAs(userID string, t models.Tracker) (*models.Tracker, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	settings, _ := d.getSettingsUnsafe()
	if err := normalizeTracker(&t, settings); err != nil {
		return nil, err
	}
	t.UpdatedAt = time.Now().UTC()
	err := d.conn.WithTx(func(tx *sqlTx) error {
		current, err := trackerByIDOn(tx, t.ID)
		if err != nil {
			return err
		}
		if current == nil {
			return fmt.Errorf("tracker non trouvé")
		}
		if other, err := trackerByIdentityOn(tx, t.Identity); err != nil {
			return err
		} else if other != nil && other.ID != t.ID {
			return fmt.Errorf("ce tracker existe déjà : %s", other.Name)
		}
		moved := trackerSourceChanged(current, &t)
		if moved {
			used, err := trackerUsageOn(tx, t.ID)
			if err != nil {
				return err
			}
			if used > 0 {
				return ErrTrackerSourceInUse
			}
		}
		if err := updateTrackerOn(tx, &t); err != nil {
			return err
		}
		if moved {
			return resetTrackerSyncWindowOn(tx, t.ID)
		}
		return nil
	})
	d.trackerCache.clear()
	if err != nil {
		return nil, err
	}
	return trackerWithUsageOn(d.conn, t.ID)
}

// trackerSourceChanged says whether a rewrite points a tracker at another
// source: another provider, site or scope, which another identity confirms. A
// site spelled differently that resolves to the same address is the same
// source, and so is a tracker left as it was while the deployment's own site
// changed under it.
func trackerSourceChanged(current, next *models.Tracker) bool {
	if current.Identity == next.Identity {
		return false
	}
	return current.Provider != next.Provider ||
		models.TrackerScope(current.Provider, current.Scope) != models.TrackerScope(next.Provider, next.Scope) ||
		models.TrackerAddress(current.Site) != models.TrackerAddress(next.Site)
}

// DeleteTrackerAs deletes a tracker no project selects and no ticket belongs
// to.
func (d *DB) DeleteTrackerAs(userID string, id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	err := d.conn.WithTx(func(tx *sqlTx) error {
		var links, tasks int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM project_trackers WHERE tracker_id = ?`, id).Scan(&links); err != nil {
			return err
		}
		if err := tx.QueryRow(`SELECT COUNT(*) FROM tasks WHERE tracker_id = ?`, id).Scan(&tasks); err != nil {
			return err
		}
		if links > 0 || tasks > 0 {
			return ErrTrackerInUse
		}
		result, err := tx.Exec(`DELETE FROM trackers WHERE id = ?`, id)
		if err != nil {
			return err
		}
		if n, _ := result.RowsAffected(); n == 0 {
			return fmt.Errorf("tracker non trouvé")
		}
		_, err = tx.Exec(`DELETE FROM auto_sync_trackers WHERE tracker_id = ?`, id)
		return err
	})
	d.trackerCache.clear()
	return err
}

// UpdateTrackerMirror changes the board mirror of one tracker, its row locked
// while change runs, so two synchronisations do not lose each other's write.
func (d *DB) UpdateTrackerMirror(trackerID string, change func(t *models.Tracker)) (*models.Tracker, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	err := d.conn.WithTx(func(tx *sqlTx) error {
		t, err := scanTracker(tx.QueryRow(trackerSelect+` WHERE id = ?`+d.forUpdate(), trackerID))
		if err == sql.ErrNoRows {
			return fmt.Errorf("tracker non trouvé")
		}
		if err != nil {
			return err
		}
		change(t)
		t.BoardID = strings.TrimSpace(t.BoardID)
		t.UpdatedAt = time.Now().UTC()
		columns, stages, sprints, types := trackerMirrorJSON(t)
		_, err = tx.Exec(`UPDATE trackers SET board_id = ?, tracker_columns = ?, stage_columns = ?, sprints = ?, issue_types = ?, updated_at = ? WHERE id = ?`,
			t.BoardID, columns, stages, sprints, types, t.UpdatedAt, t.ID)
		return err
	})
	d.trackerCache.clear()
	if err != nil {
		return nil, err
	}
	return trackerByIDOn(d.conn, trackerID)
}

// trackerFieldsTouched says which tracker fields a project write carries, so
// that only those are written through to its tracker, and whether the write
// may only join a tracker already recorded (a member's, ADR 0050, D11).
type trackerFieldsTouched struct {
	boardID, columns, stages, sprints, issueTypes, autoSync bool
	joinOnly                                                bool
}

func touchedByUpdate(req models.UpdateProjectRequest) trackerFieldsTouched {
	return trackerFieldsTouched{
		boardID:    req.BoardID != nil,
		columns:    req.TrackerColumns != nil,
		stages:     req.StageColumns != nil,
		sprints:    req.Sprints != nil,
		issueTypes: req.IssueTypes != nil,
		autoSync:   req.AutoSyncEnabled != nil || req.AutoSyncIntervalMin != nil,
		joinOnly:   req.JoinTrackerOnly,
	}
}

// ensureProjectTrackerUnsafe makes the project's default tracker the one its
// own tracker fields name: the legacy single-tracker fields write through to
// the default tracker (#741), the project's other trackers untouched.
//
// A project whose default tracker only it selects keeps that tracker, renamed
// in place when its fields now name another source nobody recorded yet, so
// its tickets stay with it. Otherwise the tracker of that identity is found or
// created and takes the default's place. The board mirror and auto-sync fields
// the write carries then land on the tracker; a tracker the project just
// joined keeps its own.
//
// A write that may only join (touched.joinOnly, a member's) never renames nor
// creates a tracker but its project's local board: naming a source nobody
// recorded is ErrUnknownTracker. Its project keeps its default tracker when
// its fields still name it, whatever the deployment's site became since.
func (d *DB) ensureProjectTrackerUnsafe(tx *sqlTx, p *models.Project, settings *models.Settings, touched trackerFieldsTouched) error {
	wanted := legacyTrackerOf(p, settings)
	var currentID string
	currentPosition := 0
	if err := tx.QueryRow(`SELECT pt.tracker_id, pt.position FROM project_trackers pt JOIN projects p ON p.id = pt.project_id
		WHERE pt.project_id = ?
		ORDER BY CASE WHEN pt.tracker_id = p.default_tracker_id THEN 0 ELSE 1 END, pt.position LIMIT 1`, p.ID).Scan(&currentID, &currentPosition); err != nil && err != sql.ErrNoRows {
		return err
	}
	var current *models.Tracker
	if currentID != "" {
		var err error
		if current, err = trackerByIDOn(tx, currentID); err != nil {
			return err
		}
	}
	target, err := trackerByIdentityOn(tx, wanted.Identity)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	fresh := false
	switch {
	case target != nil:
	case touched.joinOnly && current != nil && legacyFieldsMatchTracker(p, current):
		target = current
	case touched.joinOnly && wanted.Provider != "local":
		return fmt.Errorf("%w : %s", ErrUnknownTracker, wanted.Identity)
	case !touched.joinOnly && current != nil && current.Identity != wanted.Identity && d.trackerExclusiveTo(tx, current.ID, p.ID):
		current.Provider, current.Site, current.Scope, current.Identity, current.UpdatedAt = wanted.Provider, wanted.Site, wanted.Scope, wanted.Identity, now
		if err := updateTrackerOn(tx, current); err != nil {
			return err
		}
		// The tracker now reads another source: its next pass reads it whole.
		if err := resetTrackerSyncWindowOn(tx, current.ID); err != nil {
			return err
		}
		target = current
	default:
		wanted.ID, wanted.CreatedAt, wanted.UpdatedAt = uuid.New().String(), now, now
		if err := insertTrackerOn(tx, &wanted); err != nil {
			return err
		}
		if target, err = trackerByIdentityOn(tx, wanted.Identity); err != nil || target == nil {
			return fmt.Errorf("reading back the tracker %s: %v", wanted.Identity, err)
		}
		fresh = target.ID == wanted.ID
	}
	if current == nil || current.ID != target.ID {
		if current != nil {
			if _, err := tx.Exec(`DELETE FROM project_trackers WHERE project_id = ? AND tracker_id IN (?, ?)`, p.ID, current.ID, target.ID); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(
			`INSERT INTO project_trackers (project_id, tracker_id, position) VALUES (?, ?, ?) ON CONFLICT (project_id, tracker_id) DO NOTHING`,
			p.ID,
			target.ID,
			currentPosition,
		); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`UPDATE projects SET default_tracker_id = ? WHERE id = ?`, target.ID, p.ID); err != nil {
		return err
	}
	if fresh || (current != nil && current.ID != target.ID) {
		return nil
	}
	changed := false
	if touched.boardID {
		target.BoardID, changed = wanted.BoardID, true
	}
	if touched.columns {
		target.TrackerColumns, changed = wanted.TrackerColumns, true
	}
	if touched.stages {
		target.StageColumns, changed = wanted.StageColumns, true
	}
	if touched.sprints {
		target.Sprints, changed = wanted.Sprints, true
	}
	if touched.issueTypes {
		target.IssueTypes, changed = wanted.IssueTypes, true
	}
	if touched.autoSync {
		target.AutoSyncEnabled, target.AutoSyncIntervalMin, changed = wanted.AutoSyncEnabled, wanted.AutoSyncIntervalMin, true
	}
	if !changed {
		return nil
	}
	target.UpdatedAt = now
	return updateTrackerOn(tx, target)
}

// ErrUnknownTracker refuses a project naming a tracker nobody recorded.
var ErrUnknownTracker = errors.New("tracker inconnu")

// ErrForeignLocalTracker refuses a project selecting another project's local
// board.
var ErrForeignLocalTracker = errors.New("le tableau local d'un autre projet ne peut pas être sélectionné")

// ErrInvalidProjectLabel refuses a project label its trackers cannot carry: a
// label with a space, when one of the project's trackers is Jira. Adding a
// ticket to the project would write it on Jira, which refuses it.
var ErrInvalidProjectLabel = errors.New("le label d'un projet sur Jira ne peut pas contenir d'espace")

// applyProjectSelectionUnsafe writes what a project selects its tickets with
// (#741): its trackers, by id or identity, in order, when trackers is not nil;
// its label when label is not nil; its default tracker, which must be one of
// its trackers and falls back to the first remaining one otherwise. A local
// tracker belongs to its own project and is never accepted from another.
func (d *DB) applyProjectSelectionUnsafe(tx *sqlTx, p *models.Project, trackers *[]models.ProjectTracker, label *string, defaultID *string) error {
	if label != nil {
		p.Label = strings.TrimSpace(*label)
		if _, err := tx.Exec(`UPDATE projects SET label = ? WHERE id = ?`, p.Label, p.ID); err != nil {
			return err
		}
	}
	if trackers != nil {
		resolved := make([]models.ProjectTracker, 0, len(*trackers))
		for _, entry := range *trackers {
			var t *models.Tracker
			var err error
			if id := strings.TrimSpace(entry.TrackerID); id != "" {
				t, err = trackerByIDOn(tx, id)
			}
			if err == nil && t == nil && strings.TrimSpace(entry.Identity) != "" {
				t, err = trackerByIdentityOn(tx, strings.TrimSpace(entry.Identity))
			}
			if err != nil {
				return err
			}
			if t == nil {
				return fmt.Errorf("%w : %s%s", ErrUnknownTracker, entry.TrackerID, entry.Identity)
			}
			if t.Provider == "local" && t.Scope != p.ID {
				return ErrForeignLocalTracker
			}
			resolved = append(resolved, models.ProjectTracker{TrackerID: t.ID, Identity: t.Identity})
		}
		resolved = models.NormalizeProjectTrackers(resolved)
		if _, err := tx.Exec(`DELETE FROM project_trackers WHERE project_id = ?`, p.ID); err != nil {
			return err
		}
		for position, entry := range resolved {
			if _, err := tx.Exec(`INSERT INTO project_trackers (project_id, tracker_id, position) VALUES (?, ?, ?)`, p.ID, entry.TrackerID, position); err != nil {
				return err
			}
		}
	}
	if (label != nil || trackers != nil) && labelHasSpace(p.Label) {
		var jira int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM project_trackers pt JOIN trackers t ON t.id = pt.tracker_id WHERE pt.project_id = ? AND t.provider = 'jira'`, p.ID).Scan(&jira); err != nil {
			return err
		}
		if jira > 0 {
			return fmt.Errorf("%w : « %s »", ErrInvalidProjectLabel, p.Label)
		}
	}
	if trackers == nil && defaultID == nil {
		return nil
	}
	wanted := ""
	if defaultID != nil {
		wanted = strings.TrimSpace(*defaultID)
	}
	if wanted == "" {
		_ = tx.QueryRow(`SELECT default_tracker_id FROM projects WHERE id = ?`, p.ID).Scan(&wanted)
	}
	var chosen, first string
	rows, err := tx.Query(`SELECT pt.tracker_id, t.identity FROM project_trackers pt JOIN trackers t ON t.id = pt.tracker_id WHERE pt.project_id = ? ORDER BY pt.position`, p.ID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id, identity string
		if err := rows.Scan(&id, &identity); err != nil {
			rows.Close()
			return err
		}
		if first == "" {
			first = id
		}
		if wanted != "" && (id == wanted || identity == wanted) {
			chosen = id
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if chosen == "" {
		chosen = first
	}
	p.DefaultTrackerID = chosen
	_, err = tx.Exec(`UPDATE projects SET default_tracker_id = ? WHERE id = ?`, chosen, p.ID)
	return err
}

// trackerExclusiveTo says whether projectID is the only project selecting the
// tracker.
func (d *DB) trackerExclusiveTo(tx *sqlTx, trackerID, projectID string) bool {
	var others int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM project_trackers WHERE tracker_id = ? AND project_id <> ?`, trackerID, projectID).Scan(&others); err != nil {
		return false
	}
	return others == 0
}

// releaseProjectTrackersUnsafe unlinks a project about to be deleted from its
// trackers. The tickets of its local tracker move to the default project and
// its tracker, then the local tracker goes; the tickets of a tracker other
// projects select take the first of them as their project (the D1 shim of
// #741), and the others stay where they are.
func (d *DB) releaseProjectTrackersUnsafe(tx *sqlTx, p *models.Project, defaultProjectID string) error {
	refs := []any{p.ID, p.Slug}
	rows, err := tx.Query(`SELECT t.id, t.provider FROM project_trackers pt JOIN trackers t ON t.id = pt.tracker_id WHERE pt.project_id = ?`, p.ID)
	if err != nil {
		return err
	}
	type linked struct{ id, provider string }
	var trackers []linked
	for rows.Next() {
		var l linked
		if err := rows.Scan(&l.id, &l.provider); err != nil {
			rows.Close()
			return err
		}
		trackers = append(trackers, l)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM project_trackers WHERE project_id = ?`, p.ID); err != nil {
		return err
	}
	var defaultTrackerID string
	_ = tx.QueryRow(`SELECT pt.tracker_id FROM project_trackers pt JOIN trackers t ON t.id = pt.tracker_id
		WHERE pt.project_id = ? ORDER BY CASE WHEN t.provider = 'local' THEN 0 ELSE 1 END, pt.position LIMIT 1`, defaultProjectID).Scan(&defaultTrackerID)
	var defaultTracker any
	movedTo := defaultProjectID
	if defaultTrackerID != "" {
		defaultTracker, movedTo = defaultTrackerID, trackerSentinel(defaultTrackerID)
	}
	for _, t := range trackers {
		if t.provider == "local" {
			if _, err := tx.Exec(`UPDATE tasks SET project_id = ?, tracker_id = ? WHERE tracker_id = ?`, movedTo, defaultTracker, t.id); err != nil {
				return err
			}
			if _, err := tx.Exec(`DELETE FROM trackers WHERE id = ?`, t.id); err != nil {
				return err
			}
			continue
		}
		var next string
		_ = tx.QueryRow(`SELECT pt.project_id FROM project_trackers pt JOIN projects p ON p.id = pt.project_id WHERE pt.tracker_id = ? ORDER BY p.created_at, p.id LIMIT 1`, t.id).Scan(&next)
		if next == "" {
			continue
		}
		if _, err := tx.Exec(`UPDATE tasks SET project_id = ? WHERE tracker_id = ? AND project_id IN (?, ?)`, append([]any{next, t.id}, refs...)...); err != nil {
			return err
		}
	}
	// A ticket that names no tracker yet still moves with the project, as it
	// did before trackers existed.
	_, err = tx.Exec(`UPDATE tasks SET project_id = ? WHERE tracker_id IS NULL AND project_id IN (?, ?)`, append([]any{defaultProjectID}, refs...)...)
	return err
}
