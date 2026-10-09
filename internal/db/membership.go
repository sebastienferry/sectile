package db

import (
	"fmt"
	"strings"
	"time"

	"tasks/internal/models"
)

// trackerSentinelPrefix starts the project_id a ticket row written since #741
// carries: "tracker:" and its tracker id. The column is no longer read; the
// sentinel only keeps its UNIQUE(project_id, key) constraint true for two
// trackers holding the same key.
const trackerSentinelPrefix = "tracker:"

// trackerSentinel is the project_id a ticket of the tracker is written with.
func trackerSentinel(trackerID string) string {
	return trackerSentinelPrefix + trackerID
}

// memberProject is what deciding a ticket's projects needs of a project: the
// trackers it selects and its label.
type memberProject struct {
	ID        string
	Slug      string
	Name      string
	Label     string
	CreatedAt time.Time
	Trackers  []string
	// StageColumns is the project's own stage mapping per tracker it selects,
	// by tracker id; a tracker it maps no stage of is absent (#741).
	StageColumns map[string]map[string][]string
}

// membershipIndex holds every project's trackers and label, oldest project
// first, so the projects of a ticket are found without a query per ticket.
type membershipIndex struct {
	projects  []memberProject
	byRef     map[string]int
	byTracker map[string][]int
}

// project finds a project by id or slug.
func (m *membershipIndex) project(ref string) (memberProject, bool) {
	if m == nil {
		return memberProject{}, false
	}
	i, ok := m.byRef[strings.TrimSpace(ref)]
	if !ok {
		return memberProject{}, false
	}
	return m.projects[i], true
}

// labelCarried says whether labels hold label as a whole token, A-Z folded,
// the Go form of viewLabelScope. Only label, a project's, is trimmed, as its
// save trims it: a ticket label is compared as stored, as the SQL matches its
// JSON token, so " backend" does not carry "backend".
func labelCarried(labels []string, label string) bool {
	wanted := asciiLower(strings.TrimSpace(label))
	for _, l := range labels {
		if asciiLower(l) == wanted {
			return true
		}
	}
	return false
}

// selects says whether the project shows a ticket of the tracker carrying the
// labels: it selects the tracker, and it has no label or the ticket carries it.
func (p memberProject) selects(trackerID string, labels []string) bool {
	if trackerID == "" {
		return false
	}
	linked := false
	for _, id := range p.Trackers {
		if id == trackerID {
			linked = true
			break
		}
	}
	if !linked {
		return false
	}
	return strings.TrimSpace(p.Label) == "" || labelCarried(labels, p.Label)
}

// members lists the projects showing a ticket of the tracker carrying the
// labels, oldest first.
func (m *membershipIndex) members(trackerID string, labels []string) []memberProject {
	if m == nil {
		return nil
	}
	var result []memberProject
	for _, i := range m.byTracker[trackerID] {
		if m.projects[i].selects(trackerID, labels) {
			result = append(result, m.projects[i])
		}
	}
	return result
}

