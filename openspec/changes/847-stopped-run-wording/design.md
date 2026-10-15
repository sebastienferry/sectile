# Technical design

## Context and decisions
Use the initialized OpenSpec framework. The accepted clarification is docs/clarifications/847.md, round 2; the owner added the desktop action rename during specification.

Retain RUN_STATES.canceled and its id in shared/runStates.ts. Change only its label and announcement to “Stopped” and “was stopped”. Keep icon, color, terminal-state resolution, and cancellation visibility logic intact. Reuse existing translation keys rather than rename protocol identifiers or introduce a second status.

Audit presentation consumers in web/src/components/ActivitiesView.tsx, web/src/components/RemoteRunBadge.tsx, the remote-run label definitions in web/src/types/index.ts, and web/src/locales/translations.ts. Use “Stopped” for English counts/statuses and grammatically agreed French text such as “Arrêtées”, “Exécution distante arrêtée”, and “Exécution arrêtée avec succès”. Include success feedback that describes the finished stop action; do not rename unrelated cancel controls.

Desktop shared-state consumers include notifications.mjs and run-state rendering. Update desktop/src/skill-result.mjs to “Skill stopped” and desktop/src/run-console.mjs to describe an execution stopped before a console was created. In desktop/src/main.js, replace the Stop execution control's wording with “Stop conversation” in its label, accessible name, and tooltip wherever supplied. Retain its handler, availability, pending-state logic, and request payload. The pending “Stopping execution” wording is not a terminal status and does not require a behavior change.

## Contracts and compatibility
No new status named stopped is introduced. ActivityStatus, terminal checks, CSS class names, request routes, database values, finish_run inputs, and existing callers continue to use canceled. Existing historical structured statuses are relabeled on read; existing free-text output is not mutated. No migration, backend change, or dependency is planned.

## Rejected alternatives
- Renaming canceled on the wire requires unnecessary migration and mixed-version compatibility work.
- A global cancel-word replacement would alter unrelated product flows and tracker terminology.
- Separate per-surface state definitions would drift from the shared vocabulary.

## Verification approach
Extend existing shared run-state, web locale/indicator, desktop skill-result and notification tests to exercise canceled fixtures with the new display text, preserved id and visual metadata. Add a desktop UI assertion for the stop control's accessible name and unchanged stop request. Verify an old canceled record renders the new wording. Check excluded cancellation flows remain unchanged by reviewing the targeted diff. Run the affected tests, web build/lint and desktop build/tests using repository scripts; no backend behavior tests are required unless implementation unexpectedly touches backend code.

## Risks
Hidden presentation copies can retain old wording; audit runtime cancellation strings by context. Grammatical agreement must follow the noun on the existing French surface. A wording-only change must not broaden stop availability or change the cancellation lifecycle.
