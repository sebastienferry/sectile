# Tasks #680 - Map the epic priority and quarter to Jira custom fields

Order matters: each step leaves the tree building and its tests green.
G1 of `specs/635-epic-axes-tracker-fields/tasks.md` is closed by the owner
(`docs/clarifications/680.md`, round 2).

- [x] T1 `internal/models/epicaxisfields.go`: types, `DeduceEpicAxisOptions`,
  `FillEpicAxisOptions`, `OptionFor`, `ValueOf`; table tests on synthetic
  options (US2, FR-3).
- [x] T2 `Project.EpicAxisFields`, `UpdateProjectRequest.EpicAxisFields`,
  `Task.AxisFieldValues`; migration 48 `projects.epic_axis_fields`; project
  statements; rewind helpers (AC3).
- [x] T3 `editEpicAxisFields` with validation and hand-set tracking, wired in
  `UpdateProject` and the handler's 400 list; tests (FR-4, US1.5, US2.4).
- [x] T4 `tracker.EpicAxisFieldManager`; Jira `EpicAxisFieldCandidates`,
  `SetEpicAxisField`, mapped ids in `ListEpics` and their decoding;
  `httptest` tests with synthetic ids, including "no field: same request"
  (FR-2, FR-6, AC2).
- [x] T5 `DB.EpicAxisFieldCandidates` and
  `GET /api/projects/{id}/epic-axis-fields`, "no epic" answer; tests (US1.1,
  US1.2).
- [x] T6 Field write after the label in the priority and quarter pushes;
  learned quarters stored; notes for a missing option and an absent field;
  failure after the label; tests (US3, US5).
- [x] T7 Import field first, pending pushes on disagreement; tests (US4).
- [x] T8 Web: types, `lib/epicAxisFields.ts` and its `node --test`, context
  loader, Tracker tab block, strings in French and English (US1, US2, FR-8).
- [x] T9 Guardrail check: `git diff origin/main | grep -E 'customfield_[0-9]+'`
  finds nothing; no real field or option name (US6, AC6, AC7).
- [x] T10 `CHANGELOG.md` `Unreleased` / `Added`; docs section if one covers
  the axis prefixes.
- [x] T11 Gates: `gofmt -l`, `go vet ./...`, `go test ./...`,
  `npx tsc --noEmit`, `npx oxlint`, `node --test web/tests`. Draft PR.
