package db

import (
	"bytes"
	"log"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"tasks/internal/models"
	"tasks/internal/testsqlite"
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
	legacySourcedTask(t, d, id, projectID, key, status, "jira", updated)
	if _, err := d.conn.Exec(`UPDATE tasks SET labels = ?, pr_links = ? WHERE id = ?`, labels, prLinks, id); err != nil {
		t.Fatalf("labelling %s: %v", id, err)
	}
}

// legacySourcedTask writes a ticket of the given source the way the binary
// before #741 stored it, created and updated at the given time, with no tracker.
func legacySourcedTask(t *testing.T, d *DB, id, projectID, key, status, source string, at time.Time) {
	t.Helper()
	if _, err := d.conn.Exec(`INSERT INTO tasks (id, project_id, key, title, status, labels, pr_links, source, created_at, updated_at) VALUES (?, ?, ?, ?, ?, '[]', '[]', ?, ?, ?)`,
		id, projectID, key, "Ticket "+key, status, source, at, at); err != nil {
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
	// Only the loser knows where the work happens, and a person detached its
	// links.
	if _, err := d.conn.Exec(
		`UPDATE tasks SET branch_name = 'feat/PE-1', repo_path = '/work/delivery', repository = 'delivery', pr_links_detached = 1 WHERE id = 'jira-a-PE-1'`,
	); err != nil {
		t.Fatal(err)
	}
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
	var branch, repoPath, repository string
	var detached int
	if err := d.conn.QueryRow("SELECT COALESCE(branch_name, ''), repo_path, repository, pr_links_detached FROM tasks WHERE id = 'jira-b-PE-1'").Scan(
		&branch,
		&repoPath,
		&repository,
		&detached,
	); err != nil {
		t.Fatal(err)
	}
	if branch != "feat/PE-1" || repoPath != "/work/delivery" || repository != "delivery" || detached != 1 {
		t.Fatalf("survivor branch %q, repo path %q, repository %q, detached %d; want the loser's, still detached", branch, repoPath, repository, detached)
	}
	var untouched int
	_ = d.conn.QueryRow("SELECT COUNT(*) FROM tasks WHERE key = 'PE-2'").Scan(&untouched)
	if untouched != 1 {
		t.Fatalf("a ticket held once must stay: %d rows", untouched)
	}
}

// The branch, the repository path and the repository name come from one copy:
// a merged record never pairs a branch of one checkout with the repository of
// another.
func TestAdoptionTakesTheWorkspaceOfAMergedTicketFromOneCopy(t *testing.T) {
	d := testDB(t)
	first, second := twoProjectsOnPE(t, d)
	third := legacyProject(t, d, time.Now().Add(-time.Hour).Add(2*time.Minute).UTC(), models.CreateProjectRequest{
		Name:         "Tracking",
		IssueTracker: "jira",
		JiraProject:  "PE",
		TrackerUrl:   "https://acme.atlassian.net",
	})
	forgetTrackers(t, d)
	now := time.Now().UTC()
	legacyTask(t, d, "jira-a-PE-1", first.ID, "PE-1", "implemented", now, `[]`, `[]`)
	legacyTask(t, d, "jira-b-PE-1", second.ID, "PE-1", "specified", now, `[]`, `[]`)
	legacyTask(t, d, "jira-c-PE-1", third.ID, "PE-1", "new", now, `[]`, `[]`)
	for _, statement := range []string{
		`UPDATE tasks SET branch_name = 'feat/PE-1', repo_path = '/work/bidder' WHERE id = 'jira-b-PE-1'`,
		`UPDATE tasks SET repository = 'tracking' WHERE id = 'jira-c-PE-1'`,
	} {
		if _, err := d.conn.Exec(statement); err != nil {
			t.Fatalf("%s: %v", statement, err)
		}
	}
	adopt(t, d)

	var branch, repoPath, repository string
	if err := d.conn.QueryRow("SELECT COALESCE(branch_name, ''), repo_path, repository FROM tasks WHERE id = 'jira-a-PE-1'").Scan(&branch, &repoPath, &repository); err != nil {
		t.Fatal(err)
	}
	if branch != "feat/PE-1" || repoPath != "/work/bidder" || repository != "" {
		t.Fatalf("survivor branch %q, repo path %q, repository %q; want the first loser's three, not a repository of another copy", branch, repoPath, repository)
	}
}

func TestAdoptionUnitesPullRequestLinksAndLabelsOfMergedTickets(t *testing.T) {
	d := testDB(t)
	first, second := twoProjectsOnPE(t, d)
	now := time.Now().UTC()
	legacyTask(t, d, "jira-a-PE-1", first.ID, "PE-1", "reviewed", now, `["backend","#reviewed"]`, `[{"url":"https://github.com/o/a/pull/1","branch":"PE-1"}]`)
	legacyTask(
		t,
		d,
		"jira-b-PE-1",
		second.ID,
		"PE-1",
		"new",
		now,
		`["frontend","backend"]`,
		`[{"url":"https://github.com/o/b/pull/2","branch":"PE-1"},{"url":"https://github.com/o/a/pull/1","branch":"PE-1"}]`,
	)
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
	if _, err := d.conn.Exec(
		`UPDATE projects SET board_id = '2', tracker_columns = '[{"name":"Doing","statuses":["In Progress"]}]', auto_sync_enabled = 1, auto_sync_interval_min = 10 WHERE id = ?`,
		second.ID,
	); err != nil {
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

// keysOfTasks reads the key of each ticket named.
func keysOfTasks(t *testing.T, d *DB, ids ...string) map[string]string {
	t.Helper()
	keys := map[string]string{}
	for _, id := range ids {
		var key string
		if err := d.conn.QueryRow("SELECT key FROM tasks WHERE id = ?", id).Scan(&key); err != nil {
			t.Fatalf("the key of %s: %v", id, err)
		}
		keys[id] = key
	}
	return keys
}

func TestAdoptionGivesALocalTicketThatSharesARemoteKeyAKeyOfItsOwn(t *testing.T) {
	d := testDB(t)
	first, second := twoProjectsOnPE(t, d)
	now := time.Now().UTC()
	// Delivery numbered its local tickets with the Jira key as prefix, so its
	// PE-1 is not Bidder's PE-1 imported from Jira.
	legacySourcedTask(t, d, "local-a", first.ID, "PE-1", "specified", "local", now.Add(-time.Hour))
	legacySourcedTask(t, d, "jira-b-PE-1", second.ID, "PE-1", "new", "jira", now)
	legacySourcedTask(t, d, "jira-b-PE-7", second.ID, "PE-7", "new", "jira", now)
	for _, statement := range []string{
		`INSERT INTO task_activities (id, task_id, skill_id, skill_name, action, status, created_at) VALUES ('local-run', 'local-a', 'clarify', 'clarify', 'run', 'completed', '2026-09-01 10:00:00')`,
		`INSERT INTO task_activities (id, task_id, skill_id, skill_name, action, status, created_at) VALUES ('remote-run', 'jira-b-PE-1', 'clarify', 'clarify', 'run', 'completed', '2026-09-01 10:00:00')`,
	} {
		if _, err := d.conn.Exec(statement); err != nil {
			t.Fatalf("%s: %v", statement, err)
		}
	}
	adopt(t, d)

	// PE-8 is the key Jira issues next: the next synchronisation would import
	// it over the local ticket.
	if got, want := keysOfTasks(t, d, "local-a", "jira-b-PE-1"), map[string]string{"local-a": "PE.L-1", "jira-b-PE-1": "PE-1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("keys = %v, want %v", got, want)
	}
	assertNoRemoteTrackerIssues(t, keysOfTasks(t, d, "local-a")["local-a"])
	for query, want := range map[string]int{
		"SELECT COUNT(*) FROM task_activities WHERE id = 'local-run' AND task_id = 'local-a'":      1,
		"SELECT COUNT(*) FROM task_activities WHERE id = 'remote-run' AND task_id = 'jira-b-PE-1'": 1,
		"SELECT COUNT(*) FROM task_aliases":                                                        0,
		"SELECT COUNT(*) FROM tasks WHERE id = 'local-a' AND tracker_id IS NOT NULL":               1,
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

// remoteIssuedKey is every key a remote tracker issues: a Jira key, as
// trackerapi's jiraKeyPattern reads one, or a GitHub or GitLab #<n>.
var remoteIssuedKey = regexp.MustCompile(`^(?:[A-Z][A-Z0-9_]*-[0-9]+|#[0-9]+)$`)

// assertNoRemoteTrackerIssues checks that a re-keyed local ticket holds a key
// no remote tracker can issue, and that no ticket key is read inside it.
func assertNoRemoteTrackerIssues(t *testing.T, key string) {
	t.Helper()
	if remoteIssuedKey.MatchString(strings.ToUpper(key)) {
		t.Fatalf("the local key %q is one a remote tracker issues", key)
	}
	if found := entryKeyPattern.FindString(key); found != "" {
		t.Fatalf("the local key %q reads as the ticket key %q", key, found)
	}
}

func TestAdoptionRekeysEveryLocalTicketSharingAKeyOnARemoteTracker(t *testing.T) {
	d := testDB(t)
	first, second := twoProjectsOnPE(t, d)
	now := time.Now().UTC()
	legacySourcedTask(t, d, "local-b", second.ID, "PE-1", "new", "local", now)
	legacySourcedTask(t, d, "local-a", first.ID, "PE-1", "new", "local", now.Add(-time.Hour))
	if _, err := d.conn.Exec(
		`INSERT INTO task_comments (id, task_id, author, body) VALUES ('comment', 'local-b', 'me', 'hello')`,
	); err != nil {
		t.Fatal(err)
	}
	adopt(t, d)

	// PE-1 is a key Jira issues: neither local copy keeps it, and they are
	// numbered oldest first.
	keys := keysOfTasks(t, d, "local-a", "local-b")
	if want := map[string]string{"local-a": "PE.L-1", "local-b": "PE.L-2"}; !reflect.DeepEqual(keys, want) {
		t.Fatalf("keys = %v, want %v", keys, want)
	}
	for _, key := range keys {
		assertNoRemoteTrackerIssues(t, key)
	}
	var comments int
	_ = d.conn.QueryRow("SELECT COUNT(*) FROM task_comments WHERE id = 'comment' AND task_id = 'local-b'").Scan(&comments)
	if comments != 1 {
		t.Fatal("the comment left the re-keyed ticket")
	}
}

// On a local board, the oldest of two local tickets sharing a key keeps it:
// the board issues no key of its own. A project's tickets stored under its id
// and under its slug share its board.
func TestAdoptionKeepsTheOldestOfTwoLocalTicketsOnTheirSharedKeyOnALocalBoard(t *testing.T) {
	d := testDB(t)
	now := time.Now().UTC()
	notes := legacyProject(t, d, now.Add(-time.Hour), models.CreateProjectRequest{Name: "Notes"})
	forgetTrackers(t, d)
	legacySourcedTask(t, d, "local-b", notes.Slug, "NOTES-1", "new", "local", now)
	legacySourcedTask(t, d, "local-a", notes.ID, "NOTES-1", "new", "local", now.Add(-time.Hour))
	adopt(t, d)

	if got, want := keysOfTasks(t, d, "local-a", "local-b"), map[string]string{"local-a": "NOTES-1", "local-b": "NOTES-2"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("keys = %v, want %v", got, want)
	}
}

// plantAnUntaggedCopyAfterARollback adopts two projects' copies of PE-1 and of
// the epic PE-5, then writes what an older binary leaves after a rollback:
// Bidder's copies of PE-1 and of PE-5 again, naming no tracker, next to the
// adopted records. It returns the adopted ticket's id.
func plantAnUntaggedCopyAfterARollback(t *testing.T, d *DB) string {
	t.Helper()
	first, second := twoProjectsOnPE(t, d)
	now := time.Now().UTC()
	legacyTask(t, d, "jira-a-PE-1", first.ID, "PE-1", "implemented", now, `[]`, `[]`)
	legacyTask(t, d, "jira-b-PE-1", second.ID, "PE-1", "new", now, `[]`, `[]`)
	legacyEpic(t, d, first.ID, "PE-5", `[{"id":"t1","text":"Keep me","done":false}]`, now)
	adopt(t, d)
	if err := d.adoptTrackerEpics(); err != nil {
		t.Fatalf("adoptTrackerEpics: %v", err)
	}

	legacyTask(t, d, "jira-b-PE-1-again", second.ID, "PE-1", "new", now.Add(time.Minute), `[]`, `[]`)
	legacyEpic(t, d, second.ID, "PE-5", "[]", now.Add(-time.Hour))
	for _, statement := range []string{
		`INSERT INTO task_activities (id, task_id, skill_id, skill_name, action, status, created_at) VALUES ('adopted-run', 'jira-a-PE-1', 'clarify', 'clarify', 'run', 'completed', '2026-09-01 10:00:00')`,
		`INSERT INTO task_activities (id, task_id, skill_id, skill_name, action, status, created_at) VALUES ('rollback-run', 'jira-b-PE-1-again', 'clarify', 'clarify', 'run', 'completed', '2026-09-02 10:00:00')`,
		`INSERT INTO task_comments (id, task_id, author, body) VALUES ('rollback-comment', 'jira-b-PE-1-again', 'me', 'hello')`,
	} {
		if _, err := d.conn.Exec(statement); err != nil {
			t.Fatalf("%s: %v", statement, err)
		}
	}
	return "jira-a-PE-1"
}

// legacyEpic writes a Jira epic row the way the binary before #741 kept it:
// under its project, naming no tracker.
func legacyEpic(t *testing.T, d *DB, projectID, key, todos string, updated time.Time) {
	t.Helper()
	if _, err := d.conn.Exec("INSERT INTO macros (project_id, key, title, todos, updated_at) VALUES (?, ?, 'Epic', ?, ?)", projectID, key, todos, updated); err != nil {
		t.Fatalf("inserting the epic %s of %s: %v", key, projectID, err)
	}
}

// assertRerunMergedTheUntaggedCopies checks, after a restart, that the copies
// an older binary wrote were merged into the adopted records.
func assertRerunMergedTheUntaggedCopies(t *testing.T, d *DB, adoptedID string) {
	t.Helper()
	var rows int
	var survivor string
	if err := d.conn.QueryRow("SELECT COUNT(*), MIN(id) FROM tasks WHERE key = 'PE-1'").Scan(&rows, &survivor); err != nil {
		t.Fatal(err)
	}
	if rows != 1 || survivor != adoptedID {
		t.Fatalf("PE-1 rows = %d, survivor %q; want %s alone", rows, survivor, adoptedID)
	}
	for query, want := range map[string]int{
		"SELECT COUNT(*) FROM tasks WHERE id = ? AND tracker_id IS NOT NULL":                               1,
		"SELECT COUNT(*) FROM task_activities WHERE id IN ('adopted-run', 'rollback-run') AND task_id = ?": 2,
		"SELECT COUNT(*) FROM task_comments WHERE id = 'rollback-comment' AND task_id = ?":                 1,
		"SELECT COUNT(*) FROM task_aliases WHERE old_id = 'jira-b-PE-1-again' AND task_id = ?":             1,
	} {
		var got int
		if err := d.conn.QueryRow(query, adoptedID).Scan(&got); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
		if got != want {
			t.Errorf("%s = %d, want %d", query, got, want)
		}
	}
	var epics, tagged int
	var todos string
	if err := d.conn.QueryRow("SELECT COUNT(*), COUNT(tracker_id), MIN(todos) FROM macros WHERE key = 'PE-5'").Scan(&epics, &tagged, &todos); err != nil {
		t.Fatal(err)
	}
	if epics != 1 || tagged != 1 || len(parseMacroTodos(todos)) != 1 {
		t.Fatalf("PE-5 rows = %d, %d naming a tracker, todos %s; want the adopted epic alone, with its todos", epics, tagged, todos)
	}
	for _, index := range []string{tasksTrackerKeyIndex, macrosTrackerKeyIndex} {
		if indexed, err := d.indexExists(index); err != nil || !indexed {
			t.Fatalf("%s is missing after the rerun: %v", index, err)
		}
	}
}

// A rollback to a binary before #741 writes tickets and epics that name no
// tracker next to the adopted ones. The next start reruns the adoption, which
// must merge them rather than fail on the unique indexes and refuse to start.
func TestAdoptionRerunsAfterARollbackWroteAnUntaggedCopy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.db")
	d, err := testsqlite.New(t, path, NewDB)
	if err != nil {
		t.Fatal(err)
	}
	adoptedID := plantAnUntaggedCopyAfterARollback(t, d)
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}

	restarted, err := NewDB(path)
	if err != nil {
		t.Fatalf("the server does not start again: %v", err)
	}
	t.Cleanup(func() { restarted.Close() })
	assertRerunMergedTheUntaggedCopies(t, restarted, adoptedID)
}

// A rollback may write an epic row alone, with no ticket naming no tracker: the
// next start reruns the adoption for it too.
func TestAdoptionRerunsAfterARollbackWroteAnUntaggedEpicAlone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.db")
	d, err := testsqlite.New(t, path, NewDB)
	if err != nil {
		t.Fatal(err)
	}
	plantAnUntaggedEpicAlone(t, d)
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}

	restarted, err := NewDB(path)
	if err != nil {
		t.Fatalf("the server does not start again: %v", err)
	}
	t.Cleanup(func() { restarted.Close() })
	assertRerunMergedTheUntaggedEpic(t, restarted)
}

// plantAnUntaggedEpicAlone adopts the epic PE-5 of two projects on PE, then
// writes what an older binary leaves after a rollback: Bidder's copy of PE-5
// naming no tracker, with no ticket, next to a milestone and a local project's
// macro, which the adoption never tags.
func plantAnUntaggedEpicAlone(t *testing.T, d *DB) {
	t.Helper()
	first, second := twoProjectsOnPE(t, d)
	now := time.Now().UTC()
	legacyEpic(t, d, first.ID, "PE-5", `[{"id":"t1","text":"Keep me","done":false}]`, now)
	adopt(t, d)
	if err := d.adoptTrackerEpics(); err != nil {
		t.Fatalf("adoptTrackerEpics: %v", err)
	}
	legacyEpic(t, d, second.ID, "PE-5", "[]", now.Add(-time.Hour))
	legacyEpic(t, d, second.ID, "M-1", "[]", now)
	legacyEpic(t, d, "default", "TASK-9", "[]", now)
	if done, err := d.trackerAdoptionDone(); err != nil || done {
		t.Fatalf("an untagged Jira epic left the adoption done: %v %v", done, err)
	}
}

// assertRerunMergedTheUntaggedEpic checks, after a restart, that the epic row
// an older binary wrote was merged into the adopted one, and that the
// milestone and the local macro left untagged do not keep the adoption going.
func assertRerunMergedTheUntaggedEpic(t *testing.T, d *DB) {
	t.Helper()
	var epics, tagged int
	var todos string
	if err := d.conn.QueryRow("SELECT COUNT(*), COUNT(tracker_id), MIN(todos) FROM macros WHERE key = 'PE-5'").Scan(&epics, &tagged, &todos); err != nil {
		t.Fatal(err)
	}
	if epics != 1 || tagged != 1 || len(parseMacroTodos(todos)) != 1 {
		t.Fatalf("PE-5 rows = %d, %d naming a tracker, todos %s; want the adopted epic alone, with its todos", epics, tagged, todos)
	}
	if done, err := d.trackerAdoptionDone(); err != nil || !done {
		t.Fatalf("after the rerun the adoption is done, the milestone and the local macro untagged: %v %v", done, err)
	}
}

// A project selecting a local and a Jira tracker at one position names its
// first tracker by the same order in the adoption and in its done check: the
// pass leaves the epic of a project whose first tracker is local untagged, and
// the check must not then count it, or the adoption reruns at every start.
func TestAdoptionPicksOneFirstTrackerOfTwoAtOnePosition(t *testing.T) {
	d := testDB(t)
	p := legacyProject(t, d, time.Now().Add(-time.Hour).UTC(), models.CreateProjectRequest{Name: "Delivery", IssueTracker: "jira", JiraProject: "PE", TrackerUrl: "https://acme.atlassian.net"})
	forgetTrackers(t, d)
	now := time.Now().UTC()
	local := models.Tracker{ID: "a-local", Name: "Delivery", Provider: "local", Scope: p.ID, CreatedAt: now, UpdatedAt: now}
	local.Identity = trackerIdentityFor(&local, nil)
	jira := models.Tracker{ID: "b-jira", Name: "Delivery", Provider: "jira", Site: "https://acme.atlassian.net", Scope: "PE", CreatedAt: now, UpdatedAt: now}
	jira.Identity = trackerIdentityFor(&jira, nil)
	if err := d.conn.WithTx(func(tx *sqlTx) error {
		for _, trk := range []*models.Tracker{&local, &jira} {
			if err := insertTrackerOn(tx, trk); err != nil {
				return err
			}
			if _, err := tx.Exec(`INSERT INTO project_trackers (project_id, tracker_id, position) VALUES (?, ?, 0)`, p.ID, trk.ID); err != nil {
				return err
			}
		}
		_, err := tx.Exec(`UPDATE projects SET default_tracker_id = ? WHERE id = ?`, jira.ID, p.ID)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	d.trackerCache.clear()
	legacyEpic(t, d, p.ID, "PE-5", "[]", now)

	adopt(t, d)
	if done, err := d.trackerAdoptionDone(); err != nil || !done {
		t.Fatalf("after one pass the adoption is done: %v %v", done, err)
	}
}
