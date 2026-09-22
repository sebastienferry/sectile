package db

import (
	"testing"
	"time"

	"tasks/internal/models"
)

// The smoke subset shipped with #296 covered raw SQL through d.conn. It did not
// exercise the store's own methods, which is where the engine differences
// actually surface: a query written in SQLite's dialect, or a column scanned
// into a Go type only SQLite converts for. These tests drive the same calls the
// API handlers do.

func TestPostgresBookmarkProject(t *testing.T) {
	d := openPostgres(t)
	seedProjectAndUser(t, d)

	if err := d.BookmarkProject("u1", "p1"); err != nil {
		t.Fatalf("BookmarkProject: %v", err)
	}
	marks, err := d.GetUserProjectBookmarks("u1")
	if err != nil {
		t.Fatalf("GetUserProjectBookmarks: %v", err)
	}
	if len(marks) != 1 || marks[0] != "p1" {
		t.Fatalf("bookmarks = %v, want [p1]", marks)
	}

	// Bookmarking twice must stay idempotent rather than fail on the primary key.
	if err := d.BookmarkProject("u1", "p1"); err != nil {
		t.Fatalf("BookmarkProject twice: %v", err)
	}

	// A second project, because removing the last bookmark is not observable:
	// a user with none gets the default project seeded back on the next read.
	if _, err := d.conn.Exec(`INSERT INTO projects (id, name, slug) VALUES ('p2', 'P2', 'p2')`); err != nil {
		t.Fatalf("seeding a second project: %v", err)
	}
	if err := d.BookmarkProject("u1", "p2"); err != nil {
		t.Fatalf("BookmarkProject p2: %v", err)
	}
	if err := d.UnbookmarkProject("u1", "p2"); err != nil {
		t.Fatalf("UnbookmarkProject: %v", err)
	}
	marks, err = d.GetUserProjectBookmarks("u1")
	if err != nil {
		t.Fatalf("GetUserProjectBookmarks after unbookmark: %v", err)
	}
	for _, m := range marks {
		if m == "p2" {
			t.Fatalf("p2 is still bookmarked: %v", marks)
		}
	}
}

func TestPostgresToggleProjectBookmark(t *testing.T) {
	d := openPostgres(t)
	seedProjectAndUser(t, d)

	on, err := d.ToggleProjectBookmark("u1", "p1")
	if err != nil {
		t.Fatalf("ToggleProjectBookmark on: %v", err)
	}
	if !on {
		t.Fatal("first toggle should bookmark")
	}
	if on, err = d.ToggleProjectBookmark("u1", "p1"); err != nil || on {
		t.Fatalf("second toggle: on=%v err=%v, want off and no error", on, err)
	}
}

func TestPostgresEnsureDefaultBookmark(t *testing.T) {
	d := openPostgres(t)
	seedProjectAndUser(t, d)

	if err := d.EnsureDefaultBookmark("u1"); err != nil {
		t.Fatalf("EnsureDefaultBookmark: %v", err)
	}
	// Called on every sign-in, so it has to be idempotent.
	if err := d.EnsureDefaultBookmark("u1"); err != nil {
		t.Fatalf("EnsureDefaultBookmark twice: %v", err)
	}
}

