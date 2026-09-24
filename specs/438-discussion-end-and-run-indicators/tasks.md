# Tasks — ending a discussion, and readable run indicators

1. [ ] `agent_run.go`: `stoppedStatus`; use it in `handleRunControl`.
2. [ ] `agent_desktop.go`: use it in `/desktop/stop` and
       `recoverOrphanedPTYRun`, with the discussion note.
3. [ ] Go tests: a stopped discussion ends `completed` through the supervised
       exit and through orphan recovery; a stopped skill run stays `canceled`;
       a discussion exiting non-zero stays `failed`.
4. [ ] `skill-result.mjs` + `refreshSkillResult`: no badge for a discussion;
       unit test cases in `skill-result.ui.cjs`.
5. [ ] `main.js`: `Process:` / `Skill:` tooltips and accessible names.
6. [ ] `shared/runStates.ts`: filled dot, `pulses`, `color` on the root SVG;
       `web/tests/runStates.test.mjs` covers the markup.
7. [ ] `style.css`: pulse instead of spin, still under reduced motion;
       update `run-state-animation.ui.cjs`.
8. [ ] Web badges: `motion-safe:animate-pulse` for the running glyph.
9. [ ] Placement: state before title in the row and the header; UI test.
10. [ ] `CHANGELOG.md`: lines under `[Unreleased]`.
11. [ ] Gate: Go build/vet/tests; desktop `npm test`, build and UI tests;
        web `npm test`, `tsc`, `oxlint`.
