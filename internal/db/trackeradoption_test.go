package db

import (
	"bytes"
	"log"
	"reflect"
	"strings"
	"testing"
	"time"

	"tasks/internal/models"
)

// forgetTrackers puts a database back where every project still held its own
// tracker, as the binary before #741 left it: no tracker, no link, no ticket
// naming a tracker, no alias and no unique index.
func forgetTrackers(t *testing.T, d *DB) {
	t.Helper()
	for _, statement := range []string{
		"DELETE FROM project_trackers",
		"DELETE FROM trackers",
		"DELETE FROM task_aliases",
		"DELETE FROM auto_sync_trackers",
		"UPDATE tasks SET tracker_id = NULL",
		"UPDATE macros SET tracker_id = NULL",
		"UPDATE projects SET default_tracker_id = ''",
		"DROP INDEX IF EXISTS " + tasksTrackerKeyIndex,
	} {
		if _, err := d.conn.Exec(statement); err != nil {
			t.Fatalf("%s: %v", statement, err)
		}
	}
	d.trackerCache.clear()
}

// legacyProject creates a project, then dates it so that adoption meets the
// projects in a known order.
func legacyProject(t *testing.T, d *DB, created time.Time, req models.CreateProjectRequest) *models.Project {
	t.Helper()
	p, err := d.CreateProject(req)
	if err != nil {
		t.Fatalf("creating %s: %v", req.Name, err)
	}
	if _, err := d.conn.Exec("UPDATE projects SET created_at = ? WHERE id = ?", created, p.ID); err != nil {
		t.Fatal(err)
	}
	return p
}

// legacyTask writes a ticket the way a per-project synchronisation stored it.
func legacyTask(t *testing.T, d *DB, id, projectID, key, status string, updated time.Time, labels, prLinks string) {
	t.Helper()
	if _, err := d.conn.Exec(`INSERT INTO tasks (id, project_id, key, title, status, labels, pr_links, source, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, 'jira', ?, ?)`,
		id, projectID, key, "Ticket "+key, status, labels, prLinks, updated, updated); err != nil {
		t.Fatalf("inserting %s: %v", id, err)
	}
}

func adopt(t *testing.T, d *DB) {
	t.Helper()
	if err := d.adoptTrackers(); err != nil {
		t.Fatalf("adoptTrackers: %v", err)
	}
}

