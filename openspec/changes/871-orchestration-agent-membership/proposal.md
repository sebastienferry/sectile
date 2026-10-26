# Reserve Desktop Orchestration for orchestration agents

## Why

Ordinary ticket executions appear as orchestration agents, even for disabled projects. Prompt-derived policy links and launched activity IDs also attach workers to configured agent overviews. This duplicates normal execution access and misrepresents who is orchestrating.

Task: https://github.com/sebastienferry/sectile/issues/871. Accepted decisions: `docs/clarifications/871.md`, round 2. No open product questions.

## What Changes

- Show configured orchestration policies and only genuine dedicated orchestration sessions.
- Exclude all ordinary ticket workers from project membership, standalone rows, profile sessions and session-derived status/actions, including workers launched by an orchestrator.
- Keep a disabled connected project visible while a genuine dedicated session is active or waiting; finished history alone does not retain it.
- Preserve policy controls, instruction previews, evaluation pending/error/results and project-wide capacity information.
- Preserve normal worker access, queue ordering, lifecycle and capacity accounting.
- Add an Unreleased Fixed changelog entry during implementation.

## Capabilities

### New Capabilities

- `desktop-orchestration-membership`: role-based membership and session association in Desktop Orchestration.

### Modified Capabilities

None. This change corrects presentation without introducing orchestration capabilities.

## Impact

Primary targets are `desktop/src/orchestration.mjs` and its tests, available on the feature baseline in `origin/main`. Minimal agent discovery/persistence metadata is permitted only if an actual existing dedicated-session producer needs it. No infrastructure changes, new persistent agent service, scheduling redesign, concurrency change or new triage/issue-writing workflow is included. Existing runtime language is preserved.

The assigned branch predates the feature; integrate the feature baseline during implementation preparation while preserving the committed clarification and these specifications. Specification does not merge or rebase it.
