## Why

Desktop users need a direct way to start a project session with reusable agent instructions. The existing Project prompt menu selects a workstation engine, but cannot select or create a repository-owned prompt profile.

## What Changes

- Add an accessible plus action beside each project, opening the project-session dialog.
- Select an optional repository agent profile independently from the existing engine selector.
- Create a named profile and its prompt directly in Desktop, save it in the mapped repository, and select it after successful creation.
- Apply the chosen instructions to the new console or conversation only, preserving repository instructions and project-folder context.
- Report discovery, save, unsupported-provider, and launch failures without falsely claiming success.

## Capabilities

### New Capabilities
- `project-agent-profiles`: discover, create, select, and load repository-owned agent instructions in taskless Desktop project sessions.

### Modified Capabilities
None.

## Impact

Desktop renderer and Electron bridge; local agent profile filesystem operations, console launch and Claude/Codex conversation initialization; focused tests and user documentation. No server database migration or infrastructure change.

## Non-goals

Editing/deleting profiles, automatically committing or pushing profiles, creating tasks, submitting an initial user request, changing profile or engine defaults, and implementing native provider-specific profile registries.

## Traceability

Task: gh-7f9fa863-3320-4e5d-a2d6-d14f7b3c35cf-869.
Issue: https://github.com/sebastienferry/sectile/issues/869.
Baseline: owner-confirmed clarification round 4. No open product questions.
