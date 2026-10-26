# Design

## Context and verified baseline

Worktree branch: `feat/871`. The assigned checkout predates Orchestration. Read-only inspection of `origin/main` (clarification baseline `3d5967b2`, feature commit `93ab0b44`, #859) confirms:

- `orchestrationGroups` uses active task workers as project membership evidence and standalone agents.
- `orchestrationProfileId` parses a server prompt snapshot; `orchestrationProfileRuns` also accepts evaluation `launched[].activityId`. Both identify child work, not orchestration responsibility.
- Configured profiles are workstation policy definitions. `internal/agent/agent_desktop_orchestration.go` evaluates backlog through the server; the returned launches are ticket chains. This is not a persistent dedicated orchestration session.
- `desktopRun` has task, macro, console and session identity, but no verified dedicated orchestration role. A session ID by itself proves no such responsibility.

Use the live project setting, OpenSpec, rather than the stale framework value in the supplied skill. Specification artifacts are not ignored in this worktree. The supplied skill expressly forbids specification-time PR creation despite the live project's `prCreationStage: specified`; retain this discrepancy in the report and defer PR creation rather than silently overriding that instruction.

## Decisions

### Separate configured policy, evaluation and dedicated session

Keep existing policy definitions and evaluation response contracts unchanged. `launched` and `skipped` remain evaluation results; their IDs must not establish session ownership, profile status, console targets or stop targets. Pending and evaluation errors remain visible independently of session history. Successful evaluation without a dedicated session returns to the existing ready state with its evaluation report.

Create a single positive dedicated-session predicate for both project membership and profile association. On the verified baseline no producer supplies authoritative dedicated-session identity, so no current ordinary run qualifies. Keep this result explicit: an empty dedicated-session list is valid and must not lead to fabricating a session from a policy definition or its launched workers.

Before implementation, inspect the integrated baseline for an actual dedicated-session producer. If one exists, reuse its authoritative role and project/profile IDs. If that producer lacks a discovery contract, add only additive, optional explicit role and stable profile identity metadata at creation and carry it through discovery, state updates, persistence and restoration. Choose the exact field names in that implementation to match the discovered producer; do not add an unused producer or speculative API. A missing, unknown or inconsistent role never grants membership, including on restored legacy records. Ordinary task constructors never receive a positive dedicated role from launch origin. Role is separate from phase/status, provider and skill.

### Apply the same boundary everywhere

Use the predicate in `orchestrationGroups`, `orchestrationProfileRuns`, standalone fallback rendering and overview selection validation. Require matching project and stable profile identity for association; names, prompt snapshots and activity IDs are insufficient. Disabled projects show only live genuine sessions, including those whose definitions were disabled or removed. Preserve disconnected-project exclusion and the current bounded finished-session history for otherwise visible agents.

Derive waiting/failure/authentication state and session actions only from eligible sessions. Changing selection or receiving a refreshed run snapshot must invalidate a worker-backed overview and must not expose worker console/stop controls. Preserve evaluation state/report and configuration access. An evaluation's launched count can include workers without treating those workers as sessions.

### Keep execution paths unchanged

Filter at the Orchestration boundary without mutating shared run arrays or filtering discovery globally. `desktop/src/main.js`, `desktop/src/queue.mjs`, server capacity counters and agent scheduling continue to receive ordinary workers. Project-wide usage may include workers even when the session list is empty; do not recompute usage from visible orchestration rows.

## Target files and validation

- `desktop/src/orchestration.mjs`: selectors, state input, overview and action guards, empty-session wording consistent with the existing runtime language.
- `desktop/tests/orchestration.test.mjs`: replace expectations that endorse worker membership; cover the behavior matrix in the delta spec.
- Desktop UI tests using existing fixtures: verify selection refresh, controls and normal worker/queue access.
- Only if an existing dedicated producer requires metadata: `internal/agent/agent_desktop.go`, its actual producer and run-store paths, and focused serialization/restoration tests. No database migration is established as necessary.
- `CHANGELOG.md`: one user-facing Fixed line at implementation time.

Run focused Desktop tests, then `npm test` and `npm run build` in `desktop`. Run relevant existing UI tests for changed interaction paths. If Go contracts change, run focused agent tests including serialization/restoration and the project's required Go checks. Validate this specification with `openspec validate 871-orchestration-agent-membership --strict`.

## Rejected alternatives

- Removing only standalone rows leaves workers in profile histories and actions.
- Prompt parsing, skill allowlists, task presence, active status and launched IDs cannot prove responsibility.
- Hiding every disabled project would lose access to genuine active/waiting sessions contrary to the accepted clarification.
- Creating a persistent orchestrator to populate the pane expands scope and invents capabilities.
- Filtering workers out of discovery or capacity would change execution behavior.

## Risks and open points

Zero open product requirements. The older branch must gain the known feature baseline before implementation. A later baseline may contain an authoritative dedicated-session producer; inspect it rather than assuming one or creating it. Positive dedicated-session fixtures must be clearly identified as contract fixtures when no real producer exists. No code is implemented by this specification.
