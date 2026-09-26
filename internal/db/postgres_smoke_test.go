package db

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tasks/internal/models"
	"tasks/internal/secrets"
)

// postgresDSN is the test database, or "" when none is configured. The smoke
// tests are the only ones in this package that need a server, so they skip
// rather than fail when there is not one: a contributor running `go test ./...`
// on a laptop should not need a container.
func postgresDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("SECTILE_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set SECTILE_TEST_POSTGRES_DSN to run the PostgreSQL smoke tests")
	}
	return dsn
}

// openPostgres opens the test database and empties it, so each test starts from
// the same place whatever ran before it.
func openPostgres(t *testing.T) *DB {
	t.Helper()
	d, err := Open(Config{Driver: DriverPostgres, DSN: postgresDSN(t)})
	if err != nil {
		t.Fatalf("opening PostgreSQL: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	for _, table := range migrationTables {
		if _, err := d.conn.Exec("TRUNCATE " + table + " CASCADE"); err != nil {
			t.Fatalf("clearing %s: %v", table, err)
		}
	}
	return d
}

// TestPostgresSchemaIsCreatedFromEmpty is the first thing that breaks if a type
// name or a statement is not portable: the schema simply fails to build.
func TestPostgresSchemaIsCreatedFromEmpty(t *testing.T) {
	d := openPostgres(t)
	for _, table := range migrationTables {
		var n int
		if err := d.conn.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); err != nil {
			t.Errorf("table %q is missing or unreadable: %v", table, err)
		}
	}
}

// TestPostgresBoardViews runs the saved view selection on PostgreSQL: the
// label predicate relies on LOWER, LIKE and ESCAPE behaving as under SQLite.
func TestPostgresBoardViews(t *testing.T) {
	d := openPostgres(t)
	f := seedViewFixture(t, d)
	checkBoardViewSelection(t, f)

	view := f.create(t, "u1", "Facets", []string{f.alpha, f.beta}, []string{"platform"})
	facets, err := d.GetTaskFacetsInScope(TaskScope{UserID: "u1", ViewID: view.ID})
	if err != nil {
		t.Fatalf("GetTaskFacetsInScope: %v", err)
	}
	if facets.Total != 2 {
		t.Errorf("facet total = %d, want 2", facets.Total)
	}
	if _, err := f.db.CreateBoardView("u1", models.BoardViewRequest{Name: &view.Name, ProjectIDs: &view.ProjectIDs}); err == nil {
		t.Error("a second view with the same name was accepted")
	}
	if err := d.DeleteProject(f.beta); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
	got, err := d.GetBoardView("u1", view.ID)
	if err != nil || len(got.ProjectIDs) != 1 {
		t.Errorf("after deleting a project: %+v, %v", got, err)
	}
}

// TestPostgresStartingTwiceIsHarmless covers the second start: the schema
// statements are all IF NOT EXISTS, and a second pass must not disturb data.
func TestPostgresStartingTwiceIsHarmless(t *testing.T) {
	d := openPostgres(t)
	if _, err := d.conn.Exec(
		`INSERT INTO projects (id, name, slug) VALUES ('p1', 'Kept', 'kept')`); err != nil {
		t.Fatalf("seeding a project: %v", err)
	}

	again, err := Open(Config{Driver: DriverPostgres, DSN: postgresDSN(t)})
	if err != nil {
		t.Fatalf("second open: %v", err)
	}
	defer again.Close()

	var name string
	if err := again.conn.QueryRow(`SELECT name FROM projects WHERE id = ?`, "p1").Scan(&name); err != nil {
		t.Fatalf("the project did not survive a second start: %v", err)
	}
	if name != "Kept" {
		t.Fatalf("name = %q, want %q", name, "Kept")
	}
}

