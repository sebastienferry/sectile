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
  Before launching any new task sessions, the orchestrator queries the active runs on the project (tasks carrying queued, pending, or running executions). If the count of active runs meets or exceeds `orchestrator_max_concurrent`, the orchestrator immediately yields without launching new work.

- **Backlog Triage & Autonomy Eligibility**:
  The orchestrator retrieves candidate backlog tickets (items in `#new` / unstarted status, respecting backlog and priority order). For each candidate, the orchestrator evaluates eligibility:
  - **Eligible**: Tasks with clear, actionable specifications, bounded technical scope, and no blocking dependencies.
  - **Ineligible**: Tasks lacking sufficient details, blocked by external dependencies, or requiring critical product/architectural decisions.
  Ineligible tasks are left untouched on the board, and a note is appended to their activity log stating why autonomous execution was not initiated.

- **Autonomous Chaining to Pull Request**:
  For each eligible ticket up to the available concurrency limit, the orchestrator launches an autonomous chain run (`LaunchChainRunOnTask`) configured with `FullChainStopImplemented`. The ticket proceeds autonomously through:
  1. `clarify`: Clarifies acceptance criteria and verifies problem statement;
  2. `specify`: Generates executable specifications;
  3. `code`: Implements the change, runs local tests and linters, and pushes the branch;
  4. `#implemented`: Opens the pull request and stops.
  Human engineers retain the review and merge gate (`#reviewed` $\rightarrow$ `#finished`).

- **Structured Reporting**:
  The orchestrator invocation returns an `OrchestrationResult` summarizing:
  - Total backlog items evaluated;
  - Active runs count before and after;
  - Tickets launched into autonomous sessions;
  - Tickets skipped or deferred with rationale;
  - Remaining concurrency slots.