func TestPostgresTaskActivityHistory(t *testing.T) {
	d := openPostgres(t)
	seedProjectAndUser(t, d)
	seedTask(t, d)

	act := models.TaskActivity{
		ID:        "a1",
		TaskID:    "t1",
		SkillID:   "clarify",
		SkillName: "clarify-issue",
		Action:    "Remote skill execution",
		Status:    "running",
		Summary:   "working",
		Steps:     []string{"one", "two"},
		CreatedAt: time.Now().UTC(),
	}
	if err := d.AddTaskActivity(act); err != nil {
		t.Fatalf("AddTaskActivity: %v", err)
	}

	got, err := d.GetTaskActivities("t1")
	if err != nil {
		t.Fatalf("GetTaskActivities: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("%d activities, want 1", len(got))
	}
	if got[0].ID != "a1" || got[0].SkillName != "clarify-issue" {
		t.Fatalf("activity read back wrong: %+v", got[0])
	}
	if len(got[0].Steps) != 2 {
		t.Fatalf("steps = %v, want two", got[0].Steps)
	}
}

// The task list is what the board renders; it reads the same rows the detail
// view does, plus the boolean columns.
func TestPostgresTaskListAndPin(t *testing.T) {
	d := openPostgres(t)
	seedProjectAndUser(t, d)
	seedTask(t, d)

	tasks, err := d.GetTasks("", "", "", "", "p1", "", "", "", "", nil, nil, false)
	if err != nil {
		t.Fatalf("GetTasks: %v", err)
	}
	if len(tasks) == 0 {
		t.Fatal("no task returned")
	}
}

func seedProjectAndUser(t *testing.T, d *DB) {
	t.Helper()
	if _, err := d.conn.Exec(`INSERT INTO projects (id, name, slug) VALUES ('p1', 'P', 'p')`); err != nil {
		t.Fatalf("seeding a project: %v", err)
	}
	if _, err := d.conn.Exec(`INSERT INTO users (id, subject, email, display_name) VALUES ('u1', 's1', 'u@example.com', 'U')`); err != nil {
		t.Fatalf("seeding a user: %v", err)
	}
}

func seedTask(t *testing.T, d *DB) {
	t.Helper()
	if _, err := d.conn.Exec(
		`INSERT INTO tasks (id, project_id, key, title, status, priority) VALUES ('t1','p1','#1','T','backlog','medium')`); err != nil {
		t.Fatalf("seeding a task: %v", err)
	}
}

// users.role, users.last_sign_in, users.chosen_name, task_comments.user_id,
// macros.framing_comment and project_skills.mode were all added by an ALTER
// their CREATE TABLE never carried, so PostgreSQL shipped without them. These
// exercise the reads that touch each one.

func TestPostgresUserRoleAndProfile(t *testing.T) {
	d := openPostgres(t)
	seedProjectAndUser(t, d)

	users, err := d.ListUsers()
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("%d user(s), want 1", len(users))
	}
	if users[0].Role == "" {
		t.Fatal("the user has no role: users.role is missing or unread")
	}

	u, err := d.GetUser("u1")
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if u == nil || u.ID != "u1" {
		t.Fatalf("GetUser returned %+v", u)
	}

	if _, err := d.SetUserRole("u1", "admin"); err != nil {
		t.Fatalf("SetUserRole: %v", err)
	}
	if u, err = d.GetUser("u1"); err != nil || u.Role != "admin" {
		t.Fatalf("role after change: %+v, %v", u, err)
	}
}

func TestPostgresTaskComments(t *testing.T) {
	d := openPostgres(t)
	seedProjectAndUser(t, d)
	seedTask(t, d)

	if _, err := d.conn.Exec(
		`INSERT INTO task_comments (id, task_id, author, body, user_id) VALUES ('c1','t1','U','hello','u1')`); err != nil {
		t.Fatalf("inserting a comment: %v", err)
	}
	var body, userID string
	if err := d.conn.QueryRow(`SELECT body, user_id FROM task_comments WHERE id = ?`, "c1").Scan(&body, &userID); err != nil {
		t.Fatalf("reading the comment back: %v", err)
	}
	if body != "hello" || userID != "u1" {
		t.Fatalf("comment read back as (%q, %q)", body, userID)
	}
}

func TestPostgresProjectSkillMode(t *testing.T) {
	d := openPostgres(t)
	seedProjectAndUser(t, d)

	if _, err := d.conn.Exec(
		`INSERT INTO project_skills (project_id, skill_id, content, mode, updated_at) VALUES ('p1','clarify','body','autonomous','2026-09-21')`); err != nil {
		t.Fatalf("inserting a project skill: %v", err)
	}
	var mode string
	if err := d.conn.QueryRow(`SELECT mode FROM project_skills WHERE project_id = ? AND skill_id = ?`, "p1", "clarify").Scan(&mode); err != nil {
		t.Fatalf("reading the mode back: %v", err)
	}
	if mode != "autonomous" {
		t.Fatalf("mode = %q", mode)
	}
}

