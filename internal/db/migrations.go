package db

import (
	"fmt"
	"time"
)

// baselineVersion is the schema this package built in one pass, before changes
// were numbered: every `CREATE TABLE`, every historical `ALTER`, and the columns
// declared after PostgreSQL support shipped.
//
// It is a frozen snapshot. The statements that produce it are never edited
// again, whatever a later version needs; a later version writes a migration.
// See docs/adrs/0021.
const baselineVersion = 1

// migration is one numbered schema change, applied exactly once per database
// and recorded when it is.
//
// That "exactly once" is the whole point, and it is what the scheme before this
// one could not offer: a migration is ordinary SQL, not an `IF NOT EXISTS`
// hedged against having run before, not an error deliberately ignored.
//
// statements is the shared form, and is what almost every migration uses: the
// two engines disagree on two type names, which RewriteDDL already translates.
// sqlite and postgres override it on the engine they name, for the changes the
// engines genuinely cannot express the same way. SQLite can neither relax a
// NOT NULL, nor add a foreign key, nor add a CHECK through ALTER TABLE, so a
// migration doing any of those carries both forms.
//
// Each entry is one statement. Never a script: pgx refuses a multi-statement
// Exec under its extended protocol.
type migration struct {
	version    int
	name       string
	statements []string
	sqlite     []string
	postgres   []string
}

// statementsFor picks the form this engine runs.
func (m migration) statementsFor(engine Driver) []string {
	if engine == DriverPostgres {
		if len(m.postgres) > 0 {
			return m.postgres
		}
		return m.statements
	}
	if len(m.sqlite) > 0 {
		return m.sqlite
	}
	return m.statements
}

// migrations lists every change made since the baseline, in order.
//
// It is empty because the baseline is the schema as it stands: this list starts
// filling with the next change. A new column goes here, and nowhere else. Not
// in a CREATE TABLE, which now describes version 1 and not the current schema;
// not in applyLegacyMigrations, which repairs SQLite files written before the
// baseline; not in lateColumns, which is frozen for the same reason.
//
//	{
//	    version:    2,
//	    name:       "tasks.estimate",
//	    statements: []string{"ALTER TABLE tasks ADD COLUMN estimate INTEGER NOT NULL DEFAULT 0;"},
//	},
//
// Versions are contiguous from baselineVersion + 1 and never reordered or
// renumbered once merged: a database that has applied 2 and 3 decides what to
var migrations = []migration{
	{
		version: 2,
		name:    "tasks.creator",
		statements: []string{
			"ALTER TABLE tasks ADD COLUMN creator TEXT NOT NULL DEFAULT '';",
			"ALTER TABLE tasks ADD COLUMN creator_avatar TEXT NOT NULL DEFAULT '';",
		},
	},
	{
		// Cards carry their epic's colour only on the projects that ask for it,
		// so every existing project starts with it off.
		version: 3,
		name:    "projects.epic_colors",
		statements: []string{
			"ALTER TABLE projects ADD COLUMN epic_colors INTEGER NOT NULL DEFAULT 0;",
		},
	},
	{
		// Saved board views (#387): a personal, named selection of projects and
		// labels laid over the all-projects board. Both lists are JSON arrays in
		// TEXT, like tasks.labels and projects.enabled_views: they are read whole
		// and never queried by element.
		version: 4,
		name:    "board_views",
		statements: []string{
			`CREATE TABLE IF NOT EXISTS board_views (
				id TEXT PRIMARY KEY,
				user_id TEXT NOT NULL,
				name TEXT NOT NULL,
				name_key TEXT NOT NULL,
				project_ids TEXT NOT NULL DEFAULT '[]',
				labels TEXT NOT NULL DEFAULT '[]',
				created_at DATETIME NOT NULL,
				updated_at DATETIME NOT NULL
			);`,
			"CREATE UNIQUE INDEX IF NOT EXISTS idx_board_views_user_name ON board_views (user_id, name_key);",
			"CREATE INDEX IF NOT EXISTS idx_board_views_user ON board_views (user_id);",
		},
	},
	{
		// The server process owning a piece of work, so that one instance
		// starting does not reclaim what another live instance runs. See
		// internal/db/instances.go.
		version: 5,
		name:    "server_instances",
		statements: []string{
			"ALTER TABLE task_activities ADD COLUMN instance_id TEXT NOT NULL DEFAULT '';",
			`CREATE TABLE server_instances (
				id TEXT PRIMARY KEY,
				hostname TEXT NOT NULL DEFAULT '',
				pid INTEGER NOT NULL DEFAULT 0,
				started_at DATETIME NOT NULL,
				last_seen DATETIME NOT NULL
			);`,
		},
	},
}

