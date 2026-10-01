# Plan #678 - A context repository prepared but left unchanged blocks the implemented transition

## Stack

Go only: the server (`internal/db`), the agent protocol
(`internal/agentprotocol`) and the local agent (`internal/agent`). No web, no
Desktop, no migration, no MCP tool.

Base: `feat/678` sits on `feat/desktop-conversation-test`. The implementation
step starts by moving it onto `origin/main` (`git rebase --onto origin/main
6e148d26`, which keeps only the `docs(spec)` commits of #678, or a merge of
`origin/main` if the branch was pushed by then),
because `internal/db/stageprs.go` changed on `main` since (#584). Everything
below is written against `origin/main`.

## Design

### 1. A new agent operation, `branch_changes` (FR1, FR3, FR5)

A new action rather than new fields on `git_evidence`:

- `git_evidence` answers "the checkout with that branch checked out"; this
  question needs any checkout of the repository, branch checked out or not,
  and the branch may be gone. Mixing both would change what `found` means.
- An agent that predates the action does not announce it: the dispatcher
  (`internal/handlers/agent_dispatcher.go`) answers
  `agentprotocol.ErrUnsupportedOperation`, which is exactly a failed lookup.

Request: `Operation{Action: "branch_changes", Repository: <identity>, Branch:
<task branch>}`.

Answer:

```json
{"repository":"gitlab.example/g/tools","found":true,"defaultBranch":"main","exists":true,"ahead":0}
```

- `repository` echoes the question (missing or different: failed lookup, as
  `errAgentTooOld` already reads it).
- `found` is false when no candidate checkout's `origin` names the repository.
- `exists` says whether any ref of the branch was seen.
- `ahead` is the maximum, over every ref seen, of
  `git rev-list --count origin/<default>..<ref>`.
- Any Git or network failure is an operation error, never a `found: false`.

Agent side, in a new `internal/agent/branchchanges.go`:

1. Candidates: the workstation's mapping or attached folder for the
   repository (`repositoryFolder`, first, as `git_evidence` does), then
   `checkoutCandidates`.
2. `repositoryCheckout(ctx, repository, candidates)`: the first absolute,
   existing directory whose `git remote get-url origin` names the repository.
   It is the origin-matching half of `verifiedCheckout`; `verifiedCheckout` is
   rewritten on top of it so the matching rule stays in one place.
3. Default branch: `git symbolic-ref --quiet refs/remotes/origin/HEAD` in that
   checkout. Failure is an error (unresolvable default branch).
4. Refs of the branch:
   - `git rev-parse --verify --quiet refs/heads/<branch>`;
   - `git rev-parse --verify --quiet refs/remotes/origin/<branch>`;
   - `git ls-remote --heads origin refs/heads/<branch>`: the head `origin`
     reports now. Failure is an error. A reported head that is not a local
     commit (`git cat-file -e <sha>^{commit}` fails) is an error too: its
     commits cannot be counted.
5. No ref: `exists: false, ahead: 0`. Otherwise `exists: true` and `ahead` as
   above.

Every command goes through `gitLocal`, which already wraps
`agentexec.Hidden`: no new `exec.Command`, so no new Windows test.

`ls-remote` reaches the network, so `branch_changes` is **not** added to
`localInspections` in `internal/db/agentoperations.go`: it keeps the default
45-second deadline.

`branch_changes` is added to `agentprotocol.Operations` and dispatched in
`internal/agent/agent_operations.go` next to `git_evidence`.

### 2. Server: ask before the forge (FR1, FR2, FR6, FR7, FR9)

In `internal/db/stageprs.go`:

```go
// unchangedRepository asks the agent whether the task branch carries no
// commit of its own in a secondary repository. Only a verified "no" skips the
// repository; every other answer returns false and keeps the pull request
// required.
func (d *DB) unchangedRepository(task *models.Task, actorID, identity, branch string) (notice string, unchanged bool)
```

- It calls `callAgent` with `branch_changes`; an error, a missing echo,
  `found: false`, or `ahead > 0` returns `("", false)`.
- On `found && ahead == 0` it returns the notice line (section 3) and true.

`validateStagePRs`, in the loop over `required`: for an `identity` that is not
`primary` and has no `chosen[identity]`, call `unchangedRepository` first;
when it returns true, append the notice and `continue` (no
`validateStagePRAt`, no url appended). The primary's url stays last, so
`stagePRSet.primary()` is unchanged.

`checkSecondaryPRs` applies the same skip when `url == ""`. It returns only an
error today; the adjustment prerequisite has no report to carry the notice, so
the skip is silent there (FR10 is about transition reports).

`taskChangedRepositories` already excludes the primary repository, which
covers FR7. Nothing writes to `tasks.changed_repositories` (FR9).

Both `TransitionTaskStageWithPRs` (`internal/db/stage.go`) and the managed
result path (`internal/db/postback.go`) go through `validateStagePRs`, which
covers FR8 with the adjustment prerequisite (`internal/db/adjustment.go`
calls `checkSecondaryPRs`).

### 3. The notice (FR10)

English, like the existing stage notices it sits with:

```text
Prepared, unchanged: gitlab.example/g/tools has no commit of feat/12 ahead of main, so no merge request is expected there.
```

When the branch does not exist: `... feat/12 exists neither locally nor on
origin, so ...`. "merge request" or "pull request" follows the forge, through
`evidenceTerms` on the forge `repositoryTarget(identity)` resolves. One line
per skipped repository, joined with the other notices by `\n`, as today.

### 4. Contract documentation

`docs/contracts/server-agent-v1.md`, after the `git_evidence` paragraph:
the `branch_changes` request, both answers, the deadline, and the rule that
anything but `found: true, ahead: 0` keeps the pull request required.

### 5. Changelog (FR11)

Under `## [Unreleased]` / `### Fixed`:

```markdown
- **A repository prepared for a task but left unchanged no longer blocks its stage.** When a task opened a worktree in another repository and committed nothing there, implemented (and every later stage) now goes through with the pull requests of the repositories that did change, and the stage report names the skipped repository as prepared, unchanged. A repository with commits still needs its pull request, and an answer the agent cannot give keeps it required. Upgrade the agent along with the server. (#678)
```

## Target files

| File | Change |
| --- | --- |
| `internal/agentprotocol/operations.go` | add `branch_changes` to `Operations` |
| `internal/agent/branchchanges.go` (new) | `repositoryCheckout`, `branchChanges` |
| `internal/agent/evidence.go` | `verifiedCheckout` on top of `repositoryCheckout` |
| `internal/agent/agent_operations.go` | dispatch `branch_changes` |
| `internal/agent/branchchanges_test.go` (new) | Git fixtures, see test plan |
| `internal/db/stageprs.go` | `unchangedRepository`, skip in `validateStagePRs` and `checkSecondaryPRs` |
| `internal/db/stageprs_test.go` | `fakeRepoAgent` answers `branch_changes`; new tests |
| `internal/db/crossrepo_test.go`, `internal/db/no_repository_change_test.go`, `internal/db/activerun_test.go` | fakes answer `branch_changes` where a changed repository is set up |
| `docs/contracts/server-agent-v1.md` | the operation |
| `CHANGELOG.md` | `Fixed` line |

## Rejected alternatives

- **New fields on `git_evidence`**: changes what `found` means for an
  existing answer, and an old agent would answer the old shape without the
  fields, which must then be read as a failed lookup by absence of a field
  rather than by the dispatcher's explicit refusal.
- **Remote-tracking refs only, no `ls-remote`**: a branch pushed with commits
  from another workstation and never fetched here would read as gone, so a
  pull request would be skipped wrongly. The extra network call is the price
  of never waiving review on stale refs.
- **Dropping the repository from the task** (Q3 B): needs an MCP tool or an
  API, the agentmcp whitelist and the tool-count tests; rejected by the owner.
- **A task state "prepared, unchanged"** (Q4 B): rejected by the owner.
- **Server-only decision**: the server cannot see GitLab checkouts on the
  workstation.

## Risks

- **Fakes that refuse unknown operations.** `fakeRepoAgent` calls `t.Errorf`
  on an unexpected action, so every existing multi-repository test fails
  until the fakes answer `branch_changes`. The default answer is
  `ahead: 1`, which keeps every existing test's meaning.
- **One more agent round trip** per secondary repository without a pull
  request, before the forge lookup it may avoid. Bounded by the number of
  secondary repositories.
- **`origin/HEAD` missing** in a clone that never set it: a failed lookup,
  so the current behaviour (pull request required). The error message says
  `git remote set-head origin --auto` fixes it.
- `internal/db` runs close to the `go test` timeout on CI (memory note); the
  new tests are table-driven on the existing fixture and add no server.

## Test plan

Agent (`internal/agent/branchchanges_test.go`, temporary repositories with a
bare `origin` on disk, `origin/HEAD` set):

- branch with 0 commits ahead, local only: `exists: true, ahead: 0`;
- branch pushed with 0 ahead: same;
- branch with one commit ahead, local: `ahead: 1`;
- local 0 ahead, `origin` with one commit ahead fetched: `ahead: 1`;
- branch absent locally and on `origin`: `exists: false, ahead: 0`;
- branch on `origin` not fetched: error;
- `origin/HEAD` unset: error;
- no candidate matching the repository: `found: false`;
- `origin` unreachable (bare repository removed after clone): error.

Server (`internal/db/stageprs_test.go`, `fakeRepoAgent` with per-repository
`branch_changes` answers, the forge faked by the `pr_evidence` fake):

- secondary unchanged, primary `prUrl` only: implemented, one link (primary),
  notice names `gitlab.com/g/b`, `pr_evidence` never asked for it;
- secondary gone (`exists: false`): same;
- secondary `ahead: 1`, no merge request: refused naming `gitlab.com/g/b`
  (existing test, unchanged meaning);
- agent error, `ErrUnsupportedOperation`, missing echo, `found: false`: each
  refused as today;
- secondary merge request given in `prUrls`: `branch_changes` not asked,
  validated as today;
- secondary merge request recorded on the branch: same;
- adjustment prerequisite: unchanged secondary skipped, changed one without a
  merge request refused;
- specified transition on a project opening its pull request at
  specification: unchanged secondary skipped (FR8);
- managed result path: one test through the postback that the unchanged
  secondary is skipped.

Gates: `go build ./...`, `go vet ./...`, `go test ./internal/agent/...
./internal/agentprotocol/... ./internal/db/...` (outside the sandbox, see the
memory notes on `httptest` and `GOCACHE`), `gofmt -l`.
