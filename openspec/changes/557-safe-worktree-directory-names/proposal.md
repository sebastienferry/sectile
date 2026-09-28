# Filesystem-safe task and macro worktree directories

## Why
New worktrees currently use tracker keys as directory names. A key such as `#289` introduces a URL fragment character into the absolute path, breaking Vite dependency scanning and source loading. Directory naming must be independent of tracker identity without abandoning existing checkouts.

## What Changes
- Generate deterministic, bounded ASCII-safe directory names for new primary and secondary task worktrees and macro specification worktrees. Numeric GitHub keys use `issue-<number>`.
- Prevent normalization, case, reserved-name, and occupied-destination collisions.
- Resolve actual checkouts by assigned branch before predicting a directory, consistently across preparation, discovery, workspace information, desktop launches, editor/diff operations, and explicit cleanup.
- Preserve existing arbitrary, legacy, main-checkout, and shared batch locations, including uncommitted work.
- Prove Vite source loading in a newly generated safe worktree with dependencies installed inside it.

## Scope
In scope: workstation naming/resolution, task and macro preparation, relevant tests, architecture/agent-contract documentation, and a user-visible Unreleased changelog entry during implementation.

Out of scope: automatic migration or deletion of legacy worktrees, fixing Vite in retained unsafe paths, renaming branches or tracker/display keys, schema changes, infrastructure changes, batch naming changes, and removal of the #417 browser-test workaround. Safe naming cannot repair an unsafe ancestor repository path.

## Impact
`internal/agent` preparation, local checkout resolution and desktop discussion launch; existing repository preparation and cleanup consumers; relevant documentation. No server-owned workstation path or new persistence contract is introduced.

## Accepted Baseline
GitHub #557 and `docs/clarifications/557.md`, round 2. The owner accepted macro inclusion and legacy preservation. No product questions remain open.
