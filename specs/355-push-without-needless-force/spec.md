# #355 — Publish without a needless force push

Source: https://github.com/sebastienferry/sectile/issues/355 · Clarification: [`docs/clarifications/355.md`](../../docs/clarifications/355.md)

## Problem

The generated `create_pr` skill led an agent to run `git push --force-with-lease` on a branch that did not exist on the remote yet, and the push failed. The skill mentions force-with-lease in the base-reconciliation step without saying how to tell whether a force is needed. The `adjust` skill (embedded in `pickup` and `pickup_issues`) carries the same vague instruction.

## User stories

### P1 — First publication never forces

As a user whose agent publishes a new branch, I want the skill to push it with `git push -u origin <branch>` so that publication does not fail on a lease against a missing ref.

- **Given** `origin/<branch>` does not exist after `git fetch origin`, **when** the agent publishes, **then** the skill tells it to run `git push -u origin <branch>` and never to force.

### P1 — Fast-forward uses a plain push

- **Given** `origin/<branch>` exists and is an ancestor of `HEAD` (`git merge-base --is-ancestor origin/<branch> HEAD` succeeds), **when** the agent publishes, **then** the skill tells it to run a plain `git push`.

### P1 — Rewritten history uses a guarded force

- **Given** `origin/<branch>` exists and is not an ancestor of `HEAD` (an authorized rebase rewrote published history), **when** the agent publishes, **then** the skill tells it to run `git push --force-with-lease`.

### P2 — A push refused because the remote moved is resynced and retried once

- **Given** the push is refused (stale lease or non-fast-forward) because commits landed on `origin/<branch>`, **when** the agent handles the refusal, **then** the skill tells it to `git fetch origin`, run `git rebase origin/<branch>` to keep the remote commits, resolve conflicts, re-run the required checks if new commits came in, and retry once with the same rule.
- **Given** the retry is refused again, or the refusal has another cause (branch protection, permissions, authentication), **then** the skill tells it to stop, keep the work and report a blocker.

### P1 — Never an unguarded force

- **Given** any situation, **then** both skills forbid `git push --force`.

## Functional requirements

- **FR1** `create_pr` and `adjust` state the same three-case push rule, evaluated after `git fetch origin`.
- **FR2** Both skills state the resync-and-retry-once handling of a refused push and the stop condition.
- **FR3** Both skills forbid an unguarded `git push --force`.
- **FR4** `pickup` and `pickup_issues` inherit the rule through the `adjust` fragment they embed.
- **FR5** The base-branch reconciliation policy (rebase when private, merge when shared) is unchanged.

## Out of scope

- Sectile's Go push/branch code and the runner prompt templates (`internal/runner`), which do not name a push mode.
- The project PR policy text in `internal/db/projectskills.go`.
- Re-installing skills already copied into users' agent directories.

## Open requirements

None.
