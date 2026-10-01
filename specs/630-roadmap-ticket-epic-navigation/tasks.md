# Tasks #630 - Roadmap: go from a ticket to its epic and back

Ordered checklist. Each group is one commit and leaves the tree buildable.

## 1. Pure rules (FR1, FR4, FR5, FR6, FR7)

- [x] T1.1 Create `web/src/lib/roadmapFocus.ts`: `TICKET_VIEWS`,
  `isTicketView`, `canOpenEpicInRoadmap`, `locateEpic`, `returnView`.
- [x] T1.2 Create `web/tests/roadmapFocus.test.mjs`:
  - `canOpenEpicInRoadmap`: parent key and roadmap enabled → true; empty or
    blank key → false; project without the roadmap view → false; unknown
    project → false.
  - `locateEpic`: `now`, `next`, `later`, `hidden` horizons give their tab; no
    horizon gives `unclassified`; a closed row reports `closed`; an absent key
    gives null; the match is exact (`PE-1` does not match `PE-10`).
  - `returnView`: each ticket view comes back when enabled; triage or timeline
    disabled → board; null → board; a non-ticket view (activities) → board.

## 2. Context wiring (FR3, FR6, FR7, FR8)

- [x] T2.1 `AppContext.tsx`: `activeView` mirror ref and `roadmapOriginView`
  ref; `setActiveView` records the origin when entering the roadmap from a
  ticket view.
- [x] T2.2 `roadmapFocus` state, `openEpicInRoadmap(task)`,
  `consumeRoadmapFocus()`, `openEpicTickets(epicKey)`, added to the context
  type and value.
- [x] T2.3 `buildTaskQuery`: no `macro` parameter while `activeView ===
  'roadmap'`, with a comment next to the search exception.

## 3. Roadmap arrival and return (FR4, FR5, FR6)

- [x] T3.1 `RoadmapView.tsx`: `macrosFor` project id stored with the fetched
  macros.
- [x] T3.2 Arrival effect: consume, locate, then select, tab, closed toggle,
  clear search, labels, priority, "only issues", show panel; or the error toast
  and the return to `from`.
- [x] T3.3 Panel button "Open its tickets", shown when the epic has tickets.

## 4. Entry points and chip (FR1, FR2, FR9)

- [x] T4.1 `TaskCard.tsx`: menu entry beside "Filter by parent".
- [x] T4.2 `ListView.tsx`: icon in the action cell, before pin.
- [x] T4.3 `TaskDetailModal.tsx`: button after the parent key's copy button;
  closes the detail first.
- [x] T4.4 `Header.tsx`: parent filter chip hidden on the roadmap.

## 5. Strings and changelog (FR10, FR11)

- [x] T5.1 `translations.ts`: the keys of plan section 7, in the type, `fr`
  and `en`.
- [x] T5.2 `CHANGELOG.md`: the `Added` and `Changed` lines under
  `[Unreleased]`.

## 6. Browser regression

- [x] T6.1 Create `web/tests/roadmap-epic-navigation.browser.mjs`, on the
  `roadmap-view.browser.mjs` harness (`useApp` mocked through `window.ctx`,
  `roadmapFocus` and `consumeRoadmapFocus` in the mock):
  - a focus request on a LATER epic while NOW is remembered opens LATER with
    that epic selected and the panel shown (US2.1, US2.5);
  - a request on a closed epic with closed epics hidden shows it (US2.3);
  - an active priority filter and label filter are cleared (US2.4);
  - the grouping, sort and row shape stored before are unchanged (US2.6);
  - the request is consumed once: changing tab afterwards does not reselect
    the epic (US2.8);
  - an unknown key gives the error toast naming it and calls `setActiveView`
    with the origin, with the selection and tab unchanged (US3);
  - "Open its tickets" is shown on an epic with tickets, absent on an empty
    one, and calls `openEpicTickets` with the key (US4.1, US4.2);
  - no page error.

## 7. Checks

- [x] T7.1 `cd web && node --test tests/roadmapFocus.test.mjs
  tests/roadmap.test.mjs tests/optionalViews.test.mjs`.
- [x] T7.2 `cd web && npx tsc --noEmit` and `npx oxlint` (worktrees have no
  `node_modules`: symlink the main checkout's, then remove the link).
- [x] T7.3 `node tests/roadmap-epic-navigation.browser.mjs` and
  `node tests/roadmap-view.browser.mjs` (sandbox off, `PLAYWRIGHT_MODULE`
  pointing at the main checkout's install).
- [ ] T7.4 Manual pass in the running app: board card → epic → "Open its
  tickets" → board filtered; list → epic → back to list; detail under "all
  projects" switches project; a parent filter set does not empty the roadmap,
  and its chip is absent there. Not run by the agent: left to the review.

## Test plan by requirement

| Requirement | Covered by |
| --- | --- |
| FR1 | T1.2 `canOpenEpicInRoadmap`, T7.4 |
| FR2 | T7.4 (parent key clicks unchanged) |
| FR3 | T7.4 (switch under "all projects") |
| FR4 | T1.2 `locateEpic`, T6.1 |
| FR5 | T1.2 `locateEpic` null, T6.1 unknown key |
| FR6 | T1.2 `returnView`, T6.1 panel button |
| FR7 | T1.2 `returnView`, T7.4 |
| FR8 | T7.4 (roadmap with a parent filter) |
| FR9 | T7.4 (chip absent on the roadmap) |
| FR10 | T5.1, `tsc` (catalog type) |
| FR11 | T5.2 |
