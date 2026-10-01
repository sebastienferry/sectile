# Tasks #633 - Roadmap: let a person decide an epic's readiness

Ordered checklist. Each group is one commit and leaves the tree buildable and
the tests green. Groups 1 to 8 are Part A and do not depend on #628; group 9 is
Part B and starts only once #628 is merged on `main`.

## 1. Storage (FR1)

- [x] T1.1 Migration 37 `macros.readiness` in `internal/db/migrations.go`
  (renumber if `main` took 37).
- [x] T1.2 Rewind helpers drop `macros.readiness` wherever they drop
  `macros.quarter` (`migrations_test.go`, `activerun_test.go`,
  `branchformat_test.go`, `specartifacts_test.go`, `stagecommits_test.go`),
  and the forget-and-replay fixtures follow.
- [x] T1.3 `models.MacroMeta.Readiness`.
- [x] T1.4 Every macro read in `internal/db/macros.go` selects and scans the
  column; the move between projects carries it.
- [x] T1.5 `SaveMacroAxes` takes `readiness *string`; its callers updated.
- [x] T1.6 Tests: save, read back and clear a level, other macro fields and
  axes untouched; a moved macro keeps its level; SQLite and PostgreSQL.

## 2. Axis vocabulary (FR8, FR10)

- [x] T2.1 `macroaxes.go`: `ReadinessLabelPrefix`, `ReadinessLevels`,
  `NormalizeReadiness`, `ReadinessLabel`, `AllReadinessLabels`,
  `ReadinessFromLabels`.
- [x] T2.2 `macrolabels.go`: `readiness:` joins `macroAxisPrefixes`.
- [x] T2.3 Table-driven tests: accepted forms (`ready`, `Ready`,
  `readiness:ready`, `#Readiness:Ready`) and refused ones (`soon`, `done`,
  `readiness:soon`); two levels read as the most advanced; unknown label read
  as absent (US5.2, US5.4); `IsMacroAxisLabel("readiness:idea")` holds.

## 3. Tracker writes (FR5, FR6, FR7, FR9)

- [x] T3.1 `PushMacroReadinessLabel`.
- [x] T3.2 `TrackerOpEpicReadiness`, `TrackerOp.Readiness`, activity texts and
  runner case, with the "set in Sectile but not on the ticket" failure wording.
- [x] T3.3 `pendingAxisPushes` and `PushPendingHorizons` cover the readiness,
  failures named `<key> (readiness)`.
- [x] T3.4 Tests with a fake tracker: set adds the target and removes the other
  two; clear removes all three; only labels are sent (AC4); milestone, foreign
  epic and non-labelled tracker never pushed nor pending (FR6); a macro whose
  only differing axis is the readiness is pushed on that axis only.

## 4. Read-back (FR8, US5)

- [x] T4.1 `ImportMacroHorizons` stores the level read from the labels when
  present, keeps the local one otherwise, extends its summary count.
- [x] T4.2 Tests: tracker level replaces a different local one; no label keeps
  the local one and makes it pending; two labels read as the most advanced;
  the read sends no write.

## 5. API (FR4, FR6)

- [x] T5.1 Macro handler accepts `readiness`, refuses an invalid value with 400
  and saves nothing, enqueues only when writable.
- [x] T5.2 Handler tests: valid level queued on a Jira project; invalid level
  refused; GitHub milestone saved with no op queued and the "kept in Sectile"
  note.

## 6. Web logic (FR2, FR10)

- [x] T6.1 `types/index.ts`: `EpicReadiness`, `MacroMeta.readiness`.
- [x] T6.2 `epicAxes.ts`: `EPIC_READINESS`, `suggestReadiness`.
- [x] T6.3 `roadmap.ts`: `EpicRow.readiness` and `suggestedReadiness`;
  `'readiness:'` in `EPIC_AXIS_LABEL_PREFIXES`.
- [x] T6.4 `web/tests/epicAxes.test.mjs`: each FR2 rule and the edge cases (all
  tickets closed gives Ready; a line with a story but unticked is covered; a
  framing with an empty slicing gives Shaping; whitespace framing counts as
  none; nothing gives Idea); `freeEpicLabels` hides `readiness:ready` and
  `#Readiness:Idea`.

## 7. Web UI (US2, US3, US4, FR3, FR4, FR11, FR13)

- [x] T7.1 `readinessBadge` on the expanded and condensed rows, decided and
  suggested styles, tooltip.
- [x] T7.2 Tickets' stage badge: prefixed values and tooltip.
- [x] T7.3 Panel chip group next to the priority, second click clears,
  suggestion highlighted with `?`; `saveAxes` and the `saveMacroMeta` patch
  type widened.
- [x] T7.4 `planning.ts` and `operations.ts`: every new or changed string in fr
  and en; `activityText.test.mjs` samples; translation check.
- [x] T7.5 `web/tests/roadmap-readiness.browser.mjs` (opt-in Playwright): badge
  shows the suggestion with `?`; a chip click decides and the badge follows; a
  second click clears; the "kept in Sectile" line on a GitHub project; the
  tickets' stage badge reads "Tickets: ...".

## 8. Documentation (FR14)

- [x] T8.1 `CHANGELOG.md` under `[Unreleased]`, `Added`: the epic readiness
  level, suggested until a person decides it from the panel, written as a Jira
  label, and the tickets' stage badge now reading "Tickets: ..." (#633). Once
  Part B lands, the line also names the readiness grouping and drop.
- [x] T8.2 `docs/API_AND_DATA_SPEC.md`: the activity queue names the readiness
  label.

## 9. Part B, after #628 (US6, US7, FR12)

- [ ] T9.1 Check that #628 is merged on `main` and merge `origin/main` into
  `feat/633`; read #628's axis, section, folding and drop code before writing.
- [ ] T9.2 Add the `readiness` axis to #628's axis type, control and stored
  preference (unknown value falls back to no grouping).
- [ ] T9.3 Sections Idea, Shaping, Ready always shown; "Not decided" only when
  non-empty; grouping by decided level only; folding keys
  `readiness:<level>` and `readiness:none`.
- [ ] T9.4 Drop sets the level, "Not decided" clears it, already-at-target
  epics skipped, #628's report.
- [ ] T9.5 Unit tests of the section builder for the readiness axis (order,
  empty levels kept, "Not decided" hidden when empty, suggestion ignored), and
  the browser test extended with a drop on Shaping and on "Not decided".
- [ ] T9.6 Changelog line extended (T8.1).

## Test plan

- `go test ./internal/db/... ./internal/handlers/...` on SQLite, then with
  `SECTILE_TEST_POSTGRES_DSN` pointing at a throwaway database (never the dev
  one).
- `node --test web/tests/*.test.mjs`, the web typecheck and lint.
- The browser test `web/tests/roadmap-readiness.browser.mjs` with the
  Playwright module of the main checkout.
- Manual, against a copy of the dev database and without a tracker token
  first: decide, clear and read the suggestion on a GitHub project (level stays
  local, no activity queued); then on a Jira sandbox epic: labels added and
  removed as in US3.4, read back after editing a label on Jira (US5). After
  #628: group by readiness and drop on each section.
