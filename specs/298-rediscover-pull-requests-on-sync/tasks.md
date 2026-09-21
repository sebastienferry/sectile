# #298 — Implementation checklist

Ordered so that each layer is verified before the next is built on it: open
requirements first, tracker client second, capability third, store write path
fourth, sync wiring fifth, front-end and end-to-end last.

Tasks marked **(OPEN-n)** are blocked until the matching open requirement of
[`spec.md`](./spec.md) is answered. See [`plan.md`](./plan.md) for the decisions.

---

## 0. Unblock the open requirements

- [ ] **T0** Get an explicit answer on **OPEN-1** (authoritative GitHub mechanism),
  **OPEN-2** (deliberate detachment), **OPEN-3** (bounding rule) and **OPEN-4**
  (seeding `branch_name`). Record each answer in `spec.md` under *Open
  requirements*, replacing the recommendation with the decision. No task below
  starts before its own OPEN marker is resolved.

---

## 1. Tracker client — GitHub discovery

- [ ] **T1** In `internal/trackerapi/github.go`: add `CreatedAt time.Time` to
  `PullRequest` and populate it in `BranchPullRequest` so ordering is uniform.
- [ ] **T2 (OPEN-1)** In `internal/trackerapi/github.go`: add
  `Client.IssuePullRequests(ctx, repo string, issueNumber int) ([]PullRequest, error)`
  using the GraphQL entry point, mapping nodes to `PullRequest`, sorted ascending
  by `CreatedAt`. Return a typed not-supported error when the repository or issue
  cannot be resolved, and surface 429 the way the existing client does.
- [ ] **T3** In `internal/trackerapi/github_test.go`: `httptest` coverage of T2 —
  nominal payload (URL, branch, order), empty payload, HTTP 429, and a payload with
  a pull request lacking a head branch.

---

## 2. Capability and interface

- [ ] **T4** In `internal/tracker/ticketing.go`: declare the optional
  `PullRequestDiscoverer` interface. Do **not** implement it on
  `BaseTicketingSystem`.
- [ ] **T5** In `internal/trackerapi/adapters.go`: add the discovery capability to
  the capability set, announce it on `GithubAdapter` only, and implement
  `GithubAdapter.IssuePullRequests` by delegating to T2.
- [ ] **T6** In `internal/trackerapi/adapters_test.go`: assert GitHub announces the
  capability and that the local and Jira adapters do not.

---

## 3. Store — discovery gate and write path

- [ ] **T7** Create `internal/db/prdiscovery.go` with the injectable hook
  `d.prDiscoveryLookup`, mirroring `d.prEvidenceLookup`.
- [ ] **T8 (OPEN-3)** Implement `discoverTaskPullRequests`: evaluate the gate
  (capability, zero links, stage at or beyond `PRCreationStage`, not detached, full
  sync only) before any network call, and honour a `force` argument that bypasses
  every clause but the capability.
- [ ] **T9 (OPEN-2, OPEN-4)** Implement `applyDiscoveredPullRequests`:
  - non-empty set → filter through `models.AcceptPullRequest`, refusals become
    warnings;
  - empty set → seed via `models.AppendPullRequestLink`, oldest first;
  - no change → no write and no activity step;
  - change → single transaction writing `pr_url` and `pr_links` together, as
    `internal/db/stage.go:146-157` does, plus `branch_name` when it is empty
    (OPEN-4);
  - return the attached URLs and the warnings.
- [ ] **T10 (OPEN-2)** If the answer is "remember the detachment": add the
  `pr_links_detached` column to **both** the `CREATE TABLE tasks` statement
  (`internal/db/db.go:419`) and `applyLegacyMigrations` (`:626`); set it in
  `UpdateTask` on an explicit empty `PrLinks` (`:2528-2543`); clear it whenever a
  link is attached. Verify `TestSchemaIsCompleteWithoutLegacyMigrations` still
  passes.
- [ ] **T11** Create `internal/db/prdiscovery_test.go` covering: empty-set seeding
  and ordering with `prUrl` on the newest; a known URL producing no write; a
  same-branch follow-up appended; an unrelated-branch candidate refused and
  reported; a discovery error leaving the task untouched; a tracker without the
  capability triggering no call; the gate and the `force` bypass.

---

## 4. Sync wiring

- [ ] **T12** In `internal/db/db.go`: call discovery from `SyncSingleTaskAs`
  (`:4300`) after the import returns, threading the `force` flag, and from
  `processSyncJob` (`:3865`) / `processSyncTaskJob` (`:4410`) per imported task.
  `ImportOrUpdateTasks` stays untouched.
- [ ] **T13 (OPEN-3)** In `internal/db/autosync.go`: distinguish the full-sync pass
  from the incremental tick so the gate of T8 can read it, without changing the
  existing intervals or the one-pass-at-a-time guard.
- [ ] **T14** Report on the sync activity: one step per task with the attached URLs,
  one warning step per refusal or discovery failure, and nothing when nothing
  changed. A discovery error never becomes the job's error.
- [ ] **T15** Rate limiting: a 429 on discovery enters the existing auto-sync
  backoff (`internal/db/autosync.go:257`, `:271`) and aborts the remaining
  discoveries of the pass while letting the ticket import finish; an authorisation
  failure is reported once per pass, not once per ticket.
- [ ] **T16** Extend the fake tracker of `internal/db/jiratracker_test.go:70` and
  add an end-to-end test: a full sync of a project whose tasks hold no link
  attaches them and lists them in the activity steps.

---

## 5. Manual rediscovery (US5)

- [ ] **T17** Expose the `force` flag on the single-task sync path so a
  user-triggered sync from the task detail view bypasses the gate. Reuse the
  existing gesture if it already reaches `SyncSingleTask`; add a front-end control
  in `web/src/components/TaskDetailModal.tsx` only if it does not.
- [ ] **T18** If a front-end change is made, add the matching test under
  `web/tests/` following the dependency-free convention.

---

## 6. Verification and documentation

- [ ] **T19 (OPEN-2)** If T10 added a column, extend
  `internal/db/postgres_writepaths_test.go` accordingly.
- [ ] **T20** Run `make test` (Go, `web` node tests, `tsc --noEmit`, `oxlint`) and
  fix every failure.
- [ ] **T21** Author `docs/adrs/0017-pull-requests-are-rediscovered-on-sync.md`
  recording the issue-anchored discovery choice and the rejected alternatives of
  `plan.md`.
- [ ] **T22** Update `docs/CAPABILITIES.md` (and `docs/API_AND_DATA_SPEC.md` if the
  task payload changes) with the new tracker capability.
- [ ] **T23** Add a `CHANGELOG.md` entry if the file exists at implementation time.