func trackerCount(t *testing.T, d *DB, where string, args ...any) int {
	t.Helper()
	var n int
	if err := d.conn.QueryRow("SELECT COUNT(*) FROM trackers "+where, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func linkedTracker(t *testing.T, d *DB, projectID string) string {
	t.Helper()
	var id string
	if err := d.conn.QueryRow("SELECT tracker_id FROM project_trackers WHERE project_id = ?", projectID).Scan(&id); err != nil {
		t.Fatalf("the tracker of %s: %v", projectID, err)
	}
	return id
}

// twoProjectsOnPE creates two projects on the PE space of one site, the
// second one later, with one duplicated ticket.
func twoProjectsOnPE(t *testing.T, d *DB) (first, second *models.Project) {
	t.Helper()
	base := time.Now().Add(-time.Hour).UTC()
	first = legacyProject(t, d, base, models.CreateProjectRequest{Name: "Delivery", IssueTracker: "jira", JiraProject: "PE", TrackerUrl: "https://acme.atlassian.net"})
	second = legacyProject(t, d, base.Add(time.Minute), models.CreateProjectRequest{Name: "Bidder", IssueTracker: "jira", JiraProject: "PE", TrackerUrl: "https://ACME.atlassian.net/"})
	forgetTrackers(t, d)
	return first, second
}

func TestAdoptionGivesTwoProjectsOnOneJiraSpaceOneTracker(t *testing.T) {
	d := testDB(t)
	first, second := twoProjectsOnPE(t, d)
	adopt(t, d)

	if n := trackerCount(t, d, "WHERE provider = 'jira'"); n != 1 {
		t.Fatalf("%d Jira trackers, want 1", n)
	}
	if a, b := linkedTracker(t, d, first.ID), linkedTracker(t, d, second.ID); a != b {
		t.Fatalf("the two projects link %s and %s", a, b)
	}
	tracker, _ := d.GetTrackerByID(linkedTracker(t, d, first.ID))
	if tracker == nil || tracker.Scope != "PE" || tracker.Name != "Delivery" || tracker.Identity != "jira|acme.atlassian.net|PE" {
		t.Fatalf("tracker = %+v", tracker)
	}
}

func TestAdoptionKeepsTwoJiraSpacesOfOneSiteAsTwoTrackers(t *testing.T) {
	d := testDB(t)
	base := time.Now().Add(-time.Hour).UTC()
	gode := legacyProject(t, d, base, models.CreateProjectRequest{Name: "Gode", IssueTracker: "jira", JiraProject: "GODE", TrackerUrl: "https://acme.atlassian.net"})
	be := legacyProject(t, d, base.Add(time.Minute), models.CreateProjectRequest{Name: "Be", IssueTracker: "jira", JiraProject: "BE", TrackerUrl: "https://acme.atlassian.net"})
	forgetTrackers(t, d)
	adopt(t, d)

	if n := trackerCount(t, d, "WHERE provider = 'jira'"); n != 2 {
		t.Fatalf("%d Jira trackers, want 2", n)
	}
	if linkedTracker(t, d, gode.ID) == linkedTracker(t, d, be.ID) {
		t.Fatal("two spaces of one site share a tracker")
	}
}

func TestAdoptionGivesEveryLocalProjectItsOwnTracker(t *testing.T) {
	d := testDB(t)
	base := time.Now().Add(-time.Hour).UTC()
	one := legacyProject(t, d, base, models.CreateProjectRequest{Name: "One"})
	two := legacyProject(t, d, base.Add(time.Minute), models.CreateProjectRequest{Name: "Two"})
	forgetTrackers(t, d)
	adopt(t, d)

	seen := map[string]string{}
	for _, projectID := range []string{"default", one.ID, two.ID} {
		trackerID := linkedTracker(t, d, projectID)
		if other, ok := seen[trackerID]; ok {
			t.Fatalf("%s and %s share the local tracker %s", other, projectID, trackerID)
		}
		seen[trackerID] = projectID
		tracker, _ := d.GetTrackerByID(trackerID)
		if tracker == nil || tracker.Provider != "local" || tracker.Scope != projectID {
			t.Fatalf("the tracker of %s is %+v", projectID, tracker)
		}
	}
}

func TestAdoptionMergesDuplicateTicketsKeepingTheMostAdvancedStage(t *testing.T) {
	d := testDB(t)
	first, second := twoProjectsOnPE(t, d)
	now := time.Now().UTC()
	legacyTask(t, d, "jira-a-PE-1", first.ID, "PE-1", "clarified", now, `[]`, `[]`)
	legacyTask(t, d, "jira-b-PE-1", second.ID, "PE-1", "implemented", now.Add(-time.Hour), `[]`, `[]`)
	legacyTask(t, d, "jira-a-PE-2", first.ID, "PE-2", "new", now, `[]`, `[]`)
	adopt(t, d)

	var ids []string
	rows, err := d.conn.Query("SELECT id FROM tasks WHERE key = 'PE-1'")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id string
		_ = rows.Scan(&id)
		ids = append(ids, id)
	}
	rows.Close()
	if !reflect.DeepEqual(ids, []string{"jira-b-PE-1"}) {
		t.Fatalf("PE-1 rows = %v, want the implemented one only", ids)
	}
	var status, trackerID string
	if err := d.conn.QueryRow("SELECT status, tracker_id FROM tasks WHERE id = 'jira-b-PE-1'").Scan(&status, &trackerID); err != nil {
		t.Fatal(err)
	}
	if status != "implemented" || trackerID != linkedTracker(t, d, first.ID) {
		t.Fatalf("survivor status %q tracker %q", status, trackerID)
	}
	var untouched int
	_ = d.conn.QueryRow("SELECT COUNT(*) FROM tasks WHERE key = 'PE-2'").Scan(&untouched)
	if untouched != 1 {
		t.Fatalf("a ticket held once must stay: %d rows", untouched)
	}
}

func TestAdoptionUnitesPullRequestLinksAndLabelsOfMergedTickets(t *testing.T) {
	d := testDB(t)
	first, second := twoProjectsOnPE(t, d)
	now := time.Now().UTC()
	legacyTask(t, d, "jira-a-PE-1", first.ID, "PE-1", "reviewed", now, `["backend","#reviewed"]`, `[{"url":"https://github.com/o/a/pull/1","branch":"PE-1"}]`)
	legacyTask(t, d, "jira-b-PE-1", second.ID, "PE-1", "new", now, `["frontend","backend"]`, `[{"url":"https://github.com/o/b/pull/2","branch":"PE-1"},{"url":"https://github.com/o/a/pull/1","branch":"PE-1"}]`)
	adopt(t, d)

	task, err := d.GetTaskByID("jira-a-PE-1")
	if err != nil || task == nil {
		t.Fatalf("survivor: %v %v", task, err)
	}
	if want := []string{"backend", "#reviewed", "frontend"}; !reflect.DeepEqual(task.Labels, want) {
		t.Errorf("labels = %v, want %v", task.Labels, want)
	}
	var urls []string
	for _, link := range task.PrLinks {
		urls = append(urls, link.URL)
	}
	if want := []string{"https://github.com/o/a/pull/1", "https://github.com/o/b/pull/2"}; !reflect.DeepEqual(urls, want) {
		t.Errorf("pull requests = %v, want %v", urls, want)
	}
}

func TestAdoptionRepointsActivitiesCommentsPinsAndBatchMembersToTheSurvivor(t *testing.T) {
	d := testDB(t)
	first, second := twoProjectsOnPE(t, d)
	now := time.Now().UTC()
	legacyTask(t, d, "jira-a-PE-1", first.ID, "PE-1", "specified", now, `[]`, `[]`)
	legacyTask(t, d, "jira-b-PE-1", second.ID, "PE-1", "new", now, `[]`, `[]`)
	for _, statement := range []string{
		`INSERT INTO task_activities (id, task_id, skill_id, skill_name, action, status, created_at) VALUES ('act', 'jira-b-PE-1', 'clarify', 'clarify', 'run', 'completed', '2026-09-01 10:00:00')`,
		`INSERT INTO task_comments (id, task_id, author, body) VALUES ('comment', 'jira-b-PE-1', 'me', 'hello')`,
		`INSERT INTO pinned_tasks (task_id, pinned_at) VALUES ('jira-b-PE-1', '2026-09-01T10:00:00Z')`,
		`INSERT INTO batch_members (run_id, task_id, position) VALUES ('run', 'jira-b-PE-1', 0)`,
		`INSERT INTO batch_members (run_id, task_id, position) VALUES ('both', 'jira-a-PE-1', 0)`,
		`INSERT INTO batch_members (run_id, task_id, position) VALUES ('both', 'jira-b-PE-1', 1)`,
	} {
		if _, err := d.conn.Exec(statement); err != nil {
			t.Fatalf("%s: %v", statement, err)
		}
	}
	adopt(t, d)

	for query, want := range map[string]int{
		"SELECT COUNT(*) FROM task_activities WHERE id = 'act' AND task_id = 'jira-a-PE-1'":           1,
		"SELECT COUNT(*) FROM task_comments WHERE id = 'comment' AND task_id = 'jira-a-PE-1'":         1,
		"SELECT COUNT(*) FROM pinned_tasks WHERE task_id = 'jira-a-PE-1'":                             1,
		"SELECT COUNT(*) FROM batch_members WHERE run_id = 'run' AND task_id = 'jira-a-PE-1'":         1,
		"SELECT COUNT(*) FROM batch_members WHERE run_id = 'both'":                                    1,
		"SELECT COUNT(*) FROM task_activities WHERE task_id = 'jira-b-PE-1'":                          0,
		"SELECT COUNT(*) FROM pinned_tasks WHERE task_id = 'jira-b-PE-1'":                             0,
		"SELECT COUNT(*) FROM batch_members WHERE task_id = 'jira-b-PE-1'":                            0,
		"SELECT COUNT(*) FROM tasks WHERE id = 'jira-b-PE-1'":                                         0,
		"SELECT COUNT(*) FROM tasks WHERE id = 'jira-a-PE-1' AND tracker_id IS NOT NULL":              1,
		"SELECT COUNT(*) FROM task_comments WHERE task_id = 'jira-b-PE-1'":                            0,
		"SELECT COUNT(*) FROM batch_members WHERE run_id = 'both' AND task_id = 'jira-a-PE-1'":        1,
		"SELECT COUNT(*) FROM task_activities WHERE task_id = 'jira-a-PE-1' AND skill_id = 'clarify'": 1,
	} {
		var got int
		if err := d.conn.QueryRow(query).Scan(&got); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
		if got != want {
			t.Errorf("%s = %d, want %d", query, got, want)
		}
	}
}

func TestAdoptionWritesAnAliasForTheMergedAwayTicket(t *testing.T) {
	d := testDB(t)
	first, second := twoProjectsOnPE(t, d)
	now := time.Now().UTC()
	legacyTask(t, d, "jira-a-PE-1", first.ID, "PE-1", "finished", now, `[]`, `[]`)
	legacyTask(t, d, "jira-b-PE-1", second.ID, "PE-1", "new", now, `[]`, `[]`)
	adopt(t, d)

	var survivor string
	if err := d.conn.QueryRow("SELECT task_id FROM task_aliases WHERE old_id = 'jira-b-PE-1'").Scan(&survivor); err != nil {
		t.Fatalf("no alias for the merged-away ticket: %v", err)
	}
	if survivor != "jira-a-PE-1" {
		t.Fatalf("alias points at %q", survivor)
	}
}

func TestAdoptionTakesTheFirstProjectsBoardMirrorAndLogsTheConflict(t *testing.T) {
	d := testDB(t)
	base := time.Now().Add(-time.Hour).UTC()
	enabled, ten := true, 10
	first := legacyProject(t, d, base, models.CreateProjectRequest{Name: "Delivery", IssueTracker: "jira", JiraProject: "PE", TrackerUrl: "https://acme.atlassian.net", BoardID: "1"})
	second := legacyProject(t, d, base.Add(time.Minute), models.CreateProjectRequest{Name: "Bidder", IssueTracker: "jira", JiraProject: "PE", TrackerUrl: "https://acme.atlassian.net", BoardID: "2",
		AutoSyncEnabled: &enabled, AutoSyncIntervalMin: &ten})
	forgetTrackers(t, d)
	// Each project's columns, as its own board import left them.
	if _, err := d.conn.Exec(`UPDATE projects SET board_id = '2', tracker_columns = '[{"name":"Doing","statuses":["In Progress"]}]', auto_sync_enabled = 1, auto_sync_interval_min = 10 WHERE id = ?`, second.ID); err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(previous) })
	adopt(t, d)
	log.SetOutput(previous)

	tracker, _ := d.GetTrackerByID(linkedTracker(t, d, first.ID))
	if tracker == nil || tracker.BoardID != "1" || len(tracker.TrackerColumns) != 0 {
		t.Fatalf("the tracker does not carry the first project's mirror: %+v", tracker)
	}
	if !tracker.AutoSyncEnabled || tracker.AutoSyncIntervalMin != 10 {
		t.Errorf("auto-sync is on when either project had it: %+v", tracker)
	}
	if !strings.Contains(logs.String(), second.ID) {
		t.Errorf("the conflict was not logged: %q", logs.String())
	}
}

