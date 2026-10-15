# Implementation checklist

- [x] 1. Inventory canceled execution presentation across shared, web and desktop consumers; distinguish unrelated cancellation flows.
- [x] 2. Update shared label/announcement and existing English/French execution translations, including statistics, tooltips and stop success feedback; preserve all identifiers and visual metadata.
- [x] 3. Update desktop skill outcome and generated console-unavailable wording; rename Stop execution to Stop conversation including accessibility text without changing its handler or availability.
- [x] 4. Extend regression tests for canceled fixtures, old structured history, French/English wording, shared notification text, and desktop stop control semantics; retain compatibility assertions for canceled and unchanged styling.
- [x] 5. Add one English user-facing Changed entry under CHANGELOG.md Unreleased covering the stopped outcome and desktop conversation action.
- [x] 6. Run affected web and desktop tests, npm --prefix web test, npm --prefix web run lint, npm --prefix web run build, npm --prefix desktop test, npm --prefix desktop run build, and the relevant desktop UI tests. Review the diff for accidental wire-value, historical-log, tracker-status, or unrelated cancellation changes.
- [x] 7. Update the existing draft PR with implementation and validation evidence; leave workflow review to the adjustment stage.
