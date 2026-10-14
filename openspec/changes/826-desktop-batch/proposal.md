## Why
Desktop users cannot prepare a sequential pickup batch from existing tickets, although the server and web application support it. Clarification Round 2 in `docs/clarifications/826.md` confirms selection in both Board and Backlog.

## What Changes
- Add ticket selection checkboxes and a batch preparation action to both Desktop surfaces.
- Review and reorder at least two tickets from one project and edit the suggested shared worktree name before launch.
- Launch one sequential pickup batch using existing execution settings, membership checks and console behavior.
- Retain preparation state after refusal so users can correct and retry.

## Capabilities
### New Capabilities
- desktop-batch: selection, preparation and launch of existing-ticket pickup batches.

### Modified Capabilities
None.

## Impact
Desktop renderer, Electron launch bridge, local agent request forwarding and their tests. Reuse existing server batch execution; no migration or external service. Implementation adds a user-facing Added entry under CHANGELOG.md Unreleased.

## Non-goals
Bulk ticket creation, saved batch definitions, sprint creation, scheduling, parallel execution, a new execution-sidebar selection flow, and implementing code during specification.

## Open Questions
None. Owner-confirmed Round 2 supersedes Round 1's table-only assumption.
