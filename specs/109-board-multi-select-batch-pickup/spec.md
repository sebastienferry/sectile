# #109 — Selecting several stories on the board

## Context

The `/pickup-issues` batch skill takes several tickets through the workflow in one Git worktree.
Curation, Triage and Sprint Timeline can already launch it: each keeps a checkbox selection and a
purple "Lancer le lot (Git tree + Auto-pilot)" button. The `batch-issue-pickup` specification asks
for the same button on the Kanban board too, but the board has no way to select several cards, so
that part was never delivered.

This specification describes behaviour only. The technical choices are in `plan.md` and the
ordered work in `tasks.md`. The decisions it applies are recorded in `docs/clarifications/109.md`.

## Decision being specified

On the board, the person can select several cards that are still at the start of the workflow
(`new` or `clarified`). They then launch the batch pickup on them from a selection bar, in the
order the board shows them. Selecting a card never gets in the way of opening it or dragging it.

## User stories

### US1 — Selecting a card with its checkbox (P1)

As someone preparing a batch, I want to tick the cards I want, so that I can gather a batch
without leaving the board.

- **Given** the board shows a card whose stage is `new` or `clarified`
- **When** I hover the card, or move the keyboard focus into it
- **Then** a checkbox appears on the card.
- **Given** that checkbox is shown
- **When** I click it, or press `Space`/`Enter` while it has the focus
- **Then** the card is selected and shows it: the box is ticked and the card is highlighted.
- **Given** a selected card
- **When** I click its checkbox again
- **Then** the card is deselected.
- **Given** at least one card is selected
- **When** I look at the board
- **Then** every selectable card shows its checkbox, without needing to hover it.

### US2 — Selecting with Ctrl/Cmd+click (P1)

As someone who selects many cards, I want a keyboard-modified click to toggle a card, so that I do
not have to aim at the checkbox.

- **Given** a card whose stage is `new` or `clarified`
- **When** I click it while holding `Ctrl` (or `Cmd` on macOS)
- **Then** the card's selection toggles, and the detail dialog does not open.
- **Given** any card
- **When** I click it without a modifier
- **Then** the detail dialog opens, as it does today, and the selection is unchanged.
- **Given** a card whose stage is `specified`, `implemented`, `reviewed` or `finished`
- **When** I `Ctrl`/`Cmd`+click it
- **Then** it is not selected, and it behaves as a plain click does today.

### US3 — Only cards at the start of the workflow can be selected (P1)

As the owner of the workflow, I want the batch to start only from tickets not yet specified, so
that a batch never replays later stages by accident.

- **Given** a card whose stage is `specified`, `implemented`, `reviewed` or `finished`
- **When** I hover it or focus into it
- **Then** it shows no checkbox, even while other cards are selected.
- **Given** the rule above
- **When** the board groups by workflow stage, by status, or by the project's tracker columns
- **Then** the same cards are selectable: the rule reads the card's workflow stage, not its column.
- **Given** a `new` or `clarified` card with a run in progress
- **When** I look at it
- **Then** it is selectable like any other card at that stage.

### US4 — Launching the batch from the selection bar (P1)

As someone who gathered a batch, I want one button that launches it, so that I get the same batch
pickup the other views offer.

- **Given** at least one card is selected
- **When** I look at the board
- **Then** a bar floats at the bottom of the board. It shows how many cards are selected, a
  "Lancer le lot (Git tree + Auto-pilot)" button and a "Désélectionner tout" button, and no other
  action.
- **Given** the bar is shown
- **When** I click "Lancer le lot (Git tree + Auto-pilot)"
- **Then** `/pickup-issues` is launched once for the selected tasks, through the same path as
  Curation, Triage and Sprint Timeline.
- **Given** the launch was accepted
- **When** it returns
- **Then** the selection is cleared and the bar disappears.
- **Given** the launch failed (no local agent, a refused launch)
- **When** it returns
- **Then** the error is reported as the other views report it, and the selection is kept so that
  I can try again.
- **Given** a launch is in progress
- **When** I look at the button
- **Then** it cannot be clicked a second time.

### US5 — The batch follows the board's order (P1)

As someone launching a batch, I want the tickets processed in the order I see them, so that the
order is predictable and does not depend on how I clicked.

- **Given** I selected cards in several columns, in any order
- **When** I launch the batch
- **Then** the tasks are passed in board order: columns from left to right, then cards from top
  to bottom within a column.

### US6 — Clearing the selection (P2)

As someone who changed their mind, I want quick ways to drop the selection.

- **Given** at least one card is selected
- **When** I click "Désélectionner tout"
- **Then** no card is selected and the bar disappears.
- **Given** at least one card is selected and no dialog, menu or text field has the focus
- **When** I press `Escape`
- **Then** the selection is cleared.
- **Given** at least one card is selected and a dialog or a card menu is open
- **When** I press `Escape`
- **Then** only that dialog or menu closes, and the selection stays.
- **Given** at least one card is selected
- **When** I switch to another project
- **Then** the selection is cleared.

### US7 — The selection only holds cards on screen (P2)

As someone filtering the board, I want the selection to match what I see, so that a batch never
contains a ticket I can no longer see.

- **Given** a selected card
- **When** it leaves the board (a filter, a collapsed or hidden column, a search, a deletion, a
  synchronisation), or its stage moves past `clarified`
- **Then** it drops out of the selection, and the counter says so.
- **Given** a card that dropped out that way
- **When** it comes back on screen
- **Then** it is not selected again.

### US8 — Dragging keeps moving one card (P2)

As someone who reorganises the board while a selection exists, I want drag and drop to work as
before.

- **Given** several cards are selected
- **When** I drag one of them, selected or not, to another column
- **Then** only that card moves, as it does today.

## Out of scope

- Any change to the `pickup-issues` skill, its catalog entry or its server-side launch.
- Bulk stage, priority, label or deletion actions on the board (ListView keeps them).
- Batches across projects: the board shows one project, and `startBatchPickup` keeps refusing a
  cross-project selection.
- Moving several cards with one drag.
- Keeping the selection across views, reloads or project switches.
- A "select all" or per-column select control.

## Acceptance

- A `new` or `clarified` card can be selected with its checkbox and with `Ctrl`/`Cmd`+click, and a
  plain click still opens its detail.
- Cards at any other stage cannot be selected, in the three board groupings.
- The selection bar offers exactly the batch launch and the clear action, and the launch passes
  the tasks in board order.
- The selection clears after an accepted launch, on "Désélectionner tout", on `Escape` when
  nothing else takes the key, and on a project switch. It survives a failed launch.
- The selection never holds a card that is not on screen.
- Dragging a card moves that card only.
