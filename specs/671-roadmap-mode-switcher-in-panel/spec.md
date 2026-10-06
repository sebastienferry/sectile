# Specification #671 - Roadmap: move the mode switcher into the side panel

- Ticket: https://github.com/sebastienferry/sectile/issues/671
- Branch: `feat/671`
- Clarification: `docs/clarifications/671.md` (rounds 1 and 2, confirmed by
  the owner on 2026-10-06)
- Framework: Spec Kit

## Summary

The Roadmap's four-way mode switcher (Framing, Execution, Phases, Goals)
leaves the top toolbar and becomes a tab strip at the bottom of the selected
macro panel's header. The mode then only decides what the panel's body shows.
The list's own Execution behaviour (placement badges, the "À corriger" toggle
and its filter, the sprint strip) follows the horizon tab instead.

## Scope

In scope:

- the switcher's place and markup in the Roadmap (web build, which the desktop
  app loads as is);
- the list elements that read the mode today and will read the horizon tab;
- the Roadmap browser tests and the changelog.

Out of scope:

- new modes, and any change to what a mode's panel body contains;
- persisting the chosen mode across reloads;
- any server, tracker or desktop change.

## User stories

### US1 - The switcher belongs to the panel (P1)

As a Roadmap user, I pick what the selected macro's panel shows from the panel
itself, so the control sits next to what it changes.

- **Given** a selected macro and its panel shown, **when** I look at the
  panel, **then** Framing, Execution, Phases and Goals are tabs at the bottom
  of its header, right above the scrolling body, and the toolbar has no mode
  switcher.
- **Given** the panel is expanded, **when** I look at it, **then** the same
  tabs sit at the same place.
- **Given** no macro is selected, or the panel is hidden, **when** I look at
  the Roadmap, **then** no mode switcher is shown anywhere.
- **Given** a mode tab, **when** I click it, **then** it becomes the selected
  tab and the panel body shows that mode, as it does today.

### US2 - The list follows the horizon tab (P1)

As a Roadmap user, I see placement problems on Now and Next whatever the panel
shows, so reading a macro's framing does not hide the sprint check.

- **Given** the Now or Next tab, **when** the panel is in any mode, **then**
  each unfolded row shows its placement badge ("N à corriger" or "Tout est
  placé"), the toolbar shows the "À corriger" toggle, and the sprint strip is
  shown above the list.
- **Given** the Now or Next tab with "À corriger" on, **when** the panel is in
  any mode, **then** the list keeps only the macros with a placement issue.
- **Given** the Later, Unclassified or Masqués tab, **when** the panel is in
  any mode, **then** each unfolded row shows its TODO count, and neither the
  toggle nor the sprint strip is shown, and the list is not filtered by
  "À corriger".

### US3 - The default mode per horizon stays (P2)

- **Given** any panel mode, **when** I switch to the Later tab, **then** the
  panel shows Framing.
- **Given** any panel mode, **when** I switch to the Now or Next tab, **then**
  the panel shows Execution.
- **Given** a panel mode, **when** I select another macro in the same tab,
  **then** the mode is kept.

## Functional requirements

- **FR1** The switcher is a `tablist` of four `tab` elements, in the order
  Framing, Execution, Phases, Goals, the selected one carrying
  `aria-selected="true"`. Labels, icons and titles are unchanged.
- **FR2** The switcher is rendered only inside the macro panel, as the last
  block of its header, in both the normal and the expanded layout. The toolbar
  and the hidden-panel rail carry none.
- **FR3** Picking a tab changes the panel body only: nothing in the list or
  the toolbar depends on the mode.
- **FR4** On Now and Next, the unfolded row badge is the placement badge, the
  "À corriger" toggle is in the toolbar, its filter applies to the list, and
  the sprint strip is shown. On every other tab, the unfolded row badge is the
  TODO count, there is no toggle and no sprint strip, and the filter does not
  apply.
- **FR5** The "À corriger" state is kept when changing tab; it is only applied
  on Now and Next.
- **FR6** Changing horizon tab resets the mode to Framing on Later and to
  Execution on Now and Next; Unclassified and Masqués keep the current mode.
  Selecting another macro does not reset it.
- **FR7** The mode is not persisted across reloads.
- **FR8** `CHANGELOG.md` gains one `Changed` line under `## [Unreleased]`.

## Acceptance criteria

1. The four modes are tabs at the bottom of the macro panel's header, in both
   the normal and the expanded panel, and no longer in the toolbar.
2. With no macro selected or the panel hidden, no mode switcher is shown.
3. Changing tab in the panel changes only the panel body.
4. On Now and Next, the placement badges, the "À corriger" toggle with its
   filter and the sprint strip are shown whatever the panel's mode; on Later,
   Unclassified and Masqués, rows show the TODO count and neither the toggle
   nor the sprint strip is shown.
5. Changing horizon tab still resets the panel to Framing on Later and to
   Execution on Now and Next.
6. The Roadmap browser tests that select a mode use the new tabs, and a test
   covers criteria 2 and 4.
7. A `Changed` line under `## [Unreleased]` in `CHANGELOG.md`.

## Open points

None. Every product question was answered in the clarification.
