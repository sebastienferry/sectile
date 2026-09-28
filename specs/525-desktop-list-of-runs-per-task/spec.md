# Specification #525 - Lighter execution history drop-down in the desktop toolbar

- Ticket: https://github.com/sebastienferry/sectile/issues/525
- Branch: `feat/525`
- Clarification: `docs/clarifications/525.md` (rounds 1 and 2, confirmed by
  the owner)
- Framework: Spec Kit

## Summary

The desktop console toolbar lists the executions of the selected task in a
drop-down. Today it is drawn as a boxed field (surface background, strong
border), heavier than the unboxed icon buttons next to it. It becomes a ghost
control: no border and no background at rest, a background on hover, the
accent focus ring on keyboard focus, and a discreet chevron.

## Scope

In scope: the look of the execution history drop-down of the desktop toolbar,
in the dark and light themes, and a `Changed` line in `CHANGELOG.md`.

Out of scope:

- The web client.
- The other drop-downs of the desktop (settings, dialogs, ticket compose).
- The option labels (`N · skill · status`), which executions are listed, their
  order, and the rule that hides the drop-down under two executions.
- Replacing the native drop-down with a custom menu.

## Definitions

- **Execution history drop-down**: the `Execution history` select of the
  console toolbar, shown when the selected task has two executions or more.
- **Ghost look**: the look of the toolbar icon buttons: no border, no
  background fill at rest.

## User stories (prioritised)

### US1 - A drop-down that does not look like a form field (P1)

As a desktop user, I see the execution history in the toolbar as a light
control that matches the icon buttons around it, not as a boxed form field.

**Acceptance scenarios**

1. Given a selected task with two executions, when the toolbar renders, then
   the execution history drop-down shows no border and no background fill, its
   text reads `N · skill · status` as before, and a discreet chevron follows
   the text.
2. Given the same toolbar, when the pointer hovers the drop-down, then it gets
   the theme hover background.
3. Given the same toolbar, when the drop-down gets the keyboard focus, then it
   shows the accent focus ring used by the toolbar icon buttons.
4. Given the light theme and then the dark theme, when the toolbar renders,
   then the text, the chevron, the hover background and the focus ring follow
   the active theme.

### US2 - Nothing else changes (P1)

As a desktop user, I use the drop-down exactly as before.

**Acceptance scenarios**

1. Given a selected task with two executions or more, when I pick another
   entry, then its console is selected, as before.
2. Given a selected task with a single execution, then the drop-down stays
   hidden.
3. Given the other drop-downs of the desktop (settings, ticket compose), then
   they keep their boxed look.
4. Given a long entry label in a narrow window, then the drop-down keeps its
   maximum width and stays inside the toolbar.

## Functional requirements

- **FR1** At rest, the execution history drop-down has no visible border and a
  transparent background.
- **FR2** It replaces the native boxed arrow with a discreet chevron drawn in
  the muted text colour of the theme.
- **FR3** On hover, its background takes the theme hover background
  (settled in round 2 of the clarification).
- **FR4** On keyboard focus, it shows the accent focus ring of the toolbar
  icon buttons (2px accent outline, 2px offset).
- **FR5** Its entries stay readable in the opened list, in both themes.
- **FR6** Only the execution history drop-down changes; every other
  drop-down of the desktop keeps its current look.
- **FR7** Option labels, listing, ordering, selection and visibility rules are
  unchanged.
- **FR8** `CHANGELOG.md` gets one `Changed` line under `## [Unreleased]`.

## Open points

None. Note for the reviewer: inside the toolbar the icon buttons signal hover
by turning accent-coloured on a transparent background; the clarification
settled a hover background for the drop-down (FR3), which gives the text
control a visible target.
