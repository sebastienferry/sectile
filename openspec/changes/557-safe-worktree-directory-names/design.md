# Design

## Context and constraints
The local agent owns filesystem and Git operations. Server-provided paths are not authoritative workstation paths. Existing `ensureLocalWorktree` already reuses branches through Git metadata; `localTaskPath` duplicates that resolution and predicts a raw-key path, while desktop discussion launch probes a raw-key directory independently. Secondary repository preparation delegates to task preparation. Macros reuse branches but generate raw-key destinations separately and protect occupied paths through `clearStaleMacroPath`.

## Naming policy
Add a pure shared naming helper in `internal/agent` (suggested `worktree_paths.go`). Validate keys before both generated and legacy lookup paths: reject empty, `.`/`..`, and slash/backslash inputs as today. Preserve existing branch validation and branch fallback behavior separately.

Canonical GitHub numeric keys matching `#[1-9][0-9]*` map to `issue-<digits>` when the full basename fits the bound. Other keys use `key-<slug>-<digest>`, with lowercase ASCII alphanumeric/hyphen slug, repeated separators collapsed, and a fallback slug `item`. Compute SHA-256 from the original key bytes, not normalized/case-folded text; retain the full hexadecimal digest, so punctuation/case/Unicode variants remain distinct and the `issue-` namespace cannot collide. Limit basenames to 120 ASCII bytes, truncating the slug, never the digest. Oversized numeric keys use the general policy. Prefixes avoid reserved device names. This does not change tracker identity or branch names.

Use safe bounded sibling candidates for occupied task destinations, incorporating a digest of the branch and a bounded counter rather than raw key/branch text. Inspect entries with `Lstat` so dangling symlinks count as occupied. Never overwrite entries; propagate Git creation/race failures rather than resetting another checkout. Naming is deterministic; existing branch resolution ensures stable reuse once an occupied-path alternative has been created. No persisted directory mapping is needed.

## Checkout resolution
Consolidate branch lookup using existing `worktreeForBranch` and `sameDirectory`, preserving the caller's root spelling for the main checkout. Preparation resolves its effective branch as today; consumers use the same effective-branch rules when assigned metadata is absent. Git errors propagate rather than being masked by guesses. Match complete branch refs; detached HEAD never matches.

After branch lookup, predict the safe target for a missing checkout. Probe a legacy raw-key candidate only after key validation and only accept a Git checkout belonging to the effective branch and repository; existence alone is insufficient. An unrelated legacy entry remains untouched. Consumers must not launch into or inspect another branch's predicted destination. Missing checkout behavior remains governed by each operation's existing contract (for example workspace information reports absence).

Route `localTaskPath` and desktop discussion launch through the common resolver. Audit primary/secondary preparation, workspace information, editor/diff/removal consumers, skill discovery, and batch launches for remaining raw-key assumptions. Git-enumerated discovery and `internal/workspace/git.go` cleanup already operate on actual paths and should remain that way. Preserve protection of the main checkout and non-forced dirty removal. Do not introduce automatic migration or change batch directory validation.

Macro preparation uses the shared naming helper after its existing branch selection/reuse. Keep macro fetch/default-base behavior, locking, warnings, stale empty-directory handling, and nonempty occupied-path refusal. Do not apply task sibling creation to macros or delete legacy directories during lookup.

## Target files
- `internal/agent/agent_config.go`, `agent_operations.go`, `agent_desktop.go`, `agent_macro_worktree.go`; shared helper and focused tests.
- `internal/agent/repositories.go`, `agent_skill_files.go`, and `internal/workspace/git.go`: audit consumers; edit only if resolution is inconsistent.
- Corresponding task, macro, operation, repository, and desktop tests.
- `docs/ARCHITECTURE.md` and relevant agent-contract documentation: generated names, branch-first legacy reuse, safe-root limitation.
- `CHANGELOG.md`: one user-facing Fixed entry under Unreleased during implementation.

## Validation
Use table tests for canonical numeric mapping, determinism, character/length bounds, punctuation/case/Unicode collisions, reserved names/namespace, extreme lengths, and rejected inputs. Use temporary real Git repositories for primary and secondary reuse, arbitrary/legacy/main/shared checkout locations, detached and wrong-branch candidates, occupied paths and symlinks, dirty non-forced cleanup, and macro safeguards. Exercise desktop and workspace consumers against actual branch paths rather than only the naming helper.

For Vite, create a minimal fixture repository under a safe temporary root, prepare its worktree through the production preparation path using `#289`, install dependencies in the generated checkout, start its local Vite executable there, and use a real browser to assert successful source-module loading and a rendered DOM marker. Record actual paths and dependency location. Always stop the server and remove fixture resources. Preserve #417's existing legacy-path workaround. Run relevant Go tests including race checks and project checks required by touched surfaces; no production code is changed by this specification.

## Alternatives rejected
- Strip punctuation only: normalized and case-insensitive collisions.
- Rename existing paths: disrupts live processes and dirty/shared checkouts.
- Persist server-owned checkout paths: violates workstation ownership and requires unnecessary schema changes.
- Change keys or branches: alters identity beyond this ticket.
- Repair only Vite's root or relocate test execution: misses unsafe dependency paths and does not validate generated worktrees.

## Open questions
None. Technical helper names may follow existing conventions; accepted behavior and safeguards are fixed.
