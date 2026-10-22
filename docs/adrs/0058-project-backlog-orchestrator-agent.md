# ADR 0058: Project Backlog Orchestrator Agent

- Status: Proposed
- Date: 2026-10-22

## Context

Sectile provides a five-stage agentic workflow (`clarify` $\rightarrow$ `specify` $\rightarrow$ `code` $\rightarrow$ `review/PR` $\rightarrow$ `handoff`) and an autonomous chain engine (`internal/db/chain.go`) capable of progressing a work item from `#new` to `#implemented` (where a draft pull request is published).

However, triggering these workflows has required manual human intervention: either clicking "Start execution" or "Chain run" card-by-card in the UI, or running an interactive batch pickup (`pickup-issues`). There was no orchestrator agent capable of inspecting the backlog, evaluating which tickets are eligible for unattended execution, respecting project-level concurrency limits, and launching parallel task sessions autonomously.

## Decision

- **Per-Project Orchestrator Configuration**:
  Add three project settings in the `projects` table:
  - `orchestrator_enabled` (boolean, default false): Controls whether the orchestrator agent is activated for the project.
  - `orchestrator_max_concurrent` (integer, default 1): Sets the maximum number of parallel task execution sessions running concurrently for the project.
  - `orchestrator_prompt` (text, default empty): Optional project-specific instructions and guidelines for backlog evaluation and eligibility decisions.

- **On-Demand Orchestration Invocation**:
  Expose on-demand execution via:
  - HTTP endpoint: `POST /api/projects/{id}/orchestrate`
  - MCP tool: `orchestrate_project`
  The orchestrator does not run continuously in an unconstrained loop; it is invoked on demand (or via scheduled triggers) to assess the backlog and fill available execution slots.

- **Concurrency & Capacity Guard**:
  Before launching any new task session, the orchestrator locks the project row and checks active runs in the same transaction that inserts the queued launch. PostgreSQL row locks serialize admission across server instances; SQLite uses an immediate write transaction. Existing manual sessions consume slots too. The job is queued only after the reservation commits. The requesting user is recorded on the launch and carried through every subsequent step, together with the selected workstation.

- **Backlog Triage & Autonomy Eligibility**:
  The orchestrator retrieves tickets before `#implemented` in board order. The server applies deterministic title, description and blocker-label filters. Every dispatched step carries a mandatory preflight instruction to evaluate technical readiness and the project's natural-language guidelines before making changes or advancing the ticket:
  - **Eligible**: Tasks with clear, actionable specifications, bounded technical scope, and no blocking dependencies.
  - **Ineligible**: Tasks lacking sufficient details, blocked by external dependencies, or requiring critical product/architectural decisions.
  Server-filtered tickets remain untouched and are listed in the invocation's skipped report. If the agent declines a candidate under its readiness assessment or project guidelines, it ends the run with an explanation without advancing the ticket. The existing chain hand-back then stops instead of starting another step.

- **Autonomous Chaining to Pull Request**:
  For each eligible ticket up to the available concurrency limit, the orchestrator queues an autonomous chain run configured with `FullChainStopImplemented`, regardless of the project's ordinary full-chain stop setting. The ticket proceeds autonomously through:
  1. `clarify`: Clarifies acceptance criteria and verifies problem statement;
  2. `specify`: Generates executable specifications;
  3. `code`: Implements the change, runs local tests and linters, and pushes the branch;
  4. `#implemented`: Stops for human review. Pull request creation follows the project's configured creation stage.
  Human engineers retain the review and merge gate (`#reviewed` $\rightarrow$ `#finished`).

- **Desktop Visibility**:
  The sidebar's ORCHESTRATION section precedes PROJECTS. Optional project-discovery fields carry authoritative project-wide capacity to the agent and desktop; local live runs supply agent names, stages, waiting states and console access. Capacity refreshes every 15 seconds, with stale and older-server states labeled explicitly.

- **Structured Reporting**:
  The orchestrator invocation returns an `OrchestrationResult` summarizing:
  - Total backlog items evaluated;
  - Active runs count before and after;
  - Tickets launched into autonomous sessions;
  - Tickets skipped or deferred with rationale;
  - Remaining concurrency slots.
