# #298 — Pull requests are rediscovered as part of ticket synchronisation

## Context

A Sectile task carries an ordered set of pull requests (`prLinks`), `prUrl` being the
current one. Those links are only ever written by the workflow itself: a stage
transition that carries `prUrl`, the adjustment prerequisite that looks a pull
request up from the task branch, or a manual edit in the task detail view.

Ticket synchronisation deliberately never writes them. `ImportOrUpdateTasks`
leaves `pr_url` and `pr_links` untouched, and `SyncSingleTaskAs` re-injects the
locally known `BranchName` and `PrURL` before importing, precisely so that a sync
cannot erase links the workflow produced.

That protection has a blind spot. Pull-request knowledge lives **only** in the
local Sectile store. When the same GitHub repository is registered as a project on
a second Sectile instance — a new laptop, a fresh install, a colleague's board, a
restored deployment — the tickets are imported from the tracker but arrive with no
pull request at all, and nothing ever repairs that: no sync path rediscovers them.
The instance shows implemented and reviewed tasks with no pull request, the
adjustment and handoff stages lose their evidence, and the only recovery is to
paste every URL by hand.

Today no code path can even answer the question "which pull requests belong to this
issue". Discovery is branch-centred (`Client.BranchPullRequest`, `gh pr view`), and
the branch name is itself local state that a fresh instance does not have. The
issue synchronisation explicitly *discards* every tracker entry that is a pull
request.

This specification defines the behaviour and acceptance criteria only.
Architectural choices, data contracts and target files are in
[`plan.md`](./plan.md); the ordered implementation checklist is in
[`tasks.md`](./tasks.md).

---

## Decisions being specified

1. **Synchronisation rediscovers pull requests.** Ticket synchronisation gains a
   pull-request discovery step that asks the tracker which pull requests belong to
   the issue, and persists what it finds on the task.
2. **Discovery is issue-anchored, branch-assisted.** The tracker is asked for the
   pull requests attached to the *issue* (development links: a pull request that
   closes, references or is linked to it). When the task already has a known
   branch, the existing branch lookup is used as a second source. A fresh instance
   that knows nothing but the issue number must still find the pull requests.
3. **Discovery is additive and never destructive.** Rediscovered links are appended
   to the existing ordered set, deduplicated by URL. Synchronisation never removes,
   reorders or replaces a link a human or the workflow recorded, and never clears
   `prUrl`.
4. **The anti-substitution guard is preserved, with a defined seeding rule.** When
   the task already holds links, a rediscovered pull request is accepted under the
   existing rule: same URL, or a branch the task already used. When the task holds
   no link at all, discovery is what seeds the set, and the discovered pull
   requests are accepted together with their branch.
5. **Ordering.** When discovery seeds an empty set with several pull requests, they
   are appended oldest first, so that `prUrl` resolves to the most recent one —
   matching the "current pull request is the last link" invariant.
6. **Discovery is a capability, not an assumption.** A tracker that cannot answer
   the question is skipped silently; synchronisation of its tickets succeeds
   unchanged. GitHub is the tracker this issue delivers.
7. **Discovery is bounded.** Every ticket sync must not cost an extra API call per
   ticket forever. Discovery runs for a task when it is plausible a pull request
   exists and cheap knowledge is missing, and its failures never fail the sync.
8. **Discovery is observable.** A sync that attached pull requests says so in its
   activity, naming the task and the URLs. A discovery that failed is reported as a
   warning on the sync activity, not as a sync failure.

---

## User stories

### US1 — A project recreated on a second instance recovers its pull requests (P1)

**As a** developer registering an existing GitHub repository as a project on a
fresh Sectile instance,
**I want** the imported tickets to come back with their pull requests,
**So that** the board reflects reality and the workflow stages keep their evidence
without me pasting URLs by hand.

- **Given** a GitHub repository whose issue #42 has one merged pull request that
  closes it
- **And** a Sectile project freshly created on that repository, with no task yet
- **When** the project is synchronised
- **Then** the task for #42 exists with that pull request URL in `prLinks` and as
  its `prUrl`.

- **Given** an issue with two pull requests, an older merged one and a newer open
  one
- **When** the task is synchronised on an instance that holds no link for it
- **Then** both URLs are present in `prLinks`, oldest first
- **And** `prUrl` is the newer one.

- **Given** an issue with no pull request at all
- **When** the task is synchronised
- **Then** the task keeps an empty `prLinks` and a null `prUrl`, and the sync
  reports no error.

---

### US2 — Synchronisation never damages links already recorded (P1)

**As a** developer whose task already carries the pull request its workflow
produced,
**I want** synchronisation to leave that set alone,
**So that** a background sync can never rewrite, reorder or drop my evidence.

- **Given** a task whose `prLinks` already contains the pull request of branch
  `feat/42`
- **When** synchronisation rediscovers that same URL
- **Then** `prLinks` is unchanged, with no duplicate entry, and `prUrl` still points
  at it.

- **Given** a task whose `prLinks` contains a pull request on branch `feat/42`
- **When** synchronisation discovers an additional pull request on that same branch
- **Then** it is appended after the existing one and becomes `prUrl`.

- **Given** a task whose `prLinks` contains a pull request on branch `feat/42`
- **When** synchronisation discovers a pull request on an unrelated branch
  `feat/other`
