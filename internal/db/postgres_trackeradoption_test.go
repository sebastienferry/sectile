package db

import (
	"testing"
	"time"
)

// The adoption's merge runs inside one transaction on PostgreSQL too: the
// typed parameters of its INSERT … SELECT statements, the upserts on the alias
// and the unique index created last are where the two engines could differ.
func TestPostgresAdoptionMergesDuplicateTickets(t *testing.T) {
	d := openPostgres(t)
	first, second := twoProjectsOnPE(t, d)
	now := time.Now().UTC()
	legacyTask(t, d, "jira-a-PE-1", first.ID, "PE-1", "new", now, `["a"]`, `[]`)
	legacyTask(t, d, "jira-b-PE-1", second.ID, "PE-1", "reviewed", now, `["b"]`, `[]`)
	if _, err := d.conn.Exec(`INSERT INTO pinned_tasks (task_id, pinned_at) VALUES ('jira-a-PE-1', '2026-09-01T10:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	adopt(t, d)

	var rows int
	if err := d.conn.QueryRow("SELECT COUNT(*) FROM tasks WHERE key = 'PE-1'").Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("PE-1 rows = %d (%v), want 1", rows, err)
	}
	var survivor string
	if err := d.conn.QueryRow("SELECT task_id FROM task_aliases WHERE old_id = 'jira-a-PE-1'").Scan(&survivor); err != nil || survivor != "jira-b-PE-1" {
		t.Fatalf("alias = %q (%v)", survivor, err)
	}
	var pins int
	_ = d.conn.QueryRow("SELECT COUNT(*) FROM pinned_tasks WHERE task_id = 'jira-b-PE-1'").Scan(&pins)
	if pins != 1 {
		t.Fatalf("the pin did not follow the survivor")
	}
	if indexed, err := d.indexExists(tasksTrackerKeyIndex); err != nil || !indexed {
		t.Fatalf("the unique index is missing: %v", err)
	}
	adopt(t, d)
}

// The rerun after a rollback drops and creates both unique indexes inside the
// adoption's transaction: on PostgreSQL, the restart opens a second connection
// pool on the same database, as a restarted server would, rather than truncate
// it as openPostgres does.
func TestPostgresAdoptionRerunsAfterARollbackWroteAnUntaggedCopy(t *testing.T) {
	d := openPostgres(t)
	adoptedID := plantAnUntaggedCopyAfterARollback(t, d)

	restarted, err := Open(Config{Driver: DriverPostgres, DSN: postgresDSN(t)})
	if err != nil {
		t.Fatalf("the server does not start again: %v", err)
	}
	t.Cleanup(func() { restarted.Close() })
	assertRerunMergedTheUntaggedCopies(t, restarted, adoptedID)
}

// An epic row a rollback wrote alone reruns the adoption on PostgreSQL too: the
// count of the untagged Jira epics joins the projects, their first tracker and
// the macros.
func TestPostgresAdoptionRerunsAfterARollbackWroteAnUntaggedEpicAlone(t *testing.T) {
	d := openPostgres(t)
	plantAnUntaggedEpicAlone(t, d)

	restarted, err := Open(Config{Driver: DriverPostgres, DSN: postgresDSN(t)})
	if err != nil {
		t.Fatalf("the server does not start again: %v", err)
	}
	t.Cleanup(func() { restarted.Close() })
	assertRerunMergedTheUntaggedEpic(t, restarted)
}

// A deleted project's local tickets reach the default project's local board,
// created and linked by an INSERT … SELECT whose parameters PostgreSQL types.
func TestPostgresDeletingAProjectGivesADefaultProjectOnATrackerALocalBoard(t *testing.T) {
	checkDeletingAProjectGivesADefaultProjectOnATrackerALocalBoard(t, openPostgres(t))
}

func TestPostgresDeletingAProjectRekeysTheLocalTicketsTheDefaultBoardAlreadyHolds(t *testing.T) {
	checkDeletingAProjectRekeysTheLocalTicketsTheDefaultBoardAlreadyHolds(t, openPostgres(t))
}

// The local board kept after a switch to Jira moves after the new tracker by
// an UPDATE whose subquery reads the same table, which PostgreSQL types too.
func TestPostgresALocalProjectSwitchedToJiraKeepsItsLocalBoardAfterTheNewTracker(t *testing.T) {
	checkALocalProjectSwitchedToJiraKeepsItsLocalBoardAfterTheNewTracker(t, openPostgres(t))
}