// TestPostgresTaskRoundTrip is the CRUD path, through the same *DB methods the
// API uses, so it exercises the rebound placeholders in anger.
func TestPostgresTaskRoundTrip(t *testing.T) {
	d := openPostgres(t)
	if _, err := d.conn.Exec(`INSERT INTO projects (id, name, slug) VALUES ('p1', 'P', 'p')`); err != nil {
		t.Fatalf("seeding a project: %v", err)
	}

	task := models.Task{
		ID:        "t1",
		ProjectID: "p1",
		Key:       "#1",
		Title:     "Round trip",
		Status:    "backlog",
		Priority:  "high",
	}
	if _, err := d.conn.Exec(
		`INSERT INTO tasks (id, project_id, key, title, status, priority) VALUES (?, ?, ?, ?, ?, ?)`,
		task.ID, task.ProjectID, task.Key, task.Title, task.Status, task.Priority); err != nil {
		t.Fatalf("insert: %v", err)
	}

	var title, status string
	if err := d.conn.QueryRow(`SELECT title, status FROM tasks WHERE id = ?`, "t1").Scan(&title, &status); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if title != "Round trip" || status != "backlog" {
		t.Fatalf("read back (%q, %q), want (%q, %q)", title, status, "Round trip", "backlog")
	}

	if _, err := d.conn.Exec(`UPDATE tasks SET status = ? WHERE id = ?`, "clarified", "t1"); err != nil {
		t.Fatalf("update: %v", err)
	}
	if err := d.conn.QueryRow(`SELECT status FROM tasks WHERE id = ?`, "t1").Scan(&status); err != nil {
		t.Fatalf("read after update: %v", err)
	}
	if status != "clarified" {
		t.Fatalf("status = %q, want %q", status, "clarified")
	}

	if _, err := d.conn.Exec(`DELETE FROM tasks WHERE id = ?`, "t1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	var n int
	if err := d.conn.QueryRow(`SELECT COUNT(*) FROM tasks WHERE id = ?`, "t1").Scan(&n); err != nil {
		t.Fatalf("count after delete: %v", err)
	}
	if n != 0 {
		t.Fatalf("%d row(s) left after delete, want 0", n)
	}
}

// TestPostgresUpsertUpdatesRatherThanDuplicating covers ON CONFLICT, which the
// schema relies on in eighteen places.
func TestPostgresUpsertUpdatesRatherThanDuplicating(t *testing.T) {
	d := openPostgres(t)
	if _, err := d.conn.Exec(`INSERT INTO projects (id, name, slug) VALUES ('p1', 'P', 'p')`); err != nil {
		t.Fatalf("seeding a project: %v", err)
	}
	const upsert = `INSERT INTO tasks (id, project_id, key, title, status, priority) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(project_id, key) DO UPDATE SET title = excluded.title`
	for _, title := range []string{"first", "second"} {
		if _, err := d.conn.Exec(upsert, "t"+title, "p1", "#1", title, "backlog", "medium"); err != nil {
			t.Fatalf("upsert %q: %v", title, err)
		}
	}

	var n int
	if err := d.conn.QueryRow(`SELECT COUNT(*) FROM tasks WHERE project_id = ? AND key = ?`, "p1", "#1").Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("%d row(s) for one natural key, want 1", n)
	}
	var title string
	if err := d.conn.QueryRow(`SELECT title FROM tasks WHERE project_id = ? AND key = ?`, "p1", "#1").Scan(&title); err != nil {
		t.Fatalf("read: %v", err)
	}
	if title != "second" {
		t.Fatalf("title = %q, want the updated %q", title, "second")
	}
}

