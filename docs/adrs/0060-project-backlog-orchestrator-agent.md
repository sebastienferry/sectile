# ADR 0060: Project Backlog Orchestrator Agent

- Status: Proposed
- Date: 2026-10-22

## Context

Sectile provides a five-stage agentic workflow (`clarify` $\rightarrow$ `specify` $\rightarrow$ `code` $\rightarrow$ `review/PR` $\rightarrow$ `handoff`) and an autonomous chain engine (`internal/db/chain.go`) capable of progressing a work item from `#new` to `#implemented` (where a draft pull request is published).

However, triggering these workflows has required manual human intervention: either clicking "Start execution" or "Chain run" card-by-card in the UI, or running an interactive batch pickup (`pickup-issues`). There was no orchestrator agent capable of inspecting the backlog, evaluating which tickets are eligible for unattended execution, respecting project-level concurrency limits, and launching parallel task sessions autonomously.

## Decision

- **Per-Project Orchestrator Configuration**:
  Add three project settings in the `projects` table with migration 56, after main's forge OAuth migration 55:
  - `orchestrator_enabled` (boolean, default false): Controls whether the orchestrator agent is activated for the project.
  - `orchestrator_max_concurrent` (integer, default 1): Sets the maximum number of parallel task execution sessions running concurrently for the project.
  - `orchestrator_prompt` (text, default empty): Optional project-specific instructions and guidelines for backlog evaluation and eligibility decisions.

- **On-Demand Orchestration Invocation**:
  Expose on-demand execution via:
  - HTTP endpoint: `POST /api/projects/{id}/orchestrate`
  - MCP tool: `orchestrate_project`
  The orchestrator does not run continuously in an unconstrained loop; it is invoked on demand (or via scheduled triggers) to assess the backlog and fill available execution slots.
  Neither interface accepts an enablement or capacity override. A disabled project answers HTTP 409 and an MCP error; an unknown project answers 404; the HTTP endpoint refuses unknown fields such as `force` or `maxConcurrent` with 400 rather than ignoring them. The only accepted field is `guidelines`, the desktop's per-invocation snapshot.

- **Concurrency & Capacity Guard**:
  Before launching any new task session, the orchestrator locks the project row and checks active runs in the same transaction that inserts the queued launch. PostgreSQL row locks serialize admission across server instances; SQLite uses an immediate write transaction. Existing manual sessions consume slots too (amended by [ADR 0063](0063-only-orchestrated-runs-take-an-orchestrator-slot.md): only the orchestrator's own runs do). The cap is read from the locked project row, never from the caller, so every orchestrator, desktop profile and MCP agent shares the project's own limit, and lowering it applies at the next admission. The job is queued only after the reservation commits. The requesting user is recorded on the launch and carried through every subsequent step, together with the selected workstation.
  A chained step is enqueued by the hand-back without a second admission: its task already held the slot. Between the end of a step and the insertion of the next, another invocation can take that slot, so the project may briefly run one chain over its cap. Closing that gap would mean reserving across steps, which is not worth the complexity at a cap of 1 to 10.

- **Backlog Triage & Autonomy Eligibility**:
  The orchestrator retrieves tickets before `#implemented` in board order. The server applies deterministic title, description and blocker-label filters. Every dispatched step carries a mandatory preflight instruction to evaluate technical readiness and the project's natural-language guidelines before making changes or advancing the ticket:
  - **Eligible**: Tasks with clear, actionable specifications, bounded technical scope, and no blocking dependencies.
  - **Ineligible**: Tasks lacking sufficient details, blocked by external dependencies, or requiring critical product/architectural decisions.
  Server-filtered tickets remain untouched and are listed in the invocation's skipped report. If the agent declines a candidate under its readiness assessment or project guidelines, it ends the run with an explanation without advancing the ticket. The existing chain hand-back then stops instead of starting another step.
  The hand-back repeats a step's prompt only when it opens with the orchestration prefix (`Backlog orchestration:`). An orchestrated chain must carry its gate to every step; a manual chain's launch prompt was written for its first step and is not repeated.

- **Autonomous Chaining to Pull Request**:
  For each eligible ticket up to the available concurrency limit, the orchestrator queues an autonomous chain run configured with `FullChainStopImplemented`, regardless of the project's ordinary full-chain stop setting. The ticket proceeds autonomously through:
  1. `clarify`: Clarifies acceptance criteria and verifies problem statement;
  2. `specify`: Generates executable specifications;
  3. `code`: Implements the change, runs local tests and linters, and pushes the branch;
  4. `#implemented`: Stops for human review. Pull request creation follows the project's configured creation stage.
  Human engineers retain the review and merge gate (`#reviewed` $\rightarrow$ `#finished`).

- **Desktop Visibility**:
  The sidebar's ORCHESTRATION section precedes PROJECTS. Optional project-discovery fields carry authoritative project-wide capacity to the agent and desktop; local live runs supply agent names, stages, waiting states and console access. Capacity refreshes every 15 seconds, with stale and older-server states labeled explicitly. Setup lives in Settings → project → Orchestration. The sidebar appears only for enabled projects or active local task agents, and retains evaluation, monitoring and console access. Enablement and concurrency stay server-owned; named agent definitions stay in workstation project settings (layout 6). Each has a stable ID, unique name and its own relative .md file inside the mapped checkout. The former single-file setup becomes Default agent, retaining its path or AGENTS.md. Add agent, Remove agent and per-agent Run agent controls support multiple policies on one project; all share the same server admission cap. An explicitly empty list remains empty. The agent bounds and validates the file, rejects symlink escapes, and reads only the selected saved agent's file again on Run agent. Its name and ID accompany the policy snapshot. Its contents travel as an invocation-specific guideline snapshot through every chain step, without rewriting the shared web/MCP policy. Missing or invalid files block enablement and evaluation; disabling remains possible. The paired-user bridge binds the workstation identity. Unsaved settings and pending requests block evaluation from settings. The UI reports launches, skips and reasons without offering a force override.

- **Structured Reporting**:
  The orchestrator invocation returns an `OrchestrationResult` summarizing:
  - Total backlog items evaluated;
  - Active runs count before and after;
  - Tickets launched into autonomous sessions;
  - Tickets skipped or deferred with rationale;
  - Remaining concurrency slots.

### Desktop agent overview

The sidebar presents compact saved profiles and live manual task sessions. Selecting a profile opens an operational overview rather than its raw console. Runs are matched by the stable profile ID in the generated mandatory-guidelines snapshot, scoped to their project; a renamed profile retains its history. Earlier single-policy orchestration prompts without a named header map to the migrated `default` profile. The immediate evaluation report also links its returned activity IDs before dispatch metadata arrives. Unmatched live sessions remain visible separately. Setup remains in per-project Settings.

The project counter reports occupied server reservations, including queued work and other workstations. The ordinary workstation execution limit remains available in Settings; its second sidebar counter is hidden while orchestration is enabled, avoiding two competing sidebar limits. Failed provider traces can expose an authentication-required boolean, without exposing raw diagnostics in discovery. Recovery opens the project terminal and tells the user which provider command to run; sign-in and retries remain explicit actions.
