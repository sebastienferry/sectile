# Plan #675 - A finished run deleted from the activities view stays deleted

## 1. Stack and placement

Go server, `internal/db`. No handler, protocol, web or desktop change.

## 2. Data

Migration 46, `deleted_remote_runs`:

```sql
CREATE TABLE IF NOT EXISTS deleted_remote_runs (
    id TEXT PRIMARY KEY,
    deleted_at TIMESTAMP NOT NULL
);
```

`IF NOT EXISTS` keeps the fixtures that forget schema versions and replay
migrations working on both engines. The table is not in the frozen baseline.

## 3. Writes

`DeleteActivity(id)`, under `d.mu`:

1. `INSERT INTO deleted_remote_runs (id, deleted_at) SELECT id, ? FROM
   task_activities WHERE id = ? AND skill_id = 'remote_run' AND status IN
   ('completed', 'failed', 'canceled') ON CONFLICT (id) DO NOTHING`;
2. purge records older than 30 days;
3. the existing `DELETE`.

`ClearCompletedActivities()` does the same with the `WHERE` of its own
`DELETE`, before it.

Both statements are portable: `INSERT ... SELECT ... ON CONFLICT DO NOTHING`
works on SQLite 3.24+ and PostgreSQL. The purge runs only when something may
have been recorded, which is cheap enough on a table that holds at most what
was deleted in a month. A failure of the record or the purge fails the
deletion, so a deletion never happens without its record.

## 4. Read

`SyncRemoteRunStatusFor`, insert branch: before the `INSERT`, `SELECT 1 FROM
deleted_remote_runs WHERE id = ?`. A hit unlocks and returns `nil, nil`.
`ApplyAgentRunningTasksFor` already skips the broadcast on a nil activity.
The doc comment of `SyncRemoteRunStatus` names the rule.

## 5. Tests

In `internal/db/launcher_run_finished_test.go`, with `batchEngines`:

- `TestDeletedFinishedRunIsNotRecreatedByAgentReports`: the #393 sequence,
  `DeleteActivity`, then `running` and `queued` reports; no row afterwards.
- `TestClearedFinishedRunIsNotRecreatedByAgentReports`: the same through
  `ClearCompletedActivities`.
- `TestUnknownAgentRunIsStillInserted`: a report of a fresh id inserts a
  running, concurrent row owned by the agent's user.
- `TestDeletedRunningRunIsRecreatedByAgentReports`: a run deleted while
  running is recreated.
- `TestDeletedRemoteRunRecordsExpire`: a record dated 31 days ago is purged by
  the next deletion, and the id can be inserted again.

## 6. Changelog

`Fixed`: "A finished run deleted from the activities view no longer comes back
as running while its console is still open (#675)."
