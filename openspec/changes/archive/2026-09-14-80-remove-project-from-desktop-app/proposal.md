# Disconnect a project from the desktop workstation

## Why

Issue #80 requests removing a project from the desktop app. The desktop can add a local repository mapping but cannot disconnect it. The existing web deletion removes the server project and reassigns its tasks, which does not match the requested local action.

## What Changes

- Add a confirmed Remove from desktop action in Local project settings.
- Remove the project's workstation mapping and execution overrides, persist its disconnected state, and prevent local execution until explicit re-add.
- Block removal while the project has queued, preparing, running, or not-yet-exited executions. Never stop them implicitly.
- Hide disconnected projects and their consoles, reset stale selections, and permit re-add through project discovery.
- Preserve server/tracker data, repository files, worktrees, deployed tooling, unrelated settings, and existing finished console history.

## Capabilities

### New Capabilities
- `desktop-project-disconnection`: persistent workstation disconnection, execution exclusion, and explicit re-add.

### Modified Capabilities
None.

## Impact

Desktop renderer and Electron bridge; authenticated local-agent project operations, execution admission, repository resolution, and workstation settings. Focused Go and desktop UI tests and desktop/API documentation will change. No server database migration is required.

## Out of Scope

Server project deletion, tracker mutation, file or worktree cleanup, automatic execution cancellation, persistent console storage, and changes to other workstations.

## Decision Source

User-confirmed clarification in `docs/clarifications/80.md`: disconnect locally and block removal during active executions. No open product questions remain.