- **Then** it is refused, the existing set is unchanged, and the refusal is reported
  as a warning on the sync activity rather than as a sync failure.

- **Given** a task whose pull request links were deliberately detached by a human in
  the task detail view, and whose branch is no longer recorded
- **When** synchronisation runs
- **Then** the behaviour is the one chosen for **OPEN-2** below.

---

### US3 — A sync failure on discovery never breaks ticket synchronisation (P1)

**As a** user relying on the background auto-sync,
**I want** pull-request discovery to be best effort,
**So that** a rate limit, a permission gap or a tracker outage never stops my
tickets from syncing.

- **Given** a tracker that returns HTTP 429 to the discovery request
- **When** a ticket is synchronised
- **Then** the ticket fields are still imported and updated
- **And** the task's pull-request links are left untouched
- **And** the sync activity records a warning naming the rate limit.

- **Given** a token without the scope needed to read a repository's pull requests
- **When** a project is synchronised
- **Then** every ticket syncs normally and discovery is reported as unavailable once
  rather than once per ticket.

- **Given** a tracker with no pull-request discovery capability (for example the
  local board)
- **When** a ticket is synchronised
- **Then** no discovery is attempted and the sync behaves exactly as it does today.

---

### US4 — The user can see what synchronisation attached (P2)

**As a** developer watching the sync activity,
**I want** rediscovered pull requests to be reported,
**So that** a link that appeared on a task is explainable and auditable.

- **Given** a project sync that attaches pull requests to three tasks
- **When** the sync activity is read
- **Then** it lists the affected task keys and the URLs attached.

- **Given** a single-task sync that attaches nothing
- **When** the sync activity is read
- **Then** it reports no pull-request change rather than an empty section.

---

### US5 — A user can force rediscovery for one task (P2)

**As a** developer whose task is missing its pull request although the issue has
one,
**I want** to ask for rediscovery on that task,
**So that** I can repair it without waiting for whatever condition gates the
automatic pass.

- **Given** a task with no pull request whose issue has one
- **When** the user triggers a single-task synchronisation from the task detail view
- **Then** discovery runs for that task regardless of the bounding rule of
  **decision 7**, and the link is attached.

---

## Acceptance criteria

- **AC1** — A project created on a second instance against a repository whose
  issues have pull requests ends, after one full synchronisation, with the same
  pull-request links as the originating instance, for every task whose pull
  requests are discoverable from the issue.
- **AC2** — No synchronisation path can reduce the number of entries in a task's
  `prLinks`, reorder existing entries, or set `prUrl` to null when it was set.
- **AC3** — Discovery failures are never surfaced as ticket synchronisation
  failures, and never leave the task partially written.
- **AC4** — Existing behaviour is unchanged for trackers without the capability and
  for tasks whose links are already complete.
- **AC5** — `TestSchemaIsCompleteWithoutLegacyMigrations` still passes, and the
  SQLite and PostgreSQL schemas stay at parity, if the work introduces any column.
- **AC6** — `make test` passes.

---

## Out of scope

- Jira and GitLab development-link discovery. The capability is designed so they can
  be added, but only GitHub is implemented here.
- Importing pull requests as tasks of their own, or showing a pull-request list
  distinct from `prLinks`.
- Backfilling the branch name of historical tasks beyond what discovery itself
  yields.
- Any change to the anti-substitution rule for tasks that already hold links.

---

## Open requirements

These were not settled by the clarification stage. **The ticket carries no
clarification outcome: its GitHub discussion holds no comment, although the task
is labelled `#clarified`.** Each item below is therefore stated as an open
requirement with a recommended answer; implementation must not silently pick one.

- **OPEN-1 — Which GitHub mechanism is authoritative for "the pull requests of this
  issue".** Candidates: GraphQL `closingIssuesReferences` (only pull requests that
  *close* the issue), the REST issue timeline (`cross-referenced` and `connected`
  events, broader but noisier), or `GET /search/issues` on the issue number (cheap,
  but matches any mention). *Recommendation: GraphQL closing references as the
  authoritative source, plus the existing branch lookup when a branch is known.*
  **Blocks**: the whole discovery contract, and how noisy the result is.
- **OPEN-2 — What happens to a task whose links a human deliberately detached.**
  Re-attaching on the next sync would undo an explicit gesture; never re-attaching
  requires remembering the detachment. *Recommendation: record the detachment and
  suppress automatic rediscovery for that task until a manual rediscovery
  (US5) is asked for.* **Blocks**: US2's last scenario and whether a column is
  needed — which in turn decides whether AC5 applies.
- **OPEN-3 — The bounding rule of decision 7.** Options: discover only for tasks
  with zero links; only for tasks at or beyond the project's PR creation stage;
  only once per task unless forced; or on every full sync but not on the 30-second
  incremental pass. *Recommendation: on tasks with zero links, at or beyond the
  PR creation stage, at most once per full-sync period.* **Blocks**: the API
  call budget and the auto-sync rate-limit behaviour.
- **OPEN-4 — Whether a rediscovered branch may seed `branch_name` when it is
  empty.** It would make the branch-based lookup work from the next sync onwards,
  but synchronisation has so far never written that field. *Recommendation: yes,
  only when it is empty.* **Blocks**: the write path in `plan.md`.
