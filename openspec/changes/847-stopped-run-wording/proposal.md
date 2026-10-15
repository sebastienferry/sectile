# #847 — Stopped run wording

## Why
Execution outcomes currently use cancellation wording, which obscures that a conversation was stopped. The accepted clarification requests consistent stopped wording across web and desktop while keeping the existing protocol compatible.

## What changes
- Render the canceled execution outcome as “Stopped” in English and “Arrêtée” / “Arrêtées” in French, with grammatical agreement and each surface's current language.
- Cover state badges, tooltips, announcements, notifications, statistics, skill outcome labels, and generated console-unavailable messages describing this outcome.
- Include the owner's specification-stage addition: rename the desktop “Stop execution” action to “Stop conversation”, including its accessible name and tooltip.
- Apply display wording to existing records when rendered; preserve historical free-text logs.
- Add a user-facing Unreleased changelog entry during implementation.

## Scope boundaries
No change to stored/API/MCP status canceled, behavior, authorization, history visibility, glyphs, colors, or workflow stages. Tracker-defined task statuses and the Canceled board preset, OAuth, synchronization, dismissed dialogs, and the separate Stop agent action remain outside scope. No schema migration or new service is needed.

## Impact
Web and desktop presentation and their regression coverage. No implementation is included in this change. No product questions remain; the latest owner message supplies the desktop action wording.
