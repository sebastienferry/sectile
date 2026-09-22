package db

import (
	"path/filepath"
	"testing"
)

// A database created before the removal still carries the retired columns and
// the digests table; opening it must leave neither behind.
func TestOpenDBDropsRetiredStorage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	database, err := NewDB(path)
	if err != nil {
		t.Fatal(err)
	}
	// Recreate the legacy shape on the freshly migrated database.
	for _, statement := range []string{
		"ALTER TABLE projects ADD COLUMN linear_team TEXT NOT NULL DEFAULT '';",
		"ALTER TABLE projects ADD COLUMN project_type TEXT NOT NULL DEFAULT 'standard';",
		"ALTER TABLE settings ADD COLUMN linear_team TEXT NOT NULL DEFAULT '';",
		"ALTER TABLE settings ADD COLUMN prompt_digest_agenda TEXT NOT NULL DEFAULT '';",
		"CREATE TABLE IF NOT EXISTS daily_digests (id TEXT PRIMARY KEY, project_id TEXT NOT NULL);",
	} {
		if _, err := database.conn.Exec(statement); err != nil {
			t.Fatalf("%s: %v", statement, err)
		}
	}
	forgetSchemaVersion(t, database)
	database.Close()

	reopened, err := NewDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()

	for _, probe := range []string{
		"SELECT linear_team FROM projects LIMIT 1",
		"SELECT project_type FROM projects LIMIT 1",
		"SELECT linear_team FROM settings LIMIT 1",
		"SELECT prompt_digest_agenda FROM settings LIMIT 1",
		"SELECT id FROM daily_digests LIMIT 1",
	} {
		if _, err := reopened.conn.Query(probe); err == nil {
			t.Errorf("retired storage still readable: %s", probe)
		}
	}
	// The surviving settings row must still load.
	if _, err := reopened.GetSettings(); err != nil {
		t.Fatalf("settings unreadable after the drop: %v", err)
	}
}
