# Tasks #582 - Project picker with recents and a project overview

## Pure helpers

- [ ] T1 `lib/projectHistory.ts`: `ProjectOpening`, key, limit,
      `parseProjectHistory`, `recordProjectOpening`, `readProjectHistory`,
      `writeProjectHistory` (FR3, FR4).
- [ ] T2 `tests/projectHistory.test.mjs`:
  - an opening moves to the front and dedupes;
  - the history is capped at 50;
  - malformed JSON, a non-array value, or an entry with no id or with an
    invalid date is parsed safely;
  - a storage that throws on read or on write yields `[]` or a no-op, with no
    exception.
- [ ] T3 `lib/projectPicker.ts`: `projectRepositories`, `repositoryLabel`,
      `trackerLabel`, `matchProject`, `orderProjects`, `pickerModel`,
      `overviewProjects`, `highlightParts`, `descriptionExcerpt` (FR1, FR2,
      FR5, FR6, FR9).
- [ ] T4 `tests/projectPicker.test.mjs`:
  - with an empty query: 3 recents at most, favorites skipped from recents,
    unknown ids ignored, 6 favorites A–Z at most with `hiddenFavorites`, and no
    other project;
  - with a query: favorites first, others by history then A–Z, 6 rows at most
    across both sections, and a `hiddenMatches` count that includes left-out
    favorites (8 favorites plus 2 others gives 6 shown and 4 hidden);
  - matching: accents and case ignored (`equipe` finds "Équipe"); description;
    repository from `repositories`, `gitRemoteUrl`, `githubRepo` and
    `gitlabProject`; the tracker label; `jiraProject`; the first field reported
    in priority order;
  - `repositoryLabel` for scp-like and https remotes;
  - `highlightParts` on an accented text maps back to the original characters;
  - `descriptionExcerpt` cuts with an ellipsis;
  - `overviewProjects` combines the tracker and text filters.

## State

- [ ] T5 `AppContext.tsx`:
  - `projectHistory` state, initialized with `readProjectHistory()`;
  - `setSelectedProjectId` records every id other than `'all'`, in the state
    and in storage;
  - `isProjectOverviewOpen`, `projectOverviewQuery`, `openProjectOverview`
    and `closeProjectOverview` are exposed in the context type and value.
  - Check that every `setSelectedProjectId` caller matches FR3.

## UI

- [ ] T6 Strings: add the plan's keys to `fr` and `en` in `locales/shell.ts`,
      remove `noFavorite`, and extend `tests/shellCatalog.test.mjs` with the
      English "Browse projects…", "Recent" and overview title (FR10).
- [ ] T7 `components/ProjectPicker.tsx`:
  - sections from `pickerModel`, and the rows with a highlighted name and a
    second line;
  - the "more" links, and the empty result explaining what search covers;
  - the "All projects", "Browse projects…" (with count) and "New project…"
    entries;
  - no scroll container anywhere, and truncation on every line (US1, US3,
    US6, FR7).
- [ ] T8 Keyboard and ARIA in `ProjectPicker`:
  - combobox, listbox, options and `aria-activedescendant`;
  - focus on open, ↑/↓ that wrap, Enter, and two-step Esc that gives the focus
    back to the switcher;
  - `stopPropagation` on the handled keys (US4, FR8).
- [ ] T9 `Sidebar.tsx`: replace the inline dropdown with `ProjectPicker`,
      pass the switcher button ref, and delete `searchBookmarked` /
      `searchOthers`. The collapsed button is unchanged.
- [ ] T10 `components/ProjectOverviewModal.tsx`:
  - `useBackdropDismiss`, `aria-modal`, Esc and a close button;
  - a text filter focused on open and seeded from `projectOverviewQuery`;
  - tracker chips with counts;
  - the card grid ordered by `overviewProjects`, with a star toggle, "opened
    {elapsed}" from the history, Enter/Space/click to open, and an empty state
    (US5, FR9).
- [ ] T11 `App.tsx`: render `ProjectOverviewModal` next to `ProjectModal`.

## Docs

- [ ] T12 `CHANGELOG.md` `[Unreleased]` → `Changed`: one line for the new
      project picker (recents, search on more fields, keyboard) and the project
      overview (#582) (FR11).

## Browser regression

- [ ] T13 `tests/project-picker.browser.mjs` (real App, faked API with about
      12 projects, 9 of them favorites and a mix of trackers):
  - [ ] Open the picker. Check the search focus, 3 recents at most, 6
        favorites plus the "more favorites" link, and no scrollbar
        (`scrollHeight <= clientHeight` and `scrollWidth <= clientWidth` on the
        menu and every descendant).
  - [ ] Open a project from the picker, reload, reopen, and check that it
        leads "Recent" when it is not a favorite.
  - [ ] Type an accented query that matches only a description. Check the
        excerpt, the highlight and the 6-row cap with the "more matches" link,
        which opens the overview pre-filtered.
  - [ ] With ↓ ↓ Enter, check that the second option opens. Check that Esc
        clears the query and then closes the picker, with the focus back on the
        switcher.
  - [ ] Overview: check the tracker chip filter, that a star toggle is
        reflected in the picker, that Esc closes without changing the project,
        and that Enter on a focused card opens it.
  - [ ] Check that `/` outside a field still focuses the global search, and
        that "All projects" still selects `all`.
  - [ ] Run with `localStorage` access throwing. Check that the picker still
        renders and that openings update Recent within the session.

## Test plan

1. `cd web && npm test`: the new unit tests and all the existing ones pass.
2. `cd web && npx tsc --noEmit` (or the project's type-check and build
   script) is clean.
3. `node tests/project-picker.browser.mjs` passes with Playwright available.
   Also rerun `tests/sidebar-shortcut.browser.mjs` and
   `tests/i18n-shell.browser.mjs`, which exercise the sidebar and the shell
   strings.
4. Manual check in French and English, in light and dark themes, at a narrow
   sidebar width: nothing overflows and no scrollbar appears in the menu.
