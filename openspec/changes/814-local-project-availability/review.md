# Adjustment review — #814

## Scope and outcome

Reviewed every changed file against the accepted clarification and OpenSpec
requirements on 2026-10-11. PR #818 was verified open, draft, and matching
`sebastienferry/sectile` branch `feat/814` before any changes. The fetched
default branch is `main`. Final ancestry verification found that commit
`97e83f1b579824fecae363bf06ee82b5c2a7f5c7` is not an ancestor of this branch.
The earlier ancestry conclusion was incorrect because a later command masked
the failed shell exit status. Adjustment is blocked and not complete.

Rebasing onto that commit exposed incompatible accepted behavior. Main's #813
replaces a user's device connection even when its project scope changes and
requires an explicit device choice when multiple workstations connect. #814's
accepted design retains multiple scopes per device and preserves latest/local
compatibility routing without selecting a workstation. Choosing either behavior
would discard an accepted requirement or a merged regression test. The rebase
was aborted to preserve all work. PR #818 was returned to draft; no reviewed
transition was submitted. Owner reconciliation of these requirements is needed
before integration and checks on the combined state.

No actionable implementation defect was found in the pre-integration state.
No runtime change was made.
Reviewed collection bounds/cancellation, settings withdrawal and recovery,
authenticated project visibility, user isolation, simultaneous device sessions,
same-device replacement, exact-session forwarding, pending operation/launch/pull
ownership, shared directory publication, migration and credential lifecycle.
The existing changelog and protocol documentation match the accepted scope.

## Feedback dispositions

- PR conversation comments: retrieved successfully; none available.
- PR reviews: retrieved successfully; none available.
- Inline review comments: retrieved successfully; none available.
- Ticket clarification and specification reports: retained as accepted scope.
- Ticket implementation report: checked against the full diff and independently
  rerun validation below; no corrective action required.

## Pre-integration validation and replay checklist

- [x] `make test`: Go suite passed (agent 74.993s), 732 web tests passed,
  TypeScript and lint exited 0; formatting is included by the target.
- [x] `go test -race ./internal/agent ./internal/handlers ./internal/db -run
  'Test.*Inventory|TestSharedWorkstation|TestDistinctDevice|TestWorkstation|TestFailedReplacement'`:
  passed in all three packages.
- [x] `go vet ./internal/agentprotocol ./internal/agent ./internal/handlers ./internal/db`:
  passed.
- [x] `make build-server build-agent`: passed.
- [x] With `SECTILE_TEST_POSTGRES_DSN` pointing to an isolated PostgreSQL 17
  database, `go test -count=1 ./internal/db -run
  'TestPostgresAgentPresence|TestPostgresSchemaIsCreatedFromEmpty|TestPostgresWorkstationSessions'`:
  passed (0.654s). The temporary container was removed afterwards.
- [x] `openspec validate 814-local-project-availability --strict`: passed.
- [x] `git diff --check`: passed.

Go commands used `GOCACHE=/tmp/sectile-814-go-cache`. The initial sandboxed
suite could not access the default Go cache and was retried with local-server
permissions. The first complete suite hit an existing Desktop console replay
timeout. That test passed three isolated repetitions, and the subsequent full
suite passed without concurrent build work. No failure was suppressed. Existing
frontend lint and bundle-size warnings remain in unchanged code. GitHub reports
no CI checks for this branch; the evidence above is local validation.

## Reviewer attention

Review atomic identity replacement and teardown in `internal/db/workstations.go`,
exact-session forwarding in `internal/handlers/agent_cluster.go`, and the
inventory-independent compatibility routes. Availability remains advisory;
all server replicas must upgrade to guarantee simultaneous device retention.
Automatic workstation selection and execution readiness checks remain excluded.

Only this review evidence is added during adjustment. These checks do not
validate integration with current main. PR #818 is reused and remains draft; merge,
approval, ticket closure and worktree cleanup remain the owner's actions.
