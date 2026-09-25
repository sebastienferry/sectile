package db

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"tasks/internal/models"
)

// seedSearch writes one project and the tickets the search tests look for, each
// matching through a single field.
func seedSearch(t *testing.T, d *DB) {
	t.Helper()
	statements := []struct {
		query string
		args  []interface{}
	}{
		{`INSERT INTO projects (id, name, slug) VALUES ('p1', 'P', 'p')`, nil},
		{`INSERT INTO tasks (id, project_id, key, title) VALUES (?, 'p1', ?, ?)`, []interface{}{"bug", "#1", "Fix Bug in sync"}},
		{`INSERT INTO tasks (id, project_id, key, title) VALUES (?, 'p1', ?, ?)`, []interface{}{"equipe-accent", "#2", "Équipe plateforme"}},
		{`INSERT INTO tasks (id, project_id, key, title) VALUES (?, 'p1', ?, ?)`, []interface{}{"equipe-plain", "#3", "equipe mobile"}},
		{`INSERT INTO tasks (id, project_id, key, title, parent_title) VALUES (?, 'p1', ?, ?, ?)`, []interface{}{"child", "#4", "Header", "Refonte Écran"}},
		{`INSERT INTO tasks (id, project_id, key, title, description) VALUES (?, 'p1', ?, ?, ?)`, []interface{}{"network", "#5", "Outage", "Le Réseau tombe"}},
		{`INSERT INTO tasks (id, project_id, key, title) VALUES (?, 'p1', ?, ?)`, []interface{}{"percent", "#6", "Réduire de 50%"}},
		{`INSERT INTO tasks (id, project_id, key, title) VALUES (?, 'p1', ?, ?)`, []interface{}{"millis", "#7", "Réduire de 500 ms"}},
		{`INSERT INTO tasks (id, project_id, key, title) VALUES (?, 'p1', ?, ?)`, []interface{}{"underscore", "#8", "a_b"}},
		{`INSERT INTO tasks (id, project_id, key, title) VALUES (?, 'p1', ?, ?)`, []interface{}{"any", "#9", "axb"}},
		{`INSERT INTO tasks (id, project_id, key, title) VALUES (?, 'p1', ?, ?)`, []interface{}{"bang", "#10", "Stop!now"}},
	}
	for _, s := range statements {
		if _, err := d.conn.Exec(s.query, s.args...); err != nil {
			t.Fatalf("seeding (%s): %v", s.query, err)
		}
	}
}

// searchIDs is the ids of the tickets the header search returns for query.
func searchIDs(t *testing.T, d *DB, query string) []string {
	t.Helper()
	tasks, err := d.GetTasks(query, "", "", "", "", "", "", "", "", nil, nil, false)
	if err != nil {
		t.Fatalf("GetTasks(%q): %v", query, err)
	}
	ids := make([]string, 0, len(tasks))
	for _, task := range tasks {
		ids = append(ids, task.ID)
	}
	slices.Sort(ids)
	return ids
}

// assertFound fails unless every query returns want among its results.
func assertFound(t *testing.T, d *DB, want string, queries ...string) {
	t.Helper()
	for _, q := range queries {
		if got := searchIDs(t, d, q); !slices.Contains(got, want) {
			t.Errorf("search %q = %v, want %q among them", q, got, want)
		}
	}
}

// assertExactly fails unless query returns want and nothing else.
func assertExactly(t *testing.T, d *DB, query string, want ...string) {
	t.Helper()
	slices.Sort(want)
	if got := searchIDs(t, d, query); !slices.Equal(got, want) {
		t.Errorf("search %q = %v, want exactly %v", query, got, want)
	}
}

func TestPostgresTaskSearchIgnoresCaseAndAccents(t *testing.T) {
	d := openPostgres(t)
	seedSearch(t, d)

	assertFound(t, d, "bug", "bug", "Bug", "BUG")
	assertFound(t, d, "equipe-accent", "equipe", "Equipe", "équipe", "ÉQUIPE")
	assertFound(t, d, "equipe-plain", "ÉQUIPE", "Équipe")
	assertFound(t, d, "child", "ecran")
	assertExactly(t, d, "RESEAU", "network")
}

func TestPostgresTaskSearchTreatsWildcardsLiterally(t *testing.T) {
	d := openPostgres(t)
	seedSearch(t, d)

	assertExactly(t, d, "50%", "percent")
	assertExactly(t, d, "a_b", "underscore")
	assertExactly(t, d, "p!n", "bang")
	assertExactly(t, d, "P!NOW", "bang")
}

// TestSQLiteTaskSearchKeepsItsFold pins what SQLite still does: A-Z case is
// ignored, accents are not, and wildcards typed in the query are literal there
// too.
func TestSQLiteTaskSearchKeepsItsFold(t *testing.T) {
	d, err := NewDB(filepath.Join(t.TempDir(), "search.db"))
	if err != nil {
		t.Fatalf("creating the database: %v", err)
	}
	defer d.Close()
	if _, err := d.conn.Exec(`DELETE FROM projects`); err != nil {
		t.Fatalf("clearing projects: %v", err)
	}
	seedSearch(t, d)

	assertFound(t, d, "bug", "bug", "Bug", "BUG")
	assertExactly(t, d, "equipe", "equipe-plain")
	assertExactly(t, d, "50%", "percent")
	assertExactly(t, d, "a_b", "underscore")
	assertExactly(t, d, "P!NOW", "bang")
}

