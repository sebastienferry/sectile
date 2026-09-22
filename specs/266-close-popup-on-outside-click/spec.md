# #266 — Closing a popup by clicking outside it

## Context

Every dismissible surface in the web interface is expected to close on a click outside its
bounds. The anchored menus already do: `LookupField`, `TaskFilters`, `Sidebar`, `TaskCard`,
`ListView` and `McpSessions` each install a document-level `mousedown` listener and close when
the press lands outside their own subtree.

The modal dialogs are where the promise breaks. Each renders a full-screen backdrop and, until
recently, none of them reacted to a click on it. Four have since gained one
(`AdminModal`, `ChangelogModal`, `ProfileModal`, `ProjectModal`), plus `TaskDetailModal` in
panel mode — but ten dialogs are still inert, and the four that work do so through a bare
`e.target === e.currentTarget` test that also fires when a text selection started inside the
dialog happens to be released on the backdrop.

Five of the inert dialogs have no `Escape` handler either, so they can only be closed by
finding their × button.

This specification describes behaviour only. The technical choices are in `plan.md`, the
ordered work in `tasks.md`. The decisions it applies are recorded in
`docs/clarifications/266.md`.

## Decision being specified

A modal dialog closes when the person clicks its backdrop — the visible area outside the
dialog — and the close does exactly what the dialog's own close button does, no more. The
gesture is deliberate: it is a press *and* a release on the backdrop, so a drag that merely
ends there does not count. Every dismissible dialog also answers `Escape`.

## User stories

### US1 — Clicking beside a dialog closes it (P1)

As anyone using the board, I want a click next to an open dialog to close it, so that I do not
have to aim at the × button to get back to my work.

- **Given** any of the dialogs listed in *Scope* is open
- **When** I press and release the mouse on the dark area outside it
- **Then** the dialog closes, through the same code path as its × button.
- **Given** the same dialog is open
- **When** I click anywhere inside the dialog — a field, a tab, a button, a scrollbar, or a
  dropdown panel the dialog renders through a portal
- **Then** the dialog stays open.

### US2 — A drag that ends outside does not close the dialog (P1)

As someone selecting text or dragging a slider inside a dialog, I want the dialog to survive
my releasing the button outside it, so that a sloppy gesture does not throw away what I typed.

- **Given** a dialog is open
- **When** I press inside the dialog, move the pointer onto the backdrop and release there
- **Then** the dialog stays open.
- **Given** a dialog is open
- **When** I press on the backdrop, move the pointer over the dialog and release there
- **Then** the dialog stays open.

### US3 — One click closes exactly one layer (P1)

As someone who opened a dialog over another dialog, I want one click to close one of them, so
that I do not lose the context underneath by accident.

- **Given** `TrackerSetup` (`z-60`) is open over `ProjectModal` (`z-50`)
- **When** I click the backdrop of `TrackerSetup`
- **Then** `TrackerSetup` closes and `ProjectModal` stays open.
- **Given** the expanded specification reader (`z-60`) is open over `TaskDetailModal` (`z-50`)
- **When** I click its backdrop
- **Then** the reader closes and the task dialog stays open.

### US4 — Closing loses exactly what `Escape` loses (P1)

As someone half-way through a creation form, I want the outside click to behave like the
`Escape` I already know, so that the gesture holds no surprise.

- **Given** `TaskDetailModal` is open with edited fields
- **When** I click outside it
- **Then** `handleClose` runs, so the edits are saved exactly as they are on `Escape` or ×.
- **Given** `QuickAddModal`, `CloneTaskModal`, `ProjectModal` or `ProfileModal` holds
  unsubmitted input
- **When** I click outside it
- **Then** it closes and discards, with no confirmation prompt — the behaviour `Escape` has
  today.

### US5 — Every dismissible dialog answers Escape (P2)

As a keyboard user, I want `Escape` to close any dialog that has a close button, so that no
dialog is a dead end.

- **Given** `TrackerSetup`, `SprintTimelineView`'s close-sprint dialog, or one of
  `RoadmapView`'s three dialogs is open
- **When** I press `Escape`
- **Then** it closes, through the same path as its × button.
- **Given** one of those dialogs is open over another
- **When** I press `Escape`
- **Then** only the topmost one closes.

### US6 — The board is unaffected (P1)

As someone dragging a card across the board, I want the new handlers to be invisible to me, so
that the drag-and-drop keeps working.

- **Given** no dialog is open
- **When** I drag a task card between columns, or open and dismiss an anchored menu
- **Then** nothing changes: no new document-level listener runs, and the existing behaviour is
  exactly as before.

## Scope

Dialogs that gain the backdrop dismissal (the ten still inert), with `Escape` added to the
five that lack it:

| Dialog | File | Adds `Escape` |
|---|---|---|
| CloneTaskModal | `web/src/components/CloneTaskModal.tsx` | — |
| QuickAddModal | `web/src/components/QuickAddModal.tsx` | — |
| CommandPalette | `web/src/components/CommandPalette.tsx` | — |
| TaskDetailModal — centered mode | `web/src/components/TaskDetailModal.tsx` | — |
| TaskDetailModal — expanded spec reader | `web/src/components/TaskDetailModal.tsx` | — |
| RoadmapView — create macro | `web/src/components/RoadmapView.tsx` | yes |
| RoadmapView — migrate macro | `web/src/components/RoadmapView.tsx` | yes |
| RoadmapView — refine preview | `web/src/components/RoadmapView.tsx` | yes |
| SprintTimelineView — close sprint | `web/src/components/SprintTimelineView.tsx` | yes |
| TrackerSetup | `web/src/components/TrackerSetup.tsx` | yes |

Dialogs already dismissing, which move to the shared guard so the drag case of US2 holds for
them too: `AdminModal`, `ChangelogModal`, `ProfileModal`, `ProjectModal`, and
`TaskDetailModal` in panel mode.

## Out of scope

- Any change to what closing *does*: no confirmation prompt, no new save or discard rule.
- Focus traps, focus restoration, `aria-modal` and the rest of the dialog accessibility work.
- The anchored menus' behaviour — they already satisfy US1 and only see a mechanical
  deduplication, with no behaviour change.
- The Electron desktop interface (`desktop/`), including its native `<dialog>.showModal()`.
- `SignInScreen`, which is not dismissible by design.
- The Go backend. Nothing server-side changes.

## Open requirements

None. Q1, Q2 and Q3 of the clarification are answered and recorded.
