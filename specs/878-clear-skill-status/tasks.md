# Tasks #878 - Clear, outcome-focused skill status in Sectile Desktop

Order matters: each step leaves the tree buildable and the Desktop and Go
tests green. Tests go with the step they cover. One commit per step.

## 1. Agent: carry the reason

- [x] T1.1 `desktopRunResult`: add `summary`, `error`, `waitingReason`,
      `completedAt`, `disconnected` to `activity` and `successor`; trim summary
      and error to 2000 characters (plan, Data contract; FR3).
- [x] T1.2 (changed, see plan, Implementation notes) Launch input `launchStage`, kept on the desktop run record and
      returned with the run list, persisted with restored runs (plan, Launch
      stage; US3.4).
- Tests: `internal/agent/agent_skill_result_test.go` (fields present, values
  verbatim, `disconnected` only on the disconnect note, nil `completedAt`
  omitted, trimming); a launch test asserting `launchStage` round-trips.
  `go test ./internal/agent/...`.

## 2. Status function

- [x] T2.1 Rewrite `desktop/src/skill-result.mjs` as `primaryStatus(run,
      result, {nextStep})`, one branch per precedence row, row number in a
      comment (US2, FR1, FR2).
- [x] T2.2 Outcome table, pickup outcome from `launchStage`, stageless skills
      (US3).
- [x] T2.3 Reason extraction (error, summary without silence sentences or
      launch placeholder, first line, 120-character truncation, "reason not
      recorded", "reason not available") (US4).
- [x] T2.4 Stop reasons from `cancelRequested` and `disconnected` only (US5).
- [x] T2.5 Wait handling, parked launch, successor rule, diagnostics string
      (US1.4, US2.5, US6, US7).
- Tests: rewrite `desktop/tests/skill-result.ui.cjs`: one test per row 1 to 9
  and each case listed in plan, Desktop tests. `npm test` and
  `npm run test:ui -- tests/skill-result.ui.cjs` in `desktop/`.

## 3. Rendering

- [x] T3.1 Kind glyphs and colours: shared glyphs through `runStateSvg`, a
      Desktop-local `verify` glyph, `data-kind` styles (US1.1, US6.1).
- [x] T3.2 Task row: the leading glyph renders the primary status, the skill
      badge element is dropped from skill rows, tooltip and `aria-label` carry
      label, detail, next and diagnostics; free consoles and discussions keep
      `renderRunState` (US1.1, US1.2).
- [x] T3.3 Status bar and conversation status row: merge `#run-state` and
      `#skill-result` into one status block with a details line; pass the
      `nextTaskStep` label as `nextStep` (US1.1, US1.3, US3.6).
- [x] T3.4 (changed, see plan, Implementation notes: Desktop records it at its first read) Launch request sends `launchStage` from `taskStages` (US3.4).
- Tests: `skill-status-reset.ui.cjs` updated to the merged element, still
  asserting selection and focus are preserved (FR6);
  `run-state-animation.ui.cjs` still passes. `npm run test:ui` in `desktop/`.

## 4. Documentation

- [x] T4.1 Rewrite `desktop/README.md` "Skill result indicator": kinds,
      precedence table, diagnostics, reasons need the updated local agent
      (US8.1). Keep the heading so the anchor stays valid, or update
      `docs/USER_GUIDE.md` with it (US8.2).
- [x] T4.2 `CHANGELOG.md` `[Unreleased]`, `### Changed`: one line, e.g.
      "Desktop shows one status per task execution (Working, Needs your
      answer, Done, Needs verification, Could not finish, Stopped) that names
      the outcome, the recorded reason and the next step, with the process
      and skill details in its tooltip. (#878)" (US8.3).

## 5. Final checks

- [ ] T5.1 `make test` green; `go vet ./internal/agent/...`.
- [ ] T5.2 Manual pass in the Desktop app against a local agent: a running
      skill, a wait, a completion with and without stage, a stop from
      Desktop, a failure with a note, a successor in an open console.
- [x] T5.3 No new `exec.Command` is introduced (AGENTS.md, Windows consoles).
