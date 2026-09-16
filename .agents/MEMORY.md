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
- **Constraint**: Per-project settings exposed on `models.Project` (`specFramework`, `prCreationStage`, `aiProvider`, `issueTracker`, `parallelism`, ...) live **only** in the Sectile SQLite database. No repository file carries them: not `.taskflow/agent.json` (which stores the agent-side project/path mapping and provider defaults), not `AGENTS.md`, not a committed seed.
- **Consequence**: A ticket asking to correct such a setting is a **data fix with no diff and therefore no pull request**. Do not scaffold a specification or invent a code change to satisfy a PR-shaped workflow; report the absence of a deliverable instead. Ticket #117 (`specFramework: speckit -> openspec`) is the reference case.
- **How to read and mutate**:
  - `GET /api/projects` and `GET /api/projects/{id}` return the stored values.
  - `PUT`/`PATCH /api/projects/{id}` decodes `models.UpdateProjectRequest`, whose fields are pointers, so a partial payload such as `{"specFramework":"openspec"}` updates that single field and leaves the rest untouched.
- **Resolution order for the SDD framework** (`db.resolveSpecFrameworkTarget`): explicit request field, then the project setting, then global settings, then `models.NormalizeSpecFramework` — which falls back to `speckit` for empty or unknown values. Repository layout is **never** consulted, so the configured framework can silently disagree with the repository's actual `openspec/` or `.specify/` tree.

### 4. Linear, `projectType` and the Daily Digest Removal (2026-09-16)
- **Decision**: Linear was removed end to end: web UI, `internal/trackerapi/linear.go`, the registry entry, the runner's `linear` CLI calls and the `/api/sync/linear` route. The supported trackers are now `github`, `jira` and `local`.
- **Decision**: `models.Project.ProjectType` ("standard" / "personal") was removed with its `NormalizeProjectType` helper.
- **Decision**: the daily digest was removed entirely: `internal/db/digest.go`, the `/api/projects/{id}/daily-digest` routes, the `DailyDigest*` models, `settings.prompt_digest_agenda` and the web `DigestView`. The generic `run_prompt` agent operation stays: it is part of the server-agent v1 contract and is no longer digest-specific.
- **Retired storage is actually dropped**: `db.dropRetiredColumns` runs on every open and removes `projects.linear_team`, `projects.project_type`, `settings.linear_team`, `settings.prompt_digest_agenda` and the `daily_digests` table. The statements are idempotent by failure, like the `ADD COLUMN` migrations above: on a database created after the removal SQLite refuses the `ALTER` and the error is deliberately ignored. `internal/db/retiredstorage_test.go` rebuilds the legacy shape and asserts nothing survives a reopen.
- **Trap to avoid**: the `INSERT`/`SELECT`/`UPDATE` on `projects` and `settings` list their columns explicitly, and their `VALUES (...)` placeholder count must be updated in the same edit. Removing a column name without removing its `?` yields `SQL logic error: N values for M columns` at runtime, which only the DB tests catch.