// TestPostgresTimestampRoundTrip re-runs the #294 regression against the other
// engine. It is the likeliest place for a silent difference: DATETIME is not a
// PostgreSQL type and CURRENT_TIMESTAMP does not mean the same thing unless the
// session is pinned to UTC.
func TestPostgresTimestampRoundTrip(t *testing.T) {
	d := openPostgres(t)
	if _, err := d.conn.Exec(`INSERT INTO projects (id, name, slug) VALUES ('p1', 'P', 'p')`); err != nil {
		t.Fatalf("seeding a project: %v", err)
	}

	// A time in a zone that is not the server's own, which is exactly what a
	// tracker hands over.
	zone := time.FixedZone("UTC+7", 7*3600)
	written := time.Date(2026, 3, 14, 15, 9, 26, 0, zone)

	if _, err := d.conn.Exec(
		`INSERT INTO tasks (id, project_id, key, title, tracker_created_at) VALUES (?, ?, ?, ?, ?)`,
		"t1", "p1", "#1", "T", written); err != nil {
		t.Fatalf("insert: %v", err)
	}
	var read time.Time
	if err := d.conn.QueryRow(`SELECT tracker_created_at FROM tasks WHERE id = ?`, "t1").Scan(&read); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !read.Equal(written) {
		t.Fatalf("timestamp did not survive the round trip:\n wrote: %s\n  read: %s", written, read)
	}

	// CURRENT_TIMESTAMP must record the right instant. What it must NOT do is
	// depend on the zone the server happens to run in: a column typed
	// TIMESTAMPTZ stores an absolute instant, so the value is comparable
	// across deployments whatever each one's local zone is. The location the
	// driver hands back is a rendering detail and is deliberately not asserted.
	before := time.Now().Add(-time.Minute)
	var def time.Time
	if err := d.conn.QueryRow(`SELECT created_at FROM tasks WHERE id = ?`, "t1").Scan(&def); err != nil {
		t.Fatalf("reading the defaulted timestamp: %v", err)
	}
	if def.Before(before) || def.After(time.Now().Add(time.Minute)) {
		t.Fatalf("CURRENT_TIMESTAMP recorded %s, which is not the instant the row was written (%s)", def, time.Now())
	}
}

// TestMigrateSQLiteToPostgres is the round trip the owner asked to be covered:
// a populated SQLite database, including credentials sealed both ways, must
// arrive intact and still open.
func TestMigrateSQLiteToPostgres(t *testing.T) {
	// The key both ends share. In a real migration the operator carries it
	// across; here setting it before either database opens is the same thing.
	t.Setenv(secrets.KeyEnvVar, "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff")

	dst := openPostgres(t)

	srcPath := filepath.Join(t.TempDir(), "source.db")
	src, err := NewDB(srcPath)
	if err != nil {
		t.Fatalf("creating the source: %v", err)
	}
	seedForMigration(t, src)

	counts, err := Migrate(src, dst, nil)
	if err != nil {
		t.Fatalf("migration: %v", err)
	}
	defer src.Close()

	// The requirement is parity with the source, table by table, not a count
	// someone wrote down: the source also carries the rows a first start seeds.
	for _, c := range counts {
		var want int
		if err := src.conn.QueryRow("SELECT COUNT(*) FROM " + c.Table).Scan(&want); err != nil {
			t.Fatalf("counting %s in the source: %v", c.Table, err)
		}
		if c.Rows != want {
			t.Errorf("%s: copied %d row(s), source holds %d", c.Table, c.Rows, want)
		}
		var got int
		if err := dst.conn.QueryRow("SELECT COUNT(*) FROM " + c.Table).Scan(&got); err != nil {
			t.Fatalf("counting %s in the destination: %v", c.Table, err)
		}
		if got != want {
			t.Errorf("%s: destination holds %d row(s), source holds %d", c.Table, got, want)
		}
	}

	var title string
	if err := dst.conn.QueryRow(`SELECT title FROM tasks WHERE id = ?`, "t1").Scan(&title); err != nil {
		t.Fatalf("the migrated task is not readable: %v", err)
	}
	if title != "First" {
		t.Fatalf("title = %q, want %q", title, "First")
	}

	// The credential sealed under the server key must still open, which is the
	// whole point of the key precondition.
	_, _, token, err := dst.userTrackerCredential("u1", "jira")
	if err != nil {
		t.Fatalf("the migrated credential does not open: %v", err)
	}
	if token != "jira-token" {
		t.Fatalf("token = %q, want %q", token, "jira-token")
	}
}

// TestMigrateRefusesANonEmptyDestination protects against running it twice.
func TestMigrateRefusesANonEmptyDestination(t *testing.T) {
	t.Setenv(secrets.KeyEnvVar, "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff")

	dst := openPostgres(t)

	src, err := NewDB(filepath.Join(t.TempDir(), "source.db"))
	if err != nil {
		t.Fatalf("creating the source: %v", err)
	}
	defer src.Close()
	seedForMigration(t, src)

	if _, err := dst.conn.Exec(`INSERT INTO tasks (id, project_id, key, title) VALUES ('x', 'p', '#9', 'Already here')`); err != nil {
		t.Fatalf("seeding the destination: %v", err)
	}

	if _, err := Migrate(src, dst, nil); err == nil {
		t.Fatal("migrating into a database that already holds rows must be refused")
	}
}

