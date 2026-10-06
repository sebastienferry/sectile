# ADR 0050: Trackers are server sources and projects select their tickets by label

- Status: Accepted
- Date: 2026-10-05
- Issue: [#741](https://github.com/sebastienferry/sectile/issues/741)

Amends [ADR 0018](0018-the-admin-owns-the-roster-not-the-board.md),
[ADR 0025](0025-saved-board-views-are-personal-overlays.md) and
[ADR 0043](0043-declared-roadmap-projects-are-read-except-new-stories-and-opted-in-axes.md).

## Context

A Sectile project was a tracker scope. It named one Jira space, one GitHub
repository or one GitLab project, kept a mirror of that tracker's board (the
board, its columns, the status-to-stage mapping, the sprints and the issue
types imported) and owned every ticket imported from it. A ticket's local
identity included its project.

Two projects pointing at the same Jira space therefore each imported their own
copy of every ticket. The copies drifted apart: each had its own stage, its own
pull requests and its own activity history, and moving one on the board left
the other where it was. ADR 0025 recorded the consequence on saved views ("a
remote story synchronised by two projects of a view shows twice") and left the
fix to "how projects share a tracker scope". The work the owner wanted to model
does not fit one tracker per project either: the delivery-admin application's
tickets come from the GODE and BE Jira spaces, and the bidderAdmin
application's from GODE alone.

## Decision

**A project is an application, and a tracker is a server-side source.** A
project keeps its repositories, rules and skills. A tracker is one Jira space,
one GitHub repository, one GitLab project, or the local board of one project.
It is synchronised in full, whatever projects exist, and a project shows the
subset of its trackers' tickets that carries its label.

- **A tracker is a record of its own.** The `trackers` table holds the
  provider, the site (empty when the tracker uses the deployment's own), the
  scope (a Jira key, a GitHub `owner/repo`, a GitLab path, or the project id of
  a local board) and an `identity` derived from the three, unique, on the
  pattern of the repository identity of
  [ADR 0028](0028-repositories-are-keyed-by-remote.md). GODE and BE are two
  trackers even on one Jira site. The tracker also holds the board mirror a
  project held before (board, columns, status-to-stage mapping, sprints, issue
  types) and the background synchronisation settings, so the status-to-stage
  mapping is now one per tracker.
- **A project selects N trackers and one optional label.** `project_trackers`
  links them in order. The project's `defaultTrackerId` is where its new
  tickets go: the first tracker unless it names another, and the first
  remaining one when its default is unlinked. A local tracker belongs to its
  own project and is never accepted from another.
- **One record per remote ticket, owned by its tracker.** A ticket carries
  `tracker_id`, and `(tracker_id, key)` is unique. A ticket carrying two
  project labels shows in both projects with one stage, one set of pull
  requests and one activity history.
- **Membership is computed, never stored.** A ticket belongs to a project when
  the project selects its tracker and either has no label or the ticket carries
  that label, compared whole and regardless of ASCII case with the predicate of
  ADR 0025. A project with no label shows every ticket of its trackers, which
  is how every existing project migrates. The same rule exists in SQL, for the
  listings, and in Go, for the run's project, the agent configuration and each
  ticket's `projectIds`; a test keeps the two equal.
- **A ticket in no project waits in its tracker's backlog.** A member can give
  it a project's label from there. The label is added locally at once, so the
  ticket joins the project immediately, and written back on the tracker through
  the existing label write, with the member's own credential.
- **Creation goes to a tracker of the project.** A new ticket goes to the
  project's default tracker, or to another of its trackers the web form or MCP
  `create_task` names. The project label is added next to `#new`.
- **Jira epics are tracker records too.** An epic shows in a project when it
  carries the project label or when one of its children belongs to the project.
  The local `M-<n>` macros of GitHub and GitLab projects stay owned by their
  project.
- **A run works for one project, recorded on the run.** A run started from a
  project board, or given a `projectId`, works for that project, which must be
  one of the ticket's. A ticket in one project needs no choice. A ticket in
  several projects launched without one is asked about when the launch is
  interactive, and refused, its candidate projects listed, when it is
  unattended. A ticket in no project cannot be run until it is labelled into
  one. The chosen project is stored in `task_activities.run_project_id`, and
  every operation made from inside the run, the agent configuration and the
  repository worktrees follow it.
- **The synchronisation runs per tracker.** The background loop, its pacing
  (`auto_sync_trackers`) and the manual passes work on trackers. A pass asked
  for a project queues one pass per tracker of the project.
- **`tasks.project_id` stays, unused (D1).** Its inline `UNIQUE(project_id,
  key)` constraint cannot be dropped without a SQLite table rebuild. Ticket and
  epic rows written since this decision carry the sentinel
  `tracker:<tracker id>`, which keeps that constraint safe for two GitHub
  trackers of one project that both have `#12`. Nothing reads the column to
  decide which project a ticket belongs to; `Task.projectId` in the API is
  computed. A "home project" was rejected: it would be a second source of truth,
  and no project is the principled home when two unlabelled projects share a
  tracker.
- **No task id is rewritten (D3).** Existing rows keep their ids and gain
  `tracker_id`; new imports format ids with the tracker id. When the migration
  merges two copies of a ticket, the merged-away id is kept in `task_aliases`,
  so a link or an agent holding it still resolves. A key that two
  trackers carry is an error rather than the first match.
- **Adoption is an idempotent step at start.** Migration 47 only adds the
  tables and columns. `adoptTrackers()` then runs under the migration lock: it
  creates one tracker per identity the projects name (the first project of an
  identity giving its board mirror, auto-sync on if any project had it, with
  the smaller interval), gives every local project its own local tracker, tags
  every ticket, merges the duplicate tickets of a shared tracker (the most
  advanced stage survives, pull request links, labels and changed repositories
  are united, activities, comments, pins and batch members move to the
  survivor) and creates the unique index `ux_tasks_tracker_key`.
  `adoptTrackerEpics()` merges the duplicate rows of a Jira epic (the most
  recently updated wins and keeps the other's todos when it has none) and
  creates `ux_macros_tracker_key`. Both do nothing once done.
- **Tracker configuration is an admin's (D11).** The trackers, their source,
  board, columns, status-to-stage mapping, issue types and background
  synchronisation are configured from Administration (`/api/admin/trackers`).
  A member chooses a project's trackers, its default tracker and its label,
  reads a tracker's backlog and asks for a synchronisation (`/api/trackers`).

## Consequences

- **Admin-only tracker configuration is enforced on the server.** It reverses
  ADR 0018, which made "configuring the tracker [a project] reads from" a
  member's, and the comment `adminOnlyRoute` carried: the admin tracker routes
  are in the admin-only table, and a member's project creation or update that
  still carries `boardId`, `trackerColumns`, `stageColumns`, `issueTypes`,
  `autoSyncEnabled` or `autoSyncIntervalMin` is saved without them rather than
  refused, so an older client keeps saving the rest. An admin's still writes
  them through to the project's default tracker. The sprints are the
  exception: they stay a member's, through the sprint routes and the project
  save, because planning sprints is board work. The deployment's tracker
  settings keys of ADR 0018 (`trackerSettingsKeys`) are unchanged.
- **A member can still point a project at a tracker by its legacy fields.** A
  project save naming `issueTracker` and `jiraProject`, `githubRepo` or
  `gitlabProject` finds or creates the tracker of that identity and links it,
  without its board mirror; a tracker only that project selects is renamed in
  place to the new source, so its tickets stay with it. Configuring the new
  tracker's board is then an admin's.
- **Members lose the status-to-stage mapping editor** they had in a project's
  settings. The mapping belongs to the tracker, so only an admin edits it, in
  Administration → Trackers.
- **`tasks.project_id` is unused** and holds the `tracker:<id>` sentinel on new
  rows. **The old tracker columns of `projects` are kept** (issue tracker, Jira
  project, GitHub repository, GitLab project, board, columns, mapping, sprints,
  issue types, auto-sync): adoption reads them and a rollback needs them. A
  later migration drops them; until then a project's loaded tracker fields are
  read through from its default tracker.
- **A run on a ticket of several projects needs a project.** An interactive
  launch without one answers `409 {error, candidates, unattended: false}`, and
  the web and Desktop ask which project and retry with it. An unattended launch
  is refused with `400 {error, candidates, unattended: true}`. A launch is
  unattended when it explicitly asks for `mode: "autonomous"`, launches a batch
  (`batchTaskIds`), or runs the full chain (`advance` with `auto`); a project
  whose default mode is autonomous does not make a launch without a mode
  unattended, so it gets the interactive `409`. MCP `start_run` refuses the
  same cases in English with the candidate projects, so the calling agent asks
  its user. The agent sends the dispatched project with its configuration
  request, so the agent should be upgraded with the server.
- **ADR 0043 roadmap projects overlap with multi-tracker projects.** A Jira
  project's declared roadmap projects stay a read-only source of epics, as
  ADR 0043 describes, beside the trackers the project now selects its tickets
  from. Reconciling the two is out of scope.
- **The board's label filter keeps substring matching.** The `label` filter of
  the task list is folded for ASCII case with its wildcards escaped, but still
  matches a part of a label, as the board always did; only membership and saved
  views compare whole labels.
- **The upgrade needs a stop-then-start deploy, not a rolling one.** Migration
  47 and the adoption steps run at server start, under the migration lock.
  During a rolling deploy on PostgreSQL, an old replica keeps synchronising per
  project, writes tickets without a tracker and can re-create a duplicate the
  new replica merged; the adoption only runs at start, so those rows would wait
  for the next restart. Stop every server instance before starting the new
  version (Kubernetes `Recreate`, or scale to zero then up). A SQLite install
  runs one process and is unaffected.
- **A rollback works for the schema, not for the data model.** Nothing is
  dropped, so the previous binary starts. It imports a separate copy of each
  ticket of a shared tracker for every project again, which brings the
  duplicates back, and it reads each project's board mirror from the project's
  own columns, which this version no longer writes: a project shows the board,
  columns and mapping it had at the upgrade until it is saved again.
- **Deleting a project leaves the tickets of its shared trackers in place**,
  shown by the other projects that select them, else in the backlog. Only the
  tickets of its own local tracker move to the default project, as before.
- **Accepted adoption losses.** When two projects shared a tracker with
  different board mirrors, the project created first wins, and adoption logs
  each conflict. When two copies of a ticket are merged, the survivor's title
  and description are kept and a field only the merged-away copy held is lost.
