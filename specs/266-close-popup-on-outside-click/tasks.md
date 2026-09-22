# #266 — Implementation checklist

Ordered so the shared pieces exist and are tested before any dialog depends on them, the
user-visible fix lands next, and the like-for-like deduplication comes last — where it can be
dropped without touching the fix.

## 1. The shared dismissal

- [ ] **T1** `web/src/lib/backdropDismiss.ts`: `pressLandedOnBackdrop` and `shouldDismiss`
      (plan D3), total over `null` targets, with a comment saying why the press is checked at
      all (plan D2).
- [ ] **T2** `web/tests/backdropDismiss.test.mjs`: the truth table — press then click on the
      backdrop closes; press inside then release on the backdrop does not; press on the
      backdrop then release inside does not; a `null` target never closes.
- [ ] **T3** `web/src/hooks/useBackdropDismiss.ts`: the hook returning `{ onMouseDown, onClick }`
      to spread on a backdrop, holding the press flag in a `useRef`.

## 2. The ten dialogs that do not dismiss

- [ ] **T4** `CloneTaskModal`: spread the hook on the backdrop; drop the now-redundant
      `onClick={e => e.stopPropagation()}` on the panel, which the target check replaces.
- [ ] **T5** `QuickAddModal`: spread the hook on the backdrop.
- [ ] **T6** `CommandPalette`: spread the hook on the backdrop.
- [ ] **T7** `TaskDetailModal`, centered mode: spread the hook on the backdrop, closing through
      `handleClose` so the autosave still runs (spec US4).
- [ ] **T8** `TaskDetailModal`, expanded specification reader: spread the hook on its `z-60`
      backdrop, closing with `setIsExpandedSpec(false)` only (spec US3).
- [ ] **T9** `RoadmapView`: spread the hook on the create-macro, migrate and refine-preview
      backdrops, each closing through the same setter as its × button.
- [ ] **T10** `SprintTimelineView`: spread the hook on the close-sprint backdrop
      (`setClosingSprint(null)`).
- [ ] **T11** `TrackerSetup`: spread the hook on the backdrop, closing through the `onClose`
      prop.

## 3. `Escape` where there is none

- [ ] **T12** `RoadmapView`: one `keydown` effect per dialog, guarded by that dialog's own open
      condition, following the pattern of the sibling components (plan D4).
- [ ] **T13** `SprintTimelineView`: same, guarded by `closingSprint`.
- [ ] **T14** `TrackerSetup`: same, calling `onClose`.

## 4. The five dialogs that already dismiss

- [ ] **T15** `AdminModal`, `ChangelogModal`, `ProfileModal`, `ProjectModal`: replace the inline
      `e.target === e.currentTarget` handler with the hook, so the drag case of US2 holds for
      them too. Behaviour otherwise unchanged.
- [ ] **T16** `TaskDetailModal`, panel mode: its backdrop is a separate element from the dialog,
      so `onClick={handleClose}` is already correct for the click — give it the hook anyway, so
      a selection released on it no longer closes the panel.

## 5. Verification

- [ ] **T17** `web/tests/outside-click.browser.mjs`: the Playwright regression of plan's test
      plan — the four gestures of US1/US2 and the two-layer case of US3, asserting the close
      count. Header comment states how to run it, as `condensed-card.browser.mjs` does.
- [ ] **T18** `yarn lint` and `yarn build` clean; `node --test tests/*.test.mjs` green.
- [ ] **T19** Re-read the diff: every backdrop that gained a handler closes through the path its
      × button already used, and no dialog kept a bare `e.target === e.currentTarget`.

## 6. Anchored menus — deduplication only (no behaviour change)

- [ ] **T20** `web/src/hooks/useClickOutside.ts`: `useClickOutside(ref | ref[], onOutside, enabled)`
      (plan D5).
- [ ] **T21** Migrate the verbatim call sites: `TaskFilters` (two effects), `Sidebar`,
      `ListView`, and `LookupField` (two refs). Leave `TaskCard` and `McpSessions` alone, and
      say why in the hook's comment.
- [ ] **T22** `yarn lint`, `yarn build` and `node --test tests/*.test.mjs` again after the
      migration.