func seedForMigration(t *testing.T, d *DB) {
	t.Helper()
	if _, err := d.conn.Exec(`INSERT INTO projects (id, name, slug) VALUES ('p1', 'P', 'p')`); err != nil {
		t.Fatalf("seeding a project: %v", err)
	}
	for _, row := range [][2]string{{"t1", "First"}, {"t2", "Second"}} {
		if _, err := d.conn.Exec(
			`INSERT INTO tasks (id, project_id, key, title) VALUES (?, ?, ?, ?)`,
			row[0], "p1", "#"+row[0], row[1]); err != nil {
			t.Fatalf("seeding a task: %v", err)
		}
	}
	if _, err := d.conn.Exec(`INSERT INTO users (id, subject, email, display_name) VALUES ('u1', 'sub-1', 'u@example.com', 'U')`); err != nil {
		t.Fatalf("seeding a user: %v", err)
	}
	if err := d.SetUserTrackerCredential("u1", "jira", "https://example.atlassian.net", "u@example.com", "jira-token", ""); err != nil {
		t.Fatalf("seeding a credential: %v", err)
	}
}

// TestMigrateRefusesAKeyThatDoesNotOpenTheCredentials is the guard the whole
// precondition exists for. Sealed tokens are bound to their owner and tracker,
// never to the database, so these rows would copy perfectly well under the
// wrong key and nobody would find out until a tracker call failed. The
// migration has to refuse first, and before it has written anything.
func TestMigrateRefusesAKeyThatDoesNotOpenTheCredentials(t *testing.T) {
	dst := openPostgres(t)

	srcPath := filepath.Join(t.TempDir(), "source.db")
	func() {
		t.Setenv(secrets.KeyEnvVar, "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff")
		src, err := NewDB(srcPath)
		if err != nil {
			t.Fatalf("creating the source: %v", err)
		}
		defer src.Close()
		seedForMigration(t, src)
	}()

	// A different key, as an operator who forgot to carry theirs across would
	// have.
	t.Setenv(secrets.KeyEnvVar, "ffeeddccbbaa99887766554433221100ffeeddccbbaa99887766554433221100")
	src, err := NewDB(srcPath)
	if err != nil {
		t.Fatalf("reopening the source: %v", err)
	}
	defer src.Close()

	if _, err := Migrate(src, dst, nil); err == nil {
		t.Fatal("migrating under a key that cannot open the credentials must be refused")
	}

	var n int
	if err := dst.conn.QueryRow("SELECT COUNT(*) FROM tasks").Scan(&n); err != nil {
		t.Fatalf("counting the destination: %v", err)
	}
	if n != 0 {
		t.Fatalf("%d task(s) were written before the refusal, want none", n)
	}
}

// TestPostgresConnectsFromLibpqEnvironment proves the path a Kubernetes
// deployment actually uses: the database module hands out a username and a
// password as two separate secrets, Kubernetes cannot interpolate a secret into
// a string, so the connection arrives as the standard libpq variables rather
// than as one DATABASE_URL.
func TestPostgresConnectsFromLibpqEnvironment(t *testing.T) {
	u, err := url.Parse(postgresDSN(t))
	if err != nil {
		t.Fatalf("the test DSN is not a URL, cannot derive PG* variables: %v", err)
	}
	host, port := u.Hostname(), u.Port()
	if port == "" {
		port = "5432"
	}
	t.Setenv("PGHOST", host)
	t.Setenv("PGPORT", port)
	t.Setenv("PGDATABASE", strings.TrimPrefix(u.Path, "/"))
	t.Setenv("PGUSER", u.User.Username())
	if pw, ok := u.User.Password(); ok {
		t.Setenv("PGPASSWORD", pw)
	}
	t.Setenv("PGSSLMODE", u.Query().Get("sslmode"))

	d, err := Open(Config{Driver: DriverPostgres, FromEnvironment: true})
	if err != nil {
		t.Fatalf("opening from the libpq environment: %v", err)
	}
	defer d.Close()

	var n int
	if err := d.conn.QueryRow("SELECT COUNT(*) FROM projects").Scan(&n); err != nil {
		t.Fatalf("the schema is not usable over the environment-provided connection: %v", err)
	}
}