func TestPostgresMacroFramingComment(t *testing.T) {
	d := openPostgres(t)
	seedProjectAndUser(t, d)

	if _, err := d.conn.Exec(
		`INSERT INTO macros (project_id, key, title, framing_comment) VALUES ('p1','M-1','Macro','framed')`); err != nil {
		t.Fatalf("inserting a macro: %v", err)
	}
	var framing string
	if err := d.conn.QueryRow(`SELECT framing_comment FROM macros WHERE project_id = ? AND key = ?`, "p1", "M-1").Scan(&framing); err != nil {
		t.Fatalf("reading framing_comment back: %v", err)
	}
	if framing != "framed" {
		t.Fatalf("framing_comment = %q", framing)
	}
}

// pr_links_detached is the column #298 adds. It is written by the task edit
// that detaches every link and read by the rediscovery gate, so a PostgreSQL
// deployment missing it would rediscover what a person removed on purpose —
// the exact failure mode ADR 0016 warns about for an ALTER-only column.
func TestPostgresPullRequestDetachmentFlag(t *testing.T) {
	d := openPostgres(t)
	seedProjectAndUser(t, d)
	seedTask(t, d)

	links := []models.TaskPullRequest{{URL: "https://forge/pull/1", Branch: "feat/1"}}
	if _, err := d.UpdateTask("t1", models.UpdateTaskRequest{PrLinks: &links}); err != nil {
		t.Fatalf("recording a link: %v", err)
	}
	if d.pullRequestLinksDetached("t1") {
		t.Fatal("recording a link must leave the task attached")
	}

	empty := []models.TaskPullRequest{}
	if _, err := d.UpdateTask("t1", models.UpdateTaskRequest{PrLinks: &empty}); err != nil {
		t.Fatalf("detaching every link: %v", err)
	}
	if !d.pullRequestLinksDetached("t1") {
		t.Fatal("a deliberate detachment was not recorded")
	}

	// Attaching one again forgets the gesture, through the discovery write path.
	task, err := d.GetTaskByID("t1")
	if err != nil || task == nil {
		t.Fatalf("reading the task back: %v", err)
	}
	attached, _, err := d.applyDiscoveredPullRequests(task, links)
	if err != nil || len(attached) != 1 {
		t.Fatalf("attaching a rediscovered link: %+v %v", attached, err)
	}
	if d.pullRequestLinksDetached("t1") {
		t.Fatal("attaching a link again must clear the flag")
	}
}

// owner_user_id is the column the background synchronisation reads to know
// whose credential to borrow. It is the second column added after PostgreSQL
// support shipped, so it takes the route ADR 0017 opened: reconciled in
// initSchema rather than in the legacy migrations PostgreSQL never replays.
func TestPostgresProjectOwner(t *testing.T) {
	d := openPostgres(t)

	created, err := d.CreateProjectAs("u-ada", models.CreateProjectRequest{Name: "Owned", IssueTracker: "jira", JiraProject: "PE"})
	if err != nil {
		t.Fatalf("creating a project: %v", err)
	}
	if created.OwnerUserID != "u-ada" {
		t.Fatalf("owner = %q, want u-ada", created.OwnerUserID)
	}

	read, err := d.GetProjectByID(created.ID)
	if err != nil || read == nil {
		t.Fatalf("reading the project back: %v", err)
	}
	if read.OwnerUserID != "u-ada" {
		t.Fatalf("owner read back = %q, want u-ada", read.OwnerUserID)
	}

	saved, err := d.UpdateProjectAs("u-grace", created.ID, models.UpdateProjectRequest{})
	if err != nil {
		t.Fatalf("saving the project: %v", err)
	}
	if saved.OwnerUserID != "u-ada" {
		t.Fatalf("owner after another person saved it = %q, want u-ada", saved.OwnerUserID)
	}
}
