# Specification #612 - Web card: model menu and copy-prompt button

- Ticket: https://github.com/sebastienferry/sectile/issues/612
- Branch: `feat/612`
- Clarification: `docs/clarifications/612.md` (rounds 1 and 2, confirmed by
  the owner)
- Framework: Spec Kit

## Summary

On the web board, a task card shows which model its next launch will use, in
front of its action buttons. Today that indicator is a passive label, and
changing the model means opening the `(...)` menu and its "Modèle des
lancements" sub-list. With this change the indicator itself opens the model
list, and the full card gains an icon button that copies, in one click, the
prompt to paste into an AI engine's desktop app (Claude Code, Codex or AGY) to
run the task's next step.

## Scope

In scope: the task card of the web board, in both its full and condensed
shapes, its French and English strings, and the changelog.

Out of scope:

- Sectile Desktop, the task detail modal, the server and the local agent.
- How a launch resolves its model, and which models an engine offers.
- The wording of the copied prompt, which stays the one the `(...)` menu
  copies today, identical for every engine.
- The `(...)` menu's existing entries: "Modèle des lancements", the copy
  entries (current step and `/pickup-issue`) and the advance entries all stay.

## User stories

### US1 (P1) - Pick the launch model from the indicator

As a board user, I change the model a card will launch with by clicking the
model shown on the card, without going through the `(...)` menu.

1. Given a card whose engine reports a model slot and a model list, when I
   click the model indicator, then a menu opens next to it listing the
   configured model, marked as current, followed by the offered models, with
   a check mark on the one in effect.
2. Given that menu open, when I choose a model, then the menu closes, the
   indicator shows the chosen model in the accent colour, its tooltip reads
   "Modèle retenu pour cette tâche : <model>", and no launch starts.
3. Given a model chosen from the indicator, when I then advance the card with
   `>` or `>>`, then the launch uses the chosen model.
4. Given a chosen model, when I choose the configured model entry, then the
   pick is cleared and the indicator returns to the configured model in the
   muted colour.
5. Given the same card on the condensed board, when I click its indicator,
   then the same menu opens with the same behaviour.

### US2 (P1) - Both entry points show one selection

As a board user, I see the same pick whichever place I made it from.

1. Given a model chosen from the indicator, when I open `(...)` →
   "Modèle des lancements", then that model carries the check mark and the
   entry shows its short name.
2. Given a model chosen from the `(...)` sub-list, when I open the
   indicator's menu, then that model carries the check mark and the indicator
   already shows it.
3. Given a pick made from either place, when I reload the board, then the
   indicator and both lists still show it.

### US3 (P1) - Copy the next step's prompt in one click

As a person who runs Sectile steps from an AI engine's desktop app, I copy the
prompt for the task's next step from the card itself.

1. Given a full card in the "clarified" column, when I click the copy icon,
   then the clipboard holds exactly the prompt that `(...)` → "Copier
   /specify-issue" copies for this task, and a confirmation toast appears.
2. Given a full card in any column but "finished", the icon copies the prompt
   of that column's step: `/clarify-issue` for a new task, `/specify-issue`,
   `/implement-issue`, `/adjust-issue`, then `/handoff-issue` for a reviewed
   one.
3. Given a finished task, then the full card shows no copy icon.
4. Given the icon, its tooltip and accessible name read "Copier <command>"
   with the command it copies, for example "Copier /specify-issue".
5. Given a condensed card, then it shows no copy icon, and its `(...)` menu
   keeps both copy entries.
6. Given a project that overrides a skill's command, when I click the icon,
   then the prompt starts with the overridden command, as the `(...)` entry
   does.

### US4 (P2) - The copy still works when the clipboard is refused

As a user whose browser refuses clipboard access (insecure origin, denied
permission), I can still get the prompt.

1. Given a refused clipboard, when I click the copy icon, then a small panel
   opens next to the icon, says the clipboard is blocked, and shows the prompt
   as selectable text.
2. Given that panel open, when I press Escape or click outside it, then it
   closes and focus returns to the copy icon.

