# #298 — Implementation plan

Behaviour and acceptance criteria are in [`spec.md`](./spec.md). This file holds the
implementation choices only. Items marked **(OPEN-n)** depend on an open
requirement of `spec.md` and must be settled before the corresponding task starts.

## Stack

- **Go server / store** — `internal/db` (synchronisation, pull-request write paths),
  `internal/models` (task model and pull-request rules).
- **Tracker adapters** — `internal/trackerapi` (GitHub REST and GraphQL client,
  adapter registry and capabilities), `internal/tracker` (ticketing interfaces).
- **Web front-end** — `web/src` only for US5's manual trigger, if a new gesture is
  needed rather than reusing the existing single-task sync.

## Current state the plan builds on

- `internal/models/pullrequests.go` — `NormalizePullRequestLinks`,
  `AppendPullRequestLink` (idempotent, order-preserving), `CurrentPullRequest`,
  `PullRequestBranches`, `AcceptPullRequest` (anti-substitution guard).
- `internal/db/pullrequests.go` — `encodePullRequestLinks` / `decodePullRequestLinks`;
  `pr_url` and `pr_links` are always written by the same statement.
- `internal/db/adjustment.go:20` `adjustmentPrerequisite` — the existing precedent
  for *persisting a discovered* link (`:45-56`), with the injectable
  `d.prEvidenceLookup` hook used by tests.
- `internal/trackerapi/github.go:314` `Client.BranchPullRequest` — branch-anchored
  REST lookup returning `PullRequest{URL, Branch, SHA, Open, Draft, Merged}`.
- `internal/db/db.go:4300` `SyncSingleTaskAs`, `:873` `ImportOrUpdateTasks` (which
  writes neither `pr_url` nor `pr_links`), `:4431` `EnqueueSyncAs`,
  `internal/db/autosync.go:163` `runAutoSyncPass`.

## Architecture decisions

### D1 — A tracker capability, not a GitHub special case

Add a discovery capability alongside the existing ones in
`internal/trackerapi/adapters.go` (`CapSync`, `CapGet`, …), for example
`CapDiscoverPullRequests`, and an optional interface on the ticketing system:

```go
type PullRequestDiscoverer interface {
    // IssuePullRequests returns the pull requests attached to an issue,
    // oldest first. A tracker that cannot answer returns ErrNotSupported.
    IssuePullRequests(ctx context.Context, issueKey string) ([]PullRequest, error)
}
```

`internal/tracker.BaseTicketingSystem` does not implement it; only
`GithubAdapter` does. Every call site type-asserts and skips silently otherwise,
which satisfies US3's third scenario with no change to the local board or Jira.

### D2 — GitHub discovery (OPEN-1)

Implement `Client.IssuePullRequests(repo, issueNumber)` in
`internal/trackerapi/github.go`, next to `BranchPullRequest`, reusing the existing
authenticated HTTP client and the GraphQL entry point already used by
`transferIssue`.

Recommended query, pending OPEN-1:

```graphql
query($owner:String!, $name:String!, $number:Int!) {
  repository(owner:$owner, name:$name) {
    issue(number:$number) {
      closedByPullRequestsReferences(first:20, includeClosedPrs:true) {
        nodes { url headRefName createdAt state merged isDraft }
      }
    }
  }
}
```

Results are mapped to the existing `PullRequest` struct, sorted by `createdAt`
ascending (spec decision 5). GraphQL is the only reasonable option here: the REST
API exposes no issue→pull-request link short of walking the timeline.

### D3 — Discovery runs in the sync job, not in `ImportOrUpdateTasks`

`ImportOrUpdateTasks` stays untouched — it must keep its property of never writing
`pr_url` / `pr_links`, which is what protects US2. Discovery is a distinct step:

- a new `internal/db/prdiscovery.go` holding
  `discoverTaskPullRequests(ctx, task, ts) ([]models.TaskPullRequest, error)` and
  `applyDiscoveredPullRequests(ctx, actorID, task, found) (attached []string, err error)`;
- called from `SyncSingleTaskAs` (`internal/db/db.go:4300`) **after** the import
  returns, and from `processSyncJob` (`:3865`) per imported task;
- `d.prDiscoveryLookup` injectable hook, mirroring `d.prEvidenceLookup`, so tests
  need no live GitHub.

### D4 — The write path

`applyDiscoveredPullRequests` reuses the existing rules rather than re-deriving
them:

1. read the task's current `pr_links`;
2. if the set is **non-empty**, filter candidates through
   `models.AcceptPullRequest` (spec decision 4) — refusals become warnings,
   never errors;
3. if the set is **empty**, accept the candidates as the seed, appending oldest
   first via `models.AppendPullRequestLink`;
4. if nothing changed, write nothing at all — no UPDATE, no activity noise;
5. otherwise write `pr_url` and `pr_links` in a single statement inside one
   transaction, exactly as `internal/db/stage.go:146-157` does, so the two columns
   can never diverge;
6. **(OPEN-4)** when `branch_name` is empty and the accepted candidate carries a
   branch, set it in the same statement.

### D5 — Bounding (OPEN-3)

Recommended gate, evaluated in `discoverTaskPullRequests` before any network call:

- the tracker announces the capability;
- the task has zero pull-request links;
- the task's stage is at or beyond the project's `PRCreationStage`
  (`internal/db/projectskills.go`, `internal/db/adjustment.go:11` `prCreationOwner`);
