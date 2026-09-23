# Tasks: saved cross-project board views

Spec: [spec.md](spec.md). Plan: [plan.md](plan.md).
Requirement ids (FR-xxx) and scenario numbers (S1 to S18) refer to spec.md.

## 0. Baseline

- [ ] Fetch and compare `feat/387` with `origin/main`; rebase or merge before
      starting, and note the last migration version on `origin/main`.
- [ ] Record baseline results: `go build ./...`, `go vet ./...`,
      `go test ./...`, `npm --prefix web run build`, `npm --prefix web run lint`,
      `npm --prefix web test` (keep warning counts for comparison).

## 1. Storage (US1, US4)

- [ ] Add migration 3 `board_views` (table and both indexes) in
      `internal/db/migrations.go`.
- [ ] Add `models.BoardView`.
- [ ] Implement `internal/db/boardviews.go`: list, get, create, update, delete,
      label normalization, sentinels (not found, name taken, unknown project).
- [ ] Remove a deleted project from every view in `DeleteProject` (FR-052).
- [ ] Tests `internal/db/boardviews_test.go`:
  - [ ] create/list/get round trip, creation order (FR-006);
  - [ ] name required, trimmed, case-insensitive uniqueness per owner, same name
        allowed for two owners (FR-002, S17);
  - [ ] at least one project, unknown project refused, slug resolved to id
        (FR-001, FR-003);
  - [ ] label normalization: trim, empties dropped, case duplicates collapsed to
        the first spelling (FR-004);
  - [ ] a foreign view is not found on get, update and delete (FR-005);
  - [ ] deleting a project removes it from views and keeps an emptied view (S14);
  - [ ] migration applies on a fresh and on a baseline database (existing
        migration test pattern).

## 2. Task selection (US2)

- [ ] Introduce `TaskScope` and route `GetTasksForUser` /
      `GetTaskFacetsForUser` through it without changing existing callers.
- [ ] View scope: view projects (ids and slugs), empty list yields no task.
- [ ] View label predicate after the scan: ANY, whole label, case-insensitive.
- [ ] Facets computed on the same scope and predicate.
- [ ] Tests in `internal/db`:
  - [ ] ANY semantics (S2), whole-label and case (S3), empty labels (S4);
  - [ ] project outside the view never returned (S4);
  - [ ] duplicates of one remote key in two projects both returned (S6);
  - [ ] board filters narrow the view (S9), existing `label` substring filter
        still combines;
  - [ ] container issue types still hidden by default (FR-016);
  - [ ] foreign or missing view returns the not-found sentinel.

## 3. HTTP API (US1, US4)

- [ ] `internal/handlers/boardviews.go`: GET/POST collection, GET/PATCH/DELETE
      item; register in `cmd/server/main.go` behind the session guard.
- [ ] `viewId` on `/api/tasks` and `/api/tasks/facets`, 404 when not the caller's.
- [ ] Handler tests: status codes of the table in plan.md, 401 without session,
      identical 404 for foreign and missing ids (S11), PATCH partial update.
- [ ] Run the db and handler suites against a throwaway PostgreSQL
      (`SECTILE_TEST_POSTGRES_DSN`, never the dev database).

## 4. Web state and navigation (US2, US3)

- [ ] `BoardView` type and API helpers.
- [ ] `boardViews`, `selectedViewId` in `AppContext`; opening a view sets the
      project selection to `'all'`; selecting a project clears it (FR-041).
- [ ] `viewId` in `buildTaskQuery` and the facet fetch.
- [ ] Scope-aware filter storage key `sectile_filters_view_<id>` (FR-031, FR-032).
- [ ] `?view=<id>` written on open, removed on leave, read on startup; unknown
      view falls back with a "Vue indisponible" toast (FR-042, FR-043).
- [ ] 404 during refresh falls back to the all-projects board.
- [ ] Unit tests (`npm --prefix web test`) for the storage key selection and
      the URL parsing helpers, extracted to `web/src/lib/boardViews.ts`.

## 5. Web components (US1, US2, US4, US5)

- [ ] Sidebar: view entries in the "Vues" section, active state, edit
      affordance, "Nouvelle vue" action, collapsed mode (FR-040).
- [ ] `BoardViewModal`: create, edit, delete with confirmation, validation and
      server error display (FR-050, FR-051, S17).
- [ ] Header shows the open view name (FR-024).
- [ ] Project badge on cards while a view is open (FR-023).
- [ ] Empty state for a view with no project, with an edit action (FR-052).
- [ ] Quick add: view projects only, no preselection, required choice; single
      label prefilled, none otherwise (FR-060 to FR-062).
- [ ] Translations in `web/src/locales/translations.ts`.

## 6. Browser verification

- [ ] Browser regression `web/tests/board-views.browser.mjs` (real components
      and styles, mocked API, following the existing `*.browser.mjs` harness):
  - [ ] S1 create then open from the sidebar;
  - [ ] S8 filters remembered per view and per project;
  - [ ] S10 cold load with `?view=<id>`, S11 unknown id toast and fallback;
  - [ ] S12 edit applies immediately, S13 delete falls back;
  - [ ] S15 / S16 quick add project and label rules;
  - [ ] S6 two badged cards for one remote key;
  - [ ] S7 columns identical to the all-projects board for the same tickets;
  - [ ] narrow viewport: sidebar entries and dialog usable at 390 px.
- [ ] Run the app against a local server with two projects and several labels
      and walk through S1 to S16 by hand; attach observations to the PR.

## 7. Documentation

- [ ] `CHANGELOG.md`: `Added` line under `[Unreleased]` (S18).
- [ ] `docs/API_AND_DATA_SPEC.md`: endpoints, `viewId`, `board_views`.
- [ ] `docs/UX_COMPONENTS.md` and `README.md` where they describe the sidebar
      or the feature list.
- [ ] ADR 0025 "Saved board views are personal overlays" (number rechecked).

## 8. Closing checks

- [ ] Rerun every baseline command; compare warnings with the baseline.
- [ ] `git diff --check`; review the full diff against spec.md, requirement by
      requirement.
- [ ] Compare with `origin/main` again (migration number, concurrent changes to
      `AppContext.tsx` and `Sidebar.tsx`).
- [ ] Commit, push `feat/387`, reuse or create the draft pull request per the
      project policy (creation stage: implemented).