### US5 (P2) - Nothing to pick, nothing clickable

As a board user, I am not offered a menu that would be empty.

1. Given no agent of mine serves the project, then the indicator shows `?`
   with its current tooltip and does nothing when clicked.
2. Given an engine whose command line has no model slot, or reports no model
   list, then the indicator, when shown, is a plain label that does nothing
   when clicked.
3. Given an engine that offers models but no configured model and no pick,
   then the indicator shows a neutral chip icon that opens the model menu.

### US6 (P2) - Card controls stay isolated

1. Given a card, when I click the model indicator or the copy icon, then the
   task detail does not open and no drag starts.
2. Given the model menu opened by the indicator, when I press Escape, then it
   closes and focus returns to the indicator; arrow keys move between its
   entries as in the `(...)` sub-list; a click outside, a scroll or a resize
   closes it.

## Functional requirements

- **FR1** The model indicator is a button that opens a menu of launch models
  whenever the card's engine reports a model slot and at least one model.
  Otherwise it keeps its current passive rendering.
- **FR2** The indicator's menu lists the same entries, in the same order and
  with the same labels, as the `(...)` "Modèle des lancements" sub-list: the
  configured model marked current (or the "current" entry alone when none is
  configured), then the offered models.
- **FR3** Choosing an entry in either list stores the pick for that task
  (the per-task store both lists already share), updates the indicator and
  both lists at once, closes the open menu, and starts no launch.
- **FR4** When the engine offers models but neither a configured model nor a
  pick exists, the indicator shows a neutral chip icon so the menu stays
  reachable. When there is nothing to pick, no such icon appears.
- **FR5** The indicator's menu is exposed as a menu of radio items with an
  accessible name, takes focus on open, supports the arrow keys, closes on
  Escape (returning focus to the indicator), on an outside click, a scroll and
  a resize, and is never clipped by the card or the column.
- **FR6** The full card's bottom row carries a copy icon button, placed with
  the other action icons, on every task not in the "finished" stage. The
  condensed card carries none.
- **FR7** The copy icon copies the prompt of the current column's step, byte
  for byte identical to the prompt the `(...)` step copy entry produces for
  the same task, including the project's command override.
- **FR8** A successful copy shows a confirmation toast. A refused clipboard
  shows the prompt in a closable panel anchored to the icon, as selectable
  text, with the "clipboard blocked" message.
- **FR9** Clicking the indicator, an entry of its menu, the copy icon or its
  panel neither opens the task detail nor starts a drag.
- **FR10** Every new string exists in French and English.
- **FR11** `CHANGELOG.md` gains one line under `## [Unreleased]` → `Changed`
  (or `Added`) describing both card changes for users.

## Acceptance criteria

- **AC1** (US1, FR1-FR3) On a card with a model list, clicking the indicator
  and choosing a model changes the indicator, stores the pick, and a
  following `>` click calls the advance with that model; no advance call
  happens on the choice itself.
- **AC2** (US2, FR3) A pick made from the indicator is checked in the `(...)`
  sub-list, and conversely, without reloading.
- **AC3** (US3, FR6, FR7) On a full card, the icon's copied text equals the
  `(...)` step entry's copied text, for at least the "new" and "clarified"
  stages; a finished card has no icon; a condensed card has no icon.
- **AC4** (US4, FR8) With the clipboard write rejected, clicking the icon
  shows the prompt in a selectable panel; Escape closes it and focus is back
  on the icon.
- **AC5** (US5, FR1, FR4) `?` and a slot-less engine render a non-interactive
  indicator; an engine with models and no configured model shows the chip
  icon, which opens the menu.
- **AC6** (US6, FR5, FR9) Clicks on the new controls record no
  `setSelectedTask` call; Escape in the indicator's menu returns focus to the
  indicator.
- **AC7** (FR10) The French and English locale objects type-check with the
  new keys.
- **AC8** (FR11) `CHANGELOG.md` carries the line under `[Unreleased]`.

## Open requirements

None. The four product questions were answered in clarification round 2.
