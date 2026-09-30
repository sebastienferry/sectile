# Specification #637 - Card: autonomous next step instead of an idle full chain

- Ticket: https://github.com/sebastienferry/sectile/issues/637
- Branch: `feat/637`
- Clarification: `docs/clarifications/637.md` (rounds 1 and 2, confirmed by
  the owner)
- Framework: Spec Kit

## Summary

On the web board, a task card offers the full chain (`>>`, the `pickup`
skill) at every stage. Once the task has reached the stage where the
project's full chain stops, that chain has nothing left to do, yet the button
stays live. With this change, a card at or past the project's full-chain stop
stage no longer offers the full chain: the full card shows, in its place, a
shortcut that runs the next step autonomously, and the condensed card's
`(...)` menu drops its "Chaîne complète" entry, keeping "Avancer en autonome".

## Scope

In scope: the task card of the web board, in both its full and condensed
shapes, its French and English strings, and the changelog.

Out of scope:

- The server's chain entry, the `pickup` skill, and `get_project_context`.
- Sectile Desktop and the task detail modal, including its per-skill launch
  grid.
- The copy entries: "Copier /pickup-issue" in the card menu and in the detail
  modal stays as it is.
- Finished tasks, whose launch controls keep today's disabled rendering.

## User stories

### US1 (P1) - A reviewed card runs its next step, not an idle chain

As a board user on a project whose full chain stops at `reviewed` (the
default), I launch the handoff of a reviewed task from the card in one click.

1. Given a full card at `reviewed`, then it shows no full-chain button and
   shows, where it stood, an autonomous-step button.
2. Given that card, when I click the autonomous-step button, then the
   next step (`handoff`) is launched in autonomous mode, with the model the
   card shows.
3. Given the button, its tooltip and accessible name read
   "Lancer <skill> en autonome : <description of the next step>".
4. Given a launch pending from any of the card's launch controls, then the
   autonomous-step button is disabled; while its own launch is pending it
   shows the spinner.
5. Given a full card at `new`, `clarified`, `specified` or `implemented`,
   then it keeps its full-chain button, unchanged.

### US2 (P1) - The stop stage decides, not the column name

As a user of a project whose full chain stops at `implemented`, I get the
same shortcut as soon as that chain has nothing left to do.

1. Given a project whose stop stage is `implemented`, a full card at
   `implemented` shows the autonomous-step button instead of `>>`, and the
   button launches `adjust` autonomously.
2. Given the same project, a full card at `reviewed` shows the button, which
   launches `handoff` autonomously.
3. Given the same project, a full card at `specified` keeps `>>`.
4. Given a project with no stop stage recorded, the card behaves as for the
   `reviewed` stop stage.

### US3 (P2) - The condensed card follows the same rule

1. Given a condensed card at or past the stop stage, when I open `(...)`,
   then it lists "Avancer en autonome" and no "Chaîne complète" entry.
2. Given a condensed card before the stop stage, then its `(...)` still lists
   "Chaîne complète".
3. Given either card shape, then "Copier /pickup-issue" (the pickup copy
   entry) is still listed in the `(...)` menu.

### US4 (P3) - Finished tasks are unchanged

1. Given a finished task, then the full card keeps its disabled `>>` and
   shows no autonomous-step button, and the condensed menu keeps its
   disabled "Chaîne complète" entry.

## Functional requirements

- **FR1** One shared rule decides whether the full chain has work left for a
  task: its resolved stage comes strictly before the project's full-chain stop
  stage in the workflow order (`new`, `clarified`, `specified`, `implemented`,
  `reviewed`, `finished`). A missing stop stage means `reviewed`. Both card
  shapes use this rule.
- **FR2** A card is *swapped* when the full chain has no work left and the
  task is not finished. A task that is not swapped renders its full-chain
  controls exactly as today.
- **FR3** A swapped full card renders, in the place of `>>`, a button that
  launches the stage's next step in autonomous mode through the same path as
  the menu's "Avancer en autonome", with the card's launch model.
- **FR4** The autonomous-step button uses the `Bot` icon, shares the card's
  pending-launch guard (disabled while any launch of the card is pending), and
  shows the spinner while its own launch is pending.
- **FR5** Its tooltip and accessible name are a catalog string naming the
  next step's skill (its project label) and the stage's next-step
  description, in French and English.
- **FR6** A swapped condensed card's `(...)` menu omits the "Chaîne complète"
  entry and keeps "Avancer en autonome"; no new menu entry is added.
- **FR7** The pickup copy entry is unaffected by the swap.
- **FR8** `CHANGELOG.md` gains one line under `## [Unreleased]` → `Changed`.

## Acceptance criteria

- **AC1** (FR1) Unit test: the rule answers "work left" for every stage before
  the stop stage and "none" at and after it, for both stop stages and for a
  project without one.
- **AC2** (US1, FR2-FR4) Browser test, stop stage `reviewed`: a full card at
  `reviewed` has no `>>` and an autonomous-step button whose click calls the
  advance with `auto` false and mode `autonomous`; a card at `implemented`
  keeps `>>`, whose click calls the advance with `auto` true.
- **AC3** (US2) Browser test, stop stage `implemented`: cards at
  `implemented` and `reviewed` are swapped, a card at `specified` is not.
- **AC4** (US3, FR6, FR7) Browser test: a swapped condensed card's menu has
  "Avancer en autonome", no "Chaîne complète", and the pickup copy entry; an
  unswapped one has "Chaîne complète".
- **AC5** (US4) Browser test: a finished full card has a disabled `>>` and no
  autonomous-step button.
- **AC6** (FR5) The French and English locale objects type-check with the new
  key.
- **AC7** (FR8) `CHANGELOG.md` carries the line under `[Unreleased]`.

## Open requirements

None. The three product questions were answered in clarification round 2.
