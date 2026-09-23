package db

import (
	"path/filepath"
	"slices"
	"testing"
)

// TestUpgradeDeletesPersonalGitlabCredentials is migration 3 (#251): a database
// written while GitLab was offered as a tracker loses the personal GitLab
// tokens nothing can use any more, and keeps every other credential.
func TestUpgradeDeletesPersonalGitlabCredentials(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gitlab.db")
	d, err := NewDB(path)
	if err != nil {
		t.Fatalf("creating the database: %v", err)
	}
	for _, tracker := range []string{"gitlab", "github", "jira"} {
		if _, err := d.conn.Exec(
			`INSERT INTO user_tracker_credentials (user_id, tracker, record) VALUES (?, ?, ?)`,
			"usr_1", tracker, []byte("sealed")); err != nil {
			t.Fatalf("storing a %s credential: %v", tracker, err)
		}
	}
	// Back to the version before the migration, as a binary without it left it.
	if _, err := d.conn.Exec(`DELETE FROM schema_migrations WHERE version >= 3`); err != nil {
		t.Fatalf("rewinding the schema version: %v", err)
	}
	d.Close()

	d, err = NewDB(path)
	if err != nil {
		t.Fatalf("reopening the database: %v", err)
	}
	defer d.Close()

	if got := appliedVersions(t, d); !slices.Contains(got, 3) {
		t.Fatalf("applied versions = %v, want 3 among them", got)
	}
	rows, err := d.conn.Query(`SELECT tracker FROM user_tracker_credentials WHERE user_id = ? ORDER BY tracker`, "usr_1")
	if err != nil {
		t.Fatalf("reading the credentials: %v", err)
	}
	defer rows.Close()
	var trackers []string
	for rows.Next() {
		var tracker string
		if err := rows.Scan(&tracker); err != nil {
			t.Fatalf("scanning a credential: %v", err)
		}
		trackers = append(trackers, tracker)
	}
	if !slices.Equal(trackers, []string{"github", "jira"}) {
		t.Fatalf("credentials left = %v, want github and jira only", trackers)
	}
}
