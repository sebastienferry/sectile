# #445: Quick add action (web)

Ticket: https://github.com/sebastienferry/sectile/issues/445
Parent macro: M-7: Ux improvements and fixes.
Branch: `feat/445`.
Clarification: [`docs/clarifications/445.md`](../../docs/clarifications/445.md).

## Context

The web "Quick add" dialog is a narrow single column: a title, a three-row
description, the project, a "Destination" chooser, then the ticket metadata. It
cannot attach the new ticket to a macro, the description is cramped, the
"Destination" chooser offers GitHub and Sectile even on a Jira project, and once
the ticket exists nothing helps the user continue with it: rewriting it as a
user story or clarifying it takes a second trip through the detail modal.

Out of scope: the desktop app's own quick add, the task detail modal, the macro
view's "TODO line becomes a story" flow, and what the `rewrite_story` and
`clarify` skills do.

This file states behaviour and acceptance criteria only. Implementation choices
are in [`plan.md`](plan.md); the ordered checklist is in [`tasks.md`](tasks.md).

---

## Decisions being specified

Settled by the owner during the clarification (rounds 2 and 3).

- **D1: Two columns.** Left: title and a taller description. Right: project,
  macro, type, status, priority, sprint, labels. The columns stack on a narrow
  window.
- **D2: Optional macro.** The dialog lists the chosen project's open macros,
  "none" by default, and the attachment reaches the tracker.
- **D3: No destination.** The ticket always goes to the project's own tracker.
- **D4: Exclusive pre-save follow-up.** None (default), rewrite as a user
  story, or clarify; chosen before saving, acted upon once the ticket exists,
  reset to none at every opening.
- **D5: Rewrite is reviewed.** It opens the new ticket's detail modal, where
  the proposal is applied or dismissed with the existing controls; it is never
  applied automatically.
- **D6: Clarify stays in the background.** The dialog closes, the user stays on
  the board.

---

## User stories

### US1: Write a ticket in a roomier dialog (P1)

As someone capturing a ticket, I want a wide dialog with the description beside
the ticket's settings, so that I can write a real description without scrolling
a three-line box.

**Acceptance**

1. **Given** the board, **when** I open Quick add on a wide window, **then** the
   title and description sit in a left column and the project, macro, type,
   status, priority, sprint and labels in a right column.
2. **Given** a narrow window (phone width), **when** I open Quick add, **then**
   the columns stack and the dialog fits the window.
3. **Given** the dialog, **then** the description editor is noticeably taller
   than three lines and supports the Markdown toolbar and preview used elsewhere.

### US2: Attach the new ticket to a macro (P1)

As someone organising work in macros, I want to pick the macro while creating
the ticket, so that it lands under it on Sectile and on the tracker.

**Acceptance**

1. **Given** a project with open and closed macros, **when** I open the macro
   field, **then** only the open macros are offered, each named by its key and
   title, after a "none" choice.
2. **Given** a board filtered on macro M of the chosen project, **when** I open
   Quick add, **then** M is pre-selected; **given** any other filter (no
   filter, "no macro", a macro of another project), **then** "none" is selected.
3. **Given** a chosen macro, **when** I change the project, **then** the list is
   reloaded for the new project and the selection returns to "none".
4. **Given** a chosen macro, **when** I save, **then** the ticket is created on
   the project's tracker and then attached to the macro through the same path as
   attaching it from the detail modal, so the tracker receives the parent.
5. **Given** a chosen macro, **when** the ticket is created but the attachment is
   refused, **then** the ticket stays created, the creation toast is shown, and a
   warning says the ticket was not attached.
6. **Given** "none", **when** I save, **then** no attachment is requested.

### US3: No destination chooser (P1)

As someone creating a ticket, I want it to go to the project's tracker without
asking me, so that I cannot pick a destination the project does not use.

**Acceptance**

1. **Given** the dialog, **then** no "Destination" field is shown.
2. **Given** a GitHub, Jira or local project, **when** I save, **then** the
   request names no source and the server creates the ticket on the project's
   tracker.
3. **Given** the project header, **then** the tracker the ticket goes to is still
   named beside the project field.

### US4: Continue with the ticket right after saving (P2)

As someone capturing a rough idea, I want to ask for a rewrite or a
clarification before saving, so that the next step starts on its own.

**Acceptance**

1. **Given** the dialog, **then** an "after saving" choice offers exactly one of
   *nothing*, *rewrite as a user story* and *clarify*, with *nothing* selected.
2. **Given** *rewrite*, **when** the ticket is created, **then** the dialog
   closes, the new ticket's detail modal opens and `rewrite_story` is launched on
   it; the proposal appears there to be applied or dismissed.
3. **Given** *clarify*, **when** the ticket is created, **then** the dialog
   closes, `clarify` is launched on the new ticket with the project's usual
   execution settings, and the user stays on the board.
4. **Given** *nothing*, **when** the ticket is created, **then** the dialog
   closes and no skill is launched.
5. **Given** a follow-up whose launch fails, **then** the ticket stays created
   and the error toast says the launch failed.
6. **Given** a creation that fails, **then** no follow-up is launched and the
   dialog stays open with what I typed.
7. **Given** I chose a follow-up and closed the dialog, **when** I open it again,
   **then** *nothing* is selected.

### US5: Nothing else changes (P1)

**Acceptance**

1. From a saved view, the project is still chosen among the view's projects,
   never defaulted (#387), and the view's single label is still prefilled.
2. The creation toast still links to the new ticket (#432).
3. Escape and a click on the backdrop still close the dialog.

---

## Functional requirements

- **FR1** The dialog is at least twice as wide as before on a wide window and
  lays out two columns as in US1; below the small breakpoint the columns stack.
- **FR2** The description uses the shared Markdown editor.
- **FR3** The macro field lists the chosen project's macros that are not closed,
  loads when the project is known, reloads on project change, and ignores a
  response for a project that is no longer selected.
- **FR4** The macro pre-selection follows the board's macro filter as in US2.2.
- **FR5** A save with a macro performs creation, then attachment; an attachment
  failure is a warning, never a failed creation.
- **FR6** The creation request carries no `source`.
- **FR7** The follow-up choice is exclusive, reset on every opening, and acted
  upon only after a successful creation (and attachment attempt).
- **FR8** New user-facing strings are in `translations.ts`, in French and
  English.
- **FR9** `CHANGELOG.md` gets one line under `[Unreleased]`.

## Success criteria

- A browser regression drives the dialog through US1-US4 against a fake API and
  passes.
- Unit tests cover the macro option and pre-selection rules.
- `npm run build`, `npm run lint` and `npm test` in `web/` pass.
