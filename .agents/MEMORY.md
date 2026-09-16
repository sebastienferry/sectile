# Project Knowledge Base (MEMORY.md)

## Architecture Decisions & Constraints

### 1. Web Server HTTP API Only (No Workstation CLIs)
- **Constraint**: The web server (`cmd/server`, `internal/handlers`, `internal/db`, `internal/trackerapi`) must access remote issue trackers (GitHub) solely via HTTP REST and GraphQL APIs through `tasks/internal/trackerapi.Client`.
- **Enforcement**:
  - Web server packages must never execute `os/exec` or workstation CLI binaries (`gh`, `git`).
  - Runtime boundary tests (`cmd/server/runtime_boundary_test.go`) forbid importing `tasks/internal/runner`, `tasks/internal/workspace`, or `tasks/internal/terminal` from `../server`.

### 2. Pluggable Ticketing System Abstraction
- **Pattern**: All issue operations (`CreateIssue`, `GetIssue`, `UpdateIssue`, `DeleteIssue`, `SyncIssues`, `AddComment`, `GetComments`, `Transition`) are defined in `tasks/internal/tracker.TicketingSystem` (extending `tracker.Writer`).
- **Resolution**:
  - `tasks/internal/tracker.Registry` manages registered adapters (`github`, `local`).
  - `DB` provides `d.TrackerForProject(proj)` and `d.TrackerForTask(task)`.
  - Avoid hardcoded `if tracker == "github" ... else if tracker == "jira"` checks in database operations; always route operations through the resolved `TicketingSystem`.
- **Plugging New Trackers**:
  - Implement `tracker.TicketingSystem` (can embed `tracker.BaseTicketingSystem` to inherit default refusal behavior for unsupported capabilities).
  - Implement `FormatTaskID(projectID, key, rawID string) string` on each tracker to encapsulate canonical task ID formatting (e.g. `gh-<projID>-<num>` for GitHub vs a generated UUID locally). Never hardcode tracker-specific ID prefixing in `internal/db`.
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

### 5. Linear, `projectType` and the Daily Digest Removal (2026-09-16)
- **Decision**: Linear was removed end to end: web UI, `internal/trackerapi/linear.go`, the registry entry, the runner's `linear` CLI calls and the `/api/sync/linear` route. The supported trackers are now `github`, `jira` and `local`.
- **Decision**: `models.Project.ProjectType` ("standard" / "personal") was removed with its `NormalizeProjectType` helper.
- **Decision**: the daily digest was removed entirely: `internal/db/digest.go`, the `/api/projects/{id}/daily-digest` routes, the `DailyDigest*` models, `settings.prompt_digest_agenda` and the web `DigestView`. The generic `run_prompt` agent operation stays: it is part of the server-agent v1 contract and is no longer digest-specific.
- **Retired storage is actually dropped**: `db.dropRetiredColumns` runs on every open and removes `projects.linear_team`, `projects.project_type`, `settings.linear_team`, `settings.prompt_digest_agenda` and the `daily_digests` table. The statements are idempotent by failure, like the `ADD COLUMN` migrations above: on a database created after the removal SQLite refuses the `ALTER` and the error is deliberately ignored. `internal/db/retiredstorage_test.go` rebuilds the legacy shape and asserts nothing survives a reopen.
- **Trap to avoid**: the `INSERT`/`SELECT`/`UPDATE` on `projects` and `settings` list their columns explicitly, and their `VALUES (...)` placeholder count must be updated in the same edit. Removing a column name without removing its `?` yields `SQL logic error: N values for M columns` at runtime, which only the DB tests catch.
- **Superseded along the way**: this branch had turned the project parallelism picker into a 1-10 slider. `main` then made parallelism a workstation-only setting (`agentconfig.MaxParallelism`), removing the project field entirely, so the slider was dropped on merge rather than restored. The project-level workflow settings `main` added at the same time (`defaultSkillMode`, `fullChainStopStage`) live in the new **Agentic workflow** tab, beside Create PR/MR.
