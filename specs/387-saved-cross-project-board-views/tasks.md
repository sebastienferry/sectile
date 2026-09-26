# Tasks: saved cross-project board views

Spec: [spec.md](spec.md). Plan: [plan.md](plan.md).
Requirement ids (FR-xxx) and scenario numbers (S1 to S18) refer to spec.md.

## 0. Baseline

- [x] Fetch and compare `feat/387` with `origin/main`; rebase or merge before
      starting, and note the last migration version on `origin/main`.
- [x] Record baseline results: `go build ./...`, `go vet ./...`,
      `go test ./...`, `npm --prefix web run build`, `npm --prefix web run lint`,
      `npm --prefix web test` (keep warning counts for comparison).

## 1. Storage (US1, US4)

- [x] Add migration 4 `board_views` (table and both indexes) in
      `internal/db/migrations.go`.
- [x] Add `models.BoardView`.
- [x] Implement `internal/db/boardviews.go`: list, get, create, update, delete,
      label normalization, sentinels (not found, name taken, unknown project).
- [x] Remove a deleted project from every view in `DeleteProject` (FR-052).
- [x] Tests `internal/db/boardviews_test.go`:
  - [x] create/list/get round trip, creation order (FR-006);
  - [x] name required, trimmed, case-insensitive uniqueness per owner, same name
        allowed for two owners (FR-002, S17);
  - [x] at least one project, unknown project refused, slug resolved to id
        (FR-001, FR-003);
  - [x] label normalization: trim, empties dropped, case duplicates collapsed to
        the first spelling (FR-004);
  - [x] a foreign view is not found on get, update and delete (FR-005);
  - [x] deleting a project removes it from views and keeps an emptied view (S14);
  - [x] migration applies on a fresh and on a baseline database (existing
        migration test pattern).

## 2. Task selection (US2)

- [x] Introduce `TaskScope` and route `GetTasksForUser` /
      `GetTaskFacetsForUser` through it without changing existing callers.
- [x] View scope: view projects (ids and slugs), empty list yields no task.
- [x] View label predicate as an SQL condition: ANY, whole label,
      case-insensitive (revised from "after the scan", see plan.md).
- [x] Facets computed on the same scope and predicate.
- [x] Tests in `internal/db`:
  - [x] ANY semantics (S2), whole-label and case (S3), empty labels (S4);
  - [x] project outside the view never returned (S4);
  - [x] duplicates of one remote key in two projects both returned (S6);
  - [x] board filters narrow the view (S9), existing `label` substring filter
        still combines;
  - [x] container issue types still hidden by default (FR-016);
  - [x] foreign or missing view returns the not-found sentinel.

## 3. HTTP API (US1, US4)

- [x] `internal/handlers/boardviews.go`: GET/POST collection, GET/PATCH/DELETE
      item; register in `cmd/server/main.go` behind the session guard.
- [x] `viewId` on `/api/tasks` and `/api/tasks/facets`, 404 when not the caller's.
- [x] Handler tests: status codes of the table in plan.md, 401 without session,
      identical 404 for foreign and missing ids (S11), PATCH partial update.
- [x] Run the db and handler suites against a throwaway PostgreSQL
      (`SECTILE_TEST_POSTGRES_DSN`, never the dev database).

## 4. Web state and navigation (US2, US3)

- [x] `BoardView` type and API helpers.
- [x] `boardViews`, `selectedViewId` in `AppContext`; opening a view sets the
      project selection to `'all'`; selecting a project clears it (FR-041).
- [x] `viewId` in `buildTaskQuery` and the facet fetch.
- [x] Scope-aware filter storage key `sectile_filters_view_<id>` (FR-031, FR-032).
- [x] `?view=<id>` written on open, removed on leave, read on startup; unknown
      view falls back with a "Vue indisponible" toast (FR-042, FR-043).
- [x] 404 during refresh falls back to the all-projects board.
- [x] Unit tests (`npm --prefix web test`) for the storage key selection and
      the URL parsing helpers, extracted to `web/src/lib/boardViews.ts`.

## 5. Web components (US1, US2, US4, US5)

- [x] Sidebar: view entries in the "Vues" section, active state, edit
      affordance, "Nouvelle vue" action, collapsed mode (FR-040).
- [x] `BoardViewModal`: create, edit, delete with confirmation, validation and
      server error display (FR-050, FR-051, S17).
- [x] Header shows the open view name (FR-024).
- [x] Project badge on cards while a view is open (FR-023).
- [x] Empty state for a view with no project, with an edit action (FR-052).
- [x] Quick add: view projects only, no preselection, required choice; single
      label prefilled, none otherwise (FR-060 to FR-062).
- [x] Translations in `web/src/locales/translations.ts`.

## 6. Browser verification

- [x] Browser regression `web/tests/board-views.browser.mjs` (real components
      and styles, mocked API, following the existing `*.browser.mjs` harness):
  - [x] S1 create then open from the sidebar;
  - [x] S8 filters remembered per view and per project;
  - [x] S10 cold load with `?view=<id>`, S11 unknown id toast and fallback;
  - [x] S12 edit applies immediately, S13 delete falls back;
  - [x] S15 quick add project and label rules (S16 is covered by the
        `initialLabelsForView` unit test);
  - [x] S6 two badged cards for one remote key;
  - [x] S7 columns identical to the all-projects board for the same tickets
        (inspected on a screenshot, not asserted);
  - [x] narrow viewport: the dialog fits at 390 px.
- [ ] Run the app against a local server with two projects and several labels
      and walk through S1 to S16 by hand; attach observations to the PR.
      Not done: the local server answers 401 without a signed-in account, so
      the walk-through ran against the real App with a faked API instead (§6).

## 7. Documentation

- [x] `CHANGELOG.md`: `Added` line under `[Unreleased]` (S18).
- [x] `docs/API_AND_DATA_SPEC.md`: endpoints, `viewId`, `board_views`.
- [x] `docs/UX_COMPONENTS.md` and `README.md` where they describe the sidebar
      or the feature list.
- [x] ADR 0025 "Saved board views are personal overlays" (number rechecked).

## 8. Closing checks

- [x] Rerun every baseline command; compare warnings with the baseline.
- [x] `git diff --check`; review the full diff against spec.md, requirement by
      requirement.
- [x] Compare with `origin/main` again (migration number, concurrent changes to
      `AppContext.tsx` and `Sidebar.tsx`).
- [ ] Commit, push `feat/387`, reuse or create the draft pull request per the
      project policy (creation stage: implemented).
