# ADR 0017: Pull requests are rediscovered on ticket synchronisation

Status: Accepted

## Context

ADR 0010 gives a task an ordered set of pull request links. Those links are only
ever written by the workflow: a stage transition that carries a pull request URL,
the adjustment prerequisite that looks one up from the task branch, or a manual
edit in the task detail view.

Ticket synchronisation deliberately never writes them. `ImportOrUpdateTasks`
leaves `pr_url` and `pr_links` untouched, and `SyncSingleTaskAs` re-injects the
locally known branch and pull request before importing, precisely so that a sync
cannot erase workflow evidence.

That protection has a blind spot: pull-request knowledge lives **only** in the
local store. When the same repository is registered as a project on a second
instance — a new laptop, a fresh install, a colleague's board, a restored
deployment — the tickets are imported with no pull request at all, and nothing
ever repairs it. Implemented and reviewed tasks show no pull request, the
adjustment and handoff stages lose their evidence, and the only recovery was
pasting every URL by hand (issue #298).

Discovery was also impossible to express: it was branch-anchored
(`Client.BranchPullRequest`), and the branch name is itself local state a fresh
instance does not have.

## Decision

Synchronisation rediscovers pull requests, **issue-anchored and branch-assisted**.

- **A tracker capability, not a GitHub special case.** `CapPullRequests` and the
  optional `tracker.PullRequestDiscoverer` interface. `BaseTicketingSystem` does
  not implement it; only the GitHub adapter does. Every call site type-asserts
  and stays silent otherwise, so Jira and the local board are unchanged.
- **GitHub reads the issue's closing references** through GraphQL
  (`closedByPullRequestsReferences`, closed pull requests included), mapped
  oldest first.
- **Discovery runs after the import, never inside it** (`internal/db/prdiscovery.go`).
  `ImportOrUpdateTasks` keeps its property of never writing `pr_url` / `pr_links`,
  which is what protects the links a workflow produced.
- **The write is additive.** A task that already holds links keeps the
  anti-substitution guard of ADR 0010: a candidate on a branch no link mentions
  is refused and reported. A task that holds none is seeded, oldest first, so
  the current pull request is the newest. Nothing new means no write at all.
  `pr_url` and `pr_links` are written by one statement, and a rediscovered
  branch seeds `branch_name` only when it is empty.
- **Discovery is bounded.** It runs for tasks with zero links, at or beyond the
  project's `PRCreationStage`, on a full synchronisation only. The 30-second
  incremental pass re-reads tickets one by one and never discovers. A
  synchronisation a person triggers on one ticket bypasses the whole gate, which
  is how a task is repaired on demand.
- **A deliberate detachment is remembered.** Detaching every link from the task
  detail view raises `tasks.pr_links_detached`, which silences automatic
  rediscovery until a link is attached again by any path, or a rediscovery is
  asked for explicitly. Re-attaching a minute later what a person just removed
  would make the gesture meaningless.
- **Discovery is best effort.** A failure is a warning on the synchronisation
  activity, never a synchronisation failure. A rate limit enters the existing
  auto-sync backoff and stops the discoveries of that pass; a refused credential
  stops them too, so the same refusal is reported once instead of once per
  ticket.

## Rejected alternatives

- **Discovering inside `ImportOrUpdateTasks`** — it would give the import path
  the ability to write `pr_url`, destroying the guarantee above.
- **`GET /search/issues` on the issue number** — cheap, but it matches any
  textual mention: an unrelated pull request quoting "#298" would be attached.
- **The REST issue timeline** — broader than the closing references and noisier,
  for the same reason.
- **Re-deriving branches from a naming convention (`feat/<number>`)** — the
  convention is a recommendation, not an invariant, and a wrong guess feeds the
  anti-substitution guard with noise.
- **A dedicated `task_pull_requests` table** — out of proportion: the JSON
  column already carries order and branch.

## Consequences

- A project recreated on a second instance recovers its pull requests after one
  full synchronisation, for every task whose pull requests close their issue.
- One extra tracker call per *eligible* task per full pass, and none at all for
  a tracker without the capability or for a task that already holds links.
- `tasks.pr_links_detached` is declared in the `CREATE TABLE` as well as in the
  legacy migrations: PostgreSQL never replays the latter (ADR 0016). A
  PostgreSQL database created by an earlier version would therefore never gain
  the column, and every write path naming it would fail, so `initSchema`
  reconciles it there with an idempotent `ADD COLUMN IF NOT EXISTS`. That is the
  first column added after PostgreSQL support shipped; the next one takes the
  same route.
- The task payload is unchanged: the flag is store-side state, read only by the
  discovery gate.
