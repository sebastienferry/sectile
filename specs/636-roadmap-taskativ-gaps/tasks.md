# Tasks #636 - Roadmap: smaller gaps found in the Taskativ comparison

Ordered checklist for `plan.md`. Each task names its test. Row 2 and row 47
are independent and can land as separate commits.

## Row 2 - Timeline search

- [ ] T1 Add `web/src/lib/taskQuery.ts` with `SELF_FILTERING_VIEWS` and
      `sendsServerSearch`, and `web/tests/taskQuery.test.mjs` (FR1).
- [ ] T2 Use it in `buildTaskQuery` for the search only; leave the parent
      filter as it is; rewrite the touched comment in English (FR1-FR3).
- [ ] T3 Check by hand on the Timeline: a header search that matches nothing
      leaves sprints, counters and backlog whole; back on the board the search
      applies again (AC2, US1).

## Row 47 - Framing copy, server

- [ ] T4 Migration `macros.framing_mirror` (next free number after checking
      `origin/main`), update the db rewind helpers and replay fixtures (NFR4).
- [ ] T5 `models.MacroMeta.FramingMirror` (FR12).
- [ ] T6 Introduce `macroCopyPart` in `macrotodosmirror.go` and thread it
      through eligibility, state, status, scheduling, push and refusal, with
      the todos part behaving exactly as before. Gate: #663's tests green with
      no change to them.
- [ ] T7 Framing part: eligibility (GitHub milestone is local, D1),
      `renderFramingMirror`, empty and truncation rules; unit tests (FR6,
      FR8, FR9, FR11).
- [ ] T8 `TrackerOpEpicFraming`, its activity texts and `runEpicFramingOp`;
      `PushMacroFramingMirror` with the fake writer tests of the plan (FR7,
      FR10, FR11).
- [ ] T9 Bulk context marker set by the macro PUT handler; `UpdateMacro`
      schedules the framing copy when the framing is in the save and the edit
      is not bulk; scheduling tests (FR4, FR5, US3.3).
- [ ] T10 `fillTodosMirror` also fills `FramingMirror`; `get_macro` answer
      test (FR12).
- [ ] T11 `POST .../framing-mirror` route with 202 / 400 tests (FR13, FR14).
- [ ] T12 `go test ./internal/...` on SQLite, then the db suite on PostgreSQL
      with a throwaway DSN.

## Row 47 - Framing copy, web

- [ ] T13 `framingMirror` on the macro type; `republishMacroFraming` in
      `AppContext.tsx`.
- [ ] T14 Extract the copy status line into a component; render it under the
      todos (unchanged) and under the framing editor (US3, US4).
- [ ] T15 French and English strings in `planning.ts` and the notification
      strings.
- [ ] T16 Browser component test of the framing line states and
      **Republier**; `tsc`, `oxlint`, `npm test`, `npx vite build` (restore
      `webui/.gitkeep`).

## Docs

- [ ] T17 `CHANGELOG.md` under `## [Unreleased]`: an `Added` line for the
      framing comment on Jira epics, a `Fixed` line for the Timeline search
      (AC5).
- [ ] T18 ADR 0046: a section saying the framing of a Jira epic is copied the
      same way, Jira only, as a second marked comment.

## End-to-end check

- [ ] T19 On a Jira test epic (a server started without a token on a copy of
      the database, or a disposable epic with the owner's agreement, since
      branch writes reach the real tracker): ten framing saves leave one
      framing comment beside an untouched todos comment (AC3); a GitHub
      project's macro shows **Reste dans Sectile** (AC4).