func TestPostgresActivitySearchIgnoresCaseAndAccents(t *testing.T) {
	d := openPostgres(t)
	for _, stmt := range []string{
		`INSERT INTO projects (id, name, slug) VALUES ('p1', 'P', 'p')`,
		`INSERT INTO tasks (id, project_id, key, title) VALUES ('t1', 'p1', 'K-1', 'Écran d''accueil')`,
	} {
		if _, err := d.conn.Exec(stmt); err != nil {
			t.Fatalf("seeding (%s): %v", stmt, err)
		}
	}
	now := time.Now()
	for _, act := range []models.TaskActivity{
		{ID: "on-task", TaskID: "t1", SkillID: "clarify", SkillName: "Clarify", Action: "Clarify", Summary: "Clarification terminée", CreatedAt: now},
		{ID: "on-project", ProjectID: "p1", SkillID: "sync_github", SkillName: "Sync", Action: "Sync", Summary: "Synchronisation terminée", CreatedAt: now},
		{ID: "unrelated", ProjectID: "p1", SkillID: "sync_jira", SkillName: "Sync", Action: "Sync", Summary: "En cours", CreatedAt: now},
	} {
		if err := d.AddTaskActivity(act); err != nil {
			t.Fatalf("recording %s: %v", act.ID, err)
		}
	}

	activityIDs := func(search string) []string {
		list, err := d.GetActivities("", "", "", "", search, 0)
		if err != nil {
			t.Fatalf("GetActivities(%q): %v", search, err)
		}
		ids := make([]string, 0, len(list))
		for _, a := range list {
			ids = append(ids, a.ID)
		}
		slices.Sort(ids)
		return ids
	}
	// The activity with no task is still found by its summary: the task
	// columns are NULL for it and simply do not match.
	if got, want := activityIDs("TERMINEE"), []string{"on-project", "on-task"}; !slices.Equal(got, want) {
		t.Errorf("search TERMINEE = %v, want %v", got, want)
	}
	if got, want := activityIDs("ecran"), []string{"on-task"}; !slices.Equal(got, want) {
		t.Errorf("search ecran = %v, want %v through the task title", got, want)
	}
}

func TestPostgresUnaccentExtensionIsInstalled(t *testing.T) {
	d := openPostgres(t)
	var n int
	if err := d.conn.QueryRow(`SELECT COUNT(*) FROM pg_extension WHERE extname = 'unaccent'`).Scan(&n); err != nil {
		t.Fatalf("reading pg_extension: %v", err)
	}
	if n != 1 {
		t.Fatalf("unaccent installed %d time(s), want 1", n)
	}
}

// TestAFailedMigrationCarriesItsHint is what an operator reads when the server
// refuses to start on a database whose role cannot create unaccent: the hint
// names the extension, not only the failing statement.
func TestAFailedMigrationCarriesItsHint(t *testing.T) {
	d, err := NewDB(filepath.Join(t.TempDir(), "hint.db"))
	if err != nil {
		t.Fatalf("creating the database: %v", err)
	}
	defer d.Close()

	startVersion := latestVersion()
	list := []migration{{
		version:    startVersion + 1,
		name:       "test.hinted",
		statements: []string{"CREATE TABLE tasks (id TEXT);"}, // exists already: refused
		hint:       `the PostgreSQL extension "unaccent" could not be created`,
	}}
	err = d.applyMigrations(list, startVersion)
	if err == nil {
		t.Fatal("a migration whose statement is refused reported success")
	}
	if !strings.Contains(err.Error(), `"unaccent"`) || !strings.Contains(err.Error(), "test.hinted") {
		t.Fatalf("error = %q, want the migration name and its hint", err)
	}
	if errors.Unwrap(err) == nil {
		t.Fatalf("error = %q does not wrap the engine's error", err)
	}
}

// TestUnaccentMigrationNamesTheExtension guards the hint on the real migration,
// which only PostgreSQL runs.
func TestUnaccentMigrationNamesTheExtension(t *testing.T) {
	for _, m := range migrations {
		if m.name != "extension.unaccent" {
			continue
		}
		if len(m.statements) != 0 || len(m.sqlite) != 0 {
			t.Errorf("migration %d runs statements on SQLite, which has no unaccent", m.version)
		}
		if !strings.Contains(m.hint, `"unaccent"`) {
			t.Errorf("migration %d hint = %q, want it to name unaccent", m.version, m.hint)
		}
		return
	}
	t.Fatal("no extension.unaccent migration")
}

// TestSearchPredicateFoldsBothSides pins the SQL on both engines: the column and
// the pattern go through the same fold, and wildcards are escaped.
func TestSearchPredicateFoldsBothSides(t *testing.T) {
	cases := []struct {
		engine string
		d      *DB
		want   string
	}{
		{"SQLite", &DB{dialect: sqliteDialect{}}, "(LOWER(title) LIKE LOWER(?) ESCAPE '!' OR LOWER(t.key) LIKE LOWER(?) ESCAPE '!')"},
		{"PostgreSQL", &DB{dialect: postgresDialect{}}, `(LOWER(unaccent(title) COLLATE "C") LIKE LOWER(unaccent(?) COLLATE "C") ESCAPE '!' OR LOWER(unaccent(t.key) COLLATE "C") LIKE LOWER(unaccent(?) COLLATE "C") ESCAPE '!')`},
	}
	for _, c := range cases {
		cond, args := c.d.searchPredicate("50%_!", "title", "t.key")
		if cond != c.want {
			t.Errorf("%s condition = %q, want %q", c.engine, cond, c.want)
		}
		if len(args) != 2 || args[0] != "%50!%!_!!%" || args[1] != args[0] {
			t.Errorf("%s args = %#v, want the escaped pattern once per column", c.engine, args)
		}
	}
}
