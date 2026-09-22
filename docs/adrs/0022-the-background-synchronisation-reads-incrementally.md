# ADR 0022: The background synchronisation reads incrementally

Status: Accepted

## Context

The auto-synchronisation loop is the one piece of work nobody asks for: every
few minutes, for every project that opted in, it brings Sectile's copy of the
tracker up to date.

Since the per-project settings landed it did that by queueing one `sync_task`
job per unfinished work item, each one a `GetIssue` on the tracker. The comment
at the top of `internal/db/autosync.go` said why, and said what was missing:
a full read costs one request per hundred work items, repeating it every minute
would be rude to the instance, and "the incremental read by JQL bounded on
`updated` that the window below computes is not wired onto the adapter yet".
`autoSyncWindow` had been computing that window since the first version, and
nothing ever read it.

The unit re-read is the more expensive of the two options it was standing in
for. A project of four hundred unfinished tickets costs four hundred requests a
pass where a full read costs four, and every one of those reads wrote an
activity row on the ticket's own card — which is what #343 and the spaced sweep
of closed tickets were built to contain. Both were symptoms of reading one work
item at a time.

## Decision

**A background pass is one synchronisation per project, bounded on the update
date, and the tracker decides what has moved.**

- `tracker.SyncRequest` carries `UpdatedWithinMin`: the read covers what the
  tracker has touched in the last so many minutes. Zero is the whole project,
  which is what a synchronisation somebody asked for always sends.
- **The window is a duration, not an instant.** Jira takes `updated >= -15m`
  directly, dated with the site's own clock, so no timezone has to be agreed on
  and no drift between Sectile and Atlassian can shift it. GitHub's `since`
  wants an instant, and its `updated_at` is UTC, so the adapter converts at the
  edge.
- **A tracker declares whether it can narrow a search**, `CapIncrementalSync`,
  and one that cannot is asked for the whole project whatever the loop wanted.
  That is still one request per hundred work items: the fallback is cheaper than
  the unit re-read it replaces, so there is nothing to keep it for.
- **A full read stays on the calendar**, every half hour (`autoSyncFullEvery`),
  and on the first pass of a project. A read by update date cannot report a work
  item that left the perimeter — reassigned to another project, deleted, or
  moved out of the configured issue types — because the tracker no longer
  returns it at all.
- **What describes the project rather than its work items follows the full
  pass.** The team roster and the board columns cost the same whether one ticket
  moved or none, so a background pass only asks for them on a full read; a
  synchronisation somebody asked for always does. Pull request rediscovery
  follows the imported work items, so an incremental pass pays for exactly what
  changed.
- **A background pass that brought nothing back leaves no activity**, for the
  reason #343 gave: it runs every few minutes on every project, and a row per
  pass buries the entries the feed exists for. A refused pass is always kept,
  with the account whose credential was refused (ADR 0018).

The unit re-read path goes with it: `EnqueueSingleTaskSync`, the `sync_task`
job, and the closed-ticket sweep that existed to spare it. The single-work-item
synchronisation a person triggers on a card is untouched — it is
`ForceSyncSingleTask`, a different path, and it reads the ticket whatever its
state.

## Rejected alternatives

- **Keeping the unit re-read for trackers without an incremental search.** It is
  the expensive option in every case, including that one.
- **An absolute timestamp instead of a window.** JQL would then have to be given
  a time in the site's timezone, which Sectile does not know and which changes
  twice a year. The relative form has no such question.
- **Reading back only to the previous pass, with no overlap and no full pass.**
  A work item changed exactly between two passes falls through, and one that
  leaves the perimeter is never noticed. The overlap costs nothing; the spaced
  full pass costs one read per hundred work items every half hour.
- **Persisting the loop's state across restarts.** The first pass after a start
  is a full one, which is the safe direction to be wrong in.

## Consequences

- The cost of the loop now follows what changes in a project, not how large it
  is. A quiet project of two thousand tickets costs one request a pass.
- A ticket the loop refreshes is no longer refreshed alone: an incremental pass
  writes everything the window returned, so `ImportOrUpdateTasks` runs on a
  handful of work items rather than one.
- The activity feed shows one row per project per pass that found something,
  where it used to show one per ticket per pass.
- The tracker's own notion of "recently updated" now decides what Sectile
  re-reads. A tracker that does not bump `updated` on a change Sectile cares
  about would hide it until the next full pass, half an hour later at worst.
