# Specification #622 - Remove the description from the card view

- Ticket: https://github.com/sebastienferry/sectile/issues/622
- Branch: `claude/clarify-issue-gh-11a4f59c-fb52c8`
- Clarification: `docs/clarifications/622.md` (rounds 1 and 2, confirmed by
  the owner)
- Framework: Spec Kit

## Summary

On the board, a task card shows a two-line excerpt of the task's description
under its title, on the standard and comfortable densities. With this change the
card no longer shows it, on any density, so the card shows the key, title,
badges and links only. The full description stays in the task detail panel.

## Scope

In scope: the task card of the board, in Sectile web and Sectile Desktop (which
renders the same interface), on every density, and the changelog.

Out of scope:

- The list view, which keeps its one-line description excerpt.
- The task detail panel, which keeps the full description.
- The description itself: it is still stored, synchronised with the tracker and
  editable as today.
- A display setting to show or hide the description: none is added.

## User stories

### US1 (P1) - A card without its description

As a board user, I scan cards that show their key, title and badges without the
description text.

1. Given a task with a description, when the board shows its card on the
   standard density, then the card does not show any of the description text.
2. Given the same task, when the board uses the comfortable density, then the
   card does not show any of the description text.
3. Given the same task, when the board uses the compact density, then the
   condensed card still shows no description text, as today.
4. Given a task without a description, when the board shows its card on any
   density, then the card looks the same as a card whose task has a description.

### US2 (P1) - The description remains reachable

As a board user, I still read a task's description where I read it in full
today.

1. Given a task with a description, when I open the task from its card, then
   the detail panel shows the full description, as today.
2. Given the list view, when rows are not condensed and the task has no latest
   activity, then the row still shows the one-line description excerpt, as
   today.

## Functional requirements

- **FR1** The board card renders no description text on the standard,
  comfortable and compact densities.
- **FR2** The card's other content (key, external link, project badge,
  priority, issue type, title, PR icon, labels, branch and action controls) is
  unchanged.
- **FR3** The list view's description excerpt and the detail panel's
  description are unchanged.
- **FR4** No new setting, stored key or translated string is introduced.
- **FR5** `CHANGELOG.md` gains one user-facing line under `## [Unreleased]`.

## Acceptance criteria

- **AC1** (FR1) On the standard and comfortable densities, a card whose task has
  a description contains none of the description text.
- **AC2** (FR1) On the compact density, the condensed card still contains none of
  the description text.
- **AC3** (FR2) The existing card browser tests pass unchanged, apart from the
  description assertion that AC1 inverts.
- **AC4** (FR3) The list view and the detail panel still show the description.
- **AC5** (FR5) `CHANGELOG.md` `## [Unreleased]` has a `Changed` line
  describing the removal.

## Open points

None. Both product questions were settled in the clarification's round 2.
