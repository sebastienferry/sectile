package db

import (
	"strings"
	"testing"
)

// planOf returns the plan the engine picks for the per-call wait lookup, as
// text. PostgreSQL is told to avoid sequential scans: on a test table of a few
// rows it would scan whatever the index, and the question is whether the index
// is usable at all.
func planOf(t *testing.T, d *DB, postgres bool) string {
	t.Helper()
	tx, err := d.conn.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	explain := "EXPLAIN QUERY PLAN "
	if postgres {
		if _, err := tx.Exec("SET LOCAL enable_seqscan = off"); err != nil {
			t.Fatal(err)
		}
		explain = "EXPLAIN "
	}
	rows, err := tx.Query(explain+sessionWaitsQuery, "instance.session")
	if err != nil {
		t.Fatalf("explaining the wait lookup: %v", err)
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	var plan strings.Builder
	for rows.Next() {
		values := make([]any, len(columns))
		pointers := make([]any, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			t.Fatal(err)
		}
		for _, v := range values {
			switch v := v.(type) {
			case string:
				plan.WriteString(v)
			case []byte:
				plan.Write(v)
			}
			plan.WriteString(" ")
		}
		plan.WriteString("\n")
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return plan.String()
}

// The lookup every Sectile tool call runs reads the partial index of
// migration 43 instead of scanning every activity (#497).
func TestSessionWaitLookupUsesTheIndex(t *testing.T) {
	batchEngines(t, func(t *testing.T, d *DB) {
		seedProjectAndUser(t, d)
		seedTask(t, d)
		run, err := d.StartAgentRun("t1", "clarify-issue", RunLaunch{UserID: "u1"})
		if err != nil {
			t.Fatal(err)
		}
		if err := d.setRemoteRunWaiting(run.ID, "instance.session", true); err != nil {
			t.Fatal(err)
		}
		postgres := strings.HasSuffix(t.Name(), "/postgres")
		if plan := planOf(t, d, postgres); !strings.Contains(plan, "idx_task_activities_waiting_session") {
			t.Errorf("the wait lookup does not use the index; plan:\n%s", plan)
		}
		cleared, err := d.ResumeWaits("instance.session")
		if err != nil || len(cleared) != 1 || cleared[0] != run.ID {
			t.Errorf("ResumeWaits = %v, %v; want the waiting run", cleared, err)
		}
	})
}