- the task is not marked as deliberately detached **(OPEN-2)**;
- and the pass is a full sync, not the 30-second incremental tick of
  `internal/db/autosync.go:163`.

A single-task sync triggered by a user (US5) bypasses every clause but the first —
that is the `force` flag threaded through `SyncSingleTaskAs`.

### D6 — Persisting a deliberate detachment (OPEN-2)

If OPEN-2 is answered "remember the detachment", the cheapest shape is a boolean
column `pr_links_detached` on `tasks`, set when `UpdateTask` receives an explicit
empty `PrLinks` slice (`internal/db/db.go:2528-2543`) and cleared whenever a link
is attached by any path. The repository has **no migration directory**: a new
column must be added to *both* the `CREATE TABLE tasks` statement
(`internal/db/db.go:419`) and `applyLegacyMigrations` (`:626`), or
`internal/db/schemaparity_test.go:56` fails. PostgreSQL does not replay legacy
migrations (`internal/db/dialect_postgres.go:67`), so the `CREATE TABLE` edit is
what makes it exist there.

If OPEN-2 is answered "always re-attach", no schema change is needed and AC5 does
not apply.

### D7 — Reporting (US4)

The sync activity already carries `steps`. `applyDiscoveredPullRequests` returns
the URLs it attached; `processSyncJob` and `processSyncTaskJob`
(`internal/db/db.go:3865`, `:4410`) append one step per affected task
(`"#42 — attached https://github.com/…/pull/17"`) and one warning step per
discovery failure or refusal. A discovery error is never propagated as the job's
error (AC3).

### D8 — Rate limiting

Discovery reuses the client that already honours the auto-sync backoff
(`internal/db/autosync.go:257` `isRateLimited`, `:271` `enterAutoSyncBackoff`). A
429 on discovery enters the same backoff and aborts the remaining discoveries of
that pass while letting the ticket import finish, and an authorisation failure is
reported once per pass rather than per ticket (US3, second scenario).

## Data contracts

- `models.TaskPullRequest{URL, Branch}` — unchanged.
- `trackerapi.PullRequest{URL, Branch, SHA, Open, Draft, Merged}` — extended with
  `CreatedAt time.Time` for the ordering of decision 5.
- No API response shape changes: `prUrl` and `prLinks` already exist on the task
  payload and on `web/src/lib/pullRequests.ts`.

## Target files

| File | Change |
| --- | --- |
| `internal/trackerapi/github.go` | `IssuePullRequests`, GraphQL query, `CreatedAt` on `PullRequest` |
| `internal/trackerapi/adapters.go` | discovery capability, `GithubAdapter.IssuePullRequests` |
| `internal/tracker/ticketing.go` | optional `PullRequestDiscoverer` interface |
| `internal/db/prdiscovery.go` | **new** — gate, discovery, write path, reporting |
| `internal/db/db.go` | call sites in `SyncSingleTaskAs`, `processSyncJob`, `processSyncTaskJob`; `force` flag; `(OPEN-2)` schema + legacy migration |
| `internal/db/autosync.go` | full-sync vs incremental distinction for the gate `(OPEN-3)` |
| `internal/db/prdiscovery_test.go` | **new** — unit tests |
| `internal/trackerapi/github_test.go` | `httptest` coverage of the GraphQL call |
| `web/src/…` | US5 trigger, only if the existing single-task sync gesture is insufficient |

## Rejected alternatives

- **Discover inside `ImportOrUpdateTasks`** — rejected: it would give the import
  path the ability to write `pr_url`, destroying the guarantee that protects US2.
- **`GET /search/issues` on the issue number** — rejected as the authoritative
  source: it matches any textual mention, so an unrelated pull request quoting
  "#298" would be attached.
- **Re-deriving branches from a naming convention (`feat/<number>`)** — rejected:
  the convention is a recommendation, not an invariant, and a wrong guess feeds the
  anti-substitution guard with noise.
- **A dedicated `task_pull_requests` table** — rejected: out of proportion, and the
  JSON column already carries order and branch.

## Test plan

Conventions: `_test.go` in the same package, SQLite store via
`NewDB(filepath.Join(t.TempDir(), "x.db"))`, doubles injected through `*DB` fields,
`httptest` for HTTP clients, no external test framework.

1. `internal/trackerapi` — `httptest` server returning a canned GraphQL payload;
   assert URL, branch and ascending order; assert a 429 surfaces as a rate-limit
   error.
2. `internal/db/prdiscovery_test.go`:
   - empty set is seeded, oldest first, `prUrl` is the newest (US1);
   - an already-known URL produces no write at all (US2);
   - a same-branch follow-up is appended (US2);
   - an unrelated-branch candidate is refused and reported (US2);
   - a discovery error leaves the task untouched and the sync successful (US3);
   - a tracker without the capability triggers no call (US3);
   - the gate of D5 is respected, and `force` bypasses it (US5).
3. `internal/db` sync tests — extend the existing fake tracker
   (`internal/db/jiratracker_test.go:70`) so a full sync attaches links end to end
   and reports them in the activity steps (US4).
4. `internal/db/postgres_*_test.go` — if D6 adds a column, cover it in the
   PostgreSQL write-path subset, and rely on
   `TestSchemaIsCompleteWithoutLegacyMigrations` for parity.
5. `make test` (Go, `web` node tests, `tsc`, `oxlint`).
