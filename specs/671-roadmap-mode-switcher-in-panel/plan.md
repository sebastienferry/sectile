# Plan #671 - Roadmap: move the mode switcher into the side panel

## Stack

React 19 and TypeScript in `web/`, Tailwind classes and CSS variables as in
the rest of `RoadmapView`. Browser regressions run the real component through
Vite and Playwright (`web/tests/*.browser.mjs`). No server, API, migration or
desktop change.

## Target files

- `web/src/components/RoadmapView.tsx`: the only production file.
- `web/tests/roadmap-mode-tabs.browser.mjs`: new browser regression.
- `web/tests/roadmap-framing.browser.mjs`, `roadmap-todos.browser.mjs`,
  `roadmap-batch-stories.browser.mjs`, `roadmap-slicing-upload.browser.mjs`:
  select Framing as a tab.
- `CHANGELOG.md`.

## Design

### D1. One flag for the list's Execution behaviour

A derived boolean, `sprintCheckHere = tab === 'now' || tab === 'next'`,
replaces every read of `displayMode === 'execution'` outside the panel body:

| Today | After |
| --- | --- |
| `visibleRows`: `displayMode === 'execution' && onlyIssues` | `sprintCheckHere && onlyIssues` |
| unfolded row badge: `displayMode === 'execution' ? placement : todos` | `sprintCheckHere ? placement : todos` |
| toolbar "À corriger" toggle: `displayMode === 'execution'` | `sprintCheckHere` |
| sprint strip: `displayMode === 'execution' && (now or next)` | `sprintCheckHere` |

`visibleRows` drops `displayMode` from its dependencies. The empty-list text
that reads `onlyIssues` already sits after the Hidden and Unclassified
branches; it becomes `sprintCheckHere && onlyIssues` so Later never claims
"no macro to fix". After the change, `displayMode` is read only by the panel
body switch and the new tab strip.

### D2. The tab strip

The four toolbar buttons move, as they are, into a small `MODE_TABS` list
rendered as a `role="tablist"` (with an `aria-label`) of `role="tab"`
buttons, `aria-selected` on the current one. It is the last child of the
panel's header block (the `shrink-0 border-b` div), after the axes and the
"kept local" note, so it stays put while the body scrolls. The expanded panel
uses the same header, so it gets the same strip with no second copy.

Styling follows the existing segmented control (`bg-[var(--bg-tertiary)]`,
accent background on the selected tab) so the controls keep their look; the
strip spans the header width and its buttons may wrap on a narrow panel.

The comment that called Phases and Goals "modes of the panel" moves with the
buttons, rewritten in English as the code is touched.

### D3. Strings

No new visible label: tabs reuse `strings.modes.*` and the axis plurals. The
tablist needs an accessible name; it gets one new key,
`strings.modes.label` ("Contenu du panneau" / "Panel content"), in both
locales of `web/src/locales/planning.ts`. The French one follows the
language the surface already speaks.

### D4. Default mode per horizon

The `useEffect` on `tab` is left as it is (clarification D1).

## Rejected alternatives

- Keeping `displayMode` as the list's switch (clarification Q1, option B).
- Placing the switcher in the header's top line among the action buttons
  (clarification Q2, option B): that line already wraps.
- Rendering the switcher in the hidden-panel rail: with no panel there is no
  body for it to change.

## Test strategy

A new `roadmap-mode-tabs.browser.mjs`, on the same harness as
`roadmap-view.browser.mjs`, with two `now` macros (one with an open task out
of any sprint, so it has a placement issue) and two `later` macros carrying
TODOs:

- the toolbar has no `tab` named Framing; the panel has a `tablist` of four
  tabs, Execution selected on Now;
- clicking Framing selects it, swaps the panel body, and leaves the row
  badges, the toggle and the sprint strip of Now in place;
- with "À corriger" on, the list keeps filtering in Framing mode;
- on Later, rows show TODO counts, no toggle, no sprint strip, Framing
  selected; the filter does not hide Later's macros;
- back on Now, Execution is selected again;
- expanding the panel keeps the tablist; hiding the panel leaves no tab
  anywhere.

The four existing tests replace `getByRole('button',{name:'Framing',exact:true})`
with `getByRole('tab',{name:'Framing',exact:true})`.

Type check and lint: `npx tsc -b` (or the project script) and `oxlint` from
`web/`.