func TestAdoptionRunsTwiceAndChangesNothingTheSecondTime(t *testing.T) {
	d := testDB(t)
	first, second := twoProjectsOnPE(t, d)
	now := time.Now().UTC()
	legacyTask(t, d, "jira-a-PE-1", first.ID, "PE-1", "reviewed", now, `[]`, `[]`)
	legacyTask(t, d, "jira-b-PE-1", second.ID, "PE-1", "new", now, `[]`, `[]`)
	adopt(t, d)

	snapshot := func() string {
		var b strings.Builder
		for _, query := range []string{
			"SELECT id || '|' || identity || '|' || updated_at FROM trackers ORDER BY id",
			"SELECT project_id || '|' || tracker_id FROM project_trackers ORDER BY project_id",
			"SELECT id || '|' || COALESCE(tracker_id, '') FROM tasks ORDER BY id",
			"SELECT old_id || '|' || task_id FROM task_aliases ORDER BY old_id",
			"SELECT id || '|' || default_tracker_id FROM projects ORDER BY id",
		} {
			rows, err := d.conn.Query(query)
			if err != nil {
				t.Fatalf("%s: %v", query, err)
			}
			for rows.Next() {
				var line string
				_ = rows.Scan(&line)
				b.WriteString(line + "\n")
			}
			rows.Close()
		}
		return b.String()
	}
	before := snapshot()
	if done, err := d.trackerAdoptionDone(); err != nil || !done {
		t.Fatalf("after one run the adoption is done: %v %v", done, err)
	}
	adopt(t, d)
	if after := snapshot(); after != before {
		t.Fatalf("the second run changed the database:\n%s\nthen\n%s", before, after)
	}
}

func TestMigratedProjectsHaveNoLabelAndTheirTrackerAsDefault(t *testing.T) {
	d := testDB(t)
	first, second := twoProjectsOnPE(t, d)
	adopt(t, d)

	for _, id := range []string{first.ID, second.ID, "default"} {
		p, err := d.GetProjectByID(id)
		if err != nil || p == nil {
			t.Fatalf("%s: %v", id, err)
		}
		if p.Label != "" {
			t.Errorf("%s carries the label %q", id, p.Label)
		}
		linked := linkedTracker(t, d, id)
		if p.DefaultTrackerID != linked || len(p.Trackers) != 1 || p.Trackers[0].TrackerID != linked {
			t.Errorf("%s: default %q, trackers %+v, linked %q", id, p.DefaultTrackerID, p.Trackers, linked)
		}
	}
}
