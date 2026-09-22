# #266 — Implementation checklist

Ordered so the shared pieces exist and are tested before any dialog depends on them, the
user-visible fix lands next, and the like-for-like deduplication comes last — where it can be
dropped without touching the fix.

## 1. The shared dismissal

- [x] **T1** `web/src/lib/backdropDismiss.ts`: `landedOnBackdrop` and `createBackdropGesture`
      (plan D3), total over `null` targets, with a comment saying why the press and the release
      are checked at all (plan D2).
- [x] **T2** `web/tests/backdropDismiss.test.mjs`: the truth table — press then click on the
      backdrop closes; press inside then release on the backdrop does not; press on the
      backdrop then release inside does not; a `null` target never closes.
- [x] **T3** `web/src/hooks/useBackdropDismiss.ts`: the hook returning
      `{ onMouseDown, onMouseUp, onClick }` to spread on a backdrop, holding the gesture in a
      `useMemo`.

## 2. The ten dialogs that do not dismiss

- [x] **T4** `CloneTaskModal`: spread the hook on the backdrop; drop the now-redundant
      `onClick={e => e.stopPropagation()}` on the panel, which the target check replaces.
- [x] **T5** `QuickAddModal`: spread the hook on the backdrop.
- [x] **T6** `CommandPalette`: spread the hook on the backdrop.
- [x] **T7** `TaskDetailModal`, centered mode: spread the hook on the backdrop, closing through
      `handleClose` so the autosave still runs (spec US4).
- [x] **T8** `TaskDetailModal`, expanded specification reader: spread the hook on its `z-60`
      backdrop, closing with `setIsExpandedSpec(false)` only (spec US3).
- [x] **T9** `RoadmapView`: spread the hook on the create-macro, migrate and refine-preview
      backdrops, each closing through the same setter as its × button.
- [x] **T10** `SprintTimelineView`: spread the hook on the close-sprint backdrop
      (`setClosingSprint(null)`).
- [x] **T11** `TrackerSetup`: spread the hook on the backdrop, closing through the `onClose`
      prop.

## 3. `Escape` where there is none

- [x] **T12** `web/src/hooks/useEscapeKey.ts`: the capture-phase handler of plan D4, so a
      stacked dialog closes alone. Replaces the per-dialog `window` listener the plan first
      proposed — see D4 for why.
- [x] **T13** `RoadmapView` (three dialogs) and `SprintTimelineView` (guarded by
      `closingSprint`) adopt it.
- [x] **T14** `TrackerSetup` adopts it, calling `onClose`.

## 4. The five dialogs that already dismiss

- [x] **T15** `AdminModal`, `ChangelogModal`, `ProfileModal`, `ProjectModal`: replace the inline
      `e.target === e.currentTarget` handler with the hook, so the drag case of US2 holds for
      them too. Behaviour otherwise unchanged.
- [x] **T16** `TaskDetailModal`, panel mode: its backdrop is a separate element from the dialog,
      so `onClick={handleClose}` is already correct for the click — give it the hook anyway, so
      a selection released on it no longer closes the panel.

## 5. Verification

- [x] **T17** `web/tests/outside-click.browser.mjs`: the Playwright regression of plan's test
      plan — the four gestures of US1/US2 and the two-layer case of US3, asserting the close
      count. Header comment states how to run it, as `condensed-card.browser.mjs` does.
- [x] **T18** `yarn lint` and `yarn build` clean; `node --test tests/*.test.mjs` green.
- [x] **T19** Re-read the diff: every backdrop that gained a handler closes through the path its
      × button already used, and no dialog kept a bare `e.target === e.currentTarget`.

## 6. Anchored menus — deduplication only (no behaviour change)

- [x] **T20** `web/src/hooks/useClickOutside.ts`: `useClickOutside(ref | ref[], onOutside, enabled)`
      (plan D5).
- [x] **T21** Migrate the verbatim call sites: `TaskFilters` (two effects), `Sidebar`,
      `ListView`, and `LookupField` (two refs). Leave `TaskCard` and `McpSessions` alone, and
      say why in the hook's comment.
- [x] **T22** `yarn lint`, `yarn build` and `node --test tests/*.test.mjs` again after the
      migration.

## 7. Deviations from the plan, and why

- **T3 / plan D2** — the backdrop carries three handlers, not two. The browser regression showed
  that pressing beside the dialog and releasing over it produces the same click target as a real
  outside click, so the release has to be watched as well as the press.
- **T12 / plan D4** — the five `Escape` handlers are one shared `useEscapeKey` catching the key
  in the capture phase, not five copies of the sibling pattern. `AppContext` already keeps a
  ranked `Escape` handler, and a plain `window` listener on `TrackerSetup` would have closed
  `ProjectModal` underneath it in the same key press.
- **Left as found** — `tests/skillLaunchModel.test.mjs` fails on a Windows checkout, where
  `core.autocrlf` gives `TaskCard.tsx` CRLF endings that its line-anchored regexes cannot match.
  It is unrelated to this change (`TaskCard.tsx` is untouched) and predates it.
- **Left as found** — `tests/condensed-card.browser.mjs` no longer starts: its Vite fixture
  plugin compares a plugin id built with the platform separator, and the React plugin now
  re-imports the fixture by its absolute id. `tests/outside-click.browser.mjs` handles both, but
  repairing the older harness is another ticket's work.