// migrateSchema brings the database to the schema this binary expects, and is
// the only path that may change it.
//
// A database with no version row has never been seen by this scheme: it is
// either empty or was written by an earlier binary, and the baseline covers
// both. Everything after the baseline is a numbered migration applied in order.
//
// An error here stops the server. That is deliberate, and it is a change from
// the scheme this replaces, where a failed repair was logged and startup
// carried on: tolerable for a repair that was optional anyway, not for a
// numbered change, because the alternative is answering requests against a
// schema the code does not have.
func (d *DB) migrateSchema() error {
	unlock, err := d.dialect.LockForMigration(d.conn)
	if err != nil {
		return fmt.Errorf("taking the migration lock: %w", err)
	}
	defer unlock()

	if err := d.ensureSchemaMigrationsTable(); err != nil {
		return err
	}
	current, err := d.schemaVersion()
	if err != nil {
		return err
	}
	if current == 0 {
		if err := d.applyBaseline(); err != nil {
			return err
		}
		current = baselineVersion
	}
	return d.applyMigrations(migrations, current)
}

// ensureSchemaMigrationsTable creates the record of what has been applied. It
// is the one piece of schema that cannot itself be a migration.
func (d *DB) ensureSchemaMigrationsTable() error {
	_, err := d.conn.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		applied_at DATETIME NOT NULL
	);`)
	if err != nil {
		return fmt.Errorf("schema_migrations: %w", err)
	}
	return nil
}

// schemaVersion is the highest version the database has applied, or 0 when it
// has applied none.
func (d *DB) schemaVersion() (int, error) {
	var version int
	if err := d.conn.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&version); err != nil {
		return 0, fmt.Errorf("reading the schema version: %w", err)
	}
	return version, nil
}

// applyBaseline builds version 1 and records it.
//
// It is the startup path this package had before migrations were numbered,
// called once instead of on every start. Both populations converge here: a
// SQLite file at any point of its history, a PostgreSQL database at whatever
// schema an earlier binary left it, and a database created from nothing.
//
// It is the one step that is idempotent rather than transactional, and it has to
// stay that way: it rebuilds tables and toggles `PRAGMA foreign_keys`, which
// SQLite will not do inside a transaction. An attempt that stops halfway is
// therefore resumed in full on the next start, exactly as it was before, because
// the version is stamped only once every statement has run.
func (d *DB) applyBaseline() error {
	if err := d.initIdentitySchema(); err != nil {
		return err
	}
	if err := d.initSessionSchema(); err != nil {
		return err
	}
	if err := d.initSchema(); err != nil {
		return fmt.Errorf("failed to initialize schema: %w", err)
	}
	d.ensureUserCredentialsTable()
	// Tables that used to be created on first use. Lazy creation works, but it
	// leaves a freshly created database incomplete until something happens to
	// touch each one, which a migration into it discovers the hard way. They
	// are created here, from their own definitions, so the schema is whole the
	// moment the server is up.
	d.ensureCommentsTable()
	d.ensureTeamsTables()
	d.ensureProjectSkillsTable()
	d.ensureMacrosTable()

	// Every table now exists, so the columns an older database is missing can be
	// added whichever table they belong to. Only the engines without the legacy
	// migrations need it; under SQLite the ALTER path has already put every one
	// of them back. See lateColumns, which is frozen along with the rest of the
	// baseline: it is what carries a PostgreSQL database created before #337 up
	// to version 1, so it cannot be deleted, and nothing may be added to it.
	if !d.dialect.RunsLegacyMigrations() {
		d.reconcileLateColumns()
	}
	return d.stamp(d.conn, baselineVersion, "baseline")
}

// applyMigrations runs everything newer than the version the database carries.
func (d *DB) applyMigrations(list []migration, current int) error {
	for _, m := range list {
		if m.version <= current {
			continue
		}
		if err := d.applyMigration(m); err != nil {
			return fmt.Errorf("migration %d (%s): %w", m.version, m.name, err)
		}
	}
	return nil
}

// applyMigration runs one migration and records it, both in one transaction.
//
// The version row is written inside that transaction on purpose: a migration
// that stops halfway leaves the database exactly at the version before it,
// which is the entire reason to number them. Without it, a partial migration
// would be indistinguishable from a complete one on the next start.
func (d *DB) applyMigration(m migration) error {
	tx, err := d.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, statement := range m.statementsFor(d.dialect.Engine()) {
		if _, err := tx.Exec(statement); err != nil {
			return fmt.Errorf("%s: %w", statement, err)
		}
	}
	if err := d.stamp(tx, m.version, m.name); err != nil {
		return err
	}
	return tx.Commit()
}

// stamp records an applied version, through the connection or the transaction
// that applied it.
func (d *DB) stamp(exec execer, version int, name string) error {
	if _, err := exec.Exec(
		`INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)`,
		version, name, time.Now().UTC()); err != nil {
		return fmt.Errorf("recording version %d: %w", version, err)
	}
	return nil
}