// loadMembershipIndexUnsafe reads every project's trackers and label.
func (d *DB) loadMembershipIndexUnsafe() (*membershipIndex, error) {
	rows, err := d.conn.Query(`SELECT id, slug, name, label, created_at FROM projects ORDER BY created_at, id`)
	if err != nil {
		return nil, err
	}
	index := &membershipIndex{byRef: map[string]int{}, byTracker: map[string][]int{}}
	for rows.Next() {
		var p memberProject
		if err := rows.Scan(&p.ID, &p.Slug, &p.Name, &p.Label, &p.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		index.byRef[p.ID] = len(index.projects)
		index.projects = append(index.projects, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i, p := range index.projects {
		if p.Slug != "" {
			if _, taken := index.byRef[p.Slug]; !taken {
				index.byRef[p.Slug] = i
			}
		}
	}
	links, err := d.conn.Query(`SELECT project_id, tracker_id, stage_columns FROM project_trackers ORDER BY position`)
	if err != nil {
		return nil, err
	}
	defer links.Close()
	for links.Next() {
		var projectID, trackerID, stagesJSON string
		if err := links.Scan(&projectID, &trackerID, &stagesJSON); err != nil {
			return nil, err
		}
		i, ok := index.byRef[projectID]
		if !ok {
			continue
		}
		index.projects[i].Trackers = append(index.projects[i].Trackers, trackerID)
		if stages := parseStageColumns(stagesJSON); len(stages) > 0 {
			if index.projects[i].StageColumns == nil {
				index.projects[i].StageColumns = map[string]map[string][]string{}
			}
			index.projects[i].StageColumns[trackerID] = stages
		}
	}
	if err := links.Err(); err != nil {
		return nil, err
	}
	// byTracker follows the projects' age, not the links' positions.
	for i, p := range index.projects {
		for _, trackerID := range p.Trackers {
			index.byTracker[trackerID] = append(index.byTracker[trackerID], i)
		}
	}
	return index, nil
}

// membershipUnsafe is the membership index, through the tracker cache: a
// project or tracker write clears it.
func (d *DB) membershipUnsafe() *membershipIndex {
	if index, _, ok := d.trackerCache.getMembership(); ok {
		return index
	}
	_, gen, _ := d.trackerCache.getMembership()
	index, err := d.loadMembershipIndexUnsafe()
	if err != nil {
		return nil
	}
	d.trackerCache.putMembership(index, gen)
	return index
}

// projectMembership is the SQL form of the projects' rule over the tasks
// table: the ticket's tracker is one the project selects, and it carries the
// project's label as a whole token, A-Z folded, unless the project has none.
// lowered is the engine's lowerASCII of the labels column.
func projectMembership(p memberProject, lowered string) (string, []interface{}) {
	return projectMembershipOn(p, "", lowered)
}

// projectMembershipOn is projectMembership over the tasks table under an
// alias ("t." for "FROM tasks t"); lowered folds that alias's labels column.
func projectMembershipOn(p memberProject, alias, lowered string) (string, []interface{}) {
	if len(p.Trackers) == 0 {
		return "1 = 0", nil
	}
	args := make([]interface{}, 0, len(p.Trackers)+1)
	for _, id := range p.Trackers {
		args = append(args, id)
	}
	cond := fmt.Sprintf("%stracker_id IN (%s)", alias, placeholders(len(p.Trackers)))
	if label := strings.TrimSpace(p.Label); label != "" {
		labelCond, labelArgs := viewLabelScope([]string{label}, lowered)
		cond += " AND " + labelCond
		args = append(args, labelArgs...)
	}
	return "(" + cond + ")", args
}

// membershipScopeUnsafe is the OR of the projects' memberships, the projects
// named by id or slug. A ticket shown by two of them is one row, so it is
// listed once. No project known selects nothing.
func (d *DB) membershipScopeUnsafe(projectRefs []string) (string, []interface{}) {
	return d.membershipScopeOnUnsafe(projectRefs, "")
}

// membershipScopeOnUnsafe is membershipScopeUnsafe over the tasks table under
// an alias.
func (d *DB) membershipScopeOnUnsafe(projectRefs []string, alias string) (string, []interface{}) {
	index := d.membershipUnsafe()
	var clauses []string
	var args []interface{}
	seen := map[string]bool{}
	for _, ref := range projectRefs {
		p, ok := index.project(ref)
		if !ok || seen[p.ID] {
			continue
		}
		seen[p.ID] = true
		cond, condArgs := projectMembershipOn(p, alias, d.lowerASCII(alias+"labels"))
		clauses = append(clauses, cond)
		args = append(args, condArgs...)
	}
	if len(clauses) == 0 {
		return "1 = 0", nil
	}
	if len(clauses) == 1 {
		return clauses[0], args
	}
	return "(" + strings.Join(clauses, " OR ") + ")", args
}

// projectRowsScopeUnsafe selects the tickets of a project's trackers and the
// rows still naming the project, the scope of the project's own maintenance
// (its legacy repository paths) rather than of what it shows.
func (d *DB) projectRowsScopeUnsafe(projectID string) (string, []interface{}) {
	cond, args := "project_id = ?", []interface{}{projectID}
	if p, ok := d.membershipUnsafe().project(projectID); ok && len(p.Trackers) > 0 {
		cond += " OR tracker_id IN (" + placeholders(len(p.Trackers)) + ")"
		for _, id := range p.Trackers {
			args = append(args, id)
		}
	}
	return "(" + cond + ")", args
}

// bookmarkedProjectsUnsafe lists the ids of the projects a user bookmarked.
func (d *DB) bookmarkedProjectsUnsafe(userID string) []string {
	rows, err := d.conn.Query(`SELECT project_id FROM user_project_bookmarks WHERE user_id = ?`, userID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	return ids
}

// memberProjectIDsUnsafe lists the ids of the projects a ticket belongs to,
// oldest first.
func (d *DB) memberProjectIDsUnsafe(trackerID string, labels []string) []string {
	ids := []string{}
	for _, p := range d.membershipUnsafe().members(trackerID, labels) {
		ids = append(ids, p.ID)
	}
	return ids
}

// fillTaskProjectsUnsafe computes a ticket's projects (#741): ProjectIDs from
// the membership rule, and ProjectID from scoped, the project a listing is
// about, when the ticket belongs to it, else from the first of them. A ticket
// that names no tracker, written before the adoption reached it, keeps the
// project its row names.
func (d *DB) fillTaskProjectsUnsafe(t *models.Task, scoped string) {
	d.fillTaskProjectsWithRunUnsafe(t, scoped, false)
}

// fillTaskProjectsWithRunUnsafe is fillTaskProjectsUnsafe that, with withRun
// and no scope, takes the project the ticket's run works for when the ticket
// belongs to several: every operation on the ticket made while it runs then
// works for the run's project, whoever reads task.ProjectID.
func (d *DB) fillTaskProjectsWithRunUnsafe(t *models.Task, scoped string, withRun bool) {
	if t.TrackerID == "" {
		t.ProjectIDs = []string{}
		if !strings.HasPrefix(t.ProjectID, trackerSentinelPrefix) && t.ProjectID != "" {
			t.ProjectIDs = []string{t.ProjectID}
		} else {
			t.ProjectID = ""
		}
		return
	}
	t.ProjectIDs = d.memberProjectIDsUnsafe(t.TrackerID, t.Labels)
	t.ProjectID, t.ContextProjectID = "", ""
	if scoped != "" {
		if p, ok := d.membershipUnsafe().project(scoped); ok {
			for _, id := range t.ProjectIDs {
				if id == p.ID {
					t.ProjectID, t.ContextProjectID = p.ID, p.ID
					return
				}
			}
		}
	}
	if len(t.ProjectIDs) > 1 && withRun && scoped == "" {
		if run := d.runProjectOfTaskUnsafe(t.ID); run != "" {
			for _, id := range t.ProjectIDs {
				if id == run {
					t.ProjectID, t.ContextProjectID = run, run
					return
				}
			}
		}
	}
	if len(t.ProjectIDs) > 0 {
		t.ProjectID = t.ProjectIDs[0]
	}
}

// MemberProjects lists the projects a ticket belongs to, oldest first: those
// selecting its tracker whose label it carries, or which have no label. It is
// the Go form of projectMembership.
func (d *DB) MemberProjects(task *models.Task) ([]*models.Project, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.memberProjectsUnsafe(task)
}

func (d *DB) memberProjectsUnsafe(task *models.Task) ([]*models.Project, error) {
	if task == nil {
		return nil, nil
	}
	var ids []string
	if task.TrackerID == "" {
		ids = task.ProjectIDs
	} else {
		ids = d.memberProjectIDsUnsafe(task.TrackerID, task.Labels)
	}
	projects := []*models.Project{}
	for _, id := range ids {
		p, err := d.getProjectByIDUnsafe(id)
		if err != nil {
			return nil, err
		}
		if p != nil {
			projects = append(projects, p)
		}
	}
	return projects, nil
}

// projectTaskCountUnsafe counts the tickets a project shows.
func (d *DB) projectTaskCountUnsafe(projectID string) int {
	cond, args := d.membershipScopeUnsafe([]string{projectID})
	var count int
	_ = d.conn.QueryRow(`SELECT COUNT(*) FROM tasks WHERE `+cond, args...).Scan(&count)
	return count
}
