# Project Knowledge Base (MEMORY.md)

## Architecture Decisions & Constraints

### 1. Web Server HTTP API Only (No Workstation CLIs)
- **Constraint**: The web server (`cmd/server`, `internal/handlers`, `internal/db`, `internal/trackerapi`) must access remote issue trackers (GitHub, Linear) solely via HTTP REST and GraphQL APIs through `tasks/internal/trackerapi.Client`.
- **Enforcement**:
  - Web server packages must never execute `os/exec` or workstation CLI binaries (`gh`, `linear`, `git`).
  - Runtime boundary tests (`cmd/server/runtime_boundary_test.go`) forbid importing `tasks/internal/runner`, `tasks/internal/workspace`, or `tasks/internal/terminal` from `../server`.

### 2. Pluggable Ticketing System Abstraction
- **Pattern**: All issue operations (`CreateIssue`, `GetIssue`, `UpdateIssue`, `DeleteIssue`, `SyncIssues`, `AddComment`, `GetComments`, `Transition`) are defined in `tasks/internal/tracker.TicketingSystem` (extending `tracker.Writer`).
- **Resolution**:
  - `tasks/internal/tracker.Registry` manages registered adapters (`github`, `linear`, `local`).
  - `DB` provides `d.TrackerForProject(proj)` and `d.TrackerForTask(task)`.
  - Avoid hardcoded `if tracker == "linear" ... else if tracker == "github"` checks in database operations; always route operations through the resolved `TicketingSystem`.
- **Plugging New Trackers**:
  - Implement `tracker.TicketingSystem` (can embed `tracker.BaseTicketingSystem` to inherit default refusal behavior for unsupported capabilities).
  - Implement `FormatTaskID(projectID, key, rawID string) string` on each tracker to encapsulate canonical task ID formatting (e.g. `gh-<projID>-<num>` for GitHub vs UUID for Linear). Never hardcode tracker-specific ID prefixing in `internal/db`.
  - Register the adapter in `trackerapi.NewDefaultRegistry` or via `registry.Register(name, adapter)`.
  - Sync routines (`processSyncJob`) dynamically resolve tracker adapters from `d.TrackerRegistry().Get(trackerName)`, allowing any registered tracker with `CapSync` to synchronize without modifying core switch statements.

### 3. Project Settings Are Runtime State, Not Repository Artifacts
- **Constraint**: Per-project settings exposed on `models.Project` (`specFramework`, `prCreationStage`, `aiProvider`, `issueTracker`, ...) live **only** in the Sectile SQLite database. No repository file carries them: not `.taskflow/agent.json` (which stores the agent-side project/path mapping and provider defaults), not `AGENTS.md`, not a committed seed.
- **Consequence**: A ticket asking to correct such a setting is a **data fix with no diff and therefore no pull request**. Do not scaffold a specification or invent a code change to satisfy a PR-shaped workflow; report the absence of a deliverable instead. Ticket #117 (`specFramework: speckit -> openspec`) is the reference case.
- **How to read and mutate**:
  - `GET /api/projects` and `GET /api/projects/{id}` return the stored values.
  - `PUT`/`PATCH /api/projects/{id}` decodes `models.UpdateProjectRequest`, whose fields are pointers, so a partial payload such as `{"specFramework":"openspec"}` updates that single field and leaves the rest untouched.
- **Resolution order for the SDD framework** (`db.resolveSpecFrameworkTarget`): explicit request field, then the project setting, then global settings, then `models.NormalizeSpecFramework` — which falls back to `speckit` for empty or unknown values. Repository layout is **never** consulted, so the configured framework can silently disagree with the repository's actual `openspec/` or `.specify/` tree.

### 4. MCP Session Ownership (ADR 0007)
- **Constraint**: `/mcp` is served **statefully** (`mcp.StreamableHTTPOptions{JSONResponse: true, SessionTimeout: ...}` in `internal/handlers/agent_api.go`). Do not restore `Stateless: true`: a stateless endpoint builds a throwaway session per request, so the server can neither tell two clients apart nor observe a disconnection, which is what closes abandoned runs.
- **Ownership rule**: a run created by `start_run` is adopted by the calling session (`taskmcp.SessionRegistry`) and closed as `canceled` when that session ends. A run **reused** through `runId`/`SECTILE_RUN_ID` is deliberately **not** adopted — it belongs to the dispatching agent, whose supervisor reports the real process exit (see ADR 0006). Adopting it would cancel an execution that is still running.
- **Non-obvious SDK behaviour** (`modelcontextprotocol/go-sdk` v1.7.0):
  - `StreamableHTTPOptions.SessionTimeout` counts idle time, and **only POST requests reset it** (`sessionInfo.startPOST`). A long-lived GET/SSE stream does not. Clients that must stay connected have to ping; the stdio bridge sets `ClientOptions.KeepAlive` for exactly that reason.
  - Session lifecycle hooks: `ServerOptions.InitializedHandler` gives the start, `ServerSession.Wait()` blocks until the end (client `DELETE`, dropped connection, or idle timeout), and `ServerSession.ID()` is the `Mcp-Session-Id`.
- **Closure status**: the `finish_run` contract accepts only `completed`, `failed` and `canceled`, so a disconnection closes with `canceled` plus an explanatory note rather than a new status value.
- **Restart**: a restart destroys every session, so `NewDB` closes `remote_run` activities whose `action != db.RunActionAgent` (`internal/db/db.go`, next to the rule that fails interrupted server jobs). The action field is the persisted owner marker — `db.RunActionClient` vs `db.RunActionAgent` — and is the only thing a restart can rely on, since sessions live in memory. Do not widen that cleanup to agent-owned runs: their supervisor reconnects and reports the real exit (ADR 0006).
