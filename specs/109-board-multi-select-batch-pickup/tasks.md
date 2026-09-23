# #109 — Tasks

Ordered so that each step leaves `yarn lint`, `yarn build` and `yarn test` green.

## 1. Pure rules (D1, D6)

- [ ] 1.1 Create `web/src/lib/boardSelection.ts` with `SELECTABLE_STAGES`, `isSelectableStage`,
      `isSelectionClick`, `toggleSelected`, `pruneSelection`, `orderSelection` and
      `shouldEscapeClearSelection`.
- [ ] 1.2 Create `web/tests/boardSelection.test.mjs`:
  - only `new` and `clarified` are selectable, the four later stages are not;
  - `isSelectionClick` is true for Ctrl, for Cmd, and false without a modifier;
  - `toggleSelected` adds, removes and never mutates its input;
  - `pruneSelection` returns the same Set when nothing drops out, and drops the ids that are off
    screen or no longer selectable;
  - `orderSelection` follows the board order whatever the insertion order, and ignores ids that
    are not on the board;
  - `shouldEscapeClearSelection` is true on a bare `Escape`, and false for another key, a
    prevented event, each `AppContext` surface, a non-empty search, a focused field or terminal,
    and an open `aria-modal` dialog.

## 2. Batch launch contract (D5)

- [ ] 2.1 `AppContext.startBatchPickup` builds the batch by mapping over `taskIds`, dropping the
      unknown ids, and returns `Promise<boolean>`: `false` for an empty batch, for the
      cross-project refusal and when `runSkill` returns `null`. Update the context type.
- [ ] 2.2 Check that the Curation, Triage and Sprint Timeline callers still compile unchanged.

## 3. Card (D3)

- [ ] 3.1 Add the `selectable`, `selected`, `selectionActive` and `onToggleSelect` props to
      `TaskCard`.
- [ ] 3.2 Render the checkbox button, on the full and on the condensed card, revealed by hover,
      focus-within or `selectionActive`.
- [ ] 3.3 Route Ctrl/Cmd+click to `onToggleSelect` on the card root and on the condensed title
      button, when `selectable`. Leave every other click as it is.
- [ ] 3.4 Highlight the selected card. Leave the drag code untouched.

## 4. Board (D2, D4, D6)

- [ ] 4.1 Compute `visibleColumns` once per render for both groupings, and render the columns from
      it. Derive `boardOrder` and `selectableIds` from it.
- [ ] 4.2 Add the `selectedIds` state, the pruning effect and the clear-on-project-change effect.
- [ ] 4.3 Pass the selection props to every `TaskCard` the board renders, including "Non classé".
- [ ] 4.4 Add the `window` `Escape` listener, installed only while the selection is not empty and
      gated by `shouldEscapeClearSelection`.
- [ ] 4.5 Render the floating selection bar: counter, batch button (disabled while launching) and
      "Désélectionner tout". On launch, call
      `startBatchPickup(orderSelection(selectedIds, boardOrder))` and clear only on `true`.

## 5. Documentation

- [ ] 5.1 Add a board scenario to `openspec/specs/batch-issue-pickup/spec.md`: `new`/`clarified`
      cards only, board order.
- [ ] 5.2 Add one `Added` line to `CHANGELOG.md` under `[Unreleased]`.

## 6. Verification

- [ ] 6.1 `cd web && yarn lint && yarn build && yarn test`, all green, output quoted in the PR.
- [ ] 6.2 Manual replay in `yarn dev`, in the workflow grouping, the status grouping and the
      condensed mode:
  - [ ] hovering a `new`/`clarified` card shows the checkbox. A `specified` card never shows one;
  - [ ] `Tab` reaches the checkbox, and `Space` toggles it;
  - [ ] Ctrl/Cmd+click toggles a `new` card without opening it. A plain click opens it;
  - [ ] Ctrl/Cmd+click on a `specified` card opens it and does not select it;
  - [ ] with one card selected, every selectable card shows its checkbox;
  - [ ] selecting a card in "Clarified" then one in "New" launches `/pickup-issues <new> <clarified>`;
  - [ ] an accepted launch clears the bar. With the agent disconnected, the error toast shows and
        the selection stays;
  - [ ] `Escape` with a card menu open closes the menu only. A second `Escape` clears the selection;
  - [ ] a filter or search that hides a selected card lowers the counter;
  - [ ] switching project clears the selection;
  - [ ] dragging a selected card to another column moves that card only.
