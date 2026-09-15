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
