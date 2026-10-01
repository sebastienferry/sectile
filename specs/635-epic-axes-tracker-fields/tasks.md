# Tasks #635 - Map the epic axes to tracker fields

Ordered checklist. Each part is one pull request from the task branch
`feat/635`, merged before the next starts; after a squash-merge, the branch is
brought up to date with `origin/main` before the next part, so each later pull
request is a follow-up the task appends. Spec: `spec.md`; design: `plan.md`.

## Part A - Axis label prefixes (PR 1)

- [ ] A1 Add `EpicAxisPrefixes` to `models.Project` and
  `UpdateProjectRequest`; migration 39 `projects.epic_axis_prefixes`; carry the
  column in the project SELECT, INSERT and UPDATE statements.
- [ ] A2 Drop the new column in every rewind test helper that drops
  `roadmap_axis_writes`; run `internal/db` on SQLite and PostgreSQL.
- [ ] A3 Write `CleanEpicAxisPrefixes` (FR-A2) with table tests first: case,
  `#`, whitespace, emptied value, overlap between axes, overlap with
  `roadmap:`, default equal to typed.
- [ ] A4 Call the cleaning from project creation and update; a refusal answers
  400 with the sentence. Test the handler.
- [ ] A5 Make the readers prefix-aware (`PriorityFromLabels`,
  `QuarterFromLabels`, `ReadinessFromLabels`, `isQuarterLabel`) through a
  resolved `axisPrefixes`; keep the bare quarter. Tests for US2.
- [ ] A6 Make the writers prefix-aware (`PriorityLabel`, `QuarterLabel`,
  `ReadinessLabel`, `All*Labels`, the three `PushMacro*Label`). Tests for US3,
  including US3.4 (old prefix left alone).
- [ ] A7 Pass the project's prefixes through `ImportMacroHorizons`,
  `roadmapProjectMacros` and `pendingAxisPushes`. Tests for US4 and US3.5.
- [ ] A8 Make the protected prefixes per project in `macrolabels.go`
  (`isMacroAxisLabel(proj, label)`), update every caller. Tests for US5.1 to
  US5.3.
- [ ] A9 Web: `epicAxisPrefixes` on the `Project` type; per-project
  `epicAxisLabelPrefixes`, `isEpicAxisLabel`, `freeEpicLabels` and the client
  prefix cleaning in `roadmap.ts`; update `EpicLabelEditor.tsx`,
  `RoadmapView.tsx` and other callers. `node --test` for US5.1, US5.4.
- [ ] A10 Web: the Roadmap section of the Tracker tab in `ProjectModal.tsx`,
  three inputs with placeholders, inline errors, shown only when epic labels
  can be written. French and English strings.
- [ ] A11 `CHANGELOG.md`, `Unreleased` / `Added`: a project can name the label
  prefixes of the epic priority, quarter and readiness. Update the roadmap
  documentation under `docs/` if it lists the prefixes.
- [ ] A12 Gates: `go test ./...`, `go vet ./...`, `npx tsc --noEmit`,
  `npx oxlint`, `node --test web/tests`. Open the pull request.

## Part B - Ticket priority mapping (PR 2)

- [ ] B1 Add `PriorityMapping` and `PriorityMappingOption` to `models`; new
  `internal/models/prioritymapping.go` with `Writable`, `WritableLevels`,
  `OptionFor`, `MergePriorityMapping`; table tests first (FR-B2, FR-B3, FR-B5).
- [ ] B2 Migration 40 `projects.priority_mapping`; project statements; rewind
  helpers; validation of a mapping sent by the web (unknown ids, bad level).
- [ ] B3 `tracker.PrioritySchemeReader`; Jira implementation from the create
  screen, else the site list; exported classifier wrapping `jiraPriorityOf`
  and `priorityAt`. Tests on Atlassian default, Server, French, `P1`..`P4`.
- [ ] B4 `db.RefreshPriorityMapping`; call it from the project sync beside the
  board columns; `POST /api/projects/{id}/priority-mapping/refresh` with
  project admin rights. Tests for US6.
- [ ] B5 Jira write path: `priorityFieldFor` takes the project and uses the
  mapping when it is not empty; typed `errGuessedPriority`; update returns it;
  creation and `setPriorityAfterCreate` drop the field and set
  `PriorityNotice`. Tests for FR-B7, FR-B8, FR-B10.
- [ ] B6 `CheckPriorityWritable` at the top of `UpdateTaskBy`, before any local
  write; 422 in the handler; MCP `update_task` returns the sentence. Tests:
  row unchanged, no activity queued, other fields not written (US8.5), non-Jira
  and empty mapping unchanged.
- [ ] B7 Surface `priorityNotice` in MCP `create_task` and in the web creation
  toast.
- [ ] B8 Web: the mapping table in the Tracker tab (levels, guessed badge,
  confirm, preferred option, non-writable levels, refresh). Strings.
- [ ] B9 Web: `handleBulkPriority` continues past a refused ticket and reports
  the refused ones; a helper with a `node --test`.
- [ ] B10 `CHANGELOG.md`, `Unreleased`: `Added` the priority mapping table;
  `Changed` a Jira priority update that the table only guessed is refused,
  and a creation goes out without it.
- [ ] B11 Gates as in A12. Open the pull request.

## Gate before part C

- [ ] G1 After A and B are merged, ask the owner to confirm part C on the
  ticket. Without that confirmation, stop here: A and B stand alone.

## Part C - Axis custom fields (PR 3, only once confirmed)

- [ ] C1 `EpicAxisFields` and `EpicAxisField` on `models.Project`; migration
  41 `projects.epic_axis_fields`; project statements; rewind helpers.
- [ ] C2 Pure deduction functions for the priority and the quarter (flat and
  cascade) with table tests on synthetic options; manual entries never
  replaced.
- [ ] C3 Jira `EpicFieldManager`: candidates from `editmeta` of one epic
  (single and cascading select, detected from the schema `custom` suffix),
  field write and clear, mapped field ids requested by `ListEpics`. `httptest`
  tests with synthetic ids.
- [ ] C4 `GET /api/projects/{id}/epic-axis-fields`, with the "no epic yet"
  answer.
- [ ] C5 Axis pushes write the field after the label; missing quarter looked
  up and stored; missing value named in the activity. Tests for US12.
- [ ] C6 Import and pending pushes read the field first. Tests for US13; a
  project without field makes no extra request (request counter).
- [ ] C7 Web: field pickers and option maps in the Roadmap section. Strings.
- [ ] C8 Guardrail check: `grep -rnE 'customfield_[0-9]+' .` finds nothing
  new; no real field or option name in code, tests, fixtures or docs.
- [ ] C9 `CHANGELOG.md`, `Unreleased` / `Added`: a Jira project can map the
  epic priority and quarter to custom fields.
- [ ] C10 Gates as in A12. Open the pull request.
