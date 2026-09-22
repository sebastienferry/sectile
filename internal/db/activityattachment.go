package db

import (
	"fmt"
	"log"
)

// taskActivitiesSchema declares the activity table, parameterised by its name.
//
// The name is a parameter because the SQLite path to the current schema is a
// table rebuild: it creates a copy under another name, fills it and renames it.
// Writing the declaration once means the rebuilt table cannot drift from the
// one a fresh database is born with.
func taskActivitiesSchema(table string) string {
	return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			id TEXT PRIMARY KEY,
			task_id TEXT REFERENCES tasks(id),
			project_id TEXT REFERENCES projects(id) ON DELETE CASCADE,
			skill_id TEXT NOT NULL,
			skill_name TEXT NOT NULL,
			action TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'completed',
			summary TEXT NOT NULL DEFAULT '',
			output TEXT NOT NULL DEFAULT '',
			steps TEXT NOT NULL DEFAULT '[]',
			prompt TEXT NOT NULL DEFAULT '',
			started_at DATETIME,
			completed_at DATETIME,
			error TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			user_id TEXT NOT NULL DEFAULT '',
			run_provider TEXT NOT NULL DEFAULT '',
			run_model TEXT NOT NULL DEFAULT '',
			run_mode TEXT NOT NULL DEFAULT '',
			launch_stage TEXT NOT NULL DEFAULT '',
			chain_stop_stage TEXT NOT NULL DEFAULT '',
			waiting_since DATETIME,
			-- An activity has exactly one of three attachments: a ticket
			-- (task_id), a project (project_id), or nothing at all — a
			-- synchronisation of every tracker belongs to no single project.
			-- The CHECK is what keeps that from decaying back into a convention:
			-- before it, a project activity was filed under a made-up
			-- "sync-<project>" task_id, which PostgreSQL rejected and SQLite
			-- accepted, so a synchronisation ran and left no trace at all.
			--
			-- Deleting a task still deletes its activities explicitly (see
			-- DeleteTask), and deleting a project does the same (DeleteProject),
			-- because SQLite enforces no foreign key unless asked and this
			-- package does not ask. The ON DELETE CASCADE is what PostgreSQL
			-- relies on.
			CHECK (task_id IS NULL OR project_id IS NULL)
		);`, table)
}

// taskActivityColumns is what the SQLite rebuild carries across. project_id is
// absent on purpose: the source table predates it.
const taskActivityColumns = `id, task_id, skill_id, skill_name, action, status, summary, output, steps, prompt,
	started_at, completed_at, error, created_at, user_id, run_provider, run_model, run_mode, launch_stage,
	chain_stop_stage, waiting_since`

// backfillActivityAttachment moves the old synthetic identifiers out of
// task_id, without deleting a single row.
//
// Until #310 a project activity was filed under "sync-<projectID>" and a
// tracker-wide or global one under "sync-all" / "sync-github" / "sync-jira".
// The first kind becomes a project activity; everything else becomes a global
// activity, which is what it always was. It is engine-independent on purpose —
// a PostgreSQL database created since #304 holds those rows too — and each
// statement is idempotent, because after one pass no task_id starts with
// "sync-" any more.
func backfillActivityAttachment(conn *sqlConn) error {
	for _, step := range []struct {
		what string
		stmt string
	}{
		{
			// A suffix naming a project it still has: a project activity.
			"sync-<project> rows moved onto project_id",
			`UPDATE task_activities
			    SET project_id = SUBSTR(task_id, 6), task_id = NULL
			  WHERE task_id LIKE 'sync-%'
			    AND SUBSTR(task_id, 6) IN (SELECT id FROM projects)`,
		},
		{
			// The same convention grew two more prefixes: the installation of a
			// specification framework on a project, and a tracker write that
			// covers a batch of tickets rather than one.
			"spec-framework-<project> rows moved onto project_id",
			`UPDATE task_activities
			    SET project_id = SUBSTR(task_id, 16), task_id = NULL
			  WHERE task_id LIKE 'spec-framework-%'
			    AND SUBSTR(task_id, 16) IN (SELECT id FROM projects)`,
		},
		{
			"tracker-op-<project> rows moved onto project_id",
			`UPDATE task_activities
			    SET project_id = SUBSTR(task_id, 12), task_id = NULL
			  WHERE task_id LIKE 'tracker-op-%'
			    AND SUBSTR(task_id, 12) IN (SELECT id FROM projects)`,
		},
		{
			// sync-all, sync-github, sync-jira, and any project since deleted.
			// The other two prefixes fall to the repair below.
			"remaining sync-* rows turned into global activities",
			`UPDATE task_activities SET task_id = NULL WHERE task_id LIKE 'sync-%'`,
		},
		{
			// Belt and braces: anything else that is not a task. The foreign
			// key cannot be created while one of these is left.
			"task_id values matching no task cleared",
			`UPDATE task_activities SET task_id = NULL
			  WHERE task_id IS NOT NULL AND task_id NOT IN (SELECT id FROM tasks)`,
		},
	} {
		res, err := conn.Exec(step.stmt)
		if err != nil {
			return fmt.Errorf("%s: %w", step.what, err)
		}
		if n, err := res.RowsAffected(); err == nil && n > 0 {
			log.Printf("[task_activities] %s: %d", step.what, n)
		}
	}
	return nil
}

// hasColumn reports whether a table already declares a column, through the
// catalogue query the dialect knows how to spell.
func hasColumn(conn *sqlConn, table, column string) (bool, error) {
	rows, err := conn.Query(conn.dialect.ColumnsQuery(), table)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}
