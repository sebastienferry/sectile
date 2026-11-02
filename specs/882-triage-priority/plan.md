# Plan #882 - Triage: add priority

## Stack

Go server (`internal/db`) and the React web client. Sectile Desktop renders
the same `TriageView` and needs no change of its own. No API change, no new
endpoint, no migration: the `priority` column already exists and the task
update already accepts `priority`.

Files touched:

- Changed: `internal/db/db.go` (`ImportOrUpdateTasks`),
  `web/src/components/TriageView.tsx`, `web/src/locales/planning.ts`,
  `CHANGELOG.md`.
- New: `internal/db/syncpriority_test.go`, a triage case in a web browser test
  (see Tests).
- Not touched: the tracker adapters (`internal/trackerapi/mapping.go`,
  `gitlab_mapping.go`, `jira_mapping.go`), which keep returning `medium` for
  GitHub and GitLab and the mapped level for Jira; `internal/db/postback.go`
  and `UpdateTask` in `db.go` (see Sync).

## Sync (`internal/db/db.go`)

`ImportOrUpdateTasks` is the one path that copies a tracker ticket over a
stored row: the full sync (`db.go` ~4994), the project import (~5096) and the
single-ticket refresh (~5465) all go through it. Its update branch (~1260)
sets `priority = ?` unconditionally.

Change: the update branch writes the priority only when the ticket's tracker
carries one.

```go
// trackerCarriesPriority tells whether a ticket of this source has a priority
// of its own on its tracker. GitHub and GitLab have none: their adapters return
// medium, which must not overwrite the level set in Sectile (#882).
func trackerCarriesPriority(source string) bool {
	return source == "jira"
}
```

In the `UPDATE tasks` statement, `priority = ?` becomes
`priority = CASE WHEN ? THEN ? ELSE priority END`, bound to
`trackerCarriesPriority(src)` and `string(t.Priority)`, following the
`CASE WHEN` pattern the statement already uses for its other fields. `src` is
the source the function already derives for the row (`t.Source`, else from the
key prefix), so a ticket with no source (a local one) keeps its stored priority
too. The insert branch is unchanged: a new ticket starts at the value the
adapter gives (`medium` for GitHub and GitLab).

The decision reads the source, never the value: a Jira ticket moved to Medium
in Jira still syncs to `medium`.

Checked and left alone, as the clarification asked:

- `internal/db/postback.go` ~237 and `UpdateTask` (`db.go` ~3319) write
  `existing.Priority`, which starts from the stored row and only changes when
  the request or the postback payload names a priority. They never copy a
  tracker ticket's priority, so they do not reset it.

## Triage column (`web/src/components/TriageView.tsx`)

Same pattern as the List view's priority cell (`ListView.tsx` ~671-690):

- Header: a `<th className="py-2.5 px-3 w-8 text-center">` with
  `strings.columns.priority`, inserted after the title header (~524).
- Cell, after the title cell (~602): a `relative inline-flex` wrapper holding a
  transparent native `<select>` over a `w-2.5 h-2.5 rounded-full` dot coloured
  by `PRIORITY_COLORS[task.priority]`. Options come from `PRIORITY_LEVELS`,
  labelled `t.priority[level]`. The cell stops click propagation like the
  others.
- `onChange` calls `updateTask(task.id, { priority })`. `updateTask` already
  toasts success and failure (`AppContext.tsx` ~2830-2855), including a Jira
  refusal (#679), and the row re-renders from the task list, so a refused
  write leaves the stored priority shown. No local state.
- The select's `title` uses `t.shell.list.priorityChange`
  (`{label} : {value} (Cliquer pour changer)`) with `t.taskModal.priority`, as
  the List view does; no new string for it. The dot's `title` names the level.
- The empty-state row's `colSpan` goes from 8 to 9.

This is a native `<select>` used as a per-row editor, not as a filter: it is
out of the #866 rule "no native `<select>` as a filter".

## Strings (`web/src/locales/planning.ts`)

One key, French and English, in the `triage.columns` catalog that
`TriageView` reads (`t.planning.triage`, ~454 and ~951):

| Key | French | English |
|-----|--------|---------|
| `triage.columns.priority` | `Priorité` | `Priority` |

Level names reuse `t.priority[level]`.

## Tests

- Go, `internal/db/syncpriority_test.go`, table-driven over `github`,
  `gitlab` and `jira`:
  1. import a ticket of that source at `medium`;
  2. set its priority to `high` through `UpdateTask`;
  3. re-import the same ticket carrying `medium`;
  4. expect `high` for GitHub and GitLab, `medium` for Jira.
  Plus: a first import of a GitHub ticket stores `medium`; a re-import still
  writes the other fields (title change visible).
- Web: a browser case on the Triage view (in `view-filters.browser.mjs`, which
  already drives the triage, or a new `triage-priority.browser.mjs` if the
  fixture is easier to isolate): the column header is present after "Title",
  each row has a dot, choosing `high` sends a task update with
  `{ priority: "high" }`, and a refused update leaves the dot on its previous
  colour.
- Locale catalog test (`i18n.test.mjs`) stays green with the new key in both
  languages.

## Changelog

Under `## [Unreleased]`:

- `Added`: the Triage view shows each ticket's priority and lets you change it
  from the row.
- `Fixed`: the priority of a GitHub or GitLab ticket set in Sectile no longer
  resets to medium at each synchronisation.

## Rejected alternatives

- Deciding from the incoming value (`medium` means "no priority"): a Jira
  ticket set to Medium in Jira would never sync back. Rejected in the
  clarification (D6).
- Mirroring the priority to GitHub/GitLab as a `priority:*` label: out of
  scope (round 2).
- A capability method on the tracker service interface instead of a source
  check: only Jira carries a priority, and `ImportOrUpdateTasks` already
  branches on `src`; a new interface method would touch every adapter for one
  boolean.