// A PostgreSQL configuration naming neither source must not reach the engine at
// all: pgx would default the host to localhost and dial whatever is there.
func TestPostgresRefusesAnEmptyConfiguration(t *testing.T) {
	if _, err := Open(Config{Driver: DriverPostgres}); err == nil {
		t.Fatal("a PostgreSQL config with no DSN and no environment must be refused")
	}
}

// TestPostgresCredentialUnlocks runs the unlock storage and the presence
// queries of #501 on PostgreSQL: the correlated subqueries, the COALESCE over
// timestamps and the UPDATE that carries an agent's presence into its unlock.
func TestPostgresCredentialUnlocks(t *testing.T) {
	d := openPostgres(t)
	for _, table := range []string{"user_credential_unlocks", "agent_presence", "server_instances"} {
		if _, err := d.conn.Exec("DELETE FROM " + table); err != nil {
			t.Fatalf("clearing %s: %v", table, err)
		}
	}
	// A PostgreSQL store has no directory to hold a key file: production sets
	// SECTILE_SECRET_KEY, the test gives it one.
	key, err := secrets.ServerKey(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	d.serverKey, d.serverKeyErr = key, nil
	now := time.Now().UTC()

	// Stored, listed and resolved through the join.
	sealedAndUnlocked(t, d, "u1", now.Add(-3*time.Hour))
	if _, _, token, err := d.userTrackerCredential("u1", "jira"); err != nil || token != "sealed-token" {
		t.Fatalf("resolving the unlocked credential: %q %v", token, err)
	}
	credentials, err := d.UserTrackerCredentials("u1")
	if err != nil || len(credentials) != 1 || !credentials[0].Unlocked {
		t.Fatalf("listing: %+v %v", credentials, err)
	}

	// A session seen 5 minutes ago keeps it; its sign-out alone drops it.
	token := browserSession(t, d, "u1", now.Add(-5*time.Minute))
	if forgotten, err := d.ForgetIdleUnlocks(now); err != nil || forgotten != 0 {
		t.Fatalf("a present owner keeps the unlock: %d %v", forgotten, err)
	}
	if err := d.RevokeWebSession(token); err != nil {
		t.Fatal(err)
	}
	serverInstance(t, d, "live", now)
	agentOn(t, d, "u1", "live", now.Add(-time.Hour), time.Time{})
	if err := d.ForgetUnlocksIfAbsent("u1", now); err != nil || unlockRows(t, d, "u1") != 1 {
		t.Fatalf("a connected agent keeps the unlock through a sign-out: %v", err)
	}

	// The agent's instance dies and is reclaimed: its last heartbeat carries.
	lastHeartbeat := now.Add(-10 * time.Minute)
	if _, err := d.conn.Exec(`UPDATE server_instances SET last_seen = ? WHERE id = 'live'`, lastHeartbeat); err != nil {
		t.Fatal(err)
	}
	if _, err := d.reclaimDeadInstances(now); err != nil {
		t.Fatal(err)
	}
	if forgotten, err := d.ForgetIdleUnlocks(now); err != nil || forgotten != 0 {
		t.Fatalf("the reclaimed agent still counts: %d %v", forgotten, err)
	}
	if forgotten, err := d.ForgetIdleUnlocks(lastHeartbeat.Add(31 * time.Minute)); err != nil || forgotten != 1 {
		t.Fatalf("31 minutes after the last heartbeat it goes: %d %v", forgotten, err)
	}
	if _, _, _, err := d.userTrackerCredential("u1", "jira"); !errors.Is(err, ErrCredentialLocked) {
		t.Fatalf("forgotten, it is locked: %v", err)
	}
}
