# Tasks — ending a discussion, and readable run indicators

1. [x] `agent_run.go`: `stoppedStatus`; use it in `handleRunControl`.
2. [x] `agent_desktop.go`: use it in `/desktop/stop` and
       `recoverOrphanedPTYRun`, with the discussion note.
3. [x] Go tests: a stopped discussion ends `completed` through the supervised
       exit and through orphan recovery; a stopped skill run stays `canceled`;
       a discussion exiting non-zero stays `failed`.
4. [x] `skill-result.mjs` + `refreshSkillResult`: no badge for a discussion;
       unit test cases in `skill-result.ui.cjs`.
5. [x] `main.js`: `Process:` / `Skill:` tooltips and accessible names.
6. [x] `shared/runStates.ts`: filled dot, `pulses`, `color` on the root SVG;
       `web/tests/runStates.test.mjs` covers the markup.
7. [x] `style.css`: pulse instead of spin, still under reduced motion;
       update `run-state-animation.ui.cjs`.
8. [x] Web badges: `motion-safe:animate-pulse` for the running glyph.
9. [x] Placement: state before title in the row and the header; UI test.
10. [x] `CHANGELOG.md`: lines under `[Unreleased]`.
11. [x] Gate: Go build/vet/tests; desktop `npm test`, build and UI tests;
        web `npm test`, `tsc`, `oxlint`.
